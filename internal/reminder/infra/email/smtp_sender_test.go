package email

import (
	"context"
	"strings"
	"testing"

	notificationapp "github.com/cobo/cobo_iam_services/internal/notification/app"
	notificationregistry "github.com/cobo/cobo_iam_services/internal/notification/infra/registry"
)

func TestRenderReminderEmailDisclosureDue(t *testing.T) {
	subject, body, err := renderReminderEmail("REMINDER_DISCLOSURE_DUE", map[string]any{
		"title":         "Annual disclosure report",
		"deadline_date": "2026-05-10",
		"disclosure_id": "disc-001",
		"status":        "draft",
		"action_url":    "/app/disclosures/disc-001",
	})
	if err != nil {
		t.Fatalf("renderReminderEmail error = %v", err)
	}
	if !strings.Contains(subject, "Annual disclosure report") || !strings.Contains(subject, "2026-05-10") {
		t.Fatalf("subject missing business fields: %q", subject)
	}
	for _, want := range []string{
		"Disclosure: Annual disclosure report",
		"Disclosure ID: disc-001",
		"Deadline: 2026-05-10",
		"Current status: draft",
		"Action link: /app/disclosures/disc-001",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "map[]") {
		t.Fatalf("body must not expose raw map payload: %s", body)
	}
}

func TestRenderReminderEmailDisclosureDueRequiresFields(t *testing.T) {
	if _, _, err := renderReminderEmail("REMINDER_DISCLOSURE_DUE", map[string]any{}); err == nil {
		t.Fatal("expected missing fields error")
	}
}

func TestSender_RenderReminderEmailContentEmbedMatchesLegacy(t *testing.T) {
	payload := map[string]any{
		"title":         "Annual disclosure report",
		"deadline_date": "2026-05-10",
		"disclosure_id": "disc-001",
		"status":        "draft",
		"action_url":    "/app/disclosures/disc-001",
	}
	legacySubject, legacyBody, err := renderReminderEmail("REMINDER_DISCLOSURE_DUE", payload)
	if err != nil {
		t.Fatalf("renderReminderEmail error = %v", err)
	}
	sender := NewSMTPSender(SMTPConfig{}, WithTemplateRendering("embed", notificationregistry.NewEmbedRegistry(), notificationapp.NewEmailRenderer()))

	subject, body, err := sender.renderReminderEmailContent("REMINDER_DISCLOSURE_DUE", payload)
	if err != nil {
		t.Fatalf("renderReminderEmailContent error = %v", err)
	}
	if subject != legacySubject {
		t.Fatalf("subject mismatch\nwant: %q\ngot:  %q", legacySubject, subject)
	}
	if body != legacyBody {
		t.Fatalf("body mismatch\nwant: %q\ngot:  %q", legacyBody, body)
	}
}

func TestSender_SendReminderEmailEmbedFallsBackToLegacy(t *testing.T) {
	sender := NewSMTPSender(SMTPConfig{}, WithTemplateRendering("embed", brokenRegistry{}, notificationapp.NewEmailRenderer()))

	msgID, err := sender.SendReminderEmail(context.Background(), "REMINDER_DISCLOSURE_DUE", map[string]any{
		"title":         "Annual disclosure report",
		"deadline_date": "2026-05-10",
		"disclosure_id": "disc-001",
	}, []string{"a@example.com"}, "idem-1")
	if err != nil {
		t.Fatalf("SendReminderEmail error = %v", err)
	}
	if msgID != "mock-no-smtp" {
		t.Fatalf("expected mock-no-smtp, got %q", msgID)
	}
}

type brokenRegistry struct{}

func (brokenRegistry) Resolve(context.Context, string, string) (notificationapp.ResolvedTemplate, error) {
	return notificationapp.ResolvedTemplate{}, context.DeadlineExceeded
}
