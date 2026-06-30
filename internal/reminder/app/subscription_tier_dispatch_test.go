package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/cobo/cobo_iam_services/internal/reminder/app"
	"github.com/cobo/cobo_iam_services/internal/subscription/entitlement"
)

func TestEvaluateDispatchDecision_TierSkipWhenFlagOn(t *testing.T) {
	reader := &stubRulesReader{prefs: &app.AlertChannelPrefsDocument{
		RuleCode:   app.AlertChannelPrefsRuleCode,
		Status:     "active",
		EventScope: []string{"deadline"},
		Channels: map[string]app.ChannelPref{
			"email": {Enabled: true},
		},
		Schedules: []app.SchedulePref{
			{OffsetDays: intPtr(0), Enabled: true, PremiumOnly: true},
		},
	}}
	eval := app.NewNotificationRulesEvaluator(reader, true)
	checker := &entitlement.Checker{
		Enabled: true,
		ResolveCompanyTier: func(context.Context, string) string {
			return "Free"
		},
	}
	deps := app.DispatchDecisionDeps{
		EvaluatorRuntime:  eval,
		EvaluatorSimulate: eval,
		TierEnforcement:   checker,
	}
	now := time.Date(2026, 6, 30, 10, 0, 0, 0, time.UTC)
	dec := evaluateDispatchDecisionExported(context.Background(), deps, app.DispatchDecisionInput{
		CompanyID:   "c1",
		ScopeType:   app.ScopeTypeDisclosure,
		ScopeID:     "d1",
		EventType:   "deadline",
		DueAt:       now,
		ScheduledAt: now,
		AsOf:        now,
		Channel:     "email",
	}, app.DispatchDecisionModeRuntime)
	if !dec.Skip || dec.SkipReason != app.SkipReasonSubscriptionTierDenied {
		t.Fatalf("skip=%v reason=%q", dec.Skip, dec.SkipReason)
	}
}

func TestEvaluateDispatchDecision_TierFlagOffUnchanged(t *testing.T) {
	reader := &stubRulesReader{prefs: &app.AlertChannelPrefsDocument{
		RuleCode:   app.AlertChannelPrefsRuleCode,
		Status:     "active",
		EventScope: []string{"deadline"},
		Channels: map[string]app.ChannelPref{
			"email": {Enabled: true},
		},
		Schedules: []app.SchedulePref{
			{OffsetDays: intPtr(0), Enabled: true, PremiumOnly: true},
		},
	}}
	eval := app.NewNotificationRulesEvaluator(reader, true)
	checker := &entitlement.Checker{
		Enabled: false,
		ResolveCompanyTier: func(context.Context, string) string {
			return "Free"
		},
	}
	deps := app.DispatchDecisionDeps{
		EvaluatorRuntime:  eval,
		EvaluatorSimulate: eval,
		TierEnforcement:   checker,
	}
	now := time.Date(2026, 6, 30, 10, 0, 0, 0, time.UTC)
	dec := evaluateDispatchDecisionExported(context.Background(), deps, app.DispatchDecisionInput{
		CompanyID:   "c1",
		ScopeType:   app.ScopeTypeDisclosure,
		ScopeID:     "d1",
		EventType:   "deadline",
		DueAt:       now,
		ScheduledAt: now,
		AsOf:        now,
		Channel:     "email",
	}, app.DispatchDecisionModeRuntime)
	if dec.Skip && dec.SkipReason == app.SkipReasonSubscriptionTierDenied {
		t.Fatalf("should not tier-skip when flag off")
	}
}

// evaluateDispatchDecisionExported is a test hook — duplicate call path via prepareDispatch parity tests.
func evaluateDispatchDecisionExported(ctx context.Context, deps app.DispatchDecisionDeps, in app.DispatchDecisionInput, mode app.DispatchDecisionMode) app.DispatchDecisionResult {
	// Use the same package-level logic through SimulateDispatchDecision on a nil sim is awkward;
	// inline minimal re-invocation via NewDispatchSimulator test helper.
	sim := app.NewDispatchSimulator(deps)
	res, err := sim.SimulateDispatchDecision(ctx, app.SimulateDispatchInput{
		CompanyID:   in.CompanyID,
		EventType:   in.EventType,
		Channel:     in.Channel,
		DueAt:       in.DueAt,
		ScheduledAt: in.ScheduledAt,
		AsOf:        in.AsOf,
		ScopeType:   in.ScopeType,
		ScopeID:     in.ScopeID,
	})
	if err != nil {
		return app.DispatchDecisionResult{EvalErr: err}
	}
	out := app.DispatchDecisionResult{
		Skip:       res.WouldSkip,
		SkipReason: res.ReasonCode,
	}
	return out
}

type stubRulesReader struct {
	prefs *app.AlertChannelPrefsDocument
}

func (s *stubRulesReader) GetCompanyAlertPrefs(context.Context, string) (*app.AlertChannelPrefsDocument, error) {
	return s.prefs, nil
}

func intPtr(v int) *int { return &v }
