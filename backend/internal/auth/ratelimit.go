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

func NewLimiter(rdb *redis.Client, p Policy) *Limiter { return &Limiter{rdb: rdb, p: p} }

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
	return (time.Duration(ms)*time.Millisecond + time.Second - 1).Truncate(time.Second), nil
}

// Release gives back a request spent by Take, used when we could not send the
// email, so the user is not penalised for something that was our fault.
func (l *Limiter) Release(ctx context.Context, email, ip string) {
	_ = releaseScript.Run(ctx, l.rdb, l.keys(email, ip)).Err()
}
