// Package admin holds the admin-only case management API. Every route is wrapped
// in Handler.RequireAdmin, which checks users.is_admin on each request.
package admin

import (
	"context"
	"errors"
	"time"

	"backend/internal/db/dbgen"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrNotFound means the case or user does not exist.
	ErrNotFound = errors.New("admin: not found")
	// ErrInvalid means the database rejected a value (a CHECK constraint).
	ErrInvalid = errors.New("admin: invalid value")
)

// Case is a case as admins see it, including its model.
type Case struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	Synopsis        string    `json:"synopsis"`
	MainPlot        string    `json:"main_plot"`
	ThumbnailURL    string    `json:"thumbnail_url"`
	DifficultyLevel string    `json:"difficulty_level"`
	RewardTrophies  int       `json:"reward_trophies"`
	IsPublished     bool      `json:"is_published"`
	LLMModel        string    `json:"llm_model"`
	CreatedAt       time.Time `json:"created_at"`
}

// CaseInput is a new case; every field is set.
type CaseInput struct {
	Title, Synopsis, MainPlot, ThumbnailURL, DifficultyLevel string
	RewardTrophies                                           int32
	IsPublished                                              bool
	LLMModel                                                 string
}

// CasePatch changes only the non-nil fields.
type CasePatch struct {
	Title, Synopsis, MainPlot, ThumbnailURL, DifficultyLevel *string
	RewardTrophies                                           *int32
	IsPublished                                              *bool
	LLMModel                                                 *string
}

// Store is the PostgreSQL side; *PGStore implements it.
type Store interface {
	IsAdmin(ctx context.Context, userID string) (bool, error)
	CreateCase(ctx context.Context, in CaseInput) (*Case, error)
	// UpdateCase returns ErrNotFound for an unknown case.
	UpdateCase(ctx context.Context, id string, p CasePatch) (*Case, error)
}

// PGStore implements Store. All SQL is in sql/queries/admin.sql (sqlc).
type PGStore struct{ q *dbgen.Queries }

func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{q: dbgen.New(pool)} }

func (s *PGStore) IsAdmin(ctx context.Context, userID string) (bool, error) {
	ok, err := s.q.UserIsAdmin(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return ok, err
}

func (s *PGStore) CreateCase(ctx context.Context, in CaseInput) (*Case, error) {
	r, err := s.q.AdminCreateCase(ctx, dbgen.AdminCreateCaseParams{
		Title: in.Title, Synopsis: in.Synopsis, MainPlot: in.MainPlot, ThumbnailUrl: in.ThumbnailURL,
		DifficultyLevel: in.DifficultyLevel, RewardTrophies: in.RewardTrophies,
		IsPublished: in.IsPublished, LlmModel: in.LLMModel,
	})
	if err != nil {
		return nil, mapErr(err)
	}
	return &Case{
		ID: r.ID, Title: r.Title, Synopsis: r.Synopsis, MainPlot: r.MainPlot, ThumbnailURL: r.ThumbnailUrl,
		DifficultyLevel: r.DifficultyLevel, RewardTrophies: int(r.RewardTrophies),
		IsPublished: r.IsPublished, LLMModel: r.LlmModel, CreatedAt: r.CreatedAt,
	}, nil
}

func (s *PGStore) UpdateCase(ctx context.Context, id string, p CasePatch) (*Case, error) {
	r, err := s.q.AdminUpdateCase(ctx, dbgen.AdminUpdateCaseParams{
		ID: id, Title: p.Title, Synopsis: p.Synopsis, MainPlot: p.MainPlot, ThumbnailUrl: p.ThumbnailURL,
		DifficultyLevel: p.DifficultyLevel, RewardTrophies: p.RewardTrophies,
		IsPublished: p.IsPublished, LlmModel: p.LLMModel,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, mapErr(err)
	}
	return &Case{
		ID: r.ID, Title: r.Title, Synopsis: r.Synopsis, MainPlot: r.MainPlot, ThumbnailURL: r.ThumbnailUrl,
		DifficultyLevel: r.DifficultyLevel, RewardTrophies: int(r.RewardTrophies),
		IsPublished: r.IsPublished, LLMModel: r.LlmModel, CreatedAt: r.CreatedAt,
	}, nil
}

func mapErr(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == pgerrcode.CheckViolation {
		return ErrInvalid
	}
	return err
}
