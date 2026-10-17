// Package user holds the user record, its PostgreSQL store and the profile
// and avatar endpoints.
package user

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
	ErrNotFound      = errors.New("user: not found")
	ErrEmailTaken    = errors.New("user: email already registered")
	ErrUsernameTaken = errors.New("user: username already taken")
)

type User struct {
	ID              string
	Username        string
	Email           string
	PasswordHash    string
	AvatarDefault   *int       // 1..4 when a default avatar is chosen
	HasAvatar       bool       // true when a custom image is stored (the bytes are not loaded here)
	AvatarUpdatedAt *time.Time // when the custom image was last set
	Trophies        int
	CreatedAt       time.Time
}

// Store is the persistence the auth and profile handlers need.
type Store interface {
	// Create inserts a user with the given starting token balance.
	Create(ctx context.Context, username, email, passwordHash string, tokens int) (*User, error)
	// Availability reports whether the email / username are already registered.
	Availability(ctx context.Context, email, username string) (emailTaken, usernameTaken bool, err error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	GetByID(ctx context.Context, id string) (*User, error)
	SetAvatarDefault(ctx context.Context, id string, n int) error
	// SetAvatarCustom stores the processed JPEG and returns when it was saved.
	SetAvatarCustom(ctx context.Context, id string, jpeg []byte) (time.Time, error)
	// GetAvatar returns the custom image, or ErrNotFound if there is none.
	GetAvatar(ctx context.Context, id string) ([]byte, time.Time, error)
	// AvatarUpdatedAt returns when the custom image was last set without
	// reading its bytes, or ErrNotFound if there is none.
	AvatarUpdatedAt(ctx context.Context, id string) (time.Time, error)
}

// The users table is defined in backend/sql/*.sql. Profile reads select
// "avatar_data IS NOT NULL" so the image bytes are never loaded with a user.
// PGStore implements Store (and wallet.Store) on PostgreSQL. All SQL lives in
// sql/queries and is compiled to type-safe Go by sqlc (internal/db/dbgen).
type PGStore struct{ q *dbgen.Queries }

func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{q: dbgen.New(pool)} }

// ErrInsufficientTokens is returned by AdjustTokenBalance when the balance
// would go below zero. The wallet package re-exports the same value.
var ErrInsufficientTokens = errors.New("wallet: insufficient tokens")

func fromRow(r dbgen.User) *User {
	u := &User{
		ID: r.ID, Username: r.Username, Email: r.Email, PasswordHash: r.PasswordHash,
		HasAvatar: r.AvatarUpdatedAt != nil, AvatarUpdatedAt: r.AvatarUpdatedAt,
		Trophies: int(r.Trophies), CreatedAt: r.CreatedAt,
	}
	if r.AvatarDefault != nil {
		n := int(*r.AvatarDefault)
		u.AvatarDefault = &n
	}
	return u
}

// notFound maps "no rows" to ErrNotFound and leaves other errors alone.
func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (s *PGStore) Create(ctx context.Context, username, email, passwordHash string, tokens int) (*User, error) {
	row, err := s.q.CreateUser(ctx, dbgen.CreateUserParams{
		Username: username, Email: email, PasswordHash: passwordHash, TokenBalance: int64(tokens),
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
		if pgErr.ConstraintName == "users_username_key" {
			return nil, ErrUsernameTaken
		}
		return nil, ErrEmailTaken
	}
	if err != nil {
		return nil, err
	}
	return fromRow(row), nil
}

func (s *PGStore) Availability(ctx context.Context, email, username string) (bool, bool, error) {
	row, err := s.q.CheckAvailability(ctx, dbgen.CheckAvailabilityParams{Email: email, Username: username})
	return row.EmailTaken, row.UsernameTaken, err
}

func (s *PGStore) GetByEmail(ctx context.Context, email string) (*User, error) {
	row, err := s.q.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, notFound(err)
	}
	return fromRow(row), nil
}

func (s *PGStore) GetByID(ctx context.Context, id string) (*User, error) {
	row, err := s.q.GetUserByID(ctx, id)
	if err != nil {
		return nil, notFound(err)
	}
	return fromRow(row), nil
}

func (s *PGStore) SetAvatarDefault(ctx context.Context, id string, n int) error {
	n16 := int16(n)
	affected, err := s.q.SetDefaultAvatar(ctx, dbgen.SetDefaultAvatarParams{ID: id, AvatarDefault: &n16})
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PGStore) SetAvatarCustom(ctx context.Context, id string, jpeg []byte) (time.Time, error) {
	at, err := s.q.SetCustomAvatar(ctx, dbgen.SetCustomAvatarParams{ID: id, Data: jpeg})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.ForeignKeyViolation {
		return time.Time{}, ErrNotFound // no such user
	}
	if err != nil {
		return time.Time{}, notFound(err)
	}
	return at, nil
}

func (s *PGStore) GetAvatar(ctx context.Context, id string) ([]byte, time.Time, error) {
	row, err := s.q.GetAvatar(ctx, id)
	if err != nil {
		return nil, time.Time{}, notFound(err)
	}
	return row.Data, row.UpdatedAt, nil
}

func (s *PGStore) AvatarUpdatedAt(ctx context.Context, id string) (time.Time, error) {
	at, err := s.q.GetAvatarUpdatedAt(ctx, id)
	if err != nil {
		return time.Time{}, notFound(err)
	}
	return at, nil
}

func (s *PGStore) GetTokenBalance(ctx context.Context, id string) (int64, error) {
	n, err := s.q.GetTokenBalance(ctx, id)
	if err != nil {
		return 0, notFound(err)
	}
	return n, nil
}

// AdjustTokenBalance atomically adds delta and returns the new balance. The
// query refuses to go below zero; we then tell "unknown user" from
// "insufficient tokens" with a second read.
func (s *PGStore) AdjustTokenBalance(ctx context.Context, id string, delta int64) (int64, error) {
	n, err := s.q.AdjustTokenBalance(ctx, dbgen.AdjustTokenBalanceParams{ID: id, Delta: delta})
	if errors.Is(err, pgx.ErrNoRows) {
		if _, gErr := s.GetTokenBalance(ctx, id); gErr != nil {
			return 0, gErr
		}
		return 0, ErrInsufficientTokens
	}
	return n, err
}
