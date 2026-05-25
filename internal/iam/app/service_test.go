package app_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	iaminmem "github.com/cobo/cobo_iam_services/internal/iam/infra/inmemory"
	notificationapp "github.com/cobo/cobo_iam_services/internal/notification/app"
	notificationregistry "github.com/cobo/cobo_iam_services/internal/notification/infra/registry"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/events"
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
	if resp.PlatformAccessHint {
		t.Fatalf("platform_access_hint should be false for normal multi-company user")
	}
}

func TestLogin_multiCompany_platformAccessHintForAdminRole(t *testing.T) {
	ctx := context.Background()
	svc := newTestIAMService(t, testIAMDeps{
		cred: testCred(),
		members: &cainmem.MembershipQueryService{
			ByUser: map[string][]caapp.MembershipView{
				"u_123": {
					{MembershipID: "m_a", UserID: "u_123", CompanyID: "c_001", CompanyName: "Company X", Status: "active"},
					{MembershipID: "m_b", UserID: "u_123", CompanyID: "c_002", CompanyName: "Company Y", Status: "active"},
				},
			},
			Roles: map[string][]string{
				"m_a": {"department_staff"},
				"m_b": {"company_admin"},
			},
		},
	})

	resp, err := svc.Login(ctx, iamapp.LoginRequest{LoginID: "user@example.com", Password: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.PlatformAccessHint {
		t.Fatalf("platform_access_hint should be true when any active membership has company_admin role")
	}
	if resp.NextAction != "select_company" {
		t.Fatalf("next_action=%q", resp.NextAction)
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

	// Users with no active membership are now allowed to login with restricted access.
	resp, err := svc.Login(ctx, iamapp.LoginRequest{LoginID: "user@example.com", Password: "secret"})
	if err != nil {
		t.Fatalf("expected success for user without active membership, got error: %v", err)
	}
	if resp.NextAction != "no_company_onboarding" {
		t.Fatalf("expected next_action=no_company_onboarding, got %q", resp.NextAction)
	}
	if resp.Session.AccessToken == "" {
		t.Fatal("expected access token to be issued")
	}
}

func TestLogin_noActiveMembership_setsEmailVerifiedWhenRecoveryWired(t *testing.T) {
	ctx := context.Background()
	recovery := &stubRecoveryEmailFlag{verified: false}
	svc := newTestIAMService(t, testIAMDeps{
		cred: testCred(),
		members: &cainmem.MembershipQueryService{ByUser: map[string][]caapp.MembershipView{
			"u_123": {{MembershipID: "m1", UserID: "u_123", CompanyID: "c1", Status: "suspended"}},
		}},
		opts: []iamapp.ServiceOption{iamapp.WithAuthRecoveryRepository(recovery)},
	})

	resp, err := svc.Login(ctx, iamapp.LoginRequest{LoginID: "user@example.com", Password: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.EmailVerified {
		t.Fatalf("expected email_verified=false, got %+v", resp)
	}
}

type stubRecoveryEmailFlag struct {
	verified bool
}

func (s *stubRecoveryEmailFlag) FindUserByEmail(context.Context, string) (*iamapp.RecoveryUser, error) {
	return nil, nil
}
func (s *stubRecoveryEmailFlag) FindUserByUserID(context.Context, string) (*iamapp.RecoveryUser, error) {
	return nil, nil
}
func (s *stubRecoveryEmailFlag) StorePasswordResetToken(context.Context, iamapp.RecoveryTokenRecord) error {
	return nil
}
func (s *stubRecoveryEmailFlag) ConsumePasswordResetToken(context.Context, string, time.Time) (string, error) {
	return "", nil
}
func (s *stubRecoveryEmailFlag) StoreEmailVerificationToken(context.Context, iamapp.RecoveryTokenRecord) error {
	return nil
}
func (s *stubRecoveryEmailFlag) ConsumeEmailVerificationToken(context.Context, string, time.Time) (string, error) {
	return "", nil
}
func (s *stubRecoveryEmailFlag) UpdatePasswordHash(context.Context, string, string, time.Time) error {
	return nil
}
func (s *stubRecoveryEmailFlag) MarkEmailVerified(context.Context, string, time.Time) error {
	return nil
}
func (s *stubRecoveryEmailFlag) IsEmailVerified(context.Context, string) (bool, error) {
	return s.verified, nil
}
func (s *stubRecoveryEmailFlag) InvalidatePendingEmailVerificationOTPs(context.Context, string) error {
	return nil
}
func (s *stubRecoveryEmailFlag) StoreEmailVerificationOTP(context.Context, iamapp.EmailOTPRecord) error {
	return nil
}
func (s *stubRecoveryEmailFlag) CountEmailVerificationOTPsSince(context.Context, string, time.Time) (int, error) {
	return 0, nil
}
func (s *stubRecoveryEmailFlag) TryConsumeEmailVerificationOTP(context.Context, string, string, time.Time) (iamapp.EmailOTPConsumeOutcome, error) {
	return iamapp.EmailOTPNotFound, nil
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

func TestResetPassword_invalidToken_usesDedicatedCode(t *testing.T) {
	ctx := context.Background()
	svc := newTestIAMService(t, testIAMDeps{
		cred:    testCred(),
		members: cainmem.NewMembershipQueryService(),
	})
	_, err := svc.ResetPassword(ctx, iamapp.ResetPasswordRequest{
		Token: "bad-token", NewPassword: "new-password-123",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.Code != perr.CodePasswordResetTokenInvalid {
		t.Fatalf("got %#v", err)
	}
}

func TestVerifyEmail_invalidToken_usesDedicatedCode(t *testing.T) {
	ctx := context.Background()
	svc := newTestIAMService(t, testIAMDeps{
		cred:    testCred(),
		members: cainmem.NewMembershipQueryService(),
	})
	_, err := svc.VerifyEmail(ctx, iamapp.VerifyEmailRequest{Token: "bad-token"})
	if err == nil {
		t.Fatal("expected error")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.Code != perr.CodeEmailVerificationTokenInvalid {
		t.Fatalf("got %#v", err)
	}
}

func TestResendVerificationEmail_requiresRecipient(t *testing.T) {
	ctx := context.Background()
	svc := newTestIAMService(t, testIAMDeps{
		cred:    testCred(),
		members: cainmem.NewMembershipQueryService(),
	})
	_, err := svc.ResendVerificationEmail(ctx, iamapp.ResendVerificationEmailRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.Code != perr.CodeInvalidRequest {
		t.Fatalf("got %#v", err)
	}
}

func TestPublishUserInvitationEmail_EmbedUsesTemplate(t *testing.T) {
	ctx := context.Background()
	publisher := &captureOutboxPublisher{}
	svc := newTestIAMService(t, testIAMDeps{
		cred:    testCred(),
		members: cainmem.NewMembershipQueryService(),
		opts: []iamapp.ServiceOption{
			iamapp.WithOutboxPublisher(publisher),
			iamapp.WithAuthFlowConfig(iamapp.AuthFlowConfig{
				WebBaseURL:             "https://app.example.com",
				UserInvitationTokenTTL: 72 * time.Hour,
				EmailTemplateSource:    "embed",
				EmailTemplateRegistry:  notificationregistry.NewEmbedRegistry(),
				EmailRenderer:          notificationapp.NewEmailRenderer(),
			}),
		},
	})

	if err := svc.PublishUserInvitationEmail(ctx, "u_123", "invitee@example.com", "Nguyen Van A", "invitee@example.com", "token-123", "COBO"); err != nil {
		t.Fatalf("PublishUserInvitationEmail error = %v", err)
	}
	if publisher.last.EventType != "auth.user_invitation_sent" {
		t.Fatalf("event_type = %q", publisher.last.EventType)
	}
	if got := publisher.last.Payload["subject"]; got != "Lời mời thiết lập tài khoản CoBo Portal" {
		t.Fatalf("subject = %v", got)
	}
	body, _ := publisher.last.Payload["body"].(string)
	for _, snippet := range []string{
		"Xin chào Nguyen Van A,",
		"Quý khách đã được mời sử dụng CoBo Portal bởi:",
		"Công ty: COBO",
		"https://app.example.com/accept-invitation?token=token-123",
		"72 giờ",
		"Email hỗ trợ:",
		"Website: https://app.example.com",
	} {
		if !strings.Contains(body, snippet) {
			t.Fatalf("body missing %q\nbody: %q", snippet, body)
		}
	}
}

func TestPublishUserInvitationEmail_EmbedNoCompany(t *testing.T) {
	ctx := context.Background()
	publisher := &captureOutboxPublisher{}
	svc := newTestIAMService(t, testIAMDeps{
		cred:    testCred(),
		members: cainmem.NewMembershipQueryService(),
		opts: []iamapp.ServiceOption{
			iamapp.WithOutboxPublisher(publisher),
			iamapp.WithAuthFlowConfig(iamapp.AuthFlowConfig{
				WebBaseURL:             "https://app.example.com",
				SupportEmail:           "support@cobo.vn",
				UserInvitationTokenTTL: 72 * time.Hour,
				EmailTemplateSource:    "embed",
				EmailTemplateRegistry:  notificationregistry.NewEmbedRegistry(),
				EmailRenderer:          notificationapp.NewEmailRenderer(),
			}),
		},
	})

	if err := svc.PublishUserInvitationEmail(ctx, "u_123", "invitee@example.com", "Nguyen Van A", "invitee@example.com", "token-456", ""); err != nil {
		t.Fatalf("PublishUserInvitationEmail error = %v", err)
	}
	body, _ := publisher.last.Payload["body"].(string)
	if strings.Contains(body, "Công ty:") {
		t.Fatalf("no-company body must not contain company block: %q", body)
	}
	for _, snippet := range []string{
		"Tài khoản của Quý khách đã được tạo trên CoBo Portal",
		"accept-invitation?token=token-456",
	} {
		if !strings.Contains(body, snippet) {
			t.Fatalf("body missing %q", snippet)
		}
	}
}

func TestPublishUserInvitationEmail_EmbedFallsBackToLegacy(t *testing.T) {
	ctx := context.Background()
	publisher := &captureOutboxPublisher{}
	svc := newTestIAMService(t, testIAMDeps{
		cred:    testCred(),
		members: cainmem.NewMembershipQueryService(),
		opts: []iamapp.ServiceOption{
			iamapp.WithOutboxPublisher(publisher),
			iamapp.WithAuthFlowConfig(iamapp.AuthFlowConfig{
				WebBaseURL:             "https://app.example.com",
				UserInvitationTokenTTL: 72 * time.Hour,
				EmailTemplateSource:    "embed",
				EmailTemplateRegistry:  brokenIAMRegistry{},
				EmailRenderer:          notificationapp.NewEmailRenderer(),
			}),
		},
	})

	if err := svc.PublishUserInvitationEmail(ctx, "u_123", "invitee@example.com", "Nguyen Van A", "invitee@example.com", "", "COBO"); err != nil {
		t.Fatalf("PublishUserInvitationEmail error = %v", err)
	}
	wantBody := "Xin chao Nguyen Van A,\n\nCong ty: COBO\n\nBan da duoc them vao tai khoan cong ty tren he thong. Vui long dang nhap bang email va mat khau hien tai cua ban.\n\nNeu ban khong cho doi thao tac nay, hay lien he quan tri vien.\n"
	if got := publisher.last.Payload["body"]; got != wantBody {
		t.Fatalf("legacy fallback body mismatch\nwant: %q\ngot:  %q", wantBody, got)
	}
}

// --- test harness

type testIAMDeps struct {
	cred    *iaminmem.StaticCredentialVerifier
	members *cainmem.MembershipQueryService
	opts    []iamapp.ServiceOption
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
	return iamapp.NewService(d.cred, iaminmem.NewSessionRepository(), iaminmem.NewTokenManager(id), d.members, id, d.opts...)
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

type captureOutboxPublisher struct {
	last events.Event
}

func (c *captureOutboxPublisher) Publish(_ context.Context, event events.Event) error {
	c.last = event
	return nil
}

type brokenIAMRegistry struct{}

func (brokenIAMRegistry) Resolve(context.Context, string, string) (notificationapp.ResolvedTemplate, error) {
	return notificationapp.ResolvedTemplate{}, context.DeadlineExceeded
}
