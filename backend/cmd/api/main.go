package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"backend/internal/accusation"
	"backend/internal/admin"
	"backend/internal/auth"
	"backend/internal/cases"
	"backend/internal/chat"
	"backend/internal/config"
	"backend/internal/dashboard"
	"backend/internal/db"
	"backend/internal/email"
	"backend/internal/health"
	"backend/internal/httpcache"
	"backend/internal/interrogation"
	"backend/internal/jobs"
	"backend/internal/llm"
	"backend/internal/middleware"
	"backend/internal/queue"
	"backend/internal/settings"
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

	shutdownTelemetry, err := logger.InitWith(context.Background(), "alibi-api", logger.Options{Endpoint: cfg.OTLPEndpoint, Secure: cfg.OTLPSecure, Headers: cfg.OTLPHeaders})
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

	// Global game settings (P5.3): seeded from env on the first startup, edited from
	// the admin panel after that.
	settingsCtx, cancelSettings := context.WithTimeout(context.Background(), 10*time.Second)
	gameSettings, err := settings.New(settingsCtx, settings.NewPGStore(pg), settings.Settings{
		DailyTokens: cfg.DailyTokens, MaxTokens: max(settings.DefaultMaxTokens, cfg.DailyTokens), ResetTime: cfg.TokenResetTime, Timezone: cfg.TokenResetTimezone,
	})
	cancelSettings()
	if err != nil {
		return fmt.Errorf("load settings: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", checker.Handler)

	tokens := auth.NewTokenManager(cfg.JWTSecret, cfg.AccessTTL, cfg.RefreshTTL)
	blacklist := auth.NewBlacklist(rdb)
	blacklist.SetFailOpen(cfg.AuthFailOpen)
	walletSvc := wallet.New(userStore, rdb, gameSettings.MaxTokens)
	mailBroker := queue.NewRedisListBroker(rdb, email.DLQ, queue.WithDLQMaxLen(email.DLQMaxLen), queue.WithPopTimeout(cfg.WorkerPopTimeout))
	authH := auth.NewHandler(auth.Deps{
		Users:     userStore,
		Tokens:    tokens,
		Blacklist: blacklist,
		OTP:       auth.NewOTPStore(rdb, cfg.OTPTTL, cfg.OTPMaxAttempts, []byte(cfg.JWTSecret)),
		Limiter: auth.NewLimiter(rdb, auth.Policy{
			Cooldown: cfg.OTPCooldown, EmailDailyMax: cfg.OTPEmailDailyMax,
			IPMax: cfg.OTPIPMax, IPWindow: cfg.OTPIPWindow,
			AuthIPMax: cfg.AuthIPMax, AuthIPWindow: cfg.AuthIPWindow, SignupIPDailyMax: cfg.SignupIPDailyMax,
			LoginMaxFailures: cfg.LoginMaxFailures,
		}).WithFallback(cfg.AuthFailOpen), // login and refresh keep working, with in-memory limits, if Redis is down
		Mail:           queue.NewRedisListBroker(rdb, email.DLQ, queue.WithDLQMaxLen(email.DLQMaxLen)), // the email worker consumes it
		Coins:          walletSvc,
		TrustedProxies: cfg.TrustedProxies,
		InitialTokens:  gameSettings.DailyTokens, // a new account starts with one day's tokens
		HashWorkers:    cfg.HashWorkers,
		Google: auth.GoogleConfig{
			ClientID: cfg.GoogleClientID, ClientSecret: cfg.GoogleClientSecret,
			RedirectURL: cfg.GoogleRedirectURL, FrontendURL: cfg.FrontendURL,
		},
		OAuth:      auth.NewOAuthStore(rdb),
		Identities: userStore,
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
	// Response cache (internal/httpcache): routes are wrapped with a policy below; admin
	// routes are never cached, they only invalidate.
	hc := httpcache.New(rdb, auth.UserID)
	hc.SetEnabled(cfg.CacheEnabled)
	chatBroker := queue.NewRedisListBroker(rdb, chat.DLQ, queue.WithPopTimeout(cfg.ChatWorkerPopTimeout))
	interrogationStore := interrogation.NewPGStore(pg)
	chatH := chat.NewHandler(chatStore, auth.UserID, chat.WithPlay(func(ctx context.Context, userID, caseID string) (string, error) {
		// The conversation belongs to the play-through the player is in now; with none, they see all of it.
		lock, err := interrogationStore.Lock(ctx, userID)
		if err != nil || lock == nil || lock.StoryID != caseID {
			return "", err
		}
		return lock.CaseID, nil
	}))
	casesH := cases.NewHandler(cases.NewPGStore(pg), auth.UserID).WithLogs(chatStore) // notes can point at log entries

	guard := interrogation.NewRedisGuard(rdb, interrogation.GuardConfig{
		PerMinute:      cfg.LLMUserPerMinute,
		IPPerMinute:    cfg.LLMIPPerMinute,
		UserDailyCalls: cfg.DailyTokens + cfg.LLMRefundsPerDay, // calls <= tokens spent + the refund allowance
		DailyCalls: func(ctx context.Context) int { // the same, following the admin panel
			return gameSettings.DailyTokens(ctx) + cfg.LLMRefundsPerDay
		},
		RefundsPerDay:    cfg.LLMRefundsPerDay,
		MaxConcurrent:    cfg.LLMMaxConcurrent,
		DailyCallBudget:  int64(cfg.LLMDailyCallBudget),
		DailyTokenBudget: int64(cfg.LLMDailyTokenBudget),
		StrikesPerHour:   cfg.LLMStrikesPerHour,
		InFlightTTL:      cfg.LLMTimeout + 30*time.Second,
	})
	interrogationSvc := interrogation.NewService(interrogationStore, llmClients, walletSvc, chatBroker, chatStore, interrogation.WithGuard(guard))
	defer interrogationSvc.Wait() // let queued chat events finish publishing on shutdown

	// Background work in this process: the chat storage worker and the daily token
	// reset (P4.2). Both stop when run returns. Deferred calls run last-in first-out,
	// so they stop before the Redis and MongoDB clients close.
	bgCtx, stopBackground := context.WithCancel(context.Background())
	chatWorkerDone := make(chan struct{})
	go func() {
		defer close(chatWorkerDone)
		chat.NewWorker(chatBroker, chatStore).
			OnStored(func(ctx context.Context, ev chat.Event) { hc.Bump(ctx, "u:"+ev.UserID+":chat:"+ev.CaseID) }).
			Run(bgCtx, cfg.ChatWorkerConcurrency)
	}()
	go chat.Monitor(bgCtx, chatBroker, cfg.ChatMonitorEvery)
	// Single-service hosting: run the email worker here too (EMBED_EMAIL_WORKER=true).
	// Otherwise run cmd/worker as its own service.
	emailWorkerDone := make(chan struct{})
	if cfg.EmbedEmailWorker {
		mailer := utils.NewMailer(cfg.BrevoSMTPHost, cfg.BrevoSMTPPort, cfg.BrevoSMTPUser, cfg.BrevoSMTPPass, cfg.EmailFrom)
		go func() {
			defer close(emailWorkerDone)
			email.NewWorker(mailBroker, mailer).Run(bgCtx, cfg.WorkerConcurrency)
		}()
		go email.Monitor(bgCtx, mailBroker, cfg.WorkerMonitorEvery)
	} else {
		close(emailWorkerDone)
	}
	defer func() {
		stopBackground()
		<-chatWorkerDone
		<-emailWorkerDone
	}()
	resetCron, err := jobs.StartTokenReset(func(ctx context.Context) (jobs.Schedule, error) {
		s := gameSettings.Current(ctx)
		hour, minute, loc, err := s.Clock()
		return jobs.Schedule{Tokens: s.DailyTokens, Hour: hour, Minute: minute, Loc: loc}, err
	}, walletSvc, rdb)
	if err != nil {
		return err
	}
	defer func() { <-resetCron.Stop().Done() }() // waits for a running reset to finish
	interrogationH := interrogation.NewHandler(interrogationSvc, auth.UserID,
		interrogation.WithClientIP(func(r *http.Request) string { return utils.ClientIP(r, cfg.TrustedProxies) }))
	accuseH := accusation.NewHandler(accusation.NewPGStore(pg), accusation.NewRedisLeaderboard(rdb), auth.UserID)
	adminOpts := []admin.Option{admin.WithTokenRole(auth.Role), admin.WithSettings(gameSettings), admin.WithTokens(walletSvc), admin.WithLimitReset(guard)}
	if images != nil {
		adminOpts = append(adminOpts, admin.WithImages(images))
	}
	adminH := admin.NewHandler(admin.NewPGStore(pg), llmClients, auth.UserID, adminOpts...)
	dashH := dashboard.NewHandler(dashboard.NewPGStore(pg), walletSvc, auth.UserID).
		WithBoard(dashboard.NewRedisBoard(rdb)).WithMemoryTTL(cfg.LeaderboardTTL)
	accuseH.OnScoreChange(dashH.Forget) // a won or lost case shows on this replica's leaderboard at once

	// Cache policies. Server TTLs and the browser max-age come from the CACHE_* settings;
	// tags say what a cached response depends on, and the Invalidates wrappers below drop
	// them. {uid} is the caller and {id} / {case_id} path values.
	walletData := httpcache.Policy{TTL: cfg.CacheWalletTTL, Tags: []string{"u:{uid}", "u:{uid}:profile", "u:{uid}:wallet"}}
	// Case content rarely changes: the browser reuses it without asking, and it is dropped
	// when an admin edits any case ("catalog") or the caller's lock changes ("u:{uid}").
	caseContent := httpcache.Policy{TTL: cfg.CacheCatalogTTL, MaxAge: cfg.CacheStaticMaxAge, Tags: []string{"u:{uid}", "catalog"}}
	// A closed case's verdict never changes (only a 200 exists once it is closed, a 404 is
	// never cached), so browsers may keep it for a day. The frontend adds ?u=<user id> so
	// two accounts on one browser never share a cached verdict.
	resultData := httpcache.Policy{TTL: cfg.CacheUserTTL, MaxAge: cfg.CacheResultMaxAge, Tags: []string{"u:{uid}", "catalog"}}
	userData := func(tags ...string) httpcache.Policy {
		return httpcache.Policy{TTL: cfg.CacheUserTTL, Tags: append([]string{"u:{uid}"}, tags...)}
	}

	mux.HandleFunc("POST /api/auth/signup", authH.Signup)
	mux.HandleFunc("POST /api/auth/resend-otp", authH.ResendOTP)
	mux.HandleFunc("POST /api/auth/verify-otp", authH.VerifyOTP)
	mux.HandleFunc("POST /api/auth/login", authH.Login)
	mux.HandleFunc("POST /api/auth/refresh", authH.Refresh)
	mux.HandleFunc("GET /api/auth/google/start", authH.GoogleStart)
	mux.HandleFunc("GET /api/auth/google/callback", authH.GoogleCallback)
	mux.HandleFunc("POST /api/auth/google/exchange", authH.GoogleExchange)
	mux.HandleFunc("POST /api/auth/logout", authH.Logout)
	mux.HandleFunc("GET /api/user/profile", hc.Wrap(walletData, userH.Profile))
	mux.HandleFunc("POST /api/user/avatar", hc.Invalidates(userH.SetAvatar, "u:{uid}:profile"))
	mux.HandleFunc("PUT /api/user/mode", hc.Invalidates(userH.SetMode, "u:{uid}:profile"))
	mux.HandleFunc("PUT /api/user/username", hc.Invalidates(userH.SetUsername, "u:{uid}:profile"))
	mux.HandleFunc("GET /api/cases/{id}/briefing", hc.Wrap(caseContent, casesH.Briefing))
	mux.HandleFunc("GET /api/cases/{id}/suspects", hc.Wrap(caseContent, casesH.Suspects))
	mux.HandleFunc("GET /api/cases/{id}/hints", hc.Wrap(userData("u:{uid}:hints:{id}", "catalog"), casesH.HintState))
	mux.HandleFunc("POST /api/cases/{id}/start", hc.Invalidates(casesH.Start, "u:{uid}"))
	mux.HandleFunc("POST /api/cases/{id}/hints", hc.Invalidates(casesH.Hints, "u:{uid}:hints:{id}"))
	mux.HandleFunc("POST /api/cases/{id}/accuse", hc.Invalidates(accuseH.Accuse, "u:{uid}"))
	mux.HandleFunc("GET /api/cases/{id}/result", hc.Wrap(resultData, casesH.Result))
	mux.HandleFunc("GET /api/cases/{id}/notes", hc.Wrap(userData("u:{uid}:notes:{id}"), casesH.Notes))
	mux.HandleFunc("POST /api/cases/{id}/notes", hc.Invalidates(casesH.AddNote, "u:{uid}:notes:{id}"))
	mux.HandleFunc("DELETE /api/cases/{id}/notes/{note_id}", hc.Invalidates(casesH.DeleteNote, "u:{uid}:notes:{id}"))
	mux.HandleFunc("POST /api/cases/{id}/suspects/{suspect_id}/interrogate", hc.Invalidates(interrogationH.Ask, "u:{uid}:wallet", "u:{uid}:chat:{id}"))
	mux.HandleFunc("GET /api/chat/{case_id}/{suspect_id}", hc.Wrap(userData("u:{uid}:chat:{case_id}"), chatH.History))
	// Admin panel (P5.3): every route checks the admin role (token, then database).
	adminOnly := adminH.AdminOnly
	mux.HandleFunc("GET /api/admin/models", adminOnly(adminH.ListModels))
	mux.HandleFunc("GET /api/admin/config", adminOnly(adminH.GetConfig))
	mux.HandleFunc("PUT /api/admin/config", adminOnly(adminH.UpdateConfig))
	mux.HandleFunc("POST /api/admin/upload", adminOnly(adminH.Upload))
	mux.HandleFunc("GET /api/admin/users", adminOnly(adminH.SearchUsers))
	mux.HandleFunc("POST /api/admin/users/{id}/tokens", hc.Invalidates(adminOnly(adminH.GrantTokens), "u:{id}:wallet"))
	mux.HandleFunc("POST /api/admin/users/{id}/reset-limits", adminOnly(adminH.ResetLimits))
	mux.HandleFunc("GET /api/admin/cases", adminOnly(adminH.ListCases))
	mux.HandleFunc("POST /api/admin/cases", hc.Invalidates(adminOnly(adminH.CreateCase), "catalog"))
	mux.HandleFunc("GET /api/admin/cases/{id}", adminOnly(adminH.GetCase))
	mux.HandleFunc("PATCH /api/admin/cases/{id}", hc.Invalidates(adminOnly(adminH.UpdateCase), "catalog"))
	mux.HandleFunc("PUT /api/admin/cases/order", hc.Invalidates(adminOnly(adminH.ReorderCases), "catalog"))
	mux.HandleFunc("DELETE /api/admin/cases/{id}", hc.Invalidates(adminOnly(adminH.DeleteCase), "catalog"))
	mux.HandleFunc("GET /api/admin/cases/{id}/suspects", adminOnly(adminH.ListSuspects))
	mux.HandleFunc("POST /api/admin/cases/{id}/suspects", hc.Invalidates(adminOnly(adminH.CreateSuspect), "catalog"))
	mux.HandleFunc("PATCH /api/admin/cases/{id}/suspects/{suspect_id}", hc.Invalidates(adminOnly(adminH.UpdateSuspect), "catalog"))
	mux.HandleFunc("DELETE /api/admin/cases/{id}/suspects/{suspect_id}", hc.Invalidates(adminOnly(adminH.DeleteSuspect), "catalog"))
	mux.HandleFunc("POST /api/admin/cases/{id}/scenes", hc.Invalidates(adminOnly(adminH.CreateScene), "catalog"))
	mux.HandleFunc("PATCH /api/admin/cases/{id}/scenes/{scene_id}", hc.Invalidates(adminOnly(adminH.UpdateScene), "catalog"))
	mux.HandleFunc("DELETE /api/admin/cases/{id}/scenes/{scene_id}", hc.Invalidates(adminOnly(adminH.DeleteScene), "catalog"))
	mux.HandleFunc("GET /api/dashboard/stories", hc.Wrap(httpcache.Policy{TTL: cfg.CacheCatalogTTL, MaxAge: min(cfg.CacheStaticMaxAge, 30*time.Minute), Shared: true, Tags: []string{"catalog"}}, dashH.Stories))
	mux.HandleFunc("GET /api/dashboard/stats", hc.Wrap(httpcache.Policy{TTL: cfg.CacheWalletTTL, Tags: []string{"u:{uid}", "u:{uid}:wallet", "catalog"}}, dashH.Stats))
	mux.HandleFunc("GET /api/dashboard/leaderboard", dashH.Leaderboard)
	mux.HandleFunc("GET /api/game/rules", gameSettings.PublicRules)
	mux.HandleFunc("GET /api/dashboard/active-case", hc.Wrap(userData("catalog"), dashH.ActiveCase))

	// Deny by default: every route needs a valid JWT unless it is listed here.
	public := auth.NewPublicRoutes(
		"GET /health",
		"POST /api/auth/signup",
		"POST /api/auth/resend-otp",
		"POST /api/auth/verify-otp",
		"POST /api/auth/login",
		"POST /api/auth/refresh",
		// Google sign-in: the browser is redirected here by Google, before it has a session.
		"GET /api/auth/google/start",
		"GET /api/auth/google/callback",
		"POST /api/auth/google/exchange",
		// The landing page shows the top players to visitors (usernames and trophies only).
		"GET /api/dashboard/leaderboard",
		"GET /api/game/rules", // the landing page shows the token numbers to visitors
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

	// Long enough for an interrogation in flight to finish, so a deploy does not cut
	// off a paid answer.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), max(shutdownTimeout, cfg.LLMTimeout+5*time.Second))
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	utils.LogInfo("server stopped gracefully")
	return nil
}
