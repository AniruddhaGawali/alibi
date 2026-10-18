package user_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"sync"
	"testing"

	"backend/internal/config"
	"backend/internal/db"
	"backend/internal/user"
	"github.com/jackc/pgx/v5/pgxpool"
)

// throwawayDatabase creates an empty database named alibi_test_<random> on the
// server TEST_POSTGRES_URI points at and returns a pool for it. The test never
// touches any existing database; the new one is dropped when the test ends.
func throwawayDatabase(t *testing.T, adminURI string) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	var rnd [6]byte
	_, _ = rand.Read(rnd[:])
	name := "alibi_test_" + hex.EncodeToString(rnd[:])

	admin, err := db.ConnectPostgres(&config.Config{PostgresURI: adminURI, PGMaxConns: 2, ConnectTimeout: 5e9})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
		admin.Close()
	})

	u, err := url.Parse(adminURI)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	pool, err := db.ConnectPostgres(&config.Config{PostgresURI: u.String(), PGMaxConns: 8, ConnectTimeout: 5e9})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close) // runs before the DROP above (cleanups are LIFO)
	return pool
}

// Runs against a real PostgreSQL when TEST_POSTGRES_URI is set. It only needs
// permission to CREATE DATABASE and works in a throwaway database of its own, so
// any server is safe to point it at:
//
//	docker run --rm -d -p 55432:5432 -e POSTGRES_PASSWORD=pw postgres:18
//	TEST_POSTGRES_URI=postgres://postgres:pw@localhost:55432/postgres go test ./internal/user
func TestPGStore(t *testing.T) {
	uri := os.Getenv("TEST_POSTGRES_URI")
	if uri == "" {
		t.Skip("TEST_POSTGRES_URI not set")
	}
	ctx := context.Background()
	pool := throwawayDatabase(t, uri)

	// Several replicas starting at once must apply each migration exactly once.
	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- db.Migrate(ctx, pool) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent migrate: %v", err)
		}
	}
	if err := db.Migrate(ctx, pool); err != nil { // and re-running is a no-op
		t.Fatalf("migrate again: %v", err)
	}

	s := user.NewPGStore(pool)
	u, err := s.Create(ctx, "sherlock", "s@x.co", "hash", 10)
	if err != nil || u.AvatarURL != nil || u.AvatarDefault != nil {
		t.Fatalf("create: %+v %v", u, err)
	}
	if n, _ := s.GetTokenBalance(ctx, u.ID); n != 10 {
		t.Fatalf("starting balance = %d, want 10", n)
	}
	if e, n, err := s.Availability(ctx, "S@X.co", "nobody"); err != nil || !e || n {
		t.Fatalf("availability: email=%v username=%v err=%v", e, n, err)
	}
	if e, n, err := s.Availability(ctx, "free@x.co", "SHERLOCK"); err != nil || e || !n {
		t.Fatalf("availability: email=%v username=%v err=%v", e, n, err)
	}
	if _, err := s.Create(ctx, "other", "S@X.CO", "h", 10); !errors.Is(err, user.ErrEmailTaken) {
		t.Fatalf("dup email: %v", err)
	}
	if _, err := s.Create(ctx, "SHERLOCK", "o@x.co", "h", 10); !errors.Is(err, user.ErrUsernameTaken) {
		t.Fatalf("dup username: %v", err)
	}
	if got, err := s.GetByEmail(ctx, "S@x.co"); err != nil || got.ID != u.ID {
		t.Fatalf("by email: %v", err)
	}

	// Custom avatars: the store keeps the public URL and object path, and hands
	// back the replaced path so the image can be deleted from storage.
	if old, err := s.SetAvatarCustom(ctx, u.ID, "https://cdn/a.webp", "avatars/a.webp"); err != nil || old != "" {
		t.Fatalf("first custom: old=%q err=%v", old, err)
	}
	if old, err := s.SetAvatarCustom(ctx, u.ID, "https://cdn/b.webp", "avatars/b.webp"); err != nil || old != "avatars/a.webp" {
		t.Fatalf("replace custom: old=%q err=%v", old, err)
	}
	got, _ := s.GetByID(ctx, u.ID)
	if got.AvatarURL == nil || *got.AvatarURL != "https://cdn/b.webp" || got.AvatarDefault != nil {
		t.Fatalf("after custom: %+v", got)
	}
	if _, err := s.SetAvatarCustom(ctx, "00000000-0000-0000-0000-000000000000", "u", "p.webp"); !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("custom avatar for unknown user: %v", err)
	}
	if old, err := s.SetAvatarDefault(ctx, u.ID, 3); err != nil || old != "avatars/b.webp" {
		t.Fatalf("default: old=%q err=%v", old, err)
	}
	got, _ = s.GetByID(ctx, u.ID)
	if got.AvatarURL != nil || got.AvatarDefault == nil || *got.AvatarDefault != 3 {
		t.Fatalf("after default: %+v", got)
	}
	if old, err := s.SetAvatarDefault(ctx, u.ID, 2); err != nil || old != "" {
		t.Fatalf("default again: old=%q err=%v", old, err)
	}
	if _, err := s.SetAvatarDefault(ctx, u.ID, 9); err == nil {
		t.Fatal("avatar 9 should violate the CHECK constraint")
	}
	if _, err := s.SetAvatarDefault(ctx, "00000000-0000-0000-0000-000000000000", 1); !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("unknown user: %v", err)
	}

	// Token balance: atomic, never negative, never above 10.
	if n, err := s.AdjustTokenBalance(ctx, u.ID, -4); err != nil || n != 6 {
		t.Fatalf("spend: %d %v", n, err)
	}
	if n, err := s.AdjustTokenBalance(ctx, u.ID, 2); err != nil || n != 8 {
		t.Fatalf("grant: %d %v", n, err)
	}
	if n, err := s.AdjustTokenBalance(ctx, u.ID, 25); err != nil || n != 10 {
		t.Fatalf("grant past the cap should stop at 10: %d %v", n, err)
	}
	if n, err := s.AdjustTokenBalance(ctx, u.ID, 1); err != nil || n != 10 {
		t.Fatalf("refund at the cap: %d %v", n, err)
	}
	if _, err := s.AdjustTokenBalance(ctx, u.ID, -11); !errors.Is(err, user.ErrInsufficientTokens) {
		t.Fatalf("overdraw: %v", err)
	}
	if n, _ := s.GetTokenBalance(ctx, u.ID); n != 10 {
		t.Fatalf("balance after rejected overdraw = %d", n)
	}
	// The database itself refuses more than 10, whatever the Go code does.
	if _, err := pool.Exec(ctx, "UPDATE users SET token_balance = 11 WHERE id = $1", u.ID); err == nil {
		t.Fatal("token_balance 11 should violate the CHECK constraint")
	}
	if _, err := s.AdjustTokenBalance(ctx, "00000000-0000-0000-0000-000000000000", 1); !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("unknown user: %v", err)
	}

	// Daily reset: every balance becomes exactly 10, whether it was low or empty.
	other, err := s.Create(ctx, "watson", "w@x.co", "hash", 3)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AdjustTokenBalance(ctx, u.ID, -10); err != nil { // u is now empty
		t.Fatal(err)
	}
	if n, err := s.ResetAllTokenBalances(ctx, 10); err != nil || n != 2 {
		t.Fatalf("reset: %d %v", n, err)
	}
	for _, id := range []string{u.ID, other.ID} {
		if n, _ := s.GetTokenBalance(ctx, id); n != 10 {
			t.Fatalf("balance after reset = %d", n)
		}
	}
	if n, err := s.ResetAllTokenBalances(ctx, 10); err != nil || n != 0 {
		t.Fatalf("second reset changed %d rows: %v", n, err)
	}
	// Asking for more than the cap still stops at 10.
	if _, err := s.AdjustTokenBalance(ctx, other.ID, -3); err != nil {
		t.Fatal(err)
	}
	if n, err := s.ResetAllTokenBalances(ctx, 99); err != nil || n != 1 {
		t.Fatalf("reset above the cap: %d %v", n, err)
	}
	if n, _ := s.GetTokenBalance(ctx, other.ID); n != 10 {
		t.Fatalf("balance after a reset to 99 = %d", n)
	}
}
