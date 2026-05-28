package app_test

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func seedProvisioningUser(t *testing.T, repo *cainmem.AdminRepository, userID string, accountStatus string, unverified bool) {
	t.Helper()
	loginID := userID + "@example.com"
	if unverified {
		loginID = "unverified-" + loginID
	}
	_, err := repo.CreateUser(context.Background(), caapp.UserView{
		UserID: userID, LoginID: loginID, FullName: "Test", AccountStatus: accountStatus,
	}, "hash", caapp.CreateUserOptions{})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
}

func TestInitializeCompany_HappyPath(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	seedProvisioningUser(t, repo, "u1", "active", false)
	svc := caapp.NewAdminService(repo, fakeAuthService{decision: authapp.DecisionAllow}, fixedIDGen("m1"))

	out, err := svc.InitializeCompany(context.Background(), caapp.InitializeCompanyRequest{
		UserID: "u1", CompanyName: "Acme Corp",
	})
	if err != nil {
		t.Fatalf("InitializeCompany: %v", err)
	}
	if out.CompanyID == "" || out.MembershipID != "m1" {
		t.Fatalf("unexpected result: %+v", out)
	}
}

func TestInitializeCompany_BlocksWhenEligibleMembershipExists(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	seedProvisioningUser(t, repo, "u1", "active", false)
	_, _ = repo.CreateMembership(context.Background(), caapp.MembershipView{
		MembershipID: "m_existing", UserID: "u1", CompanyID: "c_old", Status: "active",
	})
	svc := caapp.NewAdminService(repo, fakeAuthService{decision: authapp.DecisionAllow}, fixedIDGen("m1"))

	_, err := svc.InitializeCompany(context.Background(), caapp.InitializeCompanyRequest{
		UserID: "u1", CompanyName: "Another",
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusConflict {
		t.Fatalf("want 409 conflict, got %v", err)
	}
}

func TestInitializeCompany_AllowsWhenOnlyInactiveMembership(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	seedProvisioningUser(t, repo, "u1", "active", false)
	_, _ = repo.CreateMembership(context.Background(), caapp.MembershipView{
		MembershipID: "m_inactive", UserID: "u1", CompanyID: "c_old", Status: "inactive",
	})
	svc := caapp.NewAdminService(repo, fakeAuthService{decision: authapp.DecisionAllow}, fixedIDGen("m1"))

	_, err := svc.InitializeCompany(context.Background(), caapp.InitializeCompanyRequest{
		UserID: "u1", CompanyName: "Fresh Co",
	})
	if err != nil {
		t.Fatalf("expected initialize allowed, got %v", err)
	}
}

func TestInitializeCompany_RequiresEmailVerified(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	seedProvisioningUser(t, repo, "u1", "active", true)
	svc := caapp.NewAdminService(repo, fakeAuthService{decision: authapp.DecisionAllow}, fixedIDGen("m1"))

	_, err := svc.InitializeCompany(context.Background(), caapp.InitializeCompanyRequest{
		UserID: "u1", CompanyName: "Acme",
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.Code != perr.CodeEmailVerificationRequired {
		t.Fatalf("want EMAIL_VERIFICATION_REQUIRED, got %v", err)
	}
}

func TestInitializeCompany_ConcurrentSingleCompany(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	seedProvisioningUser(t, repo, "u1", "active", false)

	var wg sync.WaitGroup
	success := 0
	var mu sync.Mutex
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			gen := fixedIDGen(fmt.Sprintf("m_%d", i))
			s := caapp.NewAdminService(repo, fakeAuthService{decision: authapp.DecisionAllow}, gen)
			_, err := s.InitializeCompany(context.Background(), caapp.InitializeCompanyRequest{
				UserID: "u1", CompanyName: "Race Co",
			})
			if err == nil {
				mu.Lock()
				success++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	if success != 1 {
		t.Fatalf("expected exactly 1 success, got %d", success)
	}
}
