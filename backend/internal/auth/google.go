package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// GoogleConfig holds the OAuth client settings. The zero value (no ClientID) means
// Google sign-in is switched off.
type GoogleConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string // this API's /api/auth/google/callback
	FrontendURL  string // browser is sent back here, e.g. http://localhost:5173

	// Endpoints, overridable in tests.
	AuthURL  string
	TokenURL string
}

const (
	googleAuthURL  = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL = "https://oauth2.googleapis.com/token"
)

func (c GoogleConfig) Enabled() bool { return c.ClientID != "" && c.ClientSecret != "" }

func (c GoogleConfig) authURL() string {
	if c.AuthURL != "" {
		return c.AuthURL
	}
	return googleAuthURL
}

func (c GoogleConfig) tokenURL() string {
	if c.TokenURL != "" {
		return c.TokenURL
	}
	return googleTokenURL
}

// secureCookies is true when the callback is served over https.
func (c GoogleConfig) secureCookies() bool { return strings.HasPrefix(c.RedirectURL, "https://") }

// codeChallenge is the PKCE S256 challenge of a verifier.
func codeChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// authorizeURL builds the Google consent URL. Only the openid, email and profile
// scopes are asked for, and the account chooser is always shown.
func (c GoogleConfig) authorizeURL(state, nonce, verifier string) string {
	q := url.Values{
		"client_id":             {c.ClientID},
		"redirect_uri":          {c.RedirectURL},
		"response_type":         {"code"},
		"scope":                 {"openid email profile"},
		"state":                 {state},
		"nonce":                 {nonce},
		"code_challenge":        {codeChallenge(verifier)},
		"code_challenge_method": {"S256"},
		"prompt":                {"select_account"},
	}
	return c.authURL() + "?" + q.Encode()
}

// googleClaims are the ID-token claims we use.
type googleClaims struct {
	Issuer        string `json:"iss"`
	Audience      string `json:"aud"`
	Expires       int64  `json:"exp"`
	Subject       string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified any    `json:"email_verified"` // bool, or the string "true"
	Nonce         string `json:"nonce"`
	Name          string `json:"name"`
}

func (g googleClaims) verified() bool {
	switch v := g.EmailVerified.(type) {
	case bool:
		return v
	case string:
		return v == "true"
	}
	return false
}

// Errors from the code exchange.
var (
	ErrGoogleExchange = errors.New("auth: google code exchange failed")
	ErrGoogleToken    = errors.New("auth: invalid google id token")
)

// exchange trades the authorization code for the user's identity. The ID token comes
// straight from Google's token endpoint over TLS, authenticated with our client
// secret, so (as OpenID Connect allows) its signature is not checked again; its
// issuer, audience, expiry and nonce are.
func (c GoogleConfig) exchange(ctx context.Context, hc *http.Client, code, verifier, nonce string, now time.Time) (*googleClaims, error) {
	form := url.Values{
		"code":          {code},
		"client_id":     {c.ClientID},
		"client_secret": {c.ClientSecret},
		"redirect_uri":  {c.RedirectURL},
		"grant_type":    {"authorization_code"},
		"code_verifier": {verifier},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL(), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	res, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGoogleExchange, err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGoogleExchange, err)
	}
	if res.StatusCode != http.StatusOK {
		// Google's error body names the problem (invalid_grant, ...) and holds no secrets.
		return nil, fmt.Errorf("%w: status %d: %s", ErrGoogleExchange, res.StatusCode, truncate(string(body), 200))
	}
	var tok struct {
		IDToken string `json:"id_token"`
	}
	if err := json.Unmarshal(body, &tok); err != nil || tok.IDToken == "" {
		return nil, fmt.Errorf("%w: no id_token", ErrGoogleExchange)
	}
	return c.checkIDToken(tok.IDToken, nonce, now)
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// checkIDToken decodes the ID token payload and checks the claims.
func (c GoogleConfig) checkIDToken(idToken, nonce string, now time.Time) (*googleClaims, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return nil, ErrGoogleToken
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrGoogleToken
	}
	var cl googleClaims
	if err := json.Unmarshal(payload, &cl); err != nil {
		return nil, ErrGoogleToken
	}
	switch {
	case cl.Issuer != "https://accounts.google.com" && cl.Issuer != "accounts.google.com":
		return nil, fmt.Errorf("%w: issuer", ErrGoogleToken)
	case cl.Audience != c.ClientID:
		return nil, fmt.Errorf("%w: audience", ErrGoogleToken)
	case cl.Expires <= now.Unix():
		return nil, fmt.Errorf("%w: expired", ErrGoogleToken)
	case cl.Nonce == "" || cl.Nonce != nonce:
		return nil, fmt.Errorf("%w: nonce", ErrGoogleToken)
	case cl.Subject == "" || len(cl.Subject) > 255:
		return nil, fmt.Errorf("%w: subject", ErrGoogleToken)
	}
	return &cl, nil
}
