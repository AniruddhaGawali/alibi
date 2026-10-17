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
	WriteTimeout     time.Duration // 0 disables the deadline (streaming / slow LLM calls)
	OTLPEndpoint     string        // host:port; a scheme or path in the env value is stripped
	OllamaHost       string
	OpenRouterAPIKey string

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
	OTPMaxAttempts   int          // wrong codes allowed before the pending signup is discarded
	TrustedProxies   []*net.IPNet // peers whose X-Forwarded-For / X-Real-IP headers are believed

	// Email delivery (Brevo SMTP), used by the email worker.
	BrevoSMTPHost string
	BrevoSMTPPort int
	BrevoSMTPUser string
	BrevoSMTPPass string
	EmailFrom     string // sender address; defaults to BREVO_SMTP_USER

	// Email worker process.
	WorkerConcurrency  int           // parallel consumers
	WorkerHTTPAddr     string        // health endpoint of the worker
	WorkerMonitorEvery time.Duration // how often queue depths are sampled and logged
}

// minJWTSecretLen is the shortest HS256 secret we accept.
const minJWTSecretLen = 32

var (
	apiRequired    = []string{"POSTGRES_URI", "MONGO_URI", "REDIS_URI", "JWT_SECRET"}
	workerRequired = []string{"REDIS_URI", "BREVO_SMTP_HOST", "BREVO_SMTP_USER", "BREVO_SMTP_PASS"}
)

// defaultTrustedProxies are loopback and private ranges: a reverse proxy or
// container network in front of the API. Set TRUSTED_PROXIES to narrow this
// (or to "none" to never believe forwarding headers).
const defaultTrustedProxies = "127.0.0.0/8,::1/128,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16"

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
	return cfg, nil
}

// LoadWorker returns the config for the email worker, which needs Redis and
// Brevo but not Postgres, MongoDB or JWT.
func LoadWorker() (*Config, error) { return load(workerRequired) }

func load(required []string) (*Config, error) {
	if err := utils.LoadEnv(required, ".env", "../.env"); err != nil {
		return nil, err
	}
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

		HTTPAddr:         envStr("HTTP_ADDR", ":8080"),
		WriteTimeout:     time.Duration(envInt("HTTP_WRITE_TIMEOUT_SECONDS", 120, 0, 86400)) * time.Second,
		OTLPEndpoint:     otlpHostPort(envStr("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4318")),
		OllamaHost:       envStr("OLLAMA_HOST", "http://localhost:11434"),
		OpenRouterAPIKey: os.Getenv("OPENROUTER_API_KEY"),

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
		TrustedProxies:   proxies,

		BrevoSMTPHost: os.Getenv("BREVO_SMTP_HOST"),
		BrevoSMTPPort: envInt("BREVO_SMTP_PORT", 587, 1, 65535),
		BrevoSMTPUser: os.Getenv("BREVO_SMTP_USER"),
		BrevoSMTPPass: os.Getenv("BREVO_SMTP_PASS"),
		EmailFrom:     envStr("EMAIL_FROM", os.Getenv("BREVO_SMTP_USER")),

		WorkerConcurrency:  envInt("WORKER_CONCURRENCY", 4, 1, 64),
		WorkerHTTPAddr:     envStr("WORKER_HTTP_ADDR", ":8081"),
		WorkerMonitorEvery: time.Duration(envInt("WORKER_MONITOR_SECONDS", 30, 1, 3600)) * time.Second,
	}, nil
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
