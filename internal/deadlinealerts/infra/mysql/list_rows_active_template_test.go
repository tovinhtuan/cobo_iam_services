package mysql

import (
	"os"
	"strings"
	"testing"

	deadlinealertsapp "github.com/cobo/cobo_iam_services/internal/deadlinealerts/app"
)

// PO decision Q1-B (16/10/2026): open obligations of an archived template stay in the alerts tab.
func TestListRows_keepsObligationsOfArchivedTemplates(t *testing.T) {
	src := readDeadlineAlertsRepositorySrc(t)
	if !strings.Contains(src, "ObligationTemplateSQLJoin") || listRowsV1ObligationMembershipSQL != deadlinealertsapp.ObligationOpenSQL {
		t.Fatal("ListRows must use the shared ObligationOpen predicate and display-only template join")
	}
	if strings.Contains(src, "INNER JOIN disclosure_types dt ON dt.type_id = dr.type_id") {
		t.Fatal("ListRows must not INNER JOIN the template: that hides obligations of archived templates")
	}
	if !strings.Contains(src, "TemplateArchivedSQL") {
		t.Fatal("ListRows must report template_archived")
	}
	if !strings.Contains(src, "WHERE dr.company_id = ?") {
		t.Fatal("T9 tenant isolation: ListRows must filter requesting company")
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
