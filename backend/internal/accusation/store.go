// Package accusation closes a case (Phase 5.1): the player names a suspect, the
// server checks it against the true culprit, pays or takes trophies and frees the
// player's Active Case Lock, all in one transaction. A win pays trophies only: there
// is no token reward (the reward_tokens column was removed in migration 00013).
package accusation

import (
	"context"
	"errors"

	"backend/internal/db/dbgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrNotLocked means the player holds no active lock for this case.
	ErrNotLocked = errors.New("accusation: no active lock for this case")
	// ErrBadSuspect means the suspect does not belong to the case.
	ErrBadSuspect = errors.New("accusation: suspect is not part of this case")
	// ErrNotFound means the player row no longer exists.
	ErrNotFound = errors.New("accusation: not found")
)

// Outcome is the result of a settled accusation.
type Outcome struct {
	Correct     bool   `json:"correct"`
	Trophies    int64  `json:"trophies"`        // the player's balance after the settlement
	Change      int64  `json:"trophies_change"` // what the balance really moved by (a loss stops at zero)
	CulpritID   string `json:"culprit_id"`
	CulpritName string `json:"culprit_name"`
	Resolution  string `json:"resolution"` // the full truth of the crime
	// HintPenalty is what the revealed hints took off the reward (0 on a loss, where they cost nothing extra).
	HintPenalty int64 `json:"hint_penalty"`
}

// Store is the PostgreSQL side; *PGStore implements it.
type Store interface {
	// Accuse settles the accusation atomically. It returns ErrNotLocked when the
	// player is not inside the case and ErrBadSuspect for a suspect of another case;
	// in both cases nothing changes (the lock stays).
	Accuse(ctx context.Context, userID, storyID, suspectID string) (*Outcome, error)
}

// PGStore implements Store. All SQL is in sql/queries/accusation.sql (sqlc).
type PGStore struct {
	pool *pgxpool.Pool
	q    *dbgen.Queries
}

func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{pool: pool, q: dbgen.New(pool)} }

func (s *PGStore) Accuse(ctx context.Context, userID, storyID, suspectID string) (*Outcome, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	// Rolls back on every early return; a no-op after Commit.
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	q := s.q.WithTx(tx)

	// Deleting the lock is the claim on this play-through: a second accusation sent
	// at the same moment finds no row and is refused.
	lock, err := q.ReleaseCaseLock(ctx, dbgen.ReleaseCaseLockParams{UserID: userID, StoryID: storyID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotLocked
		}
		return nil, err
	}
	rules, err := q.GetAccusationCase(ctx, storyID)
	if err != nil {
		return nil, err
	}
	accused, err := q.GetAccusedSuspect(ctx, dbgen.GetAccusedSuspectParams{ID: suspectID, CaseID: storyID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrBadSuspect
	}
	if err != nil {
		return nil, err
	}
	culprit, err := q.GetCulprit(ctx, storyID)
	if err != nil {
		return nil, err
	}

	delta, solved := -rules.PenaltyTrophies, int32(0)
	var hintPenalty int32
	if accused.IsCulprit {
		// Hints only cost something when the case is solved: their total comes off the
		// reward, which never drops below zero.
		reward := max(rules.RewardTrophies-HintPenalty(lock.HintsUsed), 0)
		hintPenalty = rules.RewardTrophies - reward
		delta, solved = reward, 1
	}
	res, err := q.ApplyAccusationResult(ctx, dbgen.ApplyAccusationResultParams{UserID: userID, Delta: delta, Solved: solved})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := q.InsertCaseResult(ctx, dbgen.InsertCaseResultParams{
		UserID: userID, StoryID: storyID, SuspectID: &accused.ID, Correct: accused.IsCulprit,
		TrophiesChange: res.TrophiesAfter - res.TrophiesBefore, TrophiesAfter: res.TrophiesAfter, HintPenalty: hintPenalty,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	resolution := rules.Resolution
	if resolution == "" {
		resolution = culprit.HiddenTruth
	}
	return &Outcome{
		Correct: accused.IsCulprit, Trophies: int64(res.TrophiesAfter), Change: int64(res.TrophiesAfter - res.TrophiesBefore),
		CulpritID: culprit.ID, CulpritName: culprit.Name, Resolution: resolution, HintPenalty: int64(hintPenalty),
	}, nil
}

// HintPenalty is the total cost of n revealed hints: hint k costs k trophies, so
// 1 hint = 1, 2 = 3, 3 = 6.
func HintPenalty(n int32) int32 { return n * (n + 1) / 2 }
