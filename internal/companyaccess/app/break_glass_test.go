package app_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	auditinmem "github.com/cobo/cobo_iam_services/internal/audit/infra/inmemory"
	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

type bgIDGen struct{ n int }

func (g *bgIDGen) NewUUID() string {
	g.n++
	return fmt.Sprintf("bg-%d", g.n)
}

type perMemberAuth struct {
	byMembership map[string][]string
}

func (p perMemberAuth) Authorize(_ context.Context, _ authapp.AuthorizeRequest) (*authapp.AuthorizeDecision, error) {
	return &authapp.AuthorizeDecision{Decision: authapp.DecisionAllow}, nil
}

func (p perMemberAuth) AuthorizeBatch(_ context.Context, _ authapp.AuthorizeBatchRequest) (*authapp.AuthorizeBatchResponse, error) {
	return &authapp.AuthorizeBatchResponse{}, nil
}

func (p perMemberAuth) GetEffectiveAccess(_ context.Context, membershipID, _ string) (*authapp.EffectiveAccessSummary, error) {
	perms := p.byMembership[membershipID]
	return &authapp.EffectiveAccessSummary{Permissions: perms}, nil
}

func newBreakGlassSvc(t *testing.T, repo *cainmem.AdminRepository, auth perMemberAuth) caapp.AdminService {
	t.Helper()
	return caapp.NewAdminService(repo, auth, &bgIDGen{},
		caapp.WithAuditRepository(auditinmem.NewRepository()),
	)
}

func seedBGMember(t *testing.T, repo *cainmem.AdminRepository, sub caapp.AdminSubject) {
	t.Helper()
	seedInviteScopedSubject(t, repo, sub)
}

func seedBGApprover(t *testing.T, repo *cainmem.AdminRepository, sub caapp.AdminSubject) {
	t.Helper()
	seedBGMember(t, repo, sub)
	_ = repo.AddRole(context.Background(), sub.MembershipID, "company_admin")
	_ = repo.AddRolePermission(context.Background(), "company_admin", "rbac.manage")
}

func TestBreakGlass_CreateRequiresReason(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	requester := caapp.AdminSubject{UserID: "u_req", MembershipID: "m_req", CompanyID: "c_bg"}
	target := caapp.AdminSubject{UserID: "u_tgt", MembershipID: "m_tgt", CompanyID: "c_bg"}
	seedBGMember(t, repo, requester)
	seedBGMember(t, repo, target)
	svc := newBreakGlassSvc(t, repo, perMemberAuth{byMembership: map[string][]string{
		"m_req": {"admin.membership.list"},
		"m_tgt": {},
	}})
	_, err := svc.CreateEmergencyAccessRequest(context.Background(), caapp.CreateEmergencyAccessRequest{
		Subject: requester, TargetMembershipID: target.MembershipID, Reason: "  ",
		RequestedDurationSeconds: 3600,
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("expected 400, got %v", err)
	}
}

func TestBreakGlass_CreateRequiresTTLRange(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	requester := caapp.AdminSubject{UserID: "u_req", MembershipID: "m_req", CompanyID: "c_bg"}
	target := caapp.AdminSubject{UserID: "u_tgt", MembershipID: "m_tgt", CompanyID: "c_bg"}
	seedBGMember(t, repo, requester)
	seedBGMember(t, repo, target)
	svc := newBreakGlassSvc(t, repo, perMemberAuth{byMembership: map[string][]string{"m_req": {}, "m_tgt": {}}})
	_, err := svc.CreateEmergencyAccessRequest(context.Background(), caapp.CreateEmergencyAccessRequest{
		Subject: requester, TargetMembershipID: target.MembershipID, Reason: "incident",
		RequestedDurationSeconds: 60,
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("expected 400 for short TTL, got %v", err)
	}
}

func TestBreakGlass_DualApprovalActivatesOverlay(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	requester := caapp.AdminSubject{UserID: "u_req", MembershipID: "m_req", CompanyID: "c_bg"}
	target := caapp.AdminSubject{UserID: "u_tgt", MembershipID: "m_tgt", CompanyID: "c_bg"}
	approver1 := caapp.AdminSubject{UserID: "u_a1", MembershipID: "m_a1", CompanyID: "c_bg"}
	approver2 := caapp.AdminSubject{UserID: "u_a2", MembershipID: "m_a2", CompanyID: "c_bg"}
	seedBGMember(t, repo, requester)
	seedBGMember(t, repo, target)
	seedBGApprover(t, repo, approver1)
	seedBGApprover(t, repo, approver2)
	auth := perMemberAuth{byMembership: map[string][]string{
		"m_req": {}, "m_tgt": {}, "m_a1": {"rbac.manage"}, "m_a2": {"rbac.manage"},
	}}
	svc := newBreakGlassSvc(t, repo, auth)

	grant, err := svc.CreateEmergencyAccessRequest(context.Background(), caapp.CreateEmergencyAccessRequest{
		Subject: requester, TargetMembershipID: target.MembershipID,
		Reason: "locked out", RequestedDurationSeconds: 3600,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if grant.Status != caapp.EmergencyStatusPendingFirst {
		t.Fatalf("status %s", grant.Status)
	}
	grant, err = svc.ApproveEmergencyAccessRequest(context.Background(), caapp.ApproveEmergencyAccessRequest{
		Subject: approver1, SessionID: grant.SessionID,
	})
	if err != nil || grant.Status != caapp.EmergencyStatusPendingSecond {
		t.Fatalf("approve1: %v status=%s", err, grant.Status)
	}
	grant, err = svc.ApproveEmergencyAccessRequest(context.Background(), caapp.ApproveEmergencyAccessRequest{
		Subject: approver2, SessionID: grant.SessionID,
	})
	if err != nil || grant.Status != caapp.EmergencyStatusActive {
		t.Fatalf("approve2: %v status=%s", err, grant.Status)
	}
	if grant.ExpiresAt == nil {
		t.Fatal("expected expires_at")
	}
}

func TestBreakGlass_OverlayGrantsSystemSettings(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	requester := caapp.AdminSubject{UserID: "u_req", MembershipID: "m_req", CompanyID: "c_bg"}
	target := caapp.AdminSubject{UserID: "u_tgt", MembershipID: "m_tgt", CompanyID: "c_bg"}
	approver1 := caapp.AdminSubject{UserID: "u_a1", MembershipID: "m_a1", CompanyID: "c_bg"}
	approver2 := caapp.AdminSubject{UserID: "u_a2", MembershipID: "m_a2", CompanyID: "c_bg"}
	seedBGMember(t, repo, requester)
	seedBGMember(t, repo, target)
	seedBGApprover(t, repo, approver1)
	seedBGApprover(t, repo, approver2)
	auth := perMemberAuth{byMembership: map[string][]string{
		"m_req": {}, "m_tgt": {}, "m_a1": {"rbac.manage"}, "m_a2": {"rbac.manage"},
	}}
	svc := newBreakGlassSvc(t, repo, auth)
	grant, _ := svc.CreateEmergencyAccessRequest(context.Background(), caapp.CreateEmergencyAccessRequest{
		Subject: requester, TargetMembershipID: target.MembershipID,
		Reason: "incident", RequestedDurationSeconds: 3600,
	})
	_, _ = svc.ApproveEmergencyAccessRequest(context.Background(), caapp.ApproveEmergencyAccessRequest{Subject: approver1, SessionID: grant.SessionID})
	_, _ = svc.ApproveEmergencyAccessRequest(context.Background(), caapp.ApproveEmergencyAccessRequest{Subject: approver2, SessionID: grant.SessionID})

	health, err := svc.GetConfigurationHealth(context.Background(), caapp.GetConfigurationHealthRequest{Subject: target})
	if err != nil {
		t.Fatalf("overlay health read denied: %v", err)
	}
	if health == nil {
		t.Fatal("expected health view")
	}
}

func TestBreakGlass_OverlayDoesNotGrantRbacManage(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	requester := caapp.AdminSubject{UserID: "u_req", MembershipID: "m_req", CompanyID: "c_bg"}
	target := caapp.AdminSubject{UserID: "u_tgt", MembershipID: "m_tgt", CompanyID: "c_bg"}
	approver1 := caapp.AdminSubject{UserID: "u_a1", MembershipID: "m_a1", CompanyID: "c_bg"}
	approver2 := caapp.AdminSubject{UserID: "u_a2", MembershipID: "m_a2", CompanyID: "c_bg"}
	seedBGMember(t, repo, requester)
	seedBGMember(t, repo, target)
	seedBGApprover(t, repo, approver1)
	seedBGApprover(t, repo, approver2)
	auth := perMemberAuth{byMembership: map[string][]string{
		"m_req": {}, "m_tgt": {}, "m_a1": {"rbac.manage"}, "m_a2": {"rbac.manage"},
	}}
	svc := newBreakGlassSvc(t, repo, auth)
	grant, _ := svc.CreateEmergencyAccessRequest(context.Background(), caapp.CreateEmergencyAccessRequest{
		Subject: requester, TargetMembershipID: target.MembershipID,
		Reason: "incident", RequestedDurationSeconds: 3600,
	})
	_, _ = svc.ApproveEmergencyAccessRequest(context.Background(), caapp.ApproveEmergencyAccessRequest{Subject: approver1, SessionID: grant.SessionID})
	_, _ = svc.ApproveEmergencyAccessRequest(context.Background(), caapp.ApproveEmergencyAccessRequest{Subject: approver2, SessionID: grant.SessionID})

	_, err := svc.ListEmergencyAccessRequests(context.Background(), caapp.ListEmergencyAccessRequests{Subject: target})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusForbidden {
		t.Fatalf("expected 403 without rbac.manage, got %v", err)
	}
}

func TestBreakGlass_NoSelfApproval(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	requester := caapp.AdminSubject{UserID: "u_req", MembershipID: "m_req", CompanyID: "c_bg"}
	target := caapp.AdminSubject{UserID: "u_tgt", MembershipID: "m_tgt", CompanyID: "c_bg"}
	seedBGMember(t, repo, requester)
	seedBGMember(t, repo, target)
	_ = repo.AddRole(context.Background(), requester.MembershipID, "company_admin")
	_ = repo.AddRolePermission(context.Background(), "company_admin", "rbac.manage")
	auth := perMemberAuth{byMembership: map[string][]string{"m_req": {"rbac.manage"}, "m_tgt": {}}}
	svc := newBreakGlassSvc(t, repo, auth)
	grant, _ := svc.CreateEmergencyAccessRequest(context.Background(), caapp.CreateEmergencyAccessRequest{
		Subject: requester, TargetMembershipID: target.MembershipID,
		Reason: "incident", RequestedDurationSeconds: 3600,
	})
	_, err := svc.ApproveEmergencyAccessRequest(context.Background(), caapp.ApproveEmergencyAccessRequest{
		Subject: requester, SessionID: grant.SessionID,
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusForbidden {
		t.Fatalf("expected 403 self-approve, got %v", err)
	}
}

func TestBreakGlass_DuplicateApproverDenied(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	requester := caapp.AdminSubject{UserID: "u_req", MembershipID: "m_req", CompanyID: "c_bg"}
	target := caapp.AdminSubject{UserID: "u_tgt", MembershipID: "m_tgt", CompanyID: "c_bg"}
	approver1 := caapp.AdminSubject{UserID: "u_a1", MembershipID: "m_a1", CompanyID: "c_bg"}
	seedBGMember(t, repo, requester)
	seedBGMember(t, repo, target)
	seedBGApprover(t, repo, approver1)
	auth := perMemberAuth{byMembership: map[string][]string{
		"m_req": {}, "m_tgt": {}, "m_a1": {"rbac.manage"},
	}}
	svc := newBreakGlassSvc(t, repo, auth)
	grant, _ := svc.CreateEmergencyAccessRequest(context.Background(), caapp.CreateEmergencyAccessRequest{
		Subject: requester, TargetMembershipID: target.MembershipID,
		Reason: "incident", RequestedDurationSeconds: 3600,
	})
	_, _ = svc.ApproveEmergencyAccessRequest(context.Background(), caapp.ApproveEmergencyAccessRequest{Subject: approver1, SessionID: grant.SessionID})
	_, err := svc.ApproveEmergencyAccessRequest(context.Background(), caapp.ApproveEmergencyAccessRequest{Subject: approver1, SessionID: grant.SessionID})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusConflict {
		t.Fatalf("expected 409 duplicate approver, got %v", err)
	}
}

func TestBreakGlass_RevokeRemovesOverlay(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	requester := caapp.AdminSubject{UserID: "u_req", MembershipID: "m_req", CompanyID: "c_bg"}
	target := caapp.AdminSubject{UserID: "u_tgt", MembershipID: "m_tgt", CompanyID: "c_bg"}
	approver1 := caapp.AdminSubject{UserID: "u_a1", MembershipID: "m_a1", CompanyID: "c_bg"}
	approver2 := caapp.AdminSubject{UserID: "u_a2", MembershipID: "m_a2", CompanyID: "c_bg"}
	seedBGMember(t, repo, requester)
	seedBGMember(t, repo, target)
	seedBGApprover(t, repo, approver1)
	seedBGApprover(t, repo, approver2)
	auth := perMemberAuth{byMembership: map[string][]string{
		"m_req": {}, "m_tgt": {}, "m_a1": {"rbac.manage"}, "m_a2": {"rbac.manage"},
	}}
	svc := newBreakGlassSvc(t, repo, auth)
	grant, _ := svc.CreateEmergencyAccessRequest(context.Background(), caapp.CreateEmergencyAccessRequest{
		Subject: requester, TargetMembershipID: target.MembershipID,
		Reason: "incident", RequestedDurationSeconds: 3600,
	})
	_, _ = svc.ApproveEmergencyAccessRequest(context.Background(), caapp.ApproveEmergencyAccessRequest{Subject: approver1, SessionID: grant.SessionID})
	grant, _ = svc.ApproveEmergencyAccessRequest(context.Background(), caapp.ApproveEmergencyAccessRequest{Subject: approver2, SessionID: grant.SessionID})
	_, _ = svc.RevokeEmergencyAccessRequest(context.Background(), caapp.RevokeEmergencyAccessRequest{Subject: approver1, SessionID: grant.SessionID})
	_, err := svc.GetConfigurationHealth(context.Background(), caapp.GetConfigurationHealthRequest{Subject: target})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusForbidden {
		t.Fatalf("expected 403 after revoke, got %v", err)
	}
}

func TestBreakGlass_ExpireRemovesOverlay(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	requester := caapp.AdminSubject{UserID: "u_req", MembershipID: "m_req", CompanyID: "c_bg"}
	target := caapp.AdminSubject{UserID: "u_tgt", MembershipID: "m_tgt", CompanyID: "c_bg"}
	approver1 := caapp.AdminSubject{UserID: "u_a1", MembershipID: "m_a1", CompanyID: "c_bg"}
	approver2 := caapp.AdminSubject{UserID: "u_a2", MembershipID: "m_a2", CompanyID: "c_bg"}
	seedBGMember(t, repo, requester)
	seedBGMember(t, repo, target)
	seedBGApprover(t, repo, approver1)
	seedBGApprover(t, repo, approver2)
	auth := perMemberAuth{byMembership: map[string][]string{
		"m_req": {}, "m_tgt": {}, "m_a1": {"rbac.manage"}, "m_a2": {"rbac.manage"},
	}}
	svc := newBreakGlassSvc(t, repo, auth)
	grant, _ := svc.CreateEmergencyAccessRequest(context.Background(), caapp.CreateEmergencyAccessRequest{
		Subject: requester, TargetMembershipID: target.MembershipID,
		Reason: "incident", RequestedDurationSeconds: 900,
	})
	_, _ = svc.ApproveEmergencyAccessRequest(context.Background(), caapp.ApproveEmergencyAccessRequest{Subject: approver1, SessionID: grant.SessionID})
	grant, _ = svc.ApproveEmergencyAccessRequest(context.Background(), caapp.ApproveEmergencyAccessRequest{Subject: approver2, SessionID: grant.SessionID})
	_, _ = repo.ExpireEmergencyGrant(context.Background(), "c_bg", grant.SessionID)
	_, err := svc.GetConfigurationHealth(context.Background(), caapp.GetConfigurationHealthRequest{Subject: target})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusForbidden {
		t.Fatalf("expected 403 after expire, got %v", err)
	}
}

func TestBreakGlass_CrossCompanyDenied(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	requester := caapp.AdminSubject{UserID: "u_req", MembershipID: "m_req", CompanyID: "c_bg"}
	other := caapp.AdminSubject{UserID: "u_other", MembershipID: "m_other", CompanyID: "c_other"}
	target := caapp.AdminSubject{UserID: "u_tgt", MembershipID: "m_tgt", CompanyID: "c_bg"}
	seedBGMember(t, repo, requester)
	seedBGMember(t, repo, target)
	seedBGMember(t, repo, other)
	_ = repo.AddRole(context.Background(), other.MembershipID, "company_admin")
	_ = repo.AddRolePermission(context.Background(), "company_admin", "rbac.manage")
	auth := perMemberAuth{byMembership: map[string][]string{"m_req": {}, "m_other": {"rbac.manage"}}}
	svc := newBreakGlassSvc(t, repo, auth)
	grant, _ := svc.CreateEmergencyAccessRequest(context.Background(), caapp.CreateEmergencyAccessRequest{
		Subject: requester, TargetMembershipID: target.MembershipID,
		Reason: "incident", RequestedDurationSeconds: 3600,
	})
	_, err := svc.ApproveEmergencyAccessRequest(context.Background(), caapp.ApproveEmergencyAccessRequest{
		Subject: other, SessionID: grant.SessionID,
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus == http.StatusOK {
		t.Fatalf("expected error for cross-company, got %v", err)
	}
}

func TestBreakGlass_AuditAndTimeline(t *testing.T) {
	auditRepo := auditinmem.NewRepository()
	repo := cainmem.NewAdminRepository()
	requester := caapp.AdminSubject{UserID: "u_req", MembershipID: "m_req", CompanyID: "c_bg"}
	target := caapp.AdminSubject{UserID: "u_tgt", MembershipID: "m_tgt", CompanyID: "c_bg"}
	approver1 := caapp.AdminSubject{UserID: "u_a1", MembershipID: "m_a1", CompanyID: "c_bg"}
	approver2 := caapp.AdminSubject{UserID: "u_a2", MembershipID: "m_a2", CompanyID: "c_bg"}
	seedBGMember(t, repo, requester)
	seedBGMember(t, repo, target)
	seedBGApprover(t, repo, approver1)
	seedBGApprover(t, repo, approver2)
	auth := perMemberAuth{byMembership: map[string][]string{
		"m_req": {}, "m_tgt": {"rbac.manage"}, "m_a1": {"rbac.manage"}, "m_a2": {"rbac.manage"},
	}}
	svc := caapp.NewAdminService(repo, auth, &bgIDGen{}, caapp.WithAuditRepository(auditRepo))
	grant, _ := svc.CreateEmergencyAccessRequest(context.Background(), caapp.CreateEmergencyAccessRequest{
		Subject: requester, TargetMembershipID: target.MembershipID,
		Reason: "incident", RequestedDurationSeconds: 3600,
	})
	_, _ = svc.ApproveEmergencyAccessRequest(context.Background(), caapp.ApproveEmergencyAccessRequest{Subject: approver1, SessionID: grant.SessionID})
	_, _ = svc.ApproveEmergencyAccessRequest(context.Background(), caapp.ApproveEmergencyAccessRequest{Subject: approver2, SessionID: grant.SessionID})

	entries, _ := auditRepo.ListFiltered(context.Background(), auditapp.ListFilter{
		CompanyID: "c_bg", ResourceType: "break_glass_session", ResourceID: grant.SessionID, Limit: 20,
	})
	if len(entries) < 3 {
		t.Fatalf("expected audit entries, got %d", len(entries))
	}
	tl, err := svc.GetEmergencyAccessTimeline(context.Background(), caapp.GetEmergencyAccessTimelineRequest{
		Subject: target, SessionID: grant.SessionID, Limit: 20,
	})
	if err != nil || len(tl.Items) == 0 {
		t.Fatalf("timeline: %v items=%d", err, len(tl.Items))
	}
}
