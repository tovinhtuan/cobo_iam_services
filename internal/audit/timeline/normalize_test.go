package timeline_test

import (
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
	if ev.Summary != "Thay đổi cấu hình" || ev.Category != "configuration_change" {
		t.Fatalf("unexpected: %+v", ev)
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
