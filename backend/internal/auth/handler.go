package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	emailqueue "backend/internal/email"
	"backend/internal/user"
	"backend/pkg/utils"
	"golang.org/x/crypto/bcrypt"
)

// CoinCache primes the cached token balance (implemented by wallet.Service).
type CoinCache interface {
	Cache(ctx context.Context, userID string, balance int64)
}

// Deps are the collaborators of Handler.
type Deps struct {
	Users          user.Store
	Tokens         *TokenManager
	Blacklist      *Blacklist
	OTP            *OTPStore
	Limiter        *Limiter
	Mail           emailqueue.Publisher // outbound email queue
	Coins          CoinCache
	TrustedProxies []*net.IPNet
	InitialTokens  func(context.Context) int // question tokens a new account starts with (the current daily allowance)
	HashWorkers    int                       // max concurrent bcrypt operations; 0 = number of CPUs

	// Google sign-in (all optional: Google.Enabled() false switches it off).
	Google     GoogleConfig
	OAuth      *OAuthStore
	Identities IdentityStore
	HTTP       *http.Client // for the Google token call; nil = a client with a 10 s timeout
}

type Handler struct {
	Deps
	hashSem chan struct{} // bounds concurrent bcrypt work so a flood queues instead of starving the API
}

// hashWait is how long a request waits for a free bcrypt slot before a 503.
const hashWait = 5 * time.Second

func NewHandler(d Deps) *Handler {
	n := d.HashWorkers
	if n < 1 {
		n = runtime.NumCPU()
	}
	return &Handler{Deps: d, hashSem: make(chan struct{}, n)}
}

// acquireHash reserves a bcrypt slot; call the returned func when done. When the
// API is saturated it writes a 503 and returns ok=false.
func (h *Handler) acquireHash(w http.ResponseWriter, r *http.Request) (release func(), ok bool) {
	t := time.NewTimer(hashWait)
	defer t.Stop()
	select {
	case h.hashSem <- struct{}{}:
		return func() { <-h.hashSem }, true
	case <-r.Context().Done():
		return nil, false
	case <-t.C:
		utils.LogWarnCtx(r.Context(), "bcrypt saturated; rejecting request")
		w.Header().Set("Retry-After", "5")
		utils.ErrorResponse(w, http.StatusServiceUnavailable, "server busy, try again shortly", nil)
		return nil, false
	}
}

// emailID is a short stable digest of an email for logs and rate-limit keys, so
// the address itself is not written to them.
func emailID(addr string) string {
	sum := sha256.Sum256([]byte(addr))
	return hex.EncodeToString(sum[:8])
}

type signupReq struct {
	Username string `json:"username" validate:"required,username"`
	Email    string `json:"email" validate:"required,email,max=254"`
	Password string `json:"password" validate:"required,password"`
}

type resendReq struct {
	Email string `json:"email" validate:"required,email,max=254"`
}

type verifyReq struct {
	Email string `json:"email" validate:"required,email,max=254"`
	OTP   string `json:"otp" validate:"required,len=6,numeric"`
}

type loginReq struct {
	Email    string `json:"email" validate:"required,email,max=254"`
	Password string `json:"password" validate:"required,max=72"`
}

type refreshReq struct {
	RefreshToken string `json:"refresh_token" validate:"required,max=2048"`
}

type logoutReq struct {
	RefreshToken string `json:"refresh_token" validate:"required,max=2048"`
}

type userInfo struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"` // "admin" goes to the admin panel after login
}

type tokenResp struct {
	JWTToken     string    `json:"jwt_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresIn    int       `json:"expires_in"` // access token lifetime in seconds
	User         *userInfo `json:"user,omitempty"`
}

type otpSentResp struct {
	Email          string `json:"email"`
	OTPExpiresIn   int    `json:"otp_expires_in"`   // seconds the code stays valid
	ResendAfterSec int    `json:"resend_after_sec"` // seconds before another code can be requested
}

// dummyHash lets Login spend the same bcrypt time for unknown emails, so
// response time does not reveal which emails are registered.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("alibi-dummy-password"), bcrypt.DefaultCost)

func normalizeEmail(e string) string { return strings.ToLower(strings.TrimSpace(e)) }

// issuePair signs a fresh access/refresh pair.
func (h *Handler) issuePair(userID, email, role string) (*tokenResp, error) {
	access, claims, err := h.Tokens.Issue(userID, email, role, Access)
	if err != nil {
		return nil, err
	}
	refresh, _, err := h.Tokens.Issue(userID, email, role, Refresh)
	if err != nil {
		return nil, err
	}
	return &tokenResp{
		JWTToken:     access,
		RefreshToken: refresh,
		ExpiresIn:    int(claims.Remaining(h.Tokens.now()).Seconds()),
	}, nil
}

// respondTokens issues a pair and writes it, or a generic 500.
func (h *Handler) respondTokens(w http.ResponseWriter, r *http.Request, status int, u *user.User) {
	resp, err := h.issuePair(u.ID, u.Email, u.Role)
	if err != nil {
		utils.LogErrorCtx(r.Context(), "issue tokens failed", "user_id", u.ID, "err", err)
		utils.ErrorResponse(w, http.StatusInternalServerError, "could not create session", nil)
		return
	}
	resp.User = &userInfo{ID: u.ID, Username: u.Username, Email: u.Email, Role: u.Role}
	utils.SuccessResponse(w, status, resp)
}

// tooMany writes a 429 with Retry-After.
func tooMany(w http.ResponseWriter, wait time.Duration) {
	secs := int(wait.Seconds())
	w.Header().Set("Retry-After", strconv.Itoa(secs))
	utils.ErrorResponse(w, http.StatusTooManyRequests, "too many requests, try again later",
		map[string]int{"retry_after_seconds": secs})
}

// gateIP charges the shared per-IP auth budget before any database or bcrypt
// work. When it refuses it has already written the 429 and returns false.
func (h *Handler) gateIP(w http.ResponseWriter, r *http.Request, scope, ip string) bool {
	wait, err := h.Limiter.HitIP(r.Context(), scope, ip)
	if err != nil {
		utils.LogErrorCtx(r.Context(), "ip rate limit check failed", "scope", scope, "err", err)
		utils.ErrorResponse(w, http.StatusServiceUnavailable, "service unavailable", nil)
		return false
	}
	if wait > 0 {
		utils.LogInfoCtx(r.Context(), "ip rate limited", "scope", scope, "ip", ip, "retry_after_sec", int(wait.Seconds()))
		tooMany(w, wait)
		return false
	}
	return true
}

// takeCooldown applies the dual email + IP cooldown. When it refuses it has
// already written the 429 (with Retry-After) and returns false.
func (h *Handler) takeCooldown(w http.ResponseWriter, r *http.Request, email, ip string) bool {
	wait, err := h.Limiter.Take(r.Context(), email, ip)
	if err != nil {
		utils.LogErrorCtx(r.Context(), "otp rate limit check failed", "err", err)
		utils.ErrorResponse(w, http.StatusServiceUnavailable, "Sign-up is temporarily unavailable. Please try again in a few minutes.", nil)
		return false
	}
	if wait > 0 {
		utils.LogInfoCtx(r.Context(), "otp request rate limited", "ip", ip, "retry_after_sec", int(wait.Seconds()))
		tooMany(w, wait)
		return false
	}
	return true
}

// queueOTP publishes the email job. On failure nothing was sent, so the
// caller undoes its state.
func (h *Handler) queueOTP(r *http.Request, email, otp string) error {
	return emailqueue.Publish(r.Context(), h.Mail, emailqueue.Message{
		To: email, OTP: otp, Type: emailqueue.TypeSignup,
		ExpiresAt: time.Now().Add(h.OTP.TTL()).Unix(),
	})
}

func (h *Handler) otpSent(w http.ResponseWriter, email string) {
	utils.SuccessResponse(w, http.StatusOK, otpSentResp{
		Email:          email,
		OTPExpiresIn:   int(h.OTP.TTL().Seconds()),
		ResendAfterSec: int(h.Limiter.Cooldown().Seconds()),
	})
}

// Signup handles POST /api/auth/signup. It stores the pending registration in
// Redis and queues the OTP email; no user row exists until the code is
// verified.
func (h *Handler) Signup(w http.ResponseWriter, r *http.Request) {
	var req signupReq
	if !utils.BindAndValidate(w, r, &req) {
		return
	}
	ctx := r.Context()
	addr := normalizeEmail(req.Email)

	// Charge the per-IP budget first: the availability check below hits Postgres
	// and reveals whether an email / username exists.
	ip := utils.ClientIP(r, h.TrustedProxies)
	if !h.gateIP(w, r, "signup", ip) {
		return
	}
	// Telling the user that an email or username is taken is a deliberate
	// product trade-off (they must be able to pick another); the IP budget above
	// is what limits enumeration.
	emailTaken, nameTaken, err := h.Users.Availability(ctx, addr, req.Username)
	if err != nil {
		utils.LogErrorCtx(ctx, "check availability failed", "err", err)
		utils.ErrorResponse(w, http.StatusInternalServerError, "could not start signup", nil)
		return
	}
	if emailTaken || nameTaken {
		details := map[string]string{}
		if emailTaken {
			details["email"] = "is already registered"
		}
		if nameTaken {
			details["username"] = "is already taken"
		}
		utils.ErrorResponse(w, http.StatusConflict, "account already exists", details)
		return
	}

	if !h.takeCooldown(w, r, addr, ip) {
		return
	}
	// From here on, any failure must give the cooldown back: the user got nothing.
	fail := func(status int, msg string) {
		h.Limiter.Release(context.WithoutCancel(ctx), addr, ip)
		utils.ErrorResponse(w, status, msg, nil)
	}

	releaseHash, ok := h.acquireHash(w, r)
	if !ok {
		h.Limiter.Release(context.WithoutCancel(ctx), addr, ip)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	releaseHash()
	if err != nil {
		utils.LogErrorCtx(ctx, "hash password failed", "err", err)
		fail(http.StatusInternalServerError, "could not start signup")
		return
	}
	otp, err := GenerateOTP()
	if err != nil {
		utils.LogErrorCtx(ctx, "generate otp failed", "err", err)
		fail(http.StatusInternalServerError, "could not start signup")
		return
	}
	if err := h.OTP.Save(ctx, addr, Session{OTPHash: h.OTP.Hash(addr, otp), Username: req.Username, HashedPassword: string(hash)}); err != nil {
		utils.LogErrorCtx(ctx, "save signup session failed", "err", err)
		fail(http.StatusInternalServerError, "could not start signup")
		return
	}
	if err := h.queueOTP(r, addr, otp); err != nil {
		utils.LogErrorCtx(ctx, "queue otp email failed", "err", err)
		_ = h.OTP.Delete(context.WithoutCancel(ctx), addr)
		fail(http.StatusServiceUnavailable, "could not send verification email, try again")
		return
	}
	utils.LogInfoCtx(ctx, "signup otp queued", "ip", ip)
	h.otpSent(w, addr)
}

// ResendOTP handles POST /api/auth/resend-otp: a fresh code for an existing
// pending signup, which also restarts the 10-minute window.
func (h *Handler) ResendOTP(w http.ResponseWriter, r *http.Request) {
	var req resendReq
	if !utils.BindAndValidate(w, r, &req) {
		return
	}
	ctx := r.Context()
	addr := normalizeEmail(req.Email)
	ip := utils.ClientIP(r, h.TrustedProxies)

	if !h.gateIP(w, r, "resend", ip) {
		return
	}
	if !h.takeCooldown(w, r, addr, ip) {
		return
	}
	// Nothing is sent unless every step below succeeds, so every early exit
	// gives the cooldown back (otherwise a stray request would lock the email
	// or the IP out of signing up for 5 minutes).
	fail := func(status int, msg string) {
		h.Limiter.Release(context.WithoutCancel(ctx), addr, ip)
		utils.ErrorResponse(w, status, msg, nil)
	}
	sess, err := h.OTP.Get(ctx, addr)
	if errors.Is(err, ErrNoSession) {
		// Answer exactly as for a real resend (and keep the cooldown spent), so
		// this endpoint cannot be used to find out which signups are pending.
		utils.LogInfoCtx(ctx, "resend otp for unknown signup", "ip", ip)
		h.otpSent(w, addr)
		return
	}
	if err != nil {
		utils.LogErrorCtx(ctx, "load signup session failed", "err", err)
		fail(http.StatusInternalServerError, "could not resend code")
		return
	}

	otp, err := GenerateOTP()
	if err != nil {
		utils.LogErrorCtx(ctx, "generate otp failed", "err", err)
		fail(http.StatusInternalServerError, "could not resend code")
		return
	}
	sess.OTPHash = h.OTP.Hash(addr, otp)
	if err := h.OTP.Save(ctx, addr, *sess); err != nil {
		utils.LogErrorCtx(ctx, "save signup session failed", "err", err)
		fail(http.StatusInternalServerError, "could not resend code")
		return
	}
	if err := h.queueOTP(r, addr, otp); err != nil {
		utils.LogErrorCtx(ctx, "queue otp email failed", "err", err)
		fail(http.StatusServiceUnavailable, "could not send verification email, try again")
		return
	}
	utils.LogInfoCtx(ctx, "resend otp queued", "ip", ip)
	h.otpSent(w, addr)
}

// VerifyOTP handles POST /api/auth/verify-otp. A correct code creates the
// user (with the starting token balance) and signs them in.
func (h *Handler) VerifyOTP(w http.ResponseWriter, r *http.Request) {
	var req verifyReq
	if !utils.BindAndValidate(w, r, &req) {
		return
	}
	ctx := r.Context()
	addr := normalizeEmail(req.Email)
	const invalid = "OTP expired or invalid"

	// Per-IP budget first: one source cannot burn many people's attempts or
	// spray guesses across many pending signups.
	if !h.gateIP(w, r, "verify", utils.ClientIP(r, h.TrustedProxies)) {
		return
	}
	// Verify charges the attempt before comparing, atomically in Redis.
	res, sess, err := h.OTP.Verify(ctx, addr, req.OTP)
	if err != nil {
		utils.LogErrorCtx(ctx, "verify otp failed", "err", err)
		utils.ErrorResponse(w, http.StatusInternalServerError, "could not verify code", nil)
		return
	}
	switch res {
	case VerifyOK:
	case VerifyLocked:
		utils.LogInfoCtx(ctx, "wrong otp", "session_discarded", true)
		utils.ErrorResponse(w, http.StatusBadRequest, invalid, nil)
		return
	case VerifyWrong:
		utils.LogInfoCtx(ctx, "wrong otp", "session_discarded", false)
		utils.ErrorResponse(w, http.StatusBadRequest, invalid, nil)
		return
	default:
		utils.ErrorResponse(w, http.StatusBadRequest, invalid, nil)
		return
	}

	// Insert first and delete the session after: a transient database error
	// leaves the session intact so the user can retry, and the unique indexes
	// make a double submit harmless.
	// A new account comes with a day's question tokens, so accounts per address are
	// capped. The signup session stays, so the user can finish once the wait is over.
	ip := utils.ClientIP(r, h.TrustedProxies)
	wait, err := h.Limiter.HitSignup(ctx, ip)
	if err != nil {
		utils.LogErrorCtx(ctx, "signup rate limit check failed", "err", err)
		utils.ErrorResponse(w, http.StatusServiceUnavailable, "service unavailable", nil)
		return
	}
	if wait > 0 {
		utils.LogWarnCtx(ctx, "account creation rate limited", "ip", ip, "retry_after_sec", int(wait.Seconds()))
		tooMany(w, wait)
		return
	}
	initial := h.InitialTokens(ctx)
	u, err := h.Users.Create(ctx, sess.Username, addr, sess.HashedPassword, initial)
	switch {
	case errors.Is(err, user.ErrEmailTaken), errors.Is(err, user.ErrUsernameTaken):
		// Someone else registered it while this signup was pending.
		_ = h.OTP.Delete(context.WithoutCancel(ctx), addr)
		details := map[string]string{"username": "is already taken"}
		if errors.Is(err, user.ErrEmailTaken) {
			details = map[string]string{"email": "is already registered"}
		}
		utils.ErrorResponse(w, http.StatusConflict, "account already exists", details)
		return
	case err != nil:
		utils.LogErrorCtx(ctx, "create user failed", "err", err)
		utils.ErrorResponse(w, http.StatusInternalServerError, "could not create account", nil)
		return
	}
	if err := h.OTP.Delete(context.WithoutCancel(ctx), addr); err != nil {
		utils.LogErrorCtx(ctx, "delete signup session failed", "user_id", u.ID, "err", err)
	}
	h.Coins.Cache(ctx, u.ID, int64(initial))

	utils.LogInfoCtx(ctx, "user signed up", "user_id", u.ID, "initial_tokens", initial)
	h.respondTokens(w, r, http.StatusCreated, u)
}

// Login handles POST /api/auth/login.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if !utils.BindAndValidate(w, r, &req) {
		return
	}

	ctx := r.Context()
	addr := normalizeEmail(req.Email)
	eid := emailID(addr)
	ip := utils.ClientIP(r, h.TrustedProxies)

	if !h.gateIP(w, r, "login", ip) {
		return
	}
	// Refuse locked-out emails / IPs before spending any bcrypt time.
	wait, err := h.Limiter.LoginLocked(ctx, eid, ip)
	if err != nil {
		utils.LogErrorCtx(ctx, "login lockout check failed", "err", err)
		utils.ErrorResponse(w, http.StatusServiceUnavailable, "service unavailable", nil)
		return
	}
	if wait > 0 {
		utils.LogWarnCtx(ctx, "login_blocked", "email_id", eid, "ip", ip, "retry_after_sec", int(wait.Seconds()))
		tooMany(w, wait)
		return
	}

	u, err := h.Users.GetByEmail(ctx, addr)
	hash := dummyHash
	if err == nil {
		hash = []byte(u.PasswordHash)
	} else if !errors.Is(err, user.ErrNotFound) {
		utils.LogErrorCtx(ctx, "load user failed", "err", err)
		utils.ErrorResponse(w, http.StatusInternalServerError, "could not log in", nil)
		return
	}
	releaseHash, ok := h.acquireHash(w, r)
	if !ok {
		return
	}
	// Always compare, so unknown emails cost the same time as wrong passwords.
	cmpErr := bcrypt.CompareHashAndPassword(hash, []byte(req.Password))
	releaseHash()
	if cmpErr != nil || u == nil {
		if err := h.Limiter.LoginFailed(context.WithoutCancel(ctx), eid, ip); err != nil {
			utils.LogErrorCtx(ctx, "record login failure failed", "err", err)
		}
		utils.LogWarnCtx(ctx, "login_failed", "email_id", eid, "ip", ip)
		utils.ErrorResponse(w, http.StatusUnauthorized, "invalid email or password", nil)
		return
	}
	h.Limiter.LoginSucceeded(context.WithoutCancel(ctx), eid)
	utils.LogInfoCtx(r.Context(), "user logged in", "user_id", u.ID)
	h.respondTokens(w, r, http.StatusOK, u)
}

// Refresh handles POST /api/auth/refresh. Each refresh token works once: it is
// blacklisted as it is exchanged for a new pair.
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshReq
	if !utils.BindAndValidate(w, r, &req) {
		return
	}
	if !h.gateIP(w, r, "refresh", utils.ClientIP(r, h.TrustedProxies)) {
		return
	}
	claims, err := h.Tokens.Parse(req.RefreshToken, Refresh)
	if err != nil {
		utils.ErrorResponse(w, http.StatusUnauthorized, "invalid refresh token", nil)
		return
	}

	u, err := h.Users.GetByID(r.Context(), claims.Subject)
	if errors.Is(err, user.ErrNotFound) {
		utils.ErrorResponse(w, http.StatusUnauthorized, "invalid refresh token", nil)
		return
	}
	if err != nil {
		utils.LogErrorCtx(r.Context(), "load user failed", "err", err)
		utils.ErrorResponse(w, http.StatusInternalServerError, "could not refresh session", nil)
		return
	}

	claimed, err := h.Blacklist.Claim(r.Context(), claims.ID, claims.Remaining(h.Tokens.now()))
	if err != nil {
		utils.LogErrorCtx(r.Context(), "blacklist refresh token failed", "user_id", u.ID, "err", err)
		utils.ErrorResponse(w, http.StatusServiceUnavailable, "service unavailable", nil)
		return
	}
	if !claimed {
		utils.LogInfoCtx(r.Context(), "refresh token reuse rejected", "user_id", u.ID)
		utils.ErrorResponse(w, http.StatusUnauthorized, "invalid refresh token", nil)
		return
	}
	utils.LogInfoCtx(r.Context(), "token refreshed", "user_id", u.ID)
	h.respondTokens(w, r, http.StatusOK, u)
}

// Logout handles POST /api/auth/logout (protected by Middleware). The access token comes
// from the Authorization header and the user's refresh_token from the body;
// both are blacklisted for their remaining lifetime so neither can be used
// again.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFrom(r.Context())
	if !ok {
		utils.ErrorResponse(w, http.StatusUnauthorized, "unauthorized", nil)
		return
	}
	var req logoutReq
	if !utils.BindAndValidate(w, r, &req) {
		return
	}
	// The refresh token must be valid and belong to the same user, so one user
	// cannot revoke another's session.
	rc, err := h.Tokens.Parse(req.RefreshToken, Refresh)
	if err != nil || rc.Subject != claims.Subject {
		utils.LogInfoCtx(r.Context(), "logout rejected: bad refresh token", "user_id", claims.Subject)
		utils.ErrorResponse(w, http.StatusUnauthorized, "invalid refresh token", nil)
		return
	}

	if err := h.Blacklist.RevokeAll(r.Context(), h.Tokens.now(), claims, rc); err != nil {
		utils.LogErrorCtx(r.Context(), "blacklist tokens failed", "user_id", claims.Subject, "err", err)
		if !h.Blacklist.FailOpen() {
			utils.ErrorResponse(w, http.StatusInternalServerError, "could not log out", nil)
			return
		}
		// Redis is down, but the tokens are revoked in this process's memory and the client
		// drops them: the player is logged out here, so say so.
	}
	utils.LogInfoCtx(r.Context(), "user logged out", "user_id", claims.Subject)
	utils.SuccessResponse(w, http.StatusOK, map[string]string{"message": "logged out"})
}
