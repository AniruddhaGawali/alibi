package interrogation_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"testing"

	"backend/internal/config"
	"backend/internal/db"
	"backend/internal/interrogation"
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
	s := interrogation.NewPGStore(pool)

	var story, hidden string
	if err := pool.QueryRow(ctx, `INSERT INTO stories (title, synopsis, main_plot, difficulty_level, llm_model)
		VALUES ('Case', 'syn', 'plot', 'easy', 'google/gemma-2-27b-it') RETURNING id`).Scan(&story); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO stories (title, synopsis, difficulty_level, is_published)
		VALUES ('Draft', 'syn', 'easy', false) RETURNING id`).Scan(&hidden); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO suspects (case_id, name, role, initial_testimony, is_culprit, display_order,
			relationship_to_victim, persona, public_alibi, hidden_truth, confession_triggers)
		VALUES ($1, 'Beatrice', 'The Maid', 't', true, 2, 'his maid', 'quiet', 'upstairs', 'did it', 'asked who was last'),
		       ($1, 'Thomas', 'The Butler', 't', false, 1, 'his butler', 'loyal', 'pantry', 'saw her', 'pressed')`, story); err != nil {
		t.Fatal(err)
	}

	c, err := s.Case(ctx, story)
	if err != nil || c.Model != "google/gemma-2-27b-it" || c.Title != "Case" || c.Plot != "plot" {
		t.Fatalf("case: %+v %v", c, err)
	}
	if _, err := s.Case(ctx, hidden); !errors.Is(err, interrogation.ErrNotFound) {
		t.Fatalf("unpublished case: %v", err)
	}
	sus, err := s.Suspects(ctx, story)
	if err != nil || len(sus) != 2 || sus[0].Name != "Thomas" || sus[1].Name != "Beatrice" {
		t.Fatalf("suspects: %+v %v", sus, err)
	}
	b := sus[1].Persona
	if !b.IsCulprit || b.Relationship != "his maid" || b.Persona != "quiet" || b.PublicAlibi != "upstairs" ||
		b.HiddenTruth != "did it" || b.ConfessionTriggers != "asked who was last" {
		t.Fatalf("persona: %+v", b)
	}

	u, err := user.NewPGStore(pool).Create(ctx, "ann", "ann@example.com", "hash", 10)
	if err != nil {
		t.Fatal(err)
	}
	if l, err := s.Lock(ctx, u.ID); err != nil || l != nil {
		t.Fatalf("free user: %+v %v", l, err)
	}
	const play = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	if _, err := pool.Exec(ctx, "INSERT INTO active_case_locks (user_id, case_id, story_id) VALUES ($1, $2, $3)", u.ID, play, story); err != nil {
		t.Fatal(err)
	}
	if l, err := s.Lock(ctx, u.ID); err != nil || l == nil || l.CaseID != play || l.StoryID != story {
		t.Fatalf("lock: %+v %v", l, err)
	}
}
