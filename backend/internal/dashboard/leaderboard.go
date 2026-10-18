package dashboard

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// LeaderboardKey is the Redis sorted set: member = user ID, score = trophies.
const LeaderboardKey = "leaderboard:trophies"

// Entry is one leaderboard row as stored in Redis.
type Entry struct {
	UserID   string
	Trophies int64
}

// Board reads the ranking; *Leaderboard implements it.
type Board interface {
	Top(ctx context.Context, n int) ([]Entry, error)
}

// Leaderboard is the Redis sorted set of trophy totals. Postgres
// (users.trophies) stays the source of truth; whatever changes a user's
// trophies calls Set afterwards, and the handler falls back to Postgres when
// the set is empty or Redis is down.
type Leaderboard struct{ rdb *redis.Client }

func NewLeaderboard(rdb *redis.Client) *Leaderboard { return &Leaderboard{rdb: rdb} }

// Top returns the n highest scores, best first.
func (l *Leaderboard) Top(ctx context.Context, n int) ([]Entry, error) {
	zs, err := l.rdb.ZRevRangeWithScores(ctx, LeaderboardKey, 0, int64(n)-1).Result()
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(zs))
	for _, z := range zs {
		id, ok := z.Member.(string)
		if !ok {
			continue
		}
		out = append(out, Entry{UserID: id, Trophies: int64(z.Score)})
	}
	return out, nil
}

// Set records a user's trophy total.
func (l *Leaderboard) Set(ctx context.Context, userID string, trophies int64) error {
	return l.rdb.ZAdd(ctx, LeaderboardKey, redis.Z{Score: float64(trophies), Member: userID}).Err()
}
