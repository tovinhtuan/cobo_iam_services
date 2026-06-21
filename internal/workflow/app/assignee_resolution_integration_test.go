package app

import (
	"context"
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
)

// allowAuth authorizes everything (for resolver-path tests).
type allowAuth struct{}

func (allowAuth) Authorize(_ context.Context, _ authapp.AuthorizeRequest) (*authapp.AuthorizeDecision, error) {
	return &authapp.AuthorizeDecision{Decision: authapp.DecisionAllow}, nil
}
func (allowAuth) AuthorizeBatch(_ context.Context, _ authapp.AuthorizeBatchRequest) (*authapp.AuthorizeBatchResponse, error) {
	return &authapp.AuthorizeBatchResponse{}, nil
}
func (allowAuth) GetEffectiveAccess(_ context.Context, _, _ string) (*authapp.EffectiveAccessSummary, error) {
	return nil, nil
}

type fakeResolver struct {
	ids      []string
	resolved bool
}

func (f fakeResolver) ResolveStepAssignees(_ context.Context, _, _, _ string, _ []string) ([]string, bool) {
	return f.ids, f.resolved
}

const selfMembership = "member-self"

func resolveReq() ResolveAssigneesRequest {
	return ResolveAssigneesRequest{
		Subject:            Subject{UserID: "u1", MembershipID: selfMembership, CompanyID: "c1"},
		WorkflowInstanceID: "wf-1",
		StepCode:           "review",
	}
}

func contains(ids []string, v string) bool {
	for _, x := range ids {
		if x == v {
			return true
		}
	}
	return false
}

// No resolver wired: controlled unresolved (empty), NEVER the current user.
func TestResolveAssignees_NoResolver_NoSelfFallback(t *testing.T) {
	svc := NewService(&fakeWorkflowRepository{}, allowAuth{}, fakeWorkflowIDGen{})
	res, err := svc.ResolveAssignees(context.Background(), resolveReq())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.MembershipIDs) != 0 {
		t.Fatalf("expected empty (unresolved), got %v", res.MembershipIDs)
	}
	if contains(res.MembershipIDs, selfMembership) {
		t.Fatal("must NOT fall back to current user")
	}
}

// Resolver wired + flag ON + resolved: returns resolved ids, not the current user.
func TestResolveAssignees_Resolved(t *testing.T) {
	svc := NewService(&fakeWorkflowRepository{}, allowAuth{}, fakeWorkflowIDGen{},
		WithFlags(Flags{AssigneeResolutionEnabled: true}),
		WithAssigneeResolver(fakeResolver{ids: []string{"m-x", "m-y"}, resolved: true}),
	)
	res, err := svc.ResolveAssignees(context.Background(), resolveReq())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.MembershipIDs) != 2 || res.MembershipIDs[0] != "m-x" {
		t.Fatalf("expected resolved ids, got %v", res.MembershipIDs)
	}
	if contains(res.MembershipIDs, selfMembership) {
		t.Fatal("must NOT include current user")
	}
}

// Resolver wired + flag ON + unresolved: empty, not the current user.
func TestResolveAssignees_ResolverUnresolved(t *testing.T) {
	svc := NewService(&fakeWorkflowRepository{}, allowAuth{}, fakeWorkflowIDGen{},
		WithFlags(Flags{AssigneeResolutionEnabled: true}),
		WithAssigneeResolver(fakeResolver{resolved: false}),
	)
	res, _ := svc.ResolveAssignees(context.Background(), resolveReq())
	if len(res.MembershipIDs) != 0 || contains(res.MembershipIDs, selfMembership) {
		t.Fatalf("expected empty unresolved without self, got %v", res.MembershipIDs)
	}
}

// Flag OFF ignores the resolver and still does not self-fallback.
func TestResolveAssignees_FlagOff_IgnoresResolver(t *testing.T) {
	svc := NewService(&fakeWorkflowRepository{}, allowAuth{}, fakeWorkflowIDGen{},
		WithAssigneeResolver(fakeResolver{ids: []string{"m-x"}, resolved: true}),
	)
	res, _ := svc.ResolveAssignees(context.Background(), resolveReq())
	if len(res.MembershipIDs) != 0 {
		t.Fatalf("flag OFF should not use resolver, got %v", res.MembershipIDs)
	}
	if contains(res.MembershipIDs, selfMembership) {
		t.Fatal("must NOT fall back to current user")
	}
}
