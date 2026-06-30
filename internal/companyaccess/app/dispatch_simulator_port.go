package app

import (
	"context"
	"time"
)

// NotificationDispatchSimulator runs read-only dispatch simulation (Batch 3B).
// Implemented by reminder/app via httpserver adapter.
type NotificationDispatchSimulator interface {
	SimulateDispatch(ctx context.Context, in NotificationDispatchSimulateInput) (*NotificationDispatchSimulateResult, error)
}

// NotificationDispatchSimulateInput is the reminder-layer simulation request.
type NotificationDispatchSimulateInput struct {
	CompanyID          string
	EventType          string
	Channel            string
	DueAt              time.Time
	ScheduledAt        time.Time
	AsOf               time.Time
	ScopeType          string
	ScopeID            string
	DisclosureTypeID   string
	WorkflowInstanceID string
	RecipientEmails    []string
	TemplateCode       string
	SimulationID       string
	SubscriptionTier   string
}

// NotificationDispatchTraceStep is one decision trace entry.
type NotificationDispatchTraceStep struct {
	Step     string         `json:"step"`
	Status   string         `json:"status"`
	Detail   string         `json:"detail"`
	Code     string         `json:"code,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// NotificationDispatchScheduleMatch mirrors evaluator schedule match output.
type NotificationDispatchScheduleMatch struct {
	OffsetDays  *int   `json:"offset_days"`
	Kind        string `json:"kind"`
	PremiumOnly bool   `json:"premium_only"`
}

// NotificationDispatchRecipientSummary is privacy-safe recipient block.
type NotificationDispatchRecipientSummary struct {
	Count         int      `json:"count"`
	MaskedSamples []string `json:"masked_samples"`
	PolicyApplied bool     `json:"policy_applied"`
}

// NotificationDispatchSimulateResult is the simulation API response.
type NotificationDispatchSimulateResult struct {
	SimulationID     string                                `json:"simulation_id"`
	WouldSend        bool                                  `json:"would_send"`
	WouldSkip        bool                                  `json:"would_skip"`
	Outcome          string                                `json:"outcome"`
	ReasonCode       string                                `json:"reason_code"`
	Channel          string                                `json:"channel"`
	DispatchPath     string                                `json:"dispatch_path"`
	MatchedRuleCode  string                                `json:"matched_rule_code"`
	MatchedSchedules []NotificationDispatchScheduleMatch   `json:"matched_schedules"`
	TemplateKey      string                                `json:"template_key"`
	RecipientSummary NotificationDispatchRecipientSummary  `json:"recipient_summary"`
	Trace            []NotificationDispatchTraceStep       `json:"trace"`
	Warnings         []string                              `json:"warnings"`
	EvaluatedAt      time.Time                             `json:"evaluated_at"`
}
