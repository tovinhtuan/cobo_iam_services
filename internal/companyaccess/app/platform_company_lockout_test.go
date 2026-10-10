package app_test

import (
	"context"
	"testing"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// Deactivating a company now shuts out its members, so the platform must not be able to
// deactivate the company its operators work from: neither the caller's own company nor any
// company where an active member holds platform.cms.view.

func TestSetPlatformCompanyStatus_OwnCompanyCannotBeDeactivated(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	seedPlatformCompany(repo, "c_platform")
	svc := platformAdminSvc(repo)
	sub := caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "c_platform"}
	err := svc.SetPlatformCompanyStatus(context.Background(), caapp.SetPlatformCompanyStatusRequest{Subject: sub, CompanyID: "c_platform", Status: "inactive"})
	requireHTTPCode(t, err, 409, perr.CodeStateConflict)
	if out, _ := repo.GetCompanyPlatform(context.Background(), "c_platform"); out.Status != "active" {
		t.Fatalf("own company status = %q, want active", out.Status)
	}
	if err := svc.SetPlatformCompanyStatus(context.Background(), caapp.SetPlatformCompanyStatusRequest{Subject: sub, CompanyID: "c_platform", Status: "active"}); err != nil {
		t.Fatalf("activating the own company stays allowed: %v", err)
	}
}

func TestSetPlatformCompanyStatus_CompanyWithOperatorsCannotBeDeactivated(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	seedPlatformCompany(repo, "c_ops")
	seedMem(repo, "m-op", "u-op", "c_ops")
	if err := repo.InsertDirectPermission(context.Background(), "m-op", "c_ops", "platform.cms.view", "u_seed"); err != nil {
		t.Fatal(err)
	}
	svc := platformAdminSvc(repo)
	sub := caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "c_platform"}
	err := svc.SetPlatformCompanyStatus(context.Background(), caapp.SetPlatformCompanyStatusRequest{Subject: sub, CompanyID: "c_ops", Status: "inactive"})
	requireHTTPCode(t, err, 409, perr.CodeStateConflict)

	if err := repo.RevokeDirectPermission(context.Background(), "m-op", "platform.cms.view", "u_seed"); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetPlatformCompanyStatus(context.Background(), caapp.SetPlatformCompanyStatusRequest{Subject: sub, CompanyID: "c_ops", Status: "inactive"}); err != nil {
		t.Fatalf("company without operators: %v", err)
	}
}
