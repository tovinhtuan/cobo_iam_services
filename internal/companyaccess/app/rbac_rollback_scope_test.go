package app_test

// ROLE-01 (CRITICAL): rolling back the RBAC matrix, or applying an RBAC approval, may only
// change tenant_custom roles of the caller's own company. Global roles, protected
// tenant_default roles (e.g. cms_operator) and roles of other companies must be left alone,
// and permissions outside the enterprise scope (cms/platform) must never be removed.
// A rollback that touches a critical permission has to go through the approval queue.
//
// See docs/ai-cache/bug-rbac-rollback-global-roles-2026-10-09/.

import (
	"context"
	"encoding/json"
	"testing"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	"github.com/cobo/cobo_iam_services/internal/companyaccess/configversion"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

const (
	rbRoleGlobalOwner   = "g_owner"      // system_global, shared by every company (self_reg_company_owner)
	rbRoleCMSOperator   = "cms_op"       // tenant_default, protected, carries platform/cms permissions
	rbRoleAdminDefault  = "admin_dn"     // tenant_default, protected
	rbRoleCustom        = "custom_a"     // tenant_custom of c_001
	rbRoleCustomOther   = "custom_other" // tenant_custom of c_002
	rbPermCritical      = "rbac.manage"
	rbPermPlatformView  = "platform.cms.view"
	rbPermCMSWrite      = "cms.template.write"
	rbPermComment       = "deadline.comment"
	rbPermDeadlineView  = "deadline.view"
	rbPermDisclosure    = "disclosure.view"
	rbPermDashboardView = "dashboard.view"

	rbDirectGrantable    = "template.workflow.override.read" // in GrantablePermissions, grant tier "grantable"
	rbDirectNotGrantable = "ad_hoc_alert.process_control"    // outside GrantablePermissions (tenant_admin_only)
)

type rollbackFixture struct {
	t        *testing.T
	repo     *cainmem.AdminRepository
	svc      caapp.AdminService
	owner    caapp.AdminSubject // company c_001, rbac.manage + system.settings
	approver caapp.AdminSubject // another membership of c_001
	other    caapp.AdminSubject // membership of c_002
}

func newRollbackFixture(t *testing.T) *rollbackFixture {
	t.Helper()
	ctx := context.Background()
	repo := cainmem.NewAdminRepository()
	fx := &rollbackFixture{
		t:        t,
		repo:     repo,
		owner:    caapp.AdminSubject{UserID: "u_rb_own", MembershipID: "m_rb_own", CompanyID: "c_001"},
		approver: caapp.AdminSubject{UserID: "u_rb_apv", MembershipID: "m_rb_apv", CompanyID: "c_001"},
		other:    caapp.AdminSubject{UserID: "u_rb_oth", MembershipID: "m_rb_oth", CompanyID: "c_002"},
	}
	seedInviteScopedSubject(t, repo, fx.owner)
	seedInviteScopedSubject(t, repo, fx.approver)
	seedInviteScopedSubject(t, repo, fx.other)

	for code, module := range map[string]string{
		rbPermCritical:      "admin",
		rbPermPlatformView:  "platform",
		rbPermCMSWrite:      "cms",
		rbPermComment:       "deadline",
		rbPermDeadlineView:  "deadline",
		rbPermDisclosure:    "disclosure",
		rbPermDashboardView: "dashboard",
	} {
		repo.SeedPermission(caapp.PermissionListItem{PermissionID: code, PermissionCode: code, PermissionName: code, ModuleName: module})
	}

	repo.SeedRole(caapp.RoleListItem{
		RoleID: rbRoleGlobalOwner, RoleCode: "self_reg_company_owner", RoleName: "Owner", Status: "active",
		Scope: "global", RoleType: caapp.RoleTypeSystemGlobal, IsProtected: true, IsBuiltin: true,
	})
	repo.SeedRoleForCompany(caapp.RoleListItem{
		RoleID: rbRoleCMSOperator, RoleCode: "cms_operator", RoleName: "CMS Operator", Status: "active",
		Scope: "company", RoleType: caapp.RoleTypeTenantDefault, IsProtected: true,
	}, "c_001")
	repo.SeedRoleForCompany(caapp.RoleListItem{
		RoleID: rbRoleAdminDefault, RoleCode: "admin_doanh_nghiep", RoleName: "Admin", Status: "active",
		Scope: "company", RoleType: caapp.RoleTypeTenantDefault, IsProtected: true,
	}, "c_001")
	repo.SeedRoleForCompany(caapp.RoleListItem{
		RoleID: rbRoleCustom, RoleCode: "custom_a", RoleName: "Custom A", Status: "active",
		Scope: "company", RoleType: caapp.RoleTypeTenantCustom,
	}, "c_001")
	repo.SeedRoleForCompany(caapp.RoleListItem{
		RoleID: rbRoleCustomOther, RoleCode: "custom_other", RoleName: "Custom Other", Status: "active",
		Scope: "company", RoleType: caapp.RoleTypeTenantCustom,
	}, "c_002")

	for role, perms := range map[string][]string{
		rbRoleCMSOperator:  {rbPermPlatformView, rbPermCMSWrite, rbPermDisclosure},
		rbRoleAdminDefault: {rbPermDisclosure, rbPermDeadlineView},
		rbRoleGlobalOwner:  {rbPermDisclosure},
		rbRoleCustom:       {rbPermDisclosure},
		rbRoleCustomOther:  {rbPermDisclosure},
	} {
		for _, p := range perms {
			if err := repo.AddRolePermission(ctx, role, p); err != nil {
				t.Fatalf("seed role permission %s/%s: %v", role, p, err)
			}
		}
	}
	fx.svc = newApprovalSvc(t, repo)
	return fx
}

// snapshot captures matrix version 1 through a real, allowed mutation on the custom role.
func (fx *rollbackFixture) snapshot() {
	fx.t.Helper()
	if err := fx.svc.AssignRolePermission(context.Background(), caapp.AssignRolePermissionRequest{
		Subject: fx.owner, RoleID: rbRoleCustom, PermissionID: rbPermComment,
	}); err != nil {
		fx.t.Fatalf("seed snapshot mutation: %v", err)
	}
}

func (fx *rollbackFixture) rollback(version int) (*caapp.ConfigVersionRow, error) {
	return fx.svc.RollbackRBACMatrixVersion(context.Background(), caapp.RollbackRBACMatrixVersionRequest{
		Subject: fx.owner, VersionNo: version, Reason: "test",
	})
}

// perms reads the raw role_permissions state straight from the repository.
func (fx *rollbackFixture) perms(roleID string) map[string]bool {
	fx.t.Helper()
	view, err := fx.repo.ListRolePermissions(context.Background(), "c_001", roleID)
	if err != nil {
		fx.t.Fatalf("list role permissions %s: %v", roleID, err)
	}
	out := map[string]bool{}
	for _, p := range view.Permissions {
		out[p.PermissionID] = true
	}
	return out
}

func (fx *rollbackFixture) add(roleID, perm string) {
	fx.t.Helper()
	if err := fx.repo.AddRolePermission(context.Background(), roleID, perm); err != nil {
		fx.t.Fatalf("add %s/%s: %v", roleID, perm, err)
	}
}

func (fx *rollbackFixture) remove(roleID, perm string) {
	fx.t.Helper()
	if err := fx.repo.RemoveRolePermission(context.Background(), roleID, perm); err != nil {
		fx.t.Fatalf("remove %s/%s: %v", roleID, perm, err)
	}
}

func (fx *rollbackFixture) pending() []caapp.PendingAdminChangeSummary {
	fx.t.Helper()
	list, err := fx.svc.ListConfigApprovals(context.Background(), caapp.ListConfigApprovalsRequest{
		Subject: fx.owner, Status: configversion.ApprovalStatusPending,
	})
	if err != nil {
		fx.t.Fatalf("list approvals: %v", err)
	}
	return list.Items
}

func (fx *rollbackFixture) approve(id string) error {
	_, err := fx.svc.ApproveConfigApproval(context.Background(), caapp.ApproveConfigApprovalRequest{
		Subject: fx.approver, ApprovalID: id,
	})
	return err
}

func requireHas(t *testing.T, label string, got map[string]bool, perm string) {
	t.Helper()
	if !got[perm] {
		t.Errorf("%s lost %q (now %v)", label, perm, got)
	}
}

func requireLacks(t *testing.T, label string, got map[string]bool, perm string) {
	t.Helper()
	if got[perm] {
		t.Errorf("%s unexpectedly has %q (now %v)", label, perm, got)
	}
}

func requireHTTPCode(t *testing.T, err error, status int, code perr.Code) {
	t.Helper()
	he, ok := perr.AsHTTPError(err)
	if !ok {
		t.Fatalf("expected HTTP error %d/%s, got %v", status, code, err)
	}
	if he.HTTPStatus != status || he.Code != code {
		t.Fatalf("expected %d/%s, got %d/%s", status, code, he.HTTPStatus, he.Code)
	}
}

// --- rollback must not touch roles outside the tenant's own custom roles -----------------

func TestRBACRollback_KeepsGlobalRolePermissions(t *testing.T) {
	fx := newRollbackFixture(t)
	fx.snapshot()
	// Permission added to the shared owner role after the snapshot (e.g. by a platform release).
	fx.add(rbRoleGlobalOwner, rbPermDeadlineView)

	if _, err := fx.rollback(1); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	requireHas(t, "global role "+rbRoleGlobalOwner, fx.perms(rbRoleGlobalOwner), rbPermDeadlineView)
}

func TestRBACRollback_KeepsPlatformPermsOnTenantDefaultRole(t *testing.T) {
	fx := newRollbackFixture(t)
	fx.snapshot()

	if _, err := fx.rollback(1); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	got := fx.perms(rbRoleCMSOperator)
	requireHas(t, "cms_operator", got, rbPermPlatformView)
	requireHas(t, "cms_operator", got, rbPermCMSWrite)
}

func TestRBACRollback_DoesNotModifyProtectedTenantDefaultRole(t *testing.T) {
	fx := newRollbackFixture(t)
	fx.snapshot()
	fx.add(rbRoleAdminDefault, rbPermDashboardView)
	fx.remove(rbRoleAdminDefault, rbPermDeadlineView)

	if _, err := fx.rollback(1); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	got := fx.perms(rbRoleAdminDefault)
	requireHas(t, "admin_doanh_nghiep", got, rbPermDashboardView)
	requireLacks(t, "admin_doanh_nghiep", got, rbPermDeadlineView)
}

func TestRBACRollback_RestoresTenantCustomRole(t *testing.T) {
	fx := newRollbackFixture(t)
	fx.snapshot()
	fx.add(rbRoleCustom, rbPermDashboardView)
	fx.remove(rbRoleCustom, rbPermComment)

	row, err := fx.rollback(1)
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if row == nil || row.Source != configversion.SourceRollback {
		t.Fatalf("expected a rollback version row, got %+v", row)
	}
	got := fx.perms(rbRoleCustom)
	requireHas(t, "custom role", got, rbPermComment)
	requireLacks(t, "custom role", got, rbPermDashboardView)
}

func TestRBACRollback_DoesNotTouchOtherCompany(t *testing.T) {
	fx := newRollbackFixture(t)
	fx.snapshot()
	fx.add(rbRoleCustomOther, rbPermDashboardView)
	if err := fx.repo.InsertDirectPermission(context.Background(), fx.other.MembershipID, "c_002", rbPermDisclosure, "u_x"); err != nil {
		t.Fatalf("seed direct permission: %v", err)
	}

	if _, err := fx.rollback(1); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	requireHas(t, "c_002 custom role", fx.perms(rbRoleCustomOther), rbPermDashboardView)
	direct, err := fx.repo.ListActiveDirectPermissions(context.Background(), fx.other.MembershipID)
	if err != nil {
		t.Fatalf("list direct permissions: %v", err)
	}
	if len(direct) != 1 {
		t.Fatalf("c_002 direct permission was revoked by a c_001 rollback: %+v", direct)
	}
}

// --- approval apply must follow the same scope --------------------------------------------

func TestRBACApprovalApply_KeepsOutOfScopeRoles(t *testing.T) {
	fx := newRollbackFixture(t)
	fx.snapshot()
	fx.add(rbRoleCustom, rbPermCritical)
	ctx := context.Background()

	// Removing a critical permission from a custom role is routed to approval (existing behaviour).
	err := fx.svc.RemoveRolePermission(ctx, caapp.RemoveRolePermissionRequest{
		Subject: fx.owner, RoleID: rbRoleCustom, PermissionID: rbPermCritical,
	})
	requireHTTPCode(t, err, 202, perr.CodeApprovalRouted)
	items := fx.pending()
	if len(items) != 1 {
		t.Fatalf("expected 1 pending approval, got %d", len(items))
	}

	// State of roles outside the tenant's custom roles drifts while the approval is pending.
	fx.add(rbRoleGlobalOwner, rbPermDeadlineView)

	if err := fx.approve(items[0].ApprovalID); err != nil {
		t.Fatalf("approve: %v", err)
	}
	requireLacks(t, "custom role", fx.perms(rbRoleCustom), rbPermCritical)
	requireHas(t, "global role", fx.perms(rbRoleGlobalOwner), rbPermDeadlineView)
	cms := fx.perms(rbRoleCMSOperator)
	requireHas(t, "cms_operator", cms, rbPermPlatformView)
	requireHas(t, "cms_operator", cms, rbPermCMSWrite)
}

func submitRolePermRemove(fx *rollbackFixture, roleID, permID string) error {
	_, err := fx.svc.SubmitConfigApproval(context.Background(), caapp.SubmitConfigApprovalRequest{
		Subject:       fx.owner,
		AggregateType: configversion.AggregateRBACMatrix,
		ChangeType:    configversion.ChangeTypeRBACPermissionRemove,
		Proposed:      map[string]any{"role_id": roleID, "permission_id": permID},
	})
	return err
}

func TestSubmitConfigApproval_RBACPermRemove_RejectsProtectedAndGlobalRoles(t *testing.T) {
	fx := newRollbackFixture(t)
	fx.snapshot()

	requireHTTPCode(t, submitRolePermRemove(fx, rbRoleCMSOperator, rbPermPlatformView), 403, perr.CodeProtectedRoleReadOnly)
	requireHTTPCode(t, submitRolePermRemove(fx, rbRoleAdminDefault, rbPermDisclosure), 403, perr.CodeProtectedRoleReadOnly)
	requireHTTPCode(t, submitRolePermRemove(fx, rbRoleGlobalOwner, rbPermDisclosure), 403, perr.CodeProtectedRoleReadOnly)
	requireHTTPCode(t, submitRolePermRemove(fx, rbRoleCustomOther, rbPermDisclosure), 404, perr.CodeNotFound)
	if n := len(fx.pending()); n != 0 {
		t.Fatalf("rejected submissions must not be queued, got %d pending", n)
	}
}

func TestSubmitConfigApproval_RBACPermRemove_RejectsOutOfEnterprisePermission(t *testing.T) {
	fx := newRollbackFixture(t)
	fx.snapshot()
	requireHTTPCode(t, submitRolePermRemove(fx, rbRoleCustom, rbPermPlatformView), 400, perr.CodePermissionOutOfEnterpriseScope)
}

func TestSubmitConfigApproval_RBACPermRemove_AcceptsCustomRole(t *testing.T) {
	fx := newRollbackFixture(t)
	fx.snapshot()
	fx.add(rbRoleCustom, rbPermCritical)
	if err := submitRolePermRemove(fx, rbRoleCustom, rbPermCritical); err != nil {
		t.Fatalf("submit on a custom role must still be accepted: %v", err)
	}
	if n := len(fx.pending()); n != 1 {
		t.Fatalf("expected 1 pending approval, got %d", n)
	}
}

// --- a rollback that touches a critical permission needs approval --------------------------

const rbChangeTypeMatrixRollback = "rbac.matrix.rollback"

func TestRBACRollback_CriticalChange_RoutesToApproval(t *testing.T) {
	fx := newRollbackFixture(t)
	fx.snapshot() // version 1: the custom role holds no critical permission
	// Legacy data: a critical permission that the grant policy would never allow on a custom role.
	fx.add(rbRoleCustom, rbPermCritical)

	_, err := fx.rollback(1) // would remove the critical permission
	requireHTTPCode(t, err, 202, perr.CodeApprovalRouted)
	requireHas(t, "custom role (nothing may change before approval)", fx.perms(rbRoleCustom), rbPermCritical)

	items := fx.pending()
	if len(items) != 1 || items[0].ChangeType != rbChangeTypeMatrixRollback {
		t.Fatalf("expected one pending %s, got %+v", rbChangeTypeMatrixRollback, items)
	}

	// The requester cannot approve their own rollback.
	_, err = fx.svc.ApproveConfigApproval(context.Background(), caapp.ApproveConfigApprovalRequest{
		Subject: fx.owner, ApprovalID: items[0].ApprovalID,
	})
	requireHTTPCode(t, err, 403, perr.CodeSelfApprovalNotAllowed)

	// Another approver applies it, and still only the custom role changes.
	fx.add(rbRoleGlobalOwner, rbPermDeadlineView)
	if err := fx.approve(items[0].ApprovalID); err != nil {
		t.Fatalf("approve: %v", err)
	}
	requireLacks(t, "custom role", fx.perms(rbRoleCustom), rbPermCritical)
	requireHas(t, "global role", fx.perms(rbRoleGlobalOwner), rbPermDeadlineView)
	requireHas(t, "cms_operator", fx.perms(rbRoleCMSOperator), rbPermPlatformView)
}

func TestRBACRollback_NonCriticalChange_AppliesDirectly(t *testing.T) {
	fx := newRollbackFixture(t)
	fx.snapshot()
	fx.add(rbRoleCustom, rbPermDashboardView)

	if _, err := fx.rollback(1); err != nil {
		t.Fatalf("a non-critical rollback must apply without approval: %v", err)
	}
	requireLacks(t, "custom role", fx.perms(rbRoleCustom), rbPermDashboardView)
	if n := len(fx.pending()); n != 0 {
		t.Fatalf("expected no pending approval, got %d", n)
	}
}

func TestRBACRollback_CriticalApproval_StaleAfterNewVersion(t *testing.T) {
	fx := newRollbackFixture(t)
	fx.snapshot()
	fx.add(rbRoleCustom, rbPermCritical)
	_, err := fx.rollback(1)
	requireHTTPCode(t, err, 202, perr.CodeApprovalRouted)
	items := fx.pending()
	if len(items) != 1 {
		t.Fatalf("expected 1 pending approval, got %d", len(items))
	}

	// Another change lands before the approval is decided.
	if err := fx.svc.AssignRolePermission(context.Background(), caapp.AssignRolePermissionRequest{
		Subject: fx.owner, RoleID: rbRoleCustom, PermissionID: rbPermDashboardView,
	}); err != nil {
		t.Fatalf("second mutation: %v", err)
	}
	requireHTTPCode(t, fx.approve(items[0].ApprovalID), 409, perr.CodeStaleProposal)
}

// Direct grants are permissions too: restoring or revoking a critical one needs approval.
func TestRBACRollback_DirectCriticalChange_RoutesToApproval(t *testing.T) {
	fx := newRollbackFixture(t)
	ctx := context.Background()
	const critical = "admin.membership.invite"
	if err := fx.repo.InsertDirectPermission(ctx, fx.owner.MembershipID, "c_001", critical, "u_x"); err != nil {
		t.Fatalf("seed direct permission: %v", err)
	}
	fx.snapshot() // version 1 holds the direct grant
	if err := fx.repo.RevokeDirectPermission(ctx, fx.owner.MembershipID, critical, "u_x"); err != nil {
		t.Fatalf("revoke direct permission: %v", err)
	}

	_, err := fx.rollback(1)
	requireHTTPCode(t, err, 202, perr.CodeApprovalRouted)
	if direct, _ := fx.repo.ListActiveDirectPermissions(ctx, fx.owner.MembershipID); len(direct) != 0 {
		t.Fatalf("nothing may change before approval, got %+v", direct)
	}
	items := fx.pending()
	if len(items) != 1 {
		t.Fatalf("expected 1 pending approval, got %d", len(items))
	}
	if err := fx.approve(items[0].ApprovalID); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if direct, _ := fx.repo.ListActiveDirectPermissions(ctx, fx.owner.MembershipID); len(direct) != 1 {
		t.Fatalf("direct grant must be restored after approval, got %+v", direct)
	}
}

func TestRBACRollback_DirectNonCriticalChange_AppliesDirectly(t *testing.T) {
	fx := newRollbackFixture(t)
	ctx := context.Background()
	if err := fx.repo.InsertDirectPermission(ctx, fx.owner.MembershipID, "c_001", rbDirectGrantable, "u_x"); err != nil {
		t.Fatalf("seed direct permission: %v", err)
	}
	fx.snapshot()
	if err := fx.repo.RevokeDirectPermission(ctx, fx.owner.MembershipID, rbDirectGrantable, "u_x"); err != nil {
		t.Fatalf("revoke direct permission: %v", err)
	}

	if _, err := fx.rollback(1); err != nil {
		t.Fatalf("a non-critical direct change must apply without approval: %v", err)
	}
	if direct, _ := fx.repo.ListActiveDirectPermissions(ctx, fx.owner.MembershipID); len(direct) != 1 {
		t.Fatalf("direct grant must be restored, got %+v", direct)
	}
}

// --- direct grants: only what the tenant admin API may grant is ever touched ---------------

func activeDirectCodes(t *testing.T, fx *rollbackFixture) map[string]bool {
	t.Helper()
	direct, err := fx.repo.ListActiveDirectPermissions(context.Background(), fx.owner.MembershipID)
	if err != nil {
		t.Fatalf("list direct permissions: %v", err)
	}
	out := map[string]bool{}
	for _, d := range direct {
		out[d.PermissionCode] = true
	}
	return out
}

func TestRBACRollback_KeepsOutOfScopeDirectGrants(t *testing.T) {
	fx := newRollbackFixture(t)
	ctx := context.Background()
	for _, code := range []string{rbPermPlatformView, rbDirectNotGrantable, rbDirectGrantable} {
		if err := fx.repo.InsertDirectPermission(ctx, fx.owner.MembershipID, "c_001", code, "u_x"); err != nil {
			t.Fatalf("seed direct %s: %v", code, err)
		}
	}
	fx.snapshot()
	// After the snapshot: the grantable grant is revoked, the non-grantable one is revoked too.
	for _, code := range []string{rbDirectGrantable, rbDirectNotGrantable} {
		if err := fx.repo.RevokeDirectPermission(ctx, fx.owner.MembershipID, code, "u_x"); err != nil {
			t.Fatalf("revoke %s: %v", code, err)
		}
	}

	if _, err := fx.rollback(1); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	got := activeDirectCodes(t, fx)
	if !got[rbPermPlatformView] {
		t.Errorf("a platform.cms.view direct grant must survive a rollback, now %v", got)
	}
	if !got[rbDirectGrantable] {
		t.Errorf("the grantable direct grant must be restored, now %v", got)
	}
	if got[rbDirectNotGrantable] {
		t.Errorf("a rollback must not re-grant a permission outside GrantablePermissions, now %v", got)
	}
}

func TestRBACRollback_DoesNotRevokeUnlistedDirectGrants(t *testing.T) {
	fx := newRollbackFixture(t)
	ctx := context.Background()
	fx.snapshot() // no direct grants in version 1
	for _, code := range []string{rbPermPlatformView, rbDirectNotGrantable} {
		if err := fx.repo.InsertDirectPermission(ctx, fx.owner.MembershipID, "c_001", code, "u_x"); err != nil {
			t.Fatalf("seed direct %s: %v", code, err)
		}
	}
	if _, err := fx.rollback(1); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	got := activeDirectCodes(t, fx)
	if !got[rbPermPlatformView] || !got[rbDirectNotGrantable] {
		t.Errorf("direct grants outside the tenant-grantable list must not be revoked, now %v", got)
	}
}

// --- adds follow the same grant policy as AssignRolePermission --------------------------------

func TestRBACRollback_DoesNotGrantNonGrantablePermissionToCustomRole(t *testing.T) {
	fx := newRollbackFixture(t)
	ctx := context.Background()
	legacy, err := json.Marshal(configversion.RBACMatrixSnapshot{
		SchemaVersion: configversion.RBACMatrixSnapshotSchema,
		RolePermissions: []configversion.RolePermissionEntry{
			{RoleID: rbRoleCustom, PermissionID: rbPermDisclosure},
			{RoleID: rbRoleCustom, PermissionID: rbPermDashboardView}, // grantable: restored
			{RoleID: rbRoleCustom, PermissionID: rbPermCritical},      // tenant_admin_only: never on a custom role
		},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if _, err := fx.repo.InsertRBACMatrixSnapshot(ctx, caapp.InsertRBACMatrixSnapshotInput{
		ID: "legacy-v1", CompanyID: "c_001", SnapshotJSON: legacy, CreatedBy: fx.owner.MembershipID, Source: configversion.SourceMutationAPI,
	}); err != nil {
		t.Fatalf("insert legacy snapshot: %v", err)
	}

	if _, err := fx.rollback(1); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	got := fx.perms(rbRoleCustom)
	requireHas(t, "custom role", got, rbPermDashboardView)
	requireLacks(t, "custom role", got, rbPermCritical)
}

// --- the endpoint needs rbac.manage as soon as it changes something ---------------------------

func TestRBACRollback_RequiresRbacManageWhenItChangesSomething(t *testing.T) {
	fx := newRollbackFixture(t)
	fx.snapshot()
	fx.add(rbRoleCustom, rbPermDashboardView)

	settingsOnly := newApprovalSvc(t, fx.repo, "system.settings")
	_, err := settingsOnly.RollbackRBACMatrixVersion(context.Background(), caapp.RollbackRBACMatrixVersionRequest{
		Subject: fx.owner, VersionNo: 1, Reason: "test",
	})
	requireHTTPCode(t, err, 403, perr.CodePermissionDenied)
	requireHas(t, "custom role (must be untouched)", fx.perms(rbRoleCustom), rbPermDashboardView)
}
