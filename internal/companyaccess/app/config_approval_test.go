package app_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	auditinmem "github.com/cobo/cobo_iam_services/internal/audit/infra/inmemory"
	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	authprojection "github.com/cobo/cobo_iam_services/internal/authorization/infra/projection"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	"github.com/cobo/cobo_iam_services/internal/companyaccess/configversion"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

type seqIDGen struct{ n int }

func (g *seqIDGen) NewUUID() string {
	g.n++
	return fmt.Sprintf("appr-%d", g.n)
}

func newApprovalSvc(t *testing.T, repo *cainmem.AdminRepository, perms ...string) caapp.AdminService {
	t.Helper()
	if len(perms) == 0 {
		perms = []string{"rbac.manage", "system.settings", "admin.notification_rule.update"}
	}
	cache := authprojection.NewInMemoryStore(0)
	return caapp.NewAdminService(repo, fakeAuthService{
		decision:    authapp.DecisionAllow,
		permissions: perms,
	}, &seqIDGen{},
		caapp.WithAuditRepository(auditinmem.NewRepository()),
		caapp.WithEffectiveAccessCache(cache),
	)
}

func seedApprovalAdmin(t *testing.T, repo *cainmem.AdminRepository) caapp.AdminSubject {
	t.Helper()
	sub := caapp.AdminSubject{UserID: "u_appr", MembershipID: "m_appr", CompanyID: "c_appr"}
	seedInviteScopedSubject(t, repo, sub)
	_ = repo.AddRolePermission(context.Background(), "company_admin", "rbac.manage")
	_ = repo.AddRolePermission(context.Background(), "company_admin", "system.settings")
	_ = repo.AddRolePermission(context.Background(), "company_admin", "admin.notification_rule.update")
	return sub
}

func createAlertPrefsRule(t *testing.T, svc caapp.AdminService, sub caapp.AdminSubject) string {
	t.Helper()
	err := svc.CreateNotificationRule(context.Background(), caapp.CreateNotificationRuleRequest{
		Subject: sub,
		Payload: map[string]any{
			"rule_code": caapp.AlertChannelPrefsRuleCode,
			"status":    "active",
			"channels":  map[string]any{"email": map[string]any{"enabled": true}},
		},
	})
	if err != nil {
		t.Fatalf("create rule: %v", err)
	}
	rules, _ := svc.ListNotificationRules(context.Background(), caapp.ListNotificationRulesRequest{Subject: sub})
	if len(rules) == 0 {
		t.Fatal("expected rule")
	}
	return rules[0].NotificationRuleID
}

func TestNotificationPatch_RoutesToApprovalQueue(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := seedApprovalAdmin(t, repo)
	svc := newApprovalSvc(t, repo)
	ruleID := createAlertPrefsRule(t, svc, sub)

	err := svc.UpdateNotificationRule(context.Background(), caapp.UpdateNotificationRuleRequest{
		Subject: sub, RuleID: ruleID,
		PayloadPatch: map[string]any{"channels": map[string]any{"sms": map[string]any{"enabled": false}}},
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.Code != perr.CodeApprovalRouted {
		t.Fatalf("expected approval routed, got %v", err)
	}
	list, err := svc.ListConfigApprovals(context.Background(), caapp.ListConfigApprovalsRequest{
		Subject: sub, Status: configversion.ApprovalStatusPending,
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("expected 1 pending, got %d", len(list.Items))
	}
	rules, _ := svc.ListNotificationRules(context.Background(), caapp.ListNotificationRulesRequest{Subject: sub})
	for _, r := range rules {
		if r.NotificationRuleID == ruleID {
			ch, _ := r.Payload["channels"].(map[string]any)
			if _, hasSMS := ch["sms"]; hasSMS {
				t.Fatal("live rule mutated before approval")
			}
		}
	}
}

func TestApproveConfigApproval_Success(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := seedApprovalAdmin(t, repo)
	svc := newApprovalSvc(t, repo)
	ruleID := createAlertPrefsRule(t, svc, sub)
	_ = svc.UpdateNotificationRule(context.Background(), caapp.UpdateNotificationRuleRequest{
		Subject: sub, RuleID: ruleID,
		PayloadPatch: map[string]any{"channels": map[string]any{"sms": map[string]any{"enabled": true}}},
	})
	list, _ := svc.ListConfigApprovals(context.Background(), caapp.ListConfigApprovalsRequest{Subject: sub, Status: "pending"})
	approver := caapp.AdminSubject{UserID: "u_appr2", MembershipID: "m_appr2", CompanyID: sub.CompanyID}
	seedInviteScopedSubject(t, repo, approver)
	_ = repo.AddRolePermission(context.Background(), "company_admin", "system.settings")
	out, err := svc.ApproveConfigApproval(context.Background(), caapp.ApproveConfigApprovalRequest{
		Subject: approver, ApprovalID: list.Items[0].ApprovalID,
	})
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if out.Status != configversion.ApprovalStatusApproved {
		t.Fatalf("expected approved, got %s", out.Status)
	}
}

func TestApproveConfigApproval_SelfApprovalForbidden(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := seedApprovalAdmin(t, repo)
	svc := newApprovalSvc(t, repo)
	ruleID := createAlertPrefsRule(t, svc, sub)
	_ = svc.UpdateNotificationRule(context.Background(), caapp.UpdateNotificationRuleRequest{
		Subject: sub, RuleID: ruleID, PayloadPatch: map[string]any{"channels": map[string]any{}},
	})
	list, _ := svc.ListConfigApprovals(context.Background(), caapp.ListConfigApprovalsRequest{Subject: sub, Status: "pending"})
	_, err := svc.ApproveConfigApproval(context.Background(), caapp.ApproveConfigApprovalRequest{
		Subject: sub, ApprovalID: list.Items[0].ApprovalID,
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.Code != perr.CodeSelfApprovalNotAllowed {
		t.Fatalf("expected self-approval error, got %v", err)
	}
}

func TestRejectAndCancelConfigApproval(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := seedApprovalAdmin(t, repo)
	svc := newApprovalSvc(t, repo)
	ruleID := createAlertPrefsRule(t, svc, sub)
	_ = svc.UpdateNotificationRule(context.Background(), caapp.UpdateNotificationRuleRequest{
		Subject: sub, RuleID: ruleID, PayloadPatch: map[string]any{"channels": map[string]any{}},
	})
	list, _ := svc.ListConfigApprovals(context.Background(), caapp.ListConfigApprovalsRequest{Subject: sub, Status: "pending"})
	approver := caapp.AdminSubject{UserID: "u_rej", MembershipID: "m_rej", CompanyID: sub.CompanyID}
	seedInviteScopedSubject(t, repo, approver)
	_ = repo.AddRolePermission(context.Background(), "company_admin", "system.settings")
	rej, err := svc.RejectConfigApproval(context.Background(), caapp.RejectConfigApprovalRequest{
		Subject: approver, ApprovalID: list.Items[0].ApprovalID, RejectReason: "no",
	})
	if err != nil || rej.Status != configversion.ApprovalStatusRejected {
		t.Fatalf("reject: %v status=%s", err, rej.Status)
	}

	err = svc.UpdateNotificationRule(context.Background(), caapp.UpdateNotificationRuleRequest{
		Subject: sub, RuleID: ruleID, PayloadPatch: map[string]any{"channels": map[string]any{"email": map[string]any{"enabled": false}}},
	})
	if err == nil {
		t.Fatal("expected second patch to route to approval")
	}
	list2, err := svc.ListConfigApprovals(context.Background(), caapp.ListConfigApprovalsRequest{Subject: sub, Status: "pending"})
	if err != nil || len(list2.Items) == 0 {
		t.Fatalf("expected new pending after reject: list err=%v len=%d", err, len(list2.Items))
	}
	can, err := svc.CancelConfigApproval(context.Background(), caapp.CancelConfigApprovalRequest{
		Subject: sub, ApprovalID: list2.Items[0].ApprovalID,
	})
	if err != nil {
		t.Fatalf("cancel err: %v", err)
	}
	if can == nil || can.Status != configversion.ApprovalStatusCancelled {
		t.Fatalf("cancel status: %v", can)
	}
}

func TestConfigApproval_LimitedUserForbidden(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := seedApprovalAdmin(t, repo)
	svc := newApprovalSvc(t, repo, "dashboard.view")
	_, err := svc.ListConfigApprovals(context.Background(), caapp.ListConfigApprovalsRequest{Subject: sub})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusForbidden {
		t.Fatalf("expected 403, got %v", err)
	}
}

func TestConfigApproval_NotFound(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := seedApprovalAdmin(t, repo)
	svc := newApprovalSvc(t, repo)
	_, err := svc.GetConfigApproval(context.Background(), caapp.GetConfigApprovalRequest{
		Subject: sub, ApprovalID: "missing",
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusNotFound {
		t.Fatalf("expected 404, got %v", err)
	}
}

func TestCompareConfigApproval(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := seedApprovalAdmin(t, repo)
	svc := newApprovalSvc(t, repo)
	ruleID := createAlertPrefsRule(t, svc, sub)
	_ = svc.UpdateNotificationRule(context.Background(), caapp.UpdateNotificationRuleRequest{
		Subject: sub, RuleID: ruleID,
		PayloadPatch: map[string]any{"channels": map[string]any{"zalo": map[string]any{"enabled": true}}},
	})
	list, _ := svc.ListConfigApprovals(context.Background(), caapp.ListConfigApprovalsRequest{Subject: sub, Status: "pending"})
	cmp, err := svc.CompareConfigApproval(context.Background(), caapp.CompareConfigApprovalRequest{
		Subject: sub, ApprovalID: list.Items[0].ApprovalID,
	})
	if err != nil || cmp.Compare == nil {
		t.Fatalf("compare: %v", err)
	}
}
