package accusation

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

const (
	caseID    = "11111111-1111-4111-8111-111111111111"
	suspectID = "22222222-2222-4222-8222-222222222222"
)

type fakeStore struct {
	out *Outcome
	err error
}

func (f fakeStore) Accuse(context.Context, string, string, string) (*Outcome, error) {
	return f.out, f.err
}

type fakeBoard struct {
	calls []int64
	err   error
}

func (f *fakeBoard) Incr(_ context.Context, _ string, d int64) error {
	f.calls = append(f.calls, d)
	return f.err
}

func do(h *Handler, id, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/cases/"+id+"/accuse", strings.NewReader(body))
	req.SetPathValue("id", id)
	rec := httptest.NewRecorder()
	h.Accuse(rec, req)
	return rec
}

func newHandler(s Store, b Leaderboard) *Handler {
	return NewHandler(s, b, func(context.Context) string { return "user-1" })
}

func TestAccuse(t *testing.T) {
	win := &Outcome{Correct: true, Trophies: 30, Change: 20, CulpritName: "Ada", Resolution: "Ada did it."}
	loss := &Outcome{Correct: false, Trophies: 0, Change: -3, CulpritName: "Ada", Resolution: "Ada did it."}
	good := `{"suspect_id":"` + suspectID + `"}`
	tests := []struct {
		name   string
		store  fakeStore
		id     string
		body   string
		status int
		board  []int64
	}{
		{"win", fakeStore{out: win}, caseID, good, 200, []int64{20}},
		{"loss moves the board by what really changed", fakeStore{out: loss}, caseID, good, 200, []int64{-3}},
		{"no change leaves the board alone", fakeStore{out: &Outcome{}}, caseID, good, 200, nil},
		{"not locked", fakeStore{err: ErrNotLocked}, caseID, good, 403, nil},
		{"suspect of another case", fakeStore{err: ErrBadSuspect}, caseID, good, 422, nil},
		{"database error", fakeStore{err: errors.New("boom")}, caseID, good, 500, nil},
		{"bad case id", fakeStore{}, "nope", good, 400, nil},
		{"missing suspect", fakeStore{}, caseID, `{}`, 422, nil},
		{"bad suspect id", fakeStore{}, caseID, `{"suspect_id":"x"}`, 422, nil},
		{"unknown field", fakeStore{}, caseID, `{"suspect_id":"` + suspectID + `","trophies":999}`, 400, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := &fakeBoard{}
			rec := do(newHandler(tc.store, b), tc.id, tc.body)
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.status, rec.Body)
			}
			if len(b.calls) != len(tc.board) || (len(b.calls) == 1 && b.calls[0] != tc.board[0]) {
				t.Fatalf("leaderboard calls = %v, want %v", b.calls, tc.board)
			}
		})
	}
}

func TestAccuseResponseShape(t *testing.T) {
	rec := do(newHandler(fakeStore{out: &Outcome{Correct: true, Trophies: 30, Change: 20, Resolution: "Ada did it."}}, &fakeBoard{}), caseID, `{"suspect_id":"`+suspectID+`"}`)
	var env struct {
		Status string
		Data   map[string]any
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Status != "success" || env.Data["correct"] != true || env.Data["trophies"] != float64(30) || env.Data["resolution"] != "Ada did it." {
		t.Fatalf("unexpected body: %s", rec.Body)
	}
	if _, ok := env.Data["tokens"]; ok {
		t.Fatal("a win must not carry a token reward")
	}
}

func TestLeaderboardFailureDoesNotFailTheAccusation(t *testing.T) {
	rec := do(newHandler(fakeStore{out: &Outcome{Correct: true, Change: 5}}, &fakeBoard{err: errors.New("redis down")}), caseID, `{"suspect_id":"`+suspectID+`"}`)
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestRedisLeaderboardZIncrBy(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	b := NewRedisLeaderboard(rdb)
	ctx := context.Background()
	if err := b.Incr(ctx, "u1", 20); err != nil {
		t.Fatal(err)
	}
	if err := b.Incr(ctx, "u1", -5); err != nil {
		t.Fatal(err)
	}
	if got, _ := mr.ZScore(LeaderboardKey, "u1"); got != 15 {
		t.Fatalf("score = %v, want 15", got)
	}
}
