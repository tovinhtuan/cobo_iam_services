package config

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNormalizeMySQLDSN(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "append charset and collation when missing",
			in:   "cobo:cobo@tcp(mysql:3306)/cobo_iam?parseTime=true&loc=UTC&tls=false",
			want: "cobo:cobo@tcp(mysql:3306)/cobo_iam?parseTime=true&loc=UTC&tls=false&charset=utf8mb4&collation=utf8mb4_unicode_ci",
		},
		{
			name: "keep existing charset and collation",
			in:   "u:p@tcp(localhost:3306)/db?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci",
			want: "u:p@tcp(localhost:3306)/db?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci",
		},
		{
			name: "append query params when no query exists",
			in:   "u:p@tcp(localhost:3306)/db",
			want: "u:p@tcp(localhost:3306)/db?charset=utf8mb4&collation=utf8mb4_unicode_ci",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeMySQLDSN(tc.in)
			if got != tc.want {
				t.Fatalf("normalizeMySQLDSN() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestLoad_EmailFlagDefaults locks the safe defaults so phase 0 cannot accidentally
// change current production behaviour. Every email flag added in phase 0 must default
// to a value that keeps the legacy auth/reminder paths unchanged.
func TestLoad_EmailFlagDefaults(t *testing.T) {
	for _, k := range []string{
		"EMAIL_TEMPLATE_SOURCE",
		"EMAIL_NOTIFICATION_ENABLED",
		"EMAIL_DELIVERY_PATH",
		"EMAIL_FORMAT",
		"EMAIL_SHADOW_MODE",
	} {
		t.Setenv(k, "")
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.EmailTemplateSource != "legacy" {
		t.Fatalf("EmailTemplateSource = %q, want legacy", cfg.EmailTemplateSource)
	}
	if cfg.EmailNotificationEnabled {
		t.Fatalf("EmailNotificationEnabled must default to false")
	}
	if cfg.EmailDeliveryPath != "legacy" {
		t.Fatalf("EmailDeliveryPath = %q, want legacy", cfg.EmailDeliveryPath)
	}
	if cfg.EmailFormat != "text" {
		t.Fatalf("EmailFormat = %q, want text", cfg.EmailFormat)
	}
	if cfg.EmailShadowMode {
		t.Fatalf("EmailShadowMode must default to false")
	}
}

func TestLoad_VnstockMarketFlags(t *testing.T) {
	t.Setenv("VNSTOCK_MYSQL_DSN", "vnstock:secret@tcp(mysql:3306)/vnstock?parseTime=true&loc=UTC&tls=false")
	t.Setenv("VNSTOCK_MARKET_ENABLED", "true")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.VnstockMarketEnabled {
		t.Fatal("VnstockMarketEnabled want true")
	}
	want := "vnstock:secret@tcp(mysql:3306)/vnstock?parseTime=true&loc=UTC&tls=false&charset=utf8mb4&collation=utf8mb4_unicode_ci"
	if cfg.VnstockMySQLDSN != want {
		t.Fatalf("VnstockMySQLDSN = %q, want %q", cfg.VnstockMySQLDSN, want)
	}
}

func TestLoad_EmailDeliveryPathInvalid(t *testing.T) {
	t.Setenv("EMAIL_DELIVERY_PATH", "bogus")
	if _, err := Load(); err == nil {
		t.Fatal("expected error for invalid EMAIL_DELIVERY_PATH")
	}
}

func TestLoad_EmailFormatInvalid(t *testing.T) {
	t.Setenv("EMAIL_FORMAT", "rtf")
	if _, err := Load(); err == nil {
		t.Fatal("expected error for invalid EMAIL_FORMAT")
	}
}

func TestLoad_UserAvatarDefaults(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("USER_AVATAR_UPLOAD_SIGNING_SECRET", "")
	t.Setenv("USER_AVATAR_STORAGE_DIR", "")
	t.Setenv("USER_AVATAR_MAX_BYTES", "")
	t.Setenv("USER_AVATAR_ALLOWED_TYPES", "")
	t.Setenv("USER_AVATAR_SIGNED_URL_TTL", "")
	t.Setenv("USER_AVATAR_SIGNED_URL_TTL_SECONDS", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.UserAvatarMaxBytes != 2097152 {
		t.Fatalf("UserAvatarMaxBytes = %d, want 2097152", cfg.UserAvatarMaxBytes)
	}
	if len(cfg.UserAvatarAllowedTypes) != 3 {
		t.Fatalf("UserAvatarAllowedTypes len = %d, want 3", len(cfg.UserAvatarAllowedTypes))
	}
	if cfg.UserAvatarUploadSigningSecret != "dev-user-avatar-secret" {
		t.Fatalf("dev signing secret = %q", cfg.UserAvatarUploadSigningSecret)
	}
	if cfg.UserAvatarSignedURLTTL != 900*time.Second {
		t.Fatalf("UserAvatarSignedURLTTL = %v, want 900s", cfg.UserAvatarSignedURLTTL)
	}
	wantRoot := "var/cms-media/user-avatars"
	got := cfg.ResolveUserAvatarStorageDir()
	if !strings.HasSuffix(filepath.ToSlash(got), wantRoot) {
		t.Fatalf("ResolveUserAvatarStorageDir() = %q, want suffix %q", got, wantRoot)
	}
}

func TestLoad_UserAvatarEnvOverride(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("USER_AVATAR_UPLOAD_SIGNING_SECRET", "custom-secret")
	t.Setenv("USER_AVATAR_MAX_BYTES", "1048576")
	t.Setenv("USER_AVATAR_ALLOWED_TYPES", "image/png")
	t.Setenv("USER_AVATAR_STORAGE_DIR", "/tmp/avatar-root")
	t.Setenv("USER_AVATAR_SIGNED_URL_TTL", "30m")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.UserAvatarUploadSigningSecret != "custom-secret" {
		t.Fatalf("secret = %q", cfg.UserAvatarUploadSigningSecret)
	}
	if cfg.UserAvatarMaxBytes != 1048576 {
		t.Fatalf("max bytes = %d", cfg.UserAvatarMaxBytes)
	}
	if len(cfg.UserAvatarAllowedTypes) != 1 || cfg.UserAvatarAllowedTypes[0] != "image/png" {
		t.Fatalf("allowed types = %v", cfg.UserAvatarAllowedTypes)
	}
	if cfg.ResolveUserAvatarStorageDir() != "/tmp/avatar-root" {
		t.Fatalf("storage dir = %q", cfg.ResolveUserAvatarStorageDir())
	}
	if cfg.UserAvatarSignedURLTTL != 30*time.Minute {
		t.Fatalf("ttl = %v", cfg.UserAvatarSignedURLTTL)
	}
}

func TestLoad_UserAvatarSigningSecretRequiredOutsideDevelopment(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("USER_AVATAR_UPLOAD_SIGNING_SECRET", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when signing secret missing in production")
	}
}

func TestLoad_ReminderHardeningDefaults(t *testing.T) {
	t.Setenv("REMINDER_DISPATCH_ENABLED", "")
	t.Setenv("REMINDER_VISIBILITY_TIMEOUT", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.ReminderDispatchEnabled {
		t.Fatalf("ReminderDispatchEnabled must default to true")
	}
	if cfg.ReminderVisibilityTimeout != 5*time.Minute {
		t.Fatalf("ReminderVisibilityTimeout = %v, want 5m", cfg.ReminderVisibilityTimeout)
	}
}

func TestLoad_ReminderDispatchFlagOff(t *testing.T) {
	t.Setenv("REMINDER_DISPATCH_ENABLED", "false")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ReminderDispatchEnabled {
		t.Fatalf("ReminderDispatchEnabled = true, want false (rollback switch)")
	}
}

func TestLoad_ReminderVisibilityTimeoutTooSmall(t *testing.T) {
	t.Setenv("REMINDER_VISIBILITY_TIMEOUT", "10s")
	if _, err := Load(); err == nil {
		t.Fatal("expected error for REMINDER_VISIBILITY_TIMEOUT below 30s minimum")
	}
}
