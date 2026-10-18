package cases_test

import (
	"backend/internal/testdb"
	"context"
	"errors"
	"os"
	"sync"
	"testing"

	"backend/internal/cases"
	"backend/internal/db"
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

	// A lock on a story that is unpublished later neither traps nor hides the player.
	v, err := user.NewPGStore(pool).Create(ctx, "watson", "w@x.co", "h", 10)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Start(ctx, v.ID, other); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE stories SET is_published = false WHERE id = $1", other); err != nil {
		t.Fatal(err)
	}
	if l, err := store.Lock(ctx, v.ID); err != nil || l != nil {
		t.Fatalf("lock on unpublished story still visible: %v %v", l, err)
	}
	if l, err := store.Start(ctx, v.ID, story); err != nil || l.StoryID != story {
		t.Fatalf("start after the locked story was unpublished: %+v %v", l, err)
	}
	if _, err := pool.Exec(ctx, "UPDATE stories SET is_published = true WHERE id = $1", other); err != nil {
		t.Fatal(err)
	}

	// Notes: private per user, newest first, optional suspect of the same case, capped, owner-only delete.
	var suspectID string
	mustScan(&suspectID, `SELECT id FROM suspects WHERE case_id = $1 AND name = 'Thomas'`, story)
	var otherSuspect string
	mustScan(&otherSuspect, `INSERT INTO suspects (case_id, name, role, initial_testimony) VALUES ($1, 'Elsewhere', 'r', 't') RETURNING id`, other)
	n1, err := store.AddNote(ctx, u.ID, story, nil, nil, "first")
	if err != nil || n1.SuspectID != nil || n1.Body != "first" {
		t.Fatalf("add note: %+v %v", n1, err)
	}
	if _, err := store.AddNote(ctx, u.ID, story, &suspectID, nil, "second"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddNote(ctx, u.ID, story, &otherSuspect, nil, "wrong case"); !errors.Is(err, cases.ErrBadSuspect) {
		t.Fatalf("suspect of another case: %v", err)
	}
	if _, err := store.AddNote(ctx, u.ID, hidden, nil, nil, "x"); !errors.Is(err, cases.ErrNotFound) {
		t.Fatalf("note on unpublished case: %v", err)
	}
	msgID := "7f0c6d0e-1111-4222-8333-944455556666"
	onEntry, err := store.AddNote(ctx, u.ID, story, &suspectID, &msgID, "on an entry")
	if err != nil || onEntry.ChatMessageID == nil || *onEntry.ChatMessageID != msgID {
		t.Fatalf("note on a log entry: %+v %v", onEntry, err)
	}
	if err := store.DeleteNote(ctx, u.ID, onEntry.ID); err != nil {
		t.Fatal(err)
	}
	notes, err := store.Notes(ctx, u.ID, story)
	if err != nil || len(notes) != 2 || notes[0].Body != "second" || notes[0].ChatMessageID != nil || notes[0].SuspectID == nil || *notes[0].SuspectID != suspectID {
		t.Fatalf("list notes: %+v %v", notes, err)
	}
	u2 := v // watson, created above
	if theirs, _ := store.Notes(ctx, u2.ID, story); len(theirs) != 0 {
		t.Fatalf("notes leaked to another user: %+v", theirs)
	}
	if err := store.DeleteNote(ctx, u2.ID, n1.ID); !errors.Is(err, cases.ErrNotFound) {
		t.Fatalf("deleting someone else's note: %v", err)
	}
	if err := store.DeleteNote(ctx, u.ID, n1.ID); err != nil {
		t.Fatal(err)
	}
	// the cap
	for i := 0; i < cases.MaxNotesPerCase-1; i++ {
		if _, err := store.AddNote(ctx, u.ID, story, nil, nil, "n"); err != nil {
			t.Fatalf("note %d: %v", i, err)
		}
	}
	if _, err := store.AddNote(ctx, u.ID, story, nil, nil, "one too many"); !errors.Is(err, cases.ErrNotesFull) {
		t.Fatalf("over the cap: %v", err)
	}
	// parallel requests at 199 notes cannot all get in
	for i := 0; i < cases.MaxNotesPerCase-1; i++ {
		if _, err := store.AddNote(ctx, u2.ID, story, nil, nil, "n"); err != nil {
			t.Fatalf("u2 note %d: %v", i, err)
		}
	}
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		created int
	)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := store.AddNote(ctx, u2.ID, story, nil, nil, "race"); err == nil {
				mu.Lock()
				created++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if created != 1 {
		t.Fatalf("%d parallel notes got past the cap, want 1", created)
	}
	// removing a suspect keeps the note, untied
	if _, err := pool.Exec(ctx, `DELETE FROM suspects WHERE id = $1`, suspectID); err != nil {
		t.Fatal(err)
	}
	var tied int
	mustScanInt := pool.QueryRow(ctx, `SELECT count(*) FROM case_notes WHERE body = 'second' AND suspect_id IS NULL`)
	if err := mustScanInt.Scan(&tied); err != nil || tied != 1 {
		t.Fatalf("note after suspect removal: %d %v", tied, err)
	}
}
