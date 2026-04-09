package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds process-wide settings loaded from environment variables.
type Config struct {
	ServiceName string
	Env         string

	// API
	HTTPAddr        string
	HTTPReadTimeout time.Duration
	HTTPWriteTimeout time.Duration
	HTTPIdleTimeout time.Duration

	// Worker
	WorkerTickInterval time.Duration

	// Data
	MySQLDSN string

	// Redis (optional; P2.2 effective-access projection cache)
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// EffectiveAccessCacheTTL applies to in-memory and Redis projection cache entries.
	EffectiveAccessCacheTTL time.Duration

	// Observability
	LogLevel string
}

// Load reads configuration from the environment with safe defaults for local dev.
func Load() (Config, error) {
	cfg := Config{
		ServiceName:      getenv("SERVICE_NAME", "cobo_iam_services"),
		Env:              getenv("ENV", "development"),
		HTTPAddr:         getenv("HTTP_ADDR", ":8080"),
		HTTPReadTimeout:  durationEnv("HTTP_READ_TIMEOUT", 15*time.Second),
		HTTPWriteTimeout: durationEnv("HTTP_WRITE_TIMEOUT", 15*time.Second),
		HTTPIdleTimeout:  durationEnv("HTTP_IDLE_TIMEOUT", 60*time.Second),
		WorkerTickInterval: durationEnv("WORKER_TICK_INTERVAL", 5*time.Second),
		MySQLDSN:         os.Getenv("MYSQL_DSN"),
		RedisAddr:        os.Getenv("REDIS_ADDR"),
		RedisPassword:    os.Getenv("REDIS_PASSWORD"),
		RedisDB:          intEnv("REDIS_DB", 0),
		EffectiveAccessCacheTTL: durationEnv("EFFECTIVE_ACCESS_CACHE_TTL", 5*time.Minute),
		LogLevel:         getenv("LOG_LEVEL", "info"),
	}
	if cfg.WorkerTickInterval < time.Second {
		return Config{}, fmt.Errorf("WORKER_TICK_INTERVAL too small")
	}
	return cfg, nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func intEnv(key string, def int) int {
	s := os.Getenv(key)
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

func durationEnv(key string, def time.Duration) time.Duration {
	s := os.Getenv(key)
	if s == "" {
		return def
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		if n, err2 := strconv.Atoi(s); err2 == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
		return def
	}
	return d
}
