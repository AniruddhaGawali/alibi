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
	// ErrClosed means the player already settled their accusation for this case; it is played once.
	ErrClosed = errors.New("cases: case already closed")
	// ErrNotLocked means the player is not inside this case, so they have no hints to reveal.
	ErrNotLocked = errors.New("cases: no active lock for this case")
	// ErrNoHints means every hint of the case has been revealed (or it has none).
	ErrNoHints = errors.New("cases: no hints remaining")
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
	// HintsTotal is how many hints the case has; HintsUsed how many this player revealed
	// in their current play-through, and Hints their text (only those, never the rest).
	HintsTotal int      `json:"hints_total"`
	HintsUsed  int      `json:"hints_used"`
	Hints      []string `json:"hints"`
}

// HintsPerCase is the most hints a case can have.
const HintsPerCase = 3

// Hint is a hint just revealed. Number is its position (1-3) and also what it costs:
// that many trophies come off the reward if the player solves the case.
type Hint struct {
	Hint       string `json:"hint"`
	Number     int    `json:"number"`
	Cost       int    `json:"cost"`
	HintsTotal int    `json:"hints_total"`
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
	// It carries the caller's own revealed hints (none when they are not playing it).
	Briefing(ctx context.Context, userID, id string) (*Briefing, error)
	// RevealHint reveals the player's next hint: ErrNotLocked when they are not in the
	// case, ErrNoHints when none remain.
	RevealHint(ctx context.Context, userID, storyID string) (*Hint, error)
	// Suspects returns ErrNotFound for an unknown or unpublished case.
	Suspects(ctx context.Context, id string) ([]Suspect, error)
	// Lock returns the user's active lock, or nil when they are free.
	Lock(ctx context.Context, userID string) (*Lock, error)
	// Start locks the user into the case. Starting the case they are already in
	// returns the existing lock; another case returns ErrLockedElsewhere.
	Start(ctx context.Context, userID, storyID string) (*Lock, error)

	// Result is the player's settled accusation for the case, with the closing
	// reveal; ErrNotFound while the case is still open to them.
	Result(ctx context.Context, userID, storyID string) (*Result, error)

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

// Result is a closed case as the player who closed it sees it. The solution is
// shown only because their accusation has been settled. The JSON matches the
// accuse response.
type Result struct {
	Correct     bool   `json:"correct"`
	Trophies    int64  `json:"trophies"`
	Change      int64  `json:"trophies_change"`
	CulpritID   string `json:"culprit_id"`
	CulpritName string `json:"culprit_name"`
	Resolution  string `json:"resolution"`
	// HintPenalty is what the hints took off the reward when the case was won.
	HintPenalty int64 `json:"hint_penalty"`
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
type PGStore struct {
	pool *pgxpool.Pool
	q    *dbgen.Queries
}

func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{pool: pool, q: dbgen.New(pool)} }

func (s *PGStore) Briefing(ctx context.Context, userID, id string) (*Briefing, error) {
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
		Scenes: make([]Scene, 0, len(scenes)), HintsTotal: int(row.HintsTotal), Hints: []string{},
	}
	for _, sc := range scenes {
		b.Scenes = append(b.Scenes, Scene{ID: sc.ID, SceneName: sc.SceneName, ImageURL: sc.ImageUrl, DisplayOrder: int(sc.DisplayOrder)})
	}
	h, err := s.q.GetUnlockedHints(ctx, dbgen.GetUnlockedHintsParams{UserID: userID, StoryID: id})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if err == nil {
		b.HintsUsed, b.Hints = int(h.HintsUsed), h.Hints
	}
	return b, nil
}

func (s *PGStore) RevealHint(ctx context.Context, userID, storyID string) (*Hint, error) {
	r, err := s.q.RevealNextHint(ctx, dbgen.RevealNextHintParams{UserID: userID, StoryID: storyID})
	if err == nil {
		st, err := s.q.GetHintState(ctx, dbgen.GetHintStateParams{UserID: userID, StoryID: storyID})
		if err != nil {
			return nil, err
		}
		return &Hint{Hint: r.Hint, Number: int(r.HintsUsed), Cost: int(r.HintsUsed), HintsTotal: int(st.HintsTotal)}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	// Nothing changed: either the player is not in this case, or no hint is left.
	if _, err := s.q.GetHintState(ctx, dbgen.GetHintStateParams{UserID: userID, StoryID: storyID}); errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotLocked
	} else if err != nil {
		return nil, err
	}
	return nil, ErrNoHints
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
	closed, err := s.q.CaseIsClosed(ctx, dbgen.CaseIsClosedParams{UserID: userID, StoryID: storyID})
	if err != nil {
		return nil, err
	}
	if closed {
		return nil, ErrClosed
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

func (s *PGStore) Result(ctx context.Context, userID, storyID string) (*Result, error) {
	r, err := s.q.GetCaseResult(ctx, dbgen.GetCaseResultParams{UserID: userID, StoryID: storyID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	resolution := r.Resolution
	if resolution == "" {
		resolution = r.HiddenTruth
	}
	return &Result{
		Correct: r.Correct, Trophies: int64(r.TrophiesAfter), Change: int64(r.TrophiesChange),
		CulpritID: r.CulpritID, CulpritName: r.CulpritName, Resolution: resolution, HintPenalty: int64(r.HintPenalty),
	}, nil
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
	// The count and the insert run under an advisory lock, so parallel requests
	// cannot all see 199 notes and all insert.
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	if err := q.LockCaseNotes(ctx, dbgen.LockCaseNotesParams{UserID: userID, StoryID: storyID}); err != nil {
		return nil, err
	}
	row, err := q.CreateCaseNote(ctx, dbgen.CreateCaseNoteParams{
		UserID: userID, StoryID: storyID, SuspectID: suspectID, ChatMessageID: chatMessageID, Body: body, MaxNotes: MaxNotesPerCase,
	})
	if err == nil {
		if err = tx.Commit(ctx); err != nil {
			return nil, err
		}
	}
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
