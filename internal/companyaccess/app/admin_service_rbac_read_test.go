package app_test

import (
	"context"
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
)

func TestAdminService_ListRolesPermissionsStructured(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	repo.SeedRole(caapp.RoleListItem{
		RoleID: "custom_test", RoleCode: "custom_test", RoleName: "Custom",
		Status: "active", Scope: "company",
		RoleType: caapp.RoleTypeTenantCustom, IsProtected: false, IsBuiltin: false,
	})
	svc := caapp.NewAdminService(
		repo,
		fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"rbac.manage", "system.settings"}},
		fixedIDGen("u_owner"),
	)
	sub := caapp.AdminSubject{UserID: "u_owner", MembershipID: "m_owner", CompanyID: "c_001"}

	perms, err := svc.ListPermissions(context.Background(), caapp.AdminSubjectRequest{Subject: sub})
	if err != nil {
		t.Fatalf("ListPermissions: %v", err)
	}
	if len(perms) == 0 {
		t.Fatal("expected permissions")
	}
	if perms[0].PermissionCode == "" {
		t.Fatal("expected permission_code")
	}

	roles, err := svc.ListRoles(context.Background(), caapp.AdminSubjectRequest{Subject: sub})
	if err != nil {
		t.Fatalf("ListRoles: %v", err)
	}
	var customRoleID string
	for _, r := range roles {
		if r.RoleType == caapp.RoleTypeTenantCustom && !r.IsProtected {
			customRoleID = r.RoleID
			break
		}
	}
	if customRoleID == "" {
		t.Fatal("expected unprotected custom role")
	}

	var permID string
	for _, p := range perms {
		if p.PermissionCode == "disclosure.view" {
			permID = p.PermissionID
			break
		}
	}
	if permID == "" {
		t.Fatal("expected disclosure.view permission")
	}

	if err := svc.AssignRolePermission(context.Background(), caapp.AssignRolePermissionRequest{
		Subject: sub, RoleID: customRoleID, PermissionID: permID,
	}); err != nil {
		t.Fatalf("AssignRolePermission: %v", err)
	}

	matrix, err := svc.ListRolePermissions(context.Background(), caapp.ListRolePermissionsRequest{Subject: sub, RoleID: customRoleID})
	if err != nil {
		t.Fatalf("ListRolePermissions: %v", err)
	}
	if len(matrix.Permissions) != 1 {
		t.Fatalf("len(matrix.Permissions)=%d want 1", len(matrix.Permissions))
	}
}

func TestAdminService_GetNotificationRuleStatus_NotConfigured(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	svc := caapp.NewAdminService(
		repo,
		fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"rbac.manage"}},
		fixedIDGen("u_owner"),
	)
	sub := caapp.AdminSubject{UserID: "u_owner", MembershipID: "m_owner", CompanyID: "c_001"}

	status, err := svc.GetNotificationRuleStatus(context.Background(), caapp.GetNotificationRuleStatusRequest{Subject: sub})
	if err != nil {
		t.Fatalf("GetNotificationRuleStatus: %v", err)
	}
	if status.StorageConfigured {
		t.Fatal("expected storage_configured=false")
	}
	if status.RuntimeConsumerEnabled {
		t.Fatal("must not fake runtime consumer enabled")
	}
	if status.UIState != "not_configured" {
		t.Fatalf("ui_state=%q", status.UIState)
	}
}

func TestAdminService_GetNotificationRuleStatus_StorageConfigured(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	svc := caapp.NewAdminService(
		repo,
		fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"rbac.manage"}},
		fixedIDGen("u_owner"),
	)
	sub := caapp.AdminSubject{UserID: "u_owner", MembershipID: "m_owner", CompanyID: "c_001"}

	payload := caapp.DefaultAlertChannelPrefsPayload(sub.MembershipID)
	payload["company_id"] = sub.CompanyID
	payload["rule_code"] = caapp.AlertChannelPrefsRuleCode
	if err := svc.CreateNotificationRule(context.Background(), caapp.CreateNotificationRuleRequest{Subject: sub, Payload: payload}); err != nil {
		t.Fatalf("CreateNotificationRule: %v", err)
	}

	status, err := svc.GetNotificationRuleStatus(context.Background(), caapp.GetNotificationRuleStatusRequest{Subject: sub})
	if err != nil {
		t.Fatalf("GetNotificationRuleStatus: %v", err)
	}
	if !status.StorageConfigured || !status.PayloadValid {
		t.Fatalf("status=%+v", status)
	}
	if status.UIState != "storage_configured" {
		t.Fatalf("ui_state=%q", status.UIState)
	}
	if status.RuntimeConsumerEnabled {
		t.Fatal("runtime must stay false unless env set")
	}
}
