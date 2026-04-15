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
	HTTPAddr         string
	HTTPReadTimeout  time.Duration
	HTTPWriteTimeout time.Duration
	HTTPIdleTimeout  time.Duration

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

	// Access token migration (opaque -> jwt).
	AccessTokenMode string
	AccessTokenTTL  time.Duration

	// JWT settings (used when ACCESS_TOKEN_MODE=jwt|dual).
	JWTIssuer            string
	JWTAudience          string
	JWTAlg               string
	JWTSigningPrivateKey string
	JWTVerifyPublicKeys  string
	JWTClockSkewSec      int

	// Public web app base URL used in email action links.
	PublicWebBaseURL string

	// SMTP (worker side-effect for auth email events).
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
	SMTPFrom     string
}

// Load reads configuration from the environment with safe defaults for local dev.
func Load() (Config, error) {
	cfg := Config{
		ServiceName:             getenv("SERVICE_NAME", "cobo_iam_services"),
		Env:                     getenv("ENV", "development"),
		HTTPAddr:                getenv("HTTP_ADDR", ":8080"),
		HTTPReadTimeout:         durationEnv("HTTP_READ_TIMEOUT", 15*time.Second),
		HTTPWriteTimeout:        durationEnv("HTTP_WRITE_TIMEOUT", 15*time.Second),
		HTTPIdleTimeout:         durationEnv("HTTP_IDLE_TIMEOUT", 60*time.Second),
		WorkerTickInterval:      durationEnv("WORKER_TICK_INTERVAL", 5*time.Second),
		MySQLDSN:                os.Getenv("MYSQL_DSN"),
		RedisAddr:               os.Getenv("REDIS_ADDR"),
		RedisPassword:           os.Getenv("REDIS_PASSWORD"),
		RedisDB:                 intEnv("REDIS_DB", 0),
		EffectiveAccessCacheTTL: durationEnv("EFFECTIVE_ACCESS_CACHE_TTL", 5*time.Minute),
		LogLevel:                getenv("LOG_LEVEL", "info"),
		AccessTokenMode:         getenv("ACCESS_TOKEN_MODE", "opaque"),
		AccessTokenTTL:          durationEnv("ACCESS_TOKEN_TTL", 15*time.Minute),
		JWTIssuer:               getenv("JWT_ISSUER", "cobo_iam_services"),
		JWTAudience:             getenv("JWT_AUDIENCE", "cobo_clients"),
		JWTAlg:                  getenv("JWT_ALG", "EdDSA"),
		JWTSigningPrivateKey:    os.Getenv("JWT_SIGNING_PRIVATE_KEY_PEM"),
		JWTVerifyPublicKeys:     os.Getenv("JWT_VERIFY_PUBLIC_KEYS_JSON"),
		JWTClockSkewSec:         intEnv("JWT_CLOCK_SKEW_SEC", 60),
		PublicWebBaseURL:        getenv("PUBLIC_WEB_BASE_URL", "http://localhost:5173"),
		SMTPHost:                os.Getenv("SMTP_HOST"),
		SMTPPort:                intEnv("SMTP_PORT", 587),
		SMTPUser:                os.Getenv("SMTP_USER"),
		SMTPPassword:            os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:                getenv("SMTP_FROM", "no-reply@cobo.local"),
	}
	if cfg.WorkerTickInterval < time.Second {
		return Config{}, fmt.Errorf("WORKER_TICK_INTERVAL too small")
	}
	switch cfg.AccessTokenMode {
	case "opaque", "jwt", "dual":
	default:
		return Config{}, fmt.Errorf("ACCESS_TOKEN_MODE invalid: %s", cfg.AccessTokenMode)
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
