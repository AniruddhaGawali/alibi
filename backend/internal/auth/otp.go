package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
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
// No PostgreSQL row exists for the user before that. Only an HMAC of the code is
// stored: the plaintext lives just in the queued email.
type Session struct {
	OTPHash        string `json:"otp_hash"`
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
	key         []byte // HMAC key for stored code hashes
}

// NewOTPStore creates the store. hmacKey keys the code hashes kept in Redis; use
// a server secret (it is domain-separated internally).
func NewOTPStore(rdb *redis.Client, ttl time.Duration, maxAttempts int, hmacKey []byte) *OTPStore {
	return &OTPStore{rdb: rdb, ttl: ttl, maxAttempts: maxAttempts, key: append([]byte("alibi:otp:v1:"), hmacKey...)}
}

func (s *OTPStore) TTL() time.Duration { return s.ttl }

// Hash returns the value stored in a Session for code otp sent to email.
func (s *OTPStore) Hash(email, otp string) string {
	m := hmac.New(sha256.New, s.key)
	m.Write([]byte(email))
	m.Write([]byte{0})
	m.Write([]byte(otp))
	return hex.EncodeToString(m.Sum(nil))
}

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

// VerifyResult is the outcome of OTPStore.Verify.
type VerifyResult int

const (
	VerifyNoSession VerifyResult = iota // never started, expired, or already discarded
	VerifyOK                            // the code matched
	VerifyWrong                         // wrong code; attempts remain
	VerifyLocked                        // attempts exhausted; the session was discarded
)

// verifyScript charges an attempt BEFORE comparing, atomically, so parallel
// requests cannot each get a free guess: attempt n is allowed only if n <= max.
// The wrong-code counter is cleared on success only.
//
//	KEYS: 1 session, 2 attempts
//	ARGV: 1 max attempts, 2 ttl s, 3 expected code hash
//	returns {0} no session | {1, session json} ok | {2} wrong | {3} locked
var verifyScript = redis.NewScript(`
local raw = redis.call('GET', KEYS[1])
if not raw then return {0} end
local n = redis.call('INCR', KEYS[2])
if n == 1 then redis.call('EXPIRE', KEYS[2], ARGV[2]) end
local max = tonumber(ARGV[1])
if n > max then
  redis.call('DEL', KEYS[1], KEYS[2])
  return {3}
end
local sess = cjson.decode(raw)
if sess.otp_hash == ARGV[3] then
  redis.call('DEL', KEYS[2])
  return {1, raw}
end
if n >= max then
  redis.call('DEL', KEYS[1], KEYS[2])
  return {3}
end
return {2}
`)

// Verify checks code against the pending session for email. On VerifyOK it
// returns the session; the caller creates the user and then Deletes it.
func (s *OTPStore) Verify(ctx context.Context, email, code string) (VerifyResult, *Session, error) {
	res, err := verifyScript.Run(ctx, s.rdb, []string{sessionKey(email), attemptsKey(email)},
		s.maxAttempts, int(s.ttl.Seconds()), s.Hash(email, code)).Slice()
	if err != nil {
		return VerifyNoSession, nil, fmt.Errorf("auth: verify otp: %w", err)
	}
	if len(res) == 0 {
		return VerifyNoSession, nil, nil
	}
	switch status, _ := res[0].(int64); status {
	case 1:
		raw, _ := res[1].(string)
		var sess Session
		if err := json.Unmarshal([]byte(raw), &sess); err != nil {
			return VerifyNoSession, nil, fmt.Errorf("auth: corrupt signup session: %w", err)
		}
		return VerifyOK, &sess, nil
	case 2:
		return VerifyWrong, nil, nil
	case 3:
		return VerifyLocked, nil, nil
	}
	return VerifyNoSession, nil, nil
}
