package app_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func newProvisionSvc(t *testing.T, repo *cainmem.AdminRepository, tier string) caapp.AdminService {
	t.Helper()
	opts := []caapp.AdminOption{}
	if tier != "" {
		opts = append(opts, caapp.WithSubscriptionTierLookup(func(context.Context, string) string {
			return tier
		}))
	}
	return caapp.NewAdminService(repo, fakeAuthService{decision: authapp.DecisionAllow}, fixedIDGen("m_gen"), opts...)
}

func seedUserWithInitializedCompany(t *testing.T, repo *cainmem.AdminRepository, userID string) {
	t.Helper()
	seedProvisioningUser(t, repo, userID, "active", false)
	svc := caapp.NewAdminService(repo, fakeAuthService{decision: authapp.DecisionAllow}, fixedIDGen("m_init"))
	_, err := svc.InitializeCompany(context.Background(), caapp.InitializeCompanyRequest{
		UserID: userID, CompanyName: "First Co",
	})
	if err != nil {
		t.Fatalf("InitializeCompany: %v", err)
	}
}

func TestCreateSelfServiceCompany_SuccessWhenEligibleAndUnderQuota(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	seedUserWithInitializedCompany(t, repo, "u1")
	svc := newProvisionSvc(t, repo, "Premium")

	out, err := svc.CreateSelfServiceCompany(context.Background(), caapp.CreateSelfServiceCompanyRequest{
		UserID: "u1", CompanyName: "Second Co",
	})
	if err != nil {
		t.Fatalf("CreateSelfServiceCompany: %v", err)
	}
	if out.CompanyID == "" || out.MembershipID != "m_gen" {
		t.Fatalf("unexpected: %+v", out)
	}
}

func TestCreateSelfServiceCompany_BlocksWhenEligibleZero(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	seedProvisioningUser(t, repo, "u1", "active", false)
	svc := newProvisionSvc(t, repo, "Free")

	_, err := svc.CreateSelfServiceCompany(context.Background(), caapp.CreateSelfServiceCompanyRequest{
		UserID: "u1", CompanyName: "Orphan Co",
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusConflict {
		t.Fatalf("want 409, got %v", err)
	}
}

func TestCreateSelfServiceCompany_QuotaExceededFree(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	seedUserWithInitializedCompany(t, repo, "u1")
	svc := newProvisionSvc(t, repo, "Free")

	_, err := svc.CreateSelfServiceCompany(context.Background(), caapp.CreateSelfServiceCompanyRequest{
		UserID: "u1", CompanyName: "Over Quota",
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusPaymentRequired || he.Code != perr.CodeQuotaExceeded {
		t.Fatalf("want 402 QUOTA_EXCEEDED, got %v", err)
	}
	if he.Details["limit"] != 1 || he.Details["current"] != 1 || he.Details["tier"] != "Free" {
		t.Fatalf("details=%v", he.Details)
	}
}

func TestCreateSelfServiceCompany_EnterpriseUnlimited(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	seedUserWithInitializedCompany(t, repo, "u1")

	for i := 0; i < 3; i++ {
		gen := fixedIDGen(fmt.Sprintf("m_ent_%d", i))
		s := caapp.NewAdminService(repo, fakeAuthService{decision: authapp.DecisionAllow}, gen,
			caapp.WithSubscriptionTierLookup(func(context.Context, string) string { return "Enterprise" }))
		_, err := s.CreateSelfServiceCompany(context.Background(), caapp.CreateSelfServiceCompanyRequest{
			UserID: "u1", CompanyName: "Extra Co",
		})
		if err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
	}
}

func TestCreateSelfServiceCompany_DuplicateTaxCode(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	seedProvisioningUser(t, repo, "u1", "active", false)
	svcInit := caapp.NewAdminService(repo, fakeAuthService{decision: authapp.DecisionAllow}, fixedIDGen("m1"))
	_, err := svcInit.InitializeCompany(context.Background(), caapp.InitializeCompanyRequest{
		UserID: "u1", CompanyName: "First", TaxCode: "0101234567",
	})
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	svc := newProvisionSvc(t, repo, "Premium")
	_, err = svc.CreateSelfServiceCompany(context.Background(), caapp.CreateSelfServiceCompanyRequest{
		UserID: "u1", CompanyName: "Dup Tax", TaxCode: "0101234567",
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusConflict || he.Message != "COMPANY_ALREADY_EXISTS" {
		t.Fatalf("want 409 COMPANY_ALREADY_EXISTS, got %v", err)
	}
}

func TestCreateSelfServiceCompany_PremiumQuotaThree(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	seedUserWithInitializedCompany(t, repo, "u1")
	tier := "Premium"
	for i, id := range []string{"m2", "m3"} {
		s := caapp.NewAdminService(repo, fakeAuthService{decision: authapp.DecisionAllow}, fixedIDGen(id),
			caapp.WithSubscriptionTierLookup(func(context.Context, string) string { return tier }))
		_, err := s.CreateSelfServiceCompany(context.Background(), caapp.CreateSelfServiceCompanyRequest{
			UserID: "u1", CompanyName: "Co " + id,
		})
		if err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
	}
	svc := newProvisionSvc(t, repo, tier)
	_, err := svc.CreateSelfServiceCompany(context.Background(), caapp.CreateSelfServiceCompanyRequest{
		UserID: "u1", CompanyName: "Fourth",
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.Code != perr.CodeQuotaExceeded {
		t.Fatalf("want quota exceeded on 4th company, got %v", err)
	}
	if he.Details["limit"] != 3 {
		t.Fatalf("limit=%v", he.Details["limit"])
	}
}
