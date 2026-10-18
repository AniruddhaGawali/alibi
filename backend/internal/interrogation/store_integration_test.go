package interrogation_test

import (
	"backend/internal/testdb"
	"context"
	"errors"
	"os"
	"testing"

	"backend/internal/db"
	"backend/internal/interrogation"
	"backend/internal/user"
)

// Runs against a real PostgreSQL when TEST_POSTGRES_URI is set (see internal/user).
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
			relationship_to_victim, persona, public_alibi, hidden_truth, confession_triggers, confession_keywords)
		VALUES ($1, 'Beatrice', 'The Maid', 't', true, 2, 'his maid', 'quiet', 'upstairs', 'did it', 'asked who was last', ARRAY['last+study','seen+study']),
		       ($1, 'Thomas', 'The Butler', 't', false, 1, 'his butler', 'loyal', 'pantry', 'saw her', 'pressed', '{}')`, story); err != nil {
		t.Fatal(err)
	}

	c, err := s.Case(ctx, story)
	if err != nil || c.Model != "google/gemma-2-27b-it" || c.Title != "Case" || c.Plot != "plot" {
		t.Fatalf("case: %+v %v", c, err)
	}
	if _, err := s.Case(ctx, hidden); !errors.Is(err, interrogation.ErrNotFound) {
		t.Fatalf("unpublished case: %v", err)
	}
	var beatriceID string
	if err := pool.QueryRow(ctx, "SELECT id FROM suspects WHERE case_id = $1 AND name = 'Beatrice'", story).Scan(&beatriceID); err != nil {
		t.Fatal(err)
	}
	target, others, err := s.Suspect(ctx, story, beatriceID)
	if err != nil || target.Name != "Beatrice" || len(others) != 1 || others[0].Name != "Thomas" || others[0].Role != "The Butler" {
		t.Fatalf("suspect: %+v %+v %v", target, others, err)
	}
	b := target.Persona
	if !b.IsCulprit || b.Relationship != "his maid" || b.Persona != "quiet" || b.PublicAlibi != "upstairs" ||
		b.HiddenTruth != "did it" || b.ConfessionTriggers != "asked who was last" || len(b.TriggerKeywords) != 2 || b.TriggerKeywords[0] != "last+study" {
		t.Fatalf("persona: %+v", b)
	}
	if _, _, err := s.Suspect(ctx, story, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"); !errors.Is(err, interrogation.ErrSuspectNotFound) {
		t.Fatalf("unknown suspect: %v", err)
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
