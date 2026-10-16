package app

import (
	"strings"
	"testing"
)

// PO decision Q1-B (16/10/2026, OQ-006): an obligation stays open after its template is archived.
// These tests replace the 18/08 "hide archived templates" contract.

func TestTemplateArchived(t *testing.T) {
	cases := []struct {
		name                          string
		status, companyID, reviewStat string
		want                          bool
	}{
		{"active global template", "active", "", "", false},
		{"archived global template (CMS)", "archived", "", "", true},
		{"never activated or restored to draft (status active, version 0)", "active", "", "draft", false},
		{"published company template", "active", "c1", "published", false},
		{"archived company template (lifecycle)", "active", "c1", "archived", true},
		{"review_status of a global template is not its archive state", "active", "", "archived", false},
	}
	for _, tc := range cases {
		if got := TemplateArchived(tc.status, tc.companyID, tc.reviewStat); got != tc.want {
			t.Fatalf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestObligationTemplateSQLJoin_neverFilters(t *testing.T) {
	join := ObligationTemplateSQLJoin
	if strings.Contains(join, "INNER JOIN") {
		t.Fatal("the template join must not drop records: an archived template's obligations stay open")
	}
	if strings.Contains(join, "active_version_no > 0") {
		t.Fatal("template state must not filter obligations")
	}
	if !strings.Contains(join, "LEFT JOIN disclosure_types dt") || !strings.Contains(join, "COALESCE(NULLIF(dt.active_version_no, 0), dt.archived_from_version_no)") {
		t.Fatal("must join the active version, else the version the template was archived from")
	}
	lower := strings.ToLower(join)
	if strings.Contains(lower, "created_at") || strings.Contains(lower, "company_id is null") {
		t.Fatal("must not filter by record age or keep only global templates")
	}
}

func TestObligationOpenSQL_isRecordStateOnly(t *testing.T) {
	sql := ObligationOpenSQL
	for _, want := range []string{"LOWER(TRIM(dr.status)) = 'draft'", "dr.submitted_at IS NULL", "COALESCE(pc.open_at, pc.cycle_start) <= ?"} {
		if !strings.Contains(sql, want) {
			t.Fatalf("ObligationOpenSQL lost %q", want)
		}
	}
	if strings.Contains(sql, "dt.") || strings.Contains(sql, "disclosure_types") {
		t.Fatal("ObligationOpen must not depend on the template's state")
	}
	if strings.Count(sql, "?") != 1 {
		t.Fatal("ObligationOpenSQL binds exactly one argument (todayHCM)")
	}
	// AC-11 (PO Q-12 a′): an approved ad-hoc stays open after finalize auto-submits its record.
	for _, want := range []string{"ad_hoc_proposals", "ahp.status = 'approved'", "NOT IN ('approved', 'published', 'completed', 'done')"} {
		if !strings.Contains(sql, want) {
			t.Fatalf("ObligationOpenSQL lost the ad-hoc branch (%q)", want)
		}
	}
}

func TestTemplateArchivedSQL_matchesGoTwin(t *testing.T) {
	if !strings.Contains(TemplateArchivedSQL, "dt.status") || !strings.Contains(TemplateArchivedSQL, "dt.review_status") || !strings.Contains(TemplateArchivedSQL, "dt.company_id IS NOT NULL") {
		t.Fatal("TemplateArchivedSQL drifted from TemplateArchived")
	}
	if strings.Contains(TemplateArchivedSQL, "active_version_no") {
		t.Fatal("active_version_no = 0 is not archived (never activated / restored to draft)")
	}
}
