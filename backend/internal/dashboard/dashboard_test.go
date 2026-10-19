package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"backend/internal/user"
)

const uid = "11111111-1111-1111-1111-111111111111"

type fakeStore struct {
	stories []Story
	stats   UserStats
	trErr   error
	active  *ActiveCase
	players []Player
	err     error
}

func (f *fakeStore) ListStories(context.Context, string) ([]Story, error) { return f.stories, f.err }
func (f *fakeStore) Stats(context.Context, string) (UserStats, error) {
	return f.stats, errors.Join(f.trErr, f.err)
}
func (f *fakeStore) ActiveCase(context.Context, string) (*ActiveCase, error) { return f.active, f.err }
func (f *fakeStore) TopPlayers(context.Context, int) ([]Player, error)       { return f.players, f.err }
func (f *fakeStore) Usernames(_ context.Context, ids []string) (map[string]string, error) {
	m := map[string]string{}
	for _, p := range f.players {
		m[p.ID] = p.Username
	}
	return m, f.err
}

type fakeBalance struct {
	n   int64
	err error
}

func (f fakeBalance) Balance(context.Context, string) (int64, error) { return f.n, f.err }

func newHandler(st *fakeStore, bal fakeBalance) *Handler {
	return NewHandler(st, bal, func(context.Context) string { return uid })
}

func call(t *testing.T, fn http.HandlerFunc, out any) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	fn(rec, httptest.NewRequest("GET", "/", nil))
	if out != nil && rec.Code == 200 {
		var env struct {
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(env.Data, out); err != nil {
			t.Fatal(err)
		}
	}
	return rec
}

func TestStories(t *testing.T) {
	st := &fakeStore{stories: []Story{{ID: "s1", Title: "The Locked Study", Synopsis: "x", ThumbnailURL: "u", DifficultyLevel: "hard", RewardTrophies: 5}}}
	h := newHandler(st, fakeBalance{})
	var got []map[string]any
	if rec := call(t, h.Stories, &got); rec.Code != 200 || len(got) != 1 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	for _, k := range []string{"id", "title", "synopsis", "thumbnail_url", "difficulty_level", "reward_trophies"} {
		if _, ok := got[0][k]; !ok {
			t.Errorf("missing field %q in %v", k, got[0])
		}
	}

	st.stories, st.err = nil, errors.New("pg down")
	if rec := call(t, h.Stories, nil); rec.Code != 500 || json.Valid(rec.Body.Bytes()) == false {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}

func TestStoriesEmptyIsArrayNotNull(t *testing.T) {
	h := newHandler(&fakeStore{stories: []Story{}}, fakeBalance{})
	if rec := call(t, h.Stories, nil); rec.Body.String() != `{"status":"success","data":[]}`+"\n" {
		t.Fatalf("%q", rec.Body)
	}
}

func TestStats(t *testing.T) {
	st := &fakeStore{stats: UserStats{Trophies: 7, CasesSolved: 4}}
	h := newHandler(st, fakeBalance{n: 10})
	var got statsResp
	if rec := call(t, h.Stats, &got); rec.Code != 200 || got.Trophies != 7 || got.Tokens != 10 || got.CasesSolved != 4 || got.ActiveCaseTitle != nil || got.ActiveMusic != nil {
		t.Fatalf("free user: %d %s", rec.Code, rec.Body)
	}

	st.active = &ActiveCase{CaseID: "c1", StoryID: "s1", Title: "The Locked Study", Music: "bloodmoney"}
	got = statsResp{}
	call(t, h.Stats, &got)
	if got.ActiveCaseTitle == nil || *got.ActiveCaseTitle != "The Locked Study" || got.ActiveStoryID == nil || *got.ActiveStoryID != "s1" ||
		got.ActiveMusic == nil || *got.ActiveMusic != "bloodmoney" {
		t.Fatalf("locked user: %+v", got)
	}
}

func TestStatsErrors(t *testing.T) {
	h := newHandler(&fakeStore{trErr: user.ErrNotFound}, fakeBalance{})
	if rec := call(t, h.Stats, nil); rec.Code != 404 {
		t.Fatalf("unknown user: %d", rec.Code)
	}
	h = newHandler(&fakeStore{}, fakeBalance{err: errors.New("boom")})
	if rec := call(t, h.Stats, nil); rec.Code != 500 {
		t.Fatalf("balance error: %d", rec.Code)
	}
}

func TestLeaderboardRanksAndMarksCaller(t *testing.T) {
	st := &fakeStore{players: []Player{{"x", "bob", 30}, {uid, "ann", 9}, {"y", "cy", 4}}}
	h := newHandler(st, fakeBalance{})
	var got []map[string]any
	if rec := call(t, h.Leaderboard, &got); rec.Code != 200 || len(got) != 3 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	for i, wantMe := range []bool{false, true, false} {
		if got[i]["is_me"] != wantMe || got[i]["rank"] != float64(i+1) {
			t.Fatalf("row %d = %v", i, got[i])
		}
		if _, leaked := got[i]["user_id"]; leaked {
			t.Fatalf("row %d exposes the internal user id: %v", i, got[i])
		}
	}
}

func TestLeaderboardError(t *testing.T) {
	h := newHandler(&fakeStore{err: errors.New("pg down")}, fakeBalance{})
	if rec := call(t, h.Leaderboard, nil); rec.Code != 500 {
		t.Fatalf("%d", rec.Code)
	}
}

func TestActiveCase(t *testing.T) {
	st := &fakeStore{}
	h := newHandler(st, fakeBalance{})

	if rec := call(t, h.ActiveCase, nil); rec.Body.String() != `{"status":"success","data":{"case_id":null,"story_id":null,"story_title":null}}`+"\n" {
		t.Fatalf("free user: %q", rec.Body)
	}
	st.active = &ActiveCase{CaseID: "22222222-2222-2222-2222-222222222222", StoryID: "s1", Title: "The Locked Study"}
	var got activeCaseResp
	call(t, h.ActiveCase, &got)
	if got.CaseID == nil || *got.CaseID != st.active.CaseID || got.StoryID == nil || *got.StoryID != "s1" || got.StoryTitle == nil || *got.StoryTitle != "The Locked Study" {
		t.Fatalf("locked user: %+v", got)
	}
}
