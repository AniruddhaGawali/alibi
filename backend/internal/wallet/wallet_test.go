package wallet

import (
	"context"
	"errors"
	"testing"
	"time"

	"backend/internal/user"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// fakeStore mimics the Postgres rules: unknown users, and no negative balance.
type fakeStore struct {
	bal   map[string]int64
	reads int
}

func (f *fakeStore) GetTokenBalance(_ context.Context, id string) (int64, error) {
	f.reads++
	n, ok := f.bal[id]
	if !ok {
		return 0, user.ErrNotFound
	}
	return n, nil
}

func (f *fakeStore) AdjustTokenBalance(_ context.Context, id string, d, maxTokens int64) (int64, error) {
	n, ok := f.bal[id]
	if !ok {
		return 0, user.ErrNotFound
	}
	if n+d < 0 {
		return 0, user.ErrInsufficientTokens
	}
	if d > 0 {
		f.bal[id] = min(n+d, max(maxTokens, n)) // the real store caps grants, and never lowers a balance above the cap
	} else {
		f.bal[id] = n + d
	}
	return f.bal[id], nil
}

func (f *fakeStore) ResetAllTokenBalances(_ context.Context, tokens int64) (int64, error) {
	var n int64
	for id, b := range f.bal {
		if b != tokens {
			f.bal[id] = tokens
			n++
		}
	}
	return n, nil
}

func newService(t *testing.T) (*Service, *fakeStore, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	st := &fakeStore{bal: map[string]int64{"u1": 0}}
	return New(st, rdb, nil), st, mr
}

func TestBalanceReadsThroughToPostgresThenCache(t *testing.T) {
	s, st, mr := newService(t)
	ctx := context.Background()
	st.bal["u1"] = 7

	if n, err := s.Balance(ctx, "u1"); err != nil || n != 7 {
		t.Fatalf("first read: %d %v", n, err)
	}
	if got, _ := mr.Get(Key("u1")); got != "7" {
		t.Fatalf("cache not filled: %q", got)
	}
	if ttl := mr.TTL(Key("u1")); ttl <= 0 || ttl > time.Hour {
		t.Fatalf("cache ttl = %v", ttl)
	}
	if _, _ = s.Balance(ctx, "u1"); st.reads != 1 {
		t.Fatalf("second read hit postgres (%d reads)", st.reads)
	}
}

func TestGrantAndSpendUpdatePostgresAndInvalidateCache(t *testing.T) {
	s, st, mr := newService(t)
	ctx := context.Background()

	if n, err := s.Grant(ctx, "u1", 10); err != nil || n != 10 || st.bal["u1"] != 10 {
		t.Fatalf("grant: %d %v", n, err)
	}
	_, _ = s.Balance(ctx, "u1") // fills the cache with 10
	if n, err := s.Spend(ctx, "u1", 3); err != nil || n != 7 {
		t.Fatalf("spend: %d %v", n, err)
	}
	if mr.Exists(Key("u1")) {
		t.Fatal("stale cache entry should be removed after a change")
	}
	if n, _ := s.Balance(ctx, "u1"); n != 7 {
		t.Fatalf("balance after spend = %d", n)
	}
}

func TestSpendCannotOverdraw(t *testing.T) {
	s, st, _ := newService(t)
	st.bal["u1"] = 2
	if _, err := s.Spend(context.Background(), "u1", 3); !errors.Is(err, ErrInsufficient) {
		t.Fatalf("got %v", err)
	}
	if st.bal["u1"] != 2 {
		t.Fatalf("balance changed to %d", st.bal["u1"])
	}
}

func TestInvalidAmountsAndUnknownUser(t *testing.T) {
	s, _, _ := newService(t)
	ctx := context.Background()
	for _, n := range []int64{0, -5, MaxAmount + 1} {
		if _, err := s.Grant(ctx, "u1", n); !errors.Is(err, ErrInvalid) {
			t.Errorf("grant %d: %v", n, err)
		}
		if _, err := s.Spend(ctx, "u1", n); !errors.Is(err, ErrInvalid) {
			t.Errorf("spend %d: %v", n, err)
		}
	}
	if _, err := s.Balance(ctx, "nobody"); !errors.Is(err, user.ErrNotFound) {
		t.Errorf("unknown user: %v", err)
	}
}

func TestBalanceFallsBackToPostgresWhenRedisIsDown(t *testing.T) {
	s, st, mr := newService(t)
	st.bal["u1"] = 4
	mr.Close()
	if n, err := s.Balance(context.Background(), "u1"); err != nil || n != 4 {
		t.Fatalf("got %d %v", n, err)
	}
}
