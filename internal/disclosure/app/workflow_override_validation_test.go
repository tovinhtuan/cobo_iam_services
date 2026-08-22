package app

import (
	"strings"
	"testing"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func TestValidateCompanyWorkflowOverrideSteps_Empty(t *testing.T) {
	err := ValidateCompanyWorkflowOverrideSteps(nil)
	if err == nil {
		t.Fatal("expected error for empty workflow")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.Code != perr.CodeWorkflowOverrideEmpty {
		t.Fatalf("code = %v, want WORKFLOW_OVERRIDE_EMPTY", he)
	}
}

func TestValidateCompanyWorkflowOverrideSteps_InvalidStep(t *testing.T) {
	err := ValidateCompanyWorkflowOverrideSteps([]WorkflowStepDTO{{StepID: "", Stage: "Review"}})
	if err == nil {
		t.Fatal("expected error")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.Code != perr.CodeWorkflowOverrideInvalid {
		t.Fatalf("code = %v, want WORKFLOW_OVERRIDE_INVALID", he)
	}
}

func TestValidateCompanyWorkflowOverrideSteps_Valid(t *testing.T) {
	err := ValidateCompanyWorkflowOverrideSteps([]WorkflowStepDTO{
		{StepID: "s1", Stage: "Review", ProcessingDays: 1, Instructions: "Chuẩn bị hồ sơ"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateCompanyWorkflowOverrideSteps_InstructionsTooLong(t *testing.T) {
	long := strings.Repeat("x", maxWorkflowOverrideStepInstructionsLen+1)
	err := ValidateCompanyWorkflowOverrideSteps([]WorkflowStepDTO{
		{StepID: "s1", Stage: "Review", ProcessingDays: 1, Instructions: long},
	})
	if err == nil {
		t.Fatal("expected error for long instructions")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.Code != perr.CodeWorkflowOverrideInvalid {
		t.Fatalf("code = %v, want WORKFLOW_OVERRIDE_INVALID", he)
	}
}

func TestValidateCompanyWorkflowOverrideSteps_SafeHTMLFormatAccepted(t *testing.T) {
	steps := []WorkflowStepDTO{
		{
			StepID:            "s1",
			Stage:             "Review",
			ProcessingDays:    1,
			Description:       "<p><strong>Company QA</strong></p>",
			DescriptionFormat: "safe_html",
		},
	}
	err := ValidateCompanyWorkflowOverrideSteps(steps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if steps[0].DescriptionFormat != "safe_html" {
		t.Fatalf("description_format=%q", steps[0].DescriptionFormat)
	}
}

func TestValidateCompanyWorkflowOverrideSteps_UnknownFormatRejected(t *testing.T) {
	err := ValidateCompanyWorkflowOverrideSteps([]WorkflowStepDTO{
		{StepID: "s1", Stage: "Review", ProcessingDays: 1, DescriptionFormat: "raw_html"},
	})
	if err == nil {
		t.Fatal("expected error for unknown description_format")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.Code != perr.CodeWorkflowOverrideInvalid {
		t.Fatalf("code = %v, want WORKFLOW_OVERRIDE_INVALID", he)
	}
}

func TestValidateCompanyWorkflowOverrideSteps_BlankDocumentNameRejected(t *testing.T) {
	err := ValidateCompanyWorkflowOverrideSteps([]WorkflowStepDTO{
		{
			StepID: "s1",
			Stage:  "Collect",
			Documents: []WorkflowDocumentDTO{
				{DocID: "doc_1", Name: "   ", Required: true},
			},
		},
	})
	if err == nil {
		t.Fatal("expected blank name error")
	}
}

func TestValidateCompanyWorkflowOverrideSteps_NameOnlyDocumentOK(t *testing.T) {
	err := ValidateCompanyWorkflowOverrideSteps([]WorkflowStepDTO{
		{
			StepID: "s1",
			Stage:  "Collect",
			Documents: []WorkflowDocumentDTO{
				{DocID: "doc_1", Name: "BCTC Q3", Required: true},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidateCompanyWorkflowOverrideSteps_DocumentWithFileOK(t *testing.T) {
	err := ValidateCompanyWorkflowOverrideSteps([]WorkflowStepDTO{
		{
			StepID: "s1",
			Stage:  "Collect",
			Documents: []WorkflowDocumentDTO{
				{DocID: "doc_1", Name: "BCTC Q3", Required: true, TemplateFileID: "wdt_abc", TemplateFileName: "form.xlsx"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidateCompanyWorkflowOverrideSteps_LegacyMissingFormatOK(t *testing.T) {
	steps := []WorkflowStepDTO{
		{StepID: "s1", Stage: "Review", ProcessingDays: 1, Description: "BCTT"},
	}
	err := ValidateCompanyWorkflowOverrideSteps(steps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if steps[0].DescriptionFormat != "" {
		t.Fatalf("expected empty plain default, got %q", steps[0].DescriptionFormat)
	}
}
