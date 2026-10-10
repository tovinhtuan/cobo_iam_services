package app_test

import (
	"context"
	"testing"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// RP-08: removing a high-tier permission (TenantAdminOnly / HighRisk grant policy) needs a second
// admin whether it comes from a rollback or from the remove-permission routes.
var highTierCodes = []string{
	"admin.role.permission.assign", "admin.role.permission.remove", "ad_hoc_alert.process_control",
	"company.ownership.transfer", "disclosure_type.publish", "workflow.step.override",
	"template.workflow.override.reset", "alert.channels.manage",
}

func TestRemoveRolePermission_HighTierNeedsApproval(t *testing.T) {
	for _, code := range highTierCodes {
		t.Run(code, func(t *testing.T) {
			fx := newRollbackFixture(t)
			fx.repo.SeedPermission(caapp.PermissionListItem{PermissionID: code, PermissionCode: code, PermissionName: code, ModuleName: "admin"})
			fx.add(rbRoleCustom, code)
			fx.snapshot()
			err := fx.svc.RemoveRolePermission(context.Background(), caapp.RemoveRolePermissionRequest{
				Subject: fx.owner, RoleID: rbRoleCustom, PermissionID: code,
			})
			requireHTTPCode(t, err, 202, perr.CodeApprovalRouted)
			requireHas(t, "custom role", fx.perms(rbRoleCustom), code)
		})
	}
}

func TestRemoveDirectPermission_HighTierNeedsApproval(t *testing.T) {
	for _, code := range highTierCodes {
		t.Run(code, func(t *testing.T) {
			fx := newRollbackFixture(t)
			if err := fx.repo.InsertDirectPermission(context.Background(), fx.approver.MembershipID, "c_001", code, "u_seed"); err != nil {
				t.Fatal(err)
			}
			err := fx.svc.RemoveDirectPermission(context.Background(), caapp.RemoveDirectPermissionRequest{
				Subject: fx.owner, MembershipID: fx.approver.MembershipID, PermissionCode: code,
			})
			requireHTTPCode(t, err, 202, perr.CodeApprovalRouted)
			if ok, _ := fx.repo.HasActiveDirectPermission(context.Background(), fx.approver.MembershipID, code); !ok {
				t.Fatalf("%s was removed without approval", code)
			}
		})
	}
}
