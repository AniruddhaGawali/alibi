// Package cases serves the case briefing (Phase 3.1): the dossier with its
// scene images, the suspects with their initial testimonies, and starting a
// case, which locks the player into it. "Case" here is a row of the stories table.
package cases

import (
	"context"
	"errors"
	"time"

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
	// ErrNotesFull means the user already has the maximum number of notes for the case.
	ErrNotesFull = errors.New("cases: too many notes")
	// ErrBadSuspect means the suspect does not belong to the case.
	ErrBadSuspect = errors.New("cases: suspect is not part of this case")
)

// MaxNotesPerCase caps how many notes one player can keep per case.
const MaxNotesPerCase = 200

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

	// Notes are the player's private notes for a case, newest first.
	Notes(ctx context.Context, userID, storyID string) ([]Note, error)
	// AddNote saves a note, optionally about one suspect of the case and/or tied to one
	// log entry (chatMessageID; the caller has already verified it). It returns
	// ErrNotFound for an unknown case, ErrBadSuspect for a suspect from another case
	// and ErrNotesFull at MaxNotesPerCase.
	AddNote(ctx context.Context, userID, storyID string, suspectID, chatMessageID *string, body string) (*Note, error)
	// DeleteNote removes the user's own note; ErrNotFound if it is not theirs or is gone.
	DeleteNote(ctx context.Context, userID, noteID string) error
}

// Note is one of the detective's own notes.
type Note struct {
	ID        string  `json:"id"`
	SuspectID *string `json:"suspect_id"` // null when the note is about the case in general
	// ChatMessageID is the interrogation log entry the note is tied to, if any.
	ChatMessageID *string   `json:"chat_message_id"`
	Body          string    `json:"body"`
	CreatedAt     time.Time `json:"created_at"`
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
		DifficultyLevel: row.DifficultyLevel, RewardTrophies: int(row.RewardTrophies),
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
	caseID := uuid.NewString()
	n, err := s.q.LockUserToCase(ctx, dbgen.LockUserToCaseParams{UserID: userID, CaseID: caseID, StoryID: storyID})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.ForeignKeyViolation {
		return nil, ErrUserGone
	}
	if err != nil {
		return nil, err
	}
	if n == 1 {
		return &Lock{CaseID: caseID, StoryID: storyID}, nil
	}
	// Nothing written: the case is not published, or the player is already in a case.
	ok, err := s.q.CaseIsPublished(ctx, storyID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	// Maybe a double click on Start: the same case is fine.
	cur, err := s.Lock(ctx, userID)
	if err != nil {
		return nil, err
	}
	if cur != nil && cur.StoryID == storyID {
		return cur, nil
	}
	return nil, ErrLockedElsewhere
}

func (s *PGStore) Notes(ctx context.Context, userID, storyID string) ([]Note, error) {
	rows, err := s.q.ListCaseNotes(ctx, dbgen.ListCaseNotesParams{UserID: userID, StoryID: storyID})
	if err != nil {
		return nil, err
	}
	out := make([]Note, 0, len(rows))
	for _, r := range rows {
		out = append(out, Note{ID: r.ID, SuspectID: r.SuspectID, ChatMessageID: r.ChatMessageID, Body: r.Body, CreatedAt: r.CreatedAt})
	}
	return out, nil
}

func (s *PGStore) AddNote(ctx context.Context, userID, storyID string, suspectID, chatMessageID *string, body string) (*Note, error) {
	ok, err := s.q.CaseIsPublished(ctx, storyID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	if suspectID != nil {
		found, err := s.q.SuspectInCase(ctx, dbgen.SuspectInCaseParams{ID: *suspectID, CaseID: storyID})
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, ErrBadSuspect
		}
	}
	row, err := s.q.CreateCaseNote(ctx, dbgen.CreateCaseNoteParams{
		UserID: userID, StoryID: storyID, SuspectID: suspectID, ChatMessageID: chatMessageID, Body: body, MaxNotes: MaxNotesPerCase,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotesFull
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.ForeignKeyViolation {
		return nil, ErrUserGone
	}
	if err != nil {
		return nil, err
	}
	return &Note{ID: row.ID, SuspectID: row.SuspectID, ChatMessageID: row.ChatMessageID, Body: row.Body, CreatedAt: row.CreatedAt}, nil
}

func (s *PGStore) DeleteNote(ctx context.Context, userID, noteID string) error {
	n, err := s.q.DeleteCaseNote(ctx, dbgen.DeleteCaseNoteParams{ID: noteID, UserID: userID})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
