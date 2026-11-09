package dashboard

import (
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func boardHandler(t *testing.T, st *fakeStore) (*Handler, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return newHandler(st, fakeBalance{}).WithBoard(NewRedisBoard(rdb)), mr
}

type row struct {
	Rank     int
	Username string
	Trophies int64
	IsMe     bool `json:"is_me"`
}

func TestLeaderboardRedisOrderAndSync(t *testing.T) {
	players := []Player{{ID: uid, Username: "ann", Trophies: 30}, {ID: "p2", Username: "bob", Trophies: 20}}
	st := &fakeStore{players: players}
	h, mr := boardHandler(t, st)
	call(t, h.Leaderboard, nil) // fills Redis

	mr.ZAdd(LeaderboardKey, 35, "p2") // as ZINCRBY +15 would: bob 35 overtakes ann 30
	var got []row
	call(t, h.Leaderboard, &got)
	if len(got) != 2 || got[0].Username != "bob" || got[0].Trophies != 35 || got[0].IsMe || !got[1].IsMe {
		t.Fatalf("redis order: %+v", got)
	}

	// The sync marker expires: the next read goes back to Postgres and repairs Redis.
	mr.FastForward(SyncEvery + 1)
	got = nil
	call(t, h.Leaderboard, &got)
	if got[0].Username != "ann" || got[0].Trophies != 30 {
		t.Fatalf("after expiry: %+v", got)
	}
	if s, _ := mr.ZScore(LeaderboardKey, "p2"); s != 20 {
		t.Fatalf("redis not repaired, p2 = %v", s)
	}
}

func TestLeaderboardRedisDown(t *testing.T) {
	st := &fakeStore{players: []Player{{ID: uid, Username: "ann", Trophies: 30}}}
	h, mr := boardHandler(t, st)
	mr.Close()
	var got []row
	if rec := call(t, h.Leaderboard, &got); rec.Code != 200 || len(got) != 1 {
		t.Fatalf("redis down: %d %+v", rec.Code, got)
	}
}

func TestLeaderboardMemoryCache(t *testing.T) {
	players := []Player{{ID: uid, Username: "ann", Trophies: 30}, {ID: "p2", Username: "bob", Trophies: 20}}
	st := &fakeStore{players: players}
	h, mr := boardHandler(t, st)
	clock := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	h.now = func() time.Time { return clock }
	h.WithMemoryTTL(time.Minute)
	call(t, h.Leaderboard, nil) // loads Redis and memory

	before := mr.CommandCount()
	mr.ZAdd(LeaderboardKey, 35, "p2")
	var got []row
	call(t, h.Leaderboard, &got)
	if mr.CommandCount() != before || got[0].Username != "ann" || !got[0].IsMe {
		t.Fatalf("want the remembered board and no redis call: %+v (commands %d -> %d)", got, before, mr.CommandCount())
	}

	// A score change in this process drops it at once.
	h.Forget()
	got = nil
	call(t, h.Leaderboard, &got)
	if got[0].Username != "bob" || got[0].Trophies != 35 {
		t.Fatalf("after Forget: %+v", got)
	}

	// One from elsewhere shows up when the memory expires.
	mr.ZAdd(LeaderboardKey, 50, uid)
	clock = clock.Add(time.Minute)
	got = nil
	call(t, h.Leaderboard, &got)
	if got[0].Username != "ann" || got[0].Trophies != 50 {
		t.Fatalf("after expiry: %+v", got)
	}
}

func TestLeaderboardMemoryCacheSkipsErrors(t *testing.T) {
	st := &fakeStore{err: errors.New("pg down")}
	h := newHandler(st, fakeBalance{}).WithMemoryTTL(time.Minute)
	if rec := call(t, h.Leaderboard, nil); rec.Code != 500 {
		t.Fatalf("%d", rec.Code)
	}
	st.err, st.players = nil, []Player{{ID: uid, Username: "ann", Trophies: 3}}
	var got []row
	if rec := call(t, h.Leaderboard, &got); rec.Code != 200 || len(got) != 1 {
		t.Fatalf("an error must not be remembered: %d %+v", rec.Code, got)
	}
}
