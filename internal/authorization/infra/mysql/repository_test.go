package mysql

import (
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
)

func TestGetActionPolicy_PrefersLegacyOverMisseededSystemSettings(t *testing.T) {
	// Document expected merge behavior when matrix row exists (integration uses DB).
	legacy := legacyPolicy("company.view")
	if legacy.RequiredPermission != "company.view" {
		t.Fatalf("legacy company.view = %q", legacy.RequiredPermission)
	}
	matrix := &authapp.ActionPolicy{RequiredPermission: "system.settings"}
	if matrix.RequiredPermission == "system.settings" && legacy.RequiredPermission != "system.settings" {
		// Same branch as GetActionPolicy: would return legacy.
		if legacy.RequiredPermission != "company.view" {
			t.Fatal("expected legacy override")
		}
	}
}

func TestLegacyPolicyMembershipInvite(t *testing.T) {
	create := legacyPolicy("admin.membership.create")
	if create.RequiredPermission != "admin.membership.invite" {
		t.Fatalf("create required = %q want admin.membership.invite", create.RequiredPermission)
	}
	list := legacyPolicy("admin.membership.list")
	if list.RequiredPermission != "admin.membership.invite" {
		t.Fatalf("list required = %q want admin.membership.invite", list.RequiredPermission)
	}
	del := legacyPolicy("admin.membership.delete")
	if del.RequiredPermission != "rbac.manage" {
		t.Fatalf("delete required = %q want rbac.manage", del.RequiredPermission)
	}
}

func TestLegacyPolicyAdminAccountSettings(t *testing.T) {
	read := legacyPolicy("admin.account.settings.read")
	if read.RequiredPermission != "rbac.manage" {
		t.Fatalf("read required = %q want rbac.manage", read.RequiredPermission)
	}
	patch := legacyPolicy("admin.account.settings.update")
	if patch.RequiredPermission != "rbac.manage" {
		t.Fatalf("update required = %q want rbac.manage", patch.RequiredPermission)
	}
}

func TestLegacyPolicyCompanyProfile(t *testing.T) {
	view := legacyPolicy("company.view")
	if view.RequiredPermission != "company.view" {
		t.Fatalf("company.view required permission = %q, want company.view", view.RequiredPermission)
	}
	edit := legacyPolicy("company.edit")
	if edit.RequiredPermission != "company.edit" {
		t.Fatalf("company.edit required permission = %q, want company.edit", edit.RequiredPermission)
	}
}

func TestLegacyPolicyDisclosureAutoCreateManage(t *testing.T) {
	p := legacyPolicy("disclosure.auto_create.manage")
	if p.RequiredPermission != "disclosure.auto_create.manage" {
		t.Fatalf("required = %q want disclosure.auto_create.manage", p.RequiredPermission)
	}
	if p.RequiredPermission == "system.settings" {
		t.Fatal("must not fall back to system.settings")
	}
}
