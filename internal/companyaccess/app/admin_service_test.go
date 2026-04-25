package app_test

import (
	"context"
	"net/http"
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

type fixedIDGen string

func (g fixedIDGen) NewUUID() string { return string(g) }

type fakeAuthService struct {
	decision authapp.Decision
	permissions []string
}

func (f fakeAuthService) Authorize(_ context.Context, _ authapp.AuthorizeRequest) (*authapp.AuthorizeDecision, error) {
	return &authapp.AuthorizeDecision{Decision: f.decision}, nil
}

func (f fakeAuthService) AuthorizeBatch(_ context.Context, _ authapp.AuthorizeBatchRequest) (*authapp.AuthorizeBatchResponse, error) {
	return &authapp.AuthorizeBatchResponse{}, nil
}

func (f fakeAuthService) GetEffectiveAccess(_ context.Context, _, _ string) (*authapp.EffectiveAccessSummary, error) {
	return &authapp.EffectiveAccessSummary{Permissions: f.permissions}, nil
}

func TestAdminService_CreateUser_OK(t *testing.T) {
	svc := caapp.NewAdminService(
		cainmem.NewAdminRepository(),
		fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"system.settings"}},
		fixedIDGen("u_new"),
	)

	out, err := svc.CreateUser(context.Background(), caapp.CreateUserRequest{
		Subject:  caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"},
		LoginID:  "  New.User@Example.com ",
		Password: "StrongPass123!",
		FullName: " New User ",
		Email:    "new.user@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser err=%v", err)
	}
	if out.UserID != "u_new" {
		t.Fatalf("UserID=%q want u_new", out.UserID)
	}
	if out.LoginID != "new.user@example.com" {
		t.Fatalf("LoginID=%q want lowercased", out.LoginID)
	}
	if out.AccountStatus != "active" {
		t.Fatalf("AccountStatus=%q want active", out.AccountStatus)
	}
}

func TestAdminService_CreateUser_WithOptionalMembership(t *testing.T) {
	svc := caapp.NewAdminService(
		cainmem.NewAdminRepository(),
		fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"system.settings"}},
		fixedIDGen("fixed-id"),
	)

	out, err := svc.CreateUser(context.Background(), caapp.CreateUserRequest{
		Subject:          caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"},
		LoginID:          "member.user@example.com",
		Password:         "StrongPass123!",
		FullName:         "Member User",
		CompanyID:        "c_001",
		MembershipStatus: "active",
	})
	if err != nil {
		t.Fatalf("CreateUser err=%v", err)
	}
	if out.MembershipID == "" {
		t.Fatal("expected membership_id in response")
	}
	if out.CompanyID != "c_001" {
		t.Fatalf("company_id=%q want c_001", out.CompanyID)
	}
}

func TestAdminService_CreateUser_EnterpriseAdminForcesCurrentCompany(t *testing.T) {
	svc := caapp.NewAdminService(
		cainmem.NewAdminRepository(),
		fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"system.settings"}},
		fixedIDGen("fixed-id"),
	)

	out, err := svc.CreateUser(context.Background(), caapp.CreateUserRequest{
		Subject:   caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"},
		LoginID:   "force.company@example.com",
		Password:  "StrongPass123!",
		FullName:  "Force Company",
		CompanyID: "",
	})
	if err != nil {
		t.Fatalf("CreateUser err=%v", err)
	}
	if out.CompanyID != "c_001" {
		t.Fatalf("company_id=%q want c_001", out.CompanyID)
	}
}

func TestAdminService_CreateUser_EnterpriseAdminCannotCreateOtherCompany(t *testing.T) {
	svc := caapp.NewAdminService(
		cainmem.NewAdminRepository(),
		fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"system.settings"}},
		fixedIDGen("fixed-id"),
	)

	_, err := svc.CreateUser(context.Background(), caapp.CreateUserRequest{
		Subject:   caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"},
		LoginID:   "cross.company@example.com",
		Password:  "StrongPass123!",
		FullName:  "Cross Company",
		CompanyID: "c_002",
	})
	if err == nil {
		t.Fatal("expected permission denied")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok {
		t.Fatalf("expected HTTPError got %T", err)
	}
	if he.HTTPStatus != http.StatusForbidden {
		t.Fatalf("status=%d want 403", he.HTTPStatus)
	}
}

func TestAdminService_CreateUser_WebAdminCanCreateOtherCompany(t *testing.T) {
	svc := caapp.NewAdminService(
		cainmem.NewAdminRepository(),
		fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"system.settings", "rbac.manage"}},
		fixedIDGen("fixed-id"),
	)

	out, err := svc.CreateUser(context.Background(), caapp.CreateUserRequest{
		Subject:   caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"},
		LoginID:   "web.admin.create@example.com",
		Password:  "StrongPass123!",
		FullName:  "Web Admin Create",
		CompanyID: "c_002",
	})
	if err != nil {
		t.Fatalf("CreateUser err=%v", err)
	}
	if out.CompanyID != "c_002" {
		t.Fatalf("company_id=%q want c_002", out.CompanyID)
	}
}

func TestAdminService_CreateUser_Validation(t *testing.T) {
	svc := caapp.NewAdminService(
		cainmem.NewAdminRepository(),
		fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"system.settings"}},
		idgen.UUIDv7Generator{},
	)
	_, err := svc.CreateUser(context.Background(), caapp.CreateUserRequest{
		Subject:  caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"},
		LoginID:  "",
		Password: "short",
		FullName: "",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok {
		t.Fatalf("expected HTTPError got %T", err)
	}
	if he.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", he.HTTPStatus)
	}
}

func TestAdminService_CreateUser_Denied(t *testing.T) {
	svc := caapp.NewAdminService(
		cainmem.NewAdminRepository(),
		fakeAuthService{decision: authapp.DecisionDeny},
		idgen.UUIDv7Generator{},
	)
	_, err := svc.CreateUser(context.Background(), caapp.CreateUserRequest{
		Subject:  caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"},
		LoginID:  "x@example.com",
		Password: "StrongPass123!",
		FullName: "X",
	})
	if err == nil {
		t.Fatal("expected permission denied")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok {
		t.Fatalf("expected HTTPError got %T", err)
	}
	if he.HTTPStatus != http.StatusForbidden {
		t.Fatalf("status=%d want 403", he.HTTPStatus)
	}
}

