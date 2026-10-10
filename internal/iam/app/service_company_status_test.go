package app_test

import (
	"context"
	"testing"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	iaminmem "github.com/cobo/cobo_iam_services/internal/iam/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// A deactivated company ("Ngừng hoạt động") is not offered at sign-in, cannot be selected and
// stops refresh; a user whose only companies are deactivated cannot sign in.

func companyStatusSvc(memberships ...caapp.MembershipView) (iamapp.Service, *cainmem.MembershipQueryService) {
	id := &testSeqID{}
	members := &cainmem.MembershipQueryService{ByUser: map[string][]caapp.MembershipView{"u_single": memberships}}
	return iamapp.NewService(testCred(), iaminmem.NewSessionRepository(), iaminmem.NewTokenManager(id), members, id), members
}

func requireCompanyInactive(t *testing.T, err error) {
	t.Helper()
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != 403 || he.Code != perr.CodeCompanyInactive {
		t.Fatalf("got %#v, want 403 COMPANY_INACTIVE", err)
	}
}

func TestLogin_OnlyCompanyInactiveIsRefused(t *testing.T) {
	svc, _ := companyStatusSvc(caapp.MembershipView{MembershipID: "m_010", UserID: "u_single", CompanyID: "c_010", CompanyName: "Solo", Status: "active", CompanyStatus: "inactive"})
	_, err := svc.Login(context.Background(), iamapp.LoginRequest{LoginID: "single@example.com", Password: "secret"})
	requireCompanyInactive(t, err)
}

func TestLogin_InactiveCompanyIsSkipped(t *testing.T) {
	svc, _ := companyStatusSvc(
		caapp.MembershipView{MembershipID: "m_off", UserID: "u_single", CompanyID: "c_off", CompanyName: "Off", Status: "active", CompanyStatus: "inactive"},
		caapp.MembershipView{MembershipID: "m_on", UserID: "u_single", CompanyID: "c_on", CompanyName: "On", Status: "active", CompanyStatus: "active"},
	)
	resp, err := svc.Login(context.Background(), iamapp.LoginRequest{LoginID: "single@example.com", Password: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.CurrentContext == nil || resp.CurrentContext.CompanyID != "c_on" {
		t.Fatalf("want the only active company auto-selected, got %+v (memberships %+v)", resp.CurrentContext, resp.Memberships)
	}
}

func TestSelectCompany_InactiveCompanyRefused(t *testing.T) {
	id := &testSeqID{}
	tokens := iaminmem.NewTokenManager(id)
	members := &cainmem.MembershipQueryService{ByUser: map[string][]caapp.MembershipView{"u_single": {
		{MembershipID: "m_a", UserID: "u_single", CompanyID: "c_a", CompanyName: "A", Status: "active", CompanyStatus: "active"},
		{MembershipID: "m_b", UserID: "u_single", CompanyID: "c_b", CompanyName: "B", Status: "active", CompanyStatus: "active"},
	}}}
	svc := iamapp.NewService(testCred(), iaminmem.NewSessionRepository(), tokens, members, id)
	ctx := context.Background()
	login, err := svc.Login(ctx, iamapp.LoginRequest{LoginID: "single@example.com", Password: "secret"})
	if err != nil || login.NextAction != "select_company" {
		t.Fatalf("login: %v resp=%+v", err, login)
	}
	pre, err := tokens.InspectPreCompanyToken(ctx, login.Session.PreCompanyToken)
	if err != nil {
		t.Fatal(err)
	}
	members.ByUser["u_single"][1].CompanyStatus = "inactive"
	_, err = svc.SelectCompany(ctx, iamapp.SelectCompanyRequest{UserID: pre.Sub, SessionID: pre.SessionID, CompanyID: "c_b"})
	requireCompanyInactive(t, err)
	if _, err := svc.SelectCompany(ctx, iamapp.SelectCompanyRequest{UserID: pre.Sub, SessionID: pre.SessionID, CompanyID: "c_a"}); err != nil {
		t.Fatalf("active company still selectable: %v", err)
	}
}

func TestRefresh_CompanyDeactivatedAfterLoginRefused(t *testing.T) {
	svc, members := companyStatusSvc(caapp.MembershipView{MembershipID: "m_010", UserID: "u_single", CompanyID: "c_010", CompanyName: "Solo", Status: "active", CompanyStatus: "active"})
	ctx := context.Background()
	login, err := svc.Login(ctx, iamapp.LoginRequest{LoginID: "single@example.com", Password: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	members.ByUser["u_single"][0].CompanyStatus = "inactive"
	_, err = svc.Refresh(ctx, iamapp.RefreshRequest{RefreshToken: login.Session.RefreshToken})
	requireCompanyInactive(t, err)
}
