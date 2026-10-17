package utils

import (
	"context"
	"log/slog"
)

// These helpers log through slog's default logger, which logger.Init replaces
// with the OpenTelemetry-exporting handler. Before Init they fall back to
// slog's standard stderr output.

func LogInfo(msg string, args ...any)  { slog.Info(msg, args...) }
func LogError(msg string, args ...any) { slog.Error(msg, args...) }
func LogWarn(msg string, args ...any)  { slog.Warn(msg, args...) }

// LogInfoCtx / LogErrorCtx carry the request context so the record is linked
// to the active trace span. Prefer them inside HTTP handlers.
func LogInfoCtx(ctx context.Context, msg string, args ...any) { slog.InfoContext(ctx, msg, args...) }
func LogErrorCtx(ctx context.Context, msg string, args ...any) {
	slog.ErrorContext(ctx, msg, args...)
}

// LogWarnCtx is the context-aware LogWarn.
func LogWarnCtx(ctx context.Context, msg string, args ...any) { slog.WarnContext(ctx, msg, args...) }
