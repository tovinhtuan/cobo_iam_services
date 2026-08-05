package app_test

import (
	"context"
	"net/http"
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func platformAdminSvc(repo *cainmem.AdminRepository) caapp.AdminService {
	return caapp.NewAdminService(
		repo,
		fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"rbac.manage", "system.settings", "admin.membership.create"}},
		fixedIDGen("x"),
	)
}

func seedPlatformCompany(repo *cainmem.AdminRepository, companyID string) {
	repo.SeedCompany(caapp.PlatformCompanyDetail{
		CompanyID:          companyID,
		CompanyCode:        "CODE",
		CompanyName:        "Acme",
		Status:             "active",
		VerificationStatus: "unverified",
	})
}

func TestSetPlatformCompanyStatus_ValidActiveInactive(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	seedPlatformCompany(repo, "c1")
	svc := platformAdminSvc(repo)
	sub := caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "c_platform"}

	for _, st := range []string{"active", "inactive", " ACTIVE "} {
		if err := svc.SetPlatformCompanyStatus(context.Background(), caapp.SetPlatformCompanyStatusRequest{
			Subject: sub, CompanyID: "c1", Status: st,
		}); err != nil {
			t.Fatalf("status=%q err=%v", st, err)
		}
	}
	out, err := repo.GetCompanyPlatform(context.Background(), "c1")
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != "active" {
		t.Fatalf("status=%q want active (last write)", out.Status)
	}
}

func TestSetPlatformCompanyStatus_InvalidRejected(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	seedPlatformCompany(repo, "c1")
	svc := platformAdminSvc(repo)
	sub := caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "c_platform"}

	for _, st := range []string{"suspended", "pending", "verified", "activee", "", "   ", "unknown"} {
		err := svc.SetPlatformCompanyStatus(context.Background(), caapp.SetPlatformCompanyStatusRequest{
			Subject: sub, CompanyID: "c1", Status: st,
		})
		if err == nil {
			t.Fatalf("status=%q expected error", st)
		}
		he, ok := perr.AsHTTPError(err)
		if !ok || he.HTTPStatus != http.StatusBadRequest {
			t.Fatalf("status=%q want 400 HTTPError got %v", st, err)
		}
	}
	out, _ := repo.GetCompanyPlatform(context.Background(), "c1")
	if out.Status != "active" {
		t.Fatalf("status mutated to %q", out.Status)
	}
}

func TestUpdatePlatformCompany_VerificationValid(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	seedPlatformCompany(repo, "c1")
	svc := platformAdminSvc(repo)
	sub := caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "c_platform"}
	v := " VERIFIED "
	name := "Acme Renamed"
	if err := svc.UpdatePlatformCompany(context.Background(), caapp.UpdatePlatformCompanyRequest{
		Subject: sub, CompanyID: "c1", CompanyName: &name, VerificationStatus: &v,
	}); err != nil {
		t.Fatalf("err=%v", err)
	}
	out, _ := repo.GetCompanyPlatform(context.Background(), "c1")
	if out.VerificationStatus != "verified" {
		t.Fatalf("verification=%q want verified", out.VerificationStatus)
	}
	if out.CompanyName != "Acme Renamed" {
		t.Fatalf("name=%q", out.CompanyName)
	}
}

func TestUpdatePlatformCompany_VerificationInvalid_NoPartialWrite(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	seedPlatformCompany(repo, "c1")
	svc := platformAdminSvc(repo)
	sub := caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "c_platform"}
	bad := "verifiedcvx"
	name := "Should Not Persist"
	err := svc.UpdatePlatformCompany(context.Background(), caapp.UpdatePlatformCompanyRequest{
		Subject: sub, CompanyID: "c1", CompanyName: &name, VerificationStatus: &bad,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("want 400 got %v", err)
	}
	out, _ := repo.GetCompanyPlatform(context.Background(), "c1")
	if out.CompanyName != "Acme" {
		t.Fatalf("partial write name=%q", out.CompanyName)
	}
	if out.VerificationStatus != "unverified" {
		t.Fatalf("verification mutated=%q", out.VerificationStatus)
	}
}

func TestUpdatePlatformCompany_VerificationOmitted_Unchanged(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	seedPlatformCompany(repo, "c1")
	svc := platformAdminSvc(repo)
	sub := caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "c_platform"}
	name := "Only Name"
	if err := svc.UpdatePlatformCompany(context.Background(), caapp.UpdatePlatformCompanyRequest{
		Subject: sub, CompanyID: "c1", CompanyName: &name,
	}); err != nil {
		t.Fatal(err)
	}
	out, _ := repo.GetCompanyPlatform(context.Background(), "c1")
	if out.VerificationStatus != "unverified" {
		t.Fatalf("verification changed to %q", out.VerificationStatus)
	}
	if out.CompanyName != "Only Name" {
		t.Fatalf("name=%q", out.CompanyName)
	}
}

func TestUpdatePlatformCompany_VerificationEmptyStringRejected(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	seedPlatformCompany(repo, "c1")
	svc := platformAdminSvc(repo)
	empty := ""
	err := svc.UpdatePlatformCompany(context.Background(), caapp.UpdatePlatformCompanyRequest{
		Subject:            caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "c_platform"},
		CompanyID:          "c1",
		VerificationStatus: &empty,
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetCompanyPlatform_LegacyJunkVerification_ReadTolerant(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	repo.SeedCompany(caapp.PlatformCompanyDetail{
		CompanyID:          "c-junk",
		CompanyCode:        "J",
		CompanyName:        "Junk Co",
		Status:             "active",
		VerificationStatus: "verifiedcvx",
	})
	svc := platformAdminSvc(repo)
	out, err := svc.GetPlatformCompany(context.Background(), caapp.GetPlatformCompanyRequest{
		Subject:   caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "c_platform"},
		CompanyID: "c-junk",
	})
	if err != nil {
		t.Fatalf("read should tolerate junk: %v", err)
	}
	if out.VerificationStatus != "verifiedcvx" {
		t.Fatalf("want passthrough verifiedcvx got %q", out.VerificationStatus)
	}
}

func TestGetOwnCompany_LegacyJunkVerification_ReadTolerant(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	repo.SeedCompany(caapp.PlatformCompanyDetail{
		CompanyID:          "c-own",
		CompanyCode:        "J",
		CompanyName:        "Own",
		Status:             "active",
		VerificationStatus: "verifiedcvx",
	})
	svc := caapp.NewAdminService(repo, fakeAuthService{decision: authapp.DecisionAllow}, fixedIDGen("x"))
	out, err := svc.GetOwnCompany(context.Background(), caapp.GetOwnCompanyRequest{
		Subject: caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "c-own"},
	})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if out.VerificationStatus != "verifiedcvx" {
		t.Fatalf("got %q", out.VerificationStatus)
	}
}
