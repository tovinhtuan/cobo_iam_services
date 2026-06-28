package app

import (
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
		{StepID: "s1", Stage: "Review", ProcessingDays: 1},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
