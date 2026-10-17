package utils

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/joho/godotenv"
)

// LoadEnv loads the first .env file that exists, logs which required
// variables are set, and returns an error if any of them is missing.
func LoadEnv(required []string, files ...string) error {
	loaded := false
	for _, f := range files {
		err := godotenv.Load(f)
		if err == nil {
			LogInfo("[ENV] loaded file", "file", f)
			loaded = true
			break
		}
		// A missing file is normal; one that exists but cannot be read or
		// parsed is a misconfiguration worth surfacing.
		if !errors.Is(err, fs.ErrNotExist) {
			LogError("[ENV] cannot load .env file", "file", f, "err", err)
		}
	}
	if !loaded {
		LogInfo("[ENV] no .env file loaded, using process environment", "tried", files)
	}

	for _, key := range required {
		if os.Getenv(key) == "" {
			return fmt.Errorf("missing required environment variable: %s", key)
		}
		LogInfo("[ENV] variable loaded", "key", key)
	}
	return nil
}
