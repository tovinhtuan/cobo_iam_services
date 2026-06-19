package email

import (
	"context"
	"net/smtp"
	"strings"
	"testing"

	notificationapp "github.com/cobo/cobo_iam_services/internal/notification/app"
	notificationregistry "github.com/cobo/cobo_iam_services/internal/notification/infra/registry"
)

func wireCaptureSender(t *testing.T) (*Sender, *string) {
	t.Helper()
	var captured string
	sender := NewSMTPSender(SMTPConfig{
		Host: "smtp.example.com",
		Port: 587,
		From: "no-reply@cobo.local",
	}, WithTemplateRendering("embed", notificationregistry.NewEmbedRegistry(), notificationapp.NewEmailRenderer()))
	sender.sendMail = func(_ string, _ smtp.Auth, _ string, _ []string, msg []byte) error {
		captured = string(msg)
		return nil
	}
	return sender, &captured
}

func assertWireContains(t *testing.T, wire string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(wire, want) {
			t.Fatalf("wire payload missing %q\nfull:\n%s", want, wire)
		}
	}
}

// Test 1 — HTML reminder sends multipart/alternative on the wire.
func TestSendReminderEmail_HTMLReminderSendsMultipart(t *testing.T) {
	sender, wire := wireCaptureSender(t)
	_, err := sender.SendReminderEmail(context.Background(), "reminder.deadline_approaching", fullDeadlinePayload(), []string{"tvttthptlvh@gmail.com"}, "idem-html")
	if err != nil {
		t.Fatalf("SendReminderEmail error = %v", err)
	}
	assertWireContains(t, *wire,
		"MIME-Version: 1.0",
		"multipart/alternative",
		"Content-Type: text/plain",
		"Content-Type: text/html",
		"<table",
		"href=",
		"Xem thêm",
	)
	if strings.Contains(*wire, "Content-Type: text/plain; charset=UTF-8\r\n\r\n") {
		t.Fatalf("must not be single-part text/plain root envelope:\n%s", *wire)
	}
}

// Test 2 — legacy/fallback path still sends text/plain only.
func TestSendReminderEmail_LegacyFallbackSendsPlainText(t *testing.T) {
	var captured string
	sender := NewSMTPSender(SMTPConfig{
		Host: "smtp.example.com",
		Port: 587,
		From: "no-reply@cobo.local",
	})
	sender.sendMail = func(_ string, _ smtp.Auth, _ string, _ []string, msg []byte) error {
		captured = string(msg)
		return nil
	}
	_, err := sender.SendReminderEmail(context.Background(), "REMINDER_DISCLOSURE_DUE", map[string]any{
		"disclosure_title": "Báo cáo tài chính quý 2/2026",
		"due_date":         "15/06/2026",
		"company_name":     "Công ty Cổ phần ABC",
		"portal_url":       "https://portal.cobo.vn/app/disclosures/disc-001",
	}, []string{"a@example.com"}, "idem-legacy")
	if err != nil {
		t.Fatalf("SendReminderEmail error = %v", err)
	}
	assertWireContains(t, captured, "Content-Type: text/plain; charset=UTF-8")
	if strings.Contains(captured, "multipart/alternative") {
		t.Fatalf("legacy path must not send multipart:\n%s", captured)
	}
	if strings.Contains(captured, "text/html") {
		t.Fatalf("legacy path must not send html part:\n%s", captured)
	}
}

// Test 3 — rendered HTMLBody from template appears in captured SMTP data, not only in function return.
func TestSendReminderEmail_HTMLBodyOnWire(t *testing.T) {
	sender, wire := wireCaptureSender(t)
	_, _, htmlBody, err := sender.renderReminderEmailContent("reminder.workflow_step_due", fullWorkflowStepPayload())
	if err != nil {
		t.Fatalf("renderReminderEmailContent error = %v", err)
	}
	if strings.TrimSpace(htmlBody) == "" {
		t.Fatal("expected non-empty htmlBody from renderer")
	}
	if !strings.Contains(htmlBody, "<table") {
		t.Fatalf("renderer htmlBody missing table:\n%s", htmlBody)
	}

	_, err = sender.SendReminderEmail(context.Background(), "reminder.workflow_step_due", fullWorkflowStepPayload(), []string{"a@example.com"}, "idem-wire-html")
	if err != nil {
		t.Fatalf("SendReminderEmail error = %v", err)
	}
	for _, frag := range []string{"<table", "Bước cần xử lý", "Xem thêm", "href=\"https://portal.cobo.vn/app/disclosures/disc-123\""} {
		if !strings.Contains(htmlBody, frag) {
			t.Fatalf("renderer htmlBody missing %q", frag)
		}
		if !strings.Contains(*wire, frag) {
			t.Fatalf("wire payload missing html fragment %q from rendered HTMLBody", frag)
		}
	}
}
