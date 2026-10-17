package config

import (
	"os"

	"backend/pkg/utils"
)

type Config struct {
	PostgresURI string
	MongoURI    string
	RedisURI    string
}

var requiredVars = []string{"POSTGRES_URI", "MONGO_URI", "REDIS_URI"}

// Load reads the .env file (repo root, or backend/ cwd) and returns the config.
func Load() *Config {
	utils.LoadEnv(requiredVars, ".env", "../.env")
	return &Config{
		PostgresURI: os.Getenv("POSTGRES_URI"),
		MongoURI:    os.Getenv("MONGO_URI"),
		RedisURI:    os.Getenv("REDIS_URI"),
	}
}
