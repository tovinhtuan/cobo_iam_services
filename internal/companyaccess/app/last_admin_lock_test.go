package app_test

import (
	"context"
	"sync"
	"testing"
	"time"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// ROLE-23 (BES-22): the "company keeps an admin" check ran before the write without a lock, and
// deactivate / delete / revoke admin had no check at all. In a company without a primary admin,
// two admins could remove each other at the same time (or the only admin could remove itself),
// leaving nobody able to administer the company.

func newNoPrimaryFixture(t *testing.T) *cainmem.AdminRepository {
	t.Helper()
	repo := cainmem.NewAdminRepository()
	_ = repo.AddRolePermission(context.Background(), "company_admin", "rbac.manage")
	for _, m := range []string{"m-a", "m-b"} {
		seedMem(repo, m, "u"+m[1:], "c-1")
		if err := repo.AddRole(context.Background(), m, "company_admin"); err != nil {
			t.Fatal(err)
		}
	}
	seedMem(repo, "m-staff", "u-staff", "c-1") // not admin-capable
	return repo
}

func memberSubject(m string) caapp.AdminSubject {
	return caapp.AdminSubject{UserID: "u" + m[1:], MembershipID: m, CompanyID: "c-1"}
}

func activeAdmins(t *testing.T, repo *cainmem.AdminRepository) int {
	t.Helper()
	n := 0
	for _, id := range []string{"m-a", "m-b"} {
		m, err := repo.GetMembershipByID(context.Background(), id)
		if err != nil || m.Status != "active" {
			continue
		}
		roles, _ := repo.ListMembershipRoles(context.Background(), id)
		for _, r := range roles {
			if r.RoleID == "company_admin" {
				n++
				break
			}
		}
	}
	return n
}

type leaveFunc func(svc caapp.AdminService, actor, target string) error

var leaveChanges = map[string]leaveFunc{
	"deactivate": func(svc caapp.AdminService, actor, target string) error {
		_, err := svc.UpdateMembership(context.Background(), caapp.UpdateMembershipRequest{Subject: memberSubject(actor), MembershipID: target, Status: "inactive"})
		return err
	},
	"delete": func(svc caapp.AdminService, actor, target string) error {
		return svc.DeleteMembership(context.Background(), caapp.DeleteMembershipRequest{Subject: memberSubject(actor), MembershipID: target})
	},
	"revoke admin": func(svc caapp.AdminService, actor, target string) error {
		return svc.RevokeCompanyAdmin(context.Background(), caapp.RevokeCompanyAdminRequest{Subject: memberSubject(actor), MembershipID: target})
	},
	"remove role": func(svc caapp.AdminService, actor, target string) error {
		return svc.RemoveRole(context.Background(), caapp.RemoveRoleRequest{Subject: memberSubject(actor), MembershipID: target, RoleID: "company_admin"})
	},
}

func TestLastAdmin_SoleAdminCannotLeave(t *testing.T) {
	for name, leave := range leaveChanges {
		t.Run(name, func(t *testing.T) {
			repo := newNoPrimaryFixture(t)
			if err := repo.RemoveRole(context.Background(), "m-b", "company_admin"); err != nil {
				t.Fatal(err)
			}
			err := leave(allowedSvc(repo), "m-a", "m-a")
			requireHTTPCode(t, err, 409, perr.CodeLastAdminRoleChangeBlocked)
			if got := activeAdmins(t, repo); got != 1 {
				t.Fatalf("active admins = %d, want 1", got)
			}
		})
	}
}

func TestLastAdmin_AnotherAdminLeftAllowsTheChange(t *testing.T) {
	for name, leave := range leaveChanges {
		t.Run(name, func(t *testing.T) {
			repo := newNoPrimaryFixture(t)
			if err := leave(allowedSvc(repo), "m-a", "m-b"); err != nil {
				t.Fatalf("m-a removing m-b: %v", err)
			}
			if got := activeAdmins(t, repo); got != 1 {
				t.Fatalf("active admins = %d, want 1", got)
			}
		})
	}
}

// writeBarrierRepo pairs up the two changes at their writes, so both have run every check before
// either writes. A write with no partner waits only briefly: while one change holds the lock,
// the other cannot get past its checks until the first has written.
type writeBarrierRepo struct {
	*cainmem.AdminRepository
	mu      *sync.Mutex
	waiting *chan struct{}
}

func (r writeBarrierRepo) arrive() {
	r.mu.Lock()
	if ch := *r.waiting; ch != nil {
		close(ch)
		*r.waiting = nil
		r.mu.Unlock()
		return
	}
	ch := make(chan struct{})
	*r.waiting = ch
	r.mu.Unlock()
	select {
	case <-ch:
	case <-time.After(300 * time.Millisecond):
		r.mu.Lock()
		if *r.waiting == ch {
			*r.waiting = nil
		}
		r.mu.Unlock()
	}
}

func (r writeBarrierRepo) RemoveRole(ctx context.Context, membershipID, roleID string) error {
	r.arrive()
	return r.AdminRepository.RemoveRole(ctx, membershipID, roleID)
}

func (r writeBarrierRepo) UpdateMembershipStatus(ctx context.Context, companyID, membershipID, status string) (*caapp.MembershipView, error) {
	r.arrive()
	return r.AdminRepository.UpdateMembershipStatus(ctx, companyID, membershipID, status)
}

func (r writeBarrierRepo) DeleteMembership(ctx context.Context, companyID, membershipID string) error {
	r.arrive()
	return r.AdminRepository.DeleteMembership(ctx, companyID, membershipID)
}

func TestLastAdmin_ConcurrentMutualRemovalKeepsOneAdmin(t *testing.T) {
	for name, leave := range leaveChanges {
		t.Run(name, func(t *testing.T) {
			repo := newNoPrimaryFixture(t)
			svc := allowedSvc(writeBarrierRepo{AdminRepository: repo, mu: &sync.Mutex{}, waiting: new(chan struct{})})
			errs := make([]error, 2)
			var wg sync.WaitGroup
			for i, pair := range [][2]string{{"m-a", "m-b"}, {"m-b", "m-a"}} {
				wg.Add(1)
				go func(i int, actor, target string) {
					defer wg.Done()
					errs[i] = leave(svc, actor, target)
				}(i, pair[0], pair[1])
			}
			wg.Wait()
			if got := activeAdmins(t, repo); got != 1 {
				t.Fatalf("active admins = %d, want 1 (errors: %v)", got, errs)
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
		})
	}
}
