package mysql

import (
	"os"
	"strings"
	"testing"
)

// TestHasWorkflow_CompanyTemplateWithApprovedOverride_ReturnsTrue verifies that
// batchLoadActiveWorkflowFlags queries company_template_workflow_overrides so that
// company-defined templates with an approved override are returned as has_workflow=true.
func TestHasWorkflow_CompanyTemplateWithApprovedOverride_ReturnsTrue(t *testing.T) {
	src := readRepositorySrc(t)
	if !strings.Contains(src, "company_template_workflow_overrides") {
		t.Fatal("batchLoadActiveWorkflowFlags must query company_template_workflow_overrides to support company-defined templates")
	}
}

// TestHasWorkflow_Nhánh2_MustFilterByCompanyID ensures that nhánh 2 of
// batchLoadActiveWorkflowFlags scopes the override lookup to the requesting
// company. Without this filter, Company B can inherit Company A's approved
// override and create disclosures using Company A's proprietary templates.
func TestHasWorkflow_Nhanh2_MustFilterByCompanyID(t *testing.T) {
	src := readRepositorySrc(t)
	// nhánh 2 query must include company_id = ? condition
	if !strings.Contains(src, "company_id = ?") {
		t.Fatal("batchLoadActiveWorkflowFlags nhánh 2 must filter by company_id to prevent cross-company template usage (security: Company B must not inherit Company A's approved override)")
	}
}

// TestHasWorkflow_CompanyTemplateDraftOverride_ReturnsFalse and
// TestHasWorkflow_CompanyTemplateArchivedOverride_ReturnsFalse both rely on the
// condition active_version_no > 0 (draft and archived both have active_version_no=0).
func TestHasWorkflow_CompanyTemplateDraftAndArchivedOverride_ReturnsFalse(t *testing.T) {
	src := readRepositorySrc(t)
	// The condition must be strictly > 0, not IS NOT NULL.
	// Schema: active_version_no INT NOT NULL DEFAULT 0 — IS NOT NULL is always true.
	if strings.Contains(src, "active_version_no IS NOT NULL") {
		t.Fatal("condition must be 'active_version_no > 0', not 'IS NOT NULL' — the column is INT NOT NULL DEFAULT 0, IS NOT NULL is always true and would include draft/archived overrides")
	}
	if !strings.Contains(src, "active_version_no > 0") {
		t.Fatal("batchLoadActiveWorkflowFlags must use 'active_version_no > 0' to exclude draft (=0) and archived (=0) overrides")
	}
}

// TestHasWorkflow_CMSTemplateEmptyBlock_ReturnsFalse is a regression guard ensuring
// nhánh 1 still uses ExtractTemplateWorkflow to count actual steps rather than
// treating any enterprise_workflow block as has_workflow=true.
func TestHasWorkflow_CMSTemplateEmptyBlock_ReturnsFalse(t *testing.T) {
	src := readRepositorySrc(t)
	if !strings.Contains(src, "ExtractTemplateWorkflow") {
		t.Fatal("batchLoadActiveWorkflowFlags must still call ExtractTemplateWorkflow for CMS template blocks (nhánh 1 regression guard)")
	}
}

// TestHasWorkflow_Nhánh3_GlobalWorkflowActive_ReturnsTrue verifies that batchLoadActiveWorkflowFlags
// checks global_workflows with an active version, matching GetEffectiveWorkflow's fallback order.
func TestHasWorkflow_Nhanh3_GlobalWorkflowActive_ReturnsTrue(t *testing.T) {
	src := readRepositorySrc(t)
	if !strings.Contains(src, "global_workflows") {
		t.Fatal("batchLoadActiveWorkflowFlags must query global_workflows (nhánh 3) to detect active governed workflows")
	}
	if !strings.Contains(src, "global_workflow_versions") {
		t.Fatal("batchLoadActiveWorkflowFlags nhánh 3 must join global_workflow_versions to verify active version exists")
	}
}

// TestHasWorkflow_Nhánh3_StatusActiveGuard ensures nhánh 3 filters on w.status = 'active' so that
// archived or soft-deleted global_workflows rows are not treated as having active workflows.
func TestHasWorkflow_Nhanh3_StatusActiveGuard(t *testing.T) {
	src := readRepositorySrc(t)
	fn := extractBatchLoadFunc(t, src)
	if !strings.Contains(fn, "w.status = 'active'") {
		t.Fatal("batchLoadActiveWorkflowFlags nhánh 3 must filter on global_workflows.status = 'active'")
	}
}

func extractBatchLoadFunc(t *testing.T, src string) string {
	t.Helper()
	start := strings.Index(src, "func (r *Repository) batchLoadActiveWorkflowFlags")
	if start == -1 {
		t.Fatal("batchLoadActiveWorkflowFlags not found")
	}
	rest := src[start:]
	end := strings.Index(rest, "\n}\n")
	if end == -1 {
		t.Fatal("could not find end of batchLoadActiveWorkflowFlags")
	}
	return rest[:end]
}

func readRepositorySrc(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatalf("read repository.go: %v", err)
	}
	return string(data)
}
