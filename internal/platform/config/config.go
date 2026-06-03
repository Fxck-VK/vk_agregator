// Package config loads service configuration from environment variables with
// sensible local-development defaults.
package config

import (
	"os"
	"strconv"
)

// Config is the full application configuration shared by the entrypoints.
type Config struct {
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
}

// Load reads configuration from the environment.
func Load() Config {
	host, _ := os.Hostname()
	return Config{
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

func defaultStr(v, def string) string {
	if v != "" {
		return v
	}
	return def
}
