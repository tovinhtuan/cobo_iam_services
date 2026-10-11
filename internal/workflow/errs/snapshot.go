package errs

import "errors"

// ErrEmptyWorkflowSnapshot is returned when an effective workflow has no materializable steps.
var ErrEmptyWorkflowSnapshot = errors.New("effective workflow has no steps")

// ErrDuplicateWorkflowStep is returned when two snapshot steps share a step_id or step_code.
// Duplicate identities create duplicate reminder milestones per (instance, step, type).
var ErrDuplicateWorkflowStep = errors.New("workflow step identity must be unique")

// IsEmptyEffectiveWorkflow reports whether err indicates an empty effective workflow.
func IsEmptyEffectiveWorkflow(err error) bool {
	return errors.Is(err, ErrEmptyWorkflowSnapshot)
}

// IsDuplicateWorkflowStep reports whether err indicates duplicate step identities.
func IsDuplicateWorkflowStep(err error) bool {
	return errors.Is(err, ErrDuplicateWorkflowStep)
}
