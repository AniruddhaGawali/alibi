// Package jobs holds the API's scheduled jobs, run in-process by
// github.com/robfig/cron/v3 (no external cron).
package jobs

import (
	"context"
	"fmt"
	"sync"
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
	// checkEvery is how often the API checks whether today's reset is due. The
	// schedule is edited from the admin panel, so the job re-reads it every check
	// instead of registering a fixed cron entry; the reset runs at most this late.
	checkEvery = time.Minute
	// attempts is how many times the Postgres reset is tried before giving up.
	attempts = 3
	// secondSweep clears the cache again shortly after the reset, in case a
	// request read a pre-reset balance from Postgres and cached it just after
	// the first sweep.
	secondSweep = 5 * time.Second
)

// Schedule is the reset's configuration: Tokens at Hour:Minute in Loc.
type Schedule struct {
	Tokens       int
	Hour, Minute int
	Loc          *time.Location
}

// TokenReset resets every user's question tokens to Tokens (P4.2), once a day.
// Unused tokens never roll over: the balance is set, not topped up, so a player
// with 0 gets Tokens and a player with more than Tokens drops to it.
type TokenReset struct {
	Tokens int64
	Wallet Resetter
	Redis  *redis.Client
	Loc    *time.Location
	Sched  cron.Schedule // when the reset is due; nil: CatchUp does nothing
	// Load, when set, is read before every CatchUp check and replaces Tokens, Loc
	// and Sched, so an edit in the admin panel applies from the next check.
	Load func(context.Context) (Schedule, error)
	mu   sync.Mutex // one check at a time; Load rewrites the fields above
	// doneDay is the last day this process saw finished, so the checks every minute
	// after the reset ask Redis once a day instead of once a minute.
	doneDay string
	now     func() time.Time
	sleep   func(time.Duration)
}

// apply sets the job's fields from s.
func (j *TokenReset) apply(s Schedule) error {
	if s.Loc == nil {
		return fmt.Errorf("token reset: no timezone")
	}
	sched, err := cron.ParseStandard(fmt.Sprintf("%d %d * * *", s.Minute, s.Hour))
	if err != nil {
		return fmt.Errorf("token reset time %02d:%02d: %w", s.Hour, s.Minute, err)
	}
	j.Tokens, j.Loc, j.Sched = int64(s.Tokens), s.Loc, sched
	return nil
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
	if day == j.doneDay {
		return
	}
	lock := "lock:cron:token-reset:" + day
	done := "done:cron:token-reset:" + day
	finished, err := j.Redis.Exists(ctx, done).Result()
	if err != nil {
		utils.LogError("token reset: done check failed; skipping this run", "day", day, "err", err)
		return
	}
	if finished > 0 {
		utils.LogInfo("token reset: already done for today", "day", day)
		j.doneDay = day
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
	j.doneDay = day
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

// CatchUp runs the reset when today's reset time has passed and today is not done
// yet. It runs at startup and every checkEvery, so it is both the schedule and what
// makes the reset self-healing: a run missed while the API was down, or one that
// failed, happens at the next check.
func (j *TokenReset) CatchUp() {
	if !j.mu.TryLock() {
		return // the previous check is still running
	}
	defer j.mu.Unlock()
	if j.Load != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		s, err := j.Load(ctx)
		cancel()
		if err == nil {
			err = j.apply(s)
		}
		if err != nil {
			utils.LogError("token reset: could not load the schedule; skipping this check", "err", err)
			return
		}
	}
	now := time.Now
	if j.now != nil {
		now = j.now
	}
	if !j.due(now()) {
		return
	}
	j.Run() // does nothing when today is already done
}

// StartTokenReset checks every checkEvery whether the daily reset is due, reading
// the schedule from load each time (the admin panel can change it), and starts
// the scheduler on its own goroutine. Stop the returned scheduler on shutdown.
func StartTokenReset(load func(context.Context) (Schedule, error), w Resetter, rdb *redis.Client) (*cron.Cron, error) {
	job := &TokenReset{Wallet: w, Redis: rdb, Load: load}
	s, err := load(context.Background())
	if err == nil {
		err = job.apply(s)
	}
	if err != nil {
		return nil, err
	}
	c := cron.New()
	c.Schedule(cron.Every(checkEvery), cron.FuncJob(job.CatchUp))
	c.Start()
	go job.CatchUp() // the API may have been down at the reset time
	utils.LogInfo("token reset scheduled", "at", fmt.Sprintf("%02d:%02d", s.Hour, s.Minute), "timezone", s.Loc.String(),
		"tokens", s.Tokens, "next_run", job.Sched.Next(time.Now().In(s.Loc)))
	return c, nil
}
