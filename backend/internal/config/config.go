package config

import (
	"math"
	"net/url"
	"os"
	"strconv"
	"strings"
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

	// Server and integrations (all optional).
	HTTPAddr         string
	WriteTimeout     time.Duration // 0 disables the deadline (streaming / slow LLM calls)
	OTLPEndpoint     string        // host:port; a scheme or path in the env value is stripped
	OllamaHost       string
	OpenRouterAPIKey string
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
		PGMaxConns:     int32(envInt("PG_MAX_CONNS", 10, 1, math.MaxInt32)),
		PGMinConns:     int32(envInt("PG_MIN_CONNS", 2, 0, math.MaxInt32)),
		MongoMaxPool:   uint64(envInt("MONGO_MAX_POOL_SIZE", 50, 1, math.MaxInt32)),
		RedisPoolSize:  envInt("REDIS_POOL_SIZE", 10, 1, math.MaxInt32),
		ConnectTimeout: time.Duration(envInt("DB_CONNECT_TIMEOUT_SECONDS", 5, 1, 3600)) * time.Second,

		HTTPAddr:         envStr("HTTP_ADDR", ":8080"),
		WriteTimeout:     time.Duration(envInt("HTTP_WRITE_TIMEOUT_SECONDS", 120, 0, 86400)) * time.Second,
		OTLPEndpoint:     otlpHostPort(envStr("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4318")),
		OllamaHost:       envStr("OLLAMA_HOST", "http://localhost:11434"),
		OpenRouterAPIKey: os.Getenv("OPENROUTER_API_KEY"),
	}, nil
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// otlpHostPort reduces an OTLP endpoint such as "http://collector:4318/" to
// "collector:4318", the form the exporters' WithEndpoint expects.
func otlpHostPort(endpoint string) string {
	if strings.Contains(endpoint, "://") {
		if u, err := url.Parse(endpoint); err == nil && u.Host != "" {
			return u.Host
		}
	}
	return strings.TrimRight(endpoint, "/")
}

// envInt returns the integer in key if it lies within [min, max]. If key is
// unset it returns def; if it is set but invalid or out of range it logs a
// warning and returns def.
func envInt(key string, def, min, max int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < min || v > max {
		utils.LogWarn("[ENV] invalid value, using default", "key", key, "value", raw, "default", def)
		return def
	}
	return v
}
