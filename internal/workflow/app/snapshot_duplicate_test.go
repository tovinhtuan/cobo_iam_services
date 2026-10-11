package app

import (
	"errors"
	"testing"

	workflowerrs "github.com/cobo/cobo_iam_services/internal/workflow/errs"
)

// REM-01 trigger: duplicate step identities produce duplicate milestones per
// (instance, step, type), which used to re-arm sent reminders every tick.
func TestValidateSnapshot_RejectsDuplicateStepIdentity(t *testing.T) {
	cases := []struct {
		name string
		snap []StepSnapshot
		dup  bool
	}{
		{"unique", []StepSnapshot{{StepID: "a", StepCode: "A"}, {StepID: "b", StepCode: "B"}}, false},
		{"empty codes are not duplicates", []StepSnapshot{{StepID: "a"}, {StepID: "b"}}, false},
		{"duplicate step_id", []StepSnapshot{{StepID: "a"}, {StepID: "a"}}, true},
		{"duplicate step_id ignores case and spaces", []StepSnapshot{{StepID: "A"}, {StepID: " a "}}, true},
		{"duplicate step_code", []StepSnapshot{{StepID: "a", StepCode: "X"}, {StepID: "b", StepCode: "X"}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateSnapshot(tc.snap)
			if tc.dup {
				if !errors.Is(err, workflowerrs.ErrDuplicateWorkflowStep) {
					t.Fatalf("want ErrDuplicateWorkflowStep, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}
		})
	}
}
