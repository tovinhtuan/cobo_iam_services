package email

import (
	"strings"
	"testing"
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
