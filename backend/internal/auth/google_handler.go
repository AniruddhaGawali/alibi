package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"backend/internal/user"
	"backend/pkg/utils"
)

// IdentityStore links sign-in provider accounts to users (user.PGStore implements it).
type IdentityStore interface {
	GetByIdentity(ctx context.Context, provider, subject string) (*user.User, error)
	LinkIdentity(ctx context.Context, userID, provider, subject, email string) (bool, error)
	CreateWithIdentity(ctx context.Context, username, email, provider, subject string, tokens int) (*user.User, error)
}

const (
	googleStateTTL  = 10 * time.Minute
	googleTicketTTL = time.Minute
	googleCookie    = "alibi_oauth_state"
	googleHTTPLimit = 10 * time.Second
	stateKeyPrefix  = "oauth:google:state:"
	ticketKeyPrefix = "oauth:google:ticket:"
)

// googleState is what the server remembers about a sign-in between the redirect to
// Google and the callback.
type googleState struct {
	Verifier string `json:"verifier"` // PKCE code verifier
	Nonce    string `json:"nonce"`
}

// googleSession is what the frontend gets for its one-time ticket.
type googleSession struct {
	tokenResp
	IsNew bool `json:"is_new"` // a brand-new account: send the player to pick an avatar
}

type exchangeReq struct {
	Ticket string `json:"ticket" validate:"required,max=100"`
}

// googleFail sends the browser back to the login page with a short error code.
func (h *Handler) googleFail(w http.ResponseWriter, r *http.Request, code string) {
	http.SetCookie(w, h.stateCookie("", -1))
	to := strings.TrimRight(h.Google.FrontendURL, "/") + "/login?google_error=" + url.QueryEscape(code)
	http.Redirect(w, r, to, http.StatusFound)
}

// stateCookie binds the sign-in to the browser that started it (so a link crafted by
// someone else cannot log a victim into the attacker's account).
func (h *Handler) stateCookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name: googleCookie, Value: value, Path: "/api/auth/google",
		MaxAge: maxAge, HttpOnly: true, Secure: h.Google.secureCookies(), SameSite: http.SameSiteLaxMode,
	}
}

func (h *Handler) googleReady() bool {
	return h.Google.Enabled() && h.OAuth != nil && h.Identities != nil
}

// GoogleStart handles GET /api/auth/google/start: it remembers a fresh state, nonce and
// PKCE verifier and redirects the browser to Google.
func (h *Handler) GoogleStart(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !h.googleReady() {
		h.googleFail(w, r, "google_disabled")
		return
	}
	ip := utils.ClientIP(r, h.TrustedProxies)
	if wait, err := h.Limiter.HitIP(ctx, "google", ip); err != nil || wait > 0 {
		if err != nil {
			utils.LogErrorCtx(ctx, "ip rate limit check failed", "scope", "google", "err", err)
		}
		h.googleFail(w, r, "rate_limited")
		return
	}
	state, err1 := RandomToken(32)
	nonce, err2 := RandomToken(32)
	verifier, err3 := RandomToken(48) // 64 chars, inside PKCE's 43-128
	if err := errors.Join(err1, err2, err3); err != nil {
		utils.LogErrorCtx(ctx, "google state generation failed", "err", err)
		h.googleFail(w, r, "failed")
		return
	}
	if err := h.OAuth.Put(ctx, stateKeyPrefix+state, googleState{Verifier: verifier, Nonce: nonce}, googleStateTTL); err != nil {
		utils.LogErrorCtx(ctx, "store google state failed", "err", err)
		h.googleFail(w, r, "failed")
		return
	}
	http.SetCookie(w, h.stateCookie(state, int(googleStateTTL.Seconds())))
	utils.LogInfoCtx(ctx, "google sign-in started")
	http.Redirect(w, r, h.Google.authorizeURL(state, nonce, verifier), http.StatusFound)
}

// GoogleCallback handles GET /api/auth/google/callback. After Google has authenticated
// the person it finds or creates their account, then redirects to the frontend with a
// one-time ticket (in the URL fragment, which is never sent to a server or logged) that
// GoogleExchange swaps for the session. Tokens never appear in a URL.
func (h *Handler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !h.googleReady() {
		h.googleFail(w, r, "google_disabled")
		return
	}
	q := r.URL.Query()
	state, code := q.Get("state"), q.Get("code")
	if len(state) < 20 || len(state) > 100 {
		h.googleFail(w, r, "invalid_state")
		return
	}
	// The state in the URL must be the one this browser was given.
	c, err := r.Cookie(googleCookie)
	if err != nil || subtle.ConstantTimeCompare([]byte(c.Value), []byte(state)) != 1 {
		utils.LogWarnCtx(ctx, "google callback state mismatch")
		h.googleFail(w, r, "invalid_state")
		return
	}
	var st googleState
	if err := h.OAuth.Take(ctx, stateKeyPrefix+state, &st); err != nil { // single use
		utils.LogWarnCtx(ctx, "google callback unknown state", "err", err)
		h.googleFail(w, r, "invalid_state")
		return
	}
	if q.Get("error") != "" || code == "" || len(code) > 2048 {
		utils.LogInfoCtx(ctx, "google sign-in not completed", "google_error", truncate(q.Get("error"), 40))
		h.googleFail(w, r, "denied")
		return
	}

	hc := h.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: googleHTTPLimit}
	}
	claims, err := h.Google.exchange(ctx, hc, code, st.Verifier, st.Nonce, time.Now())
	if err != nil {
		utils.LogWarnCtx(ctx, "google code exchange failed", "err", err)
		h.googleFail(w, r, "failed")
		return
	}
	email := normalizeEmail(claims.Email)
	if !claims.verified() || email == "" || len(email) > 254 {
		utils.LogInfoCtx(ctx, "google account has no verified email")
		h.googleFail(w, r, "email_unverified")
		return
	}

	u, isNew, code2 := h.resolveGoogleUser(r, claims, email)
	if code2 != "" {
		h.googleFail(w, r, code2)
		return
	}

	pair, err := h.issuePair(u.ID, u.Email, u.Role)
	if err != nil {
		utils.LogErrorCtx(ctx, "issue tokens failed", "user_id", u.ID, "err", err)
		h.googleFail(w, r, "failed")
		return
	}
	pair.User = &userInfo{ID: u.ID, Username: u.Username, Email: u.Email, Role: u.Role}
	ticket, err := RandomToken(32)
	if err == nil {
		err = h.OAuth.Put(ctx, ticketKeyPrefix+ticket, googleSession{tokenResp: *pair, IsNew: isNew}, googleTicketTTL)
	}
	if err != nil {
		utils.LogErrorCtx(ctx, "store google ticket failed", "user_id", u.ID, "err", err)
		h.googleFail(w, r, "failed")
		return
	}
	http.SetCookie(w, h.stateCookie("", -1))
	utils.LogInfoCtx(ctx, "google sign-in completed", "user_id", u.ID, "new_account", isNew)
	http.Redirect(w, r, strings.TrimRight(h.Google.FrontendURL, "/")+"/auth/google/callback#ticket="+ticket, http.StatusFound)
}

// resolveGoogleUser finds the player for a verified Google account: by the Google
// subject, else by the same verified email (which links the two), else a new account.
// On failure it returns the error code for the login page.
func (h *Handler) resolveGoogleUser(r *http.Request, cl *googleClaims, email string) (u *user.User, isNew bool, failCode string) {
	ctx := r.Context()
	u, err := h.Identities.GetByIdentity(ctx, user.ProviderGoogle, cl.Subject)
	if err == nil {
		return u, false, ""
	}
	if !errors.Is(err, user.ErrNotFound) {
		utils.LogErrorCtx(ctx, "load google identity failed", "err", err)
		return nil, false, "failed"
	}

	// An existing account with this email: Google has verified the address and so did our
	// OTP signup, so the owner is the same person. Link it.
	u, err = h.Users.GetByEmail(ctx, email)
	switch {
	case err == nil:
		if _, err := h.Identities.LinkIdentity(ctx, u.ID, user.ProviderGoogle, cl.Subject, email); err != nil {
			utils.LogErrorCtx(ctx, "link google identity failed", "user_id", u.ID, "err", err)
			return nil, false, "failed"
		}
		// If the account already had another Google account linked, the link is a no-op
		// and this Google account must not get in.
		if linked, err := h.Identities.GetByIdentity(ctx, user.ProviderGoogle, cl.Subject); err != nil || linked.ID != u.ID {
			utils.LogWarnCtx(ctx, "google account does not match the linked one", "user_id", u.ID)
			return nil, false, "account_conflict"
		}
		utils.LogInfoCtx(ctx, "google account linked", "user_id", u.ID)
		return u, false, ""
	case !errors.Is(err, user.ErrNotFound):
		utils.LogErrorCtx(ctx, "load user failed", "err", err)
		return nil, false, "failed"
	}

	// New account. Count it against the per-address signup cap, like OTP signups.
	ip := utils.ClientIP(r, h.TrustedProxies)
	if wait, err := h.Limiter.HitSignup(ctx, ip); err != nil || wait > 0 {
		if err != nil {
			utils.LogErrorCtx(ctx, "signup rate limit check failed", "err", err)
		}
		return nil, false, "rate_limited"
	}
	initial := h.InitialTokens(ctx)
	base := suggestUsername(cl.Name, email)
	for attempt := range 6 {
		name := base
		if attempt > 0 {
			name = withSuffix(base)
		}
		u, err = h.Identities.CreateWithIdentity(ctx, name, email, user.ProviderGoogle, cl.Subject, initial)
		switch {
		case err == nil:
			h.Coins.Cache(ctx, u.ID, int64(initial))
			return u, true, ""
		case errors.Is(err, user.ErrUsernameTaken):
			continue
		case errors.Is(err, user.ErrEmailTaken), errors.Is(err, user.ErrIdentityTaken):
			// Raced with another sign-in for the same person: start over, they exist now.
			if u, gErr := h.Identities.GetByIdentity(ctx, user.ProviderGoogle, cl.Subject); gErr == nil {
				return u, false, ""
			}
			return nil, false, "failed"
		default:
			utils.LogErrorCtx(ctx, "create google user failed", "err", err)
			return nil, false, "failed"
		}
	}
	return nil, false, "failed"
}

// GoogleExchange handles POST /api/auth/google/exchange: the frontend trades the
// one-time ticket from the callback redirect for the session.
func (h *Handler) GoogleExchange(w http.ResponseWriter, r *http.Request) {
	var req exchangeReq
	if !utils.BindAndValidate(w, r, &req) {
		return
	}
	if h.OAuth == nil {
		utils.ErrorResponse(w, http.StatusServiceUnavailable, "google sign-in is not available", nil)
		return
	}
	if !h.gateIP(w, r, "google-exchange", utils.ClientIP(r, h.TrustedProxies)) {
		return
	}
	var s googleSession
	if err := h.OAuth.Take(r.Context(), ticketKeyPrefix+req.Ticket, &s); err != nil {
		if !errors.Is(err, ErrNoState) {
			utils.LogErrorCtx(r.Context(), "google ticket read failed", "err", err)
			utils.ErrorResponse(w, http.StatusServiceUnavailable, "service unavailable", nil)
			return
		}
		utils.ErrorResponse(w, http.StatusBadRequest, "sign-in link expired or already used", nil)
		return
	}
	utils.SuccessResponse(w, http.StatusOK, s)
}

// suggestUsername turns a display name (or the email's local part) into a valid
// username: 3-20 letters, digits and underscores.
func suggestUsername(name, email string) string {
	clean := func(s string) string {
		var b strings.Builder
		for _, r := range s {
			switch {
			case r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)):
				b.WriteRune(r)
			case r == ' ' || r == '_' || r == '.' || r == '-':
				b.WriteRune('_')
			}
		}
		return strings.Trim(b.String(), "_")
	}
	base := clean(name)
	if len(base) < 3 {
		local, _, _ := strings.Cut(email, "@")
		base = clean(local)
	}
	if len(base) < 3 {
		base = "detective"
	}
	return truncate(base, 14)
}

// withSuffix appends a random 4-digit number (the result stays within 20 characters).
func withSuffix(base string) string {
	n, err := rand.Int(rand.Reader, big.NewInt(10000))
	if err != nil {
		n = big.NewInt(time.Now().UnixNano() % 10000)
	}
	return truncate(base, 15) + "_" + pad4(int(n.Int64()))
}

func pad4(n int) string {
	s := "0000" + strconv.Itoa(n)
	return s[len(s)-4:]
}
