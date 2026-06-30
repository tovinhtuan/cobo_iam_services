package app_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

func tierAdminService(t *testing.T, tier string, enforcement bool) caapp.AdminService {
	t.Helper()
	repo := cainmem.NewAdminRepository()
	return caapp.NewAdminService(repo,
		fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"rbac.manage"}},
		idgen.UUIDv7Generator{},
		caapp.WithSubscriptionTierLookup(func(context.Context, string) string { return tier }),
		caapp.WithSubscriptionTierEnforcementEnabled(enforcement),
	)
}

func TestUpdateNotificationRule_TierEnforcement_FlagOffAllowsPremium(t *testing.T) {
	svc := tierAdminService(t, "Free", false)
	sub := caapp.AdminSubject{UserID: "u1", CompanyID: "c1", MembershipID: "m1"}
	payload := caapp.DefaultAlertChannelPrefsPayload("m1")
	if err := svc.CreateNotificationRule(context.Background(), caapp.CreateNotificationRuleRequest{
		Subject: sub,
		Payload: map[string]any{
			"rule_code": caapp.AlertChannelPrefsRuleCode,
			"status":    "active",
			"company_id": sub.CompanyID,
			"version":    payload["version"],
			"event_scope": payload["event_scope"],
			"channels":    payload["channels"],
			"schedules": []any{
				map[string]any{"kind": "escalation", "enabled": true, "premium_only": true},
			},
			"recipient_policies": payload["recipient_policies"],
		},
	}); err != nil {
		t.Fatalf("create: %v", err)
	}
	rules, _ := svc.ListNotificationRules(context.Background(), caapp.ListNotificationRulesRequest{Subject: sub})
	if len(rules) == 0 {
		t.Fatal("no rules")
	}
	if err := svc.UpdateNotificationRule(context.Background(), caapp.UpdateNotificationRuleRequest{
		Subject: sub,
		RuleID:  rules[0].NotificationRuleID,
		PayloadPatch: map[string]any{
			"schedules": []any{
				map[string]any{"kind": "escalation", "enabled": true, "premium_only": true},
			},
		},
	}); err != nil {
		t.Fatalf("update should pass when flag off: %v", err)
	}
}

func TestUpdateNotificationRule_TierEnforcement_FlagOnDenies402(t *testing.T) {
	svc := tierAdminService(t, "Free", true)
	sub := caapp.AdminSubject{UserID: "u1", CompanyID: "c1", MembershipID: "m1"}
	payload := caapp.DefaultAlertChannelPrefsPayload("m1")
	if err := svc.CreateNotificationRule(context.Background(), caapp.CreateNotificationRuleRequest{
		Subject: sub,
		Payload: map[string]any{
			"rule_code":          caapp.AlertChannelPrefsRuleCode,
			"status":               "active",
			"company_id":           sub.CompanyID,
			"version":              payload["version"],
			"event_scope":          payload["event_scope"],
			"channels":             payload["channels"],
			"schedules":            payload["schedules"],
			"recipient_policies":   payload["recipient_policies"],
		},
	}); err != nil {
		t.Fatalf("create: %v", err)
	}
	rules, _ := svc.ListNotificationRules(context.Background(), caapp.ListNotificationRulesRequest{Subject: sub})
	err := svc.UpdateNotificationRule(context.Background(), caapp.UpdateNotificationRuleRequest{
		Subject: sub,
		RuleID:  rules[0].NotificationRuleID,
		PayloadPatch: map[string]any{
			"schedules": []any{
				map[string]any{"kind": "escalation", "enabled": true, "premium_only": true},
			},
		},
	})
	if err == nil {
		t.Fatal("expected 402")
	}
	var he *perr.HTTPError
	if !errors.As(err, &he) || he.HTTPStatus != http.StatusPaymentRequired {
		t.Fatalf("err=%v", err)
	}
}

func TestGetNotificationRuleStatus_TierEnforcedFlag(t *testing.T) {
	svc := tierAdminService(t, "Enterprise", true)
	sub := caapp.AdminSubject{UserID: "u1", CompanyID: "c1", MembershipID: "m1"}
	status, err := svc.GetNotificationRuleStatus(context.Background(), caapp.GetNotificationRuleStatusRequest{Subject: sub})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !status.SubscriptionTierEnforced {
		t.Fatal("expected subscription_tier_enforced true")
	}
}
