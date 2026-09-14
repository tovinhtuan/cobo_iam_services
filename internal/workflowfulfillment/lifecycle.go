package workflowfulfillment

import "fmt"

// ErrInvalidLifecycleTransition is raised when a Tenant mutation attempts a disallowed lifecycle change.
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

// ValidateInitialLifecycle enforces NEW ROW → ACTIVE only.
func ValidateInitialLifecycle(status string) error {
	if status != LifecycleActive {
		return ErrInvalidLifecycleTransition{To: status}
	}
	return nil
}

// ValidateTransition enforces the V1 lifecycle state machine for Tenant mutations:
//
//	ACTIVE → DELETED
//	ACTIVE → SUPERSEDED
//
// Invalid (normal Tenant path): DELETED/SUPERSEDED → anything; DELETED ↔ SUPERSEDED.
func ValidateTransition(from, to string) error {
	switch {
	case from == LifecycleActive && to == LifecycleDeleted:
		return nil
	case from == LifecycleActive && to == LifecycleSuperseded:
		return nil
	default:
		return ErrInvalidLifecycleTransition{From: from, To: to}
	}
}

// AllowedLifecycleTransitions documents the canonical V1 state machine for evidence packs/tests.
func AllowedLifecycleTransitions() map[string][]string {
	return map[string][]string{
		LifecycleActive: {LifecycleDeleted, LifecycleSuperseded},
		"":              {LifecycleActive}, // initial create
	}
}
