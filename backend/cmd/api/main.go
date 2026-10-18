package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"backend/internal/admin"
	"backend/internal/auth"
	"backend/internal/cases"
	"backend/internal/chat"
	"backend/internal/config"
	"backend/internal/dashboard"
	"backend/internal/db"
	"backend/internal/email"
	"backend/internal/health"
	"backend/internal/interrogation"
	"backend/internal/jobs"
	"backend/internal/llm"
	"backend/internal/middleware"
	"backend/internal/queue"
	"backend/internal/storage"
	"backend/internal/user"
	"backend/internal/wallet"
	"backend/pkg/logger"
	"backend/pkg/utils"
)

const (
	shutdownTimeout = 10 * time.Second

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

	llmClients, err := llm.NewClients(cfg.OllamaHost, cfg.OpenRouterAPIKey, cfg.LLMTimeout)
	if err != nil {
		return err
	}
	llmClients.PingOllama(context.Background())

	checks := map[string]health.Check{
		"postgres": pg.Ping,
		"mongodb":  func(ctx context.Context) error { return mongoClient.Ping(ctx, nil) },
		"redis":    func(ctx context.Context) error { return rdb.Ping(ctx).Err() },
	}
	// Image storage behind storage.Provider: swap NewSupabaseProvider for an S3
	// or GCS implementation without touching the handlers.
	var images storage.Provider
	if cfg.SupabaseURL != "" {
		images = storage.NewSupabaseProvider(cfg.SupabaseURL, cfg.SupabaseServiceRoleKey, cfg.SupabaseBucket)
		// Not in the public /health probe (that would let anyone make the API call
		// Supabase on demand): check once at startup instead.
		pingCtx, cancelPing := context.WithTimeout(context.Background(), 5*time.Second)
		if err := images.Ping(pingCtx); err != nil {
			utils.LogWarn("image storage check failed; custom avatar uploads may not work", "err", err)
		}
		cancelPing()
	} else {
		utils.LogWarn("SUPABASE_URL not set: custom avatar uploads are disabled")
	}
	checker := health.New(checks)

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
		InitialTokens:  cfg.DailyTokens, // a new account starts with one day's tokens
		HashWorkers:    cfg.HashWorkers,
	})
	userOpts := []user.Option{user.WithUploadLimiter(user.NewRedisUploadLimiter(rdb, avatarUploadsPerHour, time.Hour))}
	if images != nil {
		userOpts = append(userOpts, user.WithImageStorage(images))
	}
	userH := user.NewHandler(userStore, walletSvc, auth.UserID, userOpts...)

	// Chat storage (P4.3): answered questions go to queue:chat:events, and the chat
	// storage worker below moves them into MongoDB.
	chatStore := chat.NewMongoStore(mongoClient.Database(cfg.MongoDatabase))
	indexCtx, cancelIndex := context.WithTimeout(context.Background(), 30*time.Second)
	err = chatStore.EnsureIndexes(indexCtx)
	cancelIndex()
	if err != nil {
		return err
	}
	chatBroker := queue.NewRedisListBroker(rdb, chat.DLQ)
	chatH := chat.NewHandler(chatStore, auth.UserID)
	casesH := cases.NewHandler(cases.NewPGStore(pg), auth.UserID).WithLogs(chatStore) // notes can point at log entries

	interrogationSvc := interrogation.NewService(interrogation.NewPGStore(pg), llmClients, walletSvc, chatBroker, chatStore)
	defer interrogationSvc.Wait() // let queued chat events finish publishing on shutdown

	// Background work in this process: the chat storage worker and the daily token
	// reset (P4.2). Both stop when run returns. Deferred calls run last-in first-out,
	// so they stop before the Redis and MongoDB clients close.
	bgCtx, stopBackground := context.WithCancel(context.Background())
	chatWorkerDone := make(chan struct{})
	go func() {
		defer close(chatWorkerDone)
		chat.NewWorker(chatBroker, chatStore).Run(bgCtx, cfg.ChatWorkerConcurrency)
	}()
	defer func() {
		stopBackground()
		<-chatWorkerDone
	}()
	resetCron, err := jobs.StartTokenReset(cfg.TokenResetCron, cfg.TokenResetTimezone, cfg.DailyTokens, walletSvc, rdb)
	if err != nil {
		return err
	}
	defer func() { <-resetCron.Stop().Done() }() // waits for a running reset to finish
	interrogationH := interrogation.NewHandler(interrogationSvc, auth.UserID)
	adminH := admin.NewHandler(admin.NewPGStore(pg), llmClients, auth.UserID)
	dashH := dashboard.NewHandler(dashboard.NewPGStore(pg), walletSvc, auth.UserID)

	mux.HandleFunc("POST /api/auth/signup", authH.Signup)
	mux.HandleFunc("POST /api/auth/resend-otp", authH.ResendOTP)
	mux.HandleFunc("POST /api/auth/verify-otp", authH.VerifyOTP)
	mux.HandleFunc("POST /api/auth/login", authH.Login)
	mux.HandleFunc("POST /api/auth/refresh", authH.Refresh)
	mux.HandleFunc("POST /api/auth/logout", authH.Logout)
	mux.HandleFunc("GET /api/user/profile", userH.Profile)
	mux.HandleFunc("POST /api/user/avatar", userH.SetAvatar)
	mux.HandleFunc("GET /api/cases/{id}/briefing", casesH.Briefing)
	mux.HandleFunc("GET /api/cases/{id}/suspects", casesH.Suspects)
	mux.HandleFunc("POST /api/cases/{id}/start", casesH.Start)
	mux.HandleFunc("GET /api/cases/{id}/notes", casesH.Notes)
	mux.HandleFunc("POST /api/cases/{id}/notes", casesH.AddNote)
	mux.HandleFunc("DELETE /api/cases/{id}/notes/{note_id}", casesH.DeleteNote)
	mux.HandleFunc("POST /api/cases/{id}/suspects/{suspect_id}/interrogate", interrogationH.Ask)
	mux.HandleFunc("GET /api/chat/{case_id}/{suspect_id}", chatH.History)
	mux.HandleFunc("POST /api/admin/cases", adminH.RequireAdmin(adminH.CreateCase))
	mux.HandleFunc("PATCH /api/admin/cases/{id}", adminH.RequireAdmin(adminH.UpdateCase))
	mux.HandleFunc("GET /api/dashboard/stories", dashH.Stories)
	mux.HandleFunc("GET /api/dashboard/stats", dashH.Stats)
	mux.HandleFunc("GET /api/dashboard/leaderboard", dashH.Leaderboard)
	mux.HandleFunc("GET /api/dashboard/active-case", dashH.ActiveCase)

	// Deny by default: every route needs a valid JWT unless it is listed here.
	public := auth.NewPublicRoutes(
		"GET /health",
		"POST /api/auth/signup",
		"POST /api/auth/resend-otp",
		"POST /api/auth/verify-otp",
		"POST /api/auth/login",
		"POST /api/auth/refresh",
	)
	protected := auth.Middleware(tokens, blacklist, public)(mux)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           middleware.Logging(middleware.CORS(cfg.CORSOrigins)(protected)),
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
