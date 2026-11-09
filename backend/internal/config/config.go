package config

import (
	"fmt"
	"math"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"backend/pkg/utils"
)

type Config struct {
	PostgresURI string
	MongoURI    string
	RedisURI    string

	// Connection pool tuning (all optional, see defaults below).
	PGMaxConns     int32
	PGMinConns     int32
	MongoMaxPool   uint64
	RedisPoolSize  int
	ConnectTimeout time.Duration

	// Server and integrations (all optional).
	HTTPAddr         string
	WriteTimeout     time.Duration     // 0 disables the deadline (streaming / slow LLM calls)
	OTLPEndpoint     string            // host:port; a scheme or path in the env value is stripped
	OTLPSecure       bool              // TLS to the OTLP endpoint (hosted backends); off for the local collector
	OTLPHeaders      map[string]string // headers on every OTLP export (auth, dataset)
	OllamaHost       string
	CORSOrigins      []string // browser origins allowed to call the API
	OpenRouterAPIKey string
	LLMTimeout       time.Duration // one interrogation call to OpenRouter

	// Limits on the interrogation route (credit protection). 0 turns a limit off
	// where the minimum allows it.
	LLMUserPerMinute    int // questions per player per minute, blocked ones included
	LLMIPPerMinute      int // questions per client address per minute
	LLMMaxConcurrent    int // model calls in flight per API process
	LLMRefundsPerDay    int // tokens a player can get back per day after model failures
	LLMDailyCallBudget  int // model calls per UTC day across all players
	LLMDailyTokenBudget int // model tokens (prompt + answer) per UTC day across all players
	LLMStrikesPerHour   int // blocked injection attempts per hour before a player is locked out

	// Token economy and chat storage.
	// The next three only seed app_settings on the first startup; after that they are
	// edited from the admin panel (GET/PUT /api/admin/config).
	DailyTokens           int           // first-startup seed of the daily allowance (1-100); the admin panel owns it after that
	TokenResetTime        string        // HH:MM the daily reset runs at
	TokenResetTimezone    string        // IANA zone the reset time is read in
	MongoDatabase         string        // database holding chat_messages
	ChatWorkerConcurrency int           // chat storage consumers in the API process
	ChatWorkerPopTimeout  time.Duration // how long an idle chat consumer waits per Redis BRPOP
	ChatMonitorEvery      time.Duration // how often the chat queue depths are sampled

	// Image object storage (Supabase Storage). Optional: when SUPABASE_URL is
	// empty, custom avatar uploads are disabled.
	SupabaseURL            string
	SupabaseServiceRoleKey string
	SupabaseBucket         string

	// Google sign-in (optional: without GOOGLE_CLIENT_ID the Google button answers "not available").
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string // this API's callback, registered in the Google console
	FrontendURL        string // where the browser is sent back after Google

	// Response cache (internal/httpcache).
	CacheEnabled      bool
	CacheUserTTL      time.Duration // server-side TTL of per-player data (profile, notes, chat, result)
	CacheWalletTTL    time.Duration // server-side TTL of data that carries the token balance (stats, profile)
	CacheCatalogTTL   time.Duration // server-side TTL of case content (stories, briefing, suspects)
	CacheStaticMaxAge time.Duration // how long browsers reuse case content without asking
	CacheResultMaxAge time.Duration // how long browsers reuse a closed case's result (it never changes)
	LeaderboardTTL    time.Duration // how long each API process keeps the top players in memory (0 = off)

	// AuthFailOpen keeps signed-in players working while Redis is down (see auth.Blacklist.SetFailOpen).
	AuthFailOpen bool

	// Auth.
	MigrateOnStart bool // run pending goose migrations when the API starts
	JWTSecret      string
	AccessTTL      time.Duration
	RefreshTTL     time.Duration

	// OTP signup flow: 10 min code lifetime, per-email cooldown and daily cap, per-IP window budget.
	OTPTTL           time.Duration
	OTPCooldown      time.Duration // minimum gap between codes sent to one email
	OTPEmailDailyMax int           // codes per email per 24h
	OTPIPMax         int           // codes per client IP per OTPIPWindow
	OTPIPWindow      time.Duration
	OTPMaxAttempts   int // wrong codes allowed before the pending signup is discarded
	AuthIPMax        int // requests per client IP per AuthIPWindow on signup / OTP / login / refresh
	AuthIPWindow     time.Duration
	SignupIPDailyMax int          // accounts created per client IP per 24 h
	LoginMaxFailures int          // wrong passwords per email before login locks (exponential backoff)
	HashWorkers      int          // max concurrent bcrypt operations; 0 = number of CPUs
	TrustedProxies   []*net.IPNet // peers whose X-Forwarded-For / X-Real-IP headers are believed

	// Email delivery (Brevo SMTP), used by the email worker.
	BrevoSMTPHost string
	BrevoSMTPPort int
	BrevoSMTPUser string
	BrevoSMTPPass string
	EmailFrom     string // sender address; defaults to BREVO_SMTP_USER

	// EmbedEmailWorker runs the email worker (and its queue monitor) inside the API
	// process, for hosts that only allow one service. Needs the BREVO_* settings.
	EmbedEmailWorker bool

	// Email worker process.
	WorkerConcurrency  int           // parallel consumers
	WorkerPopTimeout   time.Duration // how long an idle email consumer waits per Redis BRPOP
	WorkerHTTPAddr     string        // health endpoint of the worker
	WorkerMonitorEvery time.Duration // how often queue depths are sampled and logged
}

// minJWTSecretLen is the shortest HS256 secret we accept.
const minJWTSecretLen = 32

var (
	apiRequired    = []string{"POSTGRES_URI", "MONGO_URI", "REDIS_URI", "JWT_SECRET"}
	workerRequired = []string{"REDIS_URI", "BREVO_SMTP_HOST", "BREVO_SMTP_USER", "BREVO_SMTP_PASS"}
	redisRequired  = []string{"REDIS_URI"}
	assetsRequired = []string{"SUPABASE_URL", "SUPABASE_SERVICE_ROLE_KEY"}
)

// defaultTrustedProxies is loopback only. Private ranges are deliberately not
// trusted by default: with Docker's published ports every external client
// reaches the API from the bridge gateway (172.x), which would make any client's
// X-Forwarded-For believable. In production set TRUSTED_PROXIES to the exact
// addresses of your reverse proxy / load balancer ("none" = never believe
// forwarding headers).
const defaultTrustedProxies = "127.0.0.0/8,::1/128"

// Load reads the .env file (repo root, or backend/ cwd) and returns the config
// for the API server.
func Load() (*Config, error) {
	cfg, err := load(apiRequired)
	if err != nil {
		return nil, err
	}
	if len(cfg.JWTSecret) < minJWTSecretLen {
		return nil, fmt.Errorf("JWT_SECRET must be at least %d characters", minJWTSecretLen)
	}
	if err := checkSupabase(cfg); err != nil {
		return nil, err
	}
	if cfg.EmbedEmailWorker {
		for _, key := range workerRequired {
			if os.Getenv(key) == "" {
				return nil, fmt.Errorf("missing required environment variable: %s (EMBED_EMAIL_WORKER is on)", key)
			}
		}
	}
	if err := checkGoogle(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// checkGoogle validates the Google sign-in settings when a client id is set.
func checkGoogle(cfg *Config) error {
	if cfg.GoogleClientID == "" {
		return nil
	}
	if cfg.GoogleClientSecret == "" {
		return fmt.Errorf("GOOGLE_CLIENT_SECRET is required when GOOGLE_CLIENT_ID is set")
	}
	for name, raw := range map[string]string{"GOOGLE_REDIRECT_URL": cfg.GoogleRedirectURL, "FRONTEND_URL": cfg.FrontendURL} {
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
			return fmt.Errorf("%s must be an http(s) URL", name)
		}
		if u.Scheme == "http" && !isLoopback(u.Hostname()) {
			return fmt.Errorf("%s must use https unless it points at localhost", name)
		}
	}
	return nil
}

// LoadAssets returns the config for the asset upload tool, which needs only
// the Supabase settings.
func LoadAssets() (*Config, error) {
	cfg, err := load(assetsRequired)
	if err != nil {
		return nil, err
	}
	return cfg, checkSupabase(cfg)
}

// checkSupabase validates the storage settings when they are present.
func checkSupabase(cfg *Config) error {
	if cfg.SupabaseURL == "" {
		return nil
	}
	u, err := url.Parse(cfg.SupabaseURL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return fmt.Errorf("SUPABASE_URL must be an http(s) URL such as https://<project>.supabase.co")
	}
	if u.Scheme == "http" && !isLoopback(u.Hostname()) {
		return fmt.Errorf("SUPABASE_URL must use https unless it points at localhost (the service-role key would travel in clear text)")
	}
	if cfg.SupabaseServiceRoleKey == "" {
		return fmt.Errorf("SUPABASE_SERVICE_ROLE_KEY is required when SUPABASE_URL is set")
	}
	return nil
}

// LoadWorker returns the config for the email worker, which needs Redis and
// Brevo but not Postgres, MongoDB or JWT.
func LoadWorker() (*Config, error) { return load(workerRequired) }

// envFiles lists where a .env may live: the working directory and its parents.
// That covers `go run` from the repo root, backend/ and backend/cmd/<name>/
// (three levels up at most, so a stray .env higher up is never picked up).
func envFiles() []string {
	files := []string{".env"}
	for prefix := "../"; len(files) < 4; prefix += "../" {
		files = append(files, prefix+".env")
	}
	return files
}

// LoadRedisOnly returns the config for tools that only talk to Redis (emailctl).
func LoadRedisOnly() (*Config, error) { return load(redisRequired) }

func load(required []string) (*Config, error) {
	if err := utils.LoadEnv(required, envFiles()...); err != nil {
		return nil, err
	}
	otlp := loadOTLP()
	proxies, err := parseCIDRs(envStr("TRUSTED_PROXIES", defaultTrustedProxies))
	if err != nil {
		return nil, fmt.Errorf("TRUSTED_PROXIES: %w", err)
	}
	return &Config{
		PostgresURI:    os.Getenv("POSTGRES_URI"),
		MongoURI:       os.Getenv("MONGO_URI"),
		RedisURI:       os.Getenv("REDIS_URI"),
		PGMaxConns:     int32(envInt("PG_MAX_CONNS", 10, 1, math.MaxInt32)),
		PGMinConns:     int32(envInt("PG_MIN_CONNS", 2, 0, math.MaxInt32)),
		MongoMaxPool:   uint64(envInt("MONGO_MAX_POOL_SIZE", 50, 1, math.MaxInt32)),
		RedisPoolSize:  envInt("REDIS_POOL_SIZE", 10, 1, math.MaxInt32),
		ConnectTimeout: time.Duration(envInt("DB_CONNECT_TIMEOUT_SECONDS", 5, 1, 3600)) * time.Second,

		HTTPAddr:         httpAddr(),
		WriteTimeout:     time.Duration(envInt("HTTP_WRITE_TIMEOUT_SECONDS", 120, 0, 86400)) * time.Second,
		OTLPEndpoint:     otlp.endpoint,
		OTLPSecure:       otlp.secure,
		OTLPHeaders:      otlp.headers,
		OllamaHost:       envStr("OLLAMA_HOST", "http://localhost:11434"),
		CORSOrigins:      parseOrigins(envStr("CORS_ALLOWED_ORIGINS", "http://localhost:5173,http://127.0.0.1:5173")),
		OpenRouterAPIKey: apiKey(os.Getenv("OPENROUTER_API_KEY")),
		LLMTimeout:       time.Duration(envInt("LLM_TIMEOUT_SECONDS", 20, 1, 120)) * time.Second,

		LLMUserPerMinute:    envInt("LLM_USER_PER_MINUTE", 6, 0, 600),
		LLMIPPerMinute:      envInt("LLM_IP_PER_MINUTE", 30, 0, 6000),
		LLMMaxConcurrent:    envInt("LLM_MAX_CONCURRENT", 8, 1, 1000),
		LLMRefundsPerDay:    envInt("LLM_REFUNDS_PER_DAY", 3, 0, 100),
		LLMDailyCallBudget:  envInt("LLM_DAILY_CALL_BUDGET", 5000, 0, 100_000_000),
		LLMDailyTokenBudget: envInt("LLM_DAILY_TOKEN_BUDGET", 3_000_000, 0, 2_000_000_000),
		LLMStrikesPerHour:   envInt("LLM_INJECTION_STRIKES_PER_HOUR", 5, 0, 1000),

		DailyTokens:           envInt("DAILY_TOKENS", 10, 1, 100),
		TokenResetTime:        envStr("TOKEN_RESET_TIME", "00:00"),
		TokenResetTimezone:    envStr("TOKEN_RESET_TIMEZONE", "UTC"),
		MongoDatabase:         envStr("MONGO_DATABASE", "alibi"),
		ChatWorkerConcurrency: envInt("CHAT_WORKER_CONCURRENCY", 1, 1, 32),
		ChatWorkerPopTimeout:  time.Duration(envInt("CHAT_WORKER_POP_TIMEOUT_SECONDS", 30, 1, 300)) * time.Second,
		ChatMonitorEvery:      time.Duration(envInt("CHAT_MONITOR_SECONDS", 900, 1, 86400)) * time.Second,

		SupabaseURL:            strings.TrimRight(os.Getenv("SUPABASE_URL"), "/"),
		SupabaseServiceRoleKey: os.Getenv("SUPABASE_SERVICE_ROLE_KEY"),
		SupabaseBucket:         envStr("SUPABASE_BUCKET_NAME", "alibi-assets"),

		GoogleClientID:     strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_ID")),
		GoogleClientSecret: strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_SECRET")),
		GoogleRedirectURL:  envStr("GOOGLE_REDIRECT_URL", "http://localhost:8080/api/auth/google/callback"),
		FrontendURL:        strings.TrimRight(envStr("FRONTEND_URL", "http://localhost:5173"), "/"),

		CacheEnabled:      envBool("CACHE_ENABLED", true),
		CacheUserTTL:      time.Duration(envInt("CACHE_USER_TTL_SECONDS", 300, 1, 86400)) * time.Second,
		CacheWalletTTL:    time.Duration(envInt("CACHE_WALLET_TTL_SECONDS", 60, 1, 3600)) * time.Second,
		CacheCatalogTTL:   time.Duration(envInt("CACHE_CATALOG_TTL_SECONDS", 3600, 1, 604800)) * time.Second,
		CacheStaticMaxAge: time.Duration(envInt("CACHE_BROWSER_MAX_AGE_SECONDS", 3600, 0, 2592000)) * time.Second,
		CacheResultMaxAge: time.Duration(envInt("CACHE_RESULT_MAX_AGE_SECONDS", 86400, 0, 2592000)) * time.Second,
		LeaderboardTTL:    time.Duration(envInt("LEADERBOARD_MEMORY_TTL_SECONDS", 60, 0, 3600)) * time.Second,

		AuthFailOpen: envBool("AUTH_FAIL_OPEN_WHEN_REDIS_DOWN", true),

		MigrateOnStart: envBool("MIGRATE_ON_START", true),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		AccessTTL:      time.Duration(envInt("JWT_ACCESS_TTL_MINUTES", 15, 1, 1440)) * time.Minute,
		RefreshTTL:     time.Duration(envInt("JWT_REFRESH_TTL_HOURS", 168, 1, 8760)) * time.Hour,

		OTPTTL:           time.Duration(envInt("OTP_TTL_MINUTES", 10, 1, 120)) * time.Minute,
		OTPCooldown:      time.Duration(envInt("OTP_COOLDOWN_SECONDS", 60, 1, 3600)) * time.Second,
		OTPEmailDailyMax: envInt("OTP_EMAIL_DAILY_MAX", 5, 1, 100),
		OTPIPMax:         envInt("OTP_IP_MAX", 10, 1, 10000),
		OTPIPWindow:      time.Duration(envInt("OTP_IP_WINDOW_MINUTES", 15, 1, 1440)) * time.Minute,
		OTPMaxAttempts:   envInt("OTP_MAX_ATTEMPTS", 5, 1, 20),
		AuthIPMax:        envInt("AUTH_IP_MAX", 60, 1, 100000),
		AuthIPWindow:     time.Duration(envInt("AUTH_IP_WINDOW_MINUTES", 15, 1, 1440)) * time.Minute,
		SignupIPDailyMax: envInt("SIGNUP_IP_DAILY_MAX", 10, 1, 100000),
		LoginMaxFailures: envInt("LOGIN_MAX_FAILURES", 5, 1, 100),
		HashWorkers:      envInt("BCRYPT_WORKERS", 0, 0, 256),
		TrustedProxies:   proxies,

		BrevoSMTPHost: os.Getenv("BREVO_SMTP_HOST"),
		BrevoSMTPPort: envInt("BREVO_SMTP_PORT", 587, 1, 65535),
		BrevoSMTPUser: os.Getenv("BREVO_SMTP_USER"),
		BrevoSMTPPass: os.Getenv("BREVO_SMTP_PASS"),
		EmailFrom:     envStr("EMAIL_FROM", os.Getenv("BREVO_SMTP_USER")),

		EmbedEmailWorker:   envBool("EMBED_EMAIL_WORKER", false),
		WorkerConcurrency:  envInt("WORKER_CONCURRENCY", 1, 1, 64),
		WorkerPopTimeout:   time.Duration(envInt("WORKER_POP_TIMEOUT_SECONDS", 30, 1, 300)) * time.Second,
		WorkerHTTPAddr:     envStr("WORKER_HTTP_ADDR", ":8081"),
		WorkerMonitorEvery: time.Duration(envInt("WORKER_MONITOR_SECONDS", 900, 1, 86400)) * time.Second,
	}, nil
}

// httpAddr is HTTP_ADDR, else ":$PORT" (what Koyeb, Render and Fly inject), else ":8080".
func httpAddr() string {
	if v := os.Getenv("HTTP_ADDR"); v != "" {
		return v
	}
	if p := strings.TrimSpace(os.Getenv("PORT")); p != "" {
		return ":" + p
	}
	return ":8080"
}

type otlpSettings struct {
	endpoint string
	secure   bool
	headers  map[string]string
}

// loadOTLP reads the telemetry target. Locally it is the plain-HTTP collector.
// With AXIOM_API_TOKEN (and AXIOM_DATASET) set it exports to Axiom over TLS
// (AXIOM_OTLP_ENDPOINT overrides api.axiom.co, e.g. for the EU region). Any
// OTEL_EXPORTER_OTLP_ENDPOINT with an https:// scheme, or OTEL_EXPORTER_OTLP_TLS=true,
// also turns TLS on. Extra headers: OTEL_EXPORTER_OTLP_HEADERS="k=v,k2=v2".
func loadOTLP() otlpSettings {
	raw := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	o := otlpSettings{
		endpoint: otlpHostPort(envStr("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4318")),
		secure:   strings.HasPrefix(raw, "https://") || envBool("OTEL_EXPORTER_OTLP_TLS", false),
		headers:  map[string]string{},
	}
	for _, kv := range strings.Split(os.Getenv("OTEL_EXPORTER_OTLP_HEADERS"), ",") {
		if k, v, ok := strings.Cut(kv, "="); ok && strings.TrimSpace(k) != "" {
			o.headers[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	if token := apiKey(os.Getenv("AXIOM_API_TOKEN")); token != "" {
		if raw == "" {
			o.endpoint = otlpHostPort(envStr("AXIOM_OTLP_ENDPOINT", "api.axiom.co"))
		}
		o.secure = true
		o.headers["Authorization"] = "Bearer " + token
		if ds := strings.TrimSpace(os.Getenv("AXIOM_DATASET")); ds != "" {
			o.headers["X-Axiom-Dataset"] = ds
		}
	}
	return o
}

// parseCIDRs parses a comma-separated CIDR list; "" and "none" give an empty list.
func parseCIDRs(list string) ([]*net.IPNet, error) {
	var out []*net.IPNet
	for _, part := range strings.Split(list, ",") {
		part = strings.TrimSpace(part)
		if part == "" || strings.EqualFold(part, "none") {
			continue
		}
		_, n, err := net.ParseCIDR(part)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR %q", part)
		}
		out = append(out, n)
	}
	return out, nil
}

func envBool(key string, def bool) bool {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		utils.LogWarn("[ENV] invalid value, using default", "key", key, "value", raw, "default", def)
		return def
	}
	return v
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// otlpHostPort reduces an OTLP endpoint such as "http://collector:4318/" to
// "collector:4318", the form the exporters' WithEndpoint expects.
func otlpHostPort(endpoint string) string {
	if strings.Contains(endpoint, "://") {
		if u, err := url.Parse(endpoint); err == nil && u.Host != "" {
			return u.Host
		}
	}
	return strings.TrimRight(endpoint, "/")
}

// envInt returns the integer in key if it lies within [min, max]. If key is
// unset it returns def; if it is set but invalid or out of range it logs a
// warning and returns def.
func envInt(key string, def, min, max int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < min || v > max {
		utils.LogWarn("[ENV] invalid value, using default", "key", key, "value", raw, "default", def)
		return def
	}
	return v
}

// parseOrigins splits a comma-separated origin list, dropping blanks and trailing slashes.
func parseOrigins(list string) []string {
	var out []string
	for _, o := range strings.Split(list, ",") {
		if o = strings.TrimRight(strings.TrimSpace(o), "/"); o != "" {
			out = append(out, o)
		}
	}
	return out
}

// isLoopback reports whether host is localhost or a loopback IP.
func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// apiKey cleans an API key read from the environment. A value that starts with
// "#" is a comment that ended up as the value (a blank "KEY=   # note" line) and
// counts as not set.
func apiKey(v string) string {
	v = strings.TrimSpace(v)
	if strings.HasPrefix(v, "#") {
		return ""
	}
	return v
}
