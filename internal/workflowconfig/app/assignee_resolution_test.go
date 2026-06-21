package app_test

import (
	"context"
	"testing"

	wfc "github.com/cobo/cobo_iam_services/internal/workflowconfig/app"
)

// fakeDirectory is an in-memory AssigneeDirectory for tests (no DB, no fabricated data).
type fakeDirectory struct {
	byRole map[string][]string // roleCode -> membership ids
	byDept map[string][]string
}

func (f fakeDirectory) FindMembershipsByRole(_ context.Context, _, role string) ([]string, error) {
	return f.byRole[role], nil
}
func (f fakeDirectory) FindMembershipsByDepartment(_ context.Context, _, dept string) ([]string, error) {
	return f.byDept[dept], nil
}
func (f fakeDirectory) FindMembershipsByGroup(_ context.Context, _, _ string) ([]string, error) {
	return nil, wfc.ErrUnsupportedScope
}
func (f fakeDirectory) FindCompanyDefaultAssignees(_ context.Context, _ string) ([]string, error) {
	return nil, wfc.ErrUnsupportedScope
}

func newSvc(dir wfc.AssigneeDirectory) *wfc.AssigneeResolutionService {
	return wfc.NewAssigneeResolutionService(wfc.DefaultRoleRegistry(), dir)
}

// 1. explicit user resolves.
func TestResolve_ExplicitUser(t *testing.T) {
	svc := newSvc(fakeDirectory{})
	res, err := svc.Resolve(context.Background(), wfc.AssigneeResolutionInput{
		CompanyID: "c1", ExplicitMembershipIDs: []string{"m-1", "m-2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Resolved || res.Reason != wfc.ReasonResolvedExplicitUser || len(res.MembershipIDs) != 2 {
		t.Fatalf("explicit resolve unexpected: %+v", res)
	}
}

// 2. known standard role resolves.
func TestResolve_KnownStandardRole(t *testing.T) {
	dir := fakeDirectory{byRole: map[string][]string{"reviewer": {"m-10"}}}
	res, err := newSvc(dir).Resolve(context.Background(), wfc.AssigneeResolutionInput{CompanyID: "c1", RoleID: "reviewer"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Resolved || res.Reason != wfc.ReasonResolvedRole || res.MembershipIDs[0] != "m-10" {
		t.Fatalf("role resolve unexpected: %+v", res)
	}
	// alias form (FE id) resolves to canonical code "reviewer".
	res2, _ := newSvc(dir).Resolve(context.Background(), wfc.AssigneeResolutionInput{CompanyID: "c1", RoleID: "role-reviewer"})
	if !res2.Resolved || res2.MembershipIDs[0] != "m-10" {
		t.Fatalf("alias role resolve unexpected: %+v", res2)
	}
}

// 3. unknown role unresolved.
func TestResolve_UnknownRole(t *testing.T) {
	res, _ := newSvc(fakeDirectory{}).Resolve(context.Background(), wfc.AssigneeResolutionInput{CompanyID: "c1", RoleID: "nope"})
	if res.Resolved || res.Reason != wfc.ReasonUnresolvedUnknownRole || !res.Blocking {
		t.Fatalf("unknown role unexpected: %+v", res)
	}
}

// 4. system_owned role does not resolve via tenant override path.
func TestResolve_SystemOwnedViaOverrideBlocked(t *testing.T) {
	res, _ := newSvc(fakeDirectory{}).Resolve(context.Background(), wfc.AssigneeResolutionInput{
		CompanyID: "c1", RoleID: "disclosure_owner", AttemptedOverride: true,
	})
	if res.Resolved || res.Reason != wfc.ReasonUnresolvedProtected || !res.Blocking {
		t.Fatalf("system_owned override unexpected: %+v", res)
	}
}

// 5. empty directory result unresolved.
func TestResolve_EmptyResult(t *testing.T) {
	res, _ := newSvc(fakeDirectory{byRole: map[string][]string{}}).Resolve(context.Background(), wfc.AssigneeResolutionInput{CompanyID: "c1", RoleID: "reviewer"})
	if res.Resolved || res.Reason != wfc.ReasonUnresolvedEmpty {
		t.Fatalf("empty result unexpected: %+v", res)
	}
}

// 6. unsupported group scope unresolved.
func TestResolve_UnsupportedGroupScope(t *testing.T) {
	res, _ := newSvc(fakeDirectory{}).Resolve(context.Background(), wfc.AssigneeResolutionInput{CompanyID: "c1", GroupID: "g1"})
	if res.Resolved || res.Reason != wfc.ReasonUnsupportedScope {
		t.Fatalf("group scope unexpected: %+v", res)
	}
}

// 7. department resolves.
func TestResolve_Department(t *testing.T) {
	dir := fakeDirectory{byDept: map[string][]string{"d1": {"m-20"}}}
	res, _ := newSvc(dir).Resolve(context.Background(), wfc.AssigneeResolutionInput{CompanyID: "c1", DepartmentID: "d1"})
	if !res.Resolved || res.Reason != wfc.ReasonResolvedDepartment || res.MembershipIDs[0] != "m-20" {
		t.Fatalf("department resolve unexpected: %+v", res)
	}
}

// 8. no mapping → unresolved_no_mapping (and never returns a current user).
func TestResolve_NoMapping(t *testing.T) {
	res, _ := newSvc(fakeDirectory{}).Resolve(context.Background(), wfc.AssigneeResolutionInput{CompanyID: "c1"})
	if res.Resolved || res.Reason != wfc.ReasonUnresolvedNoMapping || len(res.MembershipIDs) != 0 {
		t.Fatalf("no mapping unexpected: %+v", res)
	}
}

// 9. adapter never returns current user; unresolved -> (nil,false).
func TestAdapter_NoSilentFallback(t *testing.T) {
	a := wfc.WorkflowAssigneeResolverAdapter{Service: newSvc(fakeDirectory{})}
	ids, ok := a.ResolveStepAssignees(context.Background(), "c1", "step-1", "nope", nil)
	if ok || len(ids) != 0 {
		t.Fatalf("adapter must not resolve unknown role: ids=%v ok=%v", ids, ok)
	}
	ids2, ok2 := a.ResolveStepAssignees(context.Background(), "c1", "step-1", "", []string{"m-explicit"})
	if !ok2 || len(ids2) != 1 || ids2[0] != "m-explicit" {
		t.Fatalf("adapter explicit resolve unexpected: ids=%v ok=%v", ids2, ok2)
	}
}
