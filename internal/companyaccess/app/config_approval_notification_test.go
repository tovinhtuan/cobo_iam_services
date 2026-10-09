package app_test

// ROLE-17: now that approvals can be decided, the notification_rule.patch route must not queue
// a payload that UpdateNotificationRule would refuse, and approving must re-check the payload.

import (
	"context"
	"encoding/json"
	"testing"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	"github.com/cobo/cobo_iam_services/internal/companyaccess/configversion"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func submitNotificationPatch(svc caapp.AdminService, sub caapp.AdminSubject, ruleID string, patch map[string]any) error {
	_, err := svc.SubmitConfigApproval(context.Background(), caapp.SubmitConfigApprovalRequest{
		Subject: sub, AggregateType: configversion.AggregateNotificationRule, AggregateID: ruleID,
		ChangeType: configversion.ChangeTypeNotificationPatch, Proposed: patch,
	})
	return err
}

func TestSubmitConfigApproval_NotificationPatch_ValidatesPayload(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := seedApprovalAdmin(t, repo)
	svc := newApprovalSvc(t, repo)
	ruleID := createAlertPrefsRule(t, svc, sub)

	// version 2 is refused by UpdateNotificationRule ("version must be 1"); the queue must refuse it too.
	err := submitNotificationPatch(svc, sub, ruleID, map[string]any{"version": 2})
	requireHTTPCode(t, err, 400, perr.CodeInvalidRequest)
	list, _ := svc.ListConfigApprovals(context.Background(), caapp.ListConfigApprovalsRequest{Subject: sub, Status: configversion.ApprovalStatusPending})
	if len(list.Items) != 0 {
		t.Fatalf("an invalid patch must not be queued, got %d pending", len(list.Items))
	}

	// A valid patch is still accepted.
	if err := submitNotificationPatch(svc, sub, ruleID, map[string]any{"channels": map[string]any{"sms": map[string]any{"enabled": true}}}); err != nil {
		t.Fatalf("a valid patch must still be queued: %v", err)
	}
}

func TestSubmitConfigApproval_NotificationPatch_OnlyAlertChannelPrefs(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := seedApprovalAdmin(t, repo)
	svc := newApprovalSvc(t, repo)
	if err := svc.CreateNotificationRule(context.Background(), caapp.CreateNotificationRuleRequest{
		Subject: sub, Payload: map[string]any{"rule_code": "other_rule", "status": "active"},
	}); err != nil {
		t.Fatalf("create other rule: %v", err)
	}
	rules, _ := svc.ListNotificationRules(context.Background(), caapp.ListNotificationRulesRequest{Subject: sub})
	var otherID string
	for _, r := range rules {
		if r.RuleCode == "other_rule" {
			otherID = r.NotificationRuleID
		}
	}
	if otherID == "" {
		t.Fatal("other rule not created")
	}
	// Those rules are updated directly (no approval), so the queue is not a way around that path.
	requireHTTPCode(t, submitNotificationPatch(svc, sub, otherID, map[string]any{"x": 1}), 400, perr.CodeInvalidRequest)
}

func TestApproveConfigApproval_NotificationPayloadIsRevalidated(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	sub := seedApprovalAdmin(t, repo)
	svc := newApprovalSvc(t, repo)
	ruleID := createAlertPrefsRule(t, svc, sub)
	ctx := context.Background()

	// A proposal queued before the validation existed (or written some other way).
	bad, _ := json.Marshal(configversion.NotificationRuleSnapshot{
		SchemaVersion: configversion.NotificationSnapshotSchema, NotificationRuleID: ruleID,
		RuleCode: caapp.AlertChannelPrefsRuleCode, Status: "active",
		Payload: map[string]any{"channels": map[string]any{"email": map[string]any{"bogus": true}}},
	})
	ver, err := repo.GetMaxNotificationRuleVersionNo(ctx, sub.CompanyID, ruleID)
	if err != nil {
		t.Fatal(err)
	}
	row, err := repo.InsertPendingAdminChange(ctx, caapp.InsertPendingAdminChangeInput{
		ID: "legacy-notif", CompanyID: sub.CompanyID, ApprovalSubjectType: configversion.ApprovalSubjectConfigSnapshot,
		AggregateType: configversion.AggregateNotificationRule, AggregateID: ruleID, ChangeType: configversion.ChangeTypeNotificationPatch,
		ProposedSnapshotJSON: bad, BaseLiveVersionNo: &ver, RequestedBy: sub.MembershipID,
	})
	if err != nil {
		t.Fatal(err)
	}

	approver := caapp.AdminSubject{UserID: "u_n_apv", MembershipID: "m_n_apv", CompanyID: sub.CompanyID}
	seedInviteScopedSubject(t, repo, approver)
	_, err = newApprovalSvc(t, repo).ApproveConfigApproval(ctx, caapp.ApproveConfigApprovalRequest{Subject: approver, ApprovalID: row.ID})
	requireHTTPCode(t, err, 400, perr.CodeInvalidRequest)
	got, err := repo.GetPendingAdminChange(ctx, sub.CompanyID, row.ID)
	if err != nil || got.Status != configversion.ApprovalStatusPending {
		t.Fatalf("a refused approval must stay pending, got %+v err=%v", got, err)
	}
}
