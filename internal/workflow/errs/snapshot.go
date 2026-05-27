package errs

import "errors"

// ErrEmptyWorkflowSnapshot is returned when an effective workflow has no materializable steps.
var ErrEmptyWorkflowSnapshot = errors.New("effective workflow has no steps")

// IsEmptyEffectiveWorkflow reports whether err indicates an empty effective workflow.
func IsEmptyEffectiveWorkflow(err error) bool {
	return errors.Is(err, ErrEmptyWorkflowSnapshot)
}
