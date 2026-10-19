package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
)

type fakeWallet struct {
	resets, clears int
	tokens         int64
	err            error
}

func (f *fakeWallet) ResetAll(_ context.Context, tokens int64) (int64, error) {
	f.resets++
	f.tokens = tokens
	return 1, f.err
}
func (f *fakeWallet) ClearCache(context.Context) (int64, error) { f.clears++; return 0, nil }

func newJob(t *testing.T, w *fakeWallet, at time.Time) (*TokenReset, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return &TokenReset{
		Tokens: 10, Wallet: w, Redis: rdb, Loc: time.UTC,
		now: func() time.Time { return at }, sleep: func(time.Duration) {},
	}, mr
}

func TestTokenResetRunsOncePerDay(t *testing.T) {
	w := &fakeWallet{}
	midnight := time.Date(2026, 10, 19, 0, 0, 1, 0, time.UTC)
	job, mr := newJob(t, w, midnight)

	job.Run()
	if w.resets != 1 || w.tokens != 10 || w.clears != 1 {
		t.Fatalf("resets=%d tokens=%d clears=%d", w.resets, w.tokens, w.clears)
	}
	// Another replica firing the same minute, or any time later that day (a restart,
	// a misconfigured second schedule), does nothing.
	for _, later := range []time.Duration{20 * time.Second, 7 * time.Hour, 23*time.Hour + 58*time.Minute} {
		job.now = func() time.Time { return midnight.Add(later) }
		job.Run()
	}
	if w.resets != 1 {
		t.Fatalf("reset ran again the same day: %d", w.resets)
	}
	if !mr.Exists("done:cron:token-reset:2026-10-19") || mr.Exists("lock:cron:token-reset:2026-10-19") {
		t.Fatalf("want the done marker and no lock, keys=%v", mr.Keys())
	}
	// The next day runs again.
	job.now = func() time.Time { return midnight.Add(24 * time.Hour) }
	job.Run()
	if w.resets != 2 {
		t.Fatalf("next day: %d", w.resets)
	}
}

func TestTokenResetDayFollowsTimezone(t *testing.T) {
	ist, _ := time.LoadLocation("Asia/Kolkata")
	w := &fakeWallet{}
	// 20:00 UTC on the 18th is 01:30 on the 19th in Kolkata.
	job, mr := newJob(t, w, time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC))
	job.Loc = ist
	job.Run()
	if !mr.Exists("done:cron:token-reset:2026-10-19") || w.resets != 1 {
		t.Fatalf("keys=%v resets=%d", mr.Keys(), w.resets)
	}
}

func TestTokenResetRetriesThenReleasesLock(t *testing.T) {
	w := &fakeWallet{err: errors.New("postgres down")}
	job, mr := newJob(t, w, time.Now())
	job.Run()
	if w.resets != attempts || w.clears != 0 {
		t.Fatalf("resets=%d clears=%d", w.resets, w.clears)
	}
	if len(mr.Keys()) != 0 {
		t.Fatalf("lock should be released after a failed day: %v", mr.Keys())
	}
	// ...so a later run the same day can still do it.
	w.err, w.resets = nil, 0
	job.Run()
	if w.resets != 1 {
		t.Fatalf("retry after release: %d", w.resets)
	}
}

func TestTokenResetRedisDownSkips(t *testing.T) {
	w := &fakeWallet{}
	job, mr := newJob(t, w, time.Now())
	mr.Close()
	job.Run() // must not panic or reset without the lock
	if w.resets != 0 {
		t.Fatalf("reset without lock: %d", w.resets)
	}
}

func TestStartTokenResetValidatesConfig(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	ist, _ := time.LoadLocation("Asia/Kolkata")
	fixed := func(s Schedule, err error) func(context.Context) (Schedule, error) {
		return func(context.Context) (Schedule, error) { return s, err }
	}
	if _, err := StartTokenReset(fixed(Schedule{Tokens: 10, Hour: 25, Loc: time.UTC}, nil), &fakeWallet{}, rdb); err == nil {
		t.Fatal("bad hour accepted")
	}
	if _, err := StartTokenReset(fixed(Schedule{Tokens: 10}, nil), &fakeWallet{}, rdb); err == nil {
		t.Fatal("missing timezone accepted")
	}
	if _, err := StartTokenReset(fixed(Schedule{}, errors.New("db down")), &fakeWallet{}, rdb); err == nil {
		t.Fatal("load error ignored")
	}
	c, err := StartTokenReset(fixed(Schedule{Tokens: 10, Hour: 4, Minute: 30, Loc: ist}, nil), &fakeWallet{}, rdb)
	if err != nil {
		t.Fatal(err)
	}
	<-c.Stop().Done()
}

// An edit in the admin panel (Load) moves the reset time and the token count at the next check.
func TestTokenResetFollowsLoadedSchedule(t *testing.T) {
	w := &fakeWallet{}
	at := time.Date(2026, 10, 19, 5, 0, 0, 0, time.UTC)
	job, _ := newJob(t, w, at)
	sched := Schedule{Tokens: 7, Hour: 6, Minute: 0, Loc: time.UTC}
	job.Load = func(context.Context) (Schedule, error) { return sched, nil }

	job.CatchUp() // 05:00, due at 06:00
	if w.resets != 0 {
		t.Fatalf("ran before the reset time: %d", w.resets)
	}
	sched.Hour = 4 // the admin moves it earlier: it is now overdue
	job.CatchUp()
	if w.resets != 1 || w.tokens != 7 {
		t.Fatalf("resets=%d tokens=%d", w.resets, w.tokens)
	}
}

// A run killed mid-way leaves only the short lock behind; the day is not marked done.
func TestTokenResetCrashDoesNotSkipTheDay(t *testing.T) {
	w := &fakeWallet{}
	at := time.Date(2026, 10, 19, 0, 0, 1, 0, time.UTC)
	job, mr := newJob(t, w, at)
	if err := job.Redis.Set(context.Background(), "lock:cron:token-reset:2026-10-19", "1", lockTTL).Err(); err != nil {
		t.Fatal(err)
	}
	job.Run()
	if w.resets != 0 {
		t.Fatal("ran while another run holds the lock")
	}
	mr.FastForward(lockTTL + time.Second) // the crashed run's lock expires
	job.Run()
	if w.resets != 1 {
		t.Fatalf("reset after the stale lock expired: %d", w.resets)
	}
}

// The API was down at the cron minute: the catch-up check does today's reset.
func TestTokenResetCatchUp(t *testing.T) {
	sched, err := cron.ParseStandard("30 4 * * *")
	if err != nil {
		t.Fatal(err)
	}
	w := &fakeWallet{}
	at := time.Date(2026, 10, 19, 4, 0, 0, 0, time.UTC) // before today's 04:30 reset
	job, _ := newJob(t, w, at)
	job.Sched = sched
	job.CatchUp()
	if w.resets != 0 {
		t.Fatal("caught up before the reset time")
	}
	job.now = func() time.Time { return at.Add(2 * time.Hour) } // 06:00, the API just started
	job.CatchUp()
	if w.resets != 1 {
		t.Fatalf("missed reset not run: %d", w.resets)
	}
	job.CatchUp()
	if w.resets != 1 {
		t.Fatal("ran twice")
	}
}
