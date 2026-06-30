package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cobo/cobo_iam_services/internal/subscription/entitlement"
)

// DispatchDecisionMode selects runtime vs simulation evaluation semantics.
type DispatchDecisionMode int

const (
	DispatchDecisionModeRuntime DispatchDecisionMode = iota
	DispatchDecisionModeSimulate
)

// DispatchDecisionDeps are read-only dependencies for dispatch/simulation decisions.
type DispatchDecisionDeps struct {
	EvaluatorRuntime  *NotificationRulesEvaluator
	EvaluatorSimulate *NotificationRulesEvaluator
	AlertConfigRepo   AlertConfigRepository
	RecipientResolver RecipientResolver
	MembershipQuerier MembershipEmailQuerier
	TaskAssigneeReader WorkflowTaskAssigneeReader
	StepReader        WorkflowStepReader
	TierEnforcement   *entitlement.Checker
}

// DispatchDecisionInput is the shared input for runtime and simulation paths.
type DispatchDecisionInput struct {
	CompanyID          string
	ScopeType          ScopeType
	ScopeID            string
	WorkflowInstanceID string
	DisclosureTypeID   string
	TemplateCode       string
	RecipientEmails    []string
	DueAt              time.Time
	ScheduledAt        time.Time
	AsOf               time.Time
	Channel            string
	EventType          string
	SystemContext      EvaluateSystemContext
	IdempotencyKey     string
	SubscriptionTier   string // actor tier for simulation; empty uses company resolver at runtime
}

// TraceStep is one decision trace entry (Batch 3A).
type TraceStep struct {
	Step     string         `json:"step"`
	Status   string         `json:"status"`
	Detail   string         `json:"detail"`
	Code     string         `json:"code,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// DispatchDecisionResult is the pure decision outcome (no SMTP, no DB writes).
type DispatchDecisionResult struct {
	Skip             bool
	SkipReason       string
	TemplateCode     string
	Recipients       []string
	RuleCode         string
	AllowInApp       bool
	EvalErr          error
	MatchedSchedules []ScheduleMatch
	Trace            []TraceStep
	Layer1Policies   []string
}

func dispatchDecisionInputFromCandidate(c DispatchCandidate, asOf time.Time) DispatchDecisionInput {
	eventType := "deadline"
	if c.ScopeType == ScopeTypeWorkflowStep {
		eventType = "workflow"
	}
	channel := "email"
	return DispatchDecisionInput{
		CompanyID:          c.CompanyID,
		ScopeType:          c.ScopeType,
		ScopeID:            c.ScopeID,
		WorkflowInstanceID: c.WorkflowInstanceID,
		DisclosureTypeID:   c.DisclosureTypeID,
		TemplateCode:       c.TemplateCode,
		RecipientEmails:    append([]string(nil), c.RecipientEmails...),
		DueAt:              dispatchDeadlineAt(c),
		ScheduledAt:        c.ScheduledAt,
		AsOf:               asOf,
		Channel:            channel,
		EventType:          eventType,
		SystemContext:      EvaluateContextDispatch,
		IdempotencyKey:     c.IdempotencyKey,
	}
}

func candidateFromDispatchDecisionInput(in DispatchDecisionInput) DispatchCandidate {
	return DispatchCandidate{
		CompanyID:          in.CompanyID,
		ScopeType:          in.ScopeType,
		ScopeID:            in.ScopeID,
		WorkflowInstanceID: in.WorkflowInstanceID,
		DisclosureTypeID:   in.DisclosureTypeID,
		TemplateCode:       in.TemplateCode,
		RecipientEmails:    append([]string(nil), in.RecipientEmails...),
		DeadlineAt:         in.DueAt,
		ScheduledAt:        in.ScheduledAt,
		IdempotencyKey:     in.IdempotencyKey,
	}
}

func appendTrace(trace []TraceStep, step, status, detail, code string) []TraceStep {
	return append(trace, TraceStep{Step: step, Status: status, Detail: detail, Code: code})
}

// evaluateDispatchDecision runs layer 1–3 decision logic without payload augmentation or side effects.
func evaluateDispatchDecision(ctx context.Context, deps DispatchDecisionDeps, in DispatchDecisionInput, mode DispatchDecisionMode) DispatchDecisionResult {
	out := DispatchDecisionResult{
		TemplateCode: in.TemplateCode,
		Recipients:   append([]string(nil), in.RecipientEmails...),
		AllowInApp:   true,
	}
	withTrace := mode == DispatchDecisionModeSimulate
	var trace []TraceStep

	if withTrace {
		trace = appendTrace(trace, "tenant_scope", "pass", "company scoped via authenticated context", "")
	}

	layer1On := mode == DispatchDecisionModeSimulate
	if mode == DispatchDecisionModeRuntime {
		layer1On = deps.EvaluatorRuntime != nil && deps.EvaluatorRuntime.ConsumerEnabled()
	}

	var layer1Policies []string
	if layer1On {
		eval := deps.EvaluatorSimulate
		if mode == DispatchDecisionModeRuntime {
			eval = deps.EvaluatorRuntime
		}
		if eval == nil {
			err := errEvaluatorNotConfigured()
			out.EvalErr = err
			if withTrace {
				trace = appendTrace(trace, "layer1_rule_load", "error", err.Error(), SkipReasonEvaluatorUnavailable)
				out.Trace = trace
			}
			return out
		}
		dec, err := eval.Evaluate(ctx, EvaluateInput{
			CompanyID:        in.CompanyID,
			EventType:        in.EventType,
			ScopeType:        in.ScopeType,
			DisclosureTypeID: in.DisclosureTypeID,
			DueAt:            in.DueAt,
			ScheduledAt:      in.ScheduledAt,
			AsOf:             in.AsOf,
			Channel:          in.Channel,
			SystemContext:    in.SystemContext,
			IdempotencyKey:   in.IdempotencyKey,
		})
		if err != nil {
			out.EvalErr = err
			if withTrace {
				trace = appendTrace(trace, "layer1_rule_load", "error", err.Error(), SkipReasonEvaluatorUnavailable)
				out.Trace = trace
			}
			return out
		}
		if rc, ok := dec.AuditMetadata["rule_code"].(string); ok {
			out.RuleCode = rc
		}
		out.MatchedSchedules = dec.MatchedSchedules
		layer1Policies = dec.RecipientPolicies
		out.Layer1Policies = layer1Policies

		if withTrace {
			if dec.Layer1Applied {
				trace = appendTrace(trace, "layer1_rule_load", "pass", "active notification rule loaded", "")
				trace = appendTrace(trace, "layer1_rule_status", "pass", "rule active", "")
				if dec.SkipReason == SkipReasonEventScopeExcluded {
					trace = appendTrace(trace, "layer1_event_scope", "fail", "event not in scope", dec.SkipReason)
				} else {
					trace = appendTrace(trace, "layer1_event_scope", "pass", in.EventType, "")
				}
				if dec.SkipReason == SkipReasonRuleScheduleNotMatched {
					trace = appendTrace(trace, "layer1_schedule_match", "fail", "no schedule matched", dec.SkipReason)
				} else if dec.Allowed || dec.SkipReason == "" {
					trace = appendTrace(trace, "layer1_schedule_match", "pass", "schedule matched", "")
				}
				if dec.SkipReason == SkipReasonChannelDisabled || dec.SkipReason == SkipReasonChannelNotImplemented {
					trace = appendTrace(trace, "layer1_channel_gate", "fail", "channel not enabled", dec.SkipReason)
				} else if dec.Allowed {
					trace = appendTrace(trace, "layer1_channel_gate", "pass", in.Channel, "")
				}
			} else {
				trace = appendTrace(trace, "layer1_rule_load", "fail", "no active rule row", SkipReasonRuleMissing)
			}
		}

		if !dec.Allowed {
			out.Skip = true
			out.SkipReason = dec.SkipReason
			if out.SkipReason == "" {
				out.SkipReason = SkipReasonRuleMissing
			}
			if withTrace {
				trace = appendTrace(trace, "decision", "fail", "WOULD_SKIP", out.SkipReason)
				out.Trace = trace
			}
			return out
		}

		if tierSkip := evaluateTierEnforcement(ctx, deps, in, dec.MatchedSchedules, withTrace); tierSkip != "" {
			out.Skip = true
			out.SkipReason = tierSkip
			if withTrace {
				trace = appendTrace(trace, "layer1_tier_gate", "fail", "subscription tier insufficient", tierSkip)
				trace = appendTrace(trace, "decision", "fail", "WOULD_SKIP", tierSkip)
				out.Trace = trace
			}
			return out
		}

		out.AllowInApp = emailChannelAllowed(dec)
	} else if withTrace {
		trace = appendTrace(trace, "layer1_rule_load", "skip", "runtime consumer disabled — layer 1 skipped", SkipReasonConsumerDisabled)
	}

	// Layer 2 — CMS template.
	if deps.AlertConfigRepo != nil && in.DisclosureTypeID != "" {
		alertKind := AlertKindDeadline
		if in.ScopeType == ScopeTypeWorkflowStep {
			alertKind = AlertKindWorkflowStep
		}
		cfg, err := deps.AlertConfigRepo.GetByTypeAndKind(ctx, in.DisclosureTypeID, alertKind)
		if err != nil {
			out.EvalErr = err
			if withTrace {
				trace = appendTrace(trace, "layer2_template_lookup", "error", err.Error(), "")
				out.Trace = trace
			}
			return out
		}
		if cfg != nil {
			if !cfg.Enabled {
				out.Skip = true
				out.SkipReason = SkipReasonTemplateDisabled
				if withTrace {
					trace = appendTrace(trace, "layer2_template_lookup", "pass", "template row found", "")
					trace = appendTrace(trace, "layer2_template_enabled", "fail", "template disabled", out.SkipReason)
					trace = appendTrace(trace, "decision", "fail", "WOULD_SKIP", out.SkipReason)
					out.Trace = trace
				}
				return out
			}
			if cfg.TemplateKey != "" {
				out.TemplateCode = cfg.TemplateKey
			}
			if withTrace {
				trace = appendTrace(trace, "layer2_template_lookup", "pass", out.TemplateCode, "")
				trace = appendTrace(trace, "layer2_template_enabled", "pass", "template enabled", "")
			}
		} else if withTrace {
			trace = appendTrace(trace, "layer2_template_lookup", "pass", "no CMS row — using fallback template code", "")
			trace = appendTrace(trace, "layer2_template_enabled", "pass", "backward compat fallthrough", "")
		}
	}

	candidate := candidateFromDispatchDecisionInput(in)

	// Layer 3 — resolver.
	if len(out.Recipients) == 0 && deps.RecipientResolver != nil && in.CompanyID != "" {
		var resolveErr error
		if in.ScopeType == ScopeTypeWorkflowStep {
			out.Recipients, resolveErr = deps.RecipientResolver.ResolveForWorkflowStep(ctx, in.CompanyID, in.WorkflowInstanceID, in.ScopeID)
		} else {
			out.Recipients, resolveErr = deps.RecipientResolver.ResolveForDeadline(ctx, in.CompanyID, in.ScopeID)
		}
		if resolveErr != nil {
			out.EvalErr = resolveErr
			if withTrace {
				trace = appendTrace(trace, "layer3_resolver", "error", resolveErr.Error(), "")
				out.Trace = trace
			}
			return out
		}
	}
	if len(out.Recipients) == 0 {
		out.Skip = true
		out.SkipReason = SkipReasonRecipientEmpty
		if withTrace {
			trace = appendTrace(trace, "layer3_resolver", "fail", "no recipients", out.SkipReason)
			trace = appendTrace(trace, "decision", "fail", "WOULD_SKIP", out.SkipReason)
			out.Trace = trace
		}
		return out
	}
	if withTrace {
		trace = appendTrace(trace, "layer3_resolver", "pass", "recipients resolved", "")
	}

	// Layer 3b — policies.
	if layer1On && len(layer1Policies) > 0 {
		filtered, err := applyRecipientPolicies(ctx, deps.MembershipQuerier, deps.TaskAssigneeReader, deps.StepReader, in.CompanyID, out.Recipients, layer1Policies, candidate)
		if err != nil {
			out.EvalErr = err
			if withTrace {
				trace = appendTrace(trace, "layer3_policy_filter", "error", err.Error(), "")
				out.Trace = trace
			}
			return out
		}
		if len(filtered) == 0 {
			out.Skip = true
			out.SkipReason = SkipReasonRecipientPolicyFilteredAll
			if withTrace {
				trace = appendTrace(trace, "layer3_policy_filter", "fail", "all recipients filtered", out.SkipReason)
				trace = appendTrace(trace, "decision", "fail", "WOULD_SKIP", out.SkipReason)
				out.Trace = trace
			}
			return out
		}
		out.Recipients = filtered
		if withTrace {
			trace = appendTrace(trace, "layer3_policy_filter", "pass", "policy filter applied", "")
		}
	} else if withTrace {
		trace = appendTrace(trace, "layer3_policy_filter", "pass", "no policies — pass-through", "")
	}

	if withTrace {
		trace = appendTrace(trace, "layer3_tenant_recipients", "pass", "recipients in company scope", "")
		trace = appendTrace(trace, "decision", "pass", "WOULD_SEND", "")
		out.Trace = trace
	}
	return out
}

func errEvaluatorNotConfigured() error {
	return fmt.Errorf("notification rules evaluator not configured")
}

func evaluateTierEnforcement(ctx context.Context, deps DispatchDecisionDeps, in DispatchDecisionInput, matched []ScheduleMatch, withTrace bool) string {
	if deps.TierEnforcement == nil || !deps.TierEnforcement.Enabled {
		return ""
	}
	premiumSchedules := entitlement.MatchedPremiumSchedule(entitlementSchedulesFromMatches(matched))
	premiumChannels := premiumChannelEnabled(in.Channel)
	checker := *deps.TierEnforcement
	if strings.TrimSpace(in.SubscriptionTier) != "" {
		// Simulation path: use actor tier.
		checker.ResolveCompanyTier = func(_ context.Context, _ string) string {
			return in.SubscriptionTier
		}
	}
	res := checker.CheckRuntimePremium(ctx, in.CompanyID, premiumSchedules, premiumChannels)
	if res.Allowed {
		return ""
	}
	if res.ReasonCode == entitlement.ReasonSubscriptionTierUnknown {
		return SkipReasonSubscriptionTierUnknown
	}
	return SkipReasonSubscriptionTierDenied
}

func entitlementSchedulesFromMatches(matched []ScheduleMatch) []entitlement.ScheduleMatch {
	out := make([]entitlement.ScheduleMatch, 0, len(matched))
	for _, m := range matched {
		out = append(out, entitlement.ScheduleMatch{PremiumOnly: m.PremiumOnly})
	}
	return out
}

func premiumChannelEnabled(channel string) bool {
	switch strings.TrimSpace(strings.ToLower(channel)) {
	case "zalo", "sms":
		return true
	default:
		return false
	}
}
