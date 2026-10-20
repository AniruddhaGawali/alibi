// Package admin holds the admin panel API (P5.3): global settings, case, suspect and
// scene management, and image uploads. Every route is wrapped in
// Handler.AdminOnly, which checks the token's role and then users.role.
package admin

import (
	"context"
	"errors"
	"strings"
	"time"

	"backend/internal/db/dbgen"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrNotFound means the case, suspect, scene or user does not exist.
	ErrNotFound = errors.New("admin: not found")
	// ErrInvalid means the database rejected a value (a CHECK constraint).
	ErrInvalid = errors.New("admin: invalid value")
	// ErrInUse means the case has been played or is being played, so it cannot be deleted.
	ErrInUse = errors.New("admin: case in use")
	// ErrNameTaken means another suspect of the case has this name.
	ErrNameTaken = errors.New("admin: suspect name taken")
	// ErrOrderTaken means another scene of the case has this display order.
	ErrOrderTaken = errors.New("admin: scene order taken")
	// ErrNotReady means the change would leave a published case unplayable, or
	// publishes one that is not ready: it needs 2+ suspects and exactly one culprit.
	ErrNotReady = errors.New("admin: case not ready to publish")
	// ErrNeedsKeywords means a suspect has a hidden truth or confession triggers but
	// no confession keywords, so their secrets would be in every prompt.
	ErrNeedsKeywords = errors.New("admin: secrets need confession keywords")
	// ErrStaleOrder means a new order did not list every case exactly once (one was
	// added or deleted since the list was loaded).
	ErrStaleOrder = errors.New("admin: case order does not match the cases")
)

// MinSuspects is how many suspects a published case needs.
const MinSuspects = 2

// Case is a case as admins see it, including its model and hints.
type Case struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	Synopsis        string    `json:"synopsis"`
	MainPlot        string    `json:"main_plot"`
	ThumbnailURL    string    `json:"thumbnail_url"`
	DifficultyLevel string    `json:"difficulty_level"`
	RewardTrophies  int       `json:"reward_trophies"`
	PenaltyTrophies int       `json:"penalty_trophies"`
	Resolution      string    `json:"resolution"`
	Hints           []string  `json:"hints"` // up to 3, revealed one by one (hint n costs n trophies on a win)
	IsPublished     bool      `json:"is_published"`
	LLMModel        string    `json:"llm_model"`
	MusicTrack      string    `json:"music_track"` // a MusicTracks key; "" plays the main theme
	ChatTips        []string  `json:"chat_tips"`   // up to 5 public tips shown to investigators in the chat
	CreatedAt       time.Time `json:"created_at"`
}

// CaseSummary is a row of the admin case list.
type CaseSummary struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	Synopsis        string    `json:"synopsis"`
	ThumbnailURL    string    `json:"thumbnail_url"`
	DifficultyLevel string    `json:"difficulty_level"`
	RewardTrophies  int       `json:"reward_trophies"`
	PenaltyTrophies int       `json:"penalty_trophies"`
	IsPublished     bool      `json:"is_published"`
	LLMModel        string    `json:"llm_model"`
	MusicTrack      string    `json:"music_track"` // a MusicTracks key; "" plays the main theme
	CreatedAt       time.Time `json:"created_at"`
	SuspectCount    int       `json:"suspect_count"`
	CulpritCount    int       `json:"culprit_count"`
	Plays           int       `json:"plays"` // settled accusations
}

// CaseDetail is a case with everything the case builder edits.
type CaseDetail struct {
	Case
	Suspects []Suspect `json:"suspects"`
	Scenes   []Scene   `json:"scenes"`
}

// Suspect is a suspect with every field, secrets included. Admin responses only.
type Suspect struct {
	ID                   string   `json:"id"`
	CaseID               string   `json:"case_id"`
	Name                 string   `json:"name"`
	Role                 string   `json:"role"` // e.g. "the butler"
	AvatarURL            string   `json:"avatar_url"`
	InitialTestimony     string   `json:"initial_testimony"` // shown to players
	PublicKnowledge      string   `json:"public_knowledge"`  // background shown to investigators
	IsCulprit            bool     `json:"is_culprit"`
	DisplayOrder         int      `json:"display_order"`
	RelationshipToVictim string   `json:"relationship_to_victim"`
	Persona              string   `json:"persona"`             // public personality, also known to the other suspects
	PublicAlibi          string   `json:"public_alibi"`        // the story they tell
	HiddenTruth          string   `json:"hidden_truth"`        // what really happened
	ConfessionTriggers   string   `json:"confession_triggers"` // what makes them crack
	ConfessionKeywords   []string `json:"confession_keywords"` // gate the secrets into the prompt (llm.Unlocked)
}

// SuspectInput is a new suspect; every field is set.
type SuspectInput struct {
	Name, Role, AvatarURL, InitialTestimony, PublicKnowledge                    string
	IsCulprit                                                                   bool
	DisplayOrder                                                                int32
	RelationshipToVictim, Persona, PublicAlibi, HiddenTruth, ConfessionTriggers string
	ConfessionKeywords                                                          []string
}

// SuspectPatch changes only the non-nil fields.
type SuspectPatch struct {
	Name, Role, AvatarURL, InitialTestimony, PublicKnowledge                    *string
	IsCulprit                                                                   *bool
	DisplayOrder                                                                *int32
	RelationshipToVictim, Persona, PublicAlibi, HiddenTruth, ConfessionTriggers *string
	ConfessionKeywords                                                          []string // nil keeps them; empty clears them
}

// Scene is a crime-scene picture shown in the briefing.
type Scene struct {
	ID           string `json:"id"`
	CaseID       string `json:"case_id"`
	SceneName    string `json:"scene_name"`
	ImageURL     string `json:"image_url"`
	DisplayOrder int    `json:"display_order"`
}

// Player is an account as the admin panel lists it.
type Player struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	Role         string    `json:"role"`
	TokenBalance int64     `json:"token_balance"`
	Trophies     int       `json:"trophies"`
	CreatedAt    time.Time `json:"created_at"`
}

// ScenePatch changes only the non-nil fields.
type ScenePatch struct {
	SceneName, ImageURL *string
	DisplayOrder        *int32
}

// CaseInput is a new case; every field is set.
type CaseInput struct {
	Title, Synopsis, MainPlot, ThumbnailURL, DifficultyLevel string
	RewardTrophies, PenaltyTrophies                          int32
	Resolution                                               string
	Hints                                                    []string
	IsPublished                                              bool
	LLMModel, MusicTrack                                     string
	ChatTips                                                 []string
}

// CasePatch changes only the non-nil fields.
type CasePatch struct {
	Title, Synopsis, MainPlot, ThumbnailURL, DifficultyLevel *string
	RewardTrophies, PenaltyTrophies                          *int32
	Resolution                                               *string
	Hints                                                    []string // nil keeps the current hints; empty clears them
	IsPublished                                              *bool
	LLMModel, MusicTrack                                     *string
	ChatTips                                                 []string // nil keeps the current tips; empty clears them
}

// Store is the PostgreSQL side; *PGStore implements it. Updates and deletes also
// return the image URLs they replaced or dropped, so the handler can remove them
// from storage.
type Store interface {
	// Role returns the user's role, or ErrNotFound.
	Role(ctx context.Context, userID string) (string, error)

	ListCases(ctx context.Context) ([]CaseSummary, error)
	GetCase(ctx context.Context, id string) (*CaseDetail, error)
	CaseExists(ctx context.Context, id string) (bool, error)
	// CreateCase returns ErrNotReady for a case created as published (it has no suspects yet).
	CreateCase(ctx context.Context, in CaseInput) (*Case, error)
	// UpdateCase returns ErrNotFound for an unknown case and ErrNotReady when it
	// would publish a case that is not ready.
	UpdateCase(ctx context.Context, id string, p CasePatch) (c *Case, oldThumbnail string, err error)
	// DeleteCase returns ErrInUse when anyone has played or is playing it.
	DeleteCase(ctx context.Context, id string) (images []string, err error)
	// ReorderCases sets the order of all cases (ids first to last). It returns
	// ErrStaleOrder unless ids holds every case exactly once.
	ReorderCases(ctx context.Context, ids []string) error

	CreateSuspect(ctx context.Context, caseID string, in SuspectInput) (*Suspect, error)
	UpdateSuspect(ctx context.Context, caseID, id string, p SuspectPatch) (s *Suspect, oldAvatar string, err error)
	DeleteSuspect(ctx context.Context, caseID, id string) (avatar string, err error)

	// SearchUsers finds players by username or email (case-insensitive substring).
	SearchUsers(ctx context.Context, q string, limit int) ([]Player, error)
	// GetUser returns ErrNotFound for an unknown user.
	GetUser(ctx context.Context, id string) (*Player, error)

	CreateScene(ctx context.Context, caseID string, name, imageURL string, order int32) (*Scene, error)
	UpdateScene(ctx context.Context, caseID, id string, p ScenePatch) (s *Scene, oldImage string, err error)
	DeleteScene(ctx context.Context, caseID, id string) (image string, err error)
}

// PGStore implements Store. All SQL is in sql/queries/admin.sql (sqlc).
type PGStore struct {
	pool *pgxpool.Pool
	q    *dbgen.Queries
}

func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{pool: pool, q: dbgen.New(pool)} }

// tx runs fn in a transaction and commits when it returns nil.
func (s *PGStore) tx(ctx context.Context, fn func(q *dbgen.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }() // a no-op after Commit
	if err := fn(s.q.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PGStore) ReorderCases(ctx context.Context, ids []string) error {
	return s.tx(ctx, func(q *dbgen.Queries) error {
		total, err := q.AdminCountCases(ctx)
		if err != nil {
			return err
		}
		if int(total) != len(ids) {
			return ErrStaleOrder
		}
		n, err := q.AdminReorderCases(ctx, ids)
		if err != nil {
			return err
		}
		if n != int64(total) { // an unknown id (the handler already refuses duplicates)
			return ErrStaleOrder
		}
		return nil
	})
}

func (s *PGStore) Role(ctx context.Context, userID string) (string, error) {
	role, err := s.q.UserRole(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return role, err
}

func (s *PGStore) ListCases(ctx context.Context) ([]CaseSummary, error) {
	rows, err := s.q.AdminListCases(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]CaseSummary, len(rows))
	for i, r := range rows {
		out[i] = CaseSummary{
			ID: r.ID, Title: r.Title, Synopsis: r.Synopsis, ThumbnailURL: r.ThumbnailUrl, DifficultyLevel: r.DifficultyLevel,
			RewardTrophies: int(r.RewardTrophies), PenaltyTrophies: int(r.PenaltyTrophies), IsPublished: r.IsPublished,
			LLMModel: r.LlmModel, MusicTrack: r.MusicTrack, CreatedAt: r.CreatedAt,
			SuspectCount: int(r.SuspectCount), CulpritCount: int(r.CulpritCount), Plays: int(r.Plays),
		}
	}
	return out, nil
}

func (s *PGStore) GetCase(ctx context.Context, id string) (*CaseDetail, error) {
	r, err := s.q.AdminGetCase(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	suspects, err := s.q.AdminListSuspects(ctx, id)
	if err != nil {
		return nil, err
	}
	scenes, err := s.q.AdminListScenes(ctx, id)
	if err != nil {
		return nil, err
	}
	d := &CaseDetail{
		Case: Case{
			ID: r.ID, Title: r.Title, Synopsis: r.Synopsis, MainPlot: r.MainPlot, ThumbnailURL: r.ThumbnailUrl,
			DifficultyLevel: r.DifficultyLevel, RewardTrophies: int(r.RewardTrophies), PenaltyTrophies: int(r.PenaltyTrophies),
			Resolution: r.Resolution, Hints: r.Hints, IsPublished: r.IsPublished, LLMModel: r.LlmModel, MusicTrack: r.MusicTrack, ChatTips: r.ChatTips, CreatedAt: r.CreatedAt,
		},
		Suspects: make([]Suspect, len(suspects)),
		Scenes:   make([]Scene, len(scenes)),
	}
	for i, p := range suspects {
		d.Suspects[i] = suspectOf(p)
	}
	for i, c := range scenes {
		d.Scenes[i] = sceneOf(c)
	}
	return d, nil
}

func (s *PGStore) CaseExists(ctx context.Context, id string) (bool, error) {
	return s.q.AdminCaseExists(ctx, id)
}

func (s *PGStore) CreateCase(ctx context.Context, in CaseInput) (*Case, error) {
	if in.IsPublished {
		return nil, ErrNotReady // no suspects yet: create a draft, add suspects, then publish
	}
	if in.Hints == nil {
		in.Hints = []string{} // the column is NOT NULL
	}
	if in.ChatTips == nil {
		in.ChatTips = []string{}
	}
	r, err := s.q.AdminCreateCase(ctx, dbgen.AdminCreateCaseParams{
		Title: in.Title, Synopsis: in.Synopsis, MainPlot: in.MainPlot, ThumbnailUrl: in.ThumbnailURL,
		DifficultyLevel: in.DifficultyLevel, RewardTrophies: in.RewardTrophies, PenaltyTrophies: in.PenaltyTrophies, Resolution: in.Resolution, Hints: in.Hints,
		IsPublished: in.IsPublished, LlmModel: in.LLMModel, MusicTrack: in.MusicTrack, ChatTips: in.ChatTips,
	})
	if err != nil {
		return nil, mapErr(err)
	}
	return &Case{
		ID: r.ID, Title: r.Title, Synopsis: r.Synopsis, MainPlot: r.MainPlot, ThumbnailURL: r.ThumbnailUrl,
		DifficultyLevel: r.DifficultyLevel, RewardTrophies: int(r.RewardTrophies), PenaltyTrophies: int(r.PenaltyTrophies), Resolution: r.Resolution, Hints: r.Hints,
		IsPublished: r.IsPublished, LLMModel: r.LlmModel, MusicTrack: r.MusicTrack, ChatTips: r.ChatTips, CreatedAt: r.CreatedAt,
	}, nil
}

func (s *PGStore) UpdateCase(ctx context.Context, id string, p CasePatch) (*Case, string, error) {
	var (
		c   *Case
		old string
	)
	err := s.tx(ctx, func(q *dbgen.Queries) error {
		r, err := q.AdminUpdateCase(ctx, dbgen.AdminUpdateCaseParams{
			ID: id, Title: p.Title, Synopsis: p.Synopsis, MainPlot: p.MainPlot, ThumbnailUrl: p.ThumbnailURL,
			DifficultyLevel: p.DifficultyLevel, RewardTrophies: p.RewardTrophies, PenaltyTrophies: p.PenaltyTrophies, Resolution: p.Resolution, Hints: p.Hints,
			IsPublished: p.IsPublished, LlmModel: p.LLMModel, MusicTrack: p.MusicTrack, ChatTips: p.ChatTips,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return mapErr(err)
		}
		if r.IsPublished {
			if err := checkReady(ctx, q, id); err != nil {
				return err
			}
		}
		c = &Case{
			ID: r.ID, Title: r.Title, Synopsis: r.Synopsis, MainPlot: r.MainPlot, ThumbnailURL: r.ThumbnailUrl,
			DifficultyLevel: r.DifficultyLevel, RewardTrophies: int(r.RewardTrophies), PenaltyTrophies: int(r.PenaltyTrophies), Resolution: r.Resolution, Hints: r.Hints,
			IsPublished: r.IsPublished, LLMModel: r.LlmModel, MusicTrack: r.MusicTrack, ChatTips: r.ChatTips, CreatedAt: r.CreatedAt,
		}
		old = r.OldThumbnailUrl
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	return c, old, nil
}

func (s *PGStore) DeleteCase(ctx context.Context, id string) ([]string, error) {
	var images []string
	err := s.tx(ctx, func(q *dbgen.Queries) error {
		found, err := q.AdminCaseExists(ctx, id)
		if err != nil {
			return err
		}
		if !found {
			return ErrNotFound
		}
		inUse, err := q.AdminCaseInUse(ctx, id)
		if err != nil {
			return err
		}
		if inUse {
			return ErrInUse
		}
		if images, err = q.AdminCaseImageURLs(ctx, id); err != nil {
			return err
		}
		if _, err := q.AdminDeleteCase(ctx, id); err != nil {
			if isPg(err, pgerrcode.ForeignKeyViolation) {
				return ErrInUse // a player started it in the meantime
			}
			return err
		}
		return nil
	})
	return images, err
}

func (s *PGStore) CreateSuspect(ctx context.Context, caseID string, in SuspectInput) (*Suspect, error) {
	if in.ConfessionKeywords == nil {
		in.ConfessionKeywords = []string{}
	}
	var out Suspect
	err := s.tx(ctx, func(q *dbgen.Queries) error {
		if in.IsCulprit {
			if err := q.AdminClearCulprit(ctx, dbgen.AdminClearCulpritParams{CaseID: caseID}); err != nil {
				return err
			}
		}
		r, err := q.AdminCreateSuspect(ctx, dbgen.AdminCreateSuspectParams{
			CaseID: caseID, Name: in.Name, Role: in.Role, AvatarUrl: in.AvatarURL, InitialTestimony: in.InitialTestimony,
			IsCulprit: in.IsCulprit, DisplayOrder: in.DisplayOrder, RelationshipToVictim: in.RelationshipToVictim,
			Persona: in.Persona, PublicAlibi: in.PublicAlibi, HiddenTruth: in.HiddenTruth,
			ConfessionTriggers: in.ConfessionTriggers, ConfessionKeywords: in.ConfessionKeywords, PublicKnowledge: in.PublicKnowledge,
		})
		if err != nil {
			if isPg(err, pgerrcode.ForeignKeyViolation) {
				return ErrNotFound
			}
			return mapErr(err)
		}
		out = suspectOf(r)
		return afterSuspectWrite(ctx, q, &out)
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *PGStore) UpdateSuspect(ctx context.Context, caseID, id string, p SuspectPatch) (*Suspect, string, error) {
	var (
		out Suspect
		old string
	)
	err := s.tx(ctx, func(q *dbgen.Queries) error {
		if p.IsCulprit != nil && *p.IsCulprit {
			if err := q.AdminClearCulprit(ctx, dbgen.AdminClearCulpritParams{CaseID: caseID, KeepID: &id}); err != nil {
				return err
			}
		}
		r, err := q.AdminUpdateSuspect(ctx, dbgen.AdminUpdateSuspectParams{
			ID: id, CaseID: caseID, Name: p.Name, Role: p.Role, AvatarUrl: p.AvatarURL, InitialTestimony: p.InitialTestimony,
			IsCulprit: p.IsCulprit, DisplayOrder: p.DisplayOrder, RelationshipToVictim: p.RelationshipToVictim,
			Persona: p.Persona, PublicAlibi: p.PublicAlibi, HiddenTruth: p.HiddenTruth,
			ConfessionTriggers: p.ConfessionTriggers, ConfessionKeywords: p.ConfessionKeywords, PublicKnowledge: p.PublicKnowledge,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return mapErr(err)
		}
		out = suspectOf(dbgen.Suspect{
			ID: r.ID, CaseID: r.CaseID, Name: r.Name, Role: r.Role, AvatarUrl: r.AvatarUrl, InitialTestimony: r.InitialTestimony,
			IsCulprit: r.IsCulprit, DisplayOrder: r.DisplayOrder, RelationshipToVictim: r.RelationshipToVictim, Persona: r.Persona,
			PublicAlibi: r.PublicAlibi, HiddenTruth: r.HiddenTruth, ConfessionTriggers: r.ConfessionTriggers, ConfessionKeywords: r.ConfessionKeywords,
			PublicKnowledge: r.PublicKnowledge,
		})
		old = r.OldAvatarUrl
		return afterSuspectWrite(ctx, q, &out)
	})
	if err != nil {
		return nil, "", err
	}
	return &out, old, nil
}

func (s *PGStore) DeleteSuspect(ctx context.Context, caseID, id string) (string, error) {
	var avatar string
	err := s.tx(ctx, func(q *dbgen.Queries) error {
		var err error
		avatar, err = q.AdminDeleteSuspect(ctx, dbgen.AdminDeleteSuspectParams{ID: id, CaseID: caseID})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		return checkReadyIfPublished(ctx, q, caseID)
	})
	return avatar, err
}

// afterSuspectWrite enforces the rules a single row cannot: secrets need keywords,
// and a published case must stay playable.
func afterSuspectWrite(ctx context.Context, q *dbgen.Queries, p *Suspect) error {
	if (p.HiddenTruth != "" || p.ConfessionTriggers != "") && len(p.ConfessionKeywords) == 0 {
		return ErrNeedsKeywords
	}
	return checkReadyIfPublished(ctx, q, p.CaseID)
}

func checkReadyIfPublished(ctx context.Context, q *dbgen.Queries, caseID string) error {
	c, err := q.AdminGetCase(ctx, caseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if !c.IsPublished {
		return nil
	}
	return checkReady(ctx, q, caseID)
}

// checkReady returns ErrNotReady unless the case has MinSuspects and one culprit.
func checkReady(ctx context.Context, q *dbgen.Queries, caseID string) error {
	r, err := q.AdminCaseReadiness(ctx, caseID)
	if err != nil {
		return err
	}
	if r.SuspectCount < MinSuspects || r.CulpritCount != 1 {
		return ErrNotReady
	}
	return nil
}

func (s *PGStore) CreateScene(ctx context.Context, caseID, name, imageURL string, order int32) (*Scene, error) {
	r, err := s.q.AdminCreateScene(ctx, dbgen.AdminCreateSceneParams{CaseID: caseID, SceneName: name, ImageUrl: imageURL, DisplayOrder: order})
	if isPg(err, pgerrcode.ForeignKeyViolation) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, mapErr(err)
	}
	out := sceneOf(r)
	return &out, nil
}

func (s *PGStore) UpdateScene(ctx context.Context, caseID, id string, p ScenePatch) (*Scene, string, error) {
	r, err := s.q.AdminUpdateScene(ctx, dbgen.AdminUpdateSceneParams{
		ID: id, CaseID: caseID, SceneName: p.SceneName, ImageUrl: p.ImageURL, DisplayOrder: p.DisplayOrder,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", ErrNotFound
	}
	if err != nil {
		return nil, "", mapErr(err)
	}
	out := sceneOf(dbgen.CaseScene{ID: r.ID, CaseID: r.CaseID, SceneName: r.SceneName, ImageUrl: r.ImageUrl, DisplayOrder: r.DisplayOrder})
	return &out, r.OldImageUrl, nil
}

func (s *PGStore) DeleteScene(ctx context.Context, caseID, id string) (string, error) {
	img, err := s.q.AdminDeleteScene(ctx, dbgen.AdminDeleteSceneParams{ID: id, CaseID: caseID})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return img, err
}

// likeEscaper keeps a search term literal inside ILIKE '%term%'.
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func (s *PGStore) SearchUsers(ctx context.Context, q string, limit int) ([]Player, error) {
	rows, err := s.q.AdminSearchUsers(ctx, dbgen.AdminSearchUsersParams{Q: likeEscaper.Replace(q), Lim: int32(limit)})
	if err != nil {
		return nil, err
	}
	out := make([]Player, len(rows))
	for i, r := range rows {
		out[i] = Player{ID: r.ID, Username: r.Username, Email: r.Email, Role: r.Role, TokenBalance: r.TokenBalance, Trophies: int(r.Trophies), CreatedAt: r.CreatedAt}
	}
	return out, nil
}

func (s *PGStore) GetUser(ctx context.Context, id string) (*Player, error) {
	r, err := s.q.AdminGetUser(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &Player{ID: r.ID, Username: r.Username, Email: r.Email, Role: r.Role, TokenBalance: r.TokenBalance, Trophies: int(r.Trophies), CreatedAt: r.CreatedAt}, nil
}

func suspectOf(r dbgen.Suspect) Suspect {
	kw := r.ConfessionKeywords
	if kw == nil {
		kw = []string{}
	}
	return Suspect{
		ID: r.ID, CaseID: r.CaseID, Name: r.Name, Role: r.Role, AvatarURL: r.AvatarUrl, InitialTestimony: r.InitialTestimony,
		PublicKnowledge: r.PublicKnowledge, IsCulprit: r.IsCulprit, DisplayOrder: int(r.DisplayOrder), RelationshipToVictim: r.RelationshipToVictim,
		Persona: r.Persona, PublicAlibi: r.PublicAlibi, HiddenTruth: r.HiddenTruth,
		ConfessionTriggers: r.ConfessionTriggers, ConfessionKeywords: kw,
	}
}

func sceneOf(r dbgen.CaseScene) Scene {
	return Scene{ID: r.ID, CaseID: r.CaseID, SceneName: r.SceneName, ImageURL: r.ImageUrl, DisplayOrder: int(r.DisplayOrder)}
}

func isPg(err error, code string) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == code
}

func mapErr(err error) error {
	var pg *pgconn.PgError
	if !errors.As(err, &pg) {
		return err
	}
	switch {
	case pg.Code == pgerrcode.CheckViolation:
		return ErrInvalid
	case pg.Code == pgerrcode.UniqueViolation && pg.ConstraintName == "suspects_name_key":
		return ErrNameTaken
	case pg.Code == pgerrcode.UniqueViolation && pg.ConstraintName == "case_scenes_order_key":
		return ErrOrderTaken
	}
	return err
}
