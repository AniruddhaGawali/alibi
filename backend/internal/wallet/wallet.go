// Package wallet manages a user's question-token balance. PostgreSQL
// (users.token_balance) is the source of truth; Redis is a read cache in front
// of it. Every change goes to Postgres first and then invalidates the cache.
package wallet

import (
	"context"
	"errors"
	"time"

	"backend/internal/user"
	"backend/pkg/utils"
	"github.com/redis/go-redis/v9"
)

var (
	// ErrNotFound from the store (unknown user) is passed through as user.ErrNotFound.
	ErrInsufficient = user.ErrInsufficientTokens
	ErrInvalid      = errors.New("wallet: amount must be between 1 and MaxAmount")
)

// MaxTokens returns the most tokens a player can hold right now (the admin setting,
// settings.Service.MaxTokens); grants and refunds stop there.
type MaxTokens func(ctx context.Context) int64

const (
	MaxAmount = 1_000_000
	// cacheTTL bounds how long a stale cached value can survive if an
	// invalidation is ever lost (e.g. Redis was unreachable at that moment).
	cacheTTL = time.Hour
)

// Key is the Redis key caching a user's balance.
func Key(userID string) string { return "user:coins:" + userID }

// Store is the Postgres side; *user.PGStore implements it.
type Store interface {
	GetTokenBalance(ctx context.Context, userID string) (int64, error)
	// AdjustTokenBalance adds delta (may be negative) and returns the new
	// balance. It returns ErrInsufficient if the result would go below zero. A
	// positive delta stops at maxTokens.
	AdjustTokenBalance(ctx context.Context, userID string, delta, maxTokens int64) (int64, error)
	// ResetAllTokenBalances sets every balance to tokens; returns rows changed.
	ResetAllTokenBalances(ctx context.Context, tokens int64) (int64, error)
}

type Service struct {
	store     Store
	rdb       *redis.Client
	maxTokens MaxTokens
}

// New builds the wallet. maxTokens reads the current cap; nil means user.DefaultMaxTokens.
func New(store Store, rdb *redis.Client, maxTokens MaxTokens) *Service {
	if maxTokens == nil {
		maxTokens = func(context.Context) int64 { return user.DefaultMaxTokens }
	}
	return &Service{store: store, rdb: rdb, maxTokens: maxTokens}
}

// MaxTokens is the cap grants and refunds stop at.
func (s *Service) MaxTokens(ctx context.Context) int64 { return s.maxTokens(ctx) }

// Balance returns the balance from Redis, falling back to Postgres (and
// refilling the cache) on a miss or when Redis is unavailable.
func (s *Service) Balance(ctx context.Context, userID string) (int64, error) {
	n, err := s.rdb.Get(ctx, Key(userID)).Int64()
	if err == nil {
		return n, nil
	}
	if !errors.Is(err, redis.Nil) {
		utils.LogErrorCtx(ctx, "token cache read failed; using postgres", "user_id", userID, "err", err)
	}

	n, err = s.store.GetTokenBalance(ctx, userID)
	if err != nil {
		return 0, err
	}
	if err := s.rdb.Set(ctx, Key(userID), n, cacheTTL).Err(); err != nil {
		utils.LogErrorCtx(ctx, "token cache fill failed", "user_id", userID, "err", err)
	}
	return n, nil
}

// Cache stores balance as the cached value, e.g. right after an account is
// created with its starting tokens. Failure is only logged: Postgres holds the
// truth and Balance refills the cache on the next read.
func (s *Service) Cache(ctx context.Context, userID string, balance int64) {
	if err := s.rdb.Set(ctx, Key(userID), balance, cacheTTL).Err(); err != nil {
		utils.LogErrorCtx(ctx, "token cache fill failed", "user_id", userID, "err", err)
	}
}

// Grant adds amount tokens and returns the new balance, which never exceeds MaxTokens.
func (s *Service) Grant(ctx context.Context, userID string, amount int64) (int64, error) {
	if amount < 1 || amount > MaxAmount {
		return 0, ErrInvalid
	}
	return s.adjust(ctx, userID, amount)
}

// Spend removes amount tokens, or returns ErrInsufficient and changes nothing.
func (s *Service) Spend(ctx context.Context, userID string, amount int64) (int64, error) {
	if amount < 1 || amount > MaxAmount {
		return 0, ErrInvalid
	}
	return s.adjust(ctx, userID, -amount)
}

func (s *Service) adjust(ctx context.Context, userID string, delta int64) (int64, error) {
	n, err := s.store.AdjustTokenBalance(ctx, userID, delta, s.maxTokens(ctx))
	if err != nil {
		return 0, err
	}
	// Postgres has committed. Drop the cached value so the next read reloads
	// it; if even that fails, cacheTTL limits how long the old value lives.
	if err := s.rdb.Del(ctx, Key(userID)).Err(); err != nil {
		utils.LogErrorCtx(ctx, "token cache invalidate failed", "user_id", userID, "err", err)
	}
	utils.LogInfoCtx(ctx, "token balance changed", "user_id", userID, "delta", delta, "balance", n)
	return n, nil
}
