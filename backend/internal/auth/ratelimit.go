package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Policy is the OTP request budget.
//
//   - Per email: one request per Cooldown, and at most EmailDailyMax per 24h.
//     This is what protects a person from being mail-bombed, and it also limits
//     how long anyone can delay someone else's signup to Cooldown.
//   - Per IP: at most IPMax requests per IPWindow. It only has to stop one
//     source spraying many addresses, so offices and mobile carriers sharing an
//     address can still sign several people up.
type Policy struct {
	Cooldown      time.Duration
	EmailDailyMax int
	IPMax         int
	IPWindow      time.Duration

	// AuthIPMax requests per AuthIPWindow from one IP across the cheap-to-abuse
	// endpoints (signup availability check, resend/verify OTP, login, refresh).
	// It is charged before any database or bcrypt work.
	AuthIPMax    int
	AuthIPWindow time.Duration

	// Login lockout: after LoginMaxFailures wrong passwords for one email (or
	// LoginIPMaxFailures from one IP) further attempts are refused for
	// LoginLockBase, doubling with each extra failure up to LoginLockCap.
	LoginMaxFailures   int
	LoginIPMaxFailures int
	LoginLockBase      time.Duration
	LoginLockCap       time.Duration
}

func (p Policy) withDefaults() Policy {
	if p.AuthIPMax <= 0 {
		p.AuthIPMax = 60
	}
	if p.AuthIPWindow <= 0 {
		p.AuthIPWindow = 15 * time.Minute
	}
	if p.LoginMaxFailures <= 0 {
		p.LoginMaxFailures = 5
	}
	if p.LoginIPMaxFailures <= 0 {
		p.LoginIPMaxFailures = 20
	}
	if p.LoginLockBase <= 0 {
		p.LoginLockBase = time.Minute
	}
	if p.LoginLockCap <= 0 {
		p.LoginLockCap = time.Hour
	}
	return p
}

const dailyWindow = 24 * time.Hour

// takeScript checks every limit and, only if all pass, records the request, in
// one atomic step so concurrent requests cannot slip through together. It
// returns 0 when the caller may proceed, otherwise the milliseconds until the
// limit that blocks longest lifts.
//
//	KEYS: 1 email cooldown, 2 email daily counter, 3 ip counter
//	ARGV: 1 cooldown s, 2 daily max, 3 daily window s, 4 ip max, 5 ip window s
var takeScript = redis.NewScript(`
local function remaining(key, fallbackSec)
  local p = redis.call('PTTL', key)
  if p < 1 then return tonumber(fallbackSec) * 1000 end
  return p
end
local wait = 0
if redis.call('EXISTS', KEYS[1]) == 1 then
  wait = math.max(wait, remaining(KEYS[1], ARGV[1]))
end
if tonumber(redis.call('GET', KEYS[2]) or '0') >= tonumber(ARGV[2]) then
  wait = math.max(wait, remaining(KEYS[2], ARGV[3]))
end
if tonumber(redis.call('GET', KEYS[3]) or '0') >= tonumber(ARGV[4]) then
  wait = math.max(wait, remaining(KEYS[3], ARGV[5]))
end
if wait > 0 then return wait end

redis.call('SET', KEYS[1], '1', 'EX', ARGV[1])
if redis.call('INCR', KEYS[2]) == 1 then redis.call('EXPIRE', KEYS[2], ARGV[3]) end
if redis.call('INCR', KEYS[3]) == 1 then redis.call('EXPIRE', KEYS[3], ARGV[5]) end
return 0
`)

// releaseScript undoes one successful take.
var releaseScript = redis.NewScript(`
redis.call('DEL', KEYS[1])
for i = 2, 3 do
  if tonumber(redis.call('GET', KEYS[i]) or '0') > 0 then redis.call('DECR', KEYS[i]) end
end
return 0
`)

// Limiter enforces the Policy in Redis.
type Limiter struct {
	rdb *redis.Client
	p   Policy
}

func NewLimiter(rdb *redis.Client, p Policy) *Limiter { return &Limiter{rdb: rdb, p: p.withDefaults()} }

func (l *Limiter) Cooldown() time.Duration { return l.p.Cooldown }

func emailLimitKey(email string) string { return "ratelimit:otp:email:" + email }
func emailDailyKey(email string) string { return "ratelimit:otpdaily:email:" + email }
func ipLimitKey(ip string) string       { return "ratelimit:otp:ip:" + ip }

func (l *Limiter) keys(email, ip string) []string {
	return []string{emailLimitKey(email), emailDailyKey(email), ipLimitKey(ip)}
}

// Take spends one OTP request for email and ip. If any limit is exhausted it
// changes nothing and returns how long to wait (rounded up to whole seconds);
// a zero duration means the caller may proceed.
func (l *Limiter) Take(ctx context.Context, email, ip string) (time.Duration, error) {
	ms, err := takeScript.Run(ctx, l.rdb, l.keys(email, ip),
		int(l.p.Cooldown.Seconds()), l.p.EmailDailyMax, int(dailyWindow.Seconds()),
		l.p.IPMax, int(l.p.IPWindow.Seconds())).Int64()
	if err != nil {
		return 0, fmt.Errorf("auth: rate limit: %w", err)
	}
	if ms <= 0 {
		return 0, nil
	}
	return ceilSeconds(ms), nil
}

// Release gives back a request spent by Take, used when we could not send the
// email, so the user is not penalised for something that was our fault.
func (l *Limiter) Release(ctx context.Context, email, ip string) {
	_ = releaseScript.Run(ctx, l.rdb, l.keys(email, ip)).Err()
}

// hitScript is a fixed-window counter: it counts the hit and returns 0 while
// the count is within max, otherwise the milliseconds until the window ends.
//
//	KEYS: 1 counter   ARGV: 1 max, 2 window s
var hitScript = redis.NewScript(`
local n = redis.call('INCR', KEYS[1])
if n == 1 then redis.call('EXPIRE', KEYS[1], ARGV[2]) end
if n > tonumber(ARGV[1]) then
  local p = redis.call('PTTL', KEYS[1])
  if p < 1 then p = tonumber(ARGV[2]) * 1000 end
  return p
end
return 0
`)

func ceilSeconds(ms int64) time.Duration {
	return (time.Duration(ms)*time.Millisecond + time.Second - 1).Truncate(time.Second)
}

// HitIP charges one request of the shared auth budget to ip. It returns how
// long the caller must wait (0 = proceed). Cheap (one Redis call), so call it
// before touching Postgres or bcrypt.
func (l *Limiter) HitIP(ctx context.Context, scope, ip string) (time.Duration, error) {
	ms, err := hitScript.Run(ctx, l.rdb, []string{"ratelimit:authip:" + scope + ":" + ip},
		l.p.AuthIPMax, int(l.p.AuthIPWindow.Seconds())).Int64()
	if err != nil {
		return 0, fmt.Errorf("auth: rate limit: %w", err)
	}
	if ms <= 0 {
		return 0, nil
	}
	return ceilSeconds(ms), nil
}

func loginFailKey(kind, id string) string { return "ratelimit:login:fail:" + kind + ":" + id }
func loginLockKey(kind, id string) string { return "ratelimit:login:lock:" + kind + ":" + id }

// failScript records one failed login for a subject and, once max failures were
// seen, locks it for base * 2^(failures-max), capped.
//
//	KEYS: 1 failures, 2 lock   ARGV: 1 max, 2 base s, 3 cap s, 4 memory s
var failScript = redis.NewScript(`
local n = redis.call('INCR', KEYS[1])
redis.call('EXPIRE', KEYS[1], ARGV[4])
local max = tonumber(ARGV[1])
if n >= max then
  local lock = tonumber(ARGV[2]) * (2 ^ math.min(n - max, 20))
  lock = math.min(lock, tonumber(ARGV[3]))
  redis.call('SET', KEYS[2], '1', 'EX', math.floor(lock))
end
return n
`)

// LoginLocked reports how long login stays refused for this email and IP (0 =
// not locked). Check it before the bcrypt comparison.
func (l *Limiter) LoginLocked(ctx context.Context, emailID, ip string) (time.Duration, error) {
	var wait int64
	for _, k := range []string{loginLockKey("email", emailID), loginLockKey("ip", ip)} {
		p, err := l.rdb.PTTL(ctx, k).Result()
		if err != nil {
			return 0, fmt.Errorf("auth: rate limit: %w", err)
		}
		wait = max(wait, p.Milliseconds())
	}
	if wait <= 0 {
		return 0, nil
	}
	return ceilSeconds(wait), nil
}

// LoginFailed records a wrong password against the email and the IP.
func (l *Limiter) LoginFailed(ctx context.Context, emailID, ip string) error {
	memory := int((l.p.LoginLockCap + l.p.LoginLockBase).Seconds())
	for _, s := range []struct {
		kind, id string
		max      int
	}{{"email", emailID, l.p.LoginMaxFailures}, {"ip", ip, l.p.LoginIPMaxFailures}} {
		err := failScript.Run(ctx, l.rdb, []string{loginFailKey(s.kind, s.id), loginLockKey(s.kind, s.id)},
			s.max, int(l.p.LoginLockBase.Seconds()), int(l.p.LoginLockCap.Seconds()), memory).Err()
		if err != nil {
			return fmt.Errorf("auth: rate limit: %w", err)
		}
	}
	return nil
}

// LoginSucceeded forgets the email's failures (the IP's are kept: one correct
// login must not reset a credential-stuffing source).
func (l *Limiter) LoginSucceeded(ctx context.Context, emailID string) {
	_ = l.rdb.Del(ctx, loginFailKey("email", emailID), loginLockKey("email", emailID)).Err()
}
