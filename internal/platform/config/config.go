package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
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
	// OutboxVisibilityTimeout is how long an outbox row may stay in `processing`
	// before the worker reaper assumes the prior worker crashed and requeues it
	// to `pending` (Batch 2B). Must comfortably exceed the longest single Handle
	// (SMTP send), not the full retry budget.
	OutboxVisibilityTimeout time.Duration
	// ReminderDispatchEnabled gates the worker reminder send-path (DispatchDue +
	// stale-dispatching reaper). Default true (preserve current behavior). When
	// false, Seed/Materialize keep running so occurrences stay accurate while no
	// email is sent — a no-data-loss rollback switch.
	ReminderDispatchEnabled bool
	// ReminderVisibilityTimeout is how long a reminder_occurrence may stay in
	// DISPATCHING before the reaper assumes a crashed worker and requeues it to
	// PENDING. Mirrors OutboxVisibilityTimeout for the reminder pipeline.
	ReminderVisibilityTimeout time.Duration

	// Data
	MySQLDSN string

	// Vnstock read-only market reference (CMS listed companies; separate from cobo_iam DB).
	VnstockMySQLDSN      string
	VnstockMarketEnabled bool

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
	// SupportEmail footer in registration OTP emails (SUPPORT_EMAIL).
	SupportEmail string

	// UserInvitationTokenTTL TTL for CMS user-invitation links (user_invitations.expires_at).
	UserInvitationTokenTTL time.Duration

	// EmailVerificationOTPTTL expiry for email verification OTP emails (register/resend).
	EmailVerificationOTPTTL time.Duration

	// InviteDefaultRoleCode is roles.role_code used when CMS invite omits role_id/role_code (e.g. user_thuong).
	InviteDefaultRoleCode string

	// RegistrationDisabled when true disables POST /api/v1/auth/register (REGISTRATION_DISABLED=true).
	RegistrationDisabled bool

	// Public API base URL used when backend emits externally callable URLs.
	PublicAPIBaseURL string

	// LoginPasswordRSAPrivateKeyPEM optional PEM (PKCS#1 or PKCS#8) RSA private key.
	// When set, GET /api/v1/auth/login-password-key exposes the public half and login accepts password_cipher.
	LoginPasswordRSAPrivateKeyPEM string
	LoginPasswordRSAKeyID         string

	// CORSAllowedOrigins is a comma-separated list of allowed browser Origins
	// (e.g. "https://app.example.com,https://www.example.com"). If empty in
	// development, loopback (localhost/127.0.0.1) and PublicWebBaseURL are allowed
	// so a Vite app can call a separate API process without a proxy. Production
	// with an empty value disables CORS—set explicit origins when the SPA is on
	// another host than the API.
	CORSAllowedOrigins string

	// SMTP (worker side-effect for auth email events).
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
	SMTPFrom     string
	// EmailTemplateSource controls phase-1 rendering path: legacy | embed.
	EmailTemplateSource string
	// EmailNotificationEnabled gates the new NotificationService worker handler (phases 2-3).
	// When false, no new email.dispatch outbox handler runs.
	EmailNotificationEnabled bool
	// EmailDeliveryPath selects the delivery code path per dispatch: legacy | notification_service.
	// Used during phases 2-5 to migrate auth and reminder paths one at a time.
	EmailDeliveryPath string
	// EmailFormat selects the rendered MIME flavour: text | html. Default text.
	EmailFormat string
	// EmailShadowMode mirrors dispatch into the new pipeline without taking over delivery (phases 2-4).
	// Shadow failures must never break the legacy path.
	EmailShadowMode bool
	// AdhocEmailOutboxEnabled routes AdhocProposalNotifier's email dispatch through
	// the durable EmailNotificationService pipeline as the sole authoritative
	// sender (Batch 2 cutover flag). When false, the legacy DeliveryAdapter.Send
	// path remains authoritative.
	AdhocEmailOutboxEnabled bool
	// AdhocEmailShadowRecipient is the sink address durable-path shadow dispatches
	// rewrite "to" into while EmailShadowMode is on (and AdhocEmailOutboxEnabled is
	// off), so real users never receive a duplicate adhoc email. Empty disables the
	// shadow dispatch (logged as a warning at startup, never a crash).
	AdhocEmailShadowRecipient string

	// CMS media signed-upload/storage settings.
	CMSMediaUploadSigningSecret string
	CMSMediaUploadURLTTL        time.Duration
	CMSMediaStorageDir          string

	// User avatar (self-service /me/avatar; separate signing secret and storage subdir).
	UserAvatarMaxBytes            int64
	UserAvatarAllowedTypes        []string
	UserAvatarStorageDir          string
	UserAvatarUploadSigningSecret string
	UserAvatarSignedURLTTL        time.Duration

	// ── Customize-Workflow feature flags ─────────────────────────────────────
	// WORKFLOW_GROUPS_ENABLED: expose groups array on effective workflow response.
	WorkflowGroupsEnabled bool
	// WORKFLOW_DRAFT_ETAG_MODE: off | warn | enforce — optimistic locking on override drafts.
	WorkflowDraftEtagMode string
	// WORKFLOW_SNAPSHOT_ENABLED: freeze workflow snapshot on instance creation.
	WorkflowSnapshotEnabled bool
	// WORKFLOW_TIMELINE_ENABLED: compute step timelines and seed milestones on instance creation.
	WorkflowTimelineEnabled bool
	// WORKFLOW_REMINDERS_ENABLED: dispatch reminder emails from seeded milestones.
	WorkflowRemindersEnabled bool
	// WORKFLOW_ADHOC_ENABLED: enable ad-hoc proposal state machine.
	WorkflowAdhocEnabled bool
	// WORKFLOW_ADHOC_AUTOAPPROVE_ENABLED: skip focal approval step (single-stage admin-only).
	WorkflowAdhocAutoApproveEnabled bool
	// ADHOC_EMAIL_METRICS_ENABLED: emit cobo_adhoc_proposal_transition_total (Batch 5(a) / AK.3).
	// Default true — additive-only instrumentation, zero behavioural risk.
	AdhocEmailMetricsEnabled bool
	// PERIODIC_SEEDING_ENABLED: worker seeds + materializes periodic/custom disclosure records.
	PeriodicSeedingEnabled bool

	// COMPANY_PROVISION_IDEMPOTENCY_REQUIRED: require Idempotency-Key on POST /company/initialize (and /company/create when enabled).
	CompanyProvisionIdempotencyRequired bool
	// COMPANY_SELF_CREATE_ENABLED: expose POST /api/v1/company/create for Nth self-service company.
	CompanySelfCreateEnabled bool

	// TEMPLATE_APPLICABILITY_STRICT_FILTER: when false, global templates without rules pass filter (grace).
	TemplateApplicabilityStrictFilter bool

	// DEADLINE_ENGINE_V2: use ResolveDeadline() unified engine (Batch 2+). Default false — legacy runtime.
	DeadlineEngineV2 bool
}

// Load reads configuration from the environment with safe defaults for local dev.
func Load() (Config, error) {
	cfg := Config{
		ServiceName:                   getenv("SERVICE_NAME", "cobo_iam_services"),
		Env:                           getenv("ENV", "development"),
		HTTPAddr:                      getenv("HTTP_ADDR", ":8080"),
		HTTPReadTimeout:               durationEnv("HTTP_READ_TIMEOUT", 15*time.Second),
		HTTPWriteTimeout:              durationEnv("HTTP_WRITE_TIMEOUT", 15*time.Second),
		HTTPIdleTimeout:               durationEnv("HTTP_IDLE_TIMEOUT", 60*time.Second),
		WorkerTickInterval:            durationEnv("WORKER_TICK_INTERVAL", 5*time.Second),
		OutboxVisibilityTimeout:       durationEnv("OUTBOX_VISIBILITY_TIMEOUT", 5*time.Minute),
		ReminderDispatchEnabled:       boolEnv("REMINDER_DISPATCH_ENABLED", true),
		ReminderVisibilityTimeout:     durationEnv("REMINDER_VISIBILITY_TIMEOUT", 5*time.Minute),
		MySQLDSN:                      normalizeMySQLDSN(os.Getenv("MYSQL_DSN")),
		VnstockMySQLDSN:               normalizeMySQLDSN(os.Getenv("VNSTOCK_MYSQL_DSN")),
		VnstockMarketEnabled:          boolEnv("VNSTOCK_MARKET_ENABLED", false),
		RedisAddr:                     os.Getenv("REDIS_ADDR"),
		RedisPassword:                 os.Getenv("REDIS_PASSWORD"),
		RedisDB:                       intEnv("REDIS_DB", 0),
		EffectiveAccessCacheTTL:       durationEnv("EFFECTIVE_ACCESS_CACHE_TTL", 5*time.Minute),
		LogLevel:                      getenv("LOG_LEVEL", "info"),
		AccessTokenMode:               getenv("ACCESS_TOKEN_MODE", "opaque"),
		AccessTokenTTL:                durationEnv("ACCESS_TOKEN_TTL", 15*time.Minute),
		JWTIssuer:                     getenv("JWT_ISSUER", "cobo_iam_services"),
		JWTAudience:                   getenv("JWT_AUDIENCE", "cobo_clients"),
		JWTAlg:                        getenv("JWT_ALG", "EdDSA"),
		JWTSigningPrivateKey:          os.Getenv("JWT_SIGNING_PRIVATE_KEY_PEM"),
		JWTVerifyPublicKeys:           os.Getenv("JWT_VERIFY_PUBLIC_KEYS_JSON"),
		JWTClockSkewSec:               intEnv("JWT_CLOCK_SKEW_SEC", 60),
		PublicWebBaseURL:              getenv("PUBLIC_WEB_BASE_URL", "http://localhost:5173"),
		SupportEmail:                  getenv("SUPPORT_EMAIL", "support@cobo.vn"),
		PublicAPIBaseURL:              getenv("PUBLIC_API_BASE_URL", "http://localhost:8080"),
		UserInvitationTokenTTL:        durationEnv("USER_INVITATION_TOKEN_TTL", 72*time.Hour),
		EmailVerificationOTPTTL:       durationEnv("EMAIL_VERIFICATION_OTP_TTL", 15*time.Minute),
		InviteDefaultRoleCode:         getenv("INVITE_DEFAULT_ROLE_CODE", "user_thuong"),
		RegistrationDisabled:          strings.EqualFold(strings.TrimSpace(os.Getenv("REGISTRATION_DISABLED")), "true"),
		LoginPasswordRSAKeyID:         getenv("LOGIN_PASSWORD_RSA_KEY_ID", "default"),
		CORSAllowedOrigins:            os.Getenv("CORS_ALLOWED_ORIGINS"),
		SMTPHost:                      os.Getenv("SMTP_HOST"),
		SMTPPort:                      intEnv("SMTP_PORT", 587),
		SMTPUser:                      os.Getenv("SMTP_USER"),
		SMTPPassword:                  os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:                      getenv("SMTP_FROM", "no-reply@cobo.local"),
		EmailTemplateSource:           getenv("EMAIL_TEMPLATE_SOURCE", "legacy"),
		EmailNotificationEnabled:      boolEnv("EMAIL_NOTIFICATION_ENABLED", false),
		EmailDeliveryPath:             getenv("EMAIL_DELIVERY_PATH", "legacy"),
		EmailFormat:                   getenv("EMAIL_FORMAT", "text"),
		EmailShadowMode:               boolEnv("EMAIL_SHADOW_MODE", false),
		AdhocEmailOutboxEnabled:       boolEnv("ADHOC_EMAIL_OUTBOX_ENABLED", false),
		AdhocEmailShadowRecipient:     getenv("ADHOC_EMAIL_SHADOW_RECIPIENT", ""),
		CMSMediaUploadSigningSecret:   getenv("CMS_MEDIA_UPLOAD_SIGNING_SECRET", "dev-cms-media-secret"),
		CMSMediaUploadURLTTL:          durationEnv("CMS_MEDIA_UPLOAD_URL_TTL", 10*time.Minute),
		CMSMediaStorageDir:            getenv("CMS_MEDIA_STORAGE_DIR", "./var/cms-media"),
		UserAvatarMaxBytes:            int64(intEnv("USER_AVATAR_MAX_BYTES", 2*1024*1024)),
		UserAvatarAllowedTypes:        parseCommaSeparatedList(getenv("USER_AVATAR_ALLOWED_TYPES", "image/png,image/jpeg,image/webp")),
		UserAvatarStorageDir:          strings.TrimSpace(os.Getenv("USER_AVATAR_STORAGE_DIR")),
		UserAvatarUploadSigningSecret: resolveUserAvatarSigningSecret(os.Getenv("USER_AVATAR_UPLOAD_SIGNING_SECRET"), getenv("ENV", "development")),
		UserAvatarSignedURLTTL:        userAvatarSignedURLTTL(),

		WorkflowGroupsEnabled:               boolEnv("WORKFLOW_GROUPS_ENABLED", false),
		WorkflowDraftEtagMode:               getenv("WORKFLOW_DRAFT_ETAG_MODE", "off"),
		WorkflowSnapshotEnabled:             boolEnv("WORKFLOW_SNAPSHOT_ENABLED", false),
		WorkflowTimelineEnabled:             boolEnv("WORKFLOW_TIMELINE_ENABLED", false),
		WorkflowRemindersEnabled:            boolEnv("WORKFLOW_REMINDERS_ENABLED", false),
		WorkflowAdhocEnabled:                devAwareBoolEnv("WORKFLOW_ADHOC_ENABLED", false, true),
		WorkflowAdhocAutoApproveEnabled:     boolEnv("WORKFLOW_ADHOC_AUTOAPPROVE_ENABLED", false),
		AdhocEmailMetricsEnabled:            boolEnv("ADHOC_EMAIL_METRICS_ENABLED", true),
		PeriodicSeedingEnabled:              boolEnv("PERIODIC_SEEDING_ENABLED", false),
		CompanyProvisionIdempotencyRequired: boolEnv("COMPANY_PROVISION_IDEMPOTENCY_REQUIRED", false),
		CompanySelfCreateEnabled:            boolEnv("COMPANY_SELF_CREATE_ENABLED", false),
		TemplateApplicabilityStrictFilter:     boolEnv("TEMPLATE_APPLICABILITY_STRICT_FILTER", false),
		DeadlineEngineV2:                      boolEnv("DEADLINE_ENGINE_V2", false),
	}
	if cfg.WorkerTickInterval < time.Second {
		return Config{}, fmt.Errorf("WORKER_TICK_INTERVAL too small")
	}
	if cfg.OutboxVisibilityTimeout < 30*time.Second {
		return Config{}, fmt.Errorf("OUTBOX_VISIBILITY_TIMEOUT too small (min 30s)")
	}
	if cfg.ReminderVisibilityTimeout < 30*time.Second {
		return Config{}, fmt.Errorf("REMINDER_VISIBILITY_TIMEOUT too small (min 30s)")
	}
	if cfg.CMSMediaUploadURLTTL < time.Minute {
		return Config{}, fmt.Errorf("CMS_MEDIA_UPLOAD_URL_TTL too small")
	}
	if cfg.UserAvatarMaxBytes <= 0 {
		return Config{}, fmt.Errorf("USER_AVATAR_MAX_BYTES must be positive")
	}
	if cfg.UserAvatarSignedURLTTL < time.Minute {
		return Config{}, fmt.Errorf("USER_AVATAR_SIGNED_URL_TTL too small")
	}
	if len(cfg.UserAvatarAllowedTypes) == 0 {
		return Config{}, fmt.Errorf("USER_AVATAR_ALLOWED_TYPES must not be empty")
	}
	if !strings.EqualFold(strings.TrimSpace(cfg.Env), "development") && strings.TrimSpace(cfg.UserAvatarUploadSigningSecret) == "" {
		return Config{}, fmt.Errorf("USER_AVATAR_UPLOAD_SIGNING_SECRET is required when ENV is not development")
	}
	switch cfg.AccessTokenMode {
	case "opaque", "jwt", "dual":
	default:
		return Config{}, fmt.Errorf("ACCESS_TOKEN_MODE invalid: %s", cfg.AccessTokenMode)
	}
	switch cfg.WorkflowDraftEtagMode {
	case "off", "warn", "enforce":
	default:
		return Config{}, fmt.Errorf("WORKFLOW_DRAFT_ETAG_MODE invalid: %s", cfg.WorkflowDraftEtagMode)
	}
	switch cfg.EmailTemplateSource {
	case "legacy", "embed":
	default:
		return Config{}, fmt.Errorf("EMAIL_TEMPLATE_SOURCE invalid: %s", cfg.EmailTemplateSource)
	}
	switch cfg.EmailDeliveryPath {
	case "legacy", "notification_service":
	default:
		return Config{}, fmt.Errorf("EMAIL_DELIVERY_PATH invalid: %s", cfg.EmailDeliveryPath)
	}
	switch cfg.EmailFormat {
	case "text", "html":
	default:
		return Config{}, fmt.Errorf("EMAIL_FORMAT invalid: %s", cfg.EmailFormat)
	}
	if err := validatePublicWebBaseURL(cfg.Env, cfg.PublicWebBaseURL); err != nil {
		return Config{}, err
	}
	pem, err := loadLoginPasswordRSAPEM()
	if err != nil {
		return Config{}, err
	}
	cfg.LoginPasswordRSAPrivateKeyPEM = pem
	return cfg, nil
}

// validatePublicWebBaseURL rejects localhost/empty URLs in non-development
// environments so email action links are never silently broken in production.
func validatePublicWebBaseURL(env, rawURL string) error {
	isDev := strings.EqualFold(strings.TrimSpace(env), "development")
	url := strings.TrimSpace(rawURL)
	if !isDev {
		if url == "" {
			return fmt.Errorf("PUBLIC_WEB_BASE_URL is required when ENV is not development")
		}
		lower := strings.ToLower(url)
		if strings.Contains(lower, "localhost") || strings.Contains(lower, "127.0.0.1") {
			return fmt.Errorf("PUBLIC_WEB_BASE_URL must not point to localhost when ENV=%q (got %q) — set a real HTTPS URL", env, url)
		}
	}
	return nil
}

// loadLoginPasswordRSAPEM reads RSA private key PEM from LOGIN_PASSWORD_RSA_PRIVATE_KEY_PEM
// or LOGIN_PASSWORD_RSA_PRIVATE_KEY_PEM_FILE (used by docker-compose dev/artifacts).
// A missing PEM file is ignored so the API can still start (plaintext login fallback).
func loadLoginPasswordRSAPEM() (string, error) {
	if v := strings.TrimSpace(os.Getenv("LOGIN_PASSWORD_RSA_PRIVATE_KEY_PEM")); v != "" {
		return v, nil
	}
	path := strings.TrimSpace(os.Getenv("LOGIN_PASSWORD_RSA_PRIVATE_KEY_PEM_FILE"))
	if path == "" {
		return "", nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("LOGIN_PASSWORD_RSA_PRIVATE_KEY_PEM_FILE: read %s: %w", path, err)
	}
	return string(b), nil
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

func boolEnv(key string, def bool) bool {
	s := strings.TrimSpace(os.Getenv(key))
	if s == "" {
		return def
	}
	return strings.EqualFold(s, "true") || s == "1"
}

// devAwareBoolEnv uses devDefault when ENV=development and the variable is unset.
func devAwareBoolEnv(key string, prodDefault, devDefault bool) bool {
	if strings.TrimSpace(os.Getenv(key)) != "" {
		return boolEnv(key, prodDefault)
	}
	if strings.EqualFold(strings.TrimSpace(getenv("ENV", "development")), "development") {
		return devDefault
	}
	return prodDefault
}

func userAvatarSignedURLTTL() time.Duration {
	if d := durationEnv("USER_AVATAR_SIGNED_URL_TTL", 0); d > 0 {
		return d
	}
	sec := intEnv("USER_AVATAR_SIGNED_URL_TTL_SECONDS", 900)
	if sec <= 0 {
		sec = 900
	}
	return time.Duration(sec) * time.Second
}

func resolveUserAvatarSigningSecret(raw, env string) string {
	if v := strings.TrimSpace(raw); v != "" {
		return v
	}
	if strings.EqualFold(strings.TrimSpace(env), "development") {
		return "dev-user-avatar-secret"
	}
	return ""
}

func normalizeMySQLDSN(dsn string) string {
	trimmed := strings.TrimSpace(dsn)
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	withQuery := trimmed
	if !strings.Contains(trimmed, "?") {
		withQuery += "?"
	}
	if !strings.Contains(lower, "charset=") {
		if strings.HasSuffix(withQuery, "?") || strings.HasSuffix(withQuery, "&") {
			withQuery += "charset=utf8mb4"
		} else {
			withQuery += "&charset=utf8mb4"
		}
	}
	if !strings.Contains(strings.ToLower(withQuery), "collation=") {
		if strings.HasSuffix(withQuery, "?") || strings.HasSuffix(withQuery, "&") {
			withQuery += "collation=utf8mb4_unicode_ci"
		} else {
			withQuery += "&collation=utf8mb4_unicode_ci"
		}
	}
	return withQuery
}
