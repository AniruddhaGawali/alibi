package logger

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

// Regression test: Init must not fail on a resource schema URL conflict.
func TestInitSucceeds(t *testing.T) {
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })

	shutdown, err := Init(context.Background(), "alibi-test", "localhost:4318")
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_ = shutdown(ctx) // nothing listens on :4318; only Init's success matters here
}
