// Package testdb gives integration tests a private, empty PostgreSQL database.
package testdb

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"testing"

	"backend/internal/config"
	"backend/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// New creates an empty database named alibi_test_<random> on the
// server TEST_POSTGRES_URI points at and returns a pool for it. The test never
// touches any existing database; the new one is dropped when the test ends.
func New(t *testing.T, adminURI string) *pgxpool.Pool {
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
