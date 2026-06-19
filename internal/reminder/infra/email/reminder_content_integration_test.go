package email

import (
	"context"
	"strings"
	"testing"

	notificationapp "github.com/cobo/cobo_iam_services/internal/notification/app"
	notificationregistry "github.com/cobo/cobo_iam_services/internal/notification/infra/registry"
)

// embedSender builds an SMTP sender wired to the real embed registry + real renderer
// (EMAIL_TEMPLATE_SOURCE=embed), no SMTP host → mock-send. This is the end-to-end render
// path that service.prepareDispatch feeds in production.
func embedSender() *Sender {
	return NewSMTPSender(SMTPConfig{}, WithTemplateRendering("embed",
		notificationregistry.NewEmbedRegistry(), notificationapp.NewEmailRenderer()))
}

// fullDeadlinePayload mirrors the payload service.prepareDispatch produces after Phase 1-4
// (8 required vars for reminder.deadline_approaching).
func fullDeadlinePayload() map[string]any {
	return map[string]any{
		"recipient_name":       "Phạm Thị Lan Hương",
		"urgency_status":       "Sắp đến hạn",
		"company_name":         "Công ty CP ABC",
		"disclosure_title":     "Báo cáo tài chính năm 2025",
		"due_date":             "31/03/2026",
		"remaining_days":       12,
		"implementation_guide": "Tổng hợp số liệu và nộp báo cáo qua hệ thống.",
		"portal_url":           "https://portal.cobo.vn/app/disclosures/disc-123",
	}
}

// fullWorkflowStepPayload mirrors prepareDispatch output for reminder.workflow_step_due
// (9 required vars; adds step_name).
func fullWorkflowStepPayload() map[string]any {
	return map[string]any{
		"recipient_name":       "Nguyễn Văn A",
		"urgency_status":       "Đã đến hạn",
		"company_name":         "Công ty CP ABC",
		"disclosure_title":     "Báo cáo tài chính năm 2025",
		"step_name":            "Soát xét tài chính",
		"due_date":             "28/03/2026",
		"remaining_days":       0,
		"implementation_guide": "Kiểm tra đối chiếu số liệu trước khi trình ký.",
		"portal_url":           "https://portal.cobo.vn/app/disclosures/disc-123",
	}
}

func assertNoForbidden(t *testing.T, subject, body string) {
	t.Helper()
	for _, bad := range []string{"<no value>", "<nil>", "{{", "}}", "map["} {
		if strings.Contains(subject, bad) || strings.Contains(body, bad) {
			t.Errorf("output contains forbidden token %q\nsubject=%q\nbody=%s", bad, subject, body)
		}
	}
}

func TestIntegration_RenderDeadlineApproaching_FullPayload(t *testing.T) {
	subject, body, _, err := embedSender().renderReminderEmailContent("reminder.deadline_approaching", fullDeadlinePayload())
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	for _, want := range []string{"[CẢNH BÁO]", "Sắp đến hạn", "Báo cáo tài chính năm 2025"} {
		if !strings.Contains(subject, want) {
			t.Errorf("subject missing %q: %q", want, subject)
		}
	}
	for _, want := range []string{
		"Kính gửi Phạm Thị Lan Hương,",
		"Công ty: Công ty CP ABC",
		"Công việc: Báo cáo tài chính năm 2025",
		"Số ngày còn lại: 12 ngày",
		"Hướng dẫn thực hiện:",
		"Tổng hợp số liệu và nộp báo cáo qua hệ thống.",
		"Xem thêm: https://portal.cobo.vn/app/disclosures/disc-123",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q:\n%s", want, body)
		}
	}
	assertNoForbidden(t, subject, body)
}

func TestIntegration_RenderWorkflowStepDue_FullPayload(t *testing.T) {
	subject, body, _, err := embedSender().renderReminderEmailContent("reminder.workflow_step_due", fullWorkflowStepPayload())
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	for _, want := range []string{"[CẢNH BÁO]", "Đã đến hạn", "Soát xét tài chính", "Báo cáo tài chính năm 2025"} {
		if !strings.Contains(subject, want) {
			t.Errorf("subject missing %q: %q", want, subject)
		}
	}
	for _, want := range []string{
		"Kính gửi Nguyễn Văn A,",
		"Công ty: Công ty CP ABC",
		"Bước cần xử lý: Soát xét tài chính",
		"Số ngày còn lại: 0 ngày",
		"Kiểm tra đối chiếu số liệu trước khi trình ký.",
		"Xem thêm: https://portal.cobo.vn/app/disclosures/disc-123",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q:\n%s", want, body)
		}
	}
	assertNoForbidden(t, subject, body)
}

// Missing a required var must FAIL the render (fail-loud contract): the embed render errors
// on the missing var and the pass-through key has no legacy fallback, so the call errors.
func TestIntegration_DeadlineApproaching_MissingRequiredVar_Errors(t *testing.T) {
	payload := fullDeadlinePayload()
	delete(payload, "implementation_guide") // required var removed
	if _, _, _, err := embedSender().renderReminderEmailContent("reminder.deadline_approaching", payload); err == nil {
		t.Fatal("expected render error when a required var (implementation_guide) is missing")
	}
}

// End-to-end through SendReminderEmail (recipient check + render + mock send) for both templates.
func TestIntegration_SendReminderEmail_BothTemplates_MockSend(t *testing.T) {
	cases := []struct {
		code    string
		payload map[string]any
	}{
		{"reminder.deadline_approaching", fullDeadlinePayload()},
		{"reminder.workflow_step_due", fullWorkflowStepPayload()},
	}
	for _, tc := range cases {
		msgID, err := embedSender().SendReminderEmail(context.Background(), tc.code, tc.payload, []string{"a@example.com"}, "idem-"+tc.code)
		if err != nil {
			t.Fatalf("%s: SendReminderEmail error = %v", tc.code, err)
		}
		if msgID != "mock-no-smtp" {
			t.Fatalf("%s: msgID = %q, want mock-no-smtp", tc.code, msgID)
		}
	}
}
