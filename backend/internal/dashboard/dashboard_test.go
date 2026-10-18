package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"backend/internal/user"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

const uid = "11111111-1111-1111-1111-111111111111"

type fakeStore struct {
	stories []Story
	stats   UserStats
	trErr   error
	active  *ActiveCase
	players []Player
	names   map[string]string
	err     error
}

func (f *fakeStore) ListStories(context.Context) ([]Story, error) { return f.stories, f.err }
func (f *fakeStore) Stats(context.Context, string) (UserStats, error) {
	return f.stats, errors.Join(f.trErr, f.err)
}
func (f *fakeStore) ActiveCase(context.Context, string) (*ActiveCase, error) { return f.active, f.err }
func (f *fakeStore) TopPlayers(context.Context, int) ([]Player, error)       { return f.players, f.err }
func (f *fakeStore) Usernames(context.Context, []string) (map[string]string, error) {
	return f.names, f.err
}

type fakeBalance struct {
	n   int64
	err error
}

func (f fakeBalance) Balance(context.Context, string) (int64, error) { return f.n, f.err }

func newHandler(t *testing.T, st *fakeStore, bal fakeBalance) (*Handler, *Leaderboard, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	board := NewLeaderboard(rdb)
	return NewHandler(st, bal, board, func(context.Context) string { return uid }), board, mr
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
	h, _, _ := newHandler(t, st, fakeBalance{})
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
	h, _, _ := newHandler(t, &fakeStore{stories: []Story{}}, fakeBalance{})
	if rec := call(t, h.Stories, nil); rec.Body.String() != `{"status":"success","data":[]}`+"\n" {
		t.Fatalf("%q", rec.Body)
	}
}

func TestStats(t *testing.T) {
	st := &fakeStore{stats: UserStats{Trophies: 7, CasesSolved: 4}}
	h, _, _ := newHandler(t, st, fakeBalance{n: 10})
	var got statsResp
	if rec := call(t, h.Stats, &got); rec.Code != 200 || got.Trophies != 7 || got.Tokens != 10 || got.CasesSolved != 4 || got.ActiveCaseTitle != nil {
		t.Fatalf("free user: %d %s", rec.Code, rec.Body)
	}

	st.active = &ActiveCase{CaseID: "c1", Title: "The Locked Study"}
	got = statsResp{}
	call(t, h.Stats, &got)
	if got.ActiveCaseTitle == nil || *got.ActiveCaseTitle != "The Locked Study" {
		t.Fatalf("locked user: %+v", got)
	}
}

func TestStatsErrors(t *testing.T) {
	h, _, _ := newHandler(t, &fakeStore{trErr: user.ErrNotFound}, fakeBalance{})
	if rec := call(t, h.Stats, nil); rec.Code != 404 {
		t.Fatalf("unknown user: %d", rec.Code)
	}
	h, _, _ = newHandler(t, &fakeStore{}, fakeBalance{err: errors.New("boom")})
	if rec := call(t, h.Stats, nil); rec.Code != 500 {
		t.Fatalf("balance error: %d", rec.Code)
	}
}

func TestLeaderboardRanksFromRedis(t *testing.T) {
	st := &fakeStore{names: map[string]string{"a": "ann", "b": "bob", "c": "cy"}}
	h, board, _ := newHandler(t, st, fakeBalance{})
	ctx := context.Background()
	for id, n := range map[string]int64{"a": 5, "b": 30, "c": 12, "gone": 99} { // "gone" has no user row
		if err := board.Set(ctx, id, n); err != nil {
			t.Fatal(err)
		}
	}
	var got []rankedPlayer
	if rec := call(t, h.Leaderboard, &got); rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	want := []rankedPlayer{{1, "b", "bob", 30}, {2, "c", "cy", 12}, {3, "a", "ann", 5}}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("row %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestLeaderboardCapsAtTen(t *testing.T) {
	st := &fakeStore{names: map[string]string{}}
	h, board, _ := newHandler(t, st, fakeBalance{})
	for i := range 15 {
		id := string(rune('a' + i))
		st.names[id] = id
		_ = board.Set(context.Background(), id, int64(i+1))
	}
	var got []rankedPlayer
	call(t, h.Leaderboard, &got)
	if len(got) != 10 || got[0].Trophies != 15 || got[9].Trophies != 6 {
		t.Fatalf("got %d rows, first %+v last %+v", len(got), got[0], got[len(got)-1])
	}
}

func TestLeaderboardFallsBackToPostgres(t *testing.T) {
	st := &fakeStore{players: []Player{{"a", "ann", 9}, {"b", "bob", 4}}}
	h, _, mr := newHandler(t, st, fakeBalance{})

	var got []rankedPlayer
	call(t, h.Leaderboard, &got) // empty sorted set
	if len(got) != 2 || got[0].Username != "ann" || got[1].Rank != 2 {
		t.Fatalf("empty set: %+v", got)
	}

	mr.Close() // Redis unreachable
	got = nil
	if rec := call(t, h.Leaderboard, &got); rec.Code != 200 || len(got) != 2 {
		t.Fatalf("redis down: %d %s", rec.Code, rec.Body)
	}
}

func TestActiveCase(t *testing.T) {
	st := &fakeStore{}
	h, _, _ := newHandler(t, st, fakeBalance{})

	if rec := call(t, h.ActiveCase, nil); rec.Body.String() != `{"status":"success","data":{"case_id":null,"story_title":null}}`+"\n" {
		t.Fatalf("free user: %q", rec.Body)
	}
	st.active = &ActiveCase{CaseID: "22222222-2222-2222-2222-222222222222", Title: "The Locked Study"}
	var got activeCaseResp
	call(t, h.ActiveCase, &got)
	if got.CaseID == nil || *got.CaseID != st.active.CaseID || got.StoryTitle == nil || *got.StoryTitle != "The Locked Study" {
		t.Fatalf("locked user: %+v", got)
	}
}
