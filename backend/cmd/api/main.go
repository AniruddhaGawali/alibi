package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"backend/internal/config"
	"backend/internal/db"
	"backend/internal/llm"
	"backend/internal/middleware"
	"backend/internal/queue"
	"backend/pkg/logger"
	"backend/pkg/utils"
)

func main() {
	cfg := config.Load()

	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "localhost:4318"
	}
	shutdownTelemetry, err := logger.Init(context.Background(), "alibi-api", endpoint)
	if err != nil {
		utils.LogError("telemetry init failed", "err", err)
		panic(err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		shutdownTelemetry(ctx)
	}()

	pg := db.ConnectPostgres(cfg.PostgresURI)
	defer pg.Close()

	mongoClient := db.ConnectMongo(cfg.MongoURI)
	defer mongoClient.Disconnect(context.Background())

	rdb := db.ConnectRedis(cfg.RedisURI)
	defer rdb.Close()

	ollamaHost := os.Getenv("OLLAMA_HOST")
	if ollamaHost == "" {
		ollamaHost = "http://localhost:11434"
	}
	llmClients, err := llm.NewClients(ollamaHost, os.Getenv("OPENROUTER_API_KEY"))
	if err != nil {
		utils.LogError("llm init failed", "err", err)
		panic(err)
	}
	llmClients.PingOllama(context.Background())

	_ = queue.NewRedisListBroker(rdb, "dlq")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		utils.SuccessResponse(w, http.StatusOK, map[string]string{"service": "alibi-api"})
	})

	utils.LogInfo("all services connected; listening on :8080")
	if err := http.ListenAndServe(":8080", middleware.Logging(mux)); err != nil {
		utils.LogError("server stopped", "err", err)
		panic(err)
	}
}
