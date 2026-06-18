package email

import (
	"context"
	"strings"
	"testing"

	notificationapp "github.com/cobo/cobo_iam_services/internal/notification/app"
	notificationregistry "github.com/cobo/cobo_iam_services/internal/notification/infra/registry"
)

func TestRenderReminderEmailDisclosureDue(t *testing.T) {
	// Augmented payload (as produced by service.prepareDispatch): human-readable
	// disclosure_title + dd/mm/yyyy due_date + company_name + absolute portal_url.
	subject, body, err := renderReminderEmail("REMINDER_DISCLOSURE_DUE", map[string]any{
		"disclosure_title": "Báo cáo tài chính quý 2/2026",
		"due_date":         "15/06/2026",
		"company_name":     "Công ty Cổ phần ABC",
		"portal_url":       "https://portal.cobo.vn/app/disclosures/disc-001",
		// Legacy keys must be IGNORED by the rendered output. Distinct sentinel
		// values (not the URL slug) make the leak assertion meaningful.
		"disclosure_id": "RAW-UUID-LEAK-SENTINEL",
		"status":        "RAW-STATUS-LEAK-SENTINEL",
	})
	if err != nil {
		t.Fatalf("renderReminderEmail error = %v", err)
	}
	if !strings.Contains(subject, "Báo cáo tài chính quý 2/2026") || !strings.Contains(subject, "15/06/2026") {
		t.Fatalf("subject missing business fields: %q", subject)
	}
	for _, want := range []string{
		"Kính gửi,",
		"Công ty: Công ty Cổ phần ABC",
		"Nghĩa vụ công bố: Báo cáo tài chính quý 2/2026",
		"Hạn nộp: 15/06/2026",
		"https://portal.cobo.vn/app/disclosures/disc-001",
		"Hệ thống CoBo Portal",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q:\n%s", want, body)
		}
	}
	// No English wording, no UUID/enum exposure, no raw payload leak.
	for _, forbidden := range []string{"RAW-UUID-LEAK-SENTINEL", "RAW-STATUS-LEAK-SENTINEL", "Disclosure ID", "Current status", "is due on", "automated reminder", "map[]", "<no value>"} {
		if strings.Contains(body, forbidden) || strings.Contains(subject, forbidden) {
			t.Fatalf("output must not contain %q:\nsubject=%q\nbody=%s", forbidden, subject, body)
		}
	}
}

func TestRenderReminderEmailDisclosureDueRequiresFields(t *testing.T) {
	if _, _, err := renderReminderEmail("REMINDER_DISCLOSURE_DUE", map[string]any{}); err == nil {
		t.Fatal("expected missing fields error")
	}
}

// TestSender_RenderReminderEmailContentEmbedVietnamese verifies the embed path
// (EMAIL_TEMPLATE_SOURCE=embed) renders the disclosure-deadline reminder as a
// Vietnamese business email with no English wording, UUID, or enum exposure,
// when given the augmented payload that service.prepareDispatch produces.
func TestSender_RenderReminderEmailContentEmbedVietnamese(t *testing.T) {
	payload := map[string]any{
		"disclosure_title": "Báo cáo tài chính quý 2/2026",
		"company_name":     "Công ty Cổ phần ABC",
		"due_date":         "15/06/2026",
		"portal_url":       "https://portal.cobo.vn/app/disclosures/disc-001",
	}
	sender := NewSMTPSender(SMTPConfig{}, WithTemplateRendering("embed", notificationregistry.NewEmbedRegistry(), notificationapp.NewEmailRenderer()))

	subject, body, err := sender.renderReminderEmailContent("REMINDER_DISCLOSURE_DUE", payload)
	if err != nil {
		t.Fatalf("renderReminderEmailContent error = %v", err)
	}
	for _, want := range []string{"Nghĩa vụ công bố: Báo cáo tài chính quý 2/2026", "Hạn nộp: 15/06/2026", "https://portal.cobo.vn/app/disclosures/disc-001"} {
		if !strings.Contains(body, want) {
			t.Fatalf("embed body missing %q:\n%s", want, body)
		}
	}
	if !strings.Contains(subject, "Báo cáo tài chính quý 2/2026") {
		t.Fatalf("embed subject missing disclosure title: %q", subject)
	}
	for _, forbidden := range []string{"Disclosure ID", "Current status", "is due on", "<no value>"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("embed body must not contain %q:\n%s", forbidden, body)
		}
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

func TestReminderTemplateKey_LegacyCodeMapsToDeadline(t *testing.T) {
	key, ok := reminderTemplateKey("REMINDER_DISCLOSURE_DUE")
	if !ok {
		t.Fatal("expected ok=true for legacy code")
	}
	if key != "reminder.disclosure_deadline" {
		t.Errorf("key = %q, want reminder.disclosure_deadline", key)
	}
}

func TestReminderTemplateKey_DeadlineApproachingPassThrough(t *testing.T) {
	key, ok := reminderTemplateKey("reminder.deadline_approaching")
	if !ok {
		t.Fatal("expected ok=true for pass-through key")
	}
	if key != "reminder.deadline_approaching" {
		t.Errorf("key = %q, want reminder.deadline_approaching", key)
	}
}

func TestReminderTemplateKey_WorkflowStepDuePassThrough(t *testing.T) {
	key, ok := reminderTemplateKey("reminder.workflow_step_due")
	if !ok {
		t.Fatal("expected ok=true for pass-through key")
	}
	if key != "reminder.workflow_step_due" {
		t.Errorf("key = %q, want reminder.workflow_step_due", key)
	}
}

func TestReminderTemplateKey_EmptyReturnsFalse(t *testing.T) {
	_, ok := reminderTemplateKey("")
	if ok {
		t.Error("expected ok=false for empty template code")
	}
}

func TestSender_PassThroughKey_UsesEmbedRegistry(t *testing.T) {
	// reminder.deadline_approaching exists in embed FS → should render without error.
	sender := NewSMTPSender(SMTPConfig{}, WithTemplateRendering("embed",
		notificationregistry.NewEmbedRegistry(), notificationapp.NewEmailRenderer()))

	msgID, err := sender.SendReminderEmail(context.Background(), "reminder.deadline_approaching", map[string]any{
		"recipient_name":       "Phạm Thị Lan Hương",
		"urgency_status":       "Sắp đến hạn",
		"disclosure_title":     "Báo cáo tài chính",
		"company_name":         "ACME Co.",
		"due_date":             "15/06/2026",
		"remaining_days":       5,
		"implementation_guide": "Tổng hợp số liệu và nộp báo cáo.",
		"portal_url":           "http://localhost:3000/app/disclosures/d1",
	}, []string{"a@example.com"}, "idem-1")
	if err != nil {
		t.Fatalf("SendReminderEmail error = %v", err)
	}
	if msgID != "mock-no-smtp" {
		t.Fatalf("expected mock-no-smtp, got %q", msgID)
	}
}
