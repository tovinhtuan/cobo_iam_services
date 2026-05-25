package app_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	notificationapp "github.com/cobo/cobo_iam_services/internal/notification/app"
	notificationregistry "github.com/cobo/cobo_iam_services/internal/notification/infra/registry"
)

// GoldenFlow is the phase-0 baseline contract for a single email type:
// the template key, the canonical sample vars, and the path stem under
// testdata/email-golden/. Tests in this package and in iam/reminder share
// this list so a new template gets validated everywhere at once.
type GoldenFlow struct {
	Name      string
	Key       string
	Vars      map[string]any
	Fixture   string // stem used for {stem}.subject.txt and {stem}.body.txt
}

// GoldenFlows is the phase-0 source of truth for the six email baselines.
// Changing rendered output for any of these is a behaviour change that must
// update both the template files and the golden fixtures together.
var GoldenFlows = []GoldenFlow{
	{
		Name:    "email verification",
		Key:     "auth.email_verification",
		Vars: map[string]any{
			"otp_code": "123456", "expiry_minutes": 15,
			"support_email": "support@cobo.vn", "website_url": "https://app.example.com",
		},
		Fixture: "auth.email_verification",
	},
	{
		Name:    "user password reset",
		Key:     "auth.password_reset.user",
		Vars: map[string]any{
			"full_name": "Nguyen Van A", "reset_link": "https://app/reset", "expiry_minutes": 30,
			"support_email": "support@cobo.vn", "website_url": "https://app.example.com",
		},
		Fixture: "auth.password_reset.user",
	},
	{
		Name:    "admin password reset",
		Key:     "auth.password_reset.admin",
		Vars: map[string]any{
			"full_name": "Nguyen Van A", "reset_link": "https://app/reset", "expiry_minutes": 30,
			"support_email": "support@cobo.vn", "website_url": "https://app.example.com",
		},
		Fixture: "auth.password_reset.admin",
	},
	{
		Name: "new user invitation with company",
		Key:  "auth.user_invitation.new_user_company",
		Vars: map[string]any{
			"display_name": "Tô Vinh Tuấn", "company_name": "Company X",
			"setup_link": "https://app/invite", "expiry_hours": 72,
			"support_email": "support@cobo.vn", "website_url": "https://app.example.com",
		},
		Fixture: "auth.user_invitation.new_user",
	},
	{
		Name: "new user invitation no company",
		Key:  "auth.user_invitation.new_user_no_company",
		Vars: map[string]any{
			"display_name": "Nguyen Van A",
			"setup_link": "https://app/invite", "expiry_hours": 72,
			"support_email": "support@cobo.vn", "website_url": "https://app.example.com",
		},
		Fixture: "auth.user_invitation.new_user_no_company",
	},
	{
		Name:    "existing user invitation",
		Key:     "auth.user_invitation.existing_user",
		Vars:    map[string]any{"display_name": "Nguyen Van A", "company_name": "COBO"},
		Fixture: "auth.user_invitation.existing_user",
	},
	{
		Name: "reminder disclosure deadline",
		Key:  "reminder.disclosure_deadline",
		Vars: map[string]any{
			"title":         "Annual disclosure report",
			"deadline_date": "2026-05-10",
			"disclosure_id": "disc-001",
			"status":        "draft",
			"action_url":    "/app/disclosures/disc-001",
		},
		Fixture: "reminder.disclosure_deadline",
	},
}

// LoadGolden returns the canonical rendered subject/body bytes for a fixture.
// goldenDir is the absolute path to internal/notification/app/testdata/email-golden.
// CRLF in fixture files is normalised to LF so checked-in files behave the same
// regardless of which OS wrote them.
func LoadGolden(t *testing.T, goldenDir, fixture string) (string, string) {
	t.Helper()
	subj, err := os.ReadFile(filepath.Join(goldenDir, fixture+".subject.txt"))
	if err != nil {
		t.Fatalf("read golden subject %s: %v", fixture, err)
	}
	body, err := os.ReadFile(filepath.Join(goldenDir, fixture+".body.txt"))
	if err != nil {
		t.Fatalf("read golden body %s: %v", fixture, err)
	}
	return strings.ReplaceAll(string(subj), "\r\n", "\n"), strings.ReplaceAll(string(body), "\r\n", "\n")
}

func TestEmailRenderer_GoldenOutputs(t *testing.T) {
	registry := notificationregistry.NewEmbedRegistry()
	renderer := notificationapp.NewEmailRenderer()
	goldenDir := filepath.Join("testdata", "email-golden")

	for _, tt := range GoldenFlows {
		t.Run(tt.Name, func(t *testing.T) {
			wantSubj, wantBody := LoadGolden(t, goldenDir, tt.Fixture)
			resolved, err := registry.Resolve(context.Background(), tt.Key, "vi")
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			rendered, err := renderer.Render(resolved, tt.Vars)
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}
			if rendered.Subject != wantSubj {
				t.Fatalf("subject mismatch\nwant: %q\ngot:  %q", wantSubj, rendered.Subject)
			}
			if rendered.TextBody != wantBody {
				t.Fatalf("body mismatch\nwant: %q\ngot:  %q", wantBody, rendered.TextBody)
			}
		})
	}
}

func TestEmailRenderer_MissingRequiredVar(t *testing.T) {
	registry := notificationregistry.NewEmbedRegistry()
	renderer := notificationapp.NewEmailRenderer()

	resolved, err := registry.Resolve(context.Background(), "auth.email_verification", "vi")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	_, err = renderer.Render(resolved, map[string]any{"full_name": "Nguyen Van A", "expiry_minutes": 15})
	if err == nil || !strings.Contains(err.Error(), "otp_code") {
		t.Fatalf("expected missing otp_code error, got %v", err)
	}
}
