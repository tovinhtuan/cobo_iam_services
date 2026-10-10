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

	auditinmem "github.com/cobo/cobo_iam_services/internal/audit/infra/inmemory"
	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	authprojection "github.com/cobo/cobo_iam_services/internal/authorization/infra/projection"
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
func (fx *rollbackFixture) snapshot() int {
	fx.t.Helper()
	if err := fx.svc.AssignRolePermission(context.Background(), caapp.AssignRolePermissionRequest{
		Subject: fx.owner, RoleID: rbRoleCustom, PermissionID: rbPermComment,
	}); err != nil {
		fx.t.Fatalf("seed snapshot mutation: %v", err)
	}
	n, err := fx.repo.GetMaxRBACMatrixVersionNo(context.Background(), "c_001")
	if err != nil {
		fx.t.Fatal(err)
	}
	return n
}

func (fx *rollbackFixture) grantDirect(sub caapp.AdminSubject, code string) {
	fx.t.Helper()
	if err := fx.repo.InsertDirectPermission(context.Background(), sub.MembershipID, sub.CompanyID, code, "u_x"); err != nil {
		fx.t.Fatalf("grant direct %s: %v", code, err)
	}
}

// makeOwnerPrimaryAdmin: granting or revoking admin.membership.invite is reserved to the primary
// admin (ROLE-18), so tests about critical direct grants run as the primary admin.
func (fx *rollbackFixture) makeOwnerPrimaryAdmin() {
	fx.t.Helper()
	if err := fx.repo.SetMembershipPrimaryAdmin(context.Background(), fx.owner.MembershipID); err != nil {
		fx.t.Fatal(err)
	}
}

func (fx *rollbackFixture) revokeDirectCode(sub caapp.AdminSubject, code string) {
	fx.t.Helper()
	if err := fx.repo.RevokeDirectPermission(context.Background(), sub.MembershipID, code, "u_x"); err != nil {
		fx.t.Fatalf("revoke direct %s: %v", code, err)
	}
}

func (fx *rollbackFixture) rollbackVersion(version int) error {
	_, err := fx.rollback(version)
	return err
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
	fx.makeOwnerPrimaryAdmin()
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

// --- B: the critical check is repeated inside the restore ---------------------------------------

// racyRepo changes the state right before the restore runs, like a concurrent request would.
type racyRepo struct {
	*cainmem.AdminRepository
	beforeRestore func()
}

func (r *racyRepo) RestoreRBACMatrixFromSnapshot(ctx context.Context, companyID, actorUserID string, raw []byte, opts caapp.RBACRestoreOptions) error {
	if f := r.beforeRestore; f != nil {
		r.beforeRestore = nil
		f()
	}
	return r.AdminRepository.RestoreRBACMatrixFromSnapshot(ctx, companyID, actorUserID, raw, opts)
}

func TestRBACRollback_CriticalAppearingBeforeRestore_RoutesToApproval(t *testing.T) {
	fx := newRollbackFixture(t)
	fx.makeOwnerPrimaryAdmin()
	racy := &racyRepo{AdminRepository: fx.repo}
	fx.svc = caapp.NewAdminService(racy,
		fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"rbac.manage", "system.settings"}}, &seqIDGen{},
		caapp.WithAuditRepository(auditinmem.NewRepository()), caapp.WithEffectiveAccessCache(authprojection.NewInMemoryStore(0)))
	ctx := context.Background()
	fx.snapshot()
	fx.add(rbRoleCustom, rbPermDashboardView) // non-critical drift: the pre-check sees nothing critical
	racy.beforeRestore = func() {
		_ = fx.repo.InsertDirectPermission(ctx, fx.owner.MembershipID, "c_001", "admin.membership.invite", "u_x")
	}

	_, err := fx.rollback(1)
	requireHTTPCode(t, err, 202, perr.CodeApprovalRouted)
	requireHas(t, "custom role (nothing may be applied)", fx.perms(rbRoleCustom), rbPermDashboardView)
	if got := activeDirectCodes(t, fx); !got["admin.membership.invite"] {
		t.Errorf("the concurrently granted critical permission must not be revoked without approval, now %v", got)
	}
	if items := fx.pending(); len(items) != 1 || items[0].ChangeType != rbChangeTypeMatrixRollback {
		t.Fatalf("expected one pending %s, got %+v", rbChangeTypeMatrixRollback, items)
	}
}

// --- B: the version stored after an approval is the real state --------------------------------

func latestRBACSnapshot(t *testing.T, fx *rollbackFixture) (caapp.ConfigVersionRow, configversion.RBACMatrixSnapshot) {
	t.Helper()
	ctx := context.Background()
	list, err := fx.svc.ListRBACMatrixVersions(ctx, caapp.ListRBACMatrixVersionsRequest{Subject: fx.owner, Limit: 1})
	if err != nil || len(list.Items) == 0 {
		t.Fatalf("list versions: %v %+v", err, list)
	}
	detail, err := fx.svc.GetRBACMatrixVersion(ctx, caapp.GetRBACMatrixVersionRequest{Subject: fx.owner, VersionNo: list.Items[0].VersionNo})
	if err != nil {
		t.Fatalf("get version: %v", err)
	}
	var snap configversion.RBACMatrixSnapshot
	if err := json.Unmarshal(detail.SnapshotJSON, &snap); err != nil {
		t.Fatalf("snapshot json: %v", err)
	}
	return list.Items[0], snap
}

func TestRBACApprovalApply_StoresPostApplyState(t *testing.T) {
	fx := newRollbackFixture(t)
	fx.add(rbRoleCustom, rbPermCritical)
	fx.snapshot()
	err := fx.svc.RemoveRolePermission(context.Background(), caapp.RemoveRolePermissionRequest{
		Subject: fx.owner, RoleID: rbRoleCustom, PermissionID: rbPermCritical,
	})
	requireHTTPCode(t, err, 202, perr.CodeApprovalRouted)
	items := fx.pending()
	if len(items) != 1 {
		t.Fatalf("expected 1 pending approval, got %d", len(items))
	}
	fx.add(rbRoleGlobalOwner, rbPermDeadlineView) // real state moves on while the approval is pending

	if err := fx.approve(items[0].ApprovalID); err != nil {
		t.Fatalf("approve: %v", err)
	}
	row, snap := latestRBACSnapshot(t, fx)
	if row.Source != configversion.SourceApprovalApply {
		t.Fatalf("latest version source = %q, want %q", row.Source, configversion.SourceApprovalApply)
	}
	found, hasCritical := false, false
	for _, e := range snap.RolePermissions {
		if e.RoleID == rbRoleGlobalOwner && e.PermissionID == rbPermDeadlineView {
			found = true
		}
		if e.RoleID == rbRoleCustom && e.PermissionID == rbPermCritical {
			hasCritical = true
		}
	}
	if !found {
		t.Errorf("the stored version must describe the real state (global role holds deadline.view), got %+v", snap.RolePermissions)
	}
	if hasCritical {
		t.Errorf("the stored version must not list the permission the approval removed")
	}
}

// --- B: creating, cloning or inactivating a custom role is a new matrix version ---------------

func TestRBACApproval_GoesStaleWhenCustomRoleLifecycleChanges(t *testing.T) {
	const spare = "spare_a"
	ops := map[string]func(fx *rollbackFixture) error{
		"create": func(fx *rollbackFixture) error {
			_, err := fx.svc.CreateCustomRole(context.Background(), caapp.CreateCustomRoleRequest{Subject: fx.owner, RoleName: "stale probe"})
			return err
		},
		"clone": func(fx *rollbackFixture) error {
			_, err := fx.svc.CloneRole(context.Background(), caapp.CloneRoleRequest{Subject: fx.owner, SourceRoleID: rbRoleCustom, RoleName: "stale clone"})
			return err
		},
		"inactivate": func(fx *rollbackFixture) error {
			return fx.svc.InactivateCustomRole(context.Background(), caapp.InactivateCustomRoleRequest{Subject: fx.owner, RoleID: spare})
		},
	}
	for name, op := range ops {
		t.Run(name, func(t *testing.T) {
			fx := newRollbackFixture(t)
			fx.repo.SeedRoleForCompany(caapp.RoleListItem{
				RoleID: spare, RoleCode: spare, RoleName: spare, Status: "active", Scope: "company", RoleType: caapp.RoleTypeTenantCustom,
			}, "c_001")
			fx.add(rbRoleCustom, rbPermCritical)
			fx.snapshot()
			err := fx.svc.RemoveRolePermission(context.Background(), caapp.RemoveRolePermissionRequest{
				Subject: fx.owner, RoleID: rbRoleCustom, PermissionID: rbPermCritical,
			})
			requireHTTPCode(t, err, 202, perr.CodeApprovalRouted)
			items := fx.pending()
			if len(items) != 1 {
				t.Fatalf("expected 1 pending approval, got %d", len(items))
			}
			if err := op(fx); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			requireHTTPCode(t, fx.approve(items[0].ApprovalID), 409, perr.CodeStaleProposal)
		})
	}
}

// --- D: approvals that would change nothing ------------------------------------------------------

// legacyApproval queues an approval the way an older binary did: a proposal that only differs
// from the live state on a role the restore may no longer change.
func legacyApproval(t *testing.T, fx *rollbackFixture, dropRole, dropPerm string) string {
	t.Helper()
	ctx := context.Background()
	raw, err := fx.repo.BuildRBACMatrixSnapshotJSON(ctx, "c_001")
	if err != nil {
		t.Fatal(err)
	}
	var snap configversion.RBACMatrixSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		t.Fatal(err)
	}
	kept := snap.RolePermissions[:0]
	for _, e := range snap.RolePermissions {
		if e.RoleID == dropRole && e.PermissionID == dropPerm {
			continue
		}
		kept = append(kept, e)
	}
	snap.RolePermissions = kept
	proposed, _ := json.Marshal(snap)
	ver, err := fx.repo.GetMaxRBACMatrixVersionNo(ctx, "c_001")
	if err != nil {
		t.Fatal(err)
	}
	row, err := fx.repo.InsertPendingAdminChange(ctx, caapp.InsertPendingAdminChangeInput{
		ID: "legacy-approval", CompanyID: "c_001", ApprovalSubjectType: configversion.ApprovalSubjectConfigSnapshot,
		AggregateType: configversion.AggregateRBACMatrix, ChangeType: configversion.ChangeTypeRBACPermissionRemove,
		ProposedSnapshotJSON: proposed, BaseLiveVersionNo: &ver, RequestedBy: fx.owner.MembershipID,
	})
	if err != nil {
		t.Fatal(err)
	}
	return row.ID
}

func TestApproveConfigApproval_NothingToApply_Conflicts(t *testing.T) {
	fx := newRollbackFixture(t)
	fx.snapshot()
	// Queued by an older binary against a protected role: the restore can no longer change it.
	id := legacyApproval(t, fx, rbRoleCMSOperator, rbPermDisclosure)

	requireHTTPCode(t, fx.approve(id), 409, perr.CodeApprovalNothingToApply)
	got, err := fx.repo.GetPendingAdminChange(context.Background(), "c_001", id)
	if err != nil || got.Status != configversion.ApprovalStatusPending {
		t.Fatalf("a refused approval must stay pending, got %+v err=%v", got, err)
	}
	requireHas(t, "cms_operator", fx.perms(rbRoleCMSOperator), rbPermDisclosure)
	// It can still be rejected.
	if _, err := fx.svc.RejectConfigApproval(context.Background(), caapp.RejectConfigApprovalRequest{Subject: fx.approver, ApprovalID: id, RejectReason: "obsolete"}); err != nil {
		t.Fatalf("reject: %v", err)
	}
}

func TestApproveConfigApproval_AlreadyApplied_Conflicts(t *testing.T) {
	fx := newRollbackFixture(t)
	fx.add(rbRoleCustom, rbPermCritical)
	fx.snapshot()
	err := fx.svc.RemoveRolePermission(context.Background(), caapp.RemoveRolePermissionRequest{
		Subject: fx.owner, RoleID: rbRoleCustom, PermissionID: rbPermCritical,
	})
	requireHTTPCode(t, err, 202, perr.CodeApprovalRouted)
	items := fx.pending()
	fx.remove(rbRoleCustom, rbPermCritical) // someone already removed it outside the queue

	requireHTTPCode(t, fx.approve(items[0].ApprovalID), 409, perr.CodeApprovalNothingToApply)
}

// --- D: compare shows what approving would really change -------------------------------------------

func TestCompareConfigApproval_RBACMatrixListsRealChanges(t *testing.T) {
	fx := newRollbackFixture(t)
	fx.add(rbRoleCustom, rbPermCritical)
	fx.snapshot()
	err := fx.svc.RemoveRolePermission(context.Background(), caapp.RemoveRolePermissionRequest{
		Subject: fx.owner, RoleID: rbRoleCustom, PermissionID: rbPermCritical,
	})
	requireHTTPCode(t, err, 202, perr.CodeApprovalRouted)
	items := fx.pending()
	fx.add(rbRoleGlobalOwner, rbPermDeadlineView) // out of the restore's scope: must not be listed

	view, err := fx.svc.CompareConfigApproval(context.Background(), caapp.CompareConfigApprovalRequest{Subject: fx.owner, ApprovalID: items[0].ApprovalID})
	if err != nil {
		t.Fatalf("compare: %v", err)
	}
	if len(view.Changes) != 1 {
		t.Fatalf("expected exactly the one real change, got %+v", view.Changes)
	}
	c := view.Changes[0]
	if c.Kind != "role_permission" || c.Action != "remove" || c.RoleID != rbRoleCustom || c.PermissionCode != rbPermCritical || !c.Critical {
		t.Errorf("unexpected change %+v", c)
	}
}

// --- D: an approval to remove a direct grant applies exactly that removal ---------------------------

func TestApprovedDirectGrantRemoval_AppliesForNonGrantableHighRiskCode(t *testing.T) {
	fx := newRollbackFixture(t)
	ctx := context.Background()
	const code = "dept.manage" // not in GrantablePermissions, high risk: removal needs approval
	if err := fx.repo.InsertDirectPermission(ctx, fx.owner.MembershipID, "c_001", code, "u_x"); err != nil {
		t.Fatal(err)
	}
	if err := fx.repo.InsertDirectPermission(ctx, fx.owner.MembershipID, "c_001", rbDirectNotGrantable, "u_x"); err != nil {
		t.Fatal(err)
	}
	fx.snapshot()

	err := fx.svc.RemoveDirectPermission(ctx, caapp.RemoveDirectPermissionRequest{
		Subject: fx.owner, MembershipID: fx.owner.MembershipID, PermissionCode: code,
	})
	requireHTTPCode(t, err, 202, perr.CodeApprovalRouted)
	items := fx.pending()
	if len(items) != 1 {
		t.Fatalf("expected 1 pending approval, got %d", len(items))
	}

	if err := fx.approve(items[0].ApprovalID); err != nil {
		t.Fatalf("approve: %v", err)
	}
	got := activeDirectCodes(t, fx)
	if got[code] {
		t.Errorf("the approved removal must revoke %s, now %v", code, got)
	}
	if !got[rbDirectNotGrantable] {
		t.Errorf("only the requested grant may be revoked, now %v", got)
	}
}

// --- ROLE-18: only the primary admin may grant or revoke admin.membership.invite -------------------
//
// The direct-permission routes enforce it; the approval queue and rollback must not be a way around it.

func TestSubmitConfigApproval_DirectRemoveOfInvite_NeedsPrimaryAdmin(t *testing.T) {
	fx := newRollbackFixture(t)
	ctx := context.Background()
	fx.grantDirect(fx.owner, "admin.membership.invite")
	submit := func() error {
		_, err := fx.svc.SubmitConfigApproval(ctx, caapp.SubmitConfigApprovalRequest{
			Subject: fx.owner, AggregateType: configversion.AggregateRBACMatrix, ChangeType: configversion.ChangeTypeRBACDirectPermRemove,
			Proposed: map[string]any{"membership_id": fx.owner.MembershipID, "permission_code": "admin.membership.invite"},
		})
		return err
	}
	requireHTTPCode(t, submit(), 403, perr.CodePermissionDenied)
	if n := len(fx.pending()); n != 0 {
		t.Fatalf("a refused submission must not be queued, got %d", n)
	}
	if err := fx.repo.SetMembershipPrimaryAdmin(ctx, fx.owner.MembershipID); err != nil {
		t.Fatal(err)
	}
	if err := submit(); err != nil {
		t.Fatalf("the primary admin may queue it: %v", err)
	}
}

func TestRBACRollback_InvitePermissionChange_NeedsPrimaryAdmin(t *testing.T) {
	fx := newRollbackFixture(t)
	ctx := context.Background()
	fx.grantDirect(fx.owner, "admin.membership.invite")
	v := fx.snapshot()
	fx.revokeDirectCode(fx.owner, "admin.membership.invite")

	requireHTTPCode(t, fx.rollbackVersion(v), 403, perr.CodePermissionDenied)
	if n := len(fx.pending()); n != 0 {
		t.Fatalf("a refused rollback must not be queued, got %d", n)
	}
	if err := fx.repo.SetMembershipPrimaryAdmin(ctx, fx.owner.MembershipID); err != nil {
		t.Fatal(err)
	}
	requireHTTPCode(t, fx.rollbackVersion(v), 202, perr.CodeApprovalRouted)
}

// --- ROLE-19: an approval applies what the approver was shown ---------------------------------------

func TestRBACApproval_SingleChange_DoesNotRevokeLaterDirectGrants(t *testing.T) {
	fx := newRollbackFixture(t)
	fx.add(rbRoleCustom, rbPermCritical)
	fx.snapshot()
	err := fx.svc.RemoveRolePermission(context.Background(), caapp.RemoveRolePermissionRequest{
		Subject: fx.owner, RoleID: rbRoleCustom, PermissionID: rbPermCritical,
	})
	requireHTTPCode(t, err, 202, perr.CodeApprovalRouted)
	items := fx.pending()
	// A member is invited while the approval waits: direct grants are written without a new matrix version.
	fx.grantDirect(fx.owner, rbDirectGrantable)

	if err := fx.approve(items[0].ApprovalID); err != nil {
		t.Fatalf("approve: %v", err)
	}
	requireLacks(t, "custom role", fx.perms(rbRoleCustom), rbPermCritical)
	if got := activeDirectCodes(t, fx); !got[rbDirectGrantable] {
		t.Errorf("approving a role-permission removal must not revoke a direct grant made later, now %v", got)
	}
}

func TestRBACApproval_Rollback_IsBoundToTheReviewedPlan(t *testing.T) {
	fx := newRollbackFixture(t)
	fx.makeOwnerPrimaryAdmin()
	ctx := context.Background()
	fx.grantDirect(fx.owner, "admin.membership.invite")
	v := fx.snapshot()
	fx.revokeDirectCode(fx.owner, "admin.membership.invite")
	requireHTTPCode(t, fx.rollbackVersion(v), 202, perr.CodeApprovalRouted)
	items := fx.pending()
	// While it waits, another direct grant appears (no matrix version): the rollback would now
	// also revoke it, which nobody reviewed.
	fx.grantDirect(fx.owner, rbDirectGrantable)

	requireHTTPCode(t, fx.approve(items[0].ApprovalID), 409, perr.CodeStaleProposal)
	if got := activeDirectCodes(t, fx); !got[rbDirectGrantable] || got["admin.membership.invite"] {
		t.Errorf("a stale approval must change nothing, now %v", got)
	}
	_ = ctx
}

// --- BES-03: rolling back needs rbac.manage, whatever the pre-check saw ------------------------------

func TestRBACRollback_AlwaysRequiresRbacManage(t *testing.T) {
	fx := newRollbackFixture(t)
	v := fx.snapshot() // the state equals the version: nothing to change
	settingsOnly := newApprovalSvc(t, fx.repo, "system.settings")
	_, err := settingsOnly.RollbackRBACMatrixVersion(context.Background(), caapp.RollbackRBACMatrixVersionRequest{
		Subject: fx.owner, VersionNo: v, Reason: "test",
	})
	requireHTTPCode(t, err, 403, perr.CodePermissionDenied)
}

// --- BES-04: a rollback never honours the instructions only an approval carries -------------------------

func TestRBACRollback_IgnoresApprovalOnlyFieldsOfAStoredVersion(t *testing.T) {
	fx := newRollbackFixture(t)
	ctx := context.Background()
	fx.grantDirect(fx.owner, rbPermPlatformView) // outside the tenant-grantable list
	stored, err := json.Marshal(configversion.RBACMatrixSnapshot{
		SchemaVersion:     configversion.RBACMatrixSnapshotSchema,
		RolePermissions:   []configversion.RolePermissionEntry{{RoleID: rbRoleCustom, PermissionID: rbPermDisclosure}},
		DirectPermissions: []configversion.DirectPermissionEntry{{MembershipID: fx.owner.MembershipID, PermissionCode: rbPermPlatformView}},
		// Fields that only an approval proposal carries; a stored version never has them.
		Explicit:      true,
		DirectRevokes: []configversion.DirectPermissionEntry{{MembershipID: fx.owner.MembershipID, PermissionCode: rbPermPlatformView}},
		RoleRevokes:   []configversion.RolePermissionEntry{{RoleID: rbRoleCustom, PermissionID: rbPermDisclosure}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fx.repo.InsertRBACMatrixSnapshot(ctx, caapp.InsertRBACMatrixSnapshotInput{
		ID: "legacy-with-instructions", CompanyID: "c_001", SnapshotJSON: stored, CreatedBy: fx.owner.MembershipID, Source: configversion.SourceMutationAPI,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := fx.rollback(1); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if got := activeDirectCodes(t, fx); !got[rbPermPlatformView] {
		t.Errorf("a stored version must not be able to revoke a direct grant through an approval-only field, now %v", got)
	}
	requireHas(t, "custom role", fx.perms(rbRoleCustom), rbPermDisclosure)
}

// --- ROLE-22: changing who administers the company drops the cached effective access ---------------------

type recordingCache struct{ invalidated []string }

func (c *recordingCache) InvalidateCompany(_ context.Context, companyID string) error {
	c.invalidated = append(c.invalidated, companyID)
	return nil
}

func TestCompanyAdminRoutes_InvalidateEffectiveAccess(t *testing.T) {
	calls := map[string]func(svc caapp.AdminService) error{
		"AssignCompanyAdmin": func(svc caapp.AdminService) error {
			return svc.AssignCompanyAdmin(context.Background(), caapp.AssignCompanyAdminRequest{
				Subject: caapp.AdminSubject{UserID: "u-primary", MembershipID: "m-primary", CompanyID: "c-1"}, MembershipID: "m-target"})
		},
		"RevokeCompanyAdmin": func(svc caapp.AdminService) error {
			return svc.RevokeCompanyAdmin(context.Background(), caapp.RevokeCompanyAdminRequest{
				Subject: caapp.AdminSubject{UserID: "u-primary", MembershipID: "m-primary", CompanyID: "c-1"}, MembershipID: "m-target"})
		},
		"TransferOwnership": func(svc caapp.AdminService) error {
			return svc.TransferOwnership(context.Background(), caapp.TransferOwnershipRequest{
				Subject: caapp.AdminSubject{UserID: "u-primary", MembershipID: "m-primary", CompanyID: "c-1"}, TargetMembershipID: "m-target"})
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			repo := cainmem.NewAdminRepository()
			seedMem(repo, "m-primary", "u-primary", "c-1")
			seedMem(repo, "m-target", "u-target", "c-1")
			if err := repo.SetMembershipPrimaryAdmin(context.Background(), "m-primary"); err != nil {
				t.Fatal(err)
			}
			cache := &recordingCache{}
			svc := caapp.NewAdminService(repo, fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"rbac.manage"}},
				fixedIDGen("test-id"), caapp.WithEffectiveAccessCache(cache))
			if err := call(svc); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if len(cache.invalidated) == 0 {
				t.Fatalf("%s must drop the cached effective access of the company", name)
			}
		})
	}
}

// --- API-05: the compare view of an RBAC approval ---------------------------------------------------

func TestCompareConfigApproval_RBACMatrix_EmptyChangesIsAnEmptyListAndHidesInternalKeys(t *testing.T) {
	fx := newRollbackFixture(t)
	fx.add(rbRoleCustom, rbPermCritical)
	fx.snapshot()
	err := fx.svc.RemoveRolePermission(context.Background(), caapp.RemoveRolePermissionRequest{
		Subject: fx.owner, RoleID: rbRoleCustom, PermissionID: rbPermCritical,
	})
	requireHTTPCode(t, err, 202, perr.CodeApprovalRouted)
	items := fx.pending()

	view, err := fx.svc.CompareConfigApproval(context.Background(), caapp.CompareConfigApprovalRequest{Subject: fx.owner, ApprovalID: items[0].ApprovalID})
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range view.Compare.ChangedKeys {
		switch k {
		case "explicit", "role_revokes", "direct_revokes", "plan_digest":
			t.Errorf("internal key %q must not be shown to the approver", k)
		}
	}

	fx.remove(rbRoleCustom, rbPermCritical) // applied outside the queue: nothing left to change
	view, err = fx.svc.CompareConfigApproval(context.Background(), caapp.CompareConfigApprovalRequest{Subject: fx.owner, ApprovalID: items[0].ApprovalID})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(view)
	var out map[string]json.RawMessage
	_ = json.Unmarshal(raw, &out)
	if string(out["changes"]) != "[]" {
		t.Errorf("an RBAC approval with nothing to apply must send changes: [], got %s", out["changes"])
	}
}

// countingRepo records how often an approval is applied.
type countingRepo struct {
	*cainmem.AdminRepository
	applied int
}

func (r *countingRepo) ApplyPendingApprovalInTx(ctx context.Context, in caapp.ApplyPendingApprovalInput, row caapp.PendingAdminChange) (*caapp.ApplyPendingApprovalResult, error) {
	r.applied++
	return r.AdminRepository.ApplyPendingApprovalInTx(ctx, in, row)
}

// A plan that changed since the request was queued is refused before the apply transaction starts
// (the transaction repeats the check under its locks).
func TestRBACApproval_Rollback_StalePlanIsRefusedBeforeTheApplyTransaction(t *testing.T) {
	fx := newRollbackFixture(t)
	counting := &countingRepo{AdminRepository: fx.repo}
	fx.svc = caapp.NewAdminService(counting,
		fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"rbac.manage", "system.settings"}}, &seqIDGen{},
		caapp.WithAuditRepository(auditinmem.NewRepository()), caapp.WithEffectiveAccessCache(authprojection.NewInMemoryStore(0)))
	fx.makeOwnerPrimaryAdmin()
	fx.grantDirect(fx.owner, "admin.membership.invite")
	v := fx.snapshot()
	fx.revokeDirectCode(fx.owner, "admin.membership.invite")
	requireHTTPCode(t, fx.rollbackVersion(v), 202, perr.CodeApprovalRouted)
	items := fx.pending()
	fx.grantDirect(fx.owner, rbDirectGrantable)

	requireHTTPCode(t, fx.approve(items[0].ApprovalID), 409, perr.CodeStaleProposal)
	if counting.applied != 0 {
		t.Fatalf("the apply transaction must not start for a stale plan, started %d time(s)", counting.applied)
	}
}
