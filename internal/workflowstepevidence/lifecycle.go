package workflowstepevidence

import (
	"fmt"

	wff "github.com/cobo/cobo_iam_services/internal/workflowfulfillment"
)

// LIFECYCLE_SOURCE=REUSED — delegate to B5 fulfillment validators (identical state machine).

// ValidateInitialLifecycle enforces NEW ROW → ACTIVE only.
func ValidateInitialLifecycle(status string) error {
	if err := wff.ValidateInitialLifecycle(status); err != nil {
		return ErrInvalidLifecycleTransition{From: "", To: status}
	}
	return nil
}

// ValidateTransition enforces ACTIVE → DELETED | SUPERSEDED only.
func ValidateTransition(from, to string) error {
	if err := wff.ValidateTransition(from, to); err != nil {
		return ErrInvalidLifecycleTransition{From: from, To: to}
	}
	return nil
}

// ErrInvalidLifecycleTransition is raised for disallowed lifecycle changes.
type ErrInvalidLifecycleTransition struct {
	From string
	To   string
}

func (e ErrInvalidLifecycleTransition) Error() string {
	if e.From == "" {
		return fmt.Sprintf("invalid initial lifecycle status %q", e.To)
	}
	return fmt.Sprintf("invalid lifecycle transition %s -> %s", e.From, e.To)
}
