package app

import "testing"

func TestFinalizeRoleListItem_SystemGlobal(t *testing.T) {
	item := RoleListItem{
		RoleCode:    "dept_lead",
		Status:      "active",
		Scope:       "global",
		IsBuiltin:   true,
		RoleType:    RoleTypeSystemGlobal,
		IsProtected: true,
	}
	FinalizeRoleListItem(&item)
	if !item.IsProtected || item.IsEditable {
		t.Fatalf("system global: protected=%v editable=%v", item.IsProtected, item.IsEditable)
	}
	if !item.IsBuiltin {
		t.Fatal("is_builtin legacy mapping should remain true for global")
	}
}

func TestFinalizeRoleListItem_TenantDefaultProtected(t *testing.T) {
	item := RoleListItem{
		RoleCode:    "admin_doanh_nghiep",
		Status:      "active",
		Scope:       "company",
		IsBuiltin:   false,
		RoleType:    RoleTypeTenantDefault,
		IsProtected: true,
	}
	FinalizeRoleListItem(&item)
	if !item.IsProtected || item.IsEditable {
		t.Fatalf("tenant default: protected=%v editable=%v", item.IsProtected, item.IsEditable)
	}
}

func TestFinalizeRoleListItem_TenantCustomEditable(t *testing.T) {
	item := RoleListItem{
		RoleCode:    "legal_reviewer",
		Status:      "active",
		Scope:       "company",
		IsBuiltin:   false,
		RoleType:    RoleTypeTenantCustom,
		IsProtected: false,
	}
	FinalizeRoleListItem(&item)
	if item.IsProtected || !item.IsEditable {
		t.Fatalf("tenant custom: protected=%v editable=%v", item.IsProtected, item.IsEditable)
	}
}

func TestFinalizeRoleListItem_InferFromLegacyFields(t *testing.T) {
	item := RoleListItem{
		RoleCode:  "user_thuong",
		Status:    "active",
		Scope:     "company",
		IsBuiltin: false,
	}
	FinalizeRoleListItem(&item)
	if item.RoleType != RoleTypeTenantDefault {
		t.Fatalf("role_type=%q want tenant_default", item.RoleType)
	}
	if !item.IsProtected || item.IsEditable {
		t.Fatalf("inferred tenant default should be protected and not editable")
	}
}

func TestFinalizeRoleListItem_LegacyTenantRoleSafeDefault(t *testing.T) {
	item := RoleListItem{
		RoleCode:  "full_access",
		Status:    "active",
		Scope:     "company",
		IsBuiltin: false,
	}
	FinalizeRoleListItem(&item)
	if item.RoleType != RoleTypeTenantDefault {
		t.Fatalf("role_type=%q want tenant_default", item.RoleType)
	}
	if !item.IsProtected {
		t.Fatal("unknown tenant role should default protected")
	}
}

func TestFinalizeRoleListItem_InactiveCustomNotEditable(t *testing.T) {
	item := RoleListItem{
		RoleCode:    "legal_reviewer",
		Status:      "inactive",
		Scope:       "company",
		RoleType:    RoleTypeTenantCustom,
		IsProtected: false,
	}
	FinalizeRoleListItem(&item)
	if item.IsEditable {
		t.Fatal("inactive custom role must not be editable")
	}
}
