// Package dashboard serves the player's home screen: the crime stories, the
// user's stats, the trophy leaderboard and the active-case lock.
package dashboard

import (
	"context"
	"errors"

	"backend/internal/db/dbgen"
	"backend/internal/user"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Story is the metadata a dashboard card needs.
type Story struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	Synopsis        string `json:"synopsis"`
	ThumbnailURL    string `json:"thumbnail_url"`
	DifficultyLevel string `json:"difficulty_level"`
	RewardTrophies  int    `json:"reward_trophies"`
	// Outcome is how the caller's attempt ended: "solved", "failed", or null when the case is still open to them.
	Outcome *string `json:"outcome"`
}

// Player is one ranked user.
type Player struct {
	ID       string
	Username string
	Trophies int64
}

// UserStats are the per-user counters shown on the dashboard.
type UserStats struct {
	Trophies    int64
	CasesSolved int64
}

// ActiveCase is the case a user is currently locked into.
type ActiveCase struct {
	CaseID  string // this play-through
	StoryID string // the story being played
	Title   string // the story's title
	Music   string // the story's music track key; "" for the main theme
}

// Store is the PostgreSQL side of the dashboard; *PGStore implements it.
type Store interface {
	ListStories(ctx context.Context, userID string) ([]Story, error)
	// Stats returns user.ErrNotFound for an unknown user.
	Stats(ctx context.Context, userID string) (UserStats, error)
	// ActiveCase returns the user's locked case, or nil when they are free.
	ActiveCase(ctx context.Context, userID string) (*ActiveCase, error)
	// TopPlayers is the leaderboard: the players with the most trophies, best first.
	TopPlayers(ctx context.Context, limit int) ([]Player, error)
	// Usernames maps user ids to display names; unknown ids are left out.
	Usernames(ctx context.Context, ids []string) (map[string]string, error)
}

// PGStore implements Store. All SQL lives in sql/queries/dashboard.sql and is
// compiled to type-safe Go by sqlc.
type PGStore struct{ q *dbgen.Queries }

func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{q: dbgen.New(pool)} }

func (s *PGStore) ListStories(ctx context.Context, userID string) ([]Story, error) {
	rows, err := s.q.ListStories(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]Story, 0, len(rows))
	for _, r := range rows {
		st := Story{
			ID: r.ID, Title: r.Title, Synopsis: r.Synopsis, ThumbnailURL: r.ThumbnailUrl,
			DifficultyLevel: r.DifficultyLevel, RewardTrophies: int(r.RewardTrophies),
		}
		if r.Outcome != "" {
			st.Outcome = &r.Outcome
		}
		out = append(out, st)
	}
	return out, nil
}

func (s *PGStore) Stats(ctx context.Context, userID string) (UserStats, error) {
	row, err := s.q.GetUserStats(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return UserStats{}, user.ErrNotFound
	}
	return UserStats{Trophies: int64(row.Trophies), CasesSolved: int64(row.CasesSolved)}, err
}

func (s *PGStore) ActiveCase(ctx context.Context, userID string) (*ActiveCase, error) {
	row, err := s.q.GetActiveCase(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ActiveCase{CaseID: row.CaseID, StoryID: row.StoryID, Title: row.Title, Music: row.MusicTrack}, nil
}

func (s *PGStore) TopPlayers(ctx context.Context, limit int) ([]Player, error) {
	rows, err := s.q.TopPlayers(ctx, int32(limit))
	if err != nil {
		return nil, err
	}
	out := make([]Player, 0, len(rows))
	for _, r := range rows {
		out = append(out, Player{ID: r.ID, Username: r.Username, Trophies: int64(r.Trophies)})
	}
	return out, nil
}

func (s *PGStore) Usernames(ctx context.Context, ids []string) (map[string]string, error) {
	rows, err := s.q.UsernamesByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		out[r.ID] = r.Username
	}
	return out, nil
}
