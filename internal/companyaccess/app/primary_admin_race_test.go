package app_test

import (
	"context"
	"sync"
	"testing"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// ROLE-25: the "is this the primary admin?" guards on deactivate / delete / revoke admin / remove
// role ran before the write without a lock. An ownership transfer landing in between made the
// target primary, and the write then deactivated, deleted or demoted the new owner. The
// invariant now holds in the write itself.

// transferAfterReadRepo transfers ownership to the target right after the service has read it,
// i.e. between the service's check and its write.
type transferAfterReadRepo struct {
	*cainmem.AdminRepository
	target string
	once   *sync.Once
}

func (r transferAfterReadRepo) GetMembershipByID(ctx context.Context, id string) (*caapp.MembershipView, error) {
	v, err := r.AdminRepository.GetMembershipByID(ctx, id)
	if id == r.target {
		r.once.Do(func() { _ = r.AdminRepository.TransferPrimaryAdmin(ctx, "c-1", "m-primary", r.target) })
	}
	return v, err
}

func newRaceFixture(t *testing.T) (*cainmem.AdminRepository, caapp.AdminService) {
	t.Helper()
	repo := newOwnershipFixture(t) // m-primary owner; m-a, m-b admins (company_admin); m-staff
	svc := allowedSvc(transferAfterReadRepo{AdminRepository: repo, target: "m-a", once: &sync.Once{}})
	return repo, svc
}

func peerSubject() caapp.AdminSubject {
	return caapp.AdminSubject{UserID: "u-b", MembershipID: "m-b", CompanyID: "c-1"}
}

func requireOwnerIntact(t *testing.T, repo *cainmem.AdminRepository) {
	t.Helper()
	m, err := repo.GetMembershipByID(context.Background(), "m-a")
	if err != nil {
		t.Fatalf("new owner was removed: %v", err)
	}
	if !m.IsPrimaryAdmin || m.Status != "active" {
		t.Fatalf("new owner damaged: primary=%v status=%q", m.IsPrimaryAdmin, m.Status)
	}
	roles, _ := repo.ListMembershipRoles(context.Background(), "m-a")
	if len(roles) == 0 {
		t.Fatal("new owner lost its admin role")
	}
}

func TestPrimaryAdminRace_DeactivateRefused(t *testing.T) {
	repo, svc := newRaceFixture(t)
	_, err := svc.UpdateMembership(context.Background(), caapp.UpdateMembershipRequest{Subject: peerSubject(), MembershipID: "m-a", Status: "inactive"})
	requireHTTPCode(t, err, 409, perr.CodeStateConflict)
	requireOwnerIntact(t, repo)
}

func TestPrimaryAdminRace_DeleteRefused(t *testing.T) {
	repo, svc := newRaceFixture(t)
	err := svc.DeleteMembership(context.Background(), caapp.DeleteMembershipRequest{Subject: peerSubject(), MembershipID: "m-a"})
	requireHTTPCode(t, err, 409, perr.CodeStateConflict)
	requireOwnerIntact(t, repo)
}

func TestPrimaryAdminRace_RevokeCompanyAdminRefused(t *testing.T) {
	repo, svc := newRaceFixture(t)
	err := svc.RevokeCompanyAdmin(context.Background(), caapp.RevokeCompanyAdminRequest{Subject: peerSubject(), MembershipID: "m-a"})
	requireHTTPCode(t, err, 409, perr.CodeStateConflict)
	requireOwnerIntact(t, repo)
}

func TestPrimaryAdminRace_RemoveAdminRoleRefused(t *testing.T) {
	repo, svc := newRaceFixture(t)
	err := svc.RemoveRole(context.Background(), caapp.RemoveRoleRequest{Subject: peerSubject(), MembershipID: "m-a", RoleID: "company_admin"})
	requireHTTPCode(t, err, 409, perr.CodeStateConflict)
	requireOwnerIntact(t, repo)
}
