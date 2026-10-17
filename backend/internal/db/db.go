package db

import (
	"context"
	"fmt"

	"backend/internal/config"
	"backend/pkg/utils"
	"github.com/jackc/pgx/v5/pgxpool"
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
	client := redis.NewClient(opts)
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis: ping: %w", err)
	}
	utils.LogInfo("[DB] Redis connected successfully", "pool_size", cfg.RedisPoolSize)
	return client, nil
}
