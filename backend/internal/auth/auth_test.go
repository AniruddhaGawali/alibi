package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"backend/internal/email"
	"backend/internal/queue"
	"backend/internal/user"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// fakeStore is an in-memory user.Store.
type fakeStore struct {
	byID map[string]*user.User
}

func (f *fakeStore) Create(_ context.Context, username, addr, hash string, tokens int) (*user.User, error) {
	for _, u := range f.byID {
		if strings.EqualFold(u.Email, addr) {
			return nil, user.ErrEmailTaken
		}
		if strings.EqualFold(u.Username, username) {
			return nil, user.ErrUsernameTaken
		}
	}
	u := &user.User{ID: "user-" + strconv.Itoa(len(f.byID)+1), Username: username, Email: addr, PasswordHash: hash}
	f.byID[u.ID] = u
	return u, nil
}

func (f *fakeStore) Availability(_ context.Context, addr, username string) (bool, bool, error) {
	var e, n bool
	for _, u := range f.byID {
		e = e || strings.EqualFold(u.Email, addr)
		n = n || strings.EqualFold(u.Username, username)
	}
	return e, n, nil
}

func (f *fakeStore) GetByEmail(_ context.Context, addr string) (*user.User, error) {
	for _, u := range f.byID {
		if strings.EqualFold(u.Email, addr) {
			return u, nil
		}
	}
	return nil, user.ErrNotFound
}

func (f *fakeStore) GetByID(_ context.Context, id string) (*user.User, error) {
	if u, ok := f.byID[id]; ok {
		return u, nil
	}
	return nil, user.ErrNotFound
}

func (f *fakeStore) SetAvatarDefault(context.Context, string, int) (string, error) { return "", nil }
func (f *fakeStore) SetAvatarCustom(context.Context, string, string, string) (string, error) {
	return "", nil
}

// fakeMail records published email jobs; fail makes Publish error.
type fakeMail struct {
	sent []email.Message
	fail bool
}

func (m *fakeMail) Publish(_ context.Context, topic string, payload []byte) error {
	if m.fail {
		return errors.New("redis down")
	}
	if topic != email.Topic {
		panic("published to unexpected topic " + topic)
	}
	var msg email.Message
	if err := json.Unmarshal(payload, &msg); err != nil {
		return err
	}
	m.sent = append(m.sent, msg)
	return nil
}

type fakeCoins struct{ cached map[string]int64 }

func (c *fakeCoins) Cache(_ context.Context, id string, n int64) { c.cached[id] = n }

type env struct {
	h       *Handler
	store   *fakeStore
	mail    *fakeMail
	coins   *fakeCoins
	tm      *TokenManager
	bl      *Blacklist
	mr      *miniredis.Miniredis
	handler http.Handler
}

func newEnv(t *testing.T) *env {
	t.Helper()
	mr := miniredis.RunT(t)
	// same fail-fast settings as db.ConnectRedis, so Redis-down tests do not crawl
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), DialerRetries: 1, DialerRetryTimeout: 5 * time.Millisecond, MaxRetries: 1, MinRetryBackoff: time.Millisecond, MaxRetryBackoff: 5 * time.Millisecond})
	t.Cleanup(func() { _ = rdb.Close() })

	_, private, _ := net.ParseCIDR("10.0.0.0/8")
	e := &env{
		store: &fakeStore{byID: map[string]*user.User{}},
		mail:  &fakeMail{},
		coins: &fakeCoins{cached: map[string]int64{}},
		tm:    NewTokenManager(strings.Repeat("s", 32), 15*time.Minute, time.Hour),
		bl:    NewBlacklist(rdb),
		mr:    mr,
	}
	e.h = NewHandler(Deps{
		Users: e.store, Tokens: e.tm, Blacklist: e.bl,
		OTP:     NewOTPStore(rdb, 10*time.Minute, 3, []byte("test-hmac-key")),
		Limiter: NewLimiter(rdb, Policy{Cooldown: time.Minute, EmailDailyMax: 3, IPMax: 4, IPWindow: 15 * time.Minute}),
		Mail:    e.mail, Coins: e.coins,
		TrustedProxies: []*net.IPNet{private},
		InitialTokens:  func(context.Context) int { return 10 },
	})
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/signup", e.h.Signup)
	mux.HandleFunc("POST /api/auth/resend-otp", e.h.ResendOTP)
	mux.HandleFunc("POST /api/auth/verify-otp", e.h.VerifyOTP)
	mux.HandleFunc("POST /api/auth/login", e.h.Login)
	mux.HandleFunc("POST /api/auth/refresh", e.h.Refresh)
	mux.HandleFunc("POST /api/auth/logout", e.h.Logout)
	mux.HandleFunc("GET /api/me", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(UserID(r.Context())))
	})
	mux.HandleFunc("GET /api/open/", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("open")) })
	public := NewPublicRoutes("POST /api/auth/signup", "POST /api/auth/resend-otp", "POST /api/auth/verify-otp",
		"POST /api/auth/login", "POST /api/auth/refresh", "GET /api/open/")
	e.handler = Middleware(e.tm, e.bl, public)(mux)
	return e
}

// testPassword is generated per run (it satisfies the password rule: letters +
// digits); bodies write {{PW}} where it belongs, so no credential-like literal
// is committed.
var testPassword = "Pw" + strconv.FormatInt(time.Now().UnixNano(), 36)

type tokens struct {
	JWT     string `json:"jwt_token"`
	Refresh string `json:"refresh_token"`
}

// do sends a request from the given client IP (default 203.0.113.1).
func (e *env) do(method, path, body, bearer string, ip ...string) *httptest.ResponseRecorder {
	body = strings.ReplaceAll(body, "{{PW}}", testPassword)
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body == "" {
		req.Body = http.NoBody
	}
	req.RemoteAddr = "203.0.113.1:5000"
	if len(ip) > 0 {
		req.RemoteAddr = ip[0] + ":5000"
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, req)
	return rec
}

func decodeTokens(t *testing.T, rec *httptest.ResponseRecorder) tokens {
	t.Helper()
	var env struct {
		Data tokens `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	return env.Data
}

// pendingOTP returns the code most recently mailed to addr (the only place the
// plaintext exists), after checking a session is pending for it.
func (e *env) pendingOTP(t *testing.T, addr string) string {
	t.Helper()
	if _, err := e.mr.Get(sessionKey(addr)); err != nil {
		t.Fatalf("no pending session for %s: %v", addr, err)
	}
	for i := len(e.mail.sent) - 1; i >= 0; i-- {
		if e.mail.sent[i].To == addr {
			return e.mail.sent[i].OTP
		}
	}
	t.Fatalf("no email queued for %s", addr)
	return ""
}

const signupBody = `{"username":"sherlock","email":"Sherlock@Example.com","password":"{{PW}}"}`

// signup runs the full flow (signup -> OTP -> verify) from a fresh IP.
func signup(t *testing.T, e *env) tokens {
	t.Helper()
	return signupAs(t, e, "sherlock", "sherlock@example.com", "203.0.113."+strconv.Itoa(len(e.store.byID)+10))
}

func signupAs(t *testing.T, e *env, username, addr, ip string) tokens {
	t.Helper()
	body := `{"username":"` + username + `","email":"` + addr + `","password":"{{PW}}"}`
	if rec := e.do("POST", "/api/auth/signup", body, "", ip); rec.Code != 200 {
		t.Fatalf("signup: %d %s", rec.Code, rec.Body)
	}
	rec := e.do("POST", "/api/auth/verify-otp", `{"email":"`+addr+`","otp":"`+e.pendingOTP(t, addr)+`"}`, "", ip)
	if rec.Code != http.StatusCreated {
		t.Fatalf("verify: %d %s", rec.Code, rec.Body)
	}
	return decodeTokens(t, rec)
}

func TestSignupCreatesNoUserAndQueuesTheOTPEmail(t *testing.T) {
	e := newEnv(t)
	rec := e.do("POST", "/api/auth/signup", signupBody, "")
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if len(e.store.byID) != 0 {
		t.Fatal("no user may exist before the OTP is verified")
	}
	if len(e.mail.sent) != 1 || e.mail.sent[0].To != "sherlock@example.com" || e.mail.sent[0].Type != email.TypeSignup {
		t.Fatalf("queued = %+v", e.mail.sent)
	}
	otp := e.pendingOTP(t, "sherlock@example.com")
	if e.mail.sent[0].OTP != otp || len(otp) != 6 {
		t.Fatalf("mailed %q, stored %q", e.mail.sent[0].OTP, otp)
	}
	if ttl := e.mr.TTL(sessionKey("sherlock@example.com")); ttl <= 9*time.Minute || ttl > 10*time.Minute {
		t.Fatalf("session ttl = %v", ttl)
	}
	// The stored password is a bcrypt hash, never the password itself.
	raw, _ := e.mr.Get(sessionKey("sherlock@example.com"))
	var s Session
	_ = json.Unmarshal([]byte(raw), &s)
	if s.Username != "sherlock" || !strings.HasPrefix(s.HashedPassword, "$2") || strings.Contains(raw, testPassword) {
		t.Fatalf("session = %s", raw)
	}
	if ttl := e.mr.TTL("ratelimit:otp:email:sherlock@example.com"); ttl <= 55*time.Second || ttl > time.Minute {
		t.Errorf("email cooldown ttl = %v, want ~60s", ttl)
	}
	if v, _ := e.mr.Get("ratelimit:otp:ip:203.0.113.1"); v != "1" {
		t.Errorf("ip counter = %q, want 1", v)
	}
	if ttl := e.mr.TTL("ratelimit:otp:ip:203.0.113.1"); ttl <= 14*time.Minute || ttl > 15*time.Minute {
		t.Errorf("ip window ttl = %v, want ~15m", ttl)
	}
	if v, _ := e.mr.Get("ratelimit:otpdaily:email:sherlock@example.com"); v != "1" {
		t.Errorf("daily counter = %q, want 1", v)
	}
}

func TestSignupValidationAndDuplicates(t *testing.T) {
	e := newEnv(t)
	for _, body := range []string{
		`{"username":"ab","email":"a@b.co","password":"{{PW}}"}`,
		`{"username":"sherlock","email":"nope","password":"{{PW}}"}`,
		`{"username":"sherlock","email":"a@b.co","password":"short1"}`,
		`{"username":"sherlock","email":"a@b.co","password":"onlyletters"}`,
		`{"username":"sherlock","email":"a@b.co","password":"{{PW}}","admin":true}`,
	} {
		if rec := e.do("POST", "/api/auth/signup", body, ""); rec.Code != 422 && rec.Code != 400 {
			t.Errorf("%s: got %d", body, rec.Code)
		}
	}
	if len(e.mail.sent) != 0 {
		t.Fatal("invalid signups must not send email")
	}

	signup(t, e)
	rec := e.do("POST", "/api/auth/signup", `{"username":"other","email":"SHERLOCK@example.com","password":"{{PW}}"}`, "", "198.51.100.1")
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "email") {
		t.Fatalf("duplicate email: %d %s", rec.Code, rec.Body)
	}
	rec = e.do("POST", "/api/auth/signup", `{"username":"Sherlock","email":"new@example.com","password":"{{PW}}"}`, "", "198.51.100.2")
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "username") {
		t.Fatalf("duplicate username: %d %s", rec.Code, rec.Body)
	}
}

func signupBodyFor(name, addr string) string {
	return `{"username":"` + name + `","email":"` + addr + `","password":"` + testPassword + `"}`
}

func TestEmailCooldownBlocksSameEmailFromAnyIP(t *testing.T) {
	e := newEnv(t)
	if rec := e.do("POST", "/api/auth/signup", signupBodyFor("one", "one@x.co"), "", "198.51.100.1"); rec.Code != 200 {
		t.Fatalf("first: %d", rec.Code)
	}
	rec := e.do("POST", "/api/auth/signup", signupBodyFor("two", "one@x.co"), "", "198.51.100.2")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("same email, other ip: %d", rec.Code)
	}
	secs, _ := strconv.Atoi(rec.Header().Get("Retry-After"))
	if secs < 55 || secs > 60 || !strings.Contains(rec.Body.String(), "retry_after_seconds") {
		t.Fatalf("retry-after = %q body=%s", rec.Header().Get("Retry-After"), rec.Body)
	}
	if len(e.mail.sent) != 1 {
		t.Fatalf("the blocked request must not queue mail, sent=%d", len(e.mail.sent))
	}
	// The lockout a stranger can cause lasts one cooldown, not longer.
	e.mr.FastForward(time.Minute + time.Second)
	if rec := e.do("POST", "/api/auth/signup", signupBodyFor("two", "one@x.co"), "", "198.51.100.2"); rec.Code != 200 {
		t.Fatalf("after cooldown: %d", rec.Code)
	}
}

func TestSharedIPCanSignSeveralPeopleUpUntilItsBudgetRunsOut(t *testing.T) {
	e := newEnv(t) // IP budget: 4 per 15 minutes
	office := "198.51.100.50"
	for i, name := range []string{"ann", "bob", "cho", "dev"} {
		rec := e.do("POST", "/api/auth/signup", signupBodyFor(name, name+"@x.co"), "", office)
		if rec.Code != 200 {
			t.Fatalf("person %d from a shared IP: %d %s", i+1, rec.Code, rec.Body)
		}
	}
	rec := e.do("POST", "/api/auth/signup", signupBodyFor("eve", "eve@x.co"), "", office)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("5th request from the IP: %d, want 429", rec.Code)
	}
	if secs, _ := strconv.Atoi(rec.Header().Get("Retry-After")); secs < 14*60 || secs > 15*60 {
		t.Fatalf("retry-after = %d, want up to the 15 minute window", secs)
	}
	// Other IPs are unaffected, and the window resets.
	if rec := e.do("POST", "/api/auth/signup", signupBodyFor("eve", "eve@x.co"), "", "198.51.100.51"); rec.Code != 200 {
		t.Fatalf("other ip: %d", rec.Code)
	}
	e.mr.FastForward(15*time.Minute + time.Second)
	if rec := e.do("POST", "/api/auth/signup", signupBodyFor("fay", "fay@x.co"), "", office); rec.Code != 200 {
		t.Fatalf("after window: %d", rec.Code)
	}
}

func TestEmailDailyCapStopsMailBombing(t *testing.T) {
	e := newEnv(t) // 3 codes per email per day, 60s apart
	for i := 0; i < 3; i++ {
		ip := "198.51.100." + strconv.Itoa(10+i)
		if rec := e.do("POST", "/api/auth/signup", signupBodyFor("victim", "victim@x.co"), "", ip); rec.Code != 200 {
			t.Fatalf("send %d: %d %s", i+1, rec.Code, rec.Body)
		}
		e.mr.FastForward(time.Minute + time.Second) // clear the 60s cooldown only
	}
	rec := e.do("POST", "/api/auth/signup", signupBodyFor("victim", "victim@x.co"), "", "198.51.100.99")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("4th code in a day: %d, want 429", rec.Code)
	}
	if secs, _ := strconv.Atoi(rec.Header().Get("Retry-After")); secs < 20*3600 {
		t.Fatalf("retry-after = %ds, want hours (daily cap)", secs)
	}
	if len(e.mail.sent) != 3 {
		t.Fatalf("sent = %d, want 3", len(e.mail.sent))
	}
	e.mr.FastForward(24 * time.Hour)
	if rec := e.do("POST", "/api/auth/signup", signupBodyFor("victim", "victim@x.co"), "", "198.51.100.99"); rec.Code != 200 {
		t.Fatalf("next day: %d", rec.Code)
	}
}

func TestForgedForwardingHeadersDoNotBypassTheIPLimit(t *testing.T) {
	e := newEnv(t) // IP budget: 4
	send := func(i int) int {
		name := "user" + strconv.Itoa(i)
		fake := "9.9.9." + strconv.Itoa(i)
		req := httptest.NewRequest("POST", "/api/auth/signup", strings.NewReader(signupBodyFor(name, name+"@x.co")))
		req.RemoteAddr = "198.51.100.9:1" // direct, untrusted peer
		req.Header.Set("X-Forwarded-For", fake)
		req.Header.Set("X-Real-IP", fake)
		rec := httptest.NewRecorder()
		e.handler.ServeHTTP(rec, req)
		return rec.Code
	}
	for i := 1; i <= 4; i++ {
		if c := send(i); c != 200 {
			t.Fatalf("request %d: %d", i, c)
		}
	}
	if c := send(5); c != http.StatusTooManyRequests {
		t.Fatalf("a fresh spoofed address bypassed the limit: %d", c)
	}
}

func TestQueueFailureRollsBackAndGivesTheCooldownBack(t *testing.T) {
	e := newEnv(t)
	e.mail.fail = true
	if rec := e.do("POST", "/api/auth/signup", signupBody, ""); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("got %d", rec.Code)
	}
	if e.mr.Exists(sessionKey("sherlock@example.com")) || e.mr.Exists("ratelimit:otp:email:sherlock@example.com") {
		t.Fatal("state left behind after a failed enqueue")
	}
	for _, key := range []string{"ratelimit:otpdaily:email:sherlock@example.com", "ratelimit:otp:ip:203.0.113.1"} {
		if v, _ := e.mr.Get(key); v != "0" && v != "" {
			t.Fatalf("%s = %q, the failed attempt must not count against the budget", key, v)
		}
	}
	e.mail.fail = false
	if rec := e.do("POST", "/api/auth/signup", signupBody, ""); rec.Code != 200 {
		t.Fatalf("retry after failure: %d", rec.Code)
	}
}

func TestVerifyOTPCreatesUserWithStartingTokensAndSignsIn(t *testing.T) {
	e := newEnv(t)
	tk := signup(t, e)
	if tk.JWT == "" || tk.Refresh == "" {
		t.Fatalf("missing tokens: %+v", tk)
	}
	u, err := e.store.GetByEmail(context.Background(), "sherlock@example.com")
	if err != nil || !strings.HasPrefix(u.PasswordHash, "$2") {
		t.Fatalf("user = %+v err=%v", u, err)
	}
	if e.coins.cached[u.ID] != 10 {
		t.Fatalf("coins cache = %v, want 10", e.coins.cached)
	}
	if e.mr.Exists(sessionKey("sherlock@example.com")) {
		t.Fatal("session should be deleted after verification")
	}
	claims, err := e.tm.Parse(tk.JWT, Access)
	if err != nil || claims.Subject != u.ID || claims.Email != "sherlock@example.com" || claims.ID == "" || claims.ExpiresAt == nil {
		t.Fatalf("claims = %+v err=%v", claims, err)
	}
}

func TestVerifyOTPRejectsWrongExpiredAndMissing(t *testing.T) {
	e := newEnv(t)
	e.do("POST", "/api/auth/signup", signupBody, "")
	good := e.pendingOTP(t, "sherlock@example.com")
	wrong := "000000"
	if good == wrong {
		wrong = "111111"
	}
	verify := func(addr, otp string) *httptest.ResponseRecorder {
		return e.do("POST", "/api/auth/verify-otp", `{"email":"`+addr+`","otp":"`+otp+`"}`, "")
	}

	if rec := verify("sherlock@example.com", wrong); rec.Code != 400 || !strings.Contains(rec.Body.String(), "OTP expired or invalid") {
		t.Fatalf("wrong code: %d %s", rec.Code, rec.Body)
	}
	if rec := verify("nobody@example.com", good); rec.Code != 400 {
		t.Fatalf("no session: %d", rec.Code)
	}
	if rec := e.do("POST", "/api/auth/verify-otp", `{"email":"sherlock@example.com","otp":"12ab56"}`, ""); rec.Code != 422 {
		t.Fatalf("non-numeric: %d", rec.Code)
	}
	if len(e.store.byID) != 0 {
		t.Fatal("no user may be created by a failed verification")
	}
	e.mr.FastForward(10*time.Minute + time.Second)
	if rec := verify("sherlock@example.com", good); rec.Code != 400 {
		t.Fatalf("expired: %d", rec.Code)
	}
}

func TestTooManyWrongCodesDiscardTheSession(t *testing.T) {
	e := newEnv(t) // max 3 attempts
	e.do("POST", "/api/auth/signup", signupBody, "")
	good := e.pendingOTP(t, "sherlock@example.com")
	wrong := "000000"
	if good == wrong {
		wrong = "111111"
	}
	for i := 0; i < 3; i++ {
		e.do("POST", "/api/auth/verify-otp", `{"email":"sherlock@example.com","otp":"`+wrong+`"}`, "")
	}
	// Even the right code no longer works: brute forcing is capped.
	if rec := e.do("POST", "/api/auth/verify-otp", `{"email":"sherlock@example.com","otp":"`+good+`"}`, ""); rec.Code != 400 {
		t.Fatalf("got %d, want 400 after attempts exhausted", rec.Code)
	}
	if len(e.store.byID) != 0 {
		t.Fatal("user created after exhausted attempts")
	}
}

func TestVerifyOTPConflictWhenTakenMeanwhile(t *testing.T) {
	e := newEnv(t)
	e.do("POST", "/api/auth/signup", signupBody, "")
	otp := e.pendingOTP(t, "sherlock@example.com")
	// Someone else registers the username while this signup is pending.
	_, _ = e.store.Create(context.Background(), "Sherlock", "other@example.com", "h", 10)
	rec := e.do("POST", "/api/auth/verify-otp", `{"email":"sherlock@example.com","otp":"`+otp+`"}`, "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
	if e.mr.Exists(sessionKey("sherlock@example.com")) {
		t.Fatal("dead session should be removed")
	}
}

func TestResendOTP(t *testing.T) {
	e := newEnv(t)
	resend := func(addr, ip string) *httptest.ResponseRecorder {
		return e.do("POST", "/api/auth/resend-otp", `{"email":"`+addr+`"}`, "", ip)
	}

	// No pending signup: indistinguishable from a real resend, but nothing is sent.
	if rec := resend("nobody@example.com", "198.51.100.1"); rec.Code != 200 {
		t.Fatalf("no session: %d", rec.Code)
	}
	if len(e.mail.sent) != 0 {
		t.Fatalf("unknown signup must not send mail: %+v", e.mail.sent)
	}

	e.do("POST", "/api/auth/signup", signupBody, "", "198.51.100.5")
	first := e.pendingOTP(t, "sherlock@example.com")

	// Still inside the email cooldown from signup, whichever IP asks.
	if rec := resend("sherlock@example.com", "198.51.100.6"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("email cooldown: %d", rec.Code)
	}
	if rec := resend("sherlock@example.com", "198.51.100.5"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("ip cooldown: %d", rec.Code)
	}

	e.mr.FastForward(time.Minute + time.Second)
	// Wrong guesses first, to prove a resend also resets the attempt counter.
	e.do("POST", "/api/auth/verify-otp", `{"email":"sherlock@example.com","otp":"abcdef"}`, "") // 422, not counted
	if rec := resend("sherlock@example.com", "198.51.100.6"); rec.Code != 200 {
		t.Fatalf("resend: %d %s", rec.Code, rec.Body)
	}
	second := e.pendingOTP(t, "sherlock@example.com")
	if len(e.mail.sent) != 2 || e.mail.sent[1].OTP != second || e.mail.sent[1].To != "sherlock@example.com" {
		t.Fatalf("queued = %+v", e.mail.sent)
	}
	if ttl := e.mr.TTL(sessionKey("sherlock@example.com")); ttl <= 9*time.Minute {
		t.Fatalf("resend should restart the 10 minute window, ttl = %v", ttl)
	}
	// The previous code is dead (unless the two random codes collided).
	if first != second {
		if rec := e.do("POST", "/api/auth/verify-otp", `{"email":"sherlock@example.com","otp":"`+first+`"}`, "", "198.51.100.6"); rec.Code != 400 {
			t.Fatalf("old code still works: %d", rec.Code)
		}
	}
	if rec := e.do("POST", "/api/auth/verify-otp", `{"email":"sherlock@example.com","otp":"`+second+`"}`, "", "198.51.100.6"); rec.Code != 201 {
		t.Fatalf("new code: %d %s", rec.Code, rec.Body)
	}
}

func TestLogin(t *testing.T) {
	e := newEnv(t)
	signup(t, e)
	if rec := e.do("POST", "/api/auth/login", `{"email":"sherlock@example.com","password":"{{PW}}"}`, ""); rec.Code != 200 || decodeTokens(t, rec).JWT == "" {
		t.Fatalf("login: %d %s", rec.Code, rec.Body)
	}
	for _, body := range []string{
		`{"email":"sherlock@example.com","password":"{{PW}}x"}`,
		`{"email":"nobody@example.com","password":"{{PW}}"}`,
	} {
		if rec := e.do("POST", "/api/auth/login", body, ""); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: got %d", body, rec.Code)
		}
	}
}

func TestRefreshRotatesAndRejectsReuse(t *testing.T) {
	e := newEnv(t)
	tk := signup(t, e)
	body := `{"refresh_token":"` + tk.Refresh + `"}`

	rec := e.do("POST", "/api/auth/refresh", body, "")
	if rec.Code != 200 {
		t.Fatalf("refresh: %d %s", rec.Code, rec.Body)
	}
	next := decodeTokens(t, rec)
	if next.Refresh == tk.Refresh {
		t.Fatal("refresh token was not rotated")
	}
	if rec := e.do("POST", "/api/auth/refresh", body, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("reused refresh token: got %d", rec.Code)
	}
	// An access token must not work as a refresh token.
	if rec := e.do("POST", "/api/auth/refresh", `{"refresh_token":"`+tk.JWT+`"}`, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("access token as refresh: got %d", rec.Code)
	}
}

func TestLogoutBlacklistsBothTokensWithRemainingTTL(t *testing.T) {
	e := newEnv(t)
	tk := signup(t, e)
	claims, _ := e.tm.Parse(tk.JWT, Access)

	if rec := e.do("GET", "/api/me", "", tk.JWT); rec.Code != 200 {
		t.Fatalf("before logout: %d", rec.Code)
	}
	if rec := e.do("POST", "/api/auth/logout", `{"refresh_token":"`+tk.Refresh+`"}`, tk.JWT); rec.Code != 200 {
		t.Fatalf("logout: %d %s", rec.Code, rec.Body)
	}
	key := "blacklist:jwt:" + claims.ID
	if v, _ := e.mr.Get(key); v != "true" {
		t.Fatalf("%s = %q, want true", key, v)
	}
	if ttl := e.mr.TTL(key); ttl <= 0 || ttl > 15*time.Minute {
		t.Fatalf("access blacklist ttl = %v", ttl)
	}
	rclaims, _ := e.tm.Parse(tk.Refresh, Refresh)
	if ttl := e.mr.TTL("blacklist:jwt:" + rclaims.ID); ttl <= 15*time.Minute || ttl > time.Hour {
		t.Fatalf("refresh blacklist ttl = %v", ttl)
	}
	if rec := e.do("GET", "/api/me", "", tk.JWT); rec.Code != http.StatusUnauthorized {
		t.Fatalf("after logout: got %d", rec.Code)
	}
	if rec := e.do("POST", "/api/auth/refresh", `{"refresh_token":"`+tk.Refresh+`"}`, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("refresh after logout: got %d", rec.Code)
	}
}

func TestLogoutRequiresValidRefreshTokenOfSameUser(t *testing.T) {
	e := newEnv(t)
	tk := signup(t, e)
	other := signupAs(t, e, "watson", "w@example.com", "198.51.100.77")

	for name, body := range map[string]string{
		"no body":      "",
		"empty token":  `{"refresh_token":""}`,
		"garbage":      `{"refresh_token":"abc"}`,
		"access token": `{"refresh_token":"` + tk.JWT + `"}`,
		"other user's": `{"refresh_token":"` + other.Refresh + `"}`,
	} {
		if rec := e.do("POST", "/api/auth/logout", body, tk.JWT); rec.Code < 400 {
			t.Errorf("%s: got %d, want an error", name, rec.Code)
		}
	}
	// None of the failed attempts may have logged the caller out.
	if rec := e.do("GET", "/api/me", "", tk.JWT); rec.Code != 200 {
		t.Fatalf("token revoked by a rejected logout: %d", rec.Code)
	}
}

func TestMiddlewareRejectsBadTokens(t *testing.T) {
	e := newEnv(t)
	tk := signup(t, e)
	other := NewTokenManager(strings.Repeat("x", 32), time.Minute, time.Minute)
	forged, _, _ := other.Issue("user-1", "a@b.co", "user", Access)

	for name, tok := range map[string]string{"missing": "", "garbage": "abc", "wrong secret": forged, "refresh type": tk.Refresh} {
		if rec := e.do("GET", "/api/me", "", tok); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: got %d", name, rec.Code)
		}
	}
	e.tm.now = func() time.Time { return time.Now().Add(time.Hour) }
	if rec := e.do("GET", "/api/me", "", tk.JWT); rec.Code != http.StatusUnauthorized {
		t.Errorf("expired: got %d", rec.Code)
	}
}

func TestMiddlewareIsDenyByDefault(t *testing.T) {
	e := newEnv(t)
	tk := signup(t, e)

	// Public routes need no token.
	if rec := e.do("GET", "/api/open/anything", "", ""); rec.Code != 200 || rec.Body.String() != "open" {
		t.Fatalf("public route: %d %s", rec.Code, rec.Body)
	}
	// Everything else does, including paths that do not exist and wrong methods
	// on a public path, and it answers 401 before the handler or the mux runs.
	for _, c := range []struct{ method, path string }{
		{"GET", "/api/me"},
		{"GET", "/api/does-not-exist"},
		{"GET", "/api/auth/login"}, // login is public for POST only
		{"GET", "/api/open/../me"}, // path trick must not ride on the public prefix
		{"GET", "//api/open/x"},    // unclean path fails closed
	} {
		rec := e.do(c.method, c.path, "", "")
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without token: got %d, want 401", c.method, c.path, rec.Code)
		}
		if rec.Header().Get("WWW-Authenticate") == "" {
			t.Errorf("%s %s: missing WWW-Authenticate", c.method, c.path)
		}
	}
	// With a valid token the handler runs and sees the caller.
	if rec := e.do("GET", "/api/me", "", tk.JWT); rec.Code != 200 || rec.Body.String() != "user-1" {
		t.Fatalf("authenticated: %d %s", rec.Code, rec.Body)
	}
}

func TestMiddlewareFailsClosedWhenRedisIsDown(t *testing.T) {
	e := newEnv(t)
	tk := signup(t, e)
	e.mr.Close()
	if rec := e.do("GET", "/api/me", "", tk.JWT); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("got %d, want 503", rec.Code)
	}
}

// TestSignupToWorkerEndToEnd wires the API handler to a real Redis-list broker
// and the real email worker, with only the SMTP sender faked.
func TestSignupToWorkerEndToEnd(t *testing.T) {
	e := newEnv(t)
	rdb := redis.NewClient(&redis.Options{Addr: e.mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	broker := queue.NewRedisListBroker(rdb, email.DLQ, queue.WithPopTimeout(20*time.Millisecond))
	e.h.Mail = broker

	if rec := e.do("POST", "/api/auth/signup", signupBody, ""); rec.Code != 200 {
		t.Fatalf("signup: %d %s", rec.Code, rec.Body)
	}
	if n, _ := e.mr.List(email.Topic); len(n) != 1 {
		t.Fatalf("queue length = %d, want 1 (the API must only enqueue)", len(n))
	}

	sender := &captureSender{got: make(chan string, 1)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go email.NewWorker(broker, sender).Run(ctx, 1)

	select {
	case body := <-sender.got:
		code := regexp.MustCompile(`\b[0-9]{6}\b`).FindString(body)
		raw, _ := e.mr.Get(sessionKey("sherlock@example.com"))
		if code == "" || !strings.Contains(raw, e.h.OTP.Hash("sherlock@example.com", code)) {
			t.Fatalf("emailed code %q does not match the stored hash:\n%s", code, body)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("worker never sent the email")
	}
}

type captureSender struct{ got chan string }

func (c *captureSender) Send(_ context.Context, to, _, text, _ string) error {
	c.got <- to + "\n" + text
	return nil
}

func TestParallelWrongCodesGetOnlyMaxAttempts(t *testing.T) {
	e := newEnv(t) // 3 attempts
	e.do("POST", "/api/auth/signup", signupBody, "", "198.51.100.40")
	good := e.pendingOTP(t, "sherlock@example.com")
	wrong := "000000"
	if good == wrong {
		wrong = "111111"
	}
	e.h.Limiter.p.AuthIPMax = 1000 // isolate the attempt cap from the IP budget
	var wg sync.WaitGroup
	var mu sync.Mutex
	counted := map[int]int{}
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := e.do("POST", "/api/auth/verify-otp", `{"email":"sherlock@example.com","otp":"`+wrong+`"}`, "", "198.51.100.41")
			mu.Lock()
			counted[rec.Code]++
			mu.Unlock()
		}()
	}
	wg.Wait()
	if counted[400] != 40 {
		t.Fatalf("responses = %v", counted)
	}
	// The session is gone after the cap, so even the right code is refused.
	if rec := e.do("POST", "/api/auth/verify-otp", `{"email":"sherlock@example.com","otp":"`+good+`"}`, "", "198.51.100.41"); rec.Code != 400 {
		t.Fatalf("right code after lockout: %d", rec.Code)
	}
	if len(e.store.byID) != 0 {
		t.Fatal("no user may be created")
	}
}

func TestVerifyIsThrottledPerIP(t *testing.T) {
	e := newEnv(t)
	e.h.Limiter.p.AuthIPMax = 3
	var last int
	for i := 0; i < 5; i++ {
		last = e.do("POST", "/api/auth/verify-otp", `{"email":"a@example.com","otp":"123456"}`, "", "198.51.100.60").Code
	}
	if last != http.StatusTooManyRequests {
		t.Fatalf("got %d, want 429", last)
	}
}

func TestSignupAvailabilityCheckIsThrottledPerIP(t *testing.T) {
	e := newEnv(t)
	e.h.Limiter.p.AuthIPMax = 2
	codes := []int{}
	for _, n := range []string{"a1", "a2", "a3"} {
		codes = append(codes, e.do("POST", "/api/auth/signup", `{"username":"`+n+`user","email":"`+n+`@example.com","password":"{{PW}}"}`, "", "198.51.100.70").Code)
	}
	if codes[2] != http.StatusTooManyRequests {
		t.Fatalf("codes = %v", codes)
	}
}

func TestLoginLocksOutAfterRepeatedFailures(t *testing.T) {
	e := newEnv(t)
	signup(t, e)
	e.h.Limiter.p.AuthIPMax = 1000
	bad := `{"email":"sherlock@example.com","password":"{{PW}}x"}`
	for i := 0; i < e.h.Limiter.p.LoginMaxFailures; i++ {
		if rec := e.do("POST", "/api/auth/login", bad, "", "198.51.100.80"); rec.Code != 401 {
			t.Fatalf("attempt %d: %d", i, rec.Code)
		}
	}
	// Locked now, even for the right password and from another IP (per-email lock).
	rec := e.do("POST", "/api/auth/login", `{"email":"sherlock@example.com","password":"{{PW}}"}`, "", "198.51.100.81")
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("got %d retry-after=%q", rec.Code, rec.Header().Get("Retry-After"))
	}
	// Exponential: one more failure is impossible while locked, so fast-forward the lock.
	e.mr.FastForward(2 * time.Minute)
	if rec := e.do("POST", "/api/auth/login", `{"email":"sherlock@example.com","password":"{{PW}}"}`, "", "198.51.100.81"); rec.Code != 200 {
		t.Fatalf("after lock expired: %d %s", rec.Code, rec.Body)
	}
}

func TestSessionStoresOnlyAnHMACOfTheCode(t *testing.T) {
	e := newEnv(t)
	e.do("POST", "/api/auth/signup", signupBody, "", "198.51.100.90")
	otp := e.pendingOTP(t, "sherlock@example.com")
	raw, _ := e.mr.Get(sessionKey("sherlock@example.com"))
	if strings.Contains(raw, otp) && len(otp) == 6 && strings.Contains(raw, `"otp":`) {
		t.Fatalf("plaintext code in redis: %s", raw)
	}
	if !strings.Contains(raw, e.h.OTP.Hash("sherlock@example.com", otp)) {
		t.Fatalf("hash missing: %s", raw)
	}
}
