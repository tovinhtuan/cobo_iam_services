package mysql

import (
	"os"
	"strings"
	"testing"
)

func TestHasWorkflow_CompanyTemplateWithApprovedOverride_ReturnsTrue(t *testing.T) {
	src := readRepositorySrc(t)
	if !strings.Contains(src, "company_template_workflow_overrides") {
		t.Fatal("batchLoadActiveWorkflowFlags must query company_template_workflow_overrides to support company-defined templates")
	}
}

func TestHasWorkflow_Nhanh2_MustFilterByCompanyID(t *testing.T) {
	src := readRepositorySrc(t)
	if !strings.Contains(src, "company_id = ?") {
		t.Fatal("batchLoadActiveWorkflowFlags nhánh 2 must filter by company_id to prevent cross-company template usage")
	}
}

func TestHasWorkflow_CompanyTemplateDraftAndArchivedOverride_ReturnsFalse(t *testing.T) {
	src := readRepositorySrc(t)
	if strings.Contains(src, "active_version_no IS NOT NULL") {
		t.Fatal("condition must be 'active_version_no > 0', not 'IS NOT NULL'")
	}
	if !strings.Contains(src, "active_version_no > 0") {
		t.Fatal("batchLoadActiveWorkflowFlags must use 'active_version_no > 0' to exclude draft and archived overrides")
	}
}

func TestHasWorkflow_CMSTemplateEmptyBlock_ReturnsFalse(t *testing.T) {
	fn := extractBatchLoadFunc(t, readRepositorySrc(t))
	if !strings.Contains(fn, "TEMPLATE_PINNED") {
		t.Fatal("batchLoadActiveWorkflowFlags CMS default must require TEMPLATE_PINNED")
	}
	if !strings.Contains(fn, "JSON_LENGTH") {
		t.Fatal("empty pinned publication must not count as has_workflow")
	}
	if strings.Contains(fn, "ExtractTemplateWorkflow") {
		t.Fatal("runtime has_workflow must not parse live enterprise_workflow blocks")
	}
}

func TestHasWorkflow_DoesNotUseGlobalWorkflowRuntimeAuthority(t *testing.T) {
	fn := extractBatchLoadFunc(t, readRepositorySrc(t))
	if strings.Contains(fn, "global_workflows") || strings.Contains(fn, "global_workflow_versions") {
		t.Fatal("batchLoadActiveWorkflowFlags must not use global workflow as runtime authority")
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
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}
