package db

import (
	"context"
	"time"

	"backend/pkg/utils"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const pingTimeout = 5 * time.Second

func ConnectPostgres(uri string) *pgxpool.Pool {
	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()

	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		utils.LogError("[DB] PostgreSQL connection failed", "err", err)
		panic(err)
	}
	if err := pool.Ping(ctx); err != nil {
		utils.LogError("[DB] PostgreSQL ping failed", "err", err)
		panic(err)
	}
	utils.LogInfo("[DB] PostgreSQL connected successfully")
	return pool
}

func ConnectMongo(uri string) *mongo.Client {
	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()

	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		utils.LogError("[DB] MongoDB connection failed", "err", err)
		panic(err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		utils.LogError("[DB] MongoDB ping failed", "err", err)
		panic(err)
	}
	utils.LogInfo("[DB] MongoDB connected successfully")
	return client
}

func ConnectRedis(uri string) *redis.Client {
	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()

	opts, err := redis.ParseURL(uri)
	if err != nil {
		utils.LogError("[DB] Redis URI invalid", "err", err)
		panic(err)
	}
	client := redis.NewClient(opts)
	if err := client.Ping(ctx).Err(); err != nil {
		utils.LogError("[DB] Redis ping failed", "err", err)
		panic(err)
	}
	utils.LogInfo("[DB] Redis connected successfully")
	return client
}
