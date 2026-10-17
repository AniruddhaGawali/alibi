package auth

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrNoSession means there is no pending signup for the email (never started,
// expired, or discarded after too many wrong codes).
var ErrNoSession = errors.New("auth: no pending signup")

// Session is the pending registration kept in Redis until the OTP is verified.
// No PostgreSQL row exists for the user before that.
type Session struct {
	OTP            string `json:"otp"`
	Username       string `json:"username"`
	HashedPassword string `json:"hashed_password"`
}

func sessionKey(email string) string  { return "otp:signup:" + email }
func attemptsKey(email string) string { return "otpattempts:signup:" + email }

// OTPStore keeps signup sessions in Redis.
type OTPStore struct {
	rdb         *redis.Client
	ttl         time.Duration
	maxAttempts int
}

func NewOTPStore(rdb *redis.Client, ttl time.Duration, maxAttempts int) *OTPStore {
	return &OTPStore{rdb: rdb, ttl: ttl, maxAttempts: maxAttempts}
}

func (s *OTPStore) TTL() time.Duration { return s.ttl }

// GenerateOTP returns a uniformly random 6-digit code ("000000"-"999999").
func GenerateOTP() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// Save stores the session with a fresh TTL and clears the wrong-code counter,
// so a resent code starts with a full window and full attempts.
func (s *OTPStore) Save(ctx context.Context, email string, sess Session) error {
	raw, err := json.Marshal(sess)
	if err != nil {
		return err
	}
	pipe := s.rdb.TxPipeline()
	pipe.Set(ctx, sessionKey(email), raw, s.ttl)
	pipe.Del(ctx, attemptsKey(email))
	_, err = pipe.Exec(ctx)
	return err
}

// Get returns the pending session, or ErrNoSession.
func (s *OTPStore) Get(ctx context.Context, email string) (*Session, error) {
	raw, err := s.rdb.Get(ctx, sessionKey(email)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrNoSession
	}
	if err != nil {
		return nil, err
	}
	var sess Session
	if err := json.Unmarshal(raw, &sess); err != nil {
		return nil, fmt.Errorf("auth: corrupt signup session: %w", err)
	}
	return &sess, nil
}

// Delete removes the session and its attempt counter.
func (s *OTPStore) Delete(ctx context.Context, email string) error {
	return s.rdb.Del(ctx, sessionKey(email), attemptsKey(email)).Err()
}

// Fail records a wrong code. Once maxAttempts is reached the session is
// discarded, so a 6-digit code cannot be brute-forced within its lifetime. It
// reports whether the session was discarded.
func (s *OTPStore) Fail(ctx context.Context, email string) (discarded bool, err error) {
	pipe := s.rdb.TxPipeline()
	incr := pipe.Incr(ctx, attemptsKey(email))
	pipe.Expire(ctx, attemptsKey(email), s.ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return false, err
	}
	if int(incr.Val()) >= s.maxAttempts {
		return true, s.Delete(ctx, email)
	}
	return false, nil
}
