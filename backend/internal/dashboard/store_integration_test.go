package dashboard_test

import (
	"backend/internal/testdb"
	"context"
	"errors"
	"os"
	"testing"

	"backend/internal/cases"
	"backend/internal/dashboard"
	"backend/internal/db"
	"backend/internal/user"
)

// Runs against a real PostgreSQL when TEST_POSTGRES_URI is set (see internal/user
// for how to start one).
func TestPGStore(t *testing.T) {
	uri := os.Getenv("TEST_POSTGRES_URI")
	if uri == "" {
		t.Skip("TEST_POSTGRES_URI not set")
	}
	ctx := context.Background()
	pool := testdb.New(t, uri)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}

	users := user.NewPGStore(pool)
	store := dashboard.NewPGStore(pool)

	// Stories: only published ones, newest first, empty slice when none.
	got, err := store.ListStories(ctx)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("empty stories: %v %v", got, err)
	}
	var s1 string
	insert := func(title string, published bool) string {
		var id string
		err := pool.QueryRow(ctx, `INSERT INTO stories (title, synopsis, thumbnail_url, difficulty_level, reward_trophies, is_published)
			VALUES ($1, 'syn', 'http://img', 'hard', 5, $2) RETURNING id`, title, published).Scan(&id)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	s1 = insert("first", true)
	insert("hidden", false)
	insert("second", true)
	got, err = store.ListStories(ctx)
	if err != nil || len(got) != 2 || got[0].Title != "second" || got[1].Title != "first" {
		t.Fatalf("stories: %+v %v", got, err)
	}
	if got[0].DifficultyLevel != "hard" || got[0].RewardTrophies != 5 || got[0].ThumbnailURL != "http://img" {
		t.Fatalf("story fields: %+v", got[0])
	}

	// Users, trophies and the fallback leaderboard.
	ann, err := users.Create(ctx, "ann", "ann@example.com", "hash", 10)
	if err != nil {
		t.Fatal(err)
	}
	bob, _ := users.Create(ctx, "bob", "bob@example.com", "hash", 10)
	_, _ = users.Create(ctx, "cy", "cy@example.com", "hash", 10)
	for id, n := range map[string]int{ann.ID: 9, bob.ID: 20} {
		if _, err := pool.Exec(ctx, "UPDATE users SET trophies = $1 WHERE id = $2", n, id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, "UPDATE users SET cases_solved = 4 WHERE id = $1", ann.ID); err != nil {
		t.Fatal(err)
	}
	if st, err := store.Stats(ctx, ann.ID); err != nil || st.Trophies != 9 || st.CasesSolved != 4 {
		t.Fatalf("stats: %+v %v", st, err)
	}
	if _, err := store.Stats(ctx, "99999999-9999-9999-9999-999999999999"); !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("unknown user: %v", err)
	}
	top, err := store.TopPlayers(ctx, 10)
	if err != nil || len(top) != 2 || top[0].Username != "bob" || top[1].Username != "ann" { // cy has 0 trophies
		t.Fatalf("top players: %+v %v", top, err)
	}
	if top, _ = store.TopPlayers(ctx, 1); len(top) != 1 {
		t.Fatalf("limit not applied: %+v", top)
	}
	// Active case lock.
	if c, err := store.ActiveCase(ctx, ann.ID); err != nil || c != nil {
		t.Fatalf("free user: %v %v", c, err)
	}
	cs := cases.NewPGStore(pool)
	lock, err := cs.Start(ctx, ann.ID, s1)
	if err != nil {
		t.Fatal(err)
	}
	if c, err := store.ActiveCase(ctx, ann.ID); err != nil || c == nil || c.CaseID != lock.CaseID || c.StoryID != s1 || c.Title != "first" {
		t.Fatalf("locked user: %+v %v", c, err)
	}
	if c, _ := store.ActiveCase(ctx, bob.ID); c != nil {
		t.Fatalf("lock leaked to another user: %+v", c)
	}
	// Unpublishing the story frees the player from the dashboard's point of view.
	if _, err := pool.Exec(ctx, "UPDATE stories SET is_published = false WHERE id = $1", s1); err != nil {
		t.Fatal(err)
	}
	if c, err := store.ActiveCase(ctx, ann.ID); err != nil || c != nil {
		t.Fatalf("lock on an unpublished story still shown: %+v %v", c, err)
	}
}
