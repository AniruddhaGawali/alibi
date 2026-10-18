// Package dashboard serves the player's home screen: the crime stories, the
// user's stats, the trophy leaderboard and the active-case lock.
package dashboard

import (
	"context"
	"errors"

	"backend/internal/db/dbgen"
	"backend/internal/user"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrAlreadyLocked means the user is already inside an unfinished case.
	ErrAlreadyLocked = errors.New("dashboard: user already has an active case")
	// ErrNotFound means the user or the story does not exist.
	ErrNotFound = errors.New("dashboard: not found")
)

// Story is the metadata a dashboard card needs.
type Story struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	Synopsis        string `json:"synopsis"`
	ThumbnailURL    string `json:"thumbnail_url"`
	DifficultyLevel string `json:"difficulty_level"`
	RewardTrophies  int    `json:"reward_trophies"`
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
	CaseID string
	Title  string // the story's title
}

// Store is the PostgreSQL side of the dashboard; *PGStore implements it.
type Store interface {
	ListStories(ctx context.Context) ([]Story, error)
	// Stats returns user.ErrNotFound for an unknown user.
	Stats(ctx context.Context, userID string) (UserStats, error)
	// ActiveCase returns the user's locked case, or nil when they are free.
	ActiveCase(ctx context.Context, userID string) (*ActiveCase, error)
	// TopPlayers is the leaderboard fallback when Redis cannot answer.
	TopPlayers(ctx context.Context, limit int) ([]Player, error)
	// Usernames maps user IDs to usernames; unknown IDs are simply absent.
	Usernames(ctx context.Context, ids []string) (map[string]string, error)
}

// PGStore implements Store. All SQL lives in sql/queries/dashboard.sql and is
// compiled to type-safe Go by sqlc.
type PGStore struct{ q *dbgen.Queries }

func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{q: dbgen.New(pool)} }

func (s *PGStore) ListStories(ctx context.Context) ([]Story, error) {
	rows, err := s.q.ListStories(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Story, 0, len(rows))
	for _, r := range rows {
		out = append(out, Story{
			ID: r.ID, Title: r.Title, Synopsis: r.Synopsis, ThumbnailURL: r.ThumbnailUrl,
			DifficultyLevel: r.DifficultyLevel, RewardTrophies: int(r.RewardTrophies),
		})
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
	return &ActiveCase{CaseID: row.CaseID, Title: row.Title}, nil
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
	rows, err := s.q.GetUsernames(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		out[r.ID] = r.Username
	}
	return out, nil
}

// LockCase locks the user into a case. It returns ErrAlreadyLocked if they
// already have one, and ErrNotFound if the user or story does not exist. The
// case-start endpoint (Phase 3) calls it.
func (s *PGStore) LockCase(ctx context.Context, userID, caseID, storyID string) error {
	n, err := s.q.LockUserToCase(ctx, dbgen.LockUserToCaseParams{UserID: userID, CaseID: caseID, StoryID: storyID})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.ForeignKeyViolation {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrAlreadyLocked
	}
	return nil
}

// ReleaseCase frees the user once the case is finished. It reports whether a
// lock for that case existed.
func (s *PGStore) ReleaseCase(ctx context.Context, userID, caseID string) (bool, error) {
	n, err := s.q.ReleaseCaseLock(ctx, dbgen.ReleaseCaseLockParams{UserID: userID, CaseID: caseID})
	return n > 0, err
}
