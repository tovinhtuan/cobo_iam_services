package app

import (
	"context"
	"fmt"
	"strings"
)

// Evaluate performs read-only layer-1 notification rules evaluation (no dispatch).
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
	if doc == nil {
		decision.SkipReason = SkipReasonRuleMissing
		decision.AuditMetadata["rule_code"] = AlertChannelPrefsRuleCode
		decision.AuditMetadata["layer1_row"] = false
		decision.AuditMetadata["skip_reason"] = decision.SkipReason
		return decision, nil
	}
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

	emailCh, emailOK := doc.Channels["email"]
	inAppCh, inAppOK := doc.Channels["in_app"]
	emailEnabled := emailOK && emailCh.Enabled
	inAppOnly := inAppOK && inAppCh.Enabled && !emailEnabled
	if channel == "email" && inAppOnly {
		decision.SkipReason = SkipReasonChannelDisabled
		decision.AuditMetadata["skip_reason"] = decision.SkipReason
		return decision, nil
	}

	switch channel {
	case "zalo", "sms":
		decision.SkipReason = SkipReasonChannelNotImplemented
		decision.AuditMetadata["skip_reason"] = decision.SkipReason
		return decision, nil
	}
	ch, ok := doc.Channels[channel]
	if !ok || !ch.Enabled {
		decision.SkipReason = SkipReasonChannelDisabled
		decision.AuditMetadata["skip_reason"] = decision.SkipReason
		return decision, nil
	}

	matched, schedules := matchScheduleAtDispatch(doc, input.DueAt, input.ScheduledAt)
	decision.MatchedSchedules = schedules
	if !matched {
		decision.SkipReason = SkipReasonRuleScheduleNotMatched
		decision.AuditMetadata["skip_reason"] = decision.SkipReason
		return decision, nil
	}

	decision.Allowed = true
	decision.AuditMetadata["channel"] = channel
	return decision, nil
}
