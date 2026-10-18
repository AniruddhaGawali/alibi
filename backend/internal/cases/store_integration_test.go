package cases_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"testing"

	"backend/internal/cases"
	"backend/internal/config"
	"backend/internal/db"
	"backend/internal/user"
	"github.com/jackc/pgx/v5/pgxpool"
)

// throwawayDatabase creates an empty alibi_test_<random> database on the server
// TEST_POSTGRES_URI points at, and drops it when the test ends.
func throwawayDatabase(t *testing.T, adminURI string) *pgxpool.Pool {
	t.Helper()
	var rnd [6]byte
	_, _ = rand.Read(rnd[:])
	name := "alibi_test_" + hex.EncodeToString(rnd[:])
	admin, err := db.ConnectPostgres(&config.Config{PostgresURI: adminURI, PGMaxConns: 2, ConnectTimeout: 5e9})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(context.Background(), "CREATE DATABASE "+name); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
		admin.Close()
	})
	u, _ := url.Parse(adminURI)
	u.Path = "/" + name
	pool, err := db.ConnectPostgres(&config.Config{PostgresURI: u.String(), PGMaxConns: 4, ConnectTimeout: 5e9})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// Runs against a real PostgreSQL when TEST_POSTGRES_URI is set (see internal/user).
func TestPGStore(t *testing.T) {
	uri := os.Getenv("TEST_POSTGRES_URI")
	if uri == "" {
		t.Skip("TEST_POSTGRES_URI not set")
	}
	ctx := context.Background()
	pool := throwawayDatabase(t, uri)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	store := cases.NewPGStore(pool)

	var story, hidden, other string
	mustScan := func(dst *string, sql string, args ...any) {
		t.Helper()
		if err := pool.QueryRow(ctx, sql, args...).Scan(dst); err != nil {
			t.Fatal(err)
		}
	}
	ins := `INSERT INTO stories (title, synopsis, main_plot, difficulty_level, is_published) VALUES ($1, 's', 'plot', 'easy', $2) RETURNING id`
	mustScan(&story, ins, "Case A", true)
	mustScan(&hidden, ins, "Draft", false)
	mustScan(&other, ins, "Case B", true)
	for i, name := range []string{"Second", "First"} { // order comes from display_order, not insert order
		_, err := pool.Exec(ctx, `INSERT INTO case_scenes (case_id, scene_name, image_url, display_order) VALUES ($1, $2, 'u', $3)`, story, name, 2-i)
		if err != nil {
			t.Fatal(err)
		}
	}
	for i, name := range []string{"Thomas", "Beatrice"} {
		_, err := pool.Exec(ctx, `INSERT INTO suspects (case_id, name, role, initial_testimony, is_culprit, display_order) VALUES ($1, $2, 'r', 't', $3, $4)`,
			story, name, name == "Beatrice", i)
		if err != nil {
			t.Fatal(err)
		}
	}
	// one culprit per case
	if _, err := pool.Exec(ctx, `INSERT INTO suspects (case_id, name, role, initial_testimony, is_culprit) VALUES ($1, 'Second culprit', 'r', 't', true)`, story); err == nil {
		t.Fatal("a second culprit in one case must be rejected")
	}

	b, err := store.Briefing(ctx, story)
	if err != nil || b.MainPlot != "plot" || len(b.Scenes) != 2 || b.Scenes[0].SceneName != "First" {
		t.Fatalf("briefing: %+v %v", b, err)
	}
	if _, err := store.Briefing(ctx, hidden); !errors.Is(err, cases.ErrNotFound) {
		t.Fatalf("unpublished briefing: %v", err)
	}
	sus, err := store.Suspects(ctx, story)
	if err != nil || len(sus) != 2 || sus[0].Name != "Thomas" {
		t.Fatalf("suspects: %+v %v", sus, err)
	}
	if _, err := store.Suspects(ctx, "00000000-0000-0000-0000-000000000000"); !errors.Is(err, cases.ErrNotFound) {
		t.Fatalf("unknown suspects: %v", err)
	}

	// Starting a case: lock, idempotent restart, refusal for another case.
	u, err := user.NewPGStore(pool).Create(ctx, "sherlock", "s@x.co", "h", 10)
	if err != nil {
		t.Fatal(err)
	}
	if l, err := store.Lock(ctx, u.ID); err != nil || l != nil {
		t.Fatalf("free user lock: %v %v", l, err)
	}
	first, err := store.Start(ctx, u.ID, story)
	if err != nil || first.StoryID != story || first.CaseID == "" {
		t.Fatalf("start: %+v %v", first, err)
	}
	again, err := store.Start(ctx, u.ID, story)
	if err != nil || again.CaseID != first.CaseID {
		t.Fatalf("restart same case: %+v %v", again, err)
	}
	if _, err := store.Start(ctx, u.ID, other); !errors.Is(err, cases.ErrLockedElsewhere) {
		t.Fatalf("start other: %v", err)
	}
	if _, err := store.Start(ctx, u.ID, hidden); !errors.Is(err, cases.ErrNotFound) {
		t.Fatalf("start unpublished: %v", err)
	}
	if _, err := store.Start(ctx, "00000000-0000-0000-0000-000000000000", story); !errors.Is(err, cases.ErrUserGone) {
		t.Fatalf("start for unknown user: %v", err)
	}
	if l, _ := store.Lock(ctx, u.ID); l == nil || l.StoryID != story {
		t.Fatalf("lock after start: %+v", l)
	}
}
