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
	// lockTTL is how long a run that died mid-way (SIGKILL, OOM) keeps others out;
	// a normal run releases it as soon as it ends.
	lockTTL = resetTimeout + time.Minute
	// doneTTL outlives the day a finished reset is recorded for, so a second replica
	// (or a restarted one whose clock is slightly off) never runs the same day twice.
	doneTTL = 48 * time.Hour
	// catchUpEvery is how often the API checks that today's reset has happened.
	catchUpEvery = 10 * time.Minute
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
	Sched  cron.Schedule // when the reset is due; nil: CatchUp does nothing
	now    func() time.Time
	sleep  func(time.Duration)
}

// Run is the cron job. Every API replica schedules it, so Redis keys by the
// calendar day (in Loc) make sure the reset runs once per day across all of them,
// for all users in a single update:
//
//   - done:cron:token-reset:<day> is written after a successful reset and stops any
//     later run that day;
//   - lock:cron:token-reset:<day> only keeps two runs from overlapping. It is
//     released when the run ends, and expires on its own if the process dies
//     mid-run, so a crash never skips the day.
//
// A failed run releases the lock and writes no done marker, so the next catch-up
// check (CatchUp) tries again.
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
	done := "done:cron:token-reset:" + day
	finished, err := j.Redis.Exists(ctx, done).Result()
	if err != nil {
		utils.LogError("token reset: done check failed; skipping this run", "day", day, "err", err)
		return
	}
	if finished > 0 {
		utils.LogInfo("token reset: already done for today", "day", day)
		return
	}
	ok, err := j.Redis.SetNX(ctx, lock, "1", lockTTL).Result()
	if err != nil {
		utils.LogError("token reset: lock failed; skipping this run", "day", day, "err", err)
		return
	}
	if !ok {
		utils.LogInfo("token reset: already running", "day", day)
		return
	}
	// Always give the lock back, also after a recovered panic.
	defer func() {
		if err := j.Redis.Del(context.WithoutCancel(ctx), lock).Err(); err != nil {
			utils.LogError("token reset: could not release the lock", "day", day, "err", err)
		}
	}()

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
		return // no done marker: a later catch-up check (or another replica) tries again
	}
	if err := j.Redis.Set(context.WithoutCancel(ctx), done, "1", doneTTL).Err(); err != nil {
		utils.LogError("token reset: could not record the day as done", "day", day, "err", err)
	}
	sleep(secondSweep)
	if _, err := j.Wallet.ClearCache(ctx); err != nil {
		utils.LogError("token reset: second cache sweep failed", "day", day, "err", err)
	}
}

// due reports whether today's reset time (in Loc) has already passed.
func (j *TokenReset) due(now time.Time) bool {
	if j.Sched == nil {
		return false
	}
	local := now.In(j.Loc)
	midnight := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, j.Loc)
	first := j.Sched.Next(midnight.Add(-time.Second))
	return !first.After(local) && first.Before(midnight.AddDate(0, 0, 1))
}

// CatchUp runs the reset when it is due today but has not happened: the API was
// down or asleep at the cron minute, or an earlier run failed. robfig/cron does
// not replay missed runs, so this is what makes the reset self-healing. It runs
// at startup and every catchUpEvery.
func (j *TokenReset) CatchUp() {
	now := time.Now
	if j.now != nil {
		now = j.now
	}
	if !j.due(now()) {
		return
	}
	j.Run() // does nothing when today is already done
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
	job := &TokenReset{Tokens: int64(tokens), Wallet: w, Redis: rdb, Loc: loc, Sched: sched}
	c.Schedule(sched, job)
	c.Schedule(cron.Every(catchUpEvery), cron.FuncJob(job.CatchUp)) // heals a missed or failed run
	c.Start()
	go job.CatchUp() // the API may have been down at the cron minute
	utils.LogInfo("token reset scheduled", "cron", spec, "timezone", tz, "tokens", tokens,
		"next_run", sched.Next(time.Now().In(loc)))
	return c, nil
}
