// Package auth issues and validates JWTs, keeps the Redis blacklist for
// logged-out tokens and serves the /api/auth endpoints.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

type TokenType string

const (
	Access  TokenType = "access"
	Refresh TokenType = "refresh"

	issuer         = "alibi-api"
	blacklistKeyFm = "blacklist:jwt:%s"
)

var ErrInvalidToken = errors.New("auth: invalid token")

// Claims are the JWT claims: Subject is the user ID, ID is the unique jti.
type Claims struct {
	Type  TokenType `json:"typ"`
	Email string    `json:"email"`
	jwt.RegisteredClaims
}

// Remaining is how long the token stays valid; <= 0 when expired.
func (c *Claims) Remaining(now time.Time) time.Duration {
	if c.ExpiresAt == nil {
		return 0
	}
	return c.ExpiresAt.Sub(now)
}

type TokenManager struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
	now        func() time.Time
}

func NewTokenManager(secret string, accessTTL, refreshTTL time.Duration) *TokenManager {
	return &TokenManager{secret: []byte(secret), accessTTL: accessTTL, refreshTTL: refreshTTL, now: time.Now}
}

func newJTI() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// Issue signs a token of the given type for userID.
func (m *TokenManager) Issue(userID, email string, typ TokenType) (string, *Claims, error) {
	jti, err := newJTI()
	if err != nil {
		return "", nil, err
	}
	ttl := m.accessTTL
	if typ == Refresh {
		ttl = m.refreshTTL
	}
	now := m.now()
	claims := &Claims{
		Type:  typ,
		Email: email,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   userID,
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", nil, err
	}
	return signed, claims, nil
}

// Parse verifies signature, expiry, issuer and that the token is of type want.
func (m *TokenManager) Parse(token string, want TokenType) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(token, claims,
		func(*jwt.Token) (any, error) { return m.secret, nil },
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithIssuer(issuer),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(m.now),
	)
	if err != nil || claims.Type != want || claims.Subject == "" || claims.ID == "" {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// Blacklist records revoked token IDs in Redis. Entries expire together with
// the token, so the set never grows past the tokens still in circulation.
type Blacklist struct{ rdb *redis.Client }

func NewBlacklist(rdb *redis.Client) *Blacklist { return &Blacklist{rdb: rdb} }

func blacklistKey(jti string) string { return fmt.Sprintf(blacklistKeyFm, jti) }

// Revoke blacklists jti for ttl. An already-expired token needs no entry.
func (b *Blacklist) Revoke(ctx context.Context, jti string, ttl time.Duration) error {
	if ttl <= 0 {
		return nil
	}
	return b.rdb.Set(ctx, blacklistKey(jti), "true", ttl).Err()
}

// RevokeAll blacklists every token in one Redis transaction, so a logout
// either revokes all of them or none.
func (b *Blacklist) RevokeAll(ctx context.Context, now time.Time, claims ...*Claims) error {
	pipe := b.rdb.TxPipeline()
	for _, c := range claims {
		if ttl := c.Remaining(now); ttl > 0 {
			pipe.Set(ctx, blacklistKey(c.ID), "true", ttl)
		}
	}
	_, err := pipe.Exec(ctx)
	return err
}

// Claim blacklists jti only if it is not blacklisted yet and reports whether
// this call did it. Used for refresh rotation so a token works exactly once,
// even under concurrent requests.
func (b *Blacklist) Claim(ctx context.Context, jti string, ttl time.Duration) (bool, error) {
	if ttl <= 0 {
		return false, nil
	}
	return b.rdb.SetNX(ctx, blacklistKey(jti), "true", ttl).Result()
}

func (b *Blacklist) IsRevoked(ctx context.Context, jti string) (bool, error) {
	n, err := b.rdb.Exists(ctx, blacklistKey(jti)).Result()
	return n > 0, err
}
