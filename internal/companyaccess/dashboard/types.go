package dashboard

import "time"

// Widget status values (Batch 4 / ADR-050).
const (
	StatusOK         = "ok"
	StatusWarning    = "warning"
	StatusAttention  = "attention"
	StatusUnknown    = "unknown"
	AvailabilityOK   = "available"
	AvailabilityNA   = "not_available"
)

// Widget keys (locked set for Batch 4).
const (
	WidgetConfigurationHealth = "configuration_health"
	WidgetValidationSummary   = "validation_summary"
	WidgetNotificationRuntime = "notification_runtime"
	WidgetSubscriptionTier    = "subscription_tier"
	WidgetAuditTimeline       = "audit_timeline"
	WidgetDependencySummary   = "dependency_summary"
)

// Metric is one scalar in a widget card.
type Metric struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Value any    `json:"value"`
}

// Widget is one operational dashboard card.
type Widget struct {
	Key          string   `json:"key"`
	Status       string   `json:"status"`
	Title        string   `json:"title"`
	Summary      string   `json:"summary"`
	Metrics      []Metric `json:"metrics,omitempty"`
	ActionLink   string   `json:"action_link,omitempty"`
	Availability string   `json:"availability,omitempty"`
}

// Result is returned by GET /api/v1/admin/operational-dashboard.
type Result struct {
	OverallStatus string    `json:"overall_status"`
	CompanyID     string    `json:"company_id"`
	Widgets       []Widget  `json:"widgets"`
	EvaluatedAt   time.Time `json:"evaluated_at"`
}
