package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"backend/internal/llm"
)

const caseID = "11111111-1111-1111-1111-111111111111"

type fakeStore struct {
	admin   bool
	created *CaseInput
	patched *CasePatch
}

func (f *fakeStore) IsAdmin(context.Context, string) (bool, error) { return f.admin, nil }
func (f *fakeStore) CreateCase(_ context.Context, in CaseInput) (*Case, error) {
	f.created = &in
	return &Case{ID: caseID, Title: in.Title, LLMModel: in.LLMModel}, nil
}
func (f *fakeStore) UpdateCase(_ context.Context, id string, p CasePatch) (*Case, error) {
	if id != caseID {
		return nil, ErrNotFound
	}
	f.patched = &p
	m := llm.DefaultModel
	if p.LLMModel != nil {
		m = *p.LLMModel
	}
	return &Case{ID: id, LLMModel: m}, nil
}

type fakeModels struct {
	disabled bool
	err      error
	checked  []string
}

func (m *fakeModels) Enabled() bool { return !m.disabled }
func (m *fakeModels) CheckTextModel(_ context.Context, id string) error {
	m.checked = append(m.checked, id)
	return m.err
}

func serve(store *fakeStore, models *fakeModels, method, path, body string) *httptest.ResponseRecorder {
	h := NewHandler(store, models, func(context.Context) string { return "u1" })
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/admin/cases", h.RequireAdmin(h.CreateCase))
	mux.HandleFunc("PATCH /api/admin/cases/{id}", h.RequireAdmin(h.UpdateCase))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
	return rec
}

const newCase = `{"title":"T","synopsis":"S","difficulty_level":"easy","reward_trophies":1`

func TestNonAdminForbidden(t *testing.T) {
	s := &fakeStore{}
	if rec := serve(s, &fakeModels{}, "POST", "/api/admin/cases", newCase+"}"); rec.Code != 403 || s.created != nil {
		t.Fatalf("status %d", rec.Code)
	}
	if rec := serve(s, &fakeModels{}, "PATCH", "/api/admin/cases/"+caseID, `{"llm_model":"a/b"}`); rec.Code != 403 || s.patched != nil {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestCreateCaseModel(t *testing.T) {
	// No model: the default Gemma model, checked with OpenRouter.
	s, m := &fakeStore{admin: true}, &fakeModels{}
	rec := serve(s, m, "POST", "/api/admin/cases", newCase+"}")
	if rec.Code != 201 || s.created.LLMModel != llm.DefaultModel || s.created.IsPublished || len(m.checked) != 1 {
		t.Fatalf("status %d %s %+v", rec.Code, rec.Body, s.created)
	}
	var env struct{ Data Case }
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if env.Data.LLMModel != llm.DefaultModel {
		t.Fatalf("response %s", rec.Body)
	}

	// A chosen model is stored as given.
	s = &fakeStore{admin: true}
	if rec := serve(s, &fakeModels{}, "POST", "/api/admin/cases", newCase+`,"llm_model":"google/gemma-2-27b-it"}`); rec.Code != 201 || s.created.LLMModel != "google/gemma-2-27b-it" {
		t.Fatalf("status %d %+v", rec.Code, s.created)
	}

	for name, c := range map[string]struct {
		body   string
		models *fakeModels
		status int
	}{
		"web search suffix": {newCase + `,"llm_model":"google/gemma-4-31b-it:online"}`, &fakeModels{}, 422},
		"router model":      {newCase + `,"llm_model":"openrouter/auto"}`, &fakeModels{}, 422},
		"image model":       {newCase + `,"llm_model":"google/gemini-2.5-flash-image"}`, &fakeModels{err: llm.ErrNotTextOnly}, 422},
		"expensive model":   {newCase + `,"llm_model":"big/pricey"}`, &fakeModels{err: llm.ErrTooExpensive}, 422},
		"unknown model":     {newCase + `,"llm_model":"nobody/nothing"}`, &fakeModels{err: llm.ErrUnknownModel}, 422},
		"openrouter down":   {newCase + `,"llm_model":"a/b"}`, &fakeModels{err: context.DeadlineExceeded}, 503},
		"bad difficulty":    {`{"title":"T","synopsis":"S","difficulty_level":"insane"}`, &fakeModels{}, 422},
		"unknown field":     {newCase + `,"is_culprit":true}`, &fakeModels{}, 400},
	} {
		s := &fakeStore{admin: true}
		if rec := serve(s, c.models, "POST", "/api/admin/cases", c.body); rec.Code != c.status || s.created != nil {
			t.Errorf("%s: status %d, want %d: %s", name, rec.Code, c.status, rec.Body)
		}
	}

	// Without an OpenRouter key only the format is checked.
	s = &fakeStore{admin: true}
	if rec := serve(s, &fakeModels{disabled: true, err: llm.ErrUnknownModel}, "POST", "/api/admin/cases", newCase+`,"llm_model":"a/b"}`); rec.Code != 201 {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestUpdateCase(t *testing.T) {
	s := &fakeStore{admin: true}
	rec := serve(s, &fakeModels{}, "PATCH", "/api/admin/cases/"+caseID, `{"llm_model":"google/gemma-3-27b-it"}`)
	if rec.Code != 200 || s.patched.LLMModel == nil || *s.patched.LLMModel != "google/gemma-3-27b-it" || s.patched.Title != nil {
		t.Fatalf("status %d %s", rec.Code, rec.Body)
	}
	for name, c := range map[string]struct {
		path, body string
		status     int
	}{
		"empty":       {"/api/admin/cases/" + caseID, `{}`, 422},
		"bad id":      {"/api/admin/cases/nope", `{"title":"x"}`, 400},
		"missing":     {"/api/admin/cases/22222222-2222-2222-2222-222222222222", `{"title":"x"}`, 404},
		"bad model":   {"/api/admin/cases/" + caseID, `{"llm_model":"x:online"}`, 422},
		"blank title": {"/api/admin/cases/" + caseID, `{"title":""}`, 422},
	} {
		if rec := serve(&fakeStore{admin: true}, &fakeModels{}, "PATCH", c.path, c.body); rec.Code != c.status {
			t.Errorf("%s: status %d, want %d: %s", name, rec.Code, c.status, rec.Body)
		}
	}
}
