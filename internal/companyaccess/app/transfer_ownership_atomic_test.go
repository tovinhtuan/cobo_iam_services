package app_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// RP-10 / H16: transferring ownership must leave exactly one primary admin, whatever runs
// concurrently or fails half-way.

func newOwnershipFixture(t *testing.T) *cainmem.AdminRepository {
	t.Helper()
	repo := cainmem.NewAdminRepository()
	_ = repo.AddRolePermission(context.Background(), "company_admin", "rbac.manage")
	for _, m := range []string{"m-primary", "m-a", "m-b"} {
		seedMem(repo, m, "u"+m[1:], "c-1")
		if err := repo.AddRole(context.Background(), m, "company_admin"); err != nil {
			t.Fatal(err)
		}
	}
	seedMem(repo, "m-staff", "u-staff", "c-1") // not admin-capable
	if err := repo.SetMembershipPrimaryAdmin(context.Background(), "m-primary"); err != nil {
		t.Fatal(err)
	}
	return repo
}

func primaries(t *testing.T, repo *cainmem.AdminRepository) []string {
	t.Helper()
	var out []string
	for _, m := range []string{"m-primary", "m-a", "m-b"} {
		v, err := repo.GetMembershipByID(context.Background(), m)
		if err != nil {
			t.Fatal(err)
		}
		if v.IsPrimaryAdmin {
			out = append(out, m)
		}
	}
	return out
}

func ownerSubject() caapp.AdminSubject {
	return caapp.AdminSubject{UserID: "u-primary", MembershipID: "m-primary", CompanyID: "c-1"}
}

// barrierRepo lets every concurrent transfer read "caller is primary" before any of them writes.
type barrierRepo struct {
	*cainmem.AdminRepository
	wg *sync.WaitGroup
}

func (r barrierRepo) GetMembershipByID(ctx context.Context, id string) (*caapp.MembershipView, error) {
	v, err := r.AdminRepository.GetMembershipByID(ctx, id)
	if id == "m-primary" {
		r.wg.Done()
		r.wg.Wait()
	}
	return v, err
}

func TestTransferOwnership_ConcurrentTransfersLeaveOnePrimary(t *testing.T) {
	repo := newOwnershipFixture(t)
	var barrier sync.WaitGroup
	barrier.Add(2)
	svc := allowedSvc(barrierRepo{AdminRepository: repo, wg: &barrier})
	var done sync.WaitGroup
	errs := make([]error, 2)
	for i, target := range []string{"m-a", "m-b"} {
		done.Add(1)
		go func(i int, target string) {
			defer done.Done()
			errs[i] = svc.TransferOwnership(context.Background(), caapp.TransferOwnershipRequest{Subject: ownerSubject(), TargetMembershipID: target})
		}(i, target)
	}
	done.Wait()
	if got := primaries(t, repo); len(got) != 1 {
		t.Fatalf("primary admins after concurrent transfers = %v, want exactly one", got)
	}
	ok := 0
	for _, err := range errs {
		if err == nil {
			ok++
		} else {
			requireHTTPCode(t, err, 409, perr.CodeStateConflict)
		}
	}
	if ok != 1 {
		t.Fatalf("successful transfers = %d, want 1 (errors %v)", ok, errs)
	}
}

// failingSetRepo fails the "make target primary" write of the old two-step implementation.
type failingSetRepo struct{ *cainmem.AdminRepository }

func (r failingSetRepo) SetMembershipPrimaryAdmin(context.Context, string) error {
	return errors.New("db unavailable")
}

func TestTransferOwnership_FailedWriteKeepsOwner(t *testing.T) {
	repo := newOwnershipFixture(t)
	svc := allowedSvc(failingSetRepo{repo})
	_ = svc.TransferOwnership(context.Background(), caapp.TransferOwnershipRequest{Subject: ownerSubject(), TargetMembershipID: "m-a"})
	if got := primaries(t, repo); len(got) != 1 {
		t.Fatalf("primary admins after a failed write = %v, want exactly one", got)
	}
}

func TestTransferOwnership_ToSelfRejected(t *testing.T) {
	repo := newOwnershipFixture(t)
	err := allowedSvc(repo).TransferOwnership(context.Background(), caapp.TransferOwnershipRequest{Subject: ownerSubject(), TargetMembershipID: "m-primary"})
	requireHTTPCode(t, err, 400, perr.CodeInvalidRequest)
}

// ROLE-26: the new owner must be able to administer the company.
func TestTransferOwnership_ToNonAdminRejected(t *testing.T) {
	repo := newOwnershipFixture(t)
	err := allowedSvc(repo).TransferOwnership(context.Background(), caapp.TransferOwnershipRequest{Subject: ownerSubject(), TargetMembershipID: "m-staff"})
	requireHTTPCode(t, err, 409, perr.CodeStateConflict)
	if got := primaries(t, repo); len(got) != 1 || got[0] != "m-primary" {
		t.Fatalf("owner changed: %v", got)
	}
}

// ROLE-27 / ROLE-25: the primary admin cannot be made inactive at all, so an inactive owner (only
// possible through old data) never exists; the transfer still checks the owner's status.
func TestPrimaryAdmin_CannotBecomeInactive(t *testing.T) {
	repo := newOwnershipFixture(t)
	_, err := repo.UpdateMembershipStatus(context.Background(), "c-1", "m-primary", "inactive")
	requireHTTPCode(t, err, 409, perr.CodeStateConflict)
}

func TestTransferOwnership_ToInactiveMemberRejected(t *testing.T) {
	repo := newOwnershipFixture(t)
	if _, err := repo.UpdateMembershipStatus(context.Background(), "c-1", "m-a", "inactive"); err != nil {
		t.Fatal(err)
	}
	err := allowedSvc(repo).TransferOwnership(context.Background(), caapp.TransferOwnershipRequest{Subject: ownerSubject(), TargetMembershipID: "m-a"})
	requireHTTPCode(t, err, 409, perr.CodeStateConflict)
	if got := primaries(t, repo); len(got) != 1 || got[0] != "m-primary" {
		t.Fatalf("owner changed: %v", got)
	}
}
