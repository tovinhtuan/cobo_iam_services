package app

import (
	"time"

	"github.com/cobo/cobo_iam_services/internal/companyaccess/healthscore"
	"github.com/cobo/cobo_iam_services/internal/companyaccess/recommendation"
)

// RuleRecommendationsView is returned by GET /api/v1/admin/rule-recommendations.
type RuleRecommendationsView struct {
	CompanyID   string                `json:"company_id"`
	Source      string                `json:"source"`
	GeneratedAt time.Time             `json:"generated_at"`
	Items       []recommendation.Item `json:"items"`
	Score       *healthscore.Result   `json:"score,omitempty"`
}

type GetRuleRecommendationsRequest struct {
	Subject AdminSubject
}
