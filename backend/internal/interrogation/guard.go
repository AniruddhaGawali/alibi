package interrogation

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"backend/pkg/utils"
	"github.com/redis/go-redis/v9"
)

var (
	// ErrBusy means the player already has a question in flight. One at a time.
	ErrBusy = errors.New("interrogation: a question is already being answered")
	// ErrUnavailable means the model is closed to everybody for now: the daily
	// budget is spent, the upstream is failing, or too many calls are in flight.
	ErrUnavailable = errors.New("interrogation: suspects are unavailable right now")
)

// LimitError means the player (or their address) is over a limit. RetryAfter says when to try again.
type LimitError struct {
	Scope      string // "minute", "ip", "daily" or "strikes"
	RetryAfter time.Duration
}

func (e *LimitError) Error() string {
	return fmt.Sprintf("interrogation: %s limit reached, retry in %s", e.Scope, e.RetryAfter)
}

// Guard limits how much of the model one player, one address and everybody
// together can use. A nil Guard on the Service means no limits (tests).
type Guard interface {
	// AdmitIP charges one request against the address's per-minute budget.
	AdmitIP(ctx context.Context, ip string) error
	// Admit charges one request (even one moderation will turn away) against the
	// player's per-minute budget, and refuses players locked out by Strike.
	Admit(ctx context.Context, userID string) error
	// Strike records a question that moderation blocked as an injection attempt.
	Strike(ctx context.Context, userID string)
	// Begin takes the player's in-flight slot, one call off their daily model-call
	// allowance and one off the global budget, and a slot in the global concurrency
	// cap. Call release when the model call is over.
	Begin(ctx context.Context, userID string) (release func(), err error)
	// Finish records what a finished call cost and feeds the circuit breaker: err is
	// the model's error, nil on success.
	Finish(ctx context.Context, tokens int64, err error)
	// AllowRefund reports whether the player may still get a token back today. The
	// daily model-call counter is never refunded, so calls <= tokens + refunds.
	AllowRefund(ctx context.Context, userID string) bool
}

// GuardConfig sets the limits of a RedisGuard.
type GuardConfig struct {
	PerMinute      int // requests per player per minute
	IPPerMinute    int // requests per address per minute
	UserDailyCalls int // model calls per player per UTC day, refunded or not
	// DailyCalls, when set, replaces UserDailyCalls on every question, so it can
	// follow the daily token allowance edited in the admin panel.
	DailyCalls       func(context.Context) int
	RefundsPerDay    int           // tokens given back per player per UTC day
	MaxConcurrent    int           // model calls in flight in this process
	DailyCallBudget  int64         // model calls per UTC day across all players; 0 = no limit
	DailyTokenBudget int64         // model tokens per UTC day across all players; 0 = no limit
	StrikesPerHour   int           // blocked injection attempts per hour before the player is locked out; 0 = never
	InFlightTTL      time.Duration // how long a crashed request can hold the player's slot
	BreakerFailures  int           // consecutive upstream failures that open the circuit breaker
	BreakerOpenFor   time.Duration
}

// RedisGuard is the Guard backed by Redis counters (shared by every replica) and
// an in-process concurrency cap and circuit breaker.
type RedisGuard struct {
	rdb *redis.Client
	cfg GuardConfig
	sem chan struct{}
	now func() time.Time

	mu        sync.Mutex
	failures  int
	openUntil time.Time
}

func NewRedisGuard(rdb *redis.Client, cfg GuardConfig) *RedisGuard {
	if cfg.MaxConcurrent < 1 {
		cfg.MaxConcurrent = 1
	}
	if cfg.BreakerFailures < 1 {
		cfg.BreakerFailures = 5
	}
	if cfg.BreakerOpenFor <= 0 {
		cfg.BreakerOpenFor = 30 * time.Second
	}
	if cfg.InFlightTTL <= 0 {
		cfg.InFlightTTL = time.Minute
	}
	return &RedisGuard{rdb: rdb, cfg: cfg, sem: make(chan struct{}, cfg.MaxConcurrent), now: time.Now}
}

// windowScript is a fixed-window counter: 0 while within max, otherwise the ms
// until the window ends. KEYS: 1 counter. ARGV: 1 max, 2 window s.
var windowScript = redis.NewScript(`
local n = redis.call('INCR', KEYS[1])
if n == 1 then redis.call('EXPIRE', KEYS[1], ARGV[2]) end
if n > tonumber(ARGV[1]) then
  local p = redis.call('PTTL', KEYS[1])
  if p < 1 then p = tonumber(ARGV[2]) * 1000 end
  return p
end
return 0
`)

// window charges one hit on key and returns how long to wait when over max.
func (g *RedisGuard) window(ctx context.Context, key string, max int, window time.Duration) (time.Duration, error) {
	ms, err := windowScript.Run(ctx, g.rdb, []string{key}, max, int(window.Seconds())).Int64()
	if err != nil {
		return 0, err
	}
	if ms <= 0 {
		return 0, nil
	}
	return roundUp(time.Duration(ms) * time.Millisecond), nil
}

func roundUp(d time.Duration) time.Duration { return (d + time.Second - 1).Truncate(time.Second) }

// day is the UTC calendar day the daily counters are keyed by, and the time left in it.
func (g *RedisGuard) day() (string, time.Duration) {
	t := g.now().UTC()
	next := time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, time.UTC)
	return t.Format(time.DateOnly), next.Sub(t)
}

func (g *RedisGuard) AdmitIP(ctx context.Context, ip string) error {
	if g.cfg.IPPerMinute <= 0 || ip == "" {
		return nil
	}
	wait, err := g.window(ctx, "llm:rate:ip:"+ip, g.cfg.IPPerMinute, time.Minute)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	if wait > 0 {
		return &LimitError{Scope: "ip", RetryAfter: wait}
	}
	return nil
}

func (g *RedisGuard) Admit(ctx context.Context, userID string) error {
	if g.cfg.StrikesPerHour > 0 {
		key := "llm:strikes:" + userID
		n, err := g.rdb.Get(ctx, key).Int()
		if err != nil && !errors.Is(err, redis.Nil) {
			return fmt.Errorf("%w: %w", ErrUnavailable, err)
		}
		if n >= g.cfg.StrikesPerHour {
			ttl, _ := g.rdb.PTTL(ctx, key).Result()
			return &LimitError{Scope: "strikes", RetryAfter: roundUp(max(ttl, time.Second))}
		}
	}
	if g.cfg.PerMinute > 0 {
		wait, err := g.window(ctx, "llm:rate:user:"+userID, g.cfg.PerMinute, time.Minute)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrUnavailable, err)
		}
		if wait > 0 {
			return &LimitError{Scope: "minute", RetryAfter: wait}
		}
	}
	return nil
}

// ResetLimits clears one player's interrogation limits for today: the daily
// model-call counter, the refund counter, the per-minute window and any strike
// lockout. It is the admin panel's way to unblock a player; their token balance and
// the global budgets are untouched.
func (g *RedisGuard) ResetLimits(ctx context.Context, userID string) error {
	day, _ := g.day()
	return g.rdb.Del(ctx,
		"llm:calls:"+userID+":"+day,
		"llm:refunds:"+userID+":"+day,
		"llm:rate:user:"+userID,
		"llm:strikes:"+userID,
	).Err()
}

func (g *RedisGuard) Strike(ctx context.Context, userID string) {
	if g.cfg.StrikesPerHour <= 0 {
		return
	}
	key := "llm:strikes:" + userID
	pipe := g.rdb.TxPipeline()
	pipe.Incr(ctx, key)
	pipe.ExpireNX(ctx, key, time.Hour)
	if _, err := pipe.Exec(ctx); err != nil {
		utils.LogErrorCtx(ctx, "record injection strike failed", "user_id", userID, "err", err)
	}
}

func (g *RedisGuard) Begin(ctx context.Context, userID string) (func(), error) {
	if g.breakerOpen() {
		return nil, ErrUnavailable
	}
	day, left := g.day()

	if g.cfg.DailyTokenBudget > 0 {
		used, err := g.rdb.Get(ctx, "llm:global:tokens:"+day).Int64()
		if err != nil && !errors.Is(err, redis.Nil) {
			return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
		}
		if used >= g.cfg.DailyTokenBudget {
			utils.LogErrorCtx(ctx, "daily llm token budget spent", "used", used, "budget", g.cfg.DailyTokenBudget)
			return nil, ErrUnavailable
		}
	}

	select {
	case g.sem <- struct{}{}:
	default:
		return nil, ErrUnavailable
	}
	releaseSem := func() { <-g.sem }

	// One question at a time per player.
	slot := "llm:inflight:" + userID
	ok, err := g.rdb.SetNX(ctx, slot, "1", g.cfg.InFlightTTL).Result()
	if err != nil {
		releaseSem()
		return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	if !ok {
		releaseSem()
		return nil, ErrBusy
	}
	release := func() {
		releaseSem()
		rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		if err := g.rdb.Del(rctx, slot).Err(); err != nil {
			utils.LogErrorCtx(ctx, "release in-flight slot failed", "user_id", userID, "err", err)
		}
	}

	// The daily allowance counts every call that reached the provider, and is never
	// given back: a refund returns a token, not a call.
	dailyCalls := g.cfg.UserDailyCalls
	if g.cfg.DailyCalls != nil {
		dailyCalls = g.cfg.DailyCalls(ctx)
	}
	if dailyCalls > 0 {
		wait, err := g.window(ctx, "llm:calls:"+userID+":"+day, dailyCalls, left)
		if err != nil {
			release()
			return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
		}
		if wait > 0 {
			release()
			return nil, &LimitError{Scope: "daily", RetryAfter: wait}
		}
	}
	if g.cfg.DailyCallBudget > 0 {
		n, err := g.rdb.Incr(ctx, "llm:global:calls:"+day).Result()
		if err != nil {
			release()
			return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
		}
		if n == 1 {
			g.rdb.Expire(ctx, "llm:global:calls:"+day, left+time.Hour)
		}
		if n > g.cfg.DailyCallBudget {
			release()
			utils.LogErrorCtx(ctx, "daily llm call budget spent", "calls", n, "budget", g.cfg.DailyCallBudget)
			return nil, ErrUnavailable
		}
	}
	return release, nil
}

func (g *RedisGuard) Finish(ctx context.Context, tokens int64, err error) {
	g.report(err)
	if tokens <= 0 || g.cfg.DailyTokenBudget <= 0 {
		return
	}
	day, left := g.day()
	key := "llm:global:tokens:" + day
	n, e := g.rdb.IncrBy(context.WithoutCancel(ctx), key, tokens).Result()
	if e != nil {
		utils.LogErrorCtx(ctx, "record llm token usage failed", "err", e)
		return
	}
	if n == tokens {
		g.rdb.Expire(context.WithoutCancel(ctx), key, left+time.Hour)
	}
}

func (g *RedisGuard) AllowRefund(ctx context.Context, userID string) bool {
	if g.cfg.RefundsPerDay <= 0 {
		return false
	}
	day, left := g.day()
	wait, err := g.window(ctx, "llm:refunds:"+userID+":"+day, g.cfg.RefundsPerDay, left)
	if err != nil {
		utils.LogErrorCtx(ctx, "refund allowance check failed; not refunding", "user_id", userID, "err", err)
		return false
	}
	return wait == 0
}

// report feeds the circuit breaker: after BreakerFailures failed calls in a row
// it opens for BreakerOpenFor, so a dead or out-of-credit upstream (OpenRouter
// answers 402 and 429 then) is not hit by every request. A reply the model gave
// but we could not use is not an upstream failure.
func (g *RedisGuard) report(err error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if err == nil || !countsAsOutage(err) {
		g.failures = 0
		return
	}
	g.failures++
	if g.failures >= g.cfg.BreakerFailures {
		g.openUntil = g.now().Add(g.cfg.BreakerOpenFor)
		g.failures = 0
		utils.LogError("llm circuit breaker opened", "for", g.cfg.BreakerOpenFor.String(), "err", err)
	}
}

func (g *RedisGuard) breakerOpen() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.now().Before(g.openUntil)
}

// secondsString is the Retry-After header value for a wait.
func secondsString(d time.Duration) string { return strconv.Itoa(int(d.Seconds())) }
