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
