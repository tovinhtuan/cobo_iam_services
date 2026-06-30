package app

import (
	"context"
	"net/http"
	"time"

	"github.com/cobo/cobo_iam_services/internal/companyaccess/conflict"
	"github.com/cobo/cobo_iam_services/internal/companyaccess/healthscore"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func init() {
	conflict.RegisterValidators(conflict.ValidatorDeps{
		ValidatePrefs:         ValidateAlertChannelPrefsPayload,
		PermissionRiskLevel:   PermissionRiskLevel,
		IsGrantablePermission: IsGrantablePermission,
	})
}

func (s *adminService) GetConfigurationHealth(ctx context.Context, req GetConfigurationHealthRequest) (*ConfigurationHealthView, error) {
	if err := s.authorizeConfigurationHealth(ctx, req.Subject); err != nil {
		return nil, err
	}
	companyID := req.Subject.CompanyID
	if companyID == "" {
		return nil, perrNewBadRequest("company context required")
	}
	loader := conflict.SnapshotLoader{
		Reader:                   s.conflictReader,
		CompanyTierLookup:        s.companyTierLookup,
		SubscriptionTierEnforced: s.subscriptionTierEnforcementEnabled,
	}
	snapshot, err := loader.Load(ctx, companyID)
	if err != nil {
		return nil, err
	}
	evalAt := snapshot.EvaluatedAt
	if evalAt.IsZero() {
		evalAt = time.Now().UTC()
	}
	out := conflict.DefaultEngine().Evaluate(conflict.EvaluationInput{
		CompanyID:   companyID,
		EvaluatedAt: evalAt,
	}, snapshot)

	checks := make([]HealthCheckItem, 0, len(out.Results)+2)
	for _, r := range out.Results {
		checks = append(checks, conflictResultToHealthCheck(r))
	}
	checks = append(checks, s.basicNotificationHealthChecks(snapshot)...)

	scoreInput := make([]healthscore.Check, len(checks))
	for i, c := range checks {
		scoreInput[i] = healthscore.Check{Code: c.Code, Severity: c.Severity}
	}
	computed := healthscore.Compute(scoreInput, evalAt)

	return &ConfigurationHealthView{
		OverallStatus: deriveOverallStatus(checks),
		Checks:        checks,
		EvaluatedAt:   evalAt,
		Score:         &computed,
	}, nil
}

func (s *adminService) authorizeConfigurationHealth(ctx context.Context, sub AdminSubject) error {
	canRbac, err := s.hasPermission(ctx, sub, "rbac.manage")
	if err != nil {
		return err
	}
	if canRbac {
		return nil
	}
	canSettings, err := s.hasPermission(ctx, sub, "system.settings")
	if err != nil {
		return err
	}
	if canSettings {
		return nil
	}
	return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "access denied", nil)
}

func conflictResultToHealthCheck(r conflict.Result) HealthCheckItem {
	evidence := r.Evidence
	if evidence == nil {
		evidence = map[string]any{}
	}
	return HealthCheckItem{
		Code:        r.Code,
		Severity:    r.Severity,
		Domain:      r.Domain,
		Title:       r.Title,
		Description: r.Description,
		ActionLink:  r.ActionLink,
		Evidence:    evidence,
	}
}

func (s *adminService) basicNotificationHealthChecks(snapshot *conflict.ConfigurationSnapshot) []HealthCheckItem {
	if snapshot == nil {
		return nil
	}
	var checks []HealthCheckItem
	if !snapshot.AlertChannelPrefsExists {
		checks = append(checks, HealthCheckItem{
			Code:        "notification.storage_not_configured",
			Severity:    "info",
			Domain:      "notification",
			Title:       "Chưa cấu hình kênh cảnh báo",
			Description: "Chưa có bản ghi notification_rules cho alert channel prefs.",
			ActionLink:  "/app/admin?tab=notifications",
			Evidence:    map[string]any{"rule_code": AlertChannelPrefsRuleCode},
		})
	}
	return checks
}

func deriveOverallStatus(checks []HealthCheckItem) string {
	hasBlocking := false
	hasWarning := false
	for _, c := range checks {
		switch c.Severity {
		case conflict.SeverityBlocking:
			hasBlocking = true
		case conflict.SeverityWarning:
			hasWarning = true
		}
	}
	if hasBlocking {
		return "critical"
	}
	if hasWarning {
		return "warning"
	}
	return "ok"
}

func perrNewBadRequest(msg string) error {
	return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, msg, nil)
}
