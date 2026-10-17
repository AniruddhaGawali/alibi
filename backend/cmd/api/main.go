package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"backend/internal/auth"
	"backend/internal/config"
	"backend/internal/db"
	"backend/internal/email"
	"backend/internal/health"
	"backend/internal/llm"
	"backend/internal/middleware"
	"backend/internal/queue"
	"backend/internal/user"
	"backend/internal/wallet"
	"backend/pkg/logger"
	"backend/pkg/utils"
)

const (
	shutdownTimeout = 10 * time.Second
	initialTokens   = 10 // question tokens every new account starts with

	avatarUploadsPerHour = 5 // custom avatar uploads allowed per user per hour
)

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

	shutdownTelemetry, err := logger.Init(context.Background(), "alibi-api", cfg.OTLPEndpoint)
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

	if cfg.MigrateOnStart {
		migrateCtx, cancelMigrate := context.WithTimeout(context.Background(), time.Minute)
		err = db.Migrate(migrateCtx, pg)
		cancelMigrate()
		if err != nil {
			return err
		}
	}
	userStore := user.NewPGStore(pg)

	mongoClient, err := db.ConnectMongo(cfg)
	if err != nil {
		return err
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := mongoClient.Disconnect(ctx); err != nil {
			utils.LogError("mongo disconnect failed", "err", err)
		}
	}()

	rdb, err := db.ConnectRedis(cfg)
	if err != nil {
		return err
	}
	defer func() {
		if err := rdb.Close(); err != nil {
			utils.LogError("redis close failed", "err", err)
		}
	}()

	llmClients, err := llm.NewClients(cfg.OllamaHost, cfg.OpenRouterAPIKey)
	if err != nil {
		return err
	}
	llmClients.PingOllama(context.Background())

	checker := health.New(map[string]health.Check{
		"postgres": pg.Ping,
		"mongodb":  func(ctx context.Context) error { return mongoClient.Ping(ctx, nil) },
		"redis":    func(ctx context.Context) error { return rdb.Ping(ctx).Err() },
	})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", checker.Handler)

	tokens := auth.NewTokenManager(cfg.JWTSecret, cfg.AccessTTL, cfg.RefreshTTL)
	blacklist := auth.NewBlacklist(rdb)
	walletSvc := wallet.New(userStore, rdb)
	authH := auth.NewHandler(auth.Deps{
		Users:     userStore,
		Tokens:    tokens,
		Blacklist: blacklist,
		OTP:       auth.NewOTPStore(rdb, cfg.OTPTTL, cfg.OTPMaxAttempts, []byte(cfg.JWTSecret)),
		Limiter: auth.NewLimiter(rdb, auth.Policy{
			Cooldown: cfg.OTPCooldown, EmailDailyMax: cfg.OTPEmailDailyMax,
			IPMax: cfg.OTPIPMax, IPWindow: cfg.OTPIPWindow,
			AuthIPMax: cfg.AuthIPMax, AuthIPWindow: cfg.AuthIPWindow,
			LoginMaxFailures: cfg.LoginMaxFailures,
		}),
		Mail:           queue.NewRedisListBroker(rdb, email.DLQ, queue.WithDLQMaxLen(email.DLQMaxLen)), // the email worker consumes it
		Coins:          walletSvc,
		TrustedProxies: cfg.TrustedProxies,
		InitialTokens:  initialTokens,
		HashWorkers:    cfg.HashWorkers,
	})
	userH := user.NewHandler(userStore, walletSvc, auth.UserID,
		user.WithUploadLimiter(user.NewRedisUploadLimiter(rdb, avatarUploadsPerHour, time.Hour)))

	mux.HandleFunc("POST /api/auth/signup", authH.Signup)
	mux.HandleFunc("POST /api/auth/resend-otp", authH.ResendOTP)
	mux.HandleFunc("POST /api/auth/verify-otp", authH.VerifyOTP)
	mux.HandleFunc("POST /api/auth/login", authH.Login)
	mux.HandleFunc("POST /api/auth/refresh", authH.Refresh)
	mux.HandleFunc("POST /api/auth/logout", authH.Logout)
	mux.HandleFunc("GET /api/user/profile", userH.Profile)
	mux.HandleFunc("POST /api/user/avatar", userH.SetAvatar)
	mux.HandleFunc("GET /api/user/avatars/{id}", userH.Avatar)

	// Deny by default: every route needs a valid JWT unless it is listed here.
	public := auth.NewPublicRoutes(
		"GET /health",
		"POST /api/auth/signup",
		"POST /api/auth/resend-otp",
		"POST /api/auth/verify-otp",
		"POST /api/auth/login",
		"POST /api/auth/refresh",
		"GET /api/user/avatars/", // <img> tags cannot send an Authorization header
	)
	protected := auth.Middleware(tokens, blacklist, public)(mux)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           middleware.Logging(protected),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      cfg.WriteTimeout,
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
