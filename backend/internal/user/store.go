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
	ErrIdentityTaken = errors.New("user: sign-in account already linked")
)

type User struct {
	ID            string
	Username      string
	Email         string
	PasswordHash  string
	AvatarDefault *int    // 1..4 when a default avatar is chosen
	AvatarURL     *string // public URL of the custom image in object storage
	Trophies      int
	Role          string // RoleUser or RoleAdmin
	CreatedAt     time.Time
}

// Roles (users_role_valid). Admins are made by hand in SQL; there is no API for it.
const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

// Store is the persistence the auth and profile handlers need.
type Store interface {
	// Create inserts a user with the given starting token balance.
	Create(ctx context.Context, username, email, passwordHash string, tokens int) (*User, error)
	// Availability reports whether the email / username are already registered.
	Availability(ctx context.Context, email, username string) (emailTaken, usernameTaken bool, err error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	GetByID(ctx context.Context, id string) (*User, error)
	// SetAvatarDefault picks default avatar n. It returns the object path of the
	// custom image it replaced ("" if none) so the caller can delete it.
	SetAvatarDefault(ctx context.Context, id string, n int) (oldPath string, err error)
	// SetAvatarCustom points the user at an uploaded image (public URL and
	// object path). It returns the object path of the image it replaced ("" if none).
	SetAvatarCustom(ctx context.Context, id, url, path string) (oldPath string, err error)
}

// PGStore implements Store (and wallet.Store) on PostgreSQL. All SQL lives in
// sql/queries and is compiled to type-safe Go by sqlc (internal/db/dbgen).
type PGStore struct{ q *dbgen.Queries }

func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{q: dbgen.New(pool)} }

// MaxTokenBalance is the most question tokens a player can hold. Every grant,
// refund and reset stops here; the database also refuses more (migration 00013).
const MaxTokenBalance = 10

// ErrInsufficientTokens is returned by AdjustTokenBalance when the balance
// would go below zero. The wallet package re-exports the same value.
var ErrInsufficientTokens = errors.New("wallet: insufficient tokens")

func fromRow(r dbgen.User) *User {
	u := &User{
		ID: r.ID, Username: r.Username, Email: r.Email, PasswordHash: r.PasswordHash,
		AvatarURL: r.AvatarUrl, Trophies: int(r.Trophies), Role: r.Role, CreatedAt: r.CreatedAt,
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

func (s *PGStore) SetAvatarDefault(ctx context.Context, id string, n int) (string, error) {
	n16 := int16(n)
	old, err := s.q.SetDefaultAvatar(ctx, dbgen.SetDefaultAvatarParams{ID: id, AvatarDefault: &n16})
	if err != nil {
		return "", notFound(err)
	}
	return deref(old), nil
}

func (s *PGStore) SetAvatarCustom(ctx context.Context, id, url, path string) (string, error) {
	old, err := s.q.SetCustomAvatar(ctx, dbgen.SetCustomAvatarParams{ID: id, AvatarUrl: url, AvatarPath: path})
	if err != nil {
		return "", notFound(err)
	}
	return deref(old), nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (s *PGStore) GetTokenBalance(ctx context.Context, id string) (int64, error) {
	n, err := s.q.GetTokenBalance(ctx, id)
	if err != nil {
		return 0, notFound(err)
	}
	return n, nil
}

// ResetAllTokenBalances sets every user's balance to tokens (at most
// MaxTokenBalance) and returns how many rows changed (the daily reset).
func (s *PGStore) ResetAllTokenBalances(ctx context.Context, tokens int64) (int64, error) {
	return s.q.ResetAllTokenBalances(ctx, min(tokens, MaxTokenBalance))
}

// AdjustTokenBalance atomically adds delta and returns the new balance, which
// never exceeds MaxTokenBalance (extra tokens are simply lost). The query refuses
// to go below zero; we then tell "unknown user" from "insufficient tokens" with a
// second read.
func (s *PGStore) AdjustTokenBalance(ctx context.Context, id string, delta int64) (int64, error) {
	n, err := s.q.AdjustTokenBalance(ctx, dbgen.AdjustTokenBalanceParams{ID: id, Delta: delta, Cap: MaxTokenBalance})
	if errors.Is(err, pgx.ErrNoRows) {
		if _, gErr := s.GetTokenBalance(ctx, id); gErr != nil {
			return 0, gErr
		}
		return 0, ErrInsufficientTokens
	}
	return n, err
}
