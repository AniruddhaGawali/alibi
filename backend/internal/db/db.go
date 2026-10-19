package db

import (
	"context"
	"fmt"
	"time"

	"backend/internal/config"
	"backend/pkg/utils"
	sqlfiles "backend/sql"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func ConnectPostgres(cfg *config.Config) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout)
	defer cancel()

	pcfg, err := pgxpool.ParseConfig(cfg.PostgresURI)
	if err != nil {
		return nil, fmt.Errorf("postgres: invalid POSTGRES_URI: %w", err)
	}
	pcfg.MaxConns = cfg.PGMaxConns
	pcfg.MinConns = min(cfg.PGMinConns, cfg.PGMaxConns)

	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		return nil, fmt.Errorf("postgres: connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: ping: %w", err)
	}
	utils.LogInfo("[DB] PostgreSQL connected successfully", "max_conns", pcfg.MaxConns)
	return pool, nil
}

func ConnectMongo(cfg *config.Config) (*mongo.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout)
	defer cancel()

	client, err := mongo.Connect(options.Client().
		ApplyURI(cfg.MongoURI).
		SetMaxPoolSize(cfg.MongoMaxPool).
		SetServerSelectionTimeout(cfg.ConnectTimeout))
	if err != nil {
		return nil, fmt.Errorf("mongodb: connect: %w", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("mongodb: ping: %w", err)
	}
	utils.LogInfo("[DB] MongoDB connected successfully", "max_pool", cfg.MongoMaxPool)
	return client, nil
}

func ConnectRedis(cfg *config.Config) (*redis.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout)
	defer cancel()

	opts, err := redis.ParseURL(cfg.RedisURI)
	if err != nil {
		return nil, fmt.Errorf("redis: invalid REDIS_URI: %w", err)
	}
	opts.PoolSize = cfg.RedisPoolSize
	// Short timeouts and one retry: with Redis down, every call would otherwise wait for the
	// library's defaults (5 s dial, 3 retries with backoff, 5 dial attempts) and a single request makes many.
	opts.DialTimeout = 300 * time.Millisecond
	opts.ReadTimeout = 2 * time.Second
	opts.WriteTimeout = 2 * time.Second
	opts.MaxRetries = 1
	opts.DialerRetries = 1 // default 5 attempts: ~0.8 s per failing call while Redis is down
	opts.DialerRetryTimeout = 20 * time.Millisecond
	opts.MinRetryBackoff = 5 * time.Millisecond
	opts.MaxRetryBackoff = 25 * time.Millisecond
	client := redis.NewClient(opts)
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis: ping: %w", err)
	}
	utils.LogInfo("[DB] Redis connected successfully", "pool_size", cfg.RedisPoolSize)
	return client, nil
}

// Migrate applies the pending goose migrations embedded from sql/migrations.
// The same files are used by the goose CLI (see the Makefile). A Postgres
// advisory lock serialises concurrent runs, so several API replicas starting at
// once (rolling deploy, --scale api=N) apply each migration exactly once; the
// others wait and then find nothing pending.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	migrations, err := sqlfiles.Migrations()
	if err != nil {
		return fmt.Errorf("postgres: migrations: %w", err)
	}
	sqlDB := stdlib.OpenDBFromPool(pool) // shares the pool; do not close it
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return fmt.Errorf("postgres: migration lock: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations, goose.WithSessionLocker(locker))
	if err != nil {
		return fmt.Errorf("postgres: migrations: %w", err)
	}
	results, err := provider.Up(ctx)
	for _, r := range results {
		utils.LogInfo("[DB] migration applied", "version", r.Source.Version, "file", r.Source.Path, "took", r.Duration)
	}
	if err != nil {
		return fmt.Errorf("postgres: migrate: %w", err)
	}
	return nil
}
