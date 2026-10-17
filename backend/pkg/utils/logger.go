package utils

import (
	"log/slog"
	"os"
)

var logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

func LogInfo(msg string, args ...any) {
	logger.Info(msg, args...)
}

func LogError(msg string, args ...any) {
	logger.Error(msg, args...)
}

// LogFatal logs at error level and exits the process with status 1.
func LogFatal(msg string, args ...any) {
	logger.Error(msg, args...)
	os.Exit(1)
}
