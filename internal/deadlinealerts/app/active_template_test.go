package app

import (
	"strings"
	"testing"
)

func TestTemplateCurrentlyActive_T1_oldRecordActiveTemplate(t *testing.T) {
	// created_at is not an input — age must not affect visibility.
	if !TemplateCurrentlyActive("bao-cao-tai-chinh-quy-1", 1, true) {
		t.Fatal("old record whose template root is currently active must be visible")
	}
}

func TestTemplateCurrentlyActive_T2_newRecordInactiveTemplate(t *testing.T) {
	if TemplateCurrentlyActive("qa-workflow-reminder-dc7-20260817", 0, false) {
		t.Fatal("new record whose template is archived/inactive must be hidden")
	}
}

func TestTemplateCurrentlyActive_T3_activeGlobalTemplate(t *testing.T) {
	if !TemplateCurrentlyActive("bao-cao-tai-chinh-quy-1", 1, true) {
		t.Fatal("active global template must be visible")
	}
}

func TestTemplateCurrentlyActive_T4_archivedGlobalTemplate(t *testing.T) {
	if TemplateCurrentlyActive("bao-cao-su-co-he-thong", 0, false) {
		t.Fatal("archived global template must be hidden")
	}
}

func TestTemplateCurrentlyActive_T5_activeCompanyScopedTemplate(t *testing.T) {
	if !TemplateCurrentlyActive("dt-co-1781130158761665600", 1, true) {
		t.Fatal("active company-scoped template must be visible")
	}
}

func TestTemplateCurrentlyActive_T6_inactiveCompanyScopedTemplate(t *testing.T) {
	if TemplateCurrentlyActive("dt-co-archived-co", 0, false) {
		t.Fatal("inactive company-scoped template must be hidden")
	}
}

func TestTemplateCurrentlyActive_T7_historicalVersionUnderActiveRoot(t *testing.T) {
	// Visibility uses current root active_version_no, not the version the record was created on.
	if !TemplateCurrentlyActive("type-root", 3, true) {
		t.Fatal("record created under v1 while root now has active v3 must stay visible")
	}
}

func TestTemplateCurrentlyActive_T10_missingTypeHidden(t *testing.T) {
	if TemplateCurrentlyActive("", 1, true) {
		t.Fatal("missing type_id must not show in deadline-alert projection")
	}
	if TemplateCurrentlyActive("   ", 5, true) {
		t.Fatal("blank type_id must not show")
	}
}

func TestListRowsActiveTemplateSQLJoin_noDateCutoffAndNoGlobalOnly(t *testing.T) {
	sql := ListRowsActiveTemplateSQLJoin
	if !strings.Contains(sql, "INNER JOIN disclosure_types dt") {
		t.Fatal("must INNER JOIN disclosure_types (not LEFT JOIN) so inactive roots drop out")
	}
	if !strings.Contains(sql, "dt.active_version_no > 0") {
		t.Fatal("must use active_version_no > 0 (archive/CMS authority)")
	}
	if !strings.Contains(sql, "dtv.version_no = dt.active_version_no") {
		t.Fatal("must join current active version row like Portal ListTypes")
	}
	lower := strings.ToLower(sql)
	if strings.Contains(lower, "created_at") {
		t.Fatal("FAIL_DEADLINE_ALERT_DATE_COUPLING_REMAINS: join must not filter created_at")
	}
	if strings.Contains(lower, "company_id is null") {
		t.Fatal("FAIL_DEADLINE_ALERT_COMPANY_TEMPLATE_FILTER: must not keep only global templates")
	}
}

func TestTemplateCurrentlyActive_doesNotUseCreatedAt(t *testing.T) {
	// Same template state → same visibility regardless of record age (T1 vs T2 contrast).
	oldActive := TemplateCurrentlyActive("t-active", 1, true)
	newActive := TemplateCurrentlyActive("t-active", 1, true)
	oldArchived := TemplateCurrentlyActive("t-arch", 0, false)
	newArchived := TemplateCurrentlyActive("t-arch", 0, false)
	if !oldActive || !newActive {
		t.Fatal("active template must be visible for both old and new records")
	}
	if oldArchived || newArchived {
		t.Fatal("inactive template must be hidden for both old and new records")
	}
}
