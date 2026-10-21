package wallet

import (
	"context"
	"errors"
	"sync"
	"testing"

	"backend/internal/user"
)

func TestSpendQuestionDecrementsRedisAndPostgres(t *testing.T) {
	s, st, mr := newService(t)
	ctx := context.Background()
	st.bal["u1"] = 2

	// Not cached yet: loaded from Postgres, then DECR.
	if n, err := s.SpendQuestion(ctx, "u1"); err != nil || n != 1 {
		t.Fatalf("first spend: %d %v", n, err)
	}
	if got, _ := mr.Get(Key("u1")); got != "1" || st.bal["u1"] != 1 {
		t.Fatalf("redis=%q postgres=%d", got, st.bal["u1"])
	}
	// The key stays cached and is decremented in place.
	if n, err := s.SpendQuestion(ctx, "u1"); err != nil || n != 0 {
		t.Fatalf("second spend: %d %v", n, err)
	}
	if got, _ := mr.Get(Key("u1")); got != "0" || st.bal["u1"] != 0 {
		t.Fatalf("redis=%q postgres=%d", got, st.bal["u1"])
	}
	// Out of tokens: the DECR is undone at once and Postgres is not touched.
	if _, err := s.SpendQuestion(ctx, "u1"); !errors.Is(err, ErrInsufficient) {
		t.Fatalf("overdraw: %v", err)
	}
	if got, _ := mr.Get(Key("u1")); got != "0" || st.bal["u1"] != 0 {
		t.Fatalf("after overdraw redis=%q postgres=%d", got, st.bal["u1"])
	}
}

func TestSpendQuestionNeverOverdrawsUnderConcurrency(t *testing.T) {
	s, st, mr := newService(t)
	ctx := context.Background()
	st.bal["u1"] = 5
	var mu sync.Mutex // the fake store is a plain map
	locked := &lockedStore{fakeStore: st, mu: &mu}
	s.store = locked

	var wg sync.WaitGroup
	var okCount, outCount int
	var cmu sync.Mutex
	for range 20 {
		wg.Go(func() {
			_, err := s.SpendQuestion(ctx, "u1")
			cmu.Lock()
			defer cmu.Unlock()
			switch {
			case err == nil:
				okCount++
			case errors.Is(err, ErrInsufficient):
				outCount++
			default:
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if okCount != 5 || outCount != 15 || st.bal["u1"] != 0 {
		t.Fatalf("ok=%d out=%d postgres=%d", okCount, outCount, st.bal["u1"])
	}
	if got, _ := mr.Get(Key("u1")); got != "0" {
		t.Fatalf("redis = %q", got)
	}
}

type lockedStore struct {
	*fakeStore
	mu *sync.Mutex
}

func (l *lockedStore) GetTokenBalance(ctx context.Context, id string) (int64, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.fakeStore.GetTokenBalance(ctx, id)
}
func (l *lockedStore) AdjustTokenBalance(ctx context.Context, id string, d, maxTokens int64) (int64, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.fakeStore.AdjustTokenBalance(ctx, id, d, maxTokens)
}

func TestSpendQuestionResyncsWhenPostgresDisagrees(t *testing.T) {
	s, st, mr := newService(t)
	ctx := context.Background()
	st.bal["u1"] = 0
	_ = mr.Set(Key("u1"), "3") // stale cache claims tokens Postgres does not have
	if _, err := s.SpendQuestion(ctx, "u1"); !errors.Is(err, ErrInsufficient) {
		t.Fatalf("err = %v", err)
	}
	if mr.Exists(Key("u1")) {
		t.Fatal("stale key should be dropped")
	}
	if _, err := s.SpendQuestion(ctx, "nobody"); !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("unknown user: %v", err)
	}
}

func TestSpendQuestionFallsBackWhenRedisIsDown(t *testing.T) {
	s, st, mr := newService(t)
	st.bal["u1"] = 1
	mr.Close()
	if n, err := s.SpendQuestion(context.Background(), "u1"); err != nil || n != 0 || st.bal["u1"] != 0 {
		t.Fatalf("n=%d err=%v postgres=%d", n, err, st.bal["u1"])
	}
}

func TestRefundQuestion(t *testing.T) {
	s, st, mr := newService(t)
	ctx := context.Background()
	st.bal["u1"] = 1
	if _, err := s.SpendQuestion(ctx, "u1"); err != nil {
		t.Fatal(err)
	}
	if n, err := s.RefundQuestion(ctx, "u1"); err != nil || n != 1 {
		t.Fatalf("refund: %d %v", n, err)
	}
	if got, _ := mr.Get(Key("u1")); got != "1" || st.bal["u1"] != 1 {
		t.Fatalf("redis=%q postgres=%d", got, st.bal["u1"])
	}
	// Not cached: only Postgres changes, and no key is invented.
	mr.Del(Key("u1"))
	if n, err := s.RefundQuestion(ctx, "u1"); err != nil || n != 2 || mr.Exists(Key("u1")) {
		t.Fatalf("uncached refund: %d %v", n, err)
	}
}

func TestResetAll(t *testing.T) {
	s, st, mr := newService(t)
	ctx := context.Background()
	st.bal = map[string]int64{"a": 0, "b": 3, "c": 10, "d": 25}
	// d cannot really exist (the database caps at 10), but a reset must still land on 10
	for id, b := range map[string]string{"a": "0", "b": "3", "d": "25"} {
		_ = mr.Set(Key(id), b)
	}
	_ = mr.Set("leaderboard:trophies", "keep")

	changed, err := s.ResetAll(ctx, 10)
	if err != nil || changed != 3 {
		t.Fatalf("changed=%d err=%v", changed, err)
	}
	for id, b := range st.bal {
		if b != 10 {
			t.Errorf("%s = %d: unused or bonus tokens must not roll over", id, b)
		}
		if mr.Exists(Key(id)) {
			t.Errorf("%s still cached", id)
		}
	}
	if !mr.Exists("leaderboard:trophies") {
		t.Fatal("reset must only clear token keys")
	}
	if n, _ := s.Balance(ctx, "b"); n != 10 {
		t.Fatalf("balance after reset = %d", n)
	}
	if _, err := s.ResetAll(ctx, -1); !errors.Is(err, ErrInvalid) {
		t.Fatalf("negative: %v", err)
	}
}

func TestTokensNeverExceedTheCap(t *testing.T) {
	s, st, mr := newService(t)
	ctx := context.Background()
	st.bal["u1"] = 8

	if n, err := s.Grant(ctx, "u1", 5); err != nil || n != user.DefaultMaxTokens || st.bal["u1"] != user.DefaultMaxTokens {
		t.Fatalf("grant past the cap: %d %v", n, err)
	}
	// A refund when the Redis copy is already full does not go above 10 either.
	if _, err := s.SpendQuestion(ctx, "u1"); err != nil { // 10 -> 9, cached
		t.Fatal(err)
	}
	_ = mr.Set(Key("u1"), "10") // as if a reset landed between the spend and the refund
	st.bal["u1"] = 10
	if _, err := s.RefundQuestion(ctx, "u1"); err != nil {
		t.Fatal(err)
	}
	if got, _ := mr.Get(Key("u1")); got != "10" || st.bal["u1"] != 10 {
		t.Fatalf("refund at the cap: redis=%q postgres=%d", got, st.bal["u1"])
	}
	if _, err := s.ResetAll(ctx, user.TokenCeiling+1); !errors.Is(err, ErrInvalid) {
		t.Fatalf("reset above the cap: %v", err)
	}
}

func TestConfigurableCap(t *testing.T) {
	s, st, _ := newService(t)
	st.bal["u1"] = 15
	limit := int64(20)
	s.maxTokens = func(context.Context) int64 { return limit }
	ctx := context.Background()
	// A raised cap lets grants go past the old 10.
	if n, err := s.Grant(ctx, "u1", 10); err != nil || n != 20 {
		t.Fatalf("grant to the raised cap: %d %v", n, err)
	}
	if s.MaxTokens(ctx) != 20 {
		t.Fatalf("MaxTokens = %d", s.MaxTokens(ctx))
	}
	// Lowering the cap never takes tokens away: a grant above it keeps the balance, a spend just spends.
	limit = 10
	if n, err := s.Grant(ctx, "u1", 1); err != nil || n != 20 {
		t.Fatalf("grant above a lowered cap: %d %v", n, err)
	}
	if n, err := s.Spend(ctx, "u1", 1); err != nil || n != 19 {
		t.Fatalf("spend above a lowered cap: %d %v", n, err)
	}
}
