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

// H7 T11: the status API tells the truth about delivery — off / shadow / enforced — and whether the
// company runs on the defaults.
func TestAdminService_GetNotificationRuleStatus_H7Modes(t *testing.T) {
	sub := caapp.AdminSubject{UserID: "u_owner", MembershipID: "m_owner", CompanyID: "c_001"}
	newSvc := func(opts ...caapp.AdminOption) caapp.AdminService {
		return caapp.NewAdminService(
			cainmem.NewAdminRepository(),
			fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"rbac.manage"}},
			fixedIDGen("u_owner"),
			opts...,
		)
	}
	cases := []struct {
		name                    string
		opts                    []caapp.AdminOption
		enforced, shadow, deflt bool
	}{
		{"off", nil, false, false, true},
		{"shadow", []caapp.AdminOption{caapp.WithNotificationRulesShadowEnabled(true)}, false, true, true},
		{"enforced", []caapp.AdminOption{caapp.WithNotificationRulesConsumerEnabled(true)}, true, false, true},
		{"enforced wins over shadow", []caapp.AdminOption{caapp.WithNotificationRulesConsumerEnabled(true), caapp.WithNotificationRulesShadowEnabled(true)}, true, false, true},
	}
	for _, tc := range cases {
		status, err := newSvc(tc.opts...).GetNotificationRuleStatus(context.Background(), caapp.GetNotificationRuleStatusRequest{Subject: sub})
		if err != nil {
			t.Fatalf("%s: GetNotificationRuleStatus: %v", tc.name, err)
		}
		if status.DispatchEnforcementEnabled != tc.enforced || status.RuntimeConsumerEnabled != tc.enforced {
			t.Errorf("%s: enforcement=%v runtime=%v want %v", tc.name, status.DispatchEnforcementEnabled, status.RuntimeConsumerEnabled, tc.enforced)
		}
		if status.ShadowModeEnabled != tc.shadow {
			t.Errorf("%s: shadow=%v want %v", tc.name, status.ShadowModeEnabled, tc.shadow)
		}
		if status.DefaultsApplied != tc.deflt {
			t.Errorf("%s: defaults_applied=%v want %v", tc.name, status.DefaultsApplied, tc.deflt)
		}
	}

	svc := newSvc(caapp.WithNotificationRulesShadowEnabled(true))
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
	if status.DefaultsApplied || !status.ShadowModeEnabled {
		t.Fatalf("saved prefs under shadow: defaults_applied=%v shadow=%v", status.DefaultsApplied, status.ShadowModeEnabled)
	}
}

// H7 API-02: a disabled prefs row (no reminders once applied) is reported as ui_state=disabled.
func TestAdminService_GetNotificationRuleStatus_DisabledRow(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	svc := caapp.NewAdminService(
		repo,
		fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"rbac.manage"}},
		fixedIDGen("u_owner"),
		caapp.WithNotificationRulesConsumerEnabled(true),
	)
	sub := caapp.AdminSubject{UserID: "u_owner", MembershipID: "m_owner", CompanyID: "c_001"}
	payload := caapp.DefaultAlertChannelPrefsPayload(sub.MembershipID)
	payload["company_id"] = sub.CompanyID
	payload["rule_code"] = caapp.AlertChannelPrefsRuleCode
	if err := svc.CreateNotificationRule(context.Background(), caapp.CreateNotificationRuleRequest{Subject: sub, Payload: payload}); err != nil {
		t.Fatalf("CreateNotificationRule: %v", err)
	}
	rules, err := repo.ListNotificationRules(context.Background(), sub.CompanyID)
	if err != nil || len(rules) != 1 {
		t.Fatalf("ListNotificationRules: %v (%d rules)", err, len(rules))
	}
	inactive := "inactive"
	if err := repo.UpdateNotificationRuleMerged(context.Background(), sub.CompanyID, rules[0].NotificationRuleID, nil, &inactive); err != nil {
		t.Fatalf("disable rule: %v", err)
	}
	status, err := svc.GetNotificationRuleStatus(context.Background(), caapp.GetNotificationRuleStatusRequest{Subject: sub})
	if err != nil {
		t.Fatalf("GetNotificationRuleStatus: %v", err)
	}
	if status.UIState != "disabled" || status.DefaultsApplied {
		t.Fatalf("ui_state=%q defaults_applied=%v, want disabled/false", status.UIState, status.DefaultsApplied)
	}
}
