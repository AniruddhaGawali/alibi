// Command worker is the email worker: a separate process from the API that
// consumes the outbound email queue in Redis and sends mail through Brevo SMTP.
// It can be run in as many copies as needed; each message goes to one of them.
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
	"backend/internal/email"
	"backend/internal/queue"
	"backend/pkg/logger"
	"backend/pkg/utils"
	"github.com/redis/go-redis/v9"
)

func main() {
	if err := run(); err != nil {
		utils.LogError("fatal: email worker stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.LoadWorker()
	if err != nil {
		return err
	}

	shutdownTelemetry, err := logger.InitWith(context.Background(), "alibi-email-worker", logger.Options{Endpoint: cfg.OTLPEndpoint, Secure: cfg.OTLPSecure, Headers: cfg.OTLPHeaders})
	if err != nil {
		return err
	}
	// Deferred first so it runs last: shutdown logs still get exported.
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownTelemetry(ctx)
	}()

	rdb, err := db.ConnectRedis(cfg)
	if err != nil {
		return err
	}
	defer rdb.Close()

	mailer := utils.NewMailer(cfg.BrevoSMTPHost, cfg.BrevoSMTPPort, cfg.BrevoSMTPUser, cfg.BrevoSMTPPass, cfg.EmailFrom)
	broker := queue.NewRedisListBroker(rdb, email.DLQ, queue.WithDLQMaxLen(email.DLQMaxLen), queue.WithPopTimeout(cfg.WorkerPopTimeout))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := &http.Server{
		Addr:              cfg.WorkerHTTPAddr,
		Handler:           healthMux(rdb, broker),
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.ListenAndServe() }()
	go email.Monitor(ctx, broker, cfg.WorkerMonitorEvery)

	done := make(chan struct{})
	go func() {
		email.NewWorker(broker, mailer).Run(ctx, cfg.WorkerConcurrency)
		close(done)
	}()

	select {
	case err := <-serveErr:
		stop() // health server died: stop consuming too, let the supervisor restart us
		<-done
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-done: // signal received; consumers have drained in-flight messages
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// healthMux serves GET /health for orchestrators: 200 while Redis answers,
// with the queue depths so a dead-letter backlog is visible at a glance.
func healthMux(rdb *redis.Client, q email.Lengther) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := rdb.Ping(ctx).Err(); err != nil {
			utils.LogErrorCtx(ctx, "worker health: redis unreachable", "err", err)
			utils.ErrorResponse(w, http.StatusServiceUnavailable, "service degraded", map[string]string{"redis": "unavailable"})
			return
		}
		depths, err := email.Snapshot(ctx, q)
		if err != nil {
			utils.LogErrorCtx(ctx, "worker health: read queue depths", "err", err)
			utils.ErrorResponse(w, http.StatusServiceUnavailable, "service degraded", map[string]string{"redis": "unavailable"})
			return
		}
		utils.SuccessResponse(w, http.StatusOK, map[string]any{"redis": "ok", "queues": depths})
	})
	return mux
}
