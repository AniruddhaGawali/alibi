package accusation_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"

	"backend/internal/accusation"
	"backend/internal/cases"
	"backend/internal/dashboard"
	"backend/internal/db"
	"backend/internal/testdb"
	"backend/internal/user"
)

// Runs against a real PostgreSQL when TEST_POSTGRES_URI is set (see internal/user).
func TestPGStoreAccuse(t *testing.T) {
	uri := os.Getenv("TEST_POSTGRES_URI")
	if uri == "" {
		t.Skip("TEST_POSTGRES_URI not set")
	}
	ctx := context.Background()
	pool := testdb.New(t, uri)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	store := accusation.NewPGStore(pool)
	caseStore := cases.NewPGStore(pool)

	var story, other, culprit, innocent, foreign string
	scan := func(dst *string, sql string, args ...any) {
		t.Helper()
		if err := pool.QueryRow(ctx, sql, args...).Scan(dst); err != nil {
			t.Fatal(err)
		}
	}
	ins := `INSERT INTO stories (title, synopsis, difficulty_level, reward_trophies, penalty_trophies, resolution) VALUES ($1, 's', 'easy', 20, 8, $2) RETURNING id`
	scan(&story, ins, "Case A", "It was Beatrice.")
	scan(&other, ins, "Case B", "")
	sus := `INSERT INTO suspects (case_id, name, role, initial_testimony, is_culprit) VALUES ($1, $2, 'r', 't', $3) RETURNING id`
	scan(&culprit, sus, story, "Beatrice", true)
	scan(&innocent, sus, story, "Thomas", false)
	scan(&foreign, sus, other, "Stranger", true)

	users := user.NewPGStore(pool)
	newUser := func(name string, trophies int) string {
		t.Helper()
		u, err := users.Create(ctx, name, name+"@x.co", "h", 10)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `UPDATE users SET trophies = $1 WHERE id = $2`, trophies, u.ID); err != nil {
			t.Fatal(err)
		}
		return u.ID
	}
	trophies := func(uid string) (n, solved int) {
		t.Helper()
		if err := pool.QueryRow(ctx, `SELECT trophies, cases_solved FROM users WHERE id = $1`, uid).Scan(&n, &solved); err != nil {
			t.Fatal(err)
		}
		return
	}
	locked := func(uid string) bool {
		var n int
		_ = pool.QueryRow(ctx, `SELECT count(*) FROM active_case_locks WHERE user_id = $1`, uid).Scan(&n)
		return n == 1
	}

	// Not locked: refused, nothing changes.
	a := newUser("ann", 10)
	if _, err := store.Accuse(ctx, a, story, culprit); !errors.Is(err, accusation.ErrNotLocked) {
		t.Fatalf("not locked: %v", err)
	}

	// Locked into a different case: still refused for this one, and the lock stays.
	if _, err := caseStore.Start(ctx, a, other); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Accuse(ctx, a, story, culprit); !errors.Is(err, accusation.ErrNotLocked) || !locked(a) {
		t.Fatalf("other case lock: %v locked=%v", err, locked(a))
	}

	// A suspect from another case is rejected and the lock is kept.
	b := newUser("bob", 10)
	if _, err := caseStore.Start(ctx, b, story); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Accuse(ctx, b, story, foreign); !errors.Is(err, accusation.ErrBadSuspect) || !locked(b) {
		t.Fatalf("foreign suspect: %v locked=%v", err, locked(b))
	}
	if n, _ := trophies(b); n != 10 {
		t.Fatalf("trophies = %d after a rejected accusation", n)
	}

	// Correct: +reward, case counted, lock released, truth revealed.
	out, err := store.Accuse(ctx, b, story, culprit)
	if err != nil || !out.Correct || out.Trophies != 30 || out.Change != 20 || out.Resolution != "It was Beatrice." || out.CulpritName != "Beatrice" {
		t.Fatalf("win: %+v %v", out, err)
	}
	if n, solved := trophies(b); n != 30 || solved != 1 || locked(b) {
		t.Fatalf("after win: trophies=%d solved=%d locked=%v", n, solved, locked(b))
	}
	if _, err := store.Accuse(ctx, b, story, culprit); !errors.Is(err, accusation.ErrNotLocked) {
		t.Fatalf("second accusation must be refused: %v", err)
	}

	// The verdict is stored: the case cannot be started again and the result can be read back.
	if _, err := caseStore.Start(ctx, b, story); !errors.Is(err, cases.ErrClosed) {
		t.Fatalf("restarting a closed case: %v", err)
	}
	if res, err := caseStore.Result(ctx, b, story); err != nil || !res.Correct || res.CulpritName != "Beatrice" || res.Resolution != "It was Beatrice." || res.Trophies != 30 {
		t.Fatalf("stored result: %+v %v", res, err)
	}
	if _, err := caseStore.Result(ctx, a, story); !errors.Is(err, cases.ErrNotFound) {
		t.Fatalf("an open case must not reveal a result: %v", err)
	}

	// The dashboard cards say so too: closed for bob, still open for ann.
	outcomeOf := func(uid string) *string {
		t.Helper()
		list, err := dashboard.NewPGStore(pool).ListStories(ctx, uid)
		if err != nil {
			t.Fatal(err)
		}
		for _, st := range list {
			if st.ID == story {
				return st.Outcome
			}
		}
		t.Fatal("story missing from the dashboard list")
		return nil
	}
	if o := outcomeOf(b); o == nil || *o != "solved" {
		t.Fatalf("bob's card outcome = %v, want solved", o)
	}
	if o := outcomeOf(a); o != nil {
		t.Fatalf("ann's card outcome = %v, want nil", *o)
	}

	// Wrong: -penalty, clamped at zero, no solve, lock released.
	c := newUser("cat", 3)
	if _, err := caseStore.Start(ctx, c, story); err != nil {
		t.Fatal(err)
	}
	out, err = store.Accuse(ctx, c, story, innocent)
	if err != nil || out.Correct || out.Trophies != 0 || out.Change != -3 {
		t.Fatalf("loss: %+v %v", out, err)
	}
	if n, solved := trophies(c); n != 0 || solved != 0 || locked(c) {
		t.Fatalf("after loss: trophies=%d solved=%d locked=%v", n, solved, locked(c))
	}

	// Concurrent accusations pay once.
	d := newUser("dan", 0)
	if _, err := caseStore.Start(ctx, d, story); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wins := make(chan struct{}, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := store.Accuse(ctx, d, story, culprit); err == nil {
				wins <- struct{}{}
			}
		}()
	}
	wg.Wait()
	close(wins)
	if len(wins) != 1 {
		t.Fatalf("%d accusations succeeded, want 1", len(wins))
	}
	if n, _ := trophies(d); n != 20 {
		t.Fatalf("trophies = %d, want 20", n)
	}
}

func TestHintsCostOnlyOnAWin(t *testing.T) {
	uri := os.Getenv("TEST_POSTGRES_URI")
	if uri == "" {
		t.Skip("TEST_POSTGRES_URI not set")
	}
	ctx := context.Background()
	pool := testdb.New(t, uri)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	store := accusation.NewPGStore(pool)
	caseStore := cases.NewPGStore(pool)
	users := user.NewPGStore(pool)

	var story, culprit, innocent string
	if err := pool.QueryRow(ctx, `INSERT INTO stories (title, synopsis, difficulty_level, reward_trophies, penalty_trophies, hints)
		VALUES ('H', 's', 'easy', 5, 2, ARRAY['one','two','three']) RETURNING id`).Scan(&story); err != nil {
		t.Fatal(err)
	}
	sus := `INSERT INTO suspects (case_id, name, role, initial_testimony, is_culprit) VALUES ($1, $2, 'r', 't', $3) RETURNING id`
	if err := pool.QueryRow(ctx, sus, story, "Guilty", true).Scan(&culprit); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, sus, story, "Innocent", false).Scan(&innocent); err != nil {
		t.Fatal(err)
	}
	newUser := func(name string) string {
		t.Helper()
		u, err := users.Create(ctx, name, name+"@x.co", "h", 10)
		if err != nil {
			t.Fatal(err)
		}
		return u.ID
	}

	// Not playing the case: no hint, and the briefing carries none.
	free := newUser("free")
	if _, err := caseStore.RevealHint(ctx, free, story); !errors.Is(err, cases.ErrNotLocked) {
		t.Fatalf("reveal without a lock: %v", err)
	}
	if b, err := caseStore.Briefing(ctx, free, story); err != nil || b.HintsTotal != 3 || b.HintsUsed != 0 || len(b.Hints) != 0 {
		t.Fatalf("briefing without a lock: %+v %v", b, err)
	}

	// Hints come in order, cost their number, and the briefing shows only the revealed ones.
	p := newUser("pat")
	if _, err := caseStore.Start(ctx, p, story); err != nil {
		t.Fatal(err)
	}
	for i, want := range []string{"one", "two"} {
		h, err := caseStore.RevealHint(ctx, p, story)
		if err != nil || h.Hint != want || h.Number != i+1 || h.Cost != i+1 {
			t.Fatalf("hint %d: %+v %v", i+1, h, err)
		}
	}
	if b, _ := caseStore.Briefing(ctx, p, story); b.HintsUsed != 2 || len(b.Hints) != 2 || b.Hints[1] != "two" {
		t.Fatalf("briefing after 2 hints: %+v", b)
	}
	if _, err := caseStore.RevealHint(ctx, p, story); err != nil {
		t.Fatal(err)
	}
	if _, err := caseStore.RevealHint(ctx, p, story); !errors.Is(err, cases.ErrNoHints) {
		t.Fatalf("fourth hint: %v", err)
	}

	// Three hints cost 1+2+3 = 6, more than the reward of 5: the win pays 0, never negative.
	out, err := store.Accuse(ctx, p, story, culprit)
	if err != nil || !out.Correct || out.Change != 0 || out.Trophies != 0 || out.HintPenalty != 5 {
		t.Fatalf("win with 3 hints on a reward of 5: %+v %v", out, err)
	}
	if res, err := caseStore.Result(ctx, p, story); err != nil || res.HintPenalty != 5 {
		t.Fatalf("stored result: %+v %v", res, err)
	}

	// Two hints cost 3 of 5.
	q := newUser("quinn")
	if _, err := caseStore.Start(ctx, q, story); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := caseStore.RevealHint(ctx, q, story); err != nil {
			t.Fatal(err)
		}
	}
	if out, err := store.Accuse(ctx, q, story, culprit); err != nil || out.Change != 2 || out.HintPenalty != 3 {
		t.Fatalf("win with 2 hints: %+v %v", out, err)
	}

	// Hints cost nothing extra on a loss: just the case penalty.
	r := newUser("rae")
	if _, err := pool.Exec(ctx, `UPDATE users SET trophies = 10 WHERE id = $1`, r); err != nil {
		t.Fatal(err)
	}
	if _, err := caseStore.Start(ctx, r, story); err != nil {
		t.Fatal(err)
	}
	if _, err := caseStore.RevealHint(ctx, r, story); err != nil {
		t.Fatal(err)
	}
	if out, err := store.Accuse(ctx, r, story, innocent); err != nil || out.Correct || out.Change != -2 || out.HintPenalty != 0 {
		t.Fatalf("loss with a hint: %+v %v", out, err)
	}
}

func TestHintPenalty(t *testing.T) {
	for n, want := range map[int32]int32{0: 0, 1: 1, 2: 3, 3: 6} {
		if got := accusation.HintPenalty(n); got != want {
			t.Errorf("HintPenalty(%d) = %d, want %d", n, got, want)
		}
	}
}
