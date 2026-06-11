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

func TestPatchOwnCompany_ApplicabilityListed_CP1(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	seedCompany(repo, "c-own", "CoBo VN")
	svc := caapp.NewAdminService(repo, fakeAuthService{decision: authapp.DecisionAllow}, fixedIDGen("x"))

	listed := true
	out, err := svc.PatchOwnCompany(context.Background(), caapp.PatchOwnCompanyRequest{
		Subject:  caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "c-own"},
		IsListed: &listed,
	})
	if err != nil {
		t.Fatalf("PatchOwnCompany err=%v", err)
	}
	if !out.IsListed {
		t.Fatal("expected is_listed true")
	}
}

func TestPatchOwnCompany_BusinessSector_CP2(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	seedCompany(repo, "c-own", "CoBo VN")
	svc := caapp.NewAdminService(repo, fakeAuthService{decision: authapp.DecisionAllow}, fixedIDGen("x"))

	sector := "manufacturing"
	out, err := svc.PatchOwnCompany(context.Background(), caapp.PatchOwnCompanyRequest{
		Subject:        caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "c-own"},
		BusinessSector: &sector,
	})
	if err != nil {
		t.Fatalf("PatchOwnCompany err=%v", err)
	}
	if out.BusinessSector != "manufacturing" {
		t.Fatalf("BusinessSector=%q", out.BusinessSector)
	}
}

func TestPatchOwnCompany_InvalidBusinessSector_CP3(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	seedCompany(repo, "c-own", "CoBo VN")
	svc := caapp.NewAdminService(repo, fakeAuthService{decision: authapp.DecisionAllow}, fixedIDGen("x"))

	bad := "invalid"
	_, err := svc.PatchOwnCompany(context.Background(), caapp.PatchOwnCompanyRequest{
		Subject:        caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "c-own"},
		BusinessSector: &bad,
	})
	if err == nil {
		t.Fatal("expected error for invalid business_sector")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("expected 400 got %v", err)
	}
}

func TestGetOwnCompany_ApplicabilityDefaults_CP4(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	seedCompany(repo, "c-own", "CoBo VN")
	svc := caapp.NewAdminService(repo, fakeAuthService{decision: authapp.DecisionAllow}, fixedIDGen("x"))

	out, err := svc.GetOwnCompany(context.Background(), caapp.GetOwnCompanyRequest{
		Subject: caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "c-own"},
	})
	if err != nil {
		t.Fatalf("GetOwnCompany err=%v", err)
	}
	if out.IsListed || out.IsLargePublic || out.IsNonLargePublic || out.HasSubsidiaries || out.HasSubordinateAccountingUnits {
		t.Fatal("expected all applicability bools false by default")
	}
	if out.BusinessSector != "" {
		t.Fatalf("expected empty sector got %q", out.BusinessSector)
	}
}
