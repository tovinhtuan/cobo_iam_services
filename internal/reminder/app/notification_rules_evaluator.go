package app

import (
	"context"
	"fmt"
	"strings"
)

// Evaluate performs read-only layer-1 notification rules evaluation (no dispatch).
//
// Semantics (H7, notification-rules-runtime-h7-2026-10-15):
//   - No prefs row → the company keeps today's delivery: email + in-app, no schedule gate and no
//     recipient filter (Q-23, D-H7-1). DefaultsApplied/defaults_applied mark it.
//   - A row that is not active → RULE_DISABLED (explicit admin choice).
//   - The company schedule gates deadline reminders only; workflow-step milestones follow the
//     workflow template (Q-24).
//   - Email and in-app are evaluated separately in ChannelAllowed (AHA-08); Allowed answers for
//     the requested channel so simulation keeps its meaning.
func (e *NotificationRulesEvaluator) Evaluate(ctx context.Context, input EvaluateInput) (EvaluateDecision, error) {
	decision := EvaluateDecision{
		Allowed:        false,
		IdempotencyKey: strings.TrimSpace(input.IdempotencyKey),
		AuditMetadata:  map[string]any{},
	}
	if e == nil || e.reader == nil {
		return decision, fmt.Errorf("notification rules evaluator not configured")
	}
	if !e.consumerEnabled {
		decision.SkipReason = SkipReasonConsumerDisabled
		decision.AuditMetadata["consumer_enabled"] = false
		return decision, nil
	}
	companyID := strings.TrimSpace(input.CompanyID)
	if companyID == "" {
		return decision, fmt.Errorf("company_id is required")
	}
	channel := strings.TrimSpace(strings.ToLower(input.Channel))
	if channel == "" {
		channel = "email"
	}

	doc, err := e.reader.GetCompanyAlertPrefs(ctx, companyID)
	if err != nil {
		return decision, err
	}
	defaultsApplied := false
	if doc == nil {
		doc = defaultAlertChannelPrefsDocument(companyID)
		defaultsApplied = true
	}
	decision.DefaultsApplied = defaultsApplied
	decision.AuditMetadata["defaults_applied"] = defaultsApplied
	if strings.TrimSpace(doc.CompanyID) != "" && doc.CompanyID != companyID {
		decision.SkipReason = SkipReasonCompanyMismatch
		return decision, fmt.Errorf("company mismatch")
	}
	if strings.TrimSpace(strings.ToLower(doc.Status)) != "" && !strings.EqualFold(doc.Status, "active") {
		decision.SkipReason = SkipReasonRuleDisabled
		decision.AuditMetadata["skip_reason"] = decision.SkipReason
		return decision, nil
	}

	decision.Layer1Applied = true
	decision.ActiveChannels = activeChannelsFromDoc(doc)
	decision.DisabledChannels = disabledChannelsFromDoc(doc)
	decision.RecipientPolicies = append([]string(nil), doc.RecipientPolicies...)
	decision.AuditMetadata["rule_code"] = doc.RuleCode
	decision.AuditMetadata["prefs_version"] = doc.Version
	decision.AuditMetadata["consumer_enabled"] = true

	if !eventScopeAllows(doc, input.EventType) {
		decision.SkipReason = SkipReasonEventScopeExcluded
		decision.AuditMetadata["skip_reason"] = decision.SkipReason
		return decision, nil
	}

	switch channel {
	case "zalo", "sms":
		decision.SkipReason = SkipReasonChannelNotImplemented
		decision.AuditMetadata["skip_reason"] = decision.SkipReason
		return decision, nil
	}

	// Q-24: the company schedule only gates deadline reminders. D-H7-1: the defaults never gate.
	if !defaultsApplied && !strings.EqualFold(strings.TrimSpace(input.EventType), "workflow") {
		matched, schedules := matchScheduleAtDispatch(doc, input.DueAt, input.ScheduledAt)
		decision.MatchedSchedules = schedules
		if !matched {
			decision.SkipReason = SkipReasonRuleScheduleNotMatched
			decision.AuditMetadata["skip_reason"] = decision.SkipReason
			return decision, nil
		}
	}

	decision.ChannelAllowed = map[string]bool{
		"email":  channelEnabled(doc, "email"),
		"in_app": channelEnabled(doc, "in_app"),
	}
	if !channelEnabled(doc, channel) {
		decision.SkipReason = SkipReasonChannelDisabled
		decision.AuditMetadata["skip_reason"] = decision.SkipReason
		return decision, nil
	}

	decision.Allowed = true
	decision.AuditMetadata["channel"] = channel
	return decision, nil
}

func channelEnabled(doc *AlertChannelPrefsDocument, channel string) bool {
	if doc == nil {
		return false
	}
	ch, ok := doc.Channels[channel]
	return ok && ch.Enabled
}

// defaultAlertChannelPrefsDocument is the document used when a company never saved alert prefs.
// Channels, event scope and schedules mirror companyaccess DefaultAlertChannelPrefsPayload (parity
// test). Recipient policies are left empty on purpose (D-H7-1): policies only narrow the resolver's
// recipients, and a company that chose nothing must keep receiving what it receives today.
func defaultAlertChannelPrefsDocument(companyID string) *AlertChannelPrefsDocument {
	offsets := []int{30, 7, 3, 0}
	schedules := make([]SchedulePref, 0, len(offsets)+1)
	for i := range offsets {
		o := offsets[i]
		schedules = append(schedules, SchedulePref{OffsetDays: &o, Enabled: true})
	}
	schedules = append(schedules, SchedulePref{Kind: "escalation", Enabled: false, PremiumOnly: true})
	return &AlertChannelPrefsDocument{
		RuleCode:   AlertChannelPrefsRuleCode,
		CompanyID:  companyID,
		Status:     "active",
		Version:    1,
		EventScope: []string{"deadline", "workflow"},
		Channels: map[string]ChannelPref{
			"email":  {Enabled: true},
			"in_app": {Enabled: true},
			"zalo":   {Enabled: false},
			"sms":    {Enabled: false},
		},
		Schedules: schedules,
	}
}
