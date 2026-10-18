package admin_test

import (
	"backend/internal/testdb"
	"context"
	"errors"
	"os"
	"testing"

	"backend/internal/admin"
	"backend/internal/db"
	"backend/internal/llm"
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
	s := admin.NewPGStore(pool)

	u, err := user.NewPGStore(pool).Create(ctx, "ann", "ann@example.com", "hash", 10)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := s.IsAdmin(ctx, u.ID); err != nil || ok {
		t.Fatalf("new user is admin: %v %v", ok, err)
	}
	if _, err := pool.Exec(ctx, "UPDATE users SET is_admin = true WHERE id = $1", u.ID); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.IsAdmin(ctx, u.ID); err != nil || !ok {
		t.Fatalf("admin: %v %v", ok, err)
	}
	if ok, err := s.IsAdmin(ctx, "99999999-9999-9999-9999-999999999999"); err != nil || ok {
		t.Fatalf("unknown user: %v %v", ok, err)
	}

	// The column default matches llm.DefaultModel.
	var def string
	if err := pool.QueryRow(ctx, "INSERT INTO stories (title, synopsis, difficulty_level) VALUES ('d', 's', 'easy') RETURNING llm_model").Scan(&def); err != nil || def != llm.DefaultModel {
		t.Fatalf("column default %q, DefaultModel %q: %v", def, llm.DefaultModel, err)
	}

	c, err := s.CreateCase(ctx, admin.CaseInput{Title: "T", Synopsis: "S", DifficultyLevel: "hard", RewardTrophies: 2, LLMModel: "google/gemma-2-27b-it"})
	if err != nil || c.LLMModel != "google/gemma-2-27b-it" || c.IsPublished || c.RewardTrophies != 2 || c.ID == "" {
		t.Fatalf("create: %+v %v", c, err)
	}
	// The database refuses what the API refuses.
	if _, err := s.CreateCase(ctx, admin.CaseInput{Title: "T", Synopsis: "S", DifficultyLevel: "hard", LLMModel: "google/gemma-4-31b-it:online"}); !errors.Is(err, admin.ErrInvalid) {
		t.Fatalf(":online model: %v", err)
	}

	// Partial update: only the given fields change.
	m, pub := "google/gemma-3-27b-it:free", true
	got, err := s.UpdateCase(ctx, c.ID, admin.CasePatch{LLMModel: &m, IsPublished: &pub})
	if err != nil || got.LLMModel != m || !got.IsPublished || got.Title != "T" || got.DifficultyLevel != "hard" || got.RewardTrophies != 2 {
		t.Fatalf("update: %+v %v", got, err)
	}
	bad := "nope"
	if _, err := s.UpdateCase(ctx, c.ID, admin.CasePatch{DifficultyLevel: &bad}); !errors.Is(err, admin.ErrInvalid) {
		t.Fatalf("bad difficulty: %v", err)
	}
	if _, err := s.UpdateCase(ctx, "99999999-9999-9999-9999-999999999999", admin.CasePatch{LLMModel: &m}); !errors.Is(err, admin.ErrNotFound) {
		t.Fatalf("unknown case: %v", err)
	}
}
