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

// ROLE-12 / BES-09: the alert channel preferences only change through the approval queue and
// within the company's plan, whatever the route (update, rollback, delete, approve).

type prefsFixture struct {
	svc     caapp.AdminService
	tier    *string
	byUser  map[string]string // per-user tier overrides
	owner   caapp.AdminSubject
	peer    caapp.AdminSubject
	ruleID  string
	premium map[string]any
}

func newPrefsFixture(t *testing.T, withPremiumSchedule bool) *prefsFixture {
	t.Helper()
	tier := "Premium"
	fx := &prefsFixture{
		tier:  &tier,
		owner: caapp.AdminSubject{UserID: "u1", CompanyID: "c1", MembershipID: "m1"},
		peer:  caapp.AdminSubject{UserID: "u2", CompanyID: "c1", MembershipID: "m2"},
	}
	fx.byUser = map[string]string{}
	repo := cainmem.NewAdminRepository()
	seedMem(repo, fx.owner.MembershipID, fx.owner.UserID, fx.owner.CompanyID)
	seedMem(repo, fx.peer.MembershipID, fx.peer.UserID, fx.peer.CompanyID)
	fx.svc = caapp.NewAdminService(repo,
		fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"rbac.manage"}},
		idgen.UUIDv7Generator{},
		caapp.WithSubscriptionTierLookup(func(_ context.Context, userID string) string {
			if t, ok := fx.byUser[userID]; ok {
				return t
			}
			return *fx.tier
		}),
		caapp.WithSubscriptionTierEnforcementEnabled(true),
	)
	def := caapp.DefaultAlertChannelPrefsPayload("m1")
	schedules := []any{}
	if withPremiumSchedule {
		schedules = []any{map[string]any{"kind": "escalation", "enabled": true, "premium_only": true}}
	}
	if err := fx.svc.CreateNotificationRule(context.Background(), caapp.CreateNotificationRuleRequest{
		Subject: fx.owner,
		Payload: map[string]any{
			"rule_code": caapp.AlertChannelPrefsRuleCode, "status": "active",
			"version": def["version"], "event_scope": def["event_scope"], "channels": def["channels"],
			"schedules": schedules, "recipient_policies": def["recipient_policies"],
		},
	}); err != nil {
		t.Fatalf("create prefs rule: %v", err)
	}
	rules, err := fx.svc.ListNotificationRules(context.Background(), caapp.ListNotificationRulesRequest{Subject: fx.owner})
	if err != nil || len(rules) == 0 {
		t.Fatalf("list rules: %v %d", err, len(rules))
	}
	fx.ruleID = rules[0].NotificationRuleID
	return fx
}

func (fx *prefsFixture) rollback(version int) error {
	_, err := fx.svc.RollbackNotificationRuleVersion(context.Background(), caapp.RollbackNotificationRuleVersionRequest{
		Subject: fx.owner, RuleID: fx.ruleID, VersionNo: version, Reason: "test",
	})
	return err
}

func TestNotificationRollback_PrefsRuleIsRoutedToApproval(t *testing.T) {
	fx := newPrefsFixture(t, false)
	err := fx.rollback(1)
	requireHTTPCode(t, err, http.StatusAccepted, perr.CodeApprovalRouted)
	pending, err := fx.svc.ListConfigApprovals(context.Background(), caapp.ListConfigApprovalsRequest{Subject: fx.peer, Status: "pending"})
	if err != nil {
		t.Fatal(err)
	}
	if len(pending.Items) != 1 {
		t.Fatalf("pending approvals = %d, want 1", len(pending.Items))
	}
}

func TestNotificationRollback_PremiumPrefsAfterDowngradeIsRefused(t *testing.T) {
	fx := newPrefsFixture(t, true)
	*fx.tier = "Free"
	err := fx.rollback(1)
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusPaymentRequired {
		t.Fatalf("rollback to premium prefs on a free plan: got %v, want 402", err)
	}
}

func TestDeleteNotificationRule_PrefsRuleIsRefused(t *testing.T) {
	fx := newPrefsFixture(t, false)
	err := fx.svc.DeleteNotificationRule(context.Background(), caapp.DeleteNotificationRuleRequest{Subject: fx.owner, RuleID: fx.ruleID})
	requireHTTPCode(t, err, http.StatusConflict, perr.CodeStateConflict)
}

func TestApprovePrefs_PremiumAfterDowngradeIsRefused(t *testing.T) {
	fx := newPrefsFixture(t, false)
	err := fx.svc.UpdateNotificationRule(context.Background(), caapp.UpdateNotificationRuleRequest{
		Subject: fx.owner, RuleID: fx.ruleID,
		PayloadPatch: map[string]any{"schedules": []any{map[string]any{"kind": "escalation", "enabled": true, "premium_only": true}}},
	})
	requireHTTPCode(t, err, http.StatusAccepted, perr.CodeApprovalRouted)
	pending, err := fx.svc.ListConfigApprovals(context.Background(), caapp.ListConfigApprovalsRequest{Subject: fx.peer, Status: "pending"})
	if err != nil || len(pending.Items) != 1 {
		t.Fatalf("pending: %v %+v", err, pending)
	}
	*fx.tier = "Free"
	_, err = fx.svc.ApproveConfigApproval(context.Background(), caapp.ApproveConfigApprovalRequest{Subject: fx.peer, ApprovalID: pending.Items[0].ApprovalID})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusPaymentRequired {
		t.Fatalf("approving premium prefs on a free plan: got %v, want 402", err)
	}
}

func TestNotificationRollback_PrefsApprovedByPeerIsApplied(t *testing.T) {
	fx := newPrefsFixture(t, false)
	// Version 2: an approved edit adds a premium escalation schedule.
	err := fx.svc.UpdateNotificationRule(context.Background(), caapp.UpdateNotificationRuleRequest{
		Subject: fx.owner, RuleID: fx.ruleID,
		PayloadPatch: map[string]any{"schedules": []any{map[string]any{"kind": "escalation", "enabled": true, "premium_only": true}}},
	})
	requireHTTPCode(t, err, http.StatusAccepted, perr.CodeApprovalRouted)
	approveOnlyPending(t, fx)
	requireHTTPCode(t, fx.rollback(1), http.StatusAccepted, perr.CodeApprovalRouted)
	approveOnlyPending(t, fx)
	rules, err := fx.svc.ListNotificationRules(context.Background(), caapp.ListNotificationRulesRequest{Subject: fx.owner})
	if err != nil {
		t.Fatal(err)
	}
	if schedules, _ := rules[0].Payload["schedules"].([]any); len(schedules) != 0 {
		t.Fatalf("approved rollback must restore version 1 (no schedules), got %v", rules[0].Payload["schedules"])
	}
}

func approveOnlyPending(t *testing.T, fx *prefsFixture) {
	t.Helper()
	pending, err := fx.svc.ListConfigApprovals(context.Background(), caapp.ListConfigApprovalsRequest{Subject: fx.peer, Status: "pending"})
	if err != nil || len(pending.Items) != 1 {
		t.Fatalf("pending: %v %+v", err, pending)
	}
	if _, err := fx.svc.ApproveConfigApproval(context.Background(), caapp.ApproveConfigApprovalRequest{Subject: fx.peer, ApprovalID: pending.Items[0].ApprovalID}); err != nil {
		t.Fatalf("approve: %v", err)
	}
}

// BES-20: the plan check at approval uses the requester's tier (the one checked when the change
// was queued), not the approver's.
func TestApprovePrefs_UsesRequesterTierNotApprovers(t *testing.T) {
	fx := newPrefsFixture(t, false)
	fx.byUser["u2"] = "Free" // the approver's own tier
	err := fx.svc.UpdateNotificationRule(context.Background(), caapp.UpdateNotificationRuleRequest{
		Subject: fx.owner, RuleID: fx.ruleID,
		PayloadPatch: map[string]any{"schedules": []any{map[string]any{"kind": "escalation", "enabled": true, "premium_only": true}}},
	})
	requireHTTPCode(t, err, http.StatusAccepted, perr.CodeApprovalRouted)
	approveOnlyPending(t, fx)
}
