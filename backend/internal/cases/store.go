// Package cases serves the case briefing (Phase 3.1): the dossier with its
// scene images, the suspects with their initial testimonies, and starting a
// case, which locks the player into it. "Case" here is a row of the stories table.
package cases

import (
	"context"
	"errors"

	"backend/internal/db/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrNotFound means the case does not exist or is not published.
	ErrNotFound = errors.New("cases: not found")
	// ErrLockedElsewhere means the user is already inside a different unfinished case.
	ErrLockedElsewhere = errors.New("cases: user has another active case")
	// ErrUserGone means the user row no longer exists.
	ErrUserGone = errors.New("cases: user not found")
)

// Scene is one crime-scene image.
type Scene struct {
	ID           string `json:"id"`
	SceneName    string `json:"scene_name"`
	ImageURL     string `json:"image_url"`
	DisplayOrder int    `json:"display_order"`
}

// Briefing is the case dossier. It never carries solution data.
type Briefing struct {
	ID              string  `json:"id"`
	Title           string  `json:"title"`
	Synopsis        string  `json:"synopsis"`
	MainPlot        string  `json:"main_plot"`
	DifficultyLevel string  `json:"difficulty_level"`
	RewardTrophies  int     `json:"reward_trophies"`
	RewardTokens    int     `json:"reward_tokens"`
	Scenes          []Scene `json:"scenes"`
}

// Suspect is what the client may see of a suspect. There is deliberately no
// culprit flag here: the type cannot leak what it does not hold.
type Suspect struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Role             string `json:"role"`
	AvatarURL        string `json:"avatar_url"`
	InitialTestimony string `json:"initial_testimony"`
}

// Lock is a user's active case lock.
type Lock struct {
	CaseID  string `json:"case_id"`  // this play-through
	StoryID string `json:"story_id"` // the case being played
}

// Store is the PostgreSQL side of the briefing; *PGStore implements it.
type Store interface {
	// Briefing returns ErrNotFound for an unknown or unpublished case.
	Briefing(ctx context.Context, id string) (*Briefing, error)
	// Suspects returns ErrNotFound for an unknown or unpublished case.
	Suspects(ctx context.Context, id string) ([]Suspect, error)
	// Lock returns the user's active lock, or nil when they are free.
	Lock(ctx context.Context, userID string) (*Lock, error)
	// Start locks the user into the case. Starting the case they are already in
	// returns the existing lock; another case returns ErrLockedElsewhere.
	Start(ctx context.Context, userID, storyID string) (*Lock, error)
}

// PGStore implements Store. All SQL is in sql/queries/cases.sql (sqlc).
type PGStore struct{ q *dbgen.Queries }

func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{q: dbgen.New(pool)} }

func (s *PGStore) Briefing(ctx context.Context, id string) (*Briefing, error) {
	row, err := s.q.GetCaseBriefing(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	scenes, err := s.q.ListCaseScenes(ctx, id)
	if err != nil {
		return nil, err
	}
	b := &Briefing{
		ID: row.ID, Title: row.Title, Synopsis: row.Synopsis, MainPlot: row.MainPlot,
		DifficultyLevel: row.DifficultyLevel, RewardTrophies: int(row.RewardTrophies), RewardTokens: int(row.RewardTokens),
		Scenes: make([]Scene, 0, len(scenes)),
	}
	for _, sc := range scenes {
		b.Scenes = append(b.Scenes, Scene{ID: sc.ID, SceneName: sc.SceneName, ImageURL: sc.ImageUrl, DisplayOrder: int(sc.DisplayOrder)})
	}
	return b, nil
}

func (s *PGStore) Suspects(ctx context.Context, id string) ([]Suspect, error) {
	ok, err := s.q.CaseIsPublished(ctx, id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	rows, err := s.q.ListCaseSuspects(ctx, id)
	if err != nil {
		return nil, err
	}
	out := make([]Suspect, 0, len(rows))
	for _, r := range rows {
		out = append(out, Suspect{ID: r.ID, Name: r.Name, Role: r.Role, AvatarURL: r.AvatarUrl, InitialTestimony: r.InitialTestimony})
	}
	return out, nil
}

func (s *PGStore) Lock(ctx context.Context, userID string) (*Lock, error) {
	row, err := s.q.GetUserLock(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &Lock{CaseID: row.CaseID, StoryID: row.StoryID}, nil
}

func (s *PGStore) Start(ctx context.Context, userID, storyID string) (*Lock, error) {
	ok, err := s.q.CaseIsPublished(ctx, storyID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	caseID := uuid.NewString()
	n, err := s.q.LockUserToCase(ctx, dbgen.LockUserToCaseParams{UserID: userID, CaseID: caseID, StoryID: storyID})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.ForeignKeyViolation {
		if pgErr.ConstraintName == "active_case_locks_story_id_fkey" {
			return nil, ErrNotFound
		}
		return nil, ErrUserGone
	}
	if err != nil {
		return nil, err
	}
	if n == 1 {
		return &Lock{CaseID: caseID, StoryID: storyID}, nil
	}
	// Already locked (maybe a double click on Start): same case is fine.
	cur, err := s.Lock(ctx, userID)
	if err != nil {
		return nil, err
	}
	if cur != nil && cur.StoryID == storyID {
		return cur, nil
	}
	return nil, ErrLockedElsewhere
}
