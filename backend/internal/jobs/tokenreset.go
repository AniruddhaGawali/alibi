// Package jobs holds the API's scheduled jobs, run in-process by
// github.com/robfig/cron/v3 (no external cron).
package jobs

import (
	"context"
	"fmt"
	"time"

	"backend/pkg/utils"
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
)

// Resetter performs the reset; *wallet.Service implements it.
type Resetter interface {
	ResetAll(ctx context.Context, tokens int64) (int64, error)
	ClearCache(ctx context.Context) (int64, error)
}

const (
	resetTimeout = 2 * time.Minute
	// lockTTL outlives the day it guards, so a second replica (or a restarted one
	// whose clock is slightly off) can never run the same day's reset again.
	lockTTL = 48 * time.Hour
	// attempts is how many times the Postgres reset is tried before giving up.
	attempts = 3
	// secondSweep clears the cache again shortly after the reset, in case a
	// request read a pre-reset balance from Postgres and cached it just after
	// the first sweep.
	secondSweep = 5 * time.Second
)

// TokenReset resets every user's question tokens to Tokens (P4.2), once a day.
// Unused tokens never roll over: the balance is set, not topped up, so a player
// with 0 gets Tokens and a player with more than Tokens drops to it.
type TokenReset struct {
	Tokens int64
	Wallet Resetter
	Redis  *redis.Client
	Loc    *time.Location
	now    func() time.Time
	sleep  func(time.Duration)
}

// Run is the cron job. Every API replica schedules it, so a Redis lock keyed by
// the calendar day (in Loc) makes sure the reset runs once per day across all of
// them, for all users in a single update. A failed run releases the lock.
func (j *TokenReset) Run() {
	defer func() {
		if r := recover(); r != nil {
			utils.LogError("token reset panicked", "panic", fmt.Sprint(r))
		}
	}()
	now, sleep := time.Now, time.Sleep
	if j.now != nil {
		now = j.now
	}
	if j.sleep != nil {
		sleep = j.sleep
	}
	ctx, cancel := context.WithTimeout(context.Background(), resetTimeout)
	defer cancel()

	day := now().In(j.Loc).Format(time.DateOnly)
	lock := "lock:cron:token-reset:" + day
	ok, err := j.Redis.SetNX(ctx, lock, "1", lockTTL).Result()
	if err != nil {
		utils.LogError("token reset: lock failed; skipping this run", "day", day, "err", err)
		return
	}
	if !ok {
		utils.LogInfo("token reset: already done or running for today", "day", day)
		return
	}

	var resetErr error
	for i := 1; i <= attempts; i++ {
		if _, resetErr = j.Wallet.ResetAll(ctx, j.Tokens); resetErr == nil {
			break
		}
		utils.LogError("token reset failed", "day", day, "attempt", i, "err", resetErr)
		if i < attempts {
			sleep(time.Duration(i) * 5 * time.Second)
		}
	}
	if resetErr != nil {
		// Give the lock back so a manual run (or another replica's retry) can still do today's reset.
		if err := j.Redis.Del(context.WithoutCancel(ctx), lock).Err(); err != nil {
			utils.LogError("token reset: could not release the lock", "day", day, "err", err)
		}
		return
	}
	sleep(secondSweep)
	if _, err := j.Wallet.ClearCache(ctx); err != nil {
		utils.LogError("token reset: second cache sweep failed", "day", day, "err", err)
	}
}

// StartTokenReset schedules the daily reset with a standard 5-field cron spec
// (e.g. "0 0 * * *" for midnight) read in the IANA zone tz, and starts the
// scheduler on its own goroutine. Stop the returned scheduler on shutdown.
func StartTokenReset(spec, tz string, tokens int, w Resetter, rdb *redis.Client) (*cron.Cron, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return nil, fmt.Errorf("TOKEN_RESET_TIMEZONE: %w", err)
	}
	sched, err := cron.ParseStandard(spec)
	if err != nil {
		return nil, fmt.Errorf("TOKEN_RESET_CRON %q: %w", spec, err)
	}
	c := cron.New(cron.WithLocation(loc))
	c.Schedule(sched, &TokenReset{Tokens: int64(tokens), Wallet: w, Redis: rdb, Loc: loc})
	c.Start()
	utils.LogInfo("token reset scheduled", "cron", spec, "timezone", tz, "tokens", tokens,
		"next_run", sched.Next(time.Now().In(loc)))
	return c, nil
}
