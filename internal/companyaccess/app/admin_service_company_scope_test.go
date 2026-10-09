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

// C4/H2 (risk review 2026-10-09): thao tac sang company khac company cua token, hoac thao tac
// "khong co company", chi danh cho platform operator = platform.cms.view va (rbac.manage | system.settings).
// Cac persona con lai (ke ca owner tenant co rbac.manage) bi gioi han trong company cua token.

type scopePersona struct {
	name  string
	perms []string
}

var (
	personaTenantAdmin    = scopePersona{"tenantAdmin(rbac.manage)", []string{"rbac.manage", "admin.membership.invite"}}
	personaTenantAdminSys = scopePersona{"tenantAdmin(system.settings)", []string{"system.settings", "admin.membership.invite"}}
	personaCMSViewOnly    = scopePersona{"cmsViewOnly", []string{"platform.cms.view", "admin.membership.invite"}}
	personaPlatformOp     = scopePersona{"platformOperator(rbac.manage)", []string{"platform.cms.view", "rbac.manage", "admin.membership.invite"}}
	personaPlatformOpSys  = scopePersona{"platformOperator(system.settings)", []string{"platform.cms.view", "system.settings", "admin.membership.invite"}}

	nonOperatorPersonas = []scopePersona{personaTenantAdmin, personaTenantAdminSys, personaCMSViewOnly}
	operatorPersonas    = []scopePersona{personaPlatformOp, personaPlatformOpSys}
)

const (
	scopeOwnCompany   = "c_001"
	scopeOtherCompany = "c_002"
)

func scopeSubject() caapp.AdminSubject {
	return caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: scopeOwnCompany}
}

func newScopeSvc(t *testing.T, p scopePersona) (caapp.AdminService, *cainmem.AdminRepository) {
	t.Helper()
	repo := cainmem.NewAdminRepository()
	seedInviteScopedSubject(t, repo, scopeSubject())
	svc := caapp.NewAdminService(
		repo,
		fakeAuthService{decision: authapp.DecisionAllow, permissions: p.perms},
		idgen.UUIDv7Generator{},
	)
	return svc, repo
}

func requireScopeMismatch(t *testing.T, err error) {
	t.Helper()
	he, ok := perr.AsHTTPError(err)
	if !ok {
		t.Fatalf("expected HTTPError 403 %s, got %v", perr.CodeCompanyScopeMismatch, err)
	}
	if he.HTTPStatus != http.StatusForbidden || he.Code != perr.CodeCompanyScopeMismatch {
		t.Fatalf("expected 403 %s, got %d %s", perr.CodeCompanyScopeMismatch, he.HTTPStatus, he.Code)
	}
}

func membershipCount(t *testing.T, repo *cainmem.AdminRepository, companyID string) int {
	t.Helper()
	items, err := repo.ListMembershipsByCompany(context.Background(), companyID)
	if err != nil {
		t.Fatalf("ListMembershipsByCompany(%s): %v", companyID, err)
	}
	return len(items)
}

// seedOrphanUser creates an existing user without a membership, using the repo directly.
func seedOrphanUser(t *testing.T, repo *cainmem.AdminRepository, userID string) {
	t.Helper()
	if _, err := repo.CreateUser(context.Background(), caapp.UserView{
		UserID: userID, LoginID: userID + "@example.com", FullName: userID, AccountStatus: "active",
	}, "hash", caapp.CreateUserOptions{}); err != nil {
		t.Fatalf("seed user %s: %v", userID, err)
	}
}

// ---------------------------------------------------------------- T2 CreateUser

func TestScope_CreateUser_NonOperator_OtherCompany_Forbidden(t *testing.T) {
	for _, p := range nonOperatorPersonas {
		t.Run(p.name, func(t *testing.T) {
			svc, repo := newScopeSvc(t, p)
			before := membershipCount(t, repo, scopeOtherCompany)
			_, err := svc.CreateUser(context.Background(), caapp.CreateUserRequest{
				Subject: scopeSubject(), LoginID: "x." + p.name + "@example.com", Password: "StrongPass123!",
				FullName: "Scope Probe", CompanyID: scopeOtherCompany,
			})
			requireScopeMismatch(t, err)
			if got := membershipCount(t, repo, scopeOtherCompany); got != before {
				t.Fatalf("memberships in %s changed: %d -> %d", scopeOtherCompany, before, got)
			}
		})
	}
}

func TestScope_CreateUser_NonOperator_EmptyCompany_ForcedToOwn(t *testing.T) {
	for _, p := range nonOperatorPersonas {
		t.Run(p.name, func(t *testing.T) {
			svc, _ := newScopeSvc(t, p)
			out, err := svc.CreateUser(context.Background(), caapp.CreateUserRequest{
				Subject: scopeSubject(), LoginID: "own." + p.name + "@example.com", Password: "StrongPass123!",
				FullName: "Scope Own", CompanyID: "",
			})
			if err != nil {
				t.Fatalf("CreateUser: %v", err)
			}
			if out.CompanyID != scopeOwnCompany || out.MembershipID == "" {
				t.Fatalf("expected membership in %s, got company=%q membership=%q", scopeOwnCompany, out.CompanyID, out.MembershipID)
			}
		})
	}
}

func TestScope_CreateUser_Operator_OtherAndNoCompany_Allowed(t *testing.T) {
	for _, p := range operatorPersonas {
		t.Run(p.name, func(t *testing.T) {
			svc, _ := newScopeSvc(t, p)
			other, err := svc.CreateUser(context.Background(), caapp.CreateUserRequest{
				Subject: scopeSubject(), LoginID: "op.other." + p.name + "@example.com", Password: "StrongPass123!",
				FullName: "Op Other", CompanyID: scopeOtherCompany,
			})
			if err != nil || other.CompanyID != scopeOtherCompany {
				t.Fatalf("operator other company: out=%+v err=%v", other, err)
			}
			none, err := svc.CreateUser(context.Background(), caapp.CreateUserRequest{
				Subject: scopeSubject(), LoginID: "op.none." + p.name + "@example.com", Password: "StrongPass123!",
				FullName: "Op None", CompanyID: "",
			})
			if err != nil || none.MembershipID != "" || none.CompanyID != "" {
				t.Fatalf("operator no company: out=%+v err=%v", none, err)
			}
		})
	}
}

// ---------------------------------------------------------------- T3 CreateMembership / AssignUserToCompany

func TestScope_CreateMembership_NonOperator_OtherCompany_Forbidden(t *testing.T) {
	for _, p := range nonOperatorPersonas {
		t.Run(p.name, func(t *testing.T) {
			svc, repo := newScopeSvc(t, p)
			seedOrphanUser(t, repo, "u_target")
			_, err := svc.CreateMembership(context.Background(), caapp.CreateMembershipRequest{
				Subject: scopeSubject(), UserID: "u_target", CompanyID: scopeOtherCompany,
			})
			requireScopeMismatch(t, err)
			if got := membershipCount(t, repo, scopeOtherCompany); got != 0 {
				t.Fatalf("expected no membership in %s, got %d", scopeOtherCompany, got)
			}
		})
	}
}

func TestScope_CreateMembership_NonOperator_EmptyCompany_ForcedToOwn(t *testing.T) {
	svc, repo := newScopeSvc(t, personaTenantAdmin)
	seedOrphanUser(t, repo, "u_target")
	out, err := svc.CreateMembership(context.Background(), caapp.CreateMembershipRequest{
		Subject: scopeSubject(), UserID: "u_target", CompanyID: "",
	})
	if err != nil || out.CompanyID != scopeOwnCompany {
		t.Fatalf("expected membership in %s, got out=%+v err=%v", scopeOwnCompany, out, err)
	}
}

func TestScope_CreateMembership_Operator_OtherCompany_Allowed(t *testing.T) {
	for _, p := range operatorPersonas {
		t.Run(p.name, func(t *testing.T) {
			svc, repo := newScopeSvc(t, p)
			seedOrphanUser(t, repo, "u_target")
			out, err := svc.CreateMembership(context.Background(), caapp.CreateMembershipRequest{
				Subject: scopeSubject(), UserID: "u_target", CompanyID: scopeOtherCompany,
			})
			if err != nil || out.CompanyID != scopeOtherCompany {
				t.Fatalf("operator: out=%+v err=%v", out, err)
			}
		})
	}
}

func TestScope_AssignUserToCompany_NonOperator_OtherCompany_Forbidden(t *testing.T) {
	for _, p := range nonOperatorPersonas {
		t.Run(p.name, func(t *testing.T) {
			svc, repo := newScopeSvc(t, p)
			seedOrphanUser(t, repo, "u_target")
			_, err := svc.AssignUserToCompany(context.Background(), caapp.AssignUserToCompanyRequest{
				Subject: scopeSubject(), UserID: "u_target", CompanyID: scopeOtherCompany,
			})
			requireScopeMismatch(t, err)
			if got := membershipCount(t, repo, scopeOtherCompany); got != 0 {
				t.Fatalf("expected no membership in %s, got %d", scopeOtherCompany, got)
			}
		})
	}
}

func TestScope_AssignUserToCompany_Operator_OtherCompany_Allowed(t *testing.T) {
	for _, p := range operatorPersonas {
		t.Run(p.name, func(t *testing.T) {
			svc, repo := newScopeSvc(t, p)
			seedOrphanUser(t, repo, "u_target")
			out, err := svc.AssignUserToCompany(context.Background(), caapp.AssignUserToCompanyRequest{
				Subject: scopeSubject(), UserID: "u_target", CompanyID: scopeOtherCompany,
			})
			if err != nil || out.CompanyID != scopeOtherCompany {
				t.Fatalf("operator: out=%+v err=%v", out, err)
			}
		})
	}
}

// ---------------------------------------------------------------- T4 H2 list memberships

func TestScope_ListCompanyMemberships_NonOperator(t *testing.T) {
	for _, p := range nonOperatorPersonas {
		t.Run(p.name, func(t *testing.T) {
			svc, _ := newScopeSvc(t, p)
			_, err := svc.ListCompanyMemberships(context.Background(), caapp.ListCompanyMembershipsRequest{
				Subject: scopeSubject(), CompanyID: scopeOtherCompany,
			})
			requireScopeMismatch(t, err)

			_, err = svc.ListCompanyMemberships(context.Background(), caapp.ListCompanyMembershipsRequest{
				Subject: scopeSubject(), ListWithoutCompany: true,
			})
			he, ok := perr.AsHTTPError(err)
			if !ok || he.HTTPStatus != http.StatusForbidden {
				t.Fatalf("list without company: expected 403, got %v", err)
			}

			own, err := svc.ListCompanyMemberships(context.Background(), caapp.ListCompanyMembershipsRequest{
				Subject: scopeSubject(), CompanyID: scopeOwnCompany,
			})
			if err != nil || len(own.Items) == 0 {
				t.Fatalf("own company list: items=%d err=%v", len(own.Items), err)
			}
		})
	}
}

func TestScope_ListCompanyMemberships_Operator(t *testing.T) {
	for _, p := range operatorPersonas {
		t.Run(p.name, func(t *testing.T) {
			svc, repo := newScopeSvc(t, p)
			seedOrphanUser(t, repo, "u_orphan")
			if _, err := svc.ListCompanyMemberships(context.Background(), caapp.ListCompanyMembershipsRequest{
				Subject: scopeSubject(), CompanyID: scopeOtherCompany,
			}); err != nil {
				t.Fatalf("other company: %v", err)
			}
			res, err := svc.ListCompanyMemberships(context.Background(), caapp.ListCompanyMembershipsRequest{
				Subject: scopeSubject(), ListWithoutCompany: true,
			})
			if err != nil || len(res.Items) != 1 {
				t.Fatalf("without company: items=%d err=%v", len(res.Items), err)
			}
		})
	}
}

// ---------------------------------------------------------------- T5 invite / invite roles / resend

func TestScope_InviteUser_NonOperator(t *testing.T) {
	for _, p := range nonOperatorPersonas {
		t.Run(p.name, func(t *testing.T) {
			svc, repo := newScopeSvc(t, p)
			_, err := svc.InviteUser(context.Background(), caapp.InviteUserRequest{
				Subject: scopeSubject(), Email: "other." + p.name + "@example.com", FullName: "Invite Other",
				CompanyID: scopeOtherCompany, CreatedByUserID: "u_admin",
			})
			requireScopeMismatch(t, err)
			if got := membershipCount(t, repo, scopeOtherCompany); got != 0 {
				t.Fatalf("expected no membership in %s, got %d", scopeOtherCompany, got)
			}

			own, err := svc.InviteUser(context.Background(), caapp.InviteUserRequest{
				Subject: scopeSubject(), Email: "own." + p.name + "@example.com", FullName: "Invite Own",
				CompanyID: "", CreatedByUserID: "u_admin",
			})
			if err != nil || own.CompanyID != scopeOwnCompany || own.MembershipID == "" {
				t.Fatalf("empty company must resolve to own company: out=%+v err=%v", own, err)
			}
		})
	}
}

func TestScope_InviteUser_Operator_OtherAndNoCompany_Allowed(t *testing.T) {
	for _, p := range operatorPersonas {
		t.Run(p.name, func(t *testing.T) {
			svc, _ := newScopeSvc(t, p)
			other, err := svc.InviteUser(context.Background(), caapp.InviteUserRequest{
				Subject: scopeSubject(), Email: "op.other." + p.name + "@example.com", FullName: "Op Other",
				CompanyID: scopeOtherCompany, CreatedByUserID: "u_admin",
			})
			if err != nil || other.CompanyID != scopeOtherCompany {
				t.Fatalf("operator other company: out=%+v err=%v", other, err)
			}
			none, err := svc.InviteUser(context.Background(), caapp.InviteUserRequest{
				Subject: scopeSubject(), Email: "op.none." + p.name + "@example.com", FullName: "Op None",
				CompanyID: "", CreatedByUserID: "u_admin",
			})
			if err != nil || none.MembershipID != "" {
				t.Fatalf("operator no company: out=%+v err=%v", none, err)
			}
		})
	}
}

func TestScope_ListInviteRoles_NonOperator_OtherCompany_Forbidden(t *testing.T) {
	for _, p := range nonOperatorPersonas {
		t.Run(p.name, func(t *testing.T) {
			svc, _ := newScopeSvc(t, p)
			_, err := svc.ListInviteRoles(context.Background(), caapp.ListInviteRolesRequest{
				Subject: scopeSubject(), CompanyID: scopeOtherCompany,
			})
			requireScopeMismatch(t, err)
			if _, err := svc.ListInviteRoles(context.Background(), caapp.ListInviteRolesRequest{
				Subject: scopeSubject(), CompanyID: scopeOwnCompany,
			}); err != nil {
				t.Fatalf("own company: %v", err)
			}
		})
	}
}

func TestScope_ListInviteRoles_Operator_OtherCompany_Allowed(t *testing.T) {
	for _, p := range operatorPersonas {
		t.Run(p.name, func(t *testing.T) {
			svc, _ := newScopeSvc(t, p)
			if _, err := svc.ListInviteRoles(context.Background(), caapp.ListInviteRolesRequest{
				Subject: scopeSubject(), CompanyID: scopeOtherCompany,
			}); err != nil {
				t.Fatalf("operator: %v", err)
			}
		})
	}
}

func TestScope_ResendUserInvitation_NoCompanyScope_NonOperator_Forbidden(t *testing.T) {
	for _, p := range nonOperatorPersonas {
		t.Run(p.name, func(t *testing.T) {
			svc, _ := newScopeSvc(t, p)
			err := svc.ResendUserInvitation(context.Background(), caapp.ResendUserInvitationRequest{
				Subject: scopeSubject(), UserID: "u_any", ResendNoCompanyScope: true,
			})
			he, ok := perr.AsHTTPError(err)
			if !ok || he.HTTPStatus != http.StatusForbidden {
				t.Fatalf("expected 403, got %v", err)
			}
		})
	}
}

func TestScope_ResendUserInvitation_OtherCompany_NonOperator_Forbidden(t *testing.T) {
	for _, p := range nonOperatorPersonas {
		t.Run(p.name, func(t *testing.T) {
			svc, _ := newScopeSvc(t, p)
			err := svc.ResendUserInvitation(context.Background(), caapp.ResendUserInvitationRequest{
				Subject: scopeSubject(), UserID: "u_any", CompanyID: scopeOtherCompany,
			})
			requireScopeMismatch(t, err)
		})
	}
}

// ---------------------------------------------------------------- platform company management

// Company management belongs to platform operators; a tenant permission such as rbac.manage is not
// enough even if a handler gate were ever missing.
func TestScope_PlatformCompanyManagement_RequiresOperator(t *testing.T) {
	for _, p := range nonOperatorPersonas {
		t.Run(p.name, func(t *testing.T) {
			svc, _ := newScopeSvc(t, p)
			if _, err := svc.ListPlatformCompanies(context.Background(), caapp.ListPlatformCompaniesRequest{Subject: scopeSubject()}); err == nil {
				t.Fatal("ListPlatformCompanies: expected permission denied")
			} else if he, ok := perr.AsHTTPError(err); !ok || he.HTTPStatus != http.StatusForbidden {
				t.Fatalf("ListPlatformCompanies: expected 403, got %v", err)
			}
			if _, err := svc.GetPlatformCompany(context.Background(), caapp.GetPlatformCompanyRequest{Subject: scopeSubject(), CompanyID: scopeOtherCompany}); err == nil {
				t.Fatal("GetPlatformCompany: expected permission denied")
			} else if he, ok := perr.AsHTTPError(err); !ok || he.HTTPStatus != http.StatusForbidden {
				t.Fatalf("GetPlatformCompany: expected 403, got %v", err)
			}
		})
	}
}

func TestScope_PlatformCompanyManagement_OperatorAllowed(t *testing.T) {
	for _, p := range operatorPersonas {
		t.Run(p.name, func(t *testing.T) {
			svc, _ := newScopeSvc(t, p)
			if _, err := svc.ListPlatformCompanies(context.Background(), caapp.ListPlatformCompaniesRequest{Subject: scopeSubject()}); err != nil {
				t.Fatalf("ListPlatformCompanies: %v", err)
			}
		})
	}
}

// ---------------------------------------------------------------- edge cases

func TestScope_NonOperator_EmptyTokenCompany_FailsClosed(t *testing.T) {
	svc, repo := newScopeSvc(t, personaTenantAdmin)
	noCompany := caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: ""}

	_, err := svc.CreateUser(context.Background(), caapp.CreateUserRequest{
		Subject: noCompany, LoginID: "nocompany@example.com", Password: "StrongPass123!", FullName: "No Company", CompanyID: "",
	})
	if err == nil {
		t.Fatal("expected an error when the token has no company, got success")
	}
	if he, ok := perr.AsHTTPError(err); !ok || he.HTTPStatus != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 company context required, got %v", err)
	}
	if orphans, _ := repo.ListUsersWithNoMembership(context.Background()); len(orphans) != 0 {
		t.Fatalf("no orphan user may be created, got %d", len(orphans))
	}
}

func TestScope_CompanyIDWhitespaceAndCase(t *testing.T) {
	svc, _ := newScopeSvc(t, personaTenantAdmin)

	// Surrounding whitespace is trimmed: own company stays allowed, another company stays rejected.
	own, err := svc.CreateUser(context.Background(), caapp.CreateUserRequest{
		Subject: scopeSubject(), LoginID: "ws.own@example.com", Password: "StrongPass123!", FullName: "WS Own", CompanyID: "  " + scopeOwnCompany + "  ",
	})
	if err != nil || own.CompanyID != scopeOwnCompany {
		t.Fatalf("own company with spaces: out=%+v err=%v", own, err)
	}
	_, err = svc.CreateUser(context.Background(), caapp.CreateUserRequest{
		Subject: scopeSubject(), LoginID: "ws.other@example.com", Password: "StrongPass123!", FullName: "WS Other", CompanyID: " " + scopeOtherCompany + " ",
	})
	requireScopeMismatch(t, err)

	// Company ids are case-sensitive: a case variant is never treated as the own company.
	_, err = svc.CreateUser(context.Background(), caapp.CreateUserRequest{
		Subject: scopeSubject(), LoginID: "case.own@example.com", Password: "StrongPass123!", FullName: "Case", CompanyID: "C_001",
	})
	requireScopeMismatch(t, err)
}

// ---------------------------------------------------------------- role assignment must not grant platform access

// A tenant admin must not be able to hand out a role that carries platform-tier permissions
// (platform.cms.view, cms.*, ...): that would turn a tenant member into a platform operator and
// defeat the company scope above.
func seedScopeRole(t *testing.T, repo *cainmem.AdminRepository, roleID, roleCode string, perms ...string) {
	t.Helper()
	repo.SeedRole(caapp.RoleListItem{
		RoleID: roleID, RoleCode: roleCode, RoleName: roleCode,
		Status: "active", Scope: "company", RoleType: caapp.RoleTypeTenantCustom,
	})
	for _, p := range perms {
		if err := repo.AddRolePermission(context.Background(), roleID, p); err != nil {
			t.Fatalf("seed role permission %s: %v", p, err)
		}
	}
}

func newMembershipWithoutRole(t *testing.T, svc caapp.AdminService, repo *cainmem.AdminRepository) string {
	t.Helper()
	seedOrphanUser(t, repo, "u_target")
	m, err := svc.CreateMembership(context.Background(), caapp.CreateMembershipRequest{
		Subject: scopeSubject(), UserID: "u_target", CompanyID: scopeOwnCompany,
	})
	if err != nil {
		t.Fatalf("seed membership: %v", err)
	}
	return m.MembershipID
}

func TestScope_AssignRole_NonOperator_PlatformCapableRole_Forbidden(t *testing.T) {
	for _, p := range nonOperatorPersonas {
		t.Run(p.name, func(t *testing.T) {
			svc, repo := newScopeSvc(t, p)
			seedScopeRole(t, repo, "role_platform_like", "ops_like", "platform.cms.view", "rbac.manage")
			mid := newMembershipWithoutRole(t, svc, repo)

			err := svc.AssignRole(context.Background(), caapp.AssignRoleRequest{
				Subject: scopeSubject(), MembershipID: mid, RoleID: "role_platform_like",
			})
			he, ok := perr.AsHTTPError(err)
			if !ok || he.HTTPStatus != http.StatusForbidden {
				t.Fatalf("expected 403, got %v", err)
			}
			roles, _ := repo.ListMembershipRoles(context.Background(), mid)
			if len(roles) != 0 {
				t.Fatalf("role must not be assigned, got %+v", roles)
			}
		})
	}
}

func TestScope_AssignRole_NonOperator_OrdinaryRole_StillAllowed(t *testing.T) {
	svc, repo := newScopeSvc(t, personaTenantAdmin)
	seedScopeRole(t, repo, "role_viewer", "viewer", "disclosure.view")
	mid := newMembershipWithoutRole(t, svc, repo)
	if err := svc.AssignRole(context.Background(), caapp.AssignRoleRequest{
		Subject: scopeSubject(), MembershipID: mid, RoleID: "role_viewer",
	}); err != nil {
		t.Fatalf("ordinary role must stay assignable: %v", err)
	}
}

func TestScope_AssignRole_Operator_PlatformCapableRole_Allowed(t *testing.T) {
	svc, repo := newScopeSvc(t, personaPlatformOp)
	seedScopeRole(t, repo, "role_platform_like", "ops_like", "platform.cms.view", "rbac.manage")
	mid := newMembershipWithoutRole(t, svc, repo)
	if err := svc.AssignRole(context.Background(), caapp.AssignRoleRequest{
		Subject: scopeSubject(), MembershipID: mid, RoleID: "role_platform_like",
	}); err != nil {
		t.Fatalf("operator must be able to assign: %v", err)
	}
}
