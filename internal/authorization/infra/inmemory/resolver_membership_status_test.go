package inmemory

import (
	"context"
	"testing"
)

// H3: a membership that is no longer active (suspended, inactive, deleted, pending
// verification) must not keep the permissions and data scope of its roles.
func TestResolve_InactiveMembershipGetsNoAccess(t *testing.T) {
	repo := NewRepository()
	repo.InactiveMemberships = map[string]bool{"m_001@c_001": true}
	got, err := NewResolver(repo).Resolve(context.Background(), "m_001", "c_001")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Permissions) != 0 {
		t.Errorf("permissions = %v, want none for an inactive membership", got.Permissions)
	}
	if len(got.DataScope.Departments) != 0 || len(got.DataScope.RecordAssignments) != 0 || got.DataScope.HasCompanyWideAccess {
		t.Errorf("data scope = %+v, want empty for an inactive membership", got.DataScope)
	}
	if got.DataScope.ScopeType != "none" {
		t.Errorf("scope type = %q, want none", got.DataScope.ScopeType)
	}
	if got.MembershipID != "m_001" || got.CompanyID != "c_001" {
		t.Errorf("summary must still identify the membership, got %s@%s", got.MembershipID, got.CompanyID)
	}
}

func TestResolve_ActiveMembershipKeepsAccess(t *testing.T) {
	got, err := NewResolver(NewRepository()).Resolve(context.Background(), "m_001", "c_001")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Permissions) == 0 || len(got.DataScope.Departments) == 0 {
		t.Fatalf("active membership lost access: %+v", got)
	}
}
