package app

import (
	"context"
	"time"
)

// AlertChannelPrefsRuleCode is the canonical notification_rules.rule_code (ADR-027 layer 1).
const AlertChannelPrefsRuleCode = "company.alert_channel_prefs.v1"

// NotificationRulesReader loads enterprise alert channel prefs from storage (read-only).
type NotificationRulesReader interface {
	GetCompanyAlertPrefs(ctx context.Context, companyID string) (*AlertChannelPrefsDocument, error)
}

// ChannelPref is one channel entry in alert channel prefs.
type ChannelPref struct {
	Enabled bool
}

// SchedulePref is one schedule entry in alert channel prefs.
type SchedulePref struct {
	OffsetDays  *int
	Kind        string
	Enabled     bool
	PremiumOnly bool
}

// AlertChannelPrefsDocument is the parsed layer-1 prefs document.
type AlertChannelPrefsDocument struct {
	RuleCode           string
	CompanyID          string
	Status             string
	Version            int
	EventScope         []string
	Channels           map[string]ChannelPref
	Schedules          []SchedulePref
	RecipientPolicies  []string
	UpdatedBy          string
	UpdatedAt          string
	RawPayload         map[string]any
}

// EvaluateSystemContext distinguishes dry evaluation modes (Batch 1+).
type EvaluateSystemContext string

const (
	EvaluateContextDispatch EvaluateSystemContext = "dispatch"
	EvaluateContextSimulate EvaluateSystemContext = "simulate"
	EvaluateContextShadow   EvaluateSystemContext = "shadow"
)

// EvaluateInput is the contract input for notification rules evaluation.
type EvaluateInput struct {
	CompanyID        string
	EventType        string
	ScopeType        ScopeType
	DisclosureTypeID string
	DueAt            time.Time
	ScheduledAt      time.Time
	AsOf             time.Time
	Channel          string
	SubscriptionTier string
	SystemContext    EvaluateSystemContext
	IdempotencyKey   string
}

// ScheduleMatch describes a matched schedule entry (Batch 2 schedule logic expands).
type ScheduleMatch struct {
	OffsetDays  *int
	Kind        string
	PremiumOnly bool
}

// EvaluateDecision is the contract output for evaluation (no side effects).
type EvaluateDecision struct {
	Layer1Applied     bool
	Allowed           bool
	SkipReason        string
	DisabledChannels  []string
	ActiveChannels    []string
	MatchedSchedules  []ScheduleMatch
	RecipientPolicies []string
	TemplateReference string
	Priority          int
	IdempotencyKey    string
	AuditMetadata     map[string]any
}

// Skip reason codes (stable — Batch 2A contract, UPPER_SNAKE).
const (
	SkipReasonConsumerDisabled      = "consumer_disabled" // internal — flag OFF path does not emit
	SkipReasonRuleMissing           = "RULE_MISSING"
	SkipReasonRuleDisabled          = "RULE_DISABLED"
	SkipReasonRuleScheduleNotMatched = "RULE_SCHEDULE_NOT_MATCHED"
	SkipReasonChannelDisabled       = "CHANNEL_DISABLED"
	SkipReasonChannelNotImplemented = "channel_not_implemented"
	SkipReasonEventScopeExcluded    = "event_scope_excluded"
	SkipReasonInvalidPrefs          = "MALFORMED_RULE"
	SkipReasonCompanyMismatch       = "RECIPIENT_TENANT_MISMATCH"
	SkipReasonRecipientPolicyFilteredAll = "RECIPIENT_POLICY_FILTERED_ALL"
	SkipReasonRecipientEmpty        = "RECIPIENT_EMPTY"
	SkipReasonTemplateDisabled      = "TEMPLATE_DISABLED"
	SkipReasonTemplateMissing       = "TEMPLATE_MISSING"
	SkipReasonEvaluatorUnavailable  = "EVALUATOR_UNAVAILABLE"
	SkipReasonSubscriptionTierDenied = "SUBSCRIPTION_TIER_DENIED"
	SkipReasonSubscriptionTierUnknown  = "SUBSCRIPTION_TIER_UNKNOWN"
)

// NotificationRulesEvaluator evaluates prefs without dispatch side effects.
type NotificationRulesEvaluator struct {
	reader          NotificationRulesReader
	consumerEnabled bool
}

// NewNotificationRulesEvaluator wires a reader with the consumer feature flag.
func NewNotificationRulesEvaluator(reader NotificationRulesReader, consumerEnabled bool) *NotificationRulesEvaluator {
	return &NotificationRulesEvaluator{
		reader:          reader,
		consumerEnabled: consumerEnabled,
	}
}

// ConsumerEnabled reports whether layer-1 evaluation is active.
func (e *NotificationRulesEvaluator) ConsumerEnabled() bool {
	return e != nil && e.consumerEnabled
}
