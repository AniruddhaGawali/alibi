package user

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// UploadLimiter throttles custom avatar uploads per user.
type UploadLimiter interface {
	// Allow spends one upload. A non-zero wait means the user is over the limit.
	Allow(ctx context.Context, userID string) (wait time.Duration, err error)
}

// uploadScript is a fixed-window counter: 0 while within max, otherwise the ms
// until the window ends. KEYS: 1 counter. ARGV: 1 max, 2 window s.
var uploadScript = redis.NewScript(`
local n = redis.call('INCR', KEYS[1])
if n == 1 then redis.call('EXPIRE', KEYS[1], ARGV[2]) end
if n > tonumber(ARGV[1]) then
  local p = redis.call('PTTL', KEYS[1])
  if p < 1 then p = tonumber(ARGV[2]) * 1000 end
  return p
end
return 0
`)

// RedisUploadLimiter allows max uploads per user in each window.
type RedisUploadLimiter struct {
	rdb    *redis.Client
	max    int
	window time.Duration
}

func NewRedisUploadLimiter(rdb *redis.Client, max int, window time.Duration) *RedisUploadLimiter {
	return &RedisUploadLimiter{rdb: rdb, max: max, window: window}
}

func (l *RedisUploadLimiter) Allow(ctx context.Context, userID string) (time.Duration, error) {
	ms, err := uploadScript.Run(ctx, l.rdb, []string{"ratelimit:avatar:" + userID}, l.max, int(l.window.Seconds())).Int64()
	if err != nil {
		return 0, fmt.Errorf("user: upload rate limit: %w", err)
	}
	if ms <= 0 {
		return 0, nil
	}
	return (time.Duration(ms)*time.Millisecond + time.Second - 1).Truncate(time.Second), nil
}
