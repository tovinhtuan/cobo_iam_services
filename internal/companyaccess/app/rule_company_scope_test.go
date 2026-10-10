package app_test

import (
	"context"
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

type ruleRecordingRepo struct {
	*cainmem.AdminRepository
	scopeRules    []map[string]any
	assigneeRules []map[string]any
}

func (r *ruleRecordingRepo) AddResourceScopeRule(_ context.Context, rule map[string]any) error {
	r.scopeRules = append(r.scopeRules, rule)
	return nil
}

func (r *ruleRecordingRepo) AddWorkflowAssigneeRule(_ context.Context, rule map[string]any) error {
	r.assigneeRules = append(r.assigneeRules, rule)
	return nil
}

// ROLE-02: rules are always written to the caller's company, whatever company_id the body names.
func TestCreateRules_CompanyComesFromTheToken(t *testing.T) {
	repo := &ruleRecordingRepo{AdminRepository: cainmem.NewAdminRepository()}
	svc := caapp.NewAdminService(repo, fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"rbac.manage"}}, fixedIDGen("test-id"))
	sub := caapp.AdminSubject{UserID: "u-a", MembershipID: "m-a", CompanyID: "c-a"}
	ctx := context.Background()

	if err := svc.CreateResourceScopeRule(ctx, caapp.CreateResourceScopeRuleRequest{Subject: sub,
		Payload: map[string]any{"company_id": "c-b", "rule_code": "r1"}}); err != nil {
		t.Fatal(err)
	}
	if err := svc.CreateWorkflowAssigneeRule(ctx, caapp.CreateWorkflowAssigneeRuleRequest{Subject: sub,
		Payload: map[string]any{"company_id": "c-b", "rule_code": "r2"}}); err != nil {
		t.Fatal(err)
	}
	if got := repo.scopeRules[0]["company_id"]; got != "c-a" {
		t.Errorf("resource scope rule company_id = %v, want c-a", got)
	}
	if got := repo.assigneeRules[0]["company_id"]; got != "c-a" {
		t.Errorf("workflow assignee rule company_id = %v, want c-a", got)
	}
}

func TestCreateRules_RequirePayload(t *testing.T) {
	repo := &ruleRecordingRepo{AdminRepository: cainmem.NewAdminRepository()}
	svc := caapp.NewAdminService(repo, fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"rbac.manage"}}, fixedIDGen("test-id"))
	sub := caapp.AdminSubject{UserID: "u-a", MembershipID: "m-a", CompanyID: "c-a"}
	err := svc.CreateResourceScopeRule(context.Background(), caapp.CreateResourceScopeRuleRequest{Subject: sub})
	requireHTTPCode(t, err, 400, perr.CodeInvalidRequest)
	err = svc.CreateWorkflowAssigneeRule(context.Background(), caapp.CreateWorkflowAssigneeRuleRequest{Subject: sub})
	requireHTTPCode(t, err, 400, perr.CodeInvalidRequest)
}
