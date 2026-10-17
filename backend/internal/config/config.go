package config

import (
	"os"
	"strconv"
	"time"

	"backend/pkg/utils"
)

type Config struct {
	PostgresURI string
	MongoURI    string
	RedisURI    string

	// Connection pool tuning (all optional, see defaults below).
	PGMaxConns     int32
	PGMinConns     int32
	MongoMaxPool   uint64
	RedisPoolSize  int
	ConnectTimeout time.Duration
}

var requiredVars = []string{"POSTGRES_URI", "MONGO_URI", "REDIS_URI"}

// Load reads the .env file (repo root, or backend/ cwd) and returns the config.
func Load() (*Config, error) {
	if err := utils.LoadEnv(requiredVars, ".env", "../.env"); err != nil {
		return nil, err
	}
	return &Config{
		PostgresURI:    os.Getenv("POSTGRES_URI"),
		MongoURI:       os.Getenv("MONGO_URI"),
		RedisURI:       os.Getenv("REDIS_URI"),
		PGMaxConns:     int32(envInt("PG_MAX_CONNS", 10)),
		PGMinConns:     int32(envInt("PG_MIN_CONNS", 2)),
		MongoMaxPool:   uint64(envInt("MONGO_MAX_POOL_SIZE", 50)),
		RedisPoolSize:  envInt("REDIS_POOL_SIZE", 10),
		ConnectTimeout: time.Duration(envInt("DB_CONNECT_TIMEOUT_SECONDS", 5)) * time.Second,
	}, nil
}

// envInt returns the positive integer in key, or def if unset or invalid.
func envInt(key string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil && v > 0 {
		return v
	}
	return def
}
