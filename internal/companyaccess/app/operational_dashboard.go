package app

import (
	"context"
	"time"

	"github.com/cobo/cobo_iam_services/internal/companyaccess/dashboard"
)

// GetOperationalDashboard aggregates read-only operational signals (ADR-050).
func (s *adminService) GetOperationalDashboard(ctx context.Context, req GetOperationalDashboardRequest) (*dashboard.Result, error) {
	if err := s.authorizeConfigurationHealth(ctx, req.Subject); err != nil {
		return nil, err
	}
	companyID := req.Subject.CompanyID
	if companyID == "" {
		return nil, perrNewBadRequest("company context required")
	}
	evalAt := time.Now().UTC()

	health, healthErr := s.GetConfigurationHealth(ctx, GetConfigurationHealthRequest{Subject: req.Subject})
	validation, valErr := s.ValidateConfiguration(ctx, ValidateConfigurationRequest{Subject: req.Subject})
	notification, notifErr := s.GetNotificationRuleStatus(ctx, GetNotificationRuleStatusRequest{Subject: req.Subject})

	var auditIn dashboard.AuditTimelineInput
	if timelineView, timelineErr := s.ListChangeTimeline(ctx, ListChangeTimelineRequest{
		Subject: req.Subject,
		Limit:   5,
	}); timelineErr != nil {
		auditIn = dashboard.AuditTimelineInput{Err: timelineErr}
	} else if timelineView != nil {
		auditIn.EventCount = len(timelineView.Items)
		if len(timelineView.Items) > 0 {
			auditIn.LastSummary = timelineView.Items[0].Summary
		}
	}

	result := dashboard.Build(dashboard.AggregateInput{
		CompanyID:   companyID,
		EvaluatedAt: evalAt,
		Health:      dashboard.HealthInput{View: mapHealthView(health), Err: healthErr},
		Validation:  dashboard.ValidationInput{View: validation, Err: valErr},
		Notification: dashboard.NotificationInput{
			View: mapNotificationView(notification),
			Err:  notifErr,
		},
		AuditTimeline: auditIn,
	})
	return &result, nil
}

func mapHealthView(v *ConfigurationHealthView) *dashboard.HealthView {
	if v == nil {
		return nil
	}
	checks := make([]dashboard.HealthCheck, 0, len(v.Checks))
	for _, c := range v.Checks {
		checks = append(checks, dashboard.HealthCheck{Severity: c.Severity})
	}
	var scoreValue *int
	scoreStatus := ""
	if v.Score != nil {
		val := v.Score.Value
		scoreValue = &val
		scoreStatus = v.Score.Status
	}
	return &dashboard.HealthView{
		OverallStatus: v.OverallStatus,
		Checks:        checks,
		ScoreValue:    scoreValue,
		ScoreStatus:   scoreStatus,
	}
}

func mapNotificationView(v *NotificationRuleStatusView) *dashboard.NotificationView {
	if v == nil {
		return nil
	}
	return &dashboard.NotificationView{
		StorageConfigured:          v.StorageConfigured,
		PayloadValid:               v.PayloadValid,
		RuntimeConsumerEnabled:     v.RuntimeConsumerEnabled,
		SimulationAvailable:        v.SimulationAvailable,
		DispatchEnforcementEnabled: v.DispatchEnforcementEnabled,
		SubscriptionTierEnforced:   v.SubscriptionTierEnforced,
		UIState:                    v.UIState,
		Warnings:                   v.Warnings,
	}
}
