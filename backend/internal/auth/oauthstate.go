package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrNoState means the key was never stored, has expired or was already used.
var ErrNoState = errors.New("auth: no such state")

// OAuthStore keeps short-lived, single-use records in Redis: the pending Google
// sign-in (state to PKCE verifier and nonce) and the one-time tickets that hand the
// finished session to the frontend.
type OAuthStore struct{ rdb *redis.Client }

func NewOAuthStore(rdb *redis.Client) *OAuthStore { return &OAuthStore{rdb: rdb} }

// RandomToken returns n random bytes as URL-safe base64.
func RandomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Put stores v under key for ttl.
func (s *OAuthStore) Put(ctx context.Context, key string, v any, ttl time.Duration) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, key, raw, ttl).Err()
}

// Take reads and deletes key in one atomic step, so a record can be used once only.
func (s *OAuthStore) Take(ctx context.Context, key string, v any) error {
	raw, err := s.rdb.GetDel(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return ErrNoState
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, v)
}
