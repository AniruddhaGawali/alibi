package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"backend/internal/llm"
	"backend/internal/settings"
	"backend/internal/storage"
)

const (
	caseID    = "11111111-1111-1111-1111-111111111111"
	suspectID = "33333333-3333-3333-3333-333333333333"
)

type fakeStore struct {
	role       string
	created    *CaseInput
	patched    *CasePatch
	suspectIn  *SuspectInput
	suspectP   *SuspectPatch
	err        error  // returned by the write methods when set
	oldImage   string // what an update or delete reports it replaced
	deleted    bool
	caseImages []string
	searched   *string
	balance    int64 // the player's balance GetUser reports
}

type fakeWallet struct{ granted []int64 }

func (w *fakeWallet) Grant(_ context.Context, _ string, amount int64) (int64, error) {
	w.granted = append(w.granted, amount)
	return 10, nil // as if capped at the maximum
}

func (f *fakeStore) Role(context.Context, string) (string, error) {
	if f.role == "" {
		return "user", nil
	}
	return f.role, nil
}
func (f *fakeStore) ListCases(context.Context) ([]CaseSummary, error) {
	return []CaseSummary{{ID: caseID, Title: "T"}}, nil
}
func (f *fakeStore) GetCase(_ context.Context, id string) (*CaseDetail, error) {
	if id != caseID {
		return nil, ErrNotFound
	}
	return &CaseDetail{Case: Case{ID: id}, Suspects: []Suspect{{ID: suspectID, HiddenTruth: "did it"}}, Scenes: []Scene{}}, nil
}
func (f *fakeStore) CaseExists(_ context.Context, id string) (bool, error) { return id == caseID, nil }
func (f *fakeStore) CreateCase(_ context.Context, in CaseInput) (*Case, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.created = &in
	return &Case{ID: caseID, Title: in.Title, LLMModel: in.LLMModel}, nil
}
func (f *fakeStore) UpdateCase(_ context.Context, id string, p CasePatch) (*Case, string, error) {
	if id != caseID {
		return nil, "", ErrNotFound
	}
	if f.err != nil {
		return nil, "", f.err
	}
	f.patched = &p
	m := llm.DefaultModel
	if p.LLMModel != nil {
		m = *p.LLMModel
	}
	c := &Case{ID: id, LLMModel: m}
	if p.ThumbnailURL != nil {
		c.ThumbnailURL = *p.ThumbnailURL
	}
	return c, f.oldImage, nil
}
func (f *fakeStore) DeleteCase(_ context.Context, id string) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.deleted = true
	return f.caseImages, nil
}
func (f *fakeStore) CreateSuspect(_ context.Context, caseID string, in SuspectInput) (*Suspect, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.suspectIn = &in
	return &Suspect{ID: suspectID, CaseID: caseID, Name: in.Name}, nil
}
func (f *fakeStore) UpdateSuspect(_ context.Context, caseID, id string, p SuspectPatch) (*Suspect, string, error) {
	if f.err != nil {
		return nil, "", f.err
	}
	f.suspectP = &p
	s := &Suspect{ID: id, CaseID: caseID}
	if p.AvatarURL != nil {
		s.AvatarURL = *p.AvatarURL
	}
	return s, f.oldImage, nil
}
func (f *fakeStore) DeleteSuspect(context.Context, string, string) (string, error) {
	return f.oldImage, f.err
}
func (f *fakeStore) SearchUsers(_ context.Context, q string, limit int) ([]Player, error) {
	f.searched = &q
	return []Player{{ID: suspectID, Username: "rookie", TokenBalance: 3}}, nil
}
func (f *fakeStore) GetUser(_ context.Context, id string) (*Player, error) {
	if id != suspectID {
		return nil, ErrNotFound
	}
	return &Player{ID: id, Username: "rookie", TokenBalance: f.balance}, nil
}
func (f *fakeStore) CreateScene(_ context.Context, caseID, name, img string, order int32) (*Scene, error) {
	return &Scene{ID: suspectID, CaseID: caseID, SceneName: name, ImageURL: img, DisplayOrder: int(order)}, f.err
}
func (f *fakeStore) UpdateScene(_ context.Context, caseID, id string, p ScenePatch) (*Scene, string, error) {
	return &Scene{ID: id, CaseID: caseID}, f.oldImage, f.err
}
func (f *fakeStore) DeleteScene(context.Context, string, string) (string, error) {
	return f.oldImage, f.err
}

type fakeModels struct {
	disabled bool
	err      error
	checked  []string
	list     []llm.ModelInfo
}

func (m *fakeModels) ListTextModels(context.Context) ([]llm.ModelInfo, error) { return m.list, m.err }

func (m *fakeModels) Enabled() bool { return !m.disabled }
func (m *fakeModels) CheckTextModel(_ context.Context, id string) error {
	m.checked = append(m.checked, id)
	return m.err
}

type fakeSettings struct {
	cur   settings.Settings
	saved *settings.Settings
}

func (f *fakeSettings) Current(context.Context) settings.Settings { return f.cur }
func (f *fakeSettings) Update(_ context.Context, s settings.Settings, _ string) (*settings.Settings, error) {
	f.saved = &s
	s.UpdatedAt = time.Now()
	return &s, nil
}

type fakeImages struct {
	uploaded map[string][]byte
	deleted  []string
}

func (f *fakeImages) UploadImage(_ context.Context, path string, data []byte) (string, error) {
	if f.uploaded == nil {
		f.uploaded = map[string][]byte{}
	}
	f.uploaded[path] = data
	return f.PublicURL(path), nil
}
func (f *fakeImages) Delete(_ context.Context, path string) error {
	f.deleted = append(f.deleted, path)
	return nil
}
func (f *fakeImages) PublicURL(path string) string { return "https://cdn.test/public/b/" + path }
func (f *fakeImages) Ping(context.Context) error   { return nil }

var _ storage.Provider = (*fakeImages)(nil)

func newTestHandler(store *fakeStore, models *fakeModels, opts ...Option) http.Handler {
	h := NewHandler(store, models, func(context.Context) string { return "u1" }, opts...)
	mux := http.NewServeMux()
	a := h.AdminOnly
	mux.HandleFunc("GET /api/admin/models", a(h.ListModels))
	mux.HandleFunc("GET /api/admin/users", a(h.SearchUsers))
	mux.HandleFunc("POST /api/admin/users/{id}/tokens", a(h.GrantTokens))
	mux.HandleFunc("GET /api/admin/config", a(h.GetConfig))
	mux.HandleFunc("PUT /api/admin/config", a(h.UpdateConfig))
	mux.HandleFunc("POST /api/admin/upload", a(h.Upload))
	mux.HandleFunc("GET /api/admin/cases", a(h.ListCases))
	mux.HandleFunc("POST /api/admin/cases", a(h.CreateCase))
	mux.HandleFunc("GET /api/admin/cases/{id}", a(h.GetCase))
	mux.HandleFunc("PATCH /api/admin/cases/{id}", a(h.UpdateCase))
	mux.HandleFunc("DELETE /api/admin/cases/{id}", a(h.DeleteCase))
	mux.HandleFunc("POST /api/admin/cases/{id}/suspects", a(h.CreateSuspect))
	mux.HandleFunc("PATCH /api/admin/cases/{id}/suspects/{suspect_id}", a(h.UpdateSuspect))
	mux.HandleFunc("DELETE /api/admin/cases/{id}/suspects/{suspect_id}", a(h.DeleteSuspect))
	mux.HandleFunc("POST /api/admin/cases/{id}/scenes", a(h.CreateScene))
	return mux
}

func serve(store *fakeStore, models *fakeModels, method, path, body string) *httptest.ResponseRecorder {
	return serveWith(newTestHandler(store, models), method, path, body)
}

func serveWith(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
	return rec
}

func admin() *fakeStore { return &fakeStore{role: "admin"} }

const newCase = `{"title":"T","synopsis":"S","difficulty_level":"easy","reward_trophies":1`

func TestNonAdminForbidden(t *testing.T) {
	s := &fakeStore{}
	if rec := serve(s, &fakeModels{}, "POST", "/api/admin/cases", newCase+"}"); rec.Code != 403 || s.created != nil {
		t.Fatalf("status %d", rec.Code)
	}
	if rec := serve(s, &fakeModels{}, "PATCH", "/api/admin/cases/"+caseID, `{"llm_model":"a/b"}`); rec.Code != 403 || s.patched != nil {
		t.Fatalf("status %d", rec.Code)
	}
	if rec := serve(s, &fakeModels{}, "GET", "/api/admin/cases/"+caseID, ""); rec.Code != 403 || strings.Contains(rec.Body.String(), "did it") {
		t.Fatalf("status %d %s", rec.Code, rec.Body)
	}
}

func TestAdminOnlyChecksTokenThenDatabase(t *testing.T) {
	role := "user"
	tokenRole := WithTokenRole(func(context.Context) string { return role })

	// A player's token is refused without asking the database.
	if rec := serveWith(newTestHandler(admin(), &fakeModels{}, tokenRole), "GET", "/api/admin/cases", ""); rec.Code != 403 {
		t.Fatalf("player token: %d", rec.Code)
	}
	// An admin token for someone who was demoted since is refused by the database check.
	role = "admin"
	if rec := serveWith(newTestHandler(&fakeStore{role: "user"}, &fakeModels{}, tokenRole), "GET", "/api/admin/cases", ""); rec.Code != 403 {
		t.Fatalf("demoted admin: %d", rec.Code)
	}
	if rec := serveWith(newTestHandler(admin(), &fakeModels{}, tokenRole), "GET", "/api/admin/cases", ""); rec.Code != 200 {
		t.Fatalf("admin: %d %s", rec.Code, rec.Body)
	}
}

func TestCreateCaseModel(t *testing.T) {
	// No model: the default Gemma model, checked with OpenRouter.
	s, m := admin(), &fakeModels{}
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
	s = admin()
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
		s := admin()
		if rec := serve(s, c.models, "POST", "/api/admin/cases", c.body); rec.Code != c.status || s.created != nil {
			t.Errorf("%s: status %d, want %d: %s", name, rec.Code, c.status, rec.Body)
		}
	}

	// Without an OpenRouter key only the format is checked.
	s = admin()
	if rec := serve(s, &fakeModels{disabled: true, err: llm.ErrUnknownModel}, "POST", "/api/admin/cases", newCase+`,"llm_model":"a/b"}`); rec.Code != 201 {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestUpdateCase(t *testing.T) {
	s := admin()
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
		if rec := serve(admin(), &fakeModels{}, "PATCH", c.path, c.body); rec.Code != c.status {
			t.Errorf("%s: status %d, want %d: %s", name, rec.Code, c.status, rec.Body)
		}
	}
	// Publishing a case that is not ready is a conflict with the field named.
	s = admin()
	s.err = ErrNotReady
	if rec := serve(s, &fakeModels{}, "PATCH", "/api/admin/cases/"+caseID, `{"is_published":true}`); rec.Code != 409 || !strings.Contains(rec.Body.String(), "is_published") {
		t.Fatalf("not ready: %d %s", rec.Code, rec.Body)
	}
}

func TestMusicTrack(t *testing.T) {
	s := admin()
	if rec := serve(s, &fakeModels{}, "POST", "/api/admin/cases", newCase+`,"music_track":"bloodmoney"}`); rec.Code != 201 || s.created.MusicTrack != "bloodmoney" {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	s = admin()
	if rec := serve(s, &fakeModels{}, "POST", "/api/admin/cases", newCase+"}"); rec.Code != 201 || s.created.MusicTrack != "" {
		t.Fatalf("default main theme: %d %+v", rec.Code, s.created)
	}
	if rec := serve(admin(), &fakeModels{}, "POST", "/api/admin/cases", newCase+`,"music_track":"../main"}`); rec.Code != 422 {
		t.Fatalf("unknown track: %d", rec.Code)
	}
	// PATCH "" switches back to the main theme; an unknown key is refused.
	s = admin()
	if rec := serve(s, &fakeModels{}, "PATCH", "/api/admin/cases/"+caseID, `{"music_track":""}`); rec.Code != 200 || s.patched.MusicTrack == nil || *s.patched.MusicTrack != "" {
		t.Fatalf("clear: %d %s", rec.Code, rec.Body)
	}
	if rec := serve(admin(), &fakeModels{}, "PATCH", "/api/admin/cases/"+caseID, `{"music_track":"main"}`); rec.Code != 422 {
		t.Fatalf("main is not a case track: %d", rec.Code)
	}
}

func TestHints(t *testing.T) {
	// Up to three hints are saved in order; a case without hints gets an empty list.
	s := admin()
	if rec := serve(s, &fakeModels{}, "POST", "/api/admin/cases", newCase+`,"hints":["a","b","c"]}`); rec.Code != 201 || len(s.created.Hints) != 3 || s.created.Hints[2] != "c" {
		t.Fatalf("three hints: %d %s %+v", rec.Code, rec.Body, s.created)
	}
	s = admin()
	if rec := serve(s, &fakeModels{}, "POST", "/api/admin/cases", newCase+"}"); rec.Code != 201 || s.created.Hints == nil {
		t.Fatalf("no hints: %d %+v", rec.Code, s.created)
	}
	// Too many, empty or oversized hints are refused.
	for name, body := range map[string]string{
		"four":  newCase + `,"hints":["a","b","c","d"]}`,
		"empty": newCase + `,"hints":["a",""]}`,
		"long":  newCase + `,"hints":["` + strings.Repeat("x", 1001) + `"]}`,
	} {
		if rec := serve(admin(), &fakeModels{}, "POST", "/api/admin/cases", body); rec.Code != 422 {
			t.Errorf("%s: status %d, want 422", name, rec.Code)
		}
	}
	// PATCH: hints can be replaced or cleared with [], and left alone by omitting them.
	s = admin()
	if rec := serve(s, &fakeModels{}, "PATCH", "/api/admin/cases/"+caseID, `{"hints":["only"]}`); rec.Code != 200 || len(s.patched.Hints) != 1 {
		t.Fatalf("replace: %d %+v", rec.Code, s.patched)
	}
	s = admin()
	if rec := serve(s, &fakeModels{}, "PATCH", "/api/admin/cases/"+caseID, `{"hints":[]}`); rec.Code != 200 || s.patched.Hints == nil || len(s.patched.Hints) != 0 {
		t.Fatalf("clear: %d %+v", rec.Code, s.patched)
	}
	s = admin()
	if rec := serve(s, &fakeModels{}, "PATCH", "/api/admin/cases/"+caseID, `{"title":"x"}`); rec.Code != 200 || s.patched.Hints != nil {
		t.Fatalf("omitted: %d %+v", rec.Code, s.patched)
	}
}

func TestDeleteCase(t *testing.T) {
	imgs := &fakeImages{}
	owned := imgs.PublicURL("cases/" + caseID + "/thumbnail/0e3c1a4e-2f1b-4b8e-9d6a-1c2b3d4e5f60.webp")
	s := admin()
	s.caseImages = []string{owned, imgs.PublicURL("cases/case-01/thumbnail.webp"), "https://elsewhere.test/x.webp"}
	if rec := serveWith(newTestHandler(s, &fakeModels{}, WithImages(imgs)), "DELETE", "/api/admin/cases/"+caseID, ""); rec.Code != 200 || !s.deleted {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body)
	}
	// Only the image the admin panel uploaded is removed; assetctl art and external URLs stay.
	if len(imgs.deleted) != 1 || !strings.HasSuffix(owned, imgs.deleted[0]) {
		t.Fatalf("deleted %v", imgs.deleted)
	}

	s = admin()
	s.err = ErrInUse
	if rec := serve(s, &fakeModels{}, "DELETE", "/api/admin/cases/"+caseID, ""); rec.Code != 409 {
		t.Fatalf("in use: %d", rec.Code)
	}
}

func TestReplacedThumbnailIsDeleted(t *testing.T) {
	imgs := &fakeImages{}
	s := admin()
	s.oldImage = imgs.PublicURL("cases/" + caseID + "/thumbnail/0e3c1a4e-2f1b-4b8e-9d6a-1c2b3d4e5f60.webp")
	body := `{"thumbnail_url":"` + imgs.PublicURL("cases/"+caseID+"/thumbnail/7a1d2c3b-4e5f-4a6b-8c7d-9e0f1a2b3c4d.webp") + `"}`
	if rec := serveWith(newTestHandler(s, &fakeModels{}, WithImages(imgs)), "PATCH", "/api/admin/cases/"+caseID, body); rec.Code != 200 {
		t.Fatalf("status %d %s", rec.Code, rec.Body)
	}
	if len(imgs.deleted) != 1 || !strings.Contains(imgs.deleted[0], "0e3c1a4e") {
		t.Fatalf("deleted %v", imgs.deleted)
	}
}

const newSuspect = `{"name":"Mrs Peacock","role":"the housekeeper","initial_testimony":"I was in the kitchen."`

func TestCreateSuspect(t *testing.T) {
	s := admin()
	body := newSuspect + `,"is_culprit":true,"public_alibi":"kitchen","hidden_truth":"poisoned the tea","confession_triggers":"the receipt","confession_keywords":["tea","last+study"]}`
	rec := serve(s, &fakeModels{}, "POST", "/api/admin/cases/"+caseID+"/suspects", body)
	if rec.Code != 201 || s.suspectIn == nil || !s.suspectIn.IsCulprit || s.suspectIn.HiddenTruth != "poisoned the tea" || len(s.suspectIn.ConfessionKeywords) != 2 {
		t.Fatalf("status %d %s %+v", rec.Code, rec.Body, s.suspectIn)
	}

	for name, c := range map[string]struct {
		path, body string
		status     int
	}{
		"no name":        {"/api/admin/cases/" + caseID + "/suspects", `{"role":"r","initial_testimony":"t"}`, 422},
		"no testimony":   {"/api/admin/cases/" + caseID + "/suspects", `{"name":"n","role":"r"}`, 422},
		"bad keyword":    {"/api/admin/cases/" + caseID + "/suspects", newSuspect + `,"confession_keywords":["Tea Time"]}`, 422},
		"plus only":      {"/api/admin/cases/" + caseID + "/suspects", newSuspect + `,"confession_keywords":["tea+"]}`, 422},
		"http avatar":    {"/api/admin/cases/" + caseID + "/suspects", newSuspect + `,"avatar_url":"http://x.test/a.webp"}`, 422},
		"bad case id":    {"/api/admin/cases/nope/suspects", newSuspect + "}", 400},
		"unknown field":  {"/api/admin/cases/" + caseID + "/suspects", newSuspect + `,"system_prompt":"x"}`, 400},
		"negative order": {"/api/admin/cases/" + caseID + "/suspects", newSuspect + `,"display_order":-1}`, 422},
	} {
		if rec := serve(admin(), &fakeModels{}, "POST", c.path, c.body); rec.Code != c.status {
			t.Errorf("%s: status %d, want %d: %s", name, rec.Code, c.status, rec.Body)
		}
	}

	for err, status := range map[error]int{ErrNeedsKeywords: 422, ErrNameTaken: 409, ErrNotFound: 404, ErrNotReady: 409} {
		s := admin()
		s.err = err
		if rec := serve(s, &fakeModels{}, "POST", "/api/admin/cases/"+caseID+"/suspects", newSuspect+"}"); rec.Code != status {
			t.Errorf("%v: status %d, want %d", err, rec.Code, status)
		}
	}
}

func TestUpdateSuspect(t *testing.T) {
	s := admin()
	path := "/api/admin/cases/" + caseID + "/suspects/" + suspectID
	if rec := serve(s, &fakeModels{}, "PATCH", path, `{"is_culprit":false,"confession_keywords":[]}`); rec.Code != 200 ||
		s.suspectP.IsCulprit == nil || *s.suspectP.IsCulprit || s.suspectP.ConfessionKeywords == nil || s.suspectP.Name != nil {
		t.Fatalf("status %d %s %+v", rec.Code, rec.Body, s.suspectP)
	}
	if rec := serve(admin(), &fakeModels{}, "PATCH", path, `{}`); rec.Code != 422 {
		t.Fatalf("empty: %d", rec.Code)
	}
	if rec := serve(admin(), &fakeModels{}, "PATCH", "/api/admin/cases/"+caseID+"/suspects/nope", `{"name":"x"}`); rec.Code != 400 {
		t.Fatalf("bad suspect id: %d", rec.Code)
	}
}

func TestCreateSceneNeedsHTTPSImage(t *testing.T) {
	path := "/api/admin/cases/" + caseID + "/scenes"
	if rec := serve(admin(), &fakeModels{}, "POST", path, `{"scene_name":"Study","image_url":"https://x.test/a.webp","display_order":1}`); rec.Code != 201 {
		t.Fatalf("status %d %s", rec.Code, rec.Body)
	}
	if rec := serve(admin(), &fakeModels{}, "POST", path, `{"scene_name":"Study"}`); rec.Code != 422 {
		t.Fatalf("no image: %d", rec.Code)
	}
}

func TestConfig(t *testing.T) {
	fs := &fakeSettings{cur: settings.Settings{DailyTokens: 10, ResetTime: "00:00", Timezone: "UTC"}}
	h := newTestHandler(admin(), &fakeModels{}, WithSettings(fs))
	rec := serveWith(h, "GET", "/api/admin/config", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"daily_tokens":10`) || !strings.Contains(rec.Body.String(), `"token_reset_time":"00:00"`) {
		t.Fatalf("get: %d %s", rec.Code, rec.Body)
	}
	rec = serveWith(h, "PUT", "/api/admin/config", `{"daily_tokens":7,"token_reset_time":"04:30","token_reset_timezone":"Asia/Kolkata"}`)
	if rec.Code != 200 || fs.saved == nil || fs.saved.DailyTokens != 7 || fs.saved.ResetTime != "04:30" || fs.saved.Timezone != "Asia/Kolkata" {
		t.Fatalf("put: %d %s %+v", rec.Code, rec.Body, fs.saved)
	}
	for name, body := range map[string]string{
		"zero tokens": `{"daily_tokens":0,"token_reset_time":"04:30","token_reset_timezone":"UTC"}`,
		"too many":    `{"daily_tokens":11,"token_reset_time":"04:30","token_reset_timezone":"UTC"}`,
		"bad time":    `{"daily_tokens":5,"token_reset_time":"24:00","token_reset_timezone":"UTC"}`,
		"no colon":    `{"daily_tokens":5,"token_reset_time":"0430","token_reset_timezone":"UTC"}`,
		"bad zone":    `{"daily_tokens":5,"token_reset_time":"04:30","token_reset_timezone":"Mars/Olympus"}`,
		"local zone":  `{"daily_tokens":5,"token_reset_time":"04:30","token_reset_timezone":"Local"}`,
		"missing":     `{"daily_tokens":5}`,
	} {
		fs.saved = nil
		if rec := serveWith(h, "PUT", "/api/admin/config", body); rec.Code != 422 || fs.saved != nil {
			t.Errorf("%s: status %d, want 422: %s", name, rec.Code, rec.Body)
		}
	}
	// Without the settings service the routes say so instead of crashing.
	if rec := serve(admin(), &fakeModels{}, "GET", "/api/admin/config", ""); rec.Code != 503 {
		t.Fatalf("no settings: %d", rec.Code)
	}
}

func multipartBody(t *testing.T, fields map[string]string, file []byte) (*bytes.Buffer, string) {
	t.Helper()
	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	for k, v := range fields {
		_ = mw.WriteField(k, v)
	}
	if file != nil {
		fw, _ := mw.CreateFormFile("file", "pic.png")
		_, _ = fw.Write(file)
	}
	_ = mw.Close()
	return &b, mw.FormDataContentType()
}

func pngOf(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func upload(h http.Handler, body *bytes.Buffer, ct string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/api/admin/upload", body)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestUpload(t *testing.T) {
	imgs := &fakeImages{}
	h := newTestHandler(admin(), &fakeModels{}, WithImages(imgs))

	body, ct := multipartBody(t, map[string]string{"kind": "suspect", "case_id": caseID}, pngOf(t, 2000, 1000))
	rec := upload(h, body, ct)
	if rec.Code != 201 {
		t.Fatalf("status %d %s", rec.Code, rec.Body)
	}
	var env struct{ Data uploadResp }
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if !ownedImage.MatchString(env.Data.Path) || !strings.HasPrefix(env.Data.Path, "cases/"+caseID+"/suspects/") || env.Data.URL != imgs.PublicURL(env.Data.Path) {
		t.Fatalf("response %+v", env.Data)
	}
	// Stored as WebP, scaled so the longest side is 768.
	data := imgs.uploaded[env.Data.Path]
	if !storage.IsWebP(data) {
		t.Fatal("not WebP")
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width != 768 || cfg.Height != 384 {
		t.Fatalf("size %dx%d %v", cfg.Width, cfg.Height, err)
	}

	for name, c := range map[string]struct {
		fields map[string]string
		file   []byte
		status int
	}{
		"bad kind":     {map[string]string{"kind": "banner", "case_id": caseID}, pngOf(t, 10, 10), 422},
		"bad case id":  {map[string]string{"kind": "scene", "case_id": "nope"}, pngOf(t, 10, 10), 422},
		"unknown case": {map[string]string{"kind": "scene", "case_id": "22222222-2222-2222-2222-222222222222"}, pngOf(t, 10, 10), 404},
		"no file":      {map[string]string{"kind": "scene", "case_id": caseID}, nil, 422},
		"not an image": {map[string]string{"kind": "scene", "case_id": caseID}, []byte("<svg></svg>"), 422},
	} {
		body, ct := multipartBody(t, c.fields, c.file)
		if rec := upload(h, body, ct); rec.Code != c.status {
			t.Errorf("%s: status %d, want %d: %s", name, rec.Code, c.status, rec.Body)
		}
	}

	// Without storage the endpoint answers 503.
	body, ct = multipartBody(t, map[string]string{"kind": "scene", "case_id": caseID}, pngOf(t, 10, 10))
	if rec := upload(newTestHandler(admin(), &fakeModels{}), body, ct); rec.Code != 503 {
		t.Fatalf("no storage: %d", rec.Code)
	}
}

func TestListModels(t *testing.T) {
	m := &fakeModels{list: []llm.ModelInfo{{ID: "google/gemma-4-31b-it", Name: "Gemma"}}}
	rec := serve(admin(), m, "GET", "/api/admin/models", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"id":"google/gemma-4-31b-it"`) || !strings.Contains(rec.Body.String(), `"default":"`+llm.DefaultModel+`"`) {
		t.Fatalf("status %d %s", rec.Code, rec.Body)
	}
	if rec := serve(admin(), &fakeModels{disabled: true}, "GET", "/api/admin/models", ""); rec.Code != 503 {
		t.Fatalf("no key: %d", rec.Code)
	}
	if rec := serve(admin(), &fakeModels{err: context.DeadlineExceeded}, "GET", "/api/admin/models", ""); rec.Code != 503 {
		t.Fatalf("openrouter down: %d", rec.Code)
	}
	if rec := serve(&fakeStore{}, m, "GET", "/api/admin/models", ""); rec.Code != 403 {
		t.Fatalf("player: %d", rec.Code)
	}
}

func TestGrantTokens(t *testing.T) {
	w := &fakeWallet{}
	s := admin()
	s.balance = 7
	h := newTestHandler(s, &fakeModels{}, WithTokens(w))
	rec := serveWith(h, "POST", "/api/admin/users/"+suspectID+"/tokens", `{"amount":5}`)
	// 7 + 5 stops at 10: 3 were added and the response says it was capped.
	if rec.Code != 200 || len(w.granted) != 1 || w.granted[0] != 5 ||
		!strings.Contains(rec.Body.String(), `"granted":3`) || !strings.Contains(rec.Body.String(), `"capped":true`) {
		t.Fatalf("status %d %s %v", rec.Code, rec.Body, w.granted)
	}
	for name, c := range map[string]struct {
		path, body string
		status     int
	}{
		"zero":      {"/api/admin/users/" + suspectID + "/tokens", `{"amount":0}`, 422},
		"too many":  {"/api/admin/users/" + suspectID + "/tokens", `{"amount":11}`, 422},
		"negative":  {"/api/admin/users/" + suspectID + "/tokens", `{"amount":-3}`, 422},
		"bad id":    {"/api/admin/users/nope/tokens", `{"amount":1}`, 400},
		"no player": {"/api/admin/users/" + caseID + "/tokens", `{"amount":1}`, 404},
	} {
		w.granted = nil
		if rec := serveWith(h, "POST", c.path, c.body); rec.Code != c.status || len(w.granted) != 0 {
			t.Errorf("%s: status %d, want %d: %s", name, rec.Code, c.status, rec.Body)
		}
	}
	if rec := serve(&fakeStore{}, &fakeModels{}, "POST", "/api/admin/users/"+suspectID+"/tokens", `{"amount":1}`); rec.Code != 403 {
		t.Fatalf("player: %d", rec.Code)
	}
	if rec := serve(admin(), &fakeModels{}, "POST", "/api/admin/users/"+suspectID+"/tokens", `{"amount":1}`); rec.Code != 503 {
		t.Fatalf("no wallet: %d", rec.Code)
	}
}

func TestSearchUsers(t *testing.T) {
	s := admin()
	rec := serve(s, &fakeModels{}, "GET", "/api/admin/users?q=+rook+", "")
	if rec.Code != 200 || s.searched == nil || *s.searched != "rook" || !strings.Contains(rec.Body.String(), "rookie") {
		t.Fatalf("status %d %s", rec.Code, rec.Body)
	}
	if rec := serve(admin(), &fakeModels{}, "GET", "/api/admin/users?q="+strings.Repeat("x", 101), ""); rec.Code != 400 {
		t.Fatalf("long query: %d", rec.Code)
	}
}
