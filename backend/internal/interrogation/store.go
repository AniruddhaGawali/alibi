// Package interrogation answers a detective's question as a suspect (Phase 4.1):
// moderation, token spend and refund, the per-case OpenRouter model, the suspect's
// system prompt, and publishing the exchange for storage.
package interrogation

import (
	"context"
	"errors"
	"fmt"

	"backend/internal/db/dbgen"
	"backend/internal/llm"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound means the case is unknown or unpublished.
var ErrNotFound = errors.New("interrogation: case not found")

// Lock is the player's active case lock.
type Lock struct {
	CaseID  string // the play-through
	StoryID string // the case being played
}

// CaseConfig is what the prompt and the model routing need from a case.
type CaseConfig struct {
	ID    string
	Title string
	Plot  string
	Model string // OpenRouter model id from stories.llm_model
}

// Suspect is a suspect with the server-only fields used to build its prompt.
type Suspect struct {
	ID string
	llm.Persona
}

// Store reads the interrogation data; *PGStore implements it.
type Store interface {
	// Lock returns the user's active lock, or nil when they have none.
	Lock(ctx context.Context, userID string) (*Lock, error)
	// Case returns ErrNotFound for an unknown or unpublished case.
	Case(ctx context.Context, storyID string) (*CaseConfig, error)
	// Suspect returns the suspect being questioned, with their secrets, and the public
	// profile of every other suspect of the case. Only this one suspect's secrets are
	// loaded. It returns ErrSuspectNotFound when the suspect is not in the case.
	Suspect(ctx context.Context, storyID, suspectID string) (*Suspect, []llm.Acquaintance, error)
}

// PGStore implements Store. All SQL is in sql/queries/interrogation.sql (sqlc).
type PGStore struct{ q *dbgen.Queries }

func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{q: dbgen.New(pool)} }

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

func (s *PGStore) Case(ctx context.Context, storyID string) (*CaseConfig, error) {
	row, err := s.q.GetInterrogationCase(ctx, storyID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &CaseConfig{ID: row.ID, Title: row.Title, Plot: row.MainPlot, Model: row.LlmModel}, nil
}

func (s *PGStore) Suspect(ctx context.Context, storyID, suspectID string) (*Suspect, []llm.Acquaintance, error) {
	r, err := s.q.GetInterrogationSuspect(ctx, dbgen.GetInterrogationSuspectParams{CaseID: storyID, ID: suspectID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, ErrSuspectNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("load suspect: %w", err)
	}
	rows, err := s.q.ListInterrogationAcquaintances(ctx, dbgen.ListInterrogationAcquaintancesParams{CaseID: storyID, SuspectID: suspectID})
	if err != nil {
		return nil, nil, fmt.Errorf("load acquaintances: %w", err)
	}
	others := make([]llm.Acquaintance, len(rows))
	for i, o := range rows {
		others[i] = llm.Acquaintance{Name: o.Name, Role: o.Role, Relationship: o.RelationshipToVictim, Persona: o.Persona}
	}
	return &Suspect{ID: r.ID, Persona: llm.Persona{
		Name: r.Name, Role: r.Role, Relationship: r.RelationshipToVictim, Persona: r.Persona,
		PublicAlibi: r.PublicAlibi, HiddenTruth: r.HiddenTruth, ConfessionTriggers: r.ConfessionTriggers,
		TriggerKeywords: r.ConfessionKeywords, IsCulprit: r.IsCulprit,
	}}, others, nil
}
