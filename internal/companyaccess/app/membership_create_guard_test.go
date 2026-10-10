package app_test

import (
	"context"
	"testing"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// ROLE-09: memberships are created with a known status only (active, inactive, invited).
func TestCreateMembership_RejectsUnknownStatus(t *testing.T) {
	for _, status := range []string{"suspended", "deleted", "pending_verification"} {
		t.Run(status, func(t *testing.T) {
			svc, repo := newScopeSvc(t, personaTenantAdmin)
			seedOrphanUser(t, repo, "u_target")
			before := membershipCount(t, repo, scopeOwnCompany)
			_, err := svc.CreateMembership(context.Background(), caapp.CreateMembershipRequest{
				Subject: scopeSubject(), UserID: "u_target", CompanyID: scopeOwnCompany, Status: status,
			})
			requireHTTPCode(t, err, 400, perr.CodeInvalidRequest)
			if got := membershipCount(t, repo, scopeOwnCompany); got != before {
				t.Fatalf("membership created with status %q", status)
			}
		})
	}
}

func TestCreateMembership_NormalizesStatus(t *testing.T) {
	svc, repo := newScopeSvc(t, personaTenantAdmin)
	seedOrphanUser(t, repo, "u_target")
	out, err := svc.CreateMembership(context.Background(), caapp.CreateMembershipRequest{
		Subject: scopeSubject(), UserID: "u_target", CompanyID: scopeOwnCompany, Status: " Invited ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != "invited" {
		t.Fatalf("status = %q, want invited", out.Status)
	}
}

func TestAssignUserToCompany_RejectsUnknownStatus(t *testing.T) {
	svc, repo := newScopeSvc(t, personaPlatformOp)
	seedOrphanUser(t, repo, "u_target")
	_, err := svc.AssignUserToCompany(context.Background(), caapp.AssignUserToCompanyRequest{
		Subject: scopeSubject(), UserID: "u_target", CompanyID: scopeOtherCompany, MembershipStatus: "suspended",
	})
	requireHTTPCode(t, err, 400, perr.CodeInvalidRequest)
	if got := membershipCount(t, repo, scopeOtherCompany); got != 0 {
		t.Fatal("membership created with an unknown status")
	}
}

// ROLE-21: the role handed out on assignment is validated before any membership is written.
func TestAssignUserToCompany_RoleOfAnotherCompanyRejectedBeforeWrite(t *testing.T) {
	svc, repo := newScopeSvc(t, personaPlatformOp)
	seedOrphanUser(t, repo, "u_target")
	repo.SeedRoleForCompany(caapp.RoleListItem{RoleID: "r_foreign", RoleCode: "foreign_admin", RoleName: "Foreign", Status: "active",
		RoleType: caapp.RoleTypeTenantCustom}, scopeOwnCompany)
	_, err := svc.AssignUserToCompany(context.Background(), caapp.AssignUserToCompanyRequest{
		Subject: scopeSubject(), UserID: "u_target", CompanyID: scopeOtherCompany, RoleID: "r_foreign",
	})
	if err == nil {
		t.Fatal("a role of another company must be rejected")
	}
	if got := membershipCount(t, repo, scopeOtherCompany); got != 0 {
		t.Fatal("membership was created before the role was validated")
	}
}
