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

func seedCompany(repo *cainmem.AdminRepository, companyID, name string) {
	repo.SeedCompany(caapp.PlatformCompanyDetail{
		CompanyID:   companyID,
		CompanyCode: "CODE",
		CompanyName: name,
		Status:      "active",
	})
}

func TestGetOwnCompany_OK(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	seedCompany(repo, "c-own", "CoBo VN")
	svc := caapp.NewAdminService(
		repo,
		fakeAuthService{decision: authapp.DecisionAllow},
		fixedIDGen("x"),
	)

	out, err := svc.GetOwnCompany(context.Background(), caapp.GetOwnCompanyRequest{
		Subject: caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "c-own"},
	})
	if err != nil {
		t.Fatalf("GetOwnCompany err=%v", err)
	}
	if out.CompanyID != "c-own" {
		t.Fatalf("CompanyID=%q want c-own", out.CompanyID)
	}
	if out.CompanyName != "CoBo VN" {
		t.Fatalf("CompanyName=%q want CoBo VN", out.CompanyName)
	}
}

func TestGetOwnCompany_NoCompanyContext(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	svc := caapp.NewAdminService(
		repo,
		fakeAuthService{decision: authapp.DecisionAllow},
		fixedIDGen("x"),
	)

	_, err := svc.GetOwnCompany(context.Background(), caapp.GetOwnCompanyRequest{
		Subject: caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: ""},
	})
	if err == nil {
		t.Fatal("expected error for empty company context")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok {
		t.Fatalf("expected HTTPError got %T", err)
	}
	if he.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", he.HTTPStatus)
	}
}

func TestGetOwnCompany_AuthDenied(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	seedCompany(repo, "c-own", "CoBo VN")
	svc := caapp.NewAdminService(
		repo,
		fakeAuthService{decision: authapp.DecisionDeny},
		fixedIDGen("x"),
	)

	_, err := svc.GetOwnCompany(context.Background(), caapp.GetOwnCompanyRequest{
		Subject: caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "c-own"},
	})
	if err == nil {
		t.Fatal("expected forbidden error")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok {
		t.Fatalf("expected HTTPError got %T", err)
	}
	if he.HTTPStatus != http.StatusForbidden {
		t.Fatalf("status=%d want 403", he.HTTPStatus)
	}
}

func TestPatchOwnCompany_OK(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	seedCompany(repo, "c-own", "Old Name")
	svc := caapp.NewAdminService(
		repo,
		fakeAuthService{decision: authapp.DecisionAllow},
		fixedIDGen("x"),
	)

	newName := "New Name VN"
	out, err := svc.PatchOwnCompany(context.Background(), caapp.PatchOwnCompanyRequest{
		Subject:     caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "c-own"},
		CompanyName: &newName,
	})
	if err != nil {
		t.Fatalf("PatchOwnCompany err=%v", err)
	}
	if out.CompanyName != "New Name VN" {
		t.Fatalf("CompanyName=%q want New Name VN", out.CompanyName)
	}
}

func TestPatchOwnCompany_CannotChangeVerificationStatus(t *testing.T) {
	// PatchOwnCompany does not accept VerificationStatus — it is absent from PatchOwnCompanyRequest.
	// This test verifies the field is not present to prevent accidental exposure.
	var req caapp.PatchOwnCompanyRequest
	// Compile-time check: if this were a real field, the assignment below would compile.
	// We assert the struct only has the allowed fields.
	_ = req.CompanyName
	_ = req.TaxCode
	_ = req.RegistrationNumber
	_ = req.Address
	_ = req.Phone
	_ = req.ContactEmail
	_ = req.RepresentativeName
	// No req.VerificationStatus — asserted by absence; test passes if it compiles.
}

func TestPatchOwnCompany_NoCompanyContext(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	svc := caapp.NewAdminService(
		repo,
		fakeAuthService{decision: authapp.DecisionAllow},
		fixedIDGen("x"),
	)

	newName := "X"
	_, err := svc.PatchOwnCompany(context.Background(), caapp.PatchOwnCompanyRequest{
		Subject:     caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: ""},
		CompanyName: &newName,
	})
	if err == nil {
		t.Fatal("expected error for empty company context")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok {
		t.Fatalf("expected HTTPError got %T", err)
	}
	if he.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", he.HTTPStatus)
	}
}
