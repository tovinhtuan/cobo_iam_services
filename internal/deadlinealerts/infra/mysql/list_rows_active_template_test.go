package mysql

import (
	"os"
	"strings"
	"testing"

	deadlinealertsapp "github.com/cobo/cobo_iam_services/internal/deadlinealerts/app"
)

func TestListRows_usesActiveTemplateInnerJoin(t *testing.T) {
	src := readDeadlineAlertsRepositorySrc(t)
	if !strings.Contains(src, "ListRowsActiveTemplateSQLJoin") {
		t.Fatal("ListRows must use ListRowsActiveTemplateSQLJoin")
	}
	if strings.Contains(src, "LEFT JOIN disclosure_types dt") {
		t.Fatal("ListRows must not LEFT JOIN disclosure_types — inactive/missing templates would leak into the alert tab")
	}
	if !strings.Contains(src, "WHERE dr.company_id = ?") {
		t.Fatal("T9 tenant isolation: ListRows must filter requesting company")
	}
	join := deadlinealertsapp.ListRowsActiveTemplateSQLJoin
	if !strings.Contains(join, "INNER JOIN disclosure_types dt") || !strings.Contains(join, "dt.active_version_no > 0") {
		t.Fatal("active-template join fragment drifted")
	}
	lower := strings.ToLower(src)
	if strings.Contains(lower, "created_at >=") || strings.Contains(lower, "created_at <") || strings.Contains(src, "2026-08-17") {
		t.Fatal("FAIL_DEADLINE_ALERT_DATE_COUPLING_REMAINS")
	}
}

func readDeadlineAlertsRepositorySrc(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatalf("read repository.go: %v", err)
	}
	return string(data)
}
