package cases

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	uid     = "11111111-1111-1111-1111-111111111111"
	caseA   = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	caseB   = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	attempt = "cccccccc-cccc-cccc-cccc-cccccccccccc"
)

type fakeStore struct {
	lock     *Lock
	briefing *Briefing
	suspects []Suspect
	err      error
	notes    []Note
	notesErr error
	known    map[string]bool // cases Start accepts
	closed   map[string]bool // cases the player already closed
	result   *Result
	hintErr  error
	started  int
}

func (f *fakeStore) HintState(_ context.Context, _, id string) (*HintState, error) {
	if f.err != nil || f.briefing == nil || f.briefing.ID != id {
		return nil, errOr(f.err)
	}
	return &HintState{HintsTotal: 3, HintsUsed: 1, Hints: []string{"Look at the clock."}}, nil
}
func (f *fakeStore) Briefing(_ context.Context, id string) (*Briefing, error) {
	if f.err != nil || f.briefing == nil || f.briefing.ID != id {
		return nil, errOr(f.err)
	}
	return f.briefing, nil
}
func (f *fakeStore) Suspects(_ context.Context, id string) ([]Suspect, error) {
	if f.err != nil || f.briefing == nil || f.briefing.ID != id {
		return nil, errOr(f.err)
	}
	return f.suspects, nil
}
func (f *fakeStore) Result(context.Context, string, string) (*Result, error) {
	if f.result == nil {
		return nil, ErrNotFound
	}
	return f.result, nil
}
func (f *fakeStore) RevealHint(context.Context, string, string) (*Hint, error) {
	if f.hintErr != nil {
		return nil, f.hintErr
	}
	return &Hint{Hint: "Look at the clock.", Number: 1, Cost: 1, HintsTotal: 3}, nil
}
func (f *fakeStore) Lock(context.Context, string) (*Lock, error) { return f.lock, nil }
func (f *fakeStore) Start(_ context.Context, _, storyID string) (*Lock, error) {
	switch {
	case f.err != nil:
		return nil, f.err
	case !f.known[storyID]:
		return nil, ErrNotFound
	case f.closed[storyID]:
		return nil, ErrClosed
	case f.lock != nil && f.lock.StoryID != storyID:
		return nil, ErrLockedElsewhere
	}
	f.started++
	if f.lock == nil {
		f.lock = &Lock{CaseID: attempt, StoryID: storyID}
	}
	return f.lock, nil
}

func (f *fakeStore) Notes(context.Context, string, string) ([]Note, error) { return f.notes, f.err }
func (f *fakeStore) AddNote(_ context.Context, _, _ string, suspectID, chatMessageID *string, body string) (*Note, error) {
	if f.notesErr != nil {
		return nil, f.notesErr
	}
	n := Note{ID: "n1", SuspectID: suspectID, ChatMessageID: chatMessageID, Body: body}
	f.notes = append([]Note{n}, f.notes...)
	return &n, nil
}
func (f *fakeStore) DeleteNote(_ context.Context, _, noteID string) error {
	if noteID != "dddddddd-dddd-dddd-dddd-dddddddddddd" {
		return ErrNotFound
	}
	return nil
}

func errOr(err error) error {
	if err != nil {
		return err
	}
	return ErrNotFound
}

func newHandler() (*Handler, *fakeStore) {
	st := &fakeStore{
		briefing: &Briefing{ID: caseA, Title: "The Poisoned Earl Grey", MainPlot: "plot", DifficultyLevel: "easy", Scenes: []Scene{}},
		known:    map[string]bool{caseA: true},
		suspects: []Suspect{{ID: "s1", Name: "Thomas", Role: "The Butler", InitialTestimony: "I served the tea."}},
	}
	return NewHandler(st, func(context.Context) string { return uid }), st
}

func do(h http.HandlerFunc, method, id string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/api/cases/"+id, nil)
	req.SetPathValue("id", id)
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

func TestBriefingAndSuspects(t *testing.T) {
	h, _ := newHandler()
	rec := do(h.Briefing, "GET", caseA)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"main_plot":"plot"`) || !strings.Contains(rec.Body.String(), `"scenes":[]`) {
		t.Fatalf("briefing: %d %s", rec.Code, rec.Body)
	}
	rec = do(h.Suspects, "GET", caseA)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"initial_testimony":"I served the tea."`) {
		t.Fatalf("suspects: %d %s", rec.Code, rec.Body)
	}
}

// The solution must never reach the client: no culprit flag in any response.
func TestNoSolutionDataInResponses(t *testing.T) {
	h, _ := newHandler()
	for name, rec := range map[string]*httptest.ResponseRecorder{
		"briefing": do(h.Briefing, "GET", caseA),
		"suspects": do(h.Suspects, "GET", caseA),
	} {
		if b := strings.ToLower(rec.Body.String()); strings.Contains(b, "culprit") {
			t.Errorf("%s response mentions the culprit: %s", name, b)
		}
	}
}

func TestInvalidAndUnknownIDs(t *testing.T) {
	h, _ := newHandler()
	for name, fn := range map[string]http.HandlerFunc{"briefing": h.Briefing, "suspects": h.Suspects, "start": h.Start} {
		if rec := do(fn, "GET", "not-a-uuid"); rec.Code != 400 {
			t.Errorf("%s bad id: %d", name, rec.Code)
		}
		if rec := do(fn, "GET", caseB); rec.Code != 404 {
			t.Errorf("%s unknown case: %d", name, rec.Code)
		}
	}
}

func TestActiveLockRestrictsOtherCases(t *testing.T) {
	h, st := newHandler()
	st.briefing.ID = caseB
	st.lock = &Lock{CaseID: attempt, StoryID: caseA}
	for name, fn := range map[string]http.HandlerFunc{"briefing": h.Briefing, "suspects": h.Suspects} {
		if rec := do(fn, "GET", caseB); rec.Code != http.StatusForbidden {
			t.Errorf("%s of another case while locked: %d", name, rec.Code)
		}
	}
	// the locked case itself is readable
	st.briefing.ID = caseA
	if rec := do(h.Briefing, "GET", caseA); rec.Code != 200 {
		t.Errorf("own case: %d", rec.Code)
	}
}

// A case the player already closed stays readable while they work on another one.
func TestClosedCaseReadableWhileLocked(t *testing.T) {
	h, st := newHandler()
	st.briefing.ID = caseB
	st.lock = &Lock{CaseID: attempt, StoryID: caseA}
	st.result = &Result{Correct: true}
	for name, fn := range map[string]http.HandlerFunc{"briefing": h.Briefing, "suspects": h.Suspects} {
		if rec := do(fn, "GET", caseB); rec.Code != 200 {
			t.Errorf("%s of a closed case while locked: %d", name, rec.Code)
		}
	}
}

func TestStart(t *testing.T) {
	h, st := newHandler()
	st.known[caseB] = true
	rec := do(h.Start, "POST", caseA)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"story_id":"`+caseA+`"`) || st.lock == nil {
		t.Fatalf("start: %d %s", rec.Code, rec.Body)
	}
	// starting the same case again (double click, or Continue) is fine
	if rec := do(h.Start, "POST", caseA); rec.Code != 200 {
		t.Fatalf("restart same: %d", rec.Code)
	}
	// another case while locked: 400
	rec = do(h.Start, "POST", caseB)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "Finish your active case first") {
		t.Fatalf("start other: %d %s", rec.Code, rec.Body)
	}
}

func notesReq(h http.HandlerFunc, method, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/api/cases/"+caseA+"/notes", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", caseA)
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

func TestAddAndListNotes(t *testing.T) {
	h, st := newHandler()
	rec := notesReq(h.AddNote, "POST", `{"body":"  Beatrice never mentions the cup  ","suspect_id":"`+caseB+`"}`)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"body":"Beatrice never mentions the cup"`) {
		t.Fatalf("add: %d %s", rec.Code, rec.Body)
	}
	if rec := notesReq(h.Notes, "GET", ""); rec.Code != 200 || !strings.Contains(rec.Body.String(), "Beatrice never mentions the cup") {
		t.Fatalf("list: %d %s", rec.Code, rec.Body)
	}
	for name, body := range map[string]string{
		"empty":         `{"body":""}`,
		"blank":         `{"body":"    "}`,
		"only nul":      `{"body":"\u0000\u0007"}`,
		"too long":      `{"body":"` + strings.Repeat("x", 1001) + `"}`,
		"bad suspect":   `{"body":"hi","suspect_id":"nope"}`,
		"unknown field": `{"body":"hi","user_id":"x"}`,
	} {
		if rec := notesReq(h.AddNote, "POST", body); rec.Code != 422 && rec.Code != 400 {
			t.Errorf("%s: %d", name, rec.Code)
		}
	}
	// Control characters (NUL is refused by Postgres) are dropped before saving.
	if rec := notesReq(h.AddNote, "POST", `{"body":"a\u0000b\u0007c"}`); rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"body":"abc"`) {
		t.Fatalf("control characters: %d %s", rec.Code, rec.Body)
	}
	st.notesErr = ErrBadSuspect
	if rec := notesReq(h.AddNote, "POST", `{"body":"hi"}`); rec.Code != 422 {
		t.Errorf("suspect of another case: %d", rec.Code)
	}
	st.notesErr = ErrNotesFull
	if rec := notesReq(h.AddNote, "POST", `{"body":"hi"}`); rec.Code != http.StatusConflict {
		t.Errorf("notes full: %d", rec.Code)
	}
}

func TestDeleteNote(t *testing.T) {
	h, _ := newHandler()
	del := func(id string) int {
		req := httptest.NewRequest("DELETE", "/", nil)
		req.SetPathValue("id", caseA)
		req.SetPathValue("note_id", id)
		rec := httptest.NewRecorder()
		h.DeleteNote(rec, req)
		return rec.Code
	}
	if c := del("dddddddd-dddd-dddd-dddd-dddddddddddd"); c != 200 {
		t.Errorf("own note: %d", c)
	}
	if c := del("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"); c != 404 {
		t.Errorf("someone else's note: %d", c)
	}
	if c := del("junk"); c != 400 {
		t.Errorf("bad id: %d", c)
	}
}

type fakeLogs struct {
	suspect string
	err     error
}

func (f fakeLogs) SuspectOf(_ context.Context, userID, caseID, id string) (string, bool, error) {
	if f.err != nil {
		return "", false, f.err
	}
	// only entry "m1" of the caller's own case exists
	return f.suspect, userID == uid && caseID == caseA && id == "m1", nil
}

func TestNoteTiedToLogEntry(t *testing.T) {
	h, _ := newHandler()
	h.WithLogs(fakeLogs{suspect: caseB})

	rec := notesReq(h.AddNote, "POST", `{"body":"he lied here","chat_message_id":"m1"}`)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"chat_message_id":"m1"`) || !strings.Contains(rec.Body.String(), `"suspect_id":"`+caseB+`"`) {
		t.Fatalf("tie to entry (suspect taken from the entry): %d %s", rec.Code, rec.Body)
	}
	if rec := notesReq(h.AddNote, "POST", `{"body":"x","chat_message_id":"m1","suspect_id":"`+caseB+`"}`); rec.Code != http.StatusCreated {
		t.Fatalf("matching suspect: %d", rec.Code)
	}
	for name, body := range map[string]string{
		"unknown entry": `{"body":"x","chat_message_id":"nope"}`,
		"other suspect": `{"body":"x","chat_message_id":"m1","suspect_id":"` + caseA + `"}`,
		"empty id":      `{"body":"x","chat_message_id":""}`,
		"id too long":   `{"body":"x","chat_message_id":"` + strings.Repeat("a", 65) + `"}`,
	} {
		if rec := notesReq(h.AddNote, "POST", body); rec.Code != 422 {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Body)
		}
	}

	// A lookup failure is a 500, not a saved note; without a lookup the feature is off.
	h.WithLogs(fakeLogs{err: errors.New("mongo down")})
	if rec := notesReq(h.AddNote, "POST", `{"body":"x","chat_message_id":"m1"}`); rec.Code != 500 {
		t.Fatalf("lookup error: %d", rec.Code)
	}
	h.logs = nil
	if rec := notesReq(h.AddNote, "POST", `{"body":"x","chat_message_id":"m1"}`); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("no lookup: %d", rec.Code)
	}
}

func TestStartClosedCaseIs409(t *testing.T) {
	st := &fakeStore{known: map[string]bool{caseA: true}, closed: map[string]bool{caseA: true}}
	h := NewHandler(st, func(context.Context) string { return "u" })
	req := httptest.NewRequest(http.MethodPost, "/api/cases/"+caseA+"/start", nil)
	req.SetPathValue("id", caseA)
	rec := httptest.NewRecorder()
	h.Start(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409: %s", rec.Code, rec.Body)
	}
}

func TestResult(t *testing.T) {
	get := func(st *fakeStore) *httptest.ResponseRecorder {
		h := NewHandler(st, func(context.Context) string { return "u" })
		req := httptest.NewRequest(http.MethodGet, "/api/cases/"+caseA+"/result", nil)
		req.SetPathValue("id", caseA)
		rec := httptest.NewRecorder()
		h.Result(rec, req)
		return rec
	}
	if rec := get(&fakeStore{}); rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), "culprit_name") {
		t.Fatalf("open case must be 404 and reveal nothing: %d %s", rec.Code, rec.Body)
	}
	rec := get(&fakeStore{result: &Result{Correct: true, CulpritName: "Ada", Resolution: "Ada did it."}})
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "Ada did it.") {
		t.Fatalf("closed case: %d %s", rec.Code, rec.Body)
	}
}

func TestHints(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
	}{
		{"revealed", nil, 200},
		{"not locked", ErrNotLocked, 403},
		{"none left", ErrNoHints, 400},
		{"database error", errors.New("boom"), 500},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := NewHandler(&fakeStore{hintErr: tc.err}, func(context.Context) string { return "u" })
			req := httptest.NewRequest(http.MethodPost, "/api/cases/"+caseA+"/hints", nil)
			req.SetPathValue("id", caseA)
			rec := httptest.NewRecorder()
			h.Hints(rec, req)
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.status, rec.Body)
			}
		})
	}
	h := NewHandler(&fakeStore{}, func(context.Context) string { return "u" })
	req := httptest.NewRequest(http.MethodPost, "/api/cases/nope/hints", nil)
	req.SetPathValue("id", "nope")
	rec := httptest.NewRecorder()
	h.Hints(rec, req)
	if rec.Code != 400 {
		t.Fatalf("bad id: %d", rec.Code)
	}
}
