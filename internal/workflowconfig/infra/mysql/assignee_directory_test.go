package mysql

import (
	"testing"

	wfcapp "github.com/cobo/cobo_iam_services/internal/workflowconfig/app"
)

// Construction must not panic and must satisfy the AssigneeDirectory interface, so server wiring
// (which builds NewDirectory(pool) → NewAssigneeResolutionService → adapter) is safe.
func TestNewDirectory_ConstructsAndSatisfiesInterface(t *testing.T) {
	d := NewDirectory(nil) // nil DB: construction must not panic (methods would, but wiring only constructs)
	if d == nil {
		t.Fatal("NewDirectory returned nil")
	}
	var _ wfcapp.AssigneeDirectory = d

	// The full wiring chain used in server.go must construct without panic.
	svc := wfcapp.NewAssigneeResolutionService(wfcapp.DefaultRoleRegistry(), d)
	if svc == nil {
		t.Fatal("NewAssigneeResolutionService returned nil")
	}
	adapter := wfcapp.WorkflowAssigneeResolverAdapter{Service: svc}
	if adapter.Service == nil {
		t.Fatal("adapter service nil")
	}
}
