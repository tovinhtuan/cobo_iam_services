package app_test

import (
	"context"
	"testing"
)

// BES-10: a rollback must not give direct grants back to a membership that has been
// deactivated since the snapshot.
func TestRBACRollback_DoesNotRegrantDirectPermissionToInactiveMembership(t *testing.T) {
	fx := newRollbackFixture(t)
	ctx := context.Background()
	member := fx.approver
	if err := fx.repo.InsertDirectPermission(ctx, member.MembershipID, "c_001", rbDirectGrantable, "u_x"); err != nil {
		t.Fatalf("seed direct permission: %v", err)
	}
	fx.snapshot()
	if err := fx.repo.RevokeDirectPermission(ctx, member.MembershipID, rbDirectGrantable, "u_x"); err != nil {
		t.Fatalf("revoke direct permission: %v", err)
	}
	if _, err := fx.repo.UpdateMembershipStatus(ctx, "c_001", member.MembershipID, "inactive"); err != nil {
		t.Fatalf("deactivate membership: %v", err)
	}

	if _, err := fx.rollback(1); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	direct, err := fx.repo.ListActiveDirectPermissions(ctx, member.MembershipID)
	if err != nil {
		t.Fatal(err)
	}
	if len(direct) != 0 {
		t.Fatalf("an inactive membership got its direct grant back: %+v", direct)
	}
}
