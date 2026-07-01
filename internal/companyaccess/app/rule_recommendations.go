package app

import (
	"context"

	"github.com/cobo/cobo_iam_services/internal/companyaccess/recommendation"
)

// GetRuleRecommendations returns read-only guidance from configuration-health checks only.
func (s *adminService) GetRuleRecommendations(ctx context.Context, req GetRuleRecommendationsRequest) (*RuleRecommendationsView, error) {
	health, err := s.GetConfigurationHealth(ctx, GetConfigurationHealthRequest{Subject: req.Subject})
	if err != nil {
		return nil, err
	}
	inputs := make([]recommendation.CheckInput, len(health.Checks))
	for i, c := range health.Checks {
		inputs[i] = recommendation.CheckInput{
			Code:        c.Code,
			Severity:    c.Severity,
			Domain:      c.Domain,
			Title:       c.Title,
			Description: c.Description,
			ActionLink:  c.ActionLink,
			Evidence:    c.Evidence,
		}
	}
	items := recommendation.Format(inputs, health.EvaluatedAt)
	return &RuleRecommendationsView{
		CompanyID:   req.Subject.CompanyID,
		Source:      recommendation.SourceConfigurationHealth,
		GeneratedAt: health.EvaluatedAt,
		Items:       items,
		Score:       health.Score,
	}, nil
}
