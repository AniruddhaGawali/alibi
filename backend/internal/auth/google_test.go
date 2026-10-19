package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"backend/internal/user"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// fakeIdentities keeps provider links next to a fakeStore.
type fakeIdentities struct {
	users *fakeStore
	links map[string]string // provider:subject -> user id
}

func (f *fakeIdentities) GetByIdentity(_ context.Context, p, s string) (*user.User, error) {
	if id, ok := f.links[p+":"+s]; ok {
		return f.users.byID[id], nil
	}
	return nil, user.ErrNotFound
}

func (f *fakeIdentities) LinkIdentity(_ context.Context, userID, p, s, _ string) (bool, error) {
	for k, id := range f.links {
		if id == userID && strings.HasPrefix(k, p+":") {
			return false, nil
		}
	}
	if _, taken := f.links[p+":"+s]; taken {
		return false, nil
	}
	f.links[p+":"+s] = userID
	return true, nil
}

func (f *fakeIdentities) CreateWithIdentity(ctx context.Context, name, email, p, s string, tokens int) (*user.User, error) {
	u, err := f.users.Create(ctx, name, email, user.NoPassword, tokens)
	if err != nil {
		return nil, err
	}
	f.links[p+":"+s] = u.ID
	return u, nil
}

type googleEnv struct {
	*env
	ids      *fakeIdentities
	google   *httptest.Server
	claims   map[string]any // overrides for the ID token the fake Google returns
	nonce    string         // the nonce the start step sent to Google
	verifier string         // the PKCE verifier the callback sent to Google
	h        http.Handler
}

const testClientID = "client-id.apps.googleusercontent.com"

func newGoogleEnv(t *testing.T) *googleEnv {
	t.Helper()
	e := newEnv(t)
	ge := &googleEnv{env: e, claims: map[string]any{}}
	ge.ids = &fakeIdentities{users: e.store, links: map[string]string{}}

	ge.google = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		ge.verifier = r.Form.Get("code_verifier")
		if r.Form.Get("code") != "good-code" || r.Form.Get("client_secret") != "secret" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
			return
		}
		cl := map[string]any{
			"iss": "https://accounts.google.com", "aud": testClientID, "sub": "sub-123",
			"exp": time.Now().Add(time.Hour).Unix(), "email": "new@example.com", "email_verified": true,
			"name": "Ada Lovelace", "nonce": ge.nonce,
		}
		for k, v := range ge.claims {
			cl[k] = v
		}
		payload, _ := json.Marshal(cl)
		tok := "x." + base64.RawURLEncoding.EncodeToString(payload) + ".y"
		_ = json.NewEncoder(w).Encode(map[string]string{"id_token": tok})
	}))
	t.Cleanup(ge.google.Close)

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	_, private, _ := net.ParseCIDR("10.0.0.0/8")
	h := NewHandler(Deps{
		Users: e.store, Tokens: e.tm, Blacklist: e.bl,
		OTP:     NewOTPStore(rdb, 10*time.Minute, 3, []byte("k")),
		Limiter: NewLimiter(rdb, Policy{Cooldown: time.Minute, EmailDailyMax: 3, IPMax: 4, IPWindow: time.Minute, AuthIPMax: 60, AuthIPWindow: time.Minute, SignupIPDailyMax: 5}),
		Mail:    e.mail, Coins: e.coins, TrustedProxies: []*net.IPNet{private},
		InitialTokens: func(context.Context) int { return 10 },
		Google: GoogleConfig{
			ClientID: testClientID, ClientSecret: "secret", FrontendURL: "http://localhost:5173",
			RedirectURL: "http://localhost:8080/api/auth/google/callback", TokenURL: ge.google.URL,
		},
		OAuth: NewOAuthStore(rdb), Identities: ge.ids,
	})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/auth/google/start", h.GoogleStart)
	mux.HandleFunc("GET /api/auth/google/callback", h.GoogleCallback)
	mux.HandleFunc("POST /api/auth/google/exchange", h.GoogleExchange)
	ge.h = mux
	return ge
}

var ticketRe = regexp.MustCompile(`#ticket=([A-Za-z0-9_-]+)$`)

func (ge *googleEnv) get(path string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.RemoteAddr = "203.0.113.9:1234"
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	ge.h.ServeHTTP(rec, req)
	return rec
}

// start runs the first step and returns the state, the browser's cookie and Google's URL.
func (ge *googleEnv) start(t *testing.T) (state string, cookie *http.Cookie, loc *url.URL) {
	t.Helper()
	rec := ge.get("/api/auth/google/start", nil)
	if rec.Code != http.StatusFound {
		t.Fatalf("start: %d", rec.Code)
	}
	loc, _ = url.Parse(rec.Header().Get("Location"))
	for _, c := range rec.Result().Cookies() {
		if c.Name == googleCookie {
			cookie = c
		}
	}
	if cookie == nil || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("state cookie missing or not HttpOnly/Lax: %+v", cookie)
	}
	q := loc.Query()
	ge.nonce = q.Get("nonce")
	return q.Get("state"), cookie, loc
}

func (ge *googleEnv) callback(state string, cookie *http.Cookie, code string) *httptest.ResponseRecorder {
	return ge.get("/api/auth/google/callback?code="+code+"&state="+url.QueryEscape(state), cookie)
}

func (ge *googleEnv) exchange(ticket string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/google/exchange", strings.NewReader(`{"ticket":"`+ticket+`"}`))
	req.RemoteAddr = "203.0.113.9:1234"
	rec := httptest.NewRecorder()
	ge.h.ServeHTTP(rec, req)
	return rec
}

func TestGoogleStartRedirect(t *testing.T) {
	ge := newGoogleEnv(t)
	_, _, loc := ge.start(t)
	q := loc.Query()
	if loc.Host != "accounts.google.com" || q.Get("client_id") != testClientID || q.Get("response_type") != "code" ||
		q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" || q.Get("scope") != "openid email profile" ||
		q.Get("redirect_uri") != "http://localhost:8080/api/auth/google/callback" {
		t.Fatalf("authorize url: %s", loc)
	}
}

func TestGoogleNewAccountFlow(t *testing.T) {
	ge := newGoogleEnv(t)
	state, cookie, loc := ge.start(t)
	rec := ge.callback(state, cookie, "good-code")
	if rec.Code != http.StatusFound {
		t.Fatalf("callback: %d", rec.Code)
	}
	to := rec.Header().Get("Location")
	m := ticketRe.FindStringSubmatch(to)
	if m == nil || !strings.HasPrefix(to, "http://localhost:5173/auth/google/callback#") {
		t.Fatalf("redirect: %s", to)
	}
	if strings.Contains(to, "jwt") {
		t.Fatal("tokens must not be in the URL")
	}
	// PKCE: the verifier sent to Google hashes to the challenge from the start step.
	if codeChallenge(ge.verifier) != loc.Query().Get("code_challenge") {
		t.Fatal("PKCE verifier does not match the challenge")
	}

	ex := ge.exchange(m[1])
	if ex.Code != http.StatusOK {
		t.Fatalf("exchange: %d %s", ex.Code, ex.Body)
	}
	var resp struct {
		Data struct {
			JWT   string `json:"jwt_token"`
			IsNew bool   `json:"is_new"`
			User  struct{ Username, Email string }
		} `json:"data"`
	}
	_ = json.Unmarshal(ex.Body.Bytes(), &resp)
	if resp.Data.JWT == "" || !resp.Data.IsNew || resp.Data.User.Email != "new@example.com" || resp.Data.User.Username != "Ada_Lovelace" {
		t.Fatalf("session: %s", ex.Body)
	}
	if len(ge.store.byID) != 1 || ge.store.byID["user-1"].PasswordHash != user.NoPassword {
		t.Fatal("the account should exist without a password")
	}
	if ge.coins.cached["user-1"] != 10 {
		t.Fatal("starting tokens were not cached")
	}
	// the ticket works once
	if again := ge.exchange(m[1]); again.Code != http.StatusBadRequest {
		t.Fatalf("reused ticket: %d", again.Code)
	}
	// the password login can never match a Google-only account
	if r := ge.env.do("POST", "/api/auth/login", `{"email":"new@example.com","password":"anything1"}`, ""); r.Code != http.StatusUnauthorized {
		t.Fatalf("password login on a google account: %d", r.Code)
	}

	// signing in again finds the same account
	state, cookie, _ = ge.start(t)
	ge.callback(state, cookie, "good-code")
	if len(ge.store.byID) != 1 {
		t.Fatal("a second sign-in created a second account")
	}
}

func TestGoogleLinksExistingEmail(t *testing.T) {
	ge := newGoogleEnv(t)
	ge.store.byID["user-1"] = &user.User{ID: "user-1", Username: "ada", Email: "New@Example.com", PasswordHash: "hash", Role: "user"}
	state, cookie, _ := ge.start(t)
	rec := ge.callback(state, cookie, "good-code")
	if !ticketRe.MatchString(rec.Header().Get("Location")) {
		t.Fatalf("redirect: %s", rec.Header().Get("Location"))
	}
	if len(ge.store.byID) != 1 || ge.ids.links["google:sub-123"] != "user-1" {
		t.Fatal("the Google account should be linked to the existing user")
	}
	// a different Google account with the same email must not take over a linked account
	ge.claims["sub"] = "sub-other"
	state, cookie, _ = ge.start(t)
	rec = ge.callback(state, cookie, "good-code")
	if !strings.Contains(rec.Header().Get("Location"), "google_error=account_conflict") {
		t.Fatalf("expected a conflict, got %s", rec.Header().Get("Location"))
	}
}

func TestGoogleRejections(t *testing.T) {
	cases := []struct {
		name   string
		claims map[string]any
		code   string
		want   string
	}{
		{"unverified email", map[string]any{"email_verified": false}, "good-code", "email_unverified"},
		{"verified as a string", map[string]any{"email_verified": "true"}, "good-code", ""},
		{"wrong audience", map[string]any{"aud": "someone-else"}, "good-code", "failed"},
		{"wrong issuer", map[string]any{"iss": "https://evil.example"}, "good-code", "failed"},
		{"expired", map[string]any{"exp": time.Now().Add(-time.Minute).Unix()}, "good-code", "failed"},
		{"wrong nonce", map[string]any{"nonce": "replayed"}, "good-code", "failed"},
		{"bad code", nil, "bad-code", "failed"},
		{"denied by the user", nil, "", "denied"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ge := newGoogleEnv(t)
			for k, v := range c.claims {
				ge.claims[k] = v
			}
			state, cookie, _ := ge.start(t)
			loc := ge.callback(state, cookie, c.code).Header().Get("Location")
			if c.want == "" {
				if !ticketRe.MatchString(loc) {
					t.Fatalf("expected success, got %s", loc)
				}
				return
			}
			if !strings.Contains(loc, "/login?google_error="+c.want) {
				t.Fatalf("got %s, want error %s", loc, c.want)
			}
			if len(ge.store.byID) != 0 {
				t.Fatal("no account may be created on a rejected sign-in")
			}
		})
	}
}

func TestGoogleStateChecks(t *testing.T) {
	ge := newGoogleEnv(t)
	state, cookie, _ := ge.start(t)
	// no cookie: someone else's link opened in this browser
	if loc := ge.callback(state, nil, "good-code").Header().Get("Location"); !strings.Contains(loc, "google_error=invalid_state") {
		t.Fatalf("no cookie: %s", loc)
	}
	// a cookie for another state
	other := &http.Cookie{Name: googleCookie, Value: strings.Repeat("a", 43)}
	if loc := ge.callback(state, other, "good-code").Header().Get("Location"); !strings.Contains(loc, "google_error=invalid_state") {
		t.Fatalf("wrong cookie: %s", loc)
	}
	// the state is single use
	if loc := ge.callback(state, cookie, "good-code").Header().Get("Location"); !ticketRe.MatchString(loc) {
		t.Fatalf("first use: %s", loc)
	}
	if loc := ge.callback(state, cookie, "good-code").Header().Get("Location"); !strings.Contains(loc, "google_error=invalid_state") {
		t.Fatalf("replay: %s", loc)
	}
}

func TestGoogleDisabled(t *testing.T) {
	ge := newGoogleEnv(t)
	hh := NewHandler(Deps{Google: GoogleConfig{FrontendURL: "http://localhost:5173"}})
	rec := httptest.NewRecorder()
	hh.GoogleStart(rec, httptest.NewRequest(http.MethodGet, "/api/auth/google/start", nil))
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "google_error=google_disabled") {
		t.Fatalf("disabled: %s", loc)
	}
	_ = ge
}

func TestSuggestUsername(t *testing.T) {
	for _, c := range []struct{ name, email, want string }{
		{"Ada Lovelace", "a@x.com", "Ada_Lovelace"},
		{"", "sherlock.holmes@x.com", "sherlock_holme"},
		{"Zoë", "zo@x.com", "detective"},
		{"A very long display name here", "a@x.com", "A_very_long_di"},
		{"李雷", "li.lei@x.com", "li_lei"},
	} {
		got := suggestUsername(c.name, c.email)
		if got != c.want || !usernameOK(got) {
			t.Errorf("suggestUsername(%q, %q) = %q, want %q", c.name, c.email, got, c.want)
		}
	}
	if s := withSuffix("A_very_long_dis"); !usernameOK(s) {
		t.Errorf("suffix result %q is not a valid username", s)
	}
}

var usernameRe = regexp.MustCompile(`^[A-Za-z0-9_]{3,20}$`)

func usernameOK(s string) bool { return usernameRe.MatchString(s) }
