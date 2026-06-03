// Package config loads service configuration from environment variables with
// sensible local-development defaults.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the full application configuration shared by the entrypoints.
type Config struct {
	// Env is the deployment environment ("development" or "production"). In
	// production the API fails closed when required secrets are missing.
	Env string

	HTTPAddr      string
	DatabaseURL   string
	MigrationsDir string

	RedisAddr     string
	RedisPassword string
	RedisDB       int

	S3Endpoint  string
	S3AccessKey string
	S3SecretKey string
	S3UseSSL    bool
	S3Bucket    string

	VKConfirmationToken string
	VKSecret            string

	AdminToken string

	WorkerGroup    string
	WorkerConsumer string

	// MaxAttempts bounds retryable re-enqueues before a task is dead-lettered.
	MaxAttempts int
	// RetryBaseDelay/RetryMaxDelay parameterize exponential backoff between
	// retries.
	RetryBaseDelay time.Duration
	RetryMaxDelay  time.Duration

	// ModerationExtraTerms extends the default keyword blocklist.
	ModerationExtraTerms []string

	// WebhookRateLimitRPS/Burst bound inbound webhook traffic per source.
	WebhookRateLimitRPS   float64
	WebhookRateLimitBurst int
}

// IsProduction reports whether the service runs in a production environment.
func (c Config) IsProduction() bool {
	return strings.EqualFold(c.Env, "production") || strings.EqualFold(c.Env, "prod")
}

// Validate fails closed: in production, secrets that protect inbound webhooks
// and the admin API must be set. Returns a descriptive error otherwise.
func (c Config) Validate() error {
	if !c.IsProduction() {
		return nil
	}
	var missing []string
	if c.VKSecret == "" {
		missing = append(missing, "VK_SECRET")
	}
	if c.AdminToken == "" {
		missing = append(missing, "ADMIN_TOKEN")
	}
	if c.VKConfirmationToken == "" || c.VKConfirmationToken == "dev-confirmation" {
		missing = append(missing, "VK_CONFIRMATION_TOKEN")
	}
	if len(missing) > 0 {
		return fmt.Errorf("config: missing required production secrets: %s", strings.Join(missing, ", "))
	}
	return nil
}

// Load reads configuration from the environment.
func Load() Config {
	host, _ := os.Hostname()
	return Config{
		Env:           env("APP_ENV", "development"),
		HTTPAddr:      env("HTTP_ADDR", ":8080"),
		DatabaseURL:   env("DATABASE_URL", "postgres://vk_ai_aggregator:vk_ai_aggregator@localhost:5432/vk_ai_aggregator?sslmode=disable"),
		MigrationsDir: env("MIGRATIONS_DIR", "migrations"),

		RedisAddr:     env("REDIS_ADDR", "localhost:6379"),
		RedisPassword: env("REDIS_PASSWORD", ""),
		RedisDB:       envInt("REDIS_DB", 0),

		S3Endpoint:  env("S3_ENDPOINT", "localhost:9000"),
		S3AccessKey: env("S3_ACCESS_KEY", "minioadmin"),
		S3SecretKey: env("S3_SECRET_KEY", "minioadmin"),
		S3UseSSL:    envBool("S3_USE_SSL", false),
		S3Bucket:    env("S3_BUCKET", "artifacts"),

		VKConfirmationToken: env("VK_CONFIRMATION_TOKEN", "dev-confirmation"),
		VKSecret:            env("VK_SECRET", ""),

		AdminToken: env("ADMIN_TOKEN", ""),

		WorkerGroup:    env("WORKER_GROUP", "workers"),
		WorkerConsumer: env("WORKER_CONSUMER", defaultStr(host, "worker-1")),

		MaxAttempts:    envInt("MAX_ATTEMPTS", 3),
		RetryBaseDelay: envDuration("RETRY_BASE_DELAY", 500*time.Millisecond),
		RetryMaxDelay:  envDuration("RETRY_MAX_DELAY", 30*time.Second),

		ModerationExtraTerms: envList("MODERATION_EXTRA_TERMS"),

		WebhookRateLimitRPS:   envFloat("WEBHOOK_RATE_LIMIT_RPS", 20),
		WebhookRateLimitBurst: envInt("WEBHOOK_RATE_LIMIT_BURST", 40),
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func envFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func envList(key string) []string {
	v := os.Getenv(key)
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func defaultStr(v, def string) string {
	if v != "" {
		return v
	}
	return def
}
