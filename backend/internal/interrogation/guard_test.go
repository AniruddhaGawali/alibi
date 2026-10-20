package interrogation

import (
	"context"
	"errors"
	"testing"
	"time"

	"backend/internal/llm"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newGuard(t *testing.T, cfg GuardConfig) (*RedisGuard, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return NewRedisGuard(rdb, cfg), mr
}

func TestGuardPerMinuteAndIP(t *testing.T) {
	g, _ := newGuard(t, GuardConfig{PerMinute: 3, IPPerMinute: 2})
	ctx := context.Background()
	for range 3 {
		if err := g.Admit(ctx, "u1"); err != nil {
			t.Fatal(err)
		}
	}
	var lim *LimitError
	if err := g.Admit(ctx, "u1"); !errors.As(err, &lim) || lim.Scope != "minute" || lim.RetryAfter <= 0 {
		t.Fatalf("4th request: %v", err)
	}
	if err := g.Admit(ctx, "u2"); err != nil {
		t.Fatalf("another player must not be affected: %v", err)
	}
	_ = g.AdmitIP(ctx, "1.2.3.4")
	_ = g.AdmitIP(ctx, "1.2.3.4")
	if err := g.AdmitIP(ctx, "1.2.3.4"); !errors.As(err, &lim) || lim.Scope != "ip" {
		t.Fatalf("3rd request from one address: %v", err)
	}
}

func TestGuardOneQuestionAtATimeAndDailyCalls(t *testing.T) {
	g, _ := newGuard(t, GuardConfig{MaxConcurrent: 4, UserDailyCalls: 2})
	ctx := context.Background()
	release, err := g.Begin(ctx, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.Begin(ctx, "u1"); !errors.Is(err, ErrBusy) {
		t.Fatalf("second in-flight question: %v", err)
	}
	release()
	release, err = g.Begin(ctx, "u1")
	if err != nil {
		t.Fatalf("after release: %v", err)
	}
	release()
	// Two calls so far; a refund does not give a call back.
	var lim *LimitError
	if _, err := g.Begin(ctx, "u1"); !errors.As(err, &lim) || lim.Scope != "daily" {
		t.Fatalf("third call of the day: %v", err)
	}
	if _, err := g.Begin(ctx, "u2"); err != nil {
		t.Fatalf("another player: %v", err)
	}
}

func TestGuardGlobalConcurrencyAndBudget(t *testing.T) {
	g, _ := newGuard(t, GuardConfig{MaxConcurrent: 1, DailyCallBudget: 2, DailyTokenBudget: 1000})
	ctx := context.Background()
	release, err := g.Begin(ctx, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.Begin(ctx, "u2"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("over the concurrency cap: %v", err)
	}
	release()
	release, err = g.Begin(ctx, "u2")
	if err != nil {
		t.Fatal(err)
	}
	release()
	if _, err := g.Begin(ctx, "u3"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("over the daily call budget: %v", err)
	}

	g, _ = newGuard(t, GuardConfig{MaxConcurrent: 2, DailyTokenBudget: 1000})
	g.Finish(ctx, 1200, nil)
	if _, err := g.Begin(ctx, "u1"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("over the daily token budget: %v", err)
	}
}

func TestGuardStrikesLockOutInjectors(t *testing.T) {
	g, _ := newGuard(t, GuardConfig{StrikesPerHour: 2})
	ctx := context.Background()
	g.Strike(ctx, "u1")
	if err := g.Admit(ctx, "u1"); err != nil {
		t.Fatal(err)
	}
	g.Strike(ctx, "u1")
	var lim *LimitError
	if err := g.Admit(ctx, "u1"); !errors.As(err, &lim) || lim.Scope != "strikes" {
		t.Fatalf("after 2 strikes: %v", err)
	}
	if err := g.Admit(ctx, "u2"); err != nil {
		t.Fatalf("another player: %v", err)
	}
}

func TestGuardRefundAllowance(t *testing.T) {
	g, _ := newGuard(t, GuardConfig{RefundsPerDay: 2})
	ctx := context.Background()
	if !g.AllowRefund(ctx, "u1") || !g.AllowRefund(ctx, "u1") {
		t.Fatal("first two refunds should be allowed")
	}
	if g.AllowRefund(ctx, "u1") {
		t.Fatal("third refund of the day should be refused")
	}
	if !g.AllowRefund(ctx, "u2") {
		t.Fatal("another player's allowance is separate")
	}
}

func TestGuardCircuitBreaker(t *testing.T) {
	g, _ := newGuard(t, GuardConfig{MaxConcurrent: 2, BreakerFailures: 3, BreakerOpenFor: time.Minute})
	ctx := context.Background()
	now := time.Now()
	g.now = func() time.Time { return now }
	g.Finish(ctx, 0, llm.ErrUpstream)
	g.Finish(ctx, 0, llm.ErrBadOutput) // an unusable answer is not an outage and resets the count
	g.Finish(ctx, 0, llm.ErrUpstream)
	g.Finish(ctx, 0, llm.ErrTimeout)
	if release, err := g.Begin(ctx, "u1"); err != nil {
		t.Fatalf("breaker opened too early: %v", err)
	} else {
		release()
	}
	g.Finish(ctx, 0, llm.ErrUpstream)
	if _, err := g.Begin(ctx, "u1"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("breaker should be open: %v", err)
	}
	now = now.Add(2 * time.Minute)
	if release, err := g.Begin(ctx, "u1"); err != nil {
		t.Fatalf("breaker should have closed: %v", err)
	} else {
		release()
	}
}

func TestGuardResetLimitsUnblocksOnePlayer(t *testing.T) {
	g, _ := newGuard(t, GuardConfig{MaxConcurrent: 5, UserDailyCalls: 1, StrikesPerHour: 1})
	ctx := context.Background()
	release, err := g.Begin(ctx, "u1")
	if err != nil {
		t.Fatal(err)
	}
	release()
	g.Strike(ctx, "u1")
	var lim *LimitError
	if _, err := g.Begin(ctx, "u1"); !errors.As(err, &lim) || lim.Scope != "daily" {
		t.Fatalf("over the daily allowance: %v", err)
	}
	if err := g.Admit(ctx, "u1"); !errors.As(err, &lim) || lim.Scope != "strikes" {
		t.Fatalf("struck out: %v", err)
	}
	if err := g.ResetLimits(ctx, "u1"); err != nil {
		t.Fatal(err)
	}
	if err := g.Admit(ctx, "u1"); err != nil {
		t.Fatalf("after reset admit: %v", err)
	}
	release, err = g.Begin(ctx, "u1")
	if err != nil {
		t.Fatalf("after reset begin: %v", err)
	}
	release()
}
