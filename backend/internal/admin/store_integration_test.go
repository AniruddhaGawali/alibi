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
	"backend/internal/settings"
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
	if role, err := s.Role(ctx, u.ID); err != nil || role != user.RoleUser || u.Role != user.RoleUser {
		t.Fatalf("new user role: %q %q %v", role, u.Role, err)
	}
	if _, err := pool.Exec(ctx, "UPDATE users SET role = 'admin' WHERE id = $1", u.ID); err != nil {
		t.Fatal(err)
	}
	if role, err := s.Role(ctx, u.ID); err != nil || role != user.RoleAdmin {
		t.Fatalf("admin: %q %v", role, err)
	}
	if _, err := pool.Exec(ctx, "UPDATE users SET role = 'root' WHERE id = $1", u.ID); err == nil {
		t.Fatal("unknown role accepted")
	}
	if _, err := s.Role(ctx, "99999999-9999-9999-9999-999999999999"); !errors.Is(err, admin.ErrNotFound) {
		t.Fatalf("unknown user: %v", err)
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
	// A new case cannot be published: it has no suspects yet.
	if _, err := s.CreateCase(ctx, admin.CaseInput{Title: "T", Synopsis: "S", DifficultyLevel: "hard", LLMModel: llm.DefaultModel, IsPublished: true}); !errors.Is(err, admin.ErrNotReady) {
		t.Fatalf("published at birth: %v", err)
	}

	// Partial update: only the given fields change.
	m, thumb := "google/gemma-3-27b-it:free", "https://cdn.test/a.webp"
	got, old, err := s.UpdateCase(ctx, c.ID, admin.CasePatch{LLMModel: &m, ThumbnailURL: &thumb})
	if err != nil || got.LLMModel != m || got.IsPublished || got.Title != "T" || got.DifficultyLevel != "hard" || got.RewardTrophies != 2 || got.ThumbnailURL != thumb || old != "" {
		t.Fatalf("update: %+v %q %v", got, old, err)
	}
	thumb2 := "https://cdn.test/b.webp"
	if _, old, err := s.UpdateCase(ctx, c.ID, admin.CasePatch{ThumbnailURL: &thumb2}); err != nil || old != thumb {
		t.Fatalf("old thumbnail %q %v", old, err)
	}
	track := "too-crazy"
	if got, _, err := s.UpdateCase(ctx, c.ID, admin.CasePatch{MusicTrack: &track}); err != nil || got.MusicTrack != track {
		t.Fatalf("music track: %+v %v", got, err)
	}
	evil := "../../etc"
	if _, _, err := s.UpdateCase(ctx, c.ID, admin.CasePatch{MusicTrack: &evil}); !errors.Is(err, admin.ErrInvalid) {
		t.Fatalf("unsafe track name: %v", err)
	}
	bad := "nope"
	if _, _, err := s.UpdateCase(ctx, c.ID, admin.CasePatch{DifficultyLevel: &bad}); !errors.Is(err, admin.ErrInvalid) {
		t.Fatalf("bad difficulty: %v", err)
	}
	if _, _, err := s.UpdateCase(ctx, "99999999-9999-9999-9999-999999999999", admin.CasePatch{LLMModel: &m}); !errors.Is(err, admin.ErrNotFound) {
		t.Fatalf("unknown case: %v", err)
	}

	// Publishing needs two suspects and one culprit.
	pub := true
	if _, _, err := s.UpdateCase(ctx, c.ID, admin.CasePatch{IsPublished: &pub}); !errors.Is(err, admin.ErrNotReady) {
		t.Fatalf("publish without suspects: %v", err)
	}
	// Secrets without keywords would be in every prompt: refused.
	if _, err := s.CreateSuspect(ctx, c.ID, admin.SuspectInput{Name: "A", Role: "r", InitialTestimony: "t", HiddenTruth: "x"}); !errors.Is(err, admin.ErrNeedsKeywords) {
		t.Fatalf("secrets without keywords: %v", err)
	}
	a, err := s.CreateSuspect(ctx, c.ID, admin.SuspectInput{Name: "A", Role: "r", InitialTestimony: "t", IsCulprit: true, HiddenTruth: "did it", ConfessionKeywords: []string{"tea"}})
	if err != nil || !a.IsCulprit || a.ConfessionKeywords[0] != "tea" {
		t.Fatalf("suspect A: %+v %v", a, err)
	}
	if _, err := s.CreateSuspect(ctx, c.ID, admin.SuspectInput{Name: "A", Role: "r", InitialTestimony: "t"}); !errors.Is(err, admin.ErrNameTaken) {
		t.Fatalf("duplicate name: %v", err)
	}
	if _, err := s.CreateSuspect(ctx, "99999999-9999-9999-9999-999999999999", admin.SuspectInput{Name: "Z", Role: "r", InitialTestimony: "t"}); !errors.Is(err, admin.ErrNotFound) {
		t.Fatalf("unknown case: %v", err)
	}
	// A second culprit takes the flag from the first instead of conflicting.
	b, err := s.CreateSuspect(ctx, c.ID, admin.SuspectInput{Name: "B", Role: "r", InitialTestimony: "t", IsCulprit: true})
	if err != nil || !b.IsCulprit {
		t.Fatalf("suspect B: %+v %v", b, err)
	}
	if got, _, err := s.UpdateSuspect(ctx, c.ID, a.ID, admin.SuspectPatch{IsCulprit: &pub}); err != nil || !got.IsCulprit {
		t.Fatalf("move culprit back: %+v %v", got, err)
	}
	detail, err := s.GetCase(ctx, c.ID)
	if err != nil || len(detail.Suspects) != 2 {
		t.Fatalf("detail: %+v %v", detail, err)
	}
	culprits := 0
	for _, p := range detail.Suspects {
		if p.IsCulprit {
			culprits++
		}
	}
	if culprits != 1 {
		t.Fatalf("culprits: %d", culprits)
	}

	if got, _, err := s.UpdateCase(ctx, c.ID, admin.CasePatch{IsPublished: &pub}); err != nil || !got.IsPublished {
		t.Fatalf("publish: %+v %v", got, err)
	}
	// A published case must stay playable.
	no := false
	if _, _, err := s.UpdateSuspect(ctx, c.ID, a.ID, admin.SuspectPatch{IsCulprit: &no}); !errors.Is(err, admin.ErrNotReady) {
		t.Fatalf("unset the only culprit: %v", err)
	}
	if _, err := s.DeleteSuspect(ctx, c.ID, b.ID); !errors.Is(err, admin.ErrNotReady) {
		t.Fatalf("drop below two suspects: %v", err)
	}
	if _, _, err := s.UpdateSuspect(ctx, c.ID, b.ID, admin.SuspectPatch{IsCulprit: &no}); err != nil {
		t.Fatalf("unset a non-culprit: %v", err)
	}

	// Scenes.
	sc, err := s.CreateScene(ctx, c.ID, "Study", "https://cdn.test/s.webp", 1)
	if err != nil || sc.SceneName != "Study" {
		t.Fatalf("scene: %+v %v", sc, err)
	}
	if _, err := s.CreateScene(ctx, c.ID, "Hall", "https://cdn.test/h.webp", 1); !errors.Is(err, admin.ErrOrderTaken) {
		t.Fatalf("same order: %v", err)
	}

	// A case someone is playing cannot be deleted; an unplayed one can, with its images reported.
	if _, err := pool.Exec(ctx, "INSERT INTO active_case_locks (user_id, case_id, story_id) VALUES ($1, gen_random_uuid(), $2)", u.ID, c.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteCase(ctx, c.ID); !errors.Is(err, admin.ErrInUse) {
		t.Fatalf("delete while played: %v", err)
	}
	if _, err := pool.Exec(ctx, "DELETE FROM active_case_locks WHERE user_id = $1", u.ID); err != nil {
		t.Fatal(err)
	}
	images, err := s.DeleteCase(ctx, c.ID)
	if err != nil || len(images) != 2 { // the thumbnail and the scene
		t.Fatalf("delete: %v %v", images, err)
	}
	if _, err := s.GetCase(ctx, c.ID); !errors.Is(err, admin.ErrNotFound) {
		t.Fatalf("deleted case still there: %v", err)
	}
	if _, err := s.DeleteCase(ctx, c.ID); !errors.Is(err, admin.ErrNotFound) {
		t.Fatalf("delete twice: %v", err)
	}

	// Player search is a literal, case-insensitive substring match.
	if _, err := user.NewPGStore(pool).Create(ctx, "bob_smith", "bob@example.com", "hash", 4); err != nil {
		t.Fatal(err)
	}
	if got, err := s.SearchUsers(ctx, "BOB", 20); err != nil || len(got) != 1 || got[0].Username != "bob_smith" || got[0].TokenBalance != 4 {
		t.Fatalf("search bob: %+v %v", got, err)
	}
	if got, err := s.SearchUsers(ctx, "b_b", 20); err != nil || len(got) != 0 {
		t.Fatalf("_ must not be a wildcard: %+v %v", got, err)
	}
	if got, err := s.SearchUsers(ctx, "%", 20); err != nil || len(got) != 0 {
		t.Fatalf("%% must not be a wildcard: %+v %v", got, err)
	}
	if got, err := s.SearchUsers(ctx, "", 1); err != nil || len(got) != 1 {
		t.Fatalf("empty search, limit 1: %+v %v", got, err)
	}
	if _, err := s.GetUser(ctx, "99999999-9999-9999-9999-999999999999"); !errors.Is(err, admin.ErrNotFound) {
		t.Fatalf("unknown user: %v", err)
	}

	// Settings: seeded once, then owned by the admin panel.
	ss := settings.NewPGStore(pool)
	svc, err := settings.New(ctx, ss, settings.Settings{DailyTokens: 8, ResetTime: "01:15", Timezone: "UTC"})
	if err != nil || svc.DailyTokens(ctx) != 8 {
		t.Fatalf("seed: %v", err)
	}
	if _, err := svc.Update(ctx, settings.Settings{DailyTokens: 6, ResetTime: "04:30", Timezone: "Asia/Kolkata"}, u.ID); err != nil {
		t.Fatal(err)
	}
	again, err := settings.New(ctx, ss, settings.Settings{DailyTokens: 8, ResetTime: "01:15", Timezone: "UTC"})
	if err != nil || again.Current(ctx).DailyTokens != 6 || again.Current(ctx).ResetTime != "04:30" {
		t.Fatalf("seed overwrote the saved settings: %+v %v", again.Current(ctx), err)
	}
	if _, err := ss.Update(ctx, settings.Settings{DailyTokens: 11, ResetTime: "04:30", Timezone: "UTC"}, ""); err == nil {
		t.Fatal("11 daily tokens accepted by the database")
	}
}
