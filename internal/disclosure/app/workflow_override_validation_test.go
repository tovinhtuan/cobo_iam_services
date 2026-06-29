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
