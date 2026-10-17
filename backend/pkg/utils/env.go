package utils

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// LoadEnv loads the first .env file that exists, logs which required
// variables are set, and returns an error if any of them is missing.
func LoadEnv(required []string, files ...string) error {
	loaded := false
	for _, f := range files {
		if err := godotenv.Load(f); err == nil {
			LogInfo("[ENV] loaded file", "file", f)
			loaded = true
			break
		}
	}
	if !loaded {
		LogError("no .env file found, falling back to process environment", "tried", files)
	}

	for _, key := range required {
		if os.Getenv(key) == "" {
			return fmt.Errorf("missing required environment variable: %s", key)
		}
		LogInfo("[ENV] variable loaded", "key", key)
	}
	return nil
}
