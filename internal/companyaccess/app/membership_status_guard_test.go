package app_test

import (
	"context"
	"errors"
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// BES-12: since an inactive membership loses all access at once (H3), any non-active status
// is a deactivation; the primary admin must be protected against every spelling of it.
func TestUpdateMembership_PrimaryAdminCannotBeDeactivatedAnyWay(t *testing.T) {
	for _, status := range []string{"inactive", "Inactive", " inactive ", "INACTIVE"} {
		t.Run(status, func(t *testing.T) {
			repo, _, svc, _ := newInvalidationFixture(t)
			peer := caapp.AdminSubject{UserID: "u-target", MembershipID: "m-target", CompanyID: "c-1"}
			_, err := svc.UpdateMembership(context.Background(), caapp.UpdateMembershipRequest{Subject: peer, MembershipID: "m-primary", Status: status})
			requireHTTPCode(t, err, 409, perr.CodeStateConflict)
			m, _ := repo.GetMembershipByID(context.Background(), "m-primary")
			if m.Status != "active" {
				t.Fatalf("primary admin status changed to %q", m.Status)
			}
		})
	}
}

func TestUpdateMembership_RejectsUnknownStatus(t *testing.T) {
	for _, status := range []string{"suspended", "deleted", "", "pending_verification"} {
		t.Run(status, func(t *testing.T) {
			repo, _, svc, sub := newInvalidationFixture(t)
			_, err := svc.UpdateMembership(context.Background(), caapp.UpdateMembershipRequest{Subject: sub, MembershipID: "m-target", Status: status})
			requireHTTPCode(t, err, 400, perr.CodeInvalidRequest)
			m, _ := repo.GetMembershipByID(context.Background(), "m-target")
			if m.Status != "active" {
				t.Fatalf("status changed to %q", m.Status)
			}
		})
	}
}

func TestUpdateMembership_StoresNormalizedStatus(t *testing.T) {
	repo, _, svc, sub := newInvalidationFixture(t)
	if _, err := svc.UpdateMembership(context.Background(), caapp.UpdateMembershipRequest{Subject: sub, MembershipID: "m-target", Status: " Inactive "}); err != nil {
		t.Fatal(err)
	}
	if m, _ := repo.GetMembershipByID(context.Background(), "m-target"); m.Status != "inactive" {
		t.Fatalf("stored status = %q, want inactive", m.Status)
	}
}

// lookupFailingRepo makes the primary-admin lookup fail.
type lookupFailingRepo struct{ *cainmem.AdminRepository }

func (r lookupFailingRepo) GetMembershipByID(context.Context, string) (*caapp.MembershipView, error) {
	return nil, errors.New("db unavailable")
}

// PERF-12: the primary-admin guard must fail closed when the lookup fails.
func TestPrimaryAdminGuard_FailsClosedWhenLookupFails(t *testing.T) {
	repo, _, _, sub := newInvalidationFixture(t)
	svc := caapp.NewAdminService(lookupFailingRepo{repo}, fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"rbac.manage"}},
		fixedIDGen("test-id"))
	if _, err := svc.UpdateMembership(context.Background(), caapp.UpdateMembershipRequest{Subject: sub, MembershipID: "m-primary", Status: "inactive"}); err == nil {
		t.Error("deactivation went ahead although the primary-admin check could not run")
	}
	if err := svc.DeleteMembership(context.Background(), caapp.DeleteMembershipRequest{Subject: sub, MembershipID: "m-primary"}); err == nil {
		t.Error("deletion went ahead although the primary-admin check could not run")
	}
	if m, _ := repo.GetMembershipByID(context.Background(), "m-primary"); m == nil || m.Status != "active" {
		t.Fatalf("primary admin changed: %+v", m)
	}
}
