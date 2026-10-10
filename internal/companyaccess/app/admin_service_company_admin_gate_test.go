package app_test

// ROLE-04: assigning/revoking a company admin and transferring ownership are gated by
// rbac.manage (the permission every company owner holds), not by system.settings.

import (
	"context"
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func TestCompanyAdminRoutes_GatedByRbacManage(t *testing.T) {
	calls := map[string]func(svc caapp.AdminService) error{
		"AssignCompanyAdmin": func(svc caapp.AdminService) error {
			return svc.AssignCompanyAdmin(context.Background(), caapp.AssignCompanyAdminRequest{
				Subject: caapp.AdminSubject{UserID: "u-primary", MembershipID: "m-primary", CompanyID: "c-1"}, MembershipID: "m-target",
			})
		},
		"RevokeCompanyAdmin": func(svc caapp.AdminService) error {
			return svc.RevokeCompanyAdmin(context.Background(), caapp.RevokeCompanyAdminRequest{
				Subject: caapp.AdminSubject{UserID: "u-primary", MembershipID: "m-primary", CompanyID: "c-1"}, MembershipID: "m-target",
			})
		},
		"TransferOwnership": func(svc caapp.AdminService) error {
			return svc.TransferOwnership(context.Background(), caapp.TransferOwnershipRequest{
				Subject: caapp.AdminSubject{UserID: "u-primary", MembershipID: "m-primary", CompanyID: "c-1"}, TargetMembershipID: "m-target",
			})
		},
	}
	for name, call := range calls {
		build := func(perms ...string) caapp.AdminService {
			repo := cainmem.NewAdminRepository()
			seedMem(repo, "m-primary", "u-primary", "c-1")
			seedMem(repo, "m-target", "u-target", "c-1")
			if name == "TransferOwnership" { // the new owner must be a company admin (ROLE-26)
				_ = repo.AddRolePermission(context.Background(), "company_admin", "rbac.manage")
				_ = repo.AddRole(context.Background(), "m-target", "company_admin")
			}
			if err := repo.SetMembershipPrimaryAdmin(context.Background(), "m-primary"); err != nil {
				t.Fatal(err)
			}
			return caapp.NewAdminService(repo, fakeAuthService{decision: authapp.DecisionAllow, permissions: perms}, fixedIDGen("test-id"))
		}
		t.Run(name+"/rbac.manage only", func(t *testing.T) {
			if err := call(build("rbac.manage")); err != nil {
				t.Fatalf("a company owner with rbac.manage must be allowed, got %v", err)
			}
		})
		t.Run(name+"/system.settings only", func(t *testing.T) {
			requireHTTPCode(t, call(build("system.settings")), 403, perr.CodePermissionDenied)
		})
	}
}
