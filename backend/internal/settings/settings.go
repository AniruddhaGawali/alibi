// Package settings holds the global game settings the admin panel edits (P5.3):
// the daily question-token allowance and when it is reset. Postgres (app_settings,
// one row) is the source of truth; every API replica keeps a copy in memory for
// cacheTTL, so an edit reaches the other replicas within that time.
package settings

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"sync"
	"time"

	"backend/internal/db/dbgen"
	"backend/pkg/utils"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MaxDailyTokens is the most tokens a day can grant: a player never holds more
// than user.MaxTokenBalance, and app_settings_daily_tokens_range mirrors it.
const MaxDailyTokens = 10

// cacheTTL is how long a replica trusts its copy of the settings.
const cacheTTL = 30 * time.Second

// ErrNotFound means the settings row is missing (Seed has not run).
var ErrNotFound = errors.New("settings: not found")

// Settings are the global game settings.
type Settings struct {
	DailyTokens int       `json:"daily_tokens"`         // every balance is reset to this, and new accounts start with it
	ResetTime   string    `json:"token_reset_time"`     // HH:MM, 24-hour clock
	Timezone    string    `json:"token_reset_timezone"` // IANA zone ResetTime is read in
	UpdatedAt   time.Time `json:"updated_at"`
}

var resetTime = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

// Clock splits ResetTime into hour and minute and loads the zone. It fails for a
// malformed time or an unknown zone.
func (s Settings) Clock() (hour, minute int, loc *time.Location, err error) {
	if !resetTime.MatchString(s.ResetTime) {
		return 0, 0, nil, errors.New("settings: reset time must be HH:MM")
	}
	hour, _ = strconv.Atoi(s.ResetTime[:2])
	minute, _ = strconv.Atoi(s.ResetTime[3:])
	if s.Timezone == "" {
		return 0, 0, nil, errors.New("settings: empty timezone")
	}
	loc, err = time.LoadLocation(s.Timezone)
	if err != nil {
		return 0, 0, nil, err
	}
	return hour, minute, loc, nil
}

// Store is the Postgres side; *PGStore implements it.
type Store interface {
	// Get returns ErrNotFound when the row is missing.
	Get(ctx context.Context) (*Settings, error)
	// Seed inserts s unless a row already exists.
	Seed(ctx context.Context, s Settings) error
	Update(ctx context.Context, s Settings, by string) (*Settings, error)
}

// PGStore implements Store. The SQL is in sql/queries/admin.sql.
type PGStore struct{ q *dbgen.Queries }

func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{q: dbgen.New(pool)} }

func (s *PGStore) Get(ctx context.Context) (*Settings, error) {
	r, err := s.q.GetAppSettings(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &Settings{DailyTokens: int(r.DailyTokens), ResetTime: r.TokenResetTime, Timezone: r.TokenResetTimezone, UpdatedAt: r.UpdatedAt}, nil
}

func (s *PGStore) Seed(ctx context.Context, in Settings) error {
	return s.q.SeedAppSettings(ctx, dbgen.SeedAppSettingsParams{
		DailyTokens: int32(in.DailyTokens), TokenResetTime: in.ResetTime, TokenResetTimezone: in.Timezone,
	})
}

func (s *PGStore) Update(ctx context.Context, in Settings, by string) (*Settings, error) {
	var updatedBy *string
	if by != "" {
		updatedBy = &by
	}
	r, err := s.q.UpdateAppSettings(ctx, dbgen.UpdateAppSettingsParams{
		DailyTokens: int32(in.DailyTokens), TokenResetTime: in.ResetTime, TokenResetTimezone: in.Timezone, UpdatedBy: updatedBy,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &Settings{DailyTokens: int(r.DailyTokens), ResetTime: r.TokenResetTime, Timezone: r.TokenResetTimezone, UpdatedAt: r.UpdatedAt}, nil
}

// Service serves the settings from memory and refreshes them from the store every
// cacheTTL. If Postgres cannot be read it keeps using the last value it had, so a
// database blip never changes the game rules.
type Service struct {
	store  Store
	now    func() time.Time
	mu     sync.Mutex
	cur    Settings
	loaded time.Time
}

// New seeds the row with defaults when it is missing, then loads it. defaults come
// from the environment (DAILY_TOKENS, TOKEN_RESET_TIME, TOKEN_RESET_TIMEZONE) and
// only matter on the first startup; after that the admin panel owns the values.
func New(ctx context.Context, store Store, defaults Settings) (*Service, error) {
	if _, _, _, err := defaults.Clock(); err != nil {
		return nil, err
	}
	if err := store.Seed(ctx, defaults); err != nil {
		return nil, err
	}
	s := &Service{store: store, now: time.Now}
	cur, err := store.Get(ctx)
	if err != nil {
		return nil, err
	}
	s.cur, s.loaded = *cur, s.now()
	return s, nil
}

// Current returns the settings, refreshing them first when the copy is older than
// cacheTTL.
func (s *Service) Current(ctx context.Context) Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.now().Sub(s.loaded) < cacheTTL {
		return s.cur
	}
	cur, err := s.store.Get(ctx)
	if err != nil {
		utils.LogErrorCtx(ctx, "reload settings failed; using the last known values", "err", err)
		s.loaded = s.now() // do not retry on every call while the database is down
		return s.cur
	}
	s.cur, s.loaded = *cur, s.now()
	return s.cur
}

// DailyTokens is Current().DailyTokens.
func (s *Service) DailyTokens(ctx context.Context) int { return s.Current(ctx).DailyTokens }

// Update validates and saves new settings and uses them in this replica at once.
func (s *Service) Update(ctx context.Context, in Settings, by string) (*Settings, error) {
	if _, _, _, err := in.Clock(); err != nil {
		return nil, err
	}
	out, err := s.store.Update(ctx, in, by)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.cur, s.loaded = *out, s.now()
	s.mu.Unlock()
	return out, nil
}
