package wallet

import (
	"context"
	"errors"
	"fmt"

	"backend/internal/user"
	"backend/pkg/utils"
	"github.com/redis/go-redis/v9"
)

// decrScript is the pre-LLM deduction (P4.2): DECR the cached balance and, if
// that went below zero, INCR it straight back. Both steps run as one atomic
// script, so no other request ever sees a negative balance. It returns -2 when
// the key is not cached (the caller loads it from Postgres and retries), -1 when
// the user is out of tokens, and the new balance otherwise.
var decrScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 0 then return -2 end
local n = redis.call('DECR', KEYS[1])
if n < 0 then
  redis.call('INCR', KEYS[1])
  return -1
end
return n
`)

// incrScript refunds one token in the cache, only if the balance is cached and
// below the cap (ARGV[1]); a missing key is reloaded from Postgres on the next
// read anyway.
var incrScript = redis.NewScript(`
local n = redis.call('GET', KEYS[1])
if not n then return -2 end
if tonumber(n) >= tonumber(ARGV[1]) then return tonumber(n) end
return redis.call('INCR', KEYS[1])
`)

// SpendQuestion takes one question token before the model is called. The Redis
// counter is decremented first (fast, and the gate against overdrawing), then
// Postgres, the source of truth, is updated to match. If Postgres refuses or
// fails, the cached key is dropped so the next read reloads it from Postgres. If
// Redis is down, Postgres alone decides. Returns ErrInsufficient when out of tokens.
func (s *Service) SpendQuestion(ctx context.Context, userID string) (int64, error) {
	n, err := s.decr(ctx, userID)
	if err != nil {
		utils.LogErrorCtx(ctx, "token cache spend failed; using postgres", "user_id", userID, "err", err)
		return s.adjust(ctx, userID, -1)
	}
	if n == -1 {
		return 0, ErrInsufficient
	}
	pg, err := s.store.AdjustTokenBalance(ctx, userID, -1, s.maxTokens(ctx))
	if err != nil {
		// Redis took a token Postgres did not (or may not have): resync from Postgres.
		s.drop(ctx, userID)
		return 0, err
	}
	utils.LogInfoCtx(ctx, "token balance changed", "user_id", userID, "delta", -1, "balance", pg)
	return pg, nil
}

// decr runs decrScript, loading the balance from Postgres once if it is not cached.
func (s *Service) decr(ctx context.Context, userID string) (int64, error) {
	key := Key(userID)
	n, err := decrScript.Run(ctx, s.rdb, []string{key}).Int64()
	if err != nil || n != -2 {
		return n, err
	}
	bal, err := s.store.GetTokenBalance(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("load balance: %w", err)
	}
	// NX: if a concurrent request filled the key first, keep its value.
	if err := s.rdb.SetNX(ctx, key, bal, cacheTTL).Err(); err != nil {
		return 0, err
	}
	n, err = decrScript.Run(ctx, s.rdb, []string{key}).Int64()
	if err == nil && n == -2 {
		return 0, errors.New("balance key vanished")
	}
	return n, err
}

// RefundQuestion gives back a token taken by SpendQuestion (moderation block,
// model timeout or failure): Postgres first, then INCR on the cached counter.
func (s *Service) RefundQuestion(ctx context.Context, userID string) (int64, error) {
	maxTokens := s.maxTokens(ctx)
	pg, err := s.store.AdjustTokenBalance(ctx, userID, 1, maxTokens)
	if err != nil {
		return 0, err
	}
	if err := incrScript.Run(ctx, s.rdb, []string{Key(userID)}, maxTokens).Err(); err != nil {
		utils.LogErrorCtx(ctx, "token cache refund failed", "user_id", userID, "err", err)
		s.drop(ctx, userID)
	}
	utils.LogInfoCtx(ctx, "token refunded", "user_id", userID, "balance", pg)
	return pg, nil
}

// drop removes the cached balance so the next read reloads it from Postgres.
func (s *Service) drop(ctx context.Context, userID string) {
	if err := s.rdb.Del(ctx, Key(userID)).Err(); err != nil {
		utils.LogErrorCtx(ctx, "token cache invalidate failed", "user_id", userID, "err", err)
	}
}

// ResetAll is the daily reset: every balance becomes exactly tokens in Postgres,
// then every cached balance is deleted so Redis reloads the new value. Deleting
// rather than writing the amount into each key means a spend racing with the
// reset cannot leave Redis and Postgres disagreeing for long.
func (s *Service) ResetAll(ctx context.Context, tokens int64) (int64, error) {
	if tokens < 0 || tokens > user.TokenCeiling {
		return 0, ErrInvalid
	}
	changed, err := s.store.ResetAllTokenBalances(ctx, tokens)
	if err != nil {
		return 0, err
	}
	cleared, err := s.ClearCache(ctx)
	if err != nil {
		return changed, err
	}
	utils.LogInfoCtx(ctx, "daily token reset", "tokens", tokens, "users_changed", changed, "cache_keys_cleared", cleared)
	return changed, nil
}

// ClearCache removes every cached balance (user:coins:*) and returns how many.
func (s *Service) ClearCache(ctx context.Context) (int64, error) {
	var cleared int64
	batch := make([]string, 0, 500)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		n, err := s.rdb.Unlink(ctx, batch...).Result() // UNLINK frees the memory in the background
		cleared += n
		batch = batch[:0]
		return err
	}
	iter := s.rdb.Scan(ctx, 0, Key("*"), 500).Iterator()
	for iter.Next(ctx) {
		if batch = append(batch, iter.Val()); len(batch) == cap(batch) {
			if err := flush(); err != nil {
				return cleared, fmt.Errorf("clear token cache: %w", err)
			}
		}
	}
	if err := iter.Err(); err != nil {
		return cleared, fmt.Errorf("scan token cache: %w", err)
	}
	if err := flush(); err != nil {
		return cleared, fmt.Errorf("clear token cache: %w", err)
	}
	return cleared, nil
}
