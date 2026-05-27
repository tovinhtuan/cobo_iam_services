package config

import (
	"testing"
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
