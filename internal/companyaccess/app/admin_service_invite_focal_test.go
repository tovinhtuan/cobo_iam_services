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

func seedEnterpriseInviteRoles(t *testing.T, repo *cainmem.AdminRepository) {
	t.Helper()
	for _, code := range []string{
		"user_thuong", "dept_lead", "admin_web", "cms_operator", "full_access",
		"truong_phong_ban", "truong_nhom", "company_admin",
	} {
		if err := repo.AddRolePermission(context.Background(), code, "admin.membership.invite"); err != nil {
			t.Fatalf("seed role %s: %v", code, err)
		}
	}
}

func newEnterpriseInviteSvc(t *testing.T, repo *cainmem.AdminRepository, sub caapp.AdminSubject) caapp.AdminService {
	t.Helper()
	seedInviteScopedSubject(t, repo, sub)
	seedEnterpriseInviteRoles(t, repo)
	return caapp.NewAdminService(
		repo,
		fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"system.settings", "admin.membership.invite"}},
		idgen.UUIDv7Generator{},
	)
}

func TestInviteUser_DepartmentFocal_Required(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"}
	svc := newEnterpriseInviteSvc(t, repo, sub)

	_, err := svc.InviteUser(context.Background(), caapp.InviteUserRequest{
		Subject:            sub,
		Email:              "focal.empty@example.com",
		FullName:           "Focal Empty",
		CompanyID:          "c_001",
		CreatedByUserID:    sub.UserID,
		IsDepartmentFocal:  true,
		FocalDepartmentIDs: []string{},
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("want 400 invalid_request, got %v", err)
	}
}

func TestInviteUser_DepartmentFocal_Happy(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"}
	repo.SeedDepartmentForCompany("c_001", caapp.DepartmentView{DepartmentID: "dep_a", Name: "Dept A", Status: "active"})
	repo.SeedDepartmentForCompany("c_001", caapp.DepartmentView{DepartmentID: "dep_b", Name: "Dept B", Status: "active"})
	svc := newEnterpriseInviteSvc(t, repo, sub)

	out, err := svc.InviteUser(context.Background(), caapp.InviteUserRequest{
		Subject:            sub,
		Email:              "focal.happy@example.com",
		FullName:           "Focal Happy",
		CompanyID:          "c_001",
		CreatedByUserID:    sub.UserID,
		IsDepartmentFocal:  true,
		FocalDepartmentIDs: []string{"dep_a", "dep_b"},
	})
	if err != nil {
		t.Fatalf("InviteUser: %v", err)
	}
	if out.MembershipID == "" {
		t.Fatal("expected membership id")
	}
	ids, err := repo.ListFocalDepartmentIDs(context.Background(), out.MembershipID)
	if err != nil {
		t.Fatalf("ListFocalDepartmentIDs: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("focal dept count=%d want 2 (%v)", len(ids), ids)
	}
}

func TestInviteUser_DepartmentFocal_DepartmentOutsideCompany(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"}
	repo.SeedDepartmentForCompany("c_other", caapp.DepartmentView{DepartmentID: "dep_other", Name: "Other", Status: "active"})
	svc := newEnterpriseInviteSvc(t, repo, sub)

	_, err := svc.InviteUser(context.Background(), caapp.InviteUserRequest{
		Subject:            sub,
		Email:              "focal.other@example.com",
		FullName:           "Focal Other Co",
		CompanyID:          "c_001",
		CreatedByUserID:    sub.UserID,
		IsDepartmentFocal:  true,
		FocalDepartmentIDs: []string{"dep_other"},
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("want 400, got %v", err)
	}
}

func TestInviteUser_ScopedInviter_FocalOutOfScope(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	repo.SeedDepartmentForCompany("c_del", caapp.DepartmentView{DepartmentID: "dep_a", Name: "Dept A", Status: "active"})
	repo.SeedDepartmentForCompany("c_del", caapp.DepartmentView{DepartmentID: "dep_b", Name: "Dept B", Status: "active"})
	delegator := seedDelegator(t, repo)
	seedDelegatee(t, repo, "m_delegatee", "dep_a")
	svcAdmin := newDelegationSvc(t, repo, []string{"rbac.manage", "admin.membership.invite"})
	delegatee := caapp.AdminSubject{UserID: "m_delegatee_u", MembershipID: "m_delegatee", CompanyID: "c_del"}
	seedEnterpriseInviteRoles(t, repo)

	_, err := svcAdmin.CreateDelegation(context.Background(), caapp.CreateDelegationRequest{
		Subject: delegator, DelegateeMembershipID: "m_delegatee",
		ScopeType: "department", ScopeID: "dep_a",
		PermissionSet: []string{"admin.membership.invite"},
	})
	if err != nil {
		t.Fatalf("grant: %v", err)
	}

	svcDel := newDelegationSvc(t, repo, []string{"admin.membership.create", "admin.membership.invite"})
	_, err = svcDel.InviteUser(context.Background(), caapp.InviteUserRequest{
		Subject:            delegatee,
		Email:              "scoped.focal@example.com",
		FullName:           "Scoped Focal",
		CompanyID:          "c_del",
		CreatedByUserID:    delegatee.UserID,
		IsDepartmentFocal:  true,
		FocalDepartmentIDs: []string{"dep_b"},
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusForbidden {
		t.Fatalf("want 403, got %v", err)
	}
}

func TestInviteUser_RejectsDeptLeadRole(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"}
	svc := newEnterpriseInviteSvc(t, repo, sub)

	_, err := svc.InviteUser(context.Background(), caapp.InviteUserRequest{
		Subject:         sub,
		Email:           "deptlead@example.com",
		FullName:        "Dept Lead",
		CompanyID:       "c_001",
		CreatedByUserID: sub.UserID,
		RoleCode:        "dept_lead",
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("want 400, got %v", err)
	}
}

func TestListInviteRoles_EnterpriseFilter(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"}
	svc := newEnterpriseInviteSvc(t, repo, sub)

	items, err := svc.ListInviteRoles(context.Background(), caapp.ListInviteRolesRequest{
		Subject:   sub,
		CompanyID: "c_001",
	})
	if err != nil {
		t.Fatalf("ListInviteRoles: %v", err)
	}
	denied := map[string]struct{}{
		"dept_lead": {}, "admin_web": {}, "cms_operator": {}, "full_access": {},
		"truong_phong_ban": {}, "truong_nhom": {}, "self_reg_company_owner": {},
	}
	for _, item := range items {
		if _, bad := denied[item.RoleCode]; bad {
			t.Fatalf("denied role leaked: %s", item.RoleCode)
		}
	}
}

func TestInviteUser_RejectsCMSPermission(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"}
	svc := newEnterpriseInviteSvc(t, repo, sub)

	_, err := svc.InviteUser(context.Background(), caapp.InviteUserRequest{
		Subject:         sub,
		Email:           "cms.perm@example.com",
		FullName:        "CMS Perm",
		CompanyID:       "c_001",
		CreatedByUserID: sub.UserID,
		Permissions:     []string{"platform.cms.view"},
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("want 400, got %v", err)
	}
}

func TestInviteUser_RejectsCMSPrefixPermission(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"}
	svc := newEnterpriseInviteSvc(t, repo, sub)

	_, err := svc.InviteUser(context.Background(), caapp.InviteUserRequest{
		Subject:         sub,
		Email:           "cms.write@example.com",
		FullName:        "CMS Write",
		CompanyID:       "c_001",
		CreatedByUserID: sub.UserID,
		Permissions:     []string{"cms.template.write"},
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("want 400, got %v", err)
	}
}

func TestInviteUser_RejectsRbacManageDirectPermission(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"}
	svc := newEnterpriseInviteSvc(t, repo, sub)

	_, err := svc.InviteUser(context.Background(), caapp.InviteUserRequest{
		Subject:         sub,
		Email:           "rbac@example.com",
		FullName:        "RBAC",
		CompanyID:       "c_001",
		CreatedByUserID: sub.UserID,
		Permissions:     []string{"rbac.manage"},
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("want 400, got %v", err)
	}
}

func TestInviteUser_AcceptsWorkflowAndAdHocPermissions(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"}
	svc := newEnterpriseInviteSvc(t, repo, sub)

	_, err := svc.InviteUser(context.Background(), caapp.InviteUserRequest{
		Subject:         sub,
		Email:           "perms.ok@example.com",
		FullName:        "Perms OK",
		CompanyID:       "c_001",
		CreatedByUserID: sub.UserID,
		Permissions: []string{
			"template.workflow.override.write",
			"disclosure_type.manage",
			"ad_hoc_alert.propose",
			"ad_hoc_alert.focal_review",
		},
	})
	if err != nil {
		t.Fatalf("InviteUser: %v", err)
	}
}

func TestCreateUser_RejectsRbacManageDirectPermission(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"}
	svc := newEnterpriseInviteSvc(t, repo, sub)

	_, err := svc.CreateUser(context.Background(), caapp.CreateUserRequest{
		Subject:     sub,
		LoginID:     "create.rbac@example.com",
		Password:    "StrongPass123!",
		FullName:    "Create RBAC",
		CompanyID:   "c_001",
		Permissions: []string{"rbac.manage"},
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("want 400, got %v", err)
	}
}

func TestInviteUser_RejectsWorkflowReadInPayload(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"}
	svc := newEnterpriseInviteSvc(t, repo, sub)

	_, err := svc.InviteUser(context.Background(), caapp.InviteUserRequest{
		Subject:         sub,
		Email:           "read.only@example.com",
		FullName:        "Read Only",
		CompanyID:       "c_001",
		CreatedByUserID: sub.UserID,
		Permissions:     []string{"template.workflow.override.read"},
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("want 400, got %v", err)
	}
}

func TestCreateUser_DepartmentFocal_Happy(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"}
	repo.SeedDepartmentForCompany("c_001", caapp.DepartmentView{DepartmentID: "dep_a", Name: "Dept A", Status: "active"})
	svc := newEnterpriseInviteSvc(t, repo, sub)

	out, err := svc.CreateUser(context.Background(), caapp.CreateUserRequest{
		Subject:            sub,
		LoginID:            "create.focal@example.com",
		Password:           "StrongPass123!",
		FullName:           "Create Focal",
		CompanyID:          "c_001",
		IsDepartmentFocal:  true,
		FocalDepartmentIDs: []string{"dep_a"},
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	ids, err := repo.ListFocalDepartmentIDs(context.Background(), out.MembershipID)
	if err != nil || len(ids) != 1 || ids[0] != "dep_a" {
		t.Fatalf("focal ids=%v err=%v", ids, err)
	}
}

func TestCreateUser_DepartmentFocal_Required(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"}
	svc := newEnterpriseInviteSvc(t, repo, sub)

	_, err := svc.CreateUser(context.Background(), caapp.CreateUserRequest{
		Subject:           sub,
		LoginID:           "create.nofocal@example.com",
		Password:          "StrongPass123!",
		FullName:          "No Focal Dept",
		CompanyID:         "c_001",
		IsDepartmentFocal: true,
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("want 400, got %v", err)
	}
}

func TestInviteUser_RejectsDeniedEnterpriseRoles(t *testing.T) {
	denied := []string{
		"dept_lead", "truong_phong_ban", "truong_nhom",
		"admin_web", "cms_operator", "full_access", "self_reg_company_owner",
	}
	for _, roleCode := range denied {
		t.Run(roleCode, func(t *testing.T) {
			repo := cainmem.NewAdminRepository()
			sub := caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"}
			svc := newEnterpriseInviteSvc(t, repo, sub)
			_, err := svc.InviteUser(context.Background(), caapp.InviteUserRequest{
				Subject:         sub,
				Email:           roleCode + "@example.com",
				FullName:        "Denied Role",
				CompanyID:       "c_001",
				CreatedByUserID: sub.UserID,
				RoleCode:        roleCode,
			})
			he, ok := perr.AsHTTPError(err)
			if !ok || he.HTTPStatus != http.StatusBadRequest {
				t.Fatalf("want 400, got %v", err)
			}
		})
	}
}

func TestCreateUser_RejectsDeniedEnterpriseRole(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"}
	svc := newEnterpriseInviteSvc(t, repo, sub)
	_, err := svc.CreateUser(context.Background(), caapp.CreateUserRequest{
		Subject:   sub,
		LoginID:   "create.denied@example.com",
		Password:  "StrongPass123!",
		FullName:  "Denied",
		CompanyID: "c_001",
		RoleCode:  "truong_phong_ban",
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("want 400, got %v", err)
	}
}
