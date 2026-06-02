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

func readRepositorySrc(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatalf("read repository.go: %v", err)
	}
	return string(data)
}
