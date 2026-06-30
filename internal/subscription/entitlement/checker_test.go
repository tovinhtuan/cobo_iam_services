package entitlement_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/subscription/entitlement"
)

func TestChecker_FlagDefaultDisabled(t *testing.T) {
	c := entitlement.Checker{Enabled: false, ResolveUserTier: func(context.Context, string) string { return "Free" }}
	res := c.Check(context.Background(), "u1", entitlement.FeatureNotificationRulesMutation)
	if !res.Allowed {
		t.Fatalf("expected allow when flag off")
	}
}

func TestChecker_AllowPremiumTier(t *testing.T) {
	c := entitlement.Checker{
		Enabled:         true,
		ResolveUserTier: func(context.Context, string) string { return "Premium" },
	}
	res := c.Check(context.Background(), "u1", entitlement.FeatureNotificationRulesMutation)
	if !res.Allowed || res.ReasonCode != "" {
		t.Fatalf("res=%+v", res)
	}
}

func TestChecker_DenyFreeTier(t *testing.T) {
	c := entitlement.Checker{
		Enabled:         true,
		ResolveUserTier: func(context.Context, string) string { return "Free" },
	}
	res := c.Check(context.Background(), "u1", entitlement.FeatureNotificationRulesMutation)
	if res.Allowed || res.ReasonCode != entitlement.ReasonSubscriptionTierDenied {
		t.Fatalf("res=%+v", res)
	}
}

func TestChecker_UnknownTierDenyMutation(t *testing.T) {
	c := entitlement.Checker{
		Enabled:         true,
		ResolveUserTier: func(context.Context, string) string { return "" },
	}
	prefs := map[string]any{
		"schedules": []any{
			map[string]any{"kind": "escalation", "enabled": true, "premium_only": true},
		},
	}
	err := c.ValidateAlertChannelPrefsMutation(context.Background(), "u1", prefs)
	if err == nil {
		t.Fatal("expected error")
	}
	var he *perr.HTTPError
	if !errors.As(err, &he) || he.HTTPStatus != http.StatusPaymentRequired {
		t.Fatalf("err=%v", err)
	}
	if he.Details["reason_code"] != entitlement.ReasonSubscriptionTierUnknown {
		t.Fatalf("details=%v", he.Details)
	}
}

func TestChecker_RuntimePremiumSkip(t *testing.T) {
	c := entitlement.Checker{
		Enabled: true,
		ResolveCompanyTier: func(context.Context, string) string {
			return "Free"
		},
	}
	res := c.CheckRuntimePremium(context.Background(), "c1", true, false)
	if res.Allowed || res.ReasonCode != entitlement.ReasonSubscriptionTierDenied {
		t.Fatalf("res=%+v", res)
	}
}

func TestChecker_RuntimeUnknownTier(t *testing.T) {
	c := entitlement.Checker{
		Enabled: true,
		ResolveCompanyTier: func(context.Context, string) string {
			return ""
		},
	}
	res := c.CheckRuntimePremium(context.Background(), "c1", true, false)
	if res.Allowed || res.ReasonCode != entitlement.ReasonSubscriptionTierUnknown {
		t.Fatalf("res=%+v", res)
	}
}

func TestValidateMutation_FlagOffAllowsPremium(t *testing.T) {
	c := entitlement.Checker{
		Enabled:         false,
		ResolveUserTier: func(context.Context, string) string { return "Free" },
	}
	prefs := map[string]any{
		"schedules": []any{
			map[string]any{"kind": "escalation", "enabled": true, "premium_only": true},
		},
	}
	if err := c.ValidateAlertChannelPrefsMutation(context.Background(), "u1", prefs); err != nil {
		t.Fatalf("unexpected deny: %v", err)
	}
}

func TestValidateMutation_Deny402PremiumSchedule(t *testing.T) {
	c := entitlement.Checker{
		Enabled:         true,
		ResolveUserTier: func(context.Context, string) string { return "Free" },
	}
	prefs := map[string]any{
		"schedules": []any{
			map[string]any{"kind": "escalation", "enabled": true, "premium_only": true},
		},
	}
	err := c.ValidateAlertChannelPrefsMutation(context.Background(), "u1", prefs)
	if err == nil {
		t.Fatal("expected 402")
	}
	var he *perr.HTTPError
	if !errors.As(err, &he) || he.Code != perr.CodeSubscriptionTierRequired {
		t.Fatalf("err=%v", err)
	}
	if he.Details["feature"] != entitlement.FeatureNotificationRulesMutation {
		t.Fatalf("details=%v", he.Details)
	}
	if _, ok := he.Details["password"]; ok {
		t.Fatal("must not leak secrets in details")
	}
}
