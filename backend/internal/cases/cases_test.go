package cases

import (
	"context"
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
	known    map[string]bool // cases Start accepts
	started  int
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
func (f *fakeStore) Lock(context.Context, string) (*Lock, error) { return f.lock, nil }
func (f *fakeStore) Start(_ context.Context, _, storyID string) (*Lock, error) {
	switch {
	case f.err != nil:
		return nil, f.err
	case !f.known[storyID]:
		return nil, ErrNotFound
	case f.lock != nil && f.lock.StoryID != storyID:
		return nil, ErrLockedElsewhere
	}
	f.started++
	if f.lock == nil {
		f.lock = &Lock{CaseID: attempt, StoryID: storyID}
	}
	return f.lock, nil
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
