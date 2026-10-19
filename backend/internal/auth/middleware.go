package auth

import (
	"context"
	"net/http"
	"path"
	"strings"

	"backend/pkg/utils"
)

type ctxKey struct{}

// ClaimsFrom returns the validated access-token claims set by Middleware.
func ClaimsFrom(ctx context.Context) (*Claims, bool) {
	c, ok := ctx.Value(ctxKey{}).(*Claims)
	return c, ok
}

// Role returns the role claim of the authenticated user's access token ("" for a
// token issued before roles were added).
func Role(ctx context.Context) string {
	if c, ok := ClaimsFrom(ctx); ok {
		return c.Role
	}
	return ""
}

// UserID returns the authenticated user's ID set by Middleware.
func UserID(ctx context.Context) string {
	if c, ok := ClaimsFrom(ctx); ok {
		return c.Subject
	}
	return ""
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if len(h) > 7 && strings.EqualFold(h[:7], "Bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

// PublicRoutes lists the routes that may be called without a token. Everything
// else requires a valid access token: the check is deny-by-default, so a new
// route is protected until someone deliberately adds it here.
type PublicRoutes struct {
	exact  map[string]bool
	prefix []string
}

// NewPublicRoutes builds the allow-list from "METHOD /path" patterns. A
// pattern ending in "/" matches every path under it ("GET /api/user/avatars/").
func NewPublicRoutes(patterns ...string) *PublicRoutes {
	p := &PublicRoutes{exact: make(map[string]bool)}
	for _, pat := range patterns {
		if strings.HasSuffix(pat, "/") {
			p.prefix = append(p.prefix, pat)
		} else {
			p.exact[pat] = true
		}
	}
	return p
}

func (p *PublicRoutes) allows(r *http.Request) bool {
	// Only exact, already-clean paths can be public. Anything odd ("/a/../b",
	// "//a", trailing dots) fails closed and needs a token.
	if path.Clean(r.URL.Path) != r.URL.Path {
		return false
	}
	key := r.Method + " " + r.URL.Path
	if p.exact[key] {
		return true
	}
	for _, pre := range p.prefix {
		if strings.HasPrefix(key, pre) {
			return true
		}
	}
	return false
}

// Middleware checks the JWT before any handler runs. Public routes pass
// straight through; every other request must carry a valid, unexpired,
// non-blacklisted access token, otherwise it is answered with 401 and never
// reaches the handler. Valid claims are put in the request context
// (ClaimsFrom, UserID).
//
// Install it inside middleware.Logging so rejected requests are still logged.
func Middleware(tm *TokenManager, bl *Blacklist, public *PublicRoutes) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if public.allows(r) {
				next.ServeHTTP(w, r)
				return
			}

			claims, err := tm.Parse(bearerToken(r), Access)
			if err != nil {
				utils.LogInfoCtx(r.Context(), "auth rejected: invalid token", "method", r.Method, "path", r.URL.Path)
				unauthorized(w)
				return
			}
			revoked, err := bl.IsRevoked(r.Context(), claims.ID)
			if err != nil {
				// Fail closed: without the blacklist we cannot tell a logged-out token apart.
				utils.LogErrorCtx(r.Context(), "blacklist check failed", "err", err)
				utils.ErrorResponse(w, http.StatusServiceUnavailable, "service unavailable", nil)
				return
			}
			if revoked {
				utils.LogInfoCtx(r.Context(), "auth rejected: revoked token", "user_id", claims.Subject)
				unauthorized(w)
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, claims)))
		})
	}
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="alibi"`)
	utils.ErrorResponse(w, http.StatusUnauthorized, "unauthorized", nil)
}
