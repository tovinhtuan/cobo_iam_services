package app_test

import (
	"context"
	"fmt"
	"testing"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	iaminmem "github.com/cobo/cobo_iam_services/internal/iam/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func TestLogin_invalidCredentials(t *testing.T) {
	ctx := context.Background()
	svc := newTestIAMService(t, testIAMDeps{
		cred: &iaminmem.StaticCredentialVerifier{Users: map[string]iaminmem.StaticUser{
			"a@x.com": {UserID: "u1", LoginID: "a@x.com", Password: "ok", FullName: "A", Status: "active"},
		}},
		members: &cainmem.MembershipQueryService{ByUser: map[string][]caapp.MembershipView{
			"u1": {{MembershipID: "m1", UserID: "u1", CompanyID: "c1", Status: "active"}},
		}},
	})

	_, err := svc.Login(ctx, iamapp.LoginRequest{LoginID: "a@x.com", Password: "wrong"})
	if err == nil {
		t.Fatal("expected error")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.Code != perr.CodeInvalidCredentials {
		t.Fatalf("got %#v", err)
	}
}

func TestLogin_singleCompany_autoContext(t *testing.T) {
	ctx := context.Background()
	svc := newTestIAMService(t, testIAMDeps{
		cred: testCred(),
		members: &cainmem.MembershipQueryService{ByUser: map[string][]caapp.MembershipView{
			"u_single": {{MembershipID: "m_010", UserID: "u_single", CompanyID: "c_010", CompanyName: "Solo", Status: "active"}},
		}},
	})

	resp, err := svc.Login(ctx, iamapp.LoginRequest{LoginID: "single@example.com", Password: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.NextAction != "load_effective_access" {
		t.Fatalf("next_action=%q", resp.NextAction)
	}
	if resp.Session.AccessToken == "" || resp.Session.RefreshToken == "" {
		t.Fatalf("missing tokens: %+v", resp.Session)
	}
	if resp.CurrentContext == nil || resp.CurrentContext.CompanyID != "c_010" || !resp.CurrentContext.AutoSelected {
		t.Fatalf("context=%+v", resp.CurrentContext)
	}
}

func TestLogin_multiCompany_preCompanyToken(t *testing.T) {
	ctx := context.Background()
	svc := newTestIAMService(t, testIAMDeps{
		cred:    testCred(),
		members: cainmem.NewMembershipQueryService(),
	})

	resp, err := svc.Login(ctx, iamapp.LoginRequest{LoginID: "user@example.com", Password: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.NextAction != "select_company" {
		t.Fatalf("next_action=%q", resp.NextAction)
	}
	if resp.Session.PreCompanyToken == "" || resp.Session.AccessToken != "" {
		t.Fatalf("session=%+v", resp.Session)
	}
	if len(resp.Memberships) != 2 {
		t.Fatalf("memberships=%d", len(resp.Memberships))
	}
}

func TestLogin_noActiveMembership(t *testing.T) {
	ctx := context.Background()
	svc := newTestIAMService(t, testIAMDeps{
		cred: testCred(),
		members: &cainmem.MembershipQueryService{ByUser: map[string][]caapp.MembershipView{
			"u_123": {{MembershipID: "m1", UserID: "u_123", CompanyID: "c1", Status: "suspended"}},
		}},
	})

	_, err := svc.Login(ctx, iamapp.LoginRequest{LoginID: "user@example.com", Password: "secret"})
	if err == nil {
		t.Fatal("expected error")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.Code != perr.CodeNoActiveCompanyAccess {
		t.Fatalf("got %#v", err)
	}
}

func TestLogin_accountNotActive(t *testing.T) {
	ctx := context.Background()
	svc := newTestIAMService(t, testIAMDeps{
		cred: &iaminmem.StaticCredentialVerifier{Users: map[string]iaminmem.StaticUser{
			"inactive@x.com": {UserID: "u_i", LoginID: "inactive@x.com", Password: "x", FullName: "I", Status: "suspended"},
		}},
		members: &cainmem.MembershipQueryService{ByUser: map[string][]caapp.MembershipView{
			"u_i": {{MembershipID: "m1", UserID: "u_i", CompanyID: "c1", Status: "active"}},
		}},
	})

	_, err := svc.Login(ctx, iamapp.LoginRequest{LoginID: "inactive@x.com", Password: "x"})
	if err == nil {
		t.Fatal("expected error")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.Code != perr.CodeAccountLocked {
		t.Fatalf("got %#v", err)
	}
}

func TestRefresh_requiresCompanyContext(t *testing.T) {
	ctx := context.Background()
	sessions := iaminmem.NewSessionRepository()
	id := &testSeqID{}
	tokens := iaminmem.NewTokenManager(id)
	svc := iamapp.NewService(testCred(), sessions, tokens, cainmem.NewMembershipQueryService(), id)

	resp, err := svc.Login(ctx, iamapp.LoginRequest{LoginID: "user@example.com", Password: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	// Session created without company for multi-company flow
	ss, _ := sessions.FindByRefreshToken(ctx, resp.Session.RefreshToken)
	if ss.CompanyID != "" {
		t.Fatalf("expected empty company on session, got %q", ss.CompanyID)
	}

	_, err = svc.Refresh(ctx, iamapp.RefreshRequest{RefreshToken: resp.Session.RefreshToken})
	if err == nil {
		t.Fatal("expected error")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.Code != perr.CodeCompanyContextRequired {
		t.Fatalf("got %#v", err)
	}
}

func TestRefresh_rotatesRefreshToken(t *testing.T) {
	ctx := context.Background()
	sessions := iaminmem.NewSessionRepository()
	id := &testSeqID{}
	tokens := iaminmem.NewTokenManager(id)
	members := &cainmem.MembershipQueryService{ByUser: map[string][]caapp.MembershipView{
		"u_single": {{MembershipID: "m_010", UserID: "u_single", CompanyID: "c_010", CompanyName: "Solo", Status: "active"}},
	}}
	svc := iamapp.NewService(testCred(), sessions, tokens, members, id)

	login, err := svc.Login(ctx, iamapp.LoginRequest{LoginID: "single@example.com", Password: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	r1 := login.Session.RefreshToken
	if r1 == "" {
		t.Fatal("missing refresh from login")
	}

	ref1, err := svc.Refresh(ctx, iamapp.RefreshRequest{RefreshToken: r1})
	if err != nil {
		t.Fatal(err)
	}
	if ref1.RefreshToken == "" || ref1.RefreshToken == r1 {
		t.Fatalf("expected new refresh token, got %q", ref1.RefreshToken)
	}
	if ref1.AccessToken == "" {
		t.Fatal("missing access token")
	}

	_, err = svc.Refresh(ctx, iamapp.RefreshRequest{RefreshToken: r1})
	if err == nil {
		t.Fatal("old refresh token should be invalid after rotation")
	}

	ref2, err := svc.Refresh(ctx, iamapp.RefreshRequest{RefreshToken: ref1.RefreshToken})
	if err != nil {
		t.Fatal(err)
	}
	if ref2.RefreshToken == "" || ref2.RefreshToken == ref1.RefreshToken {
		t.Fatalf("expected second rotation to issue new refresh, got %q", ref2.RefreshToken)
	}
}

func TestMFACheck_blocksBeforeMemberships(t *testing.T) {
	ctx := context.Background()
	mfaErr := perr.NewHTTPError(403, perr.CodeMFARequired, "need mfa", nil)
	id := &testSeqID{}
	svc := iamapp.NewService(testCred(), iaminmem.NewSessionRepository(), iaminmem.NewTokenManager(id),
		cainmem.NewMembershipQueryService(), id,
		iamapp.WithMFACheck(mfaStub{err: mfaErr}),
	)

	_, err := svc.Login(ctx, iamapp.LoginRequest{LoginID: "user@example.com", Password: "secret"})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.Code != perr.CodeMFARequired {
		t.Fatalf("expected MFA_REQUIRED, got %v", err)
	}
}

func TestSSOBridge_skipsPassword(t *testing.T) {
	ctx := context.Background()
	id := &testSeqID{}
	svc := iamapp.NewService(
		&iaminmem.StaticCredentialVerifier{Users: map[string]iaminmem.StaticUser{}},
		iaminmem.NewSessionRepository(),
		iaminmem.NewTokenManager(id),
		&cainmem.MembershipQueryService{ByUser: map[string][]caapp.MembershipView{
			"u_sso": {{MembershipID: "m_s", UserID: "u_sso", CompanyID: "c_s", Status: "active"}},
		}},
		id,
		iamapp.WithSSOLoginBridge(ssoStub{user: &iamapp.AuthenticatedUser{UserID: "u_sso", FullName: "SSO", Status: "active"}}),
	)

	resp, err := svc.Login(ctx, iamapp.LoginRequest{LoginID: "", Password: ""})
	if err != nil {
		t.Fatal(err)
	}
	if resp.NextAction != "load_effective_access" {
		t.Fatalf("next_action=%q", resp.NextAction)
	}
}

// --- test harness

type testIAMDeps struct {
	cred    *iaminmem.StaticCredentialVerifier
	members *cainmem.MembershipQueryService
}

func testCred() *iaminmem.StaticCredentialVerifier {
	return &iaminmem.StaticCredentialVerifier{Users: map[string]iaminmem.StaticUser{
		"user@example.com":   {UserID: "u_123", LoginID: "user@example.com", Password: "secret", FullName: "U", Status: "active"},
		"single@example.com": {UserID: "u_single", LoginID: "single@example.com", Password: "secret", FullName: "S", Status: "active"},
	}}
}

func newTestIAMService(t *testing.T, d testIAMDeps) iamapp.Service {
	t.Helper()
	id := &testSeqID{}
	return iamapp.NewService(d.cred, iaminmem.NewSessionRepository(), iaminmem.NewTokenManager(id), d.members, id)
}

type testSeqID struct{ n int }

func (s *testSeqID) NewUUID() string {
	s.n++
	return fmt.Sprintf("test-uuid-%d", s.n)
}

type mfaStub struct{ err error }

func (m mfaStub) VerifyAfterPrimaryAuth(ctx context.Context, user *iamapp.AuthenticatedUser, req iamapp.LoginRequest) error {
	return m.err
}

type ssoStub struct{ user *iamapp.AuthenticatedUser }

func (s ssoStub) TryExternalPrimaryAuth(ctx context.Context, req iamapp.LoginRequest) (*iamapp.AuthenticatedUser, bool, error) {
	return s.user, true, nil
}
