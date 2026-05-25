package app_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	notificationapp "github.com/cobo/cobo_iam_services/internal/notification/app"
	"github.com/cobo/cobo_iam_services/internal/notification/infra/inmemory"
	notificationregistry "github.com/cobo/cobo_iam_services/internal/notification/infra/registry"
)

// seqIDGen returns deterministic IDs so tests can assert specific values.
type seqIDGen struct{ n int }

func (g *seqIDGen) NewUUID() string {
	g.n++
	return fmt.Sprintf("notif-%03d", g.n)
}

func fixedClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

func newTestEmailService() (*notificationapp.EmailNotificationService, *inmemory.EmailNotificationRepository) {
	repo := inmemory.NewEmailNotificationRepository()
	svc := notificationapp.NewEmailNotificationService(
		repo,
		notificationregistry.NewEmbedRegistry(),
		notificationapp.NewEmailRenderer(),
		&seqIDGen{},
		fixedClock(time.Date(2026, 5, 22, 12, 0, 0, 0, time.UTC)),
	)
	return svc, repo
}

func validOTPRequest() notificationapp.DispatchEmailRequest {
	return notificationapp.DispatchEmailRequest{
		To:                  "nguyen@example.com",
		TemplateKey:         "auth.email_verification",
		Locale:              "vi",
		Variables: map[string]any{
			"otp_code": "123456", "expiry_minutes": 15,
			"support_email": "support@cobo.vn", "website_url": "https://app.example.com",
		},
		IdempotencyKey:      "auth.email_verification:u_123:otp_001",
		TriggeredByUserID:   "u_123",
		SourceEventType:     "auth.email_verification_requested",
		SourceAggregateType: "user",
		SourceAggregateID:   "u_123",
	}
}

func TestDispatchEmail_HappyPath(t *testing.T) {
	svc, repo := newTestEmailService()
	got, err := svc.DispatchEmail(context.Background(), validOTPRequest())
	if err != nil {
		t.Fatalf("DispatchEmail error = %v", err)
	}
	if got.EmailNotificationID != "notif-001" {
		t.Errorf("notification id = %q", got.EmailNotificationID)
	}
	if got.Status != notificationapp.EmailStatusPending {
		t.Errorf("status = %q", got.Status)
	}
	if got.Locale != "vi" {
		t.Errorf("locale = %q", got.Locale)
	}
	if got.RecipientEmail != "nguyen@example.com" {
		t.Errorf("recipient = %q", got.RecipientEmail)
	}
	// Sanitised JSON must NEVER contain the OTP code in plain text.
	if strings.Contains(got.VariablesJSONSanitized, "123456") {
		t.Fatalf("raw OTP leaked into sanitised JSON: %s", got.VariablesJSONSanitized)
	}
	if !strings.Contains(got.VariablesJSONSanitized, notificationapp.RedactedVarPlaceholder) {
		t.Fatalf("expected REDACTED placeholder, got %s", got.VariablesJSONSanitized)
	}
	// Repository must have exactly one row matching the returned ID.
	rows := repo.Snapshot()
	if len(rows) != 1 || rows[0].EmailNotificationID != got.EmailNotificationID {
		t.Fatalf("repo snapshot = %+v", rows)
	}
}

func TestDispatchEmail_DuplicateIdempotencyReturnsExisting(t *testing.T) {
	svc, repo := newTestEmailService()
	first, err := svc.DispatchEmail(context.Background(), validOTPRequest())
	if err != nil {
		t.Fatalf("first dispatch error = %v", err)
	}

	// Same idempotency key, different OTP — must return the SAME record and
	// MUST NOT create a duplicate row.
	second, err := svc.DispatchEmail(context.Background(), notificationapp.DispatchEmailRequest{
		To:             validOTPRequest().To,
		TemplateKey:    validOTPRequest().TemplateKey,
		Locale:         "vi",
		Variables: map[string]any{
			"otp_code": "999999", "expiry_minutes": 15,
			"support_email": "support@cobo.vn", "website_url": "https://app.example.com",
		},
		IdempotencyKey: validOTPRequest().IdempotencyKey,
	})
	if err != nil {
		t.Fatalf("second dispatch error = %v", err)
	}
	if second.EmailNotificationID != first.EmailNotificationID {
		t.Fatalf("expected identical notification ID, got %q vs %q", first.EmailNotificationID, second.EmailNotificationID)
	}
	if got := repo.Snapshot(); len(got) != 1 {
		t.Fatalf("expected exactly one row after duplicate dispatch, got %d", len(got))
	}
}

func TestDispatchEmail_ValidationFailures(t *testing.T) {
	svc, _ := newTestEmailService()
	base := validOTPRequest()

	cases := []struct {
		name  string
		mutate func(*notificationapp.DispatchEmailRequest)
		want  error
	}{
		{"empty To", func(r *notificationapp.DispatchEmailRequest) { r.To = "" }, notificationapp.ErrDispatchValidation},
		{"invalid email", func(r *notificationapp.DispatchEmailRequest) { r.To = "not-an-email" }, notificationapp.ErrDispatchValidation},
		{"empty template key", func(r *notificationapp.DispatchEmailRequest) { r.TemplateKey = "" }, notificationapp.ErrDispatchValidation},
		{"empty idempotency key", func(r *notificationapp.DispatchEmailRequest) { r.IdempotencyKey = "" }, notificationapp.ErrDispatchValidation},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := base
			tc.mutate(&req)
			_, err := svc.DispatchEmail(context.Background(), req)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want wrap of %v", err, tc.want)
			}
		})
	}
}

func TestDispatchEmail_TemplateNotResolved(t *testing.T) {
	svc, repo := newTestEmailService()
	req := validOTPRequest()
	req.TemplateKey = "does.not.exist"
	_, err := svc.DispatchEmail(context.Background(), req)
	if !errors.Is(err, notificationapp.ErrTemplateNotResolved) {
		t.Fatalf("err = %v, want ErrTemplateNotResolved", err)
	}
	if got := repo.Snapshot(); len(got) != 0 {
		t.Fatalf("repo must be empty when template missing, got %+v", got)
	}
}

func TestDispatchEmail_RenderFailureSkipsPersist(t *testing.T) {
	svc, repo := newTestEmailService()
	req := validOTPRequest()
	// Drop a required var so the renderer rejects the request.
	req.Variables = map[string]any{"full_name": "Nguyen Van A", "expiry_minutes": 15}
	_, err := svc.DispatchEmail(context.Background(), req)
	if !errors.Is(err, notificationapp.ErrTemplateRender) {
		t.Fatalf("err = %v, want ErrTemplateRender", err)
	}
	if got := repo.Snapshot(); len(got) != 0 {
		t.Fatalf("repo must stay empty on render failure, got %+v", got)
	}
}

func TestDispatchEmail_LocaleFallbackUsesDefault(t *testing.T) {
	svc, _ := newTestEmailService()
	req := validOTPRequest()
	req.Locale = "en" // no English template exists; registry falls back to vi
	got, err := svc.DispatchEmail(context.Background(), req)
	if err != nil {
		t.Fatalf("DispatchEmail error = %v", err)
	}
	if got.Locale != "vi" {
		t.Fatalf("expected locale fallback to vi, got %q", got.Locale)
	}
}

func TestDispatchEmail_SanitisesAllSensitiveVars(t *testing.T) {
	svc, repo := newTestEmailService()
	req := notificationapp.DispatchEmailRequest{
		To:             "user@example.com",
		TemplateKey:    "auth.user_invitation.new_user_company",
		Locale:         "vi",
		IdempotencyKey: "invite-001",
		Variables: map[string]any{
			"display_name": "Nguyen Van A",
			"company_name": "COBO",
			"setup_link":   "https://app/setup?token=abcdef",
			"raw_token":    "tok-xyz",
			"expiry_hours": 72,
		},
	}
	got, err := svc.DispatchEmail(context.Background(), req)
	if err != nil {
		t.Fatalf("DispatchEmail error = %v", err)
	}
	stored := got.VariablesJSONSanitized
	for _, leak := range []string{"abcdef", "tok-xyz", "token=abcdef"} {
		if strings.Contains(stored, leak) {
			t.Fatalf("sensitive value %q leaked into stored JSON: %s", leak, stored)
		}
	}
	if !strings.Contains(stored, "Nguyen Van A") {
		t.Fatalf("non-sensitive display_name missing from stored JSON: %s", stored)
	}
	if len(repo.Snapshot()) != 1 {
		t.Fatalf("expected exactly one row, got %d", len(repo.Snapshot()))
	}
}

func TestPreviewEmail_RendersWithoutPersisting(t *testing.T) {
	svc, repo := newTestEmailService()
	rendered, err := svc.PreviewEmail(context.Background(), "auth.email_verification", "vi", map[string]any{
		"otp_code":       "111111",
		"expiry_minutes": 15,
		"support_email":  "support@cobo.vn",
		"website_url":    "https://app.example.com",
	})
	if err != nil {
		t.Fatalf("PreviewEmail error = %v", err)
	}
	if rendered.Subject != "Mã xác thực đăng ký tài khoản CoBo Portal" {
		t.Fatalf("preview subject = %q", rendered.Subject)
	}
	if !strings.Contains(rendered.TextBody, "111111") {
		t.Fatalf("preview body missing OTP: %q", rendered.TextBody)
	}
	if got := repo.Snapshot(); len(got) != 0 {
		t.Fatalf("preview must not persist anything, repo = %+v", got)
	}
}

func TestPreviewEmail_RequiresTemplateKey(t *testing.T) {
	svc, _ := newTestEmailService()
	_, err := svc.PreviewEmail(context.Background(), "", "vi", nil)
	if !errors.Is(err, notificationapp.ErrDispatchValidation) {
		t.Fatalf("err = %v", err)
	}
}
