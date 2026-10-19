package user

import (
	"context"
	"errors"

	"backend/internal/db/dbgen"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
)

// NoPassword is the password_hash of an account created through a sign-in provider.
// It is not a valid bcrypt hash, so no password can ever match it.
const NoPassword = "!"

// ProviderGoogle is the provider name stored in user_identities.
const ProviderGoogle = "google"

// GetByIdentity returns the user linked to a provider account (ErrNotFound if none).
func (s *PGStore) GetByIdentity(ctx context.Context, provider, subject string) (*User, error) {
	row, err := s.q.GetUserByIdentity(ctx, dbgen.GetUserByIdentityParams{Provider: provider, Subject: subject})
	if err != nil {
		return nil, notFound(err)
	}
	return fromRow(row), nil
}

// LinkIdentity attaches a provider account to an existing user. It reports false when
// nothing was linked (the user already has one for this provider).
func (s *PGStore) LinkIdentity(ctx context.Context, userID, provider, subject, email string) (bool, error) {
	n, err := s.q.LinkIdentity(ctx, dbgen.LinkIdentityParams{Provider: provider, Subject: subject, UserID: userID, Email: email})
	return n > 0, err
}

// CreateWithIdentity creates a password-less user and its provider link in one statement.
func (s *PGStore) CreateWithIdentity(ctx context.Context, username, email, provider, subject string, tokens int) (*User, error) {
	row, err := s.q.CreateUserWithIdentity(ctx, dbgen.CreateUserWithIdentityParams{
		Username: username, Email: email, PasswordHash: NoPassword, TokenBalance: int64(tokens),
		Provider: provider, Subject: subject,
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
		switch pgErr.ConstraintName {
		case "users_username_key":
			return nil, ErrUsernameTaken
		case "users_email_key":
			return nil, ErrEmailTaken
		default:
			return nil, ErrIdentityTaken
		}
	}
	if err != nil {
		return nil, err
	}
	return fromRow(dbgen.User(row)), nil
}
