package app

import "testing"

func TestLookupGrantPolicy_KnownTiers(t *testing.T) {
	cases := []struct {
		code       string
		wantTier   PermissionGrantTier
		wantCustom bool
	}{
		{"rbac.manage", GrantTierTenantAdminOnly, false},
		{"system.settings", GrantTierTenantAdminOnly, false},
		{"disclosure.publish", GrantTierHighRisk, false},
		{"cms.template.read", GrantTierSystemOnly, false},
		{"disclosure.view", GrantTierGrantable, true},
		{"unknown.fake.permission", GrantTierDeprecatedOrUnknown, false},
	}
	for _, tc := range cases {
		pol := LookupGrantPolicy(tc.code)
		if pol.GrantTier != tc.wantTier {
			t.Errorf("%s: grant_tier=%q want %q", tc.code, pol.GrantTier, tc.wantTier)
		}
		if pol.AllowedOnCustomRole != tc.wantCustom {
			t.Errorf("%s: allowed_on_custom_role=%v want %v", tc.code, pol.AllowedOnCustomRole, tc.wantCustom)
		}
	}
}

func TestLookupGrantPolicy_NotificationRulePrefix(t *testing.T) {
	pol := LookupGrantPolicy("admin.notification_rule.email")
	if pol.GrantTier != GrantTierDeprecatedOrUnknown {
		t.Fatalf("grant_tier=%q want deprecated_or_unknown", pol.GrantTier)
	}
	if pol.AllowedOnCustomRole {
		t.Fatal("expected blocked on custom role")
	}
}

func TestIsRoleProtectedForMutation(t *testing.T) {
	protected := RoleListItem{RoleType: RoleTypeTenantDefault, IsProtected: true}
	if !IsRoleProtectedForMutation(&protected) {
		t.Fatal("tenant_default must be protected")
	}
	custom := RoleListItem{RoleType: RoleTypeTenantCustom, IsProtected: false, Status: "active"}
	if IsRoleProtectedForMutation(&custom) {
		t.Fatal("unprotected tenant_custom must be editable")
	}
}

func TestValidateAssignPermissionGrant(t *testing.T) {
	if err := ValidateAssignPermissionGrant(LookupGrantPolicy("disclosure.view")); err != nil {
		t.Fatalf("grantable: %v", err)
	}
	if err := ValidateAssignPermissionGrant(LookupGrantPolicy("rbac.manage")); err == nil {
		t.Fatal("tenant_admin_only must reject")
	}
	if err := ValidateAssignPermissionGrant(LookupGrantPolicy("disclosure.publish")); err == nil {
		t.Fatal("high_risk must reject")
	}
}

func TestBuildGrantableCatalogItems_MergesDB(t *testing.T) {
	items := BuildGrantableCatalogItems([]PermissionListItem{
		{PermissionCode: "disclosure.view", PermissionName: "View", ModuleName: "disclosure"},
	})
	found := false
	for _, it := range items {
		if it.PermissionCode == "disclosure.view" {
			found = true
			if it.GrantTier != GrantTierGrantable {
				t.Fatalf("grant_tier=%q", it.GrantTier)
			}
		}
	}
	if !found {
		t.Fatal("disclosure.view missing from catalog")
	}
}
