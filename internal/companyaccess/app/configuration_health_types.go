package app

import (
	"time"

	"github.com/cobo/cobo_iam_services/internal/companyaccess/healthscore"
)

// HealthCheckItem is one actionable configuration-health check (ADR-014).
type HealthCheckItem struct {
	Code        string         `json:"code"`
	Severity    string         `json:"severity"`
	Domain      string         `json:"domain"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	ActionLink  string         `json:"action_link"`
	Evidence    map[string]any `json:"evidence"`
}

// ConfigurationHealthView is returned by GET /api/v1/admin/configuration-health.
type ConfigurationHealthView struct {
	OverallStatus string             `json:"overall_status"`
	Checks        []HealthCheckItem  `json:"checks"`
	EvaluatedAt   time.Time          `json:"evaluated_at"`
	Score         *healthscore.Result `json:"score,omitempty"`
}

type GetConfigurationHealthRequest struct {
	Subject AdminSubject
}
