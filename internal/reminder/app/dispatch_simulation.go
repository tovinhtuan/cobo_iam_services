package app

import (
	"context"
	"strings"
	"time"
)

// SimulateDispatchInput is the reminder-layer simulation request (Batch 3B).
type SimulateDispatchInput struct {
	CompanyID          string
	EventType          string
	Channel            string
	DueAt              time.Time
	ScheduledAt        time.Time
	AsOf               time.Time
	ScopeType          ScopeType
	ScopeID            string
	DisclosureTypeID   string
	WorkflowInstanceID string
	RecipientEmails    []string
	TemplateCode       string
	SimulationID       string
	SubscriptionTier   string
}

// SimulateRecipientSummary is the privacy-safe recipient block in API responses.
type SimulateRecipientSummary struct {
	Count          int      `json:"count"`
	MaskedSamples  []string `json:"masked_samples"`
	PolicyApplied  bool     `json:"policy_applied"`
}

// SimulateDispatchResult is the simulation API response shape.
type SimulateDispatchResult struct {
	SimulationID     string                   `json:"simulation_id"`
	WouldSend        bool                     `json:"would_send"`
	WouldSkip        bool                     `json:"would_skip"`
	Outcome          string                   `json:"outcome"`
	ReasonCode       string                   `json:"reason_code"`
	Channel          string                   `json:"channel"`
	DispatchPath     string                   `json:"dispatch_path"`
	MatchedRuleCode  string                   `json:"matched_rule_code"`
	MatchedSchedules []ScheduleMatch          `json:"matched_schedules"`
	TemplateKey      string                   `json:"template_key"`
	RecipientSummary SimulateRecipientSummary `json:"recipient_summary"`
	Trace            []TraceStep              `json:"trace"`
	Warnings         []string                 `json:"warnings"`
	EvaluatedAt      time.Time                `json:"evaluated_at"`
	EvalErr          error                    `json:"-"`
}

// DispatchSimulator runs read-only dispatch simulations (no side effects).
type DispatchSimulator struct {
	deps DispatchDecisionDeps
}

// NewDispatchSimulator wires read-only simulation dependencies.
func NewDispatchSimulator(deps DispatchDecisionDeps) *DispatchSimulator {
	return &DispatchSimulator{deps: deps}
}

// SimulateDispatchDecision predicts dispatch outcome with full decision trace.
func (sim *DispatchSimulator) SimulateDispatchDecision(ctx context.Context, in SimulateDispatchInput) (*SimulateDispatchResult, error) {
	if sim == nil {
		return nil, errEvaluatorNotConfigured()
	}
	if in.AsOf.IsZero() {
		in.AsOf = time.Now().UTC()
	}
	channel := strings.TrimSpace(strings.ToLower(in.Channel))
	if channel == "" {
		channel = "email"
	}
	eventType := strings.TrimSpace(strings.ToLower(in.EventType))

	decIn := DispatchDecisionInput{
		CompanyID:          strings.TrimSpace(in.CompanyID),
		ScopeType:          in.ScopeType,
		ScopeID:            strings.TrimSpace(in.ScopeID),
		WorkflowInstanceID: strings.TrimSpace(in.WorkflowInstanceID),
		DisclosureTypeID:   strings.TrimSpace(in.DisclosureTypeID),
		TemplateCode:       strings.TrimSpace(in.TemplateCode),
		RecipientEmails:    append([]string(nil), in.RecipientEmails...),
		DueAt:              in.DueAt.UTC(),
		ScheduledAt:        in.ScheduledAt.UTC(),
		AsOf:               in.AsOf.UTC(),
		Channel:            channel,
		EventType:          eventType,
		SystemContext:      EvaluateContextSimulate,
	}
	if strings.TrimSpace(in.SubscriptionTier) != "" {
		decIn.SubscriptionTier = strings.TrimSpace(in.SubscriptionTier)
	}
	dec := evaluateDispatchDecision(ctx, sim.deps, decIn, DispatchDecisionModeSimulate)

	enforced := sim.deps.TierEnforcement != nil && sim.deps.TierEnforcement.Enabled
	warnings := []string{"subscription_tier_enforced: " + boolString(enforced)}
	if enforced {
		wouldBlock := dec.Skip && (dec.SkipReason == SkipReasonSubscriptionTierDenied || dec.SkipReason == SkipReasonSubscriptionTierUnknown)
		warnings = append(warnings, "would_be_blocked_by_tier: "+boolString(wouldBlock))
	}

	res := &SimulateDispatchResult{
		SimulationID:     strings.TrimSpace(in.SimulationID),
		Channel:          channel,
		DispatchPath:     "notification_rules_consumer",
		MatchedRuleCode:  dec.RuleCode,
		MatchedSchedules: dec.MatchedSchedules,
		TemplateKey:      dec.TemplateCode,
		Trace:            dec.Trace,
		Warnings:         warnings,
		EvaluatedAt:      time.Now().UTC(),
		EvalErr:          dec.EvalErr,
	}
	if res.MatchedRuleCode == "" && dec.SkipReason == SkipReasonRuleMissing {
		res.MatchedRuleCode = AlertChannelPrefsRuleCode
	}

	if dec.EvalErr != nil {
		return res, dec.EvalErr
	}

	policyApplied := len(dec.Layer1Policies) > 0
	res.RecipientSummary = SimulateRecipientSummary{
		Count:         len(dec.Recipients),
		MaskedSamples: maskEmailSamples(dec.Recipients, 3),
		PolicyApplied: policyApplied,
	}

	if dec.Skip {
		res.WouldSkip = true
		res.Outcome = "WOULD_SKIP"
		res.ReasonCode = dec.SkipReason
		return res, nil
	}

	res.WouldSend = true
	res.Outcome = "WOULD_SEND"
	return res, nil
}

func maskEmailSamples(emails []string, max int) []string {
	if max <= 0 || len(emails) == 0 {
		return nil
	}
	out := make([]string, 0, max)
	for _, e := range emails {
		if m := maskEmail(e); m != "" {
			out = append(out, m)
		}
		if len(out) >= max {
			break
		}
	}
	return out
}

func maskEmail(email string) string {
	email = strings.TrimSpace(strings.ToLower(email))
	at := strings.LastIndex(email, "@")
	if at <= 0 || at == len(email)-1 {
		return ""
	}
	local := email[:at]
	domain := email[at+1:]
	if len(local) == 0 {
		return "***@" + domain
	}
	return string([]rune(local)[0]) + "***@" + domain
}

func boolString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
