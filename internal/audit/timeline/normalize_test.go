package timeline_test

import (
	"strings"
	"testing"

	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
	"github.com/cobo/cobo_iam_services/internal/audit/timeline"
)

func TestNormalizeKnownAction(t *testing.T) {
	ev := timeline.Normalize(auditapp.Entry{
		EventID:    "e1",
		OccurredAt: "2026-06-30T12:00:00Z",
		Action:     "admin.department.create",
		ActorUserID: "u1",
		ResourceType: "department",
		ResourceID:   "d1",
	})
	if ev.Summary != "Tạo phòng ban" || ev.Domain != "org" {
		t.Fatalf("unexpected: %+v", ev)
	}
	if ev.Source != timeline.SourceAuditLogV1 {
		t.Fatalf("source: %s", ev.Source)
	}
}

func TestNormalizeUnknownAction(t *testing.T) {
	ev := timeline.Normalize(auditapp.Entry{Action: "admin.custom.thing"})
	if ev.Summary != "Hoạt động hệ thống" || ev.Category != "configuration_change" {
		t.Fatalf("unexpected: %+v", ev)
	}
	if ev.Actor.Display != "" {
		t.Fatalf("actor display must not mirror raw user id, got %q", ev.Actor.Display)
	}
}

func TestSummaryForAuthAndCmsActions(t *testing.T) {
	if got := timeline.SummaryForAction("login_success"); got != "Đăng nhập thành công" {
		t.Fatalf("login_success: %q", got)
	}
	if got := timeline.SummaryForAction("select_company"); got != "Chuyển công ty đang làm việc" {
		t.Fatalf("select_company: %q", got)
	}
	if got := timeline.SummaryForAction("cms_workflow.upsert"); got != "Cập nhật workflow mẫu" {
		t.Fatalf("cms_workflow.upsert: %q", got)
	}
	desc := timeline.FriendlyDescription("cms_workflow.upsert", "disclosure_type")
	if !strings.Contains(desc, "workflow") || strings.Contains(desc, "disclosure_type") {
		t.Fatalf("friendly desc should be humanized, got %q", desc)
	}
}

func TestSanitizeMetadataStripsSecrets(t *testing.T) {
	out := timeline.SanitizeMetadata(map[string]any{
		"password":        "x",
		"permission_code": "rbac.manage",
		"token":           "secret",
	})
	if len(out) != 1 {
		t.Fatalf("metadata: %+v", out)
	}
}
