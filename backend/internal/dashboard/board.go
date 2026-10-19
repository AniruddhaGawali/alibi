package dashboard

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// LeaderboardKey is the Redis sorted set of scores: member = user id, score =
	// trophies. The accusation flow moves it with ZINCRBY.
	LeaderboardKey = "leaderboard"
	// syncedKey says the set was rebuilt from Postgres recently. While it is there
	// the set is trusted; when it expires the next dashboard read rebuilds it, which
	// also repairs any drift (a missed ZINCRBY, a flushed Redis).
	syncedKey = "leaderboard:synced"
	// SyncEvery is how long a rebuild is trusted.
	SyncEvery = 5 * time.Minute
	// RebuildSize is how many top players a rebuild keeps. Only the top few are
	// shown, and anyone below this has a score the live increments cannot lift into view.
	RebuildSize = 1000
)

// Entry is one leaderboard row as Redis holds it.
type Entry struct {
	ID       string
	Trophies int64
}

// Board is the Redis leaderboard; *RedisBoard implements it.
type Board interface {
	// Top returns the best n entries, and false when the set is not in sync with
	// Postgres (never built, expired, or Redis is unreachable).
	Top(ctx context.Context, n int) ([]Entry, bool, error)
	// Rebuild replaces the whole set with players and marks it in sync.
	Rebuild(ctx context.Context, players []Player) error
}

type RedisBoard struct{ rdb *redis.Client }

func NewRedisBoard(rdb *redis.Client) *RedisBoard { return &RedisBoard{rdb: rdb} }

func (b *RedisBoard) Top(ctx context.Context, n int) ([]Entry, bool, error) {
	if err := b.rdb.Get(ctx, syncedKey).Err(); err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, false, nil
		}
		return nil, false, err
	}
	zs, err := b.rdb.ZRevRangeWithScores(ctx, LeaderboardKey, 0, int64(n-1)).Result()
	if err != nil {
		return nil, false, err
	}
	out := make([]Entry, 0, len(zs))
	for _, z := range zs {
		id, ok := z.Member.(string)
		if !ok || z.Score <= 0 {
			continue
		}
		out = append(out, Entry{ID: id, Trophies: int64(z.Score)})
	}
	return out, true, nil
}

func (b *RedisBoard) Rebuild(ctx context.Context, players []Player) error {
	_, err := b.rdb.TxPipelined(ctx, func(p redis.Pipeliner) error {
		p.Del(ctx, LeaderboardKey)
		if len(players) > 0 {
			members := make([]redis.Z, 0, len(players))
			for _, pl := range players {
				members = append(members, redis.Z{Score: float64(pl.Trophies), Member: pl.ID})
			}
			p.ZAdd(ctx, LeaderboardKey, members...)
		}
		p.Set(ctx, syncedKey, strconv.FormatInt(time.Now().Unix(), 10), SyncEvery)
		return nil
	})
	return err
}
