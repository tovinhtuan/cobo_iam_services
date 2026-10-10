package app_test

import (
	"context"
	"sync"
	"testing"

	auditinmem "github.com/cobo/cobo_iam_services/internal/audit/infra/inmemory"
	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	authprojection "github.com/cobo/cobo_iam_services/internal/authorization/infra/projection"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// ROLE-23 (role level): an RBAC approval (remove a role permission, remove a direct grant, a
// rollback) could take rbac.manage from the last admin-capable members, since only membership
// changes checked for the last admin. Submitting such a change is refused, and approving it
// re-checks under the company's admin lock.

func (fx *rollbackFixture) bind(sub caapp.AdminSubject, roleID string) {
	fx.t.Helper()
	if err := fx.repo.AddRole(context.Background(), sub.MembershipID, roleID); err != nil {
		fx.t.Fatalf("bind %s to %s: %v", roleID, sub.MembershipID, err)
	}
}

// ownerAdminViaCustomRole makes the owner admin-capable only through the tenant_custom role.
func (fx *rollbackFixture) ownerAdminViaCustomRole() {
	fx.t.Helper()
	fx.add(rbRoleCustom, rbPermCritical)
	fx.bind(fx.owner, rbRoleCustom)
}

func (fx *rollbackFixture) adminCapable() []string {
	fx.t.Helper()
	ctx := context.Background()
	var out []string
	for _, sub := range []caapp.AdminSubject{fx.owner, fx.approver} {
		m, err := fx.repo.GetMembershipByID(ctx, sub.MembershipID)
		if err != nil || m.Status != "active" {
			continue
		}
		fromRole, _ := fx.repo.MembershipHasPermissionFromRole(ctx, sub.MembershipID, sub.CompanyID, rbPermCritical)
		direct, _ := fx.repo.HasActiveDirectPermission(ctx, sub.MembershipID, rbPermCritical)
		if fromRole || direct {
			out = append(out, sub.MembershipID)
		}
	}
	return out
}

func (fx *rollbackFixture) requireNothingPending() {
	fx.t.Helper()
	if p := fx.pending(); len(p) != 0 {
		fx.t.Fatalf("nothing should be queued, got %d approval(s)", len(p))
	}
}

func TestRBACApproval_RoleRevokeOfLastAdminRefusedAtSubmit(t *testing.T) {
	fx := newRollbackFixture(t)
	fx.ownerAdminViaCustomRole()
	err := fx.svc.RemoveRolePermission(context.Background(), caapp.RemoveRolePermissionRequest{
		Subject: fx.owner, RoleID: rbRoleCustom, PermissionID: rbPermCritical,
	})
	requireHTTPCode(t, err, 409, perr.CodeLastAdminRoleChangeBlocked)
	requireHas(t, "custom role", fx.perms(rbRoleCustom), rbPermCritical)
	fx.requireNothingPending()
}

func TestRBACApproval_DirectRevokeOfLastAdminRefusedAtSubmit(t *testing.T) {
	fx := newRollbackFixture(t)
	fx.grantDirect(fx.owner, rbPermCritical)
	err := fx.svc.RemoveDirectPermission(context.Background(), caapp.RemoveDirectPermissionRequest{
		Subject: fx.owner, MembershipID: fx.owner.MembershipID, PermissionCode: rbPermCritical,
	})
	requireHTTPCode(t, err, 409, perr.CodeLastAdminRoleChangeBlocked)
	if got := fx.adminCapable(); len(got) != 1 {
		t.Fatalf("admin-capable members = %v, want the owner", got)
	}
	fx.requireNothingPending()
}

func TestRBACApproval_RollbackRemovingLastAdminRefused(t *testing.T) {
	fx := newRollbackFixture(t)
	v := fx.snapshot() // the custom role without rbac.manage
	fx.ownerAdminViaCustomRole()
	err := fx.rollbackVersion(v)
	requireHTTPCode(t, err, 409, perr.CodeLastAdminRoleChangeBlocked)
	requireHas(t, "custom role", fx.perms(rbRoleCustom), rbPermCritical)
	fx.requireNothingPending()
}

func TestRBACApproval_ApproveRechecksLastAdmin(t *testing.T) {
	fx := newRollbackFixture(t)
	fx.ownerAdminViaCustomRole()
	fx.grantDirect(fx.approver, rbPermCritical) // a second admin: the request may be queued
	err := fx.svc.RemoveRolePermission(context.Background(), caapp.RemoveRolePermissionRequest{
		Subject: fx.owner, RoleID: rbRoleCustom, PermissionID: rbPermCritical,
	})
	requireHTTPCode(t, err, 202, perr.CodeApprovalRouted)
	fx.revokeDirectCode(fx.approver, rbPermCritical) // ...and is gone before the approval
	requireHTTPCode(t, fx.approve(fx.pending()[0].ApprovalID), 409, perr.CodeLastAdminRoleChangeBlocked)
	requireHas(t, "custom role", fx.perms(rbRoleCustom), rbPermCritical)
}

func TestRBACApproval_AnotherAdminLeftApplies(t *testing.T) {
	fx := newRollbackFixture(t)
	fx.ownerAdminViaCustomRole()
	fx.grantDirect(fx.approver, rbPermCritical)
	err := fx.svc.RemoveRolePermission(context.Background(), caapp.RemoveRolePermissionRequest{
		Subject: fx.owner, RoleID: rbRoleCustom, PermissionID: rbPermCritical,
	})
	requireHTTPCode(t, err, 202, perr.CodeApprovalRouted)
	if err := fx.approve(fx.pending()[0].ApprovalID); err != nil {
		t.Fatalf("approve: %v", err)
	}
	requireLacks(t, "custom role", fx.perms(rbRoleCustom), rbPermCritical)
	if got := fx.adminCapable(); len(got) != 1 || got[0] != fx.approver.MembershipID {
		t.Fatalf("admin-capable members = %v, want only the approver", got)
	}
}

func (r writeBarrierRepo) ApplyPendingApprovalInTx(ctx context.Context, in caapp.ApplyPendingApprovalInput, row caapp.PendingAdminChange) (*caapp.ApplyPendingApprovalResult, error) {
	r.arrive()
	return r.AdminRepository.ApplyPendingApprovalInTx(ctx, in, row)
}

// The approval (owner loses rbac.manage with the custom role) and a deactivation of the other
// admin each leave one admin on their own; run together without a lock they leave none.
func TestRBACApproval_ConcurrentWithDeactivateKeepsOneAdmin(t *testing.T) {
	fx := newRollbackFixture(t)
	fx.ownerAdminViaCustomRole()
	fx.grantDirect(fx.approver, rbPermCritical)
	err := fx.svc.RemoveRolePermission(context.Background(), caapp.RemoveRolePermissionRequest{
		Subject: fx.owner, RoleID: rbRoleCustom, PermissionID: rbPermCritical,
	})
	requireHTTPCode(t, err, 202, perr.CodeApprovalRouted)
	id := fx.pending()[0].ApprovalID

	repo := writeBarrierRepo{AdminRepository: fx.repo, mu: &sync.Mutex{}, waiting: new(chan struct{})}
	svc := caapp.NewAdminService(repo, fakeAuthService{
		decision:    authapp.DecisionAllow,
		permissions: []string{"rbac.manage", "system.settings"},
	}, &seqIDGen{},
		caapp.WithAuditRepository(auditinmem.NewRepository()),
		caapp.WithEffectiveAccessCache(authprojection.NewInMemoryStore(0)),
	)
	errs := make([]error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, errs[0] = svc.ApproveConfigApproval(context.Background(), caapp.ApproveConfigApprovalRequest{Subject: fx.approver, ApprovalID: id})
	}()
	go func() {
		defer wg.Done()
		_, errs[1] = svc.UpdateMembership(context.Background(), caapp.UpdateMembershipRequest{
			Subject: fx.owner, MembershipID: fx.approver.MembershipID, Status: "inactive",
		})
	}()
	wg.Wait()
	if got := fx.adminCapable(); len(got) != 1 {
		t.Fatalf("admin-capable members = %v, want one (errors: %v)", got, errs)
	}
	failed := 0
	for _, err := range errs {
		if err != nil {
			requireHTTPCode(t, err, 409, perr.CodeLastAdminRoleChangeBlocked)
			failed++
		}
	}
	if failed != 1 {
		t.Fatalf("want exactly one change refused, got errors %v", errs)
	}
}
