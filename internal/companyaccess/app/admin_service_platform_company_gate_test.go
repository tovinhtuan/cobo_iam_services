package app_test

import (
	"context"
	"net/http"
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

// policyAuthService mirrors the production action→permission mapping
// (authorization/infra/mysql repository.go: admin.membership.create|list → admin.membership.invite).
// fakeAuthService accepts rbac.manage for those actions, which hid the bug below.
type policyAuthService struct {
	permissions []string
}

func (p policyAuthService) has(perm string) bool {
	for _, have := range p.permissions {
		if have == perm {
			return true
		}
	}
	return false
}

func (p policyAuthService) Authorize(_ context.Context, req authapp.AuthorizeRequest) (*authapp.AuthorizeDecision, error) {
	required := req.Action
	switch req.Action {
	case "admin.membership.create", "admin.membership.list":
		required = "admin.membership.invite"
	}
	if p.has(required) {
		return &authapp.AuthorizeDecision{Decision: authapp.DecisionAllow}, nil
	}
	return &authapp.AuthorizeDecision{Decision: authapp.DecisionDeny}, nil
}

func (p policyAuthService) AuthorizeBatch(_ context.Context, _ authapp.AuthorizeBatchRequest) (*authapp.AuthorizeBatchResponse, error) {
	return &authapp.AuthorizeBatchResponse{}, nil
}

func (p policyAuthService) GetEffectiveAccess(_ context.Context, _, _ string) (*authapp.EffectiveAccessSummary, error) {
	return &authapp.EffectiveAccessSummary{Permissions: p.permissions}, nil
}

func newPolicyScopeSvc(t *testing.T, perms []string) caapp.AdminService {
	t.Helper()
	repo := cainmem.NewAdminRepository()
	seedInviteScopedSubject(t, repo, scopeSubject())
	return caapp.NewAdminService(repo, policyAuthService{permissions: perms}, idgen.UUIDv7Generator{})
}

// Platform company management = platform operator (platform.cms.view AND rbac.manage|system.settings),
// per CLAUDE.md and bug-cross-tenant-membership-create-2026-10-09. The seeded cms_operator role has no
// admin.membership.invite; it got 403 on GET /platform/cms/admin/companies (DEV smoke 2026-10-11).
func TestPlatformCompanyManagement_CMSOperatorWithoutInviteAllowed(t *testing.T) {
	for _, perms := range [][]string{
		{"platform.cms.view", "rbac.manage"},
		{"platform.cms.view", "system.settings"},
	} {
		svc := newPolicyScopeSvc(t, perms)
		if _, err := svc.ListPlatformCompanies(context.Background(), caapp.ListPlatformCompaniesRequest{Subject: scopeSubject()}); err != nil {
			t.Fatalf("%v ListPlatformCompanies: %v", perms, err)
		}
		// The in-memory repo has no company row; only the authorization outcome matters here.
		_, err := svc.GetPlatformCompany(context.Background(), caapp.GetPlatformCompanyRequest{Subject: scopeSubject(), CompanyID: scopeOtherCompany})
		if he, ok := perr.AsHTTPError(err); ok && he.HTTPStatus == http.StatusForbidden {
			t.Fatalf("%v GetPlatformCompany: unexpected 403 %v", perms, err)
		}
	}
}

// A tenant permission must never be enough, including admin.membership.invite.
func TestPlatformCompanyManagement_TenantPermissionsStillForbidden(t *testing.T) {
	for _, perms := range [][]string{
		{"rbac.manage", "admin.membership.invite"},
		{"system.settings", "admin.membership.invite"},
		{"platform.cms.view", "admin.membership.invite"},
	} {
		svc := newPolicyScopeSvc(t, perms)
		_, err := svc.ListPlatformCompanies(context.Background(), caapp.ListPlatformCompaniesRequest{Subject: scopeSubject()})
		he, ok := perr.AsHTTPError(err)
		if !ok || he.HTTPStatus != http.StatusForbidden {
			t.Fatalf("%v ListPlatformCompanies: expected 403, got %v", perms, err)
		}
	}
}
