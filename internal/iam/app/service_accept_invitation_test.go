package app_test

import (
	"context"
	"strings"
	"testing"
	"time"

	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
)

type stubInviteExecutor struct {
	companyID    string
	membershipID string
	userID       string
}

func (s *stubInviteExecutor) PeekUserInvitation(_ context.Context, _ string, _ time.Time) (bool, string, string, time.Time, error) {
	return true, "", "i***@example.com", time.Now().Add(24 * time.Hour), nil
}

func (s *stubInviteExecutor) AcceptUserInvitation(_ context.Context, _ string, _ string, _ time.Time) (string, string, string, string, error) {
	return s.userID, "invitee@example.com", s.companyID, s.membershipID, nil
}

func TestAcceptUserInvitation_UsesInvitedCompanyContext(t *testing.T) {
	ctx := context.Background()
	invite := &stubInviteExecutor{
		userID:       "u_invited",
		companyID:    "c_invited",
		membershipID: "m_invited",
	}
	svc := newTestIAMService(t, testIAMDeps{
		cred:    testCred(),
		members: cainmem.NewMembershipQueryService(),
		opts:    []iamapp.ServiceOption{iamapp.WithUserInvitationExecutor(invite)},
	})

	resp, err := svc.AcceptUserInvitation(ctx, iamapp.AcceptUserInvitationRequest{
		Token:           "raw-token",
		Password:        "longpassword12",
		ConfirmPassword: "longpassword12",
	})
	if err != nil {
		t.Fatalf("AcceptUserInvitation: %v", err)
	}
	if resp.CurrentContext == nil || resp.CurrentContext.CompanyID != "c_invited" || resp.CurrentContext.MembershipID != "m_invited" {
		t.Fatalf("current_context=%+v", resp.CurrentContext)
	}
	if resp.Session == nil || strings.TrimSpace(resp.Session.AccessToken) == "" {
		t.Fatalf("expected session tokens, got %+v", resp.Session)
	}
}
