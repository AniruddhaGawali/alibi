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
//	docker run --rm -d -p 55432:5432 -e POSTGRES_PASSWORD=pw postgres:16
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
	if err != nil || u.HasAvatar || u.AvatarDefault != nil {
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

	if _, _, err := s.GetAvatar(ctx, u.ID); !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("no avatar yet: %v", err)
	}
	if _, err := s.SetAvatarCustom(ctx, u.ID, []byte{0xFF, 0xD8, 1, 2, 3}); err != nil {
		t.Fatal(err)
	}
	img, at, err := s.GetAvatar(ctx, u.ID)
	if err != nil || len(img) != 5 || at.IsZero() {
		t.Fatalf("get avatar: %v %v", img, err)
	}
	if at2, err := s.AvatarUpdatedAt(ctx, u.ID); err != nil || !at2.Equal(at) {
		t.Fatalf("AvatarUpdatedAt = %v %v, want %v", at2, err, at)
	}
	if _, err := s.SetAvatarCustom(ctx, u.ID, make([]byte, 1<<20+1)); err == nil {
		t.Fatal("an image over 1 MB must violate user_avatars_size")
	}
	if _, err := s.SetAvatarCustom(ctx, u.ID, []byte{9, 9}); err != nil { // replace, not duplicate
		t.Fatal(err)
	}
	if img, _, _ := s.GetAvatar(ctx, u.ID); len(img) != 2 {
		t.Fatalf("avatar not replaced: %v", img)
	}
	if _, err := s.SetAvatarCustom(ctx, "00000000-0000-0000-0000-000000000000", []byte{1}); !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("custom avatar for unknown user: %v", err)
	}
	got, _ := s.GetByID(ctx, u.ID)
	if !got.HasAvatar || got.AvatarDefault != nil || got.AvatarUpdatedAt == nil {
		t.Fatalf("after custom: %+v", got)
	}

	if err := s.SetAvatarDefault(ctx, u.ID, 3); err != nil {
		t.Fatal(err)
	}
	got, _ = s.GetByID(ctx, u.ID)
	if got.HasAvatar || got.AvatarDefault == nil || *got.AvatarDefault != 3 {
		t.Fatalf("after default: %+v", got)
	}
	if _, _, err := s.GetAvatar(ctx, u.ID); !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("custom image should be deleted when a default is chosen: %v", err)
	}
	if err := s.SetAvatarDefault(ctx, u.ID, 9); err == nil {
		t.Fatal("avatar 9 should violate the CHECK constraint")
	}
	if err := s.SetAvatarDefault(ctx, "00000000-0000-0000-0000-000000000000", 1); !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("unknown user: %v", err)
	}

	// Token balance: atomic, never negative.
	if n, err := s.AdjustTokenBalance(ctx, u.ID, 10); err != nil || n != 20 {
		t.Fatalf("grant: %d %v", n, err)
	}
	if n, err := s.AdjustTokenBalance(ctx, u.ID, -4); err != nil || n != 16 {
		t.Fatalf("spend: %d %v", n, err)
	}
	if _, err := s.AdjustTokenBalance(ctx, u.ID, -17); !errors.Is(err, user.ErrInsufficientTokens) {
		t.Fatalf("overdraw: %v", err)
	}
	if n, _ := s.GetTokenBalance(ctx, u.ID); n != 16 {
		t.Fatalf("balance after rejected overdraw = %d", n)
	}
	// The balance is 64-bit: repeated large grants must not overflow.
	for i := 0; i < 3; i++ {
		if _, err := s.AdjustTokenBalance(ctx, u.ID, 2_000_000_000); err != nil {
			t.Fatalf("large grant %d: %v", i, err)
		}
	}
	if n, _ := s.GetTokenBalance(ctx, u.ID); n != 6_000_000_016 {
		t.Fatalf("balance = %d", n)
	}
	if _, err := s.AdjustTokenBalance(ctx, "00000000-0000-0000-0000-000000000000", 1); !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("unknown user: %v", err)
	}
}
