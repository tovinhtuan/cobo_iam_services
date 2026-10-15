package app

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strings"
	"time"
)

// Shadow evaluation (H7): while NOTIFICATION_RULES_CONSUMER_ENABLED is off, the company alert prefs
// are evaluated next to the real dispatch decision and the difference is recorded. Delivery never
// depends on it: no DB writes, no sends, every error or panic is contained here.

// Shadow outcomes (metric label `outcome`, log field `outcome`).
const (
	ShadowOutcomeSame              = "same"
	ShadowOutcomeWouldSkip         = "would_skip"
	ShadowOutcomeWouldSkipEmail    = "would_skip_email"
	ShadowOutcomeWouldSkipInApp    = "would_skip_in_app"
	ShadowOutcomeRecipientsChanged = "recipients_changed"
	ShadowOutcomeWouldSend         = "would_send"
	ShadowOutcomeBothSkip          = "both_skip"
	ShadowOutcomeError             = "shadow_error"
)

const shadowMetric = "reminder_rules_shadow_total"

func (s *service) recordShadowDecision(ctx context.Context, c DispatchCandidate, asOf time.Time, actual DispatchDecisionResult) {
	defer func() {
		if r := recover(); r != nil {
			// Never re-enter emitShadow here: a second panic would escape (PERF-04).
			if s.dispatchLogger != nil {
				s.dispatchLogger.Error("notification_rules_shadow_panic",
					slog.String("occurrence_id", c.OccurrenceID),
					slog.String("panic", fmt.Sprintf("%.200v", r)),
					slog.String("stack", string(debug.Stack())))
			}
		}
	}()
	deps := s.dispatchDecisionDeps()
	deps.EvaluatorRuntime = s.shadowEvaluator
	in := dispatchDecisionInputFromCandidate(c, asOf)
	in.SystemContext = EvaluateContextShadow
	if len(actual.Recipients) > 0 {
		// Reuse the recipients the real decision resolved: no second resolver round trip.
		in.RecipientEmails = append([]string(nil), actual.Recipients...)
	}
	shadow := evaluateDispatchDecision(ctx, deps, in, DispatchDecisionModeRuntime)
	outcome, reason := classifyShadow(actual, shadow)
	s.emitShadow(c, outcome, reason, actual, shadow)
}

// classifyShadow compares the real decision (prefs not applied) with the shadow one (prefs applied).
func classifyShadow(actual, shadow DispatchDecisionResult) (outcome, reason string) {
	switch {
	case shadow.EvalErr != nil:
		return ShadowOutcomeError, "eval_error"
	case actual.Skip && shadow.Skip:
		return ShadowOutcomeBothSkip, shadow.SkipReason
	case actual.Skip:
		return ShadowOutcomeWouldSend, actual.SkipReason
	case shadow.Skip:
		return ShadowOutcomeWouldSkip, shadow.SkipReason
	case shadow.SkipEmail:
		return ShadowOutcomeWouldSkipEmail, SkipReasonChannelDisabled
	case actual.AllowInApp && !shadow.AllowInApp:
		return ShadowOutcomeWouldSkipInApp, SkipReasonChannelDisabled
	}
	if changed, how := recipientsChanged(actual.Recipients, shadow.Recipients); changed {
		return ShadowOutcomeRecipientsChanged, how
	}
	return ShadowOutcomeSame, "none"
}

func recipientsChanged(actual, shadow []string) (bool, string) {
	a := emailSet(actual)
	b := emailSet(shadow)
	if len(a) == len(b) {
		for e := range a {
			if _, ok := b[e]; !ok {
				return true, "different"
			}
		}
		return false, ""
	}
	if len(b) < len(a) {
		return true, "fewer"
	}
	return true, "more"
}

func emailSet(in []string) map[string]struct{} {
	out := make(map[string]struct{}, len(in))
	for _, e := range in {
		if v := strings.ToLower(strings.TrimSpace(e)); v != "" {
			out[v] = struct{}{}
		}
	}
	return out
}

// emitShadow records the comparison. Recipients are counted, never logged (PII).
func (s *service) emitShadow(c DispatchCandidate, outcome, reason string, actual, shadow DispatchDecisionResult) {
	if reason == "" {
		reason = "none"
	}
	if s.metrics != nil {
		s.metrics.IncCounter(shadowMetric, map[string]string{"outcome": outcome, "reason": reason})
	}
	if s.dispatchLogger == nil {
		return
	}
	attrs := []any{
		slog.String("company_id", c.CompanyID),
		slog.String("occurrence_id", c.OccurrenceID),
		slog.String("scope_type", string(c.ScopeType)),
		slog.String("outcome", outcome),
		slog.String("reason", reason),
		slog.Bool("defaults_applied", shadow.DefaultsApplied),
		slog.Bool("email_allowed", !shadow.Skip && !shadow.SkipEmail),
		slog.Bool("in_app_allowed", !shadow.Skip && shadow.AllowInApp),
		slog.Int("recipients_actual", len(actual.Recipients)),
		slog.Int("recipients_shadow", len(shadow.Recipients)),
	}
	if shadow.EvalErr != nil {
		attrs = append(attrs, slog.String("error", fmt.Sprintf("%.200s", shadow.EvalErr.Error())))
	}
	s.dispatchLogger.Info("notification_rules_shadow", attrs...)
}

// inAppDeliverer returns the in-app creator when it can report delivery (InAppNotificationDeliverer).
func (s *service) inAppDeliverer() InAppNotificationDeliverer {
	if s.inAppCreator == nil {
		return nil
	}
	d, _ := s.inAppCreator.(InAppNotificationDeliverer)
	return d
}

const inAppOnlyMetric = "reminder_in_app_only_total"

// deliverInAppOnly delivers an in-app-only reminder (company prefs turned email off) right after the
// occurrence was claimed and marked SENT. When no in-app notification could be created (no matching
// user, write error), the reminder falls back to email with the occurrence idempotency key, so it is
// never lost silently. Recipients are counted, never logged.
func (s *service) deliverInAppOnly(ctx context.Context, c DispatchCandidate, templateCode string) {
	deliverer := s.inAppDeliverer()
	created, err := 0, error(nil)
	if deliverer != nil {
		created, err = deliverer.DeliverForReminderDispatch(ctx, c)
	}
	if deliverer != nil && err == nil && created > 0 {
		s.metrics.IncCounter(inAppOnlyMetric, map[string]string{"result": "in_app"})
		return
	}
	result := "email_fallback"
	var sendErr error
	if s.emailSender == nil {
		result = "undelivered"
	} else if _, sendErr = s.emailSender.SendReminderEmail(ctx, templateCode, c.TemplatePayload, normalizeEmails(c.RecipientEmails), c.IdempotencyKey); sendErr != nil {
		result = "undelivered"
	}
	s.metrics.IncCounter(inAppOnlyMetric, map[string]string{"result": result})
	if s.dispatchLogger == nil {
		return
	}
	attrs := []any{
		slog.String("occurrence_id", c.OccurrenceID),
		slog.String("company_id", c.CompanyID),
		slog.String("result", result),
		slog.Int("in_app_created", created),
		slog.Int("recipients", len(c.RecipientEmails)),
	}
	if err != nil {
		attrs = append(attrs, slog.String("in_app_error", fmt.Sprintf("%.200s", err.Error())))
	}
	if sendErr != nil {
		attrs = append(attrs, slog.String("email_error", fmt.Sprintf("%.200s", sendErr.Error())))
	}
	if result == "undelivered" {
		s.dispatchLogger.Error("reminder_in_app_only_undelivered", attrs...)
		return
	}
	s.dispatchLogger.Warn("reminder_in_app_only_email_fallback", attrs...)
}
