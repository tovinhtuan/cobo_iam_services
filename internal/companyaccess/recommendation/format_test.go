package recommendation_test

import (
	"testing"
	"time"

	"github.com/cobo/cobo_iam_services/internal/companyaccess/recommendation"
)

var fixedAt = time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)

func TestFormatKnownCode(t *testing.T) {
	items := recommendation.Format([]recommendation.CheckInput{{
		Code:        "conflict.notification.prefs_invalid",
		Severity:    "blocking",
		Title:       "Server title",
		Description: "Payload invalid",
		ActionLink:  "/app/admin?tab=notifications",
		Evidence:    map[string]any{"rule_id": "r1"},
	}}, fixedAt)
	if len(items) != 1 {
		t.Fatalf("len=%d", len(items))
	}
	item := items[0]
	if item.SourceCode != "conflict.notification.prefs_invalid" {
		t.Fatalf("source_code=%q", item.SourceCode)
	}
	if item.Priority != recommendation.PriorityHigh {
		t.Fatalf("priority=%q", item.Priority)
	}
	if item.Title != "Kiểm tra lại cấu hình kênh cảnh báo" {
		t.Fatalf("title=%q", item.Title)
	}
	if item.ActionLabel != "Mở kênh nhận cảnh báo" {
		t.Fatalf("action_label=%q", item.ActionLabel)
	}
	if item.ID != "rec.configuration_health.conflict.notification.prefs_invalid" {
		t.Fatalf("id=%q", item.ID)
	}
}

func TestFormatUnknownCodeFallback(t *testing.T) {
	items := recommendation.Format([]recommendation.CheckInput{{
		Code:        "custom.unknown_check",
		Severity:    "warning",
		Description: "Need review",
		ActionLink:  "/app/admin?tab=staff",
	}}, fixedAt)
	if len(items) != 1 {
		t.Fatal("expected one item")
	}
	if items[0].Priority != recommendation.PriorityMedium {
		t.Fatalf("priority=%q", items[0].Priority)
	}
	if items[0].ActionLink != "/app/admin?tab=staff" {
		t.Fatalf("action_link=%q", items[0].ActionLink)
	}
	if items[0].Title != "Xem chi tiết cấu hình liên quan" {
		t.Fatalf("title=%q", items[0].Title)
	}
}

func TestSeverityToPriority(t *testing.T) {
	if recommendation.SeverityToPriority("blocking") != recommendation.PriorityHigh {
		t.Fatal("blocking")
	}
	if recommendation.SeverityToPriority("warning") != recommendation.PriorityMedium {
		t.Fatal("warning")
	}
	if recommendation.SeverityToPriority("info") != recommendation.PriorityLow {
		t.Fatal("info")
	}
}

func TestFormatEmptyChecks(t *testing.T) {
	items := recommendation.Format(nil, fixedAt)
	if len(items) != 0 {
		t.Fatalf("len=%d want 0", len(items))
	}
}

func TestEvidenceSanitized(t *testing.T) {
	items := recommendation.Format([]recommendation.CheckInput{{
		Code:     "custom.x",
		Severity: "info",
		Evidence: map[string]any{"token": "secret", "rule_id": "r1"},
	}}, fixedAt)
	if _, ok := items[0].Evidence["token"]; ok {
		t.Fatal("token should be stripped")
	}
	if items[0].Evidence["rule_id"] != "r1" {
		t.Fatal("rule_id should remain")
	}
}

func TestActionLinkDefaultWhenMissing(t *testing.T) {
	items := recommendation.Format([]recommendation.CheckInput{{
		Code:     "custom.x",
		Severity: "info",
	}}, fixedAt)
	if items[0].ActionLink != "/app/admin" {
		t.Fatalf("action_link=%q", items[0].ActionLink)
	}
}

func TestSourceIsConfigurationHealth(t *testing.T) {
	items := recommendation.Format([]recommendation.CheckInput{{
		Code: "notification.storage_not_configured", Severity: "info",
	}}, fixedAt)
	if items[0].Source != recommendation.SourceConfigurationHealth {
		t.Fatalf("source=%q", items[0].Source)
	}
}
