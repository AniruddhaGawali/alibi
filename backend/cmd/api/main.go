package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"backend/internal/config"
	"backend/internal/db"
	"backend/internal/health"
	"backend/internal/llm"
	"backend/internal/middleware"
	"backend/internal/queue"
	"backend/pkg/logger"
	"backend/pkg/utils"
)

const shutdownTimeout = 10 * time.Second

func main() {
	if err := run(); err != nil {
		utils.LogError("fatal: server stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "localhost:4318"
	}
	shutdownTelemetry, err := logger.Init(context.Background(), "alibi-api", endpoint)
	if err != nil {
		return err
	}
	// Deferred first so it runs last: shutdown logs still get exported.
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownTelemetry(ctx)
	}()

	pg, err := db.ConnectPostgres(cfg)
	if err != nil {
		return err
	}
	defer pg.Close()

	mongoClient, err := db.ConnectMongo(cfg)
	if err != nil {
		return err
	}
	defer mongoClient.Disconnect(context.Background())

	rdb, err := db.ConnectRedis(cfg)
	if err != nil {
		return err
	}
	defer rdb.Close()

	ollamaHost := os.Getenv("OLLAMA_HOST")
	if ollamaHost == "" {
		ollamaHost = "http://localhost:11434"
	}
	llmClients, err := llm.NewClients(ollamaHost, os.Getenv("OPENROUTER_API_KEY"))
	if err != nil {
		return err
	}
	llmClients.PingOllama(context.Background())

	_ = queue.NewRedisListBroker(rdb, "dlq")

	checker := health.New(map[string]health.Check{
		"postgres": pg.Ping,
		"mongodb":  func(ctx context.Context) error { return mongoClient.Ping(ctx, nil) },
		"redis":    func(ctx context.Context) error { return rdb.Ping(ctx).Err() },
	})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", checker.Handler)

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           middleware.Logging(mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)
	go func() {
		utils.LogInfo("all services connected; listening", "addr", srv.Addr)
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
		utils.LogInfo("shutdown signal received; draining requests")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	utils.LogInfo("server stopped gracefully")
	return nil
}
