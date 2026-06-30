package app

import (
	"context"
	"time"

	"github.com/cobo/cobo_iam_services/internal/companyaccess/conflict"
	"github.com/cobo/cobo_iam_services/internal/companyaccess/validation"
)

func init() {
	validation.RegisterValidators(validation.ValidatorDeps{
		ValidatePrefs:          ValidateAlertChannelPrefsPayload,
		ValidateDepartmentName: validateDepartmentName,
	})
	validation.WireConflictValidators(PermissionRiskLevel)
}

// ValidateConfiguration runs the read-only validation pipeline for the tenant (Sprint 4 Batch 2).
func (s *adminService) ValidateConfiguration(ctx context.Context, req ValidateConfigurationRequest) (*validation.Result, error) {
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
	conflictOut := conflict.DefaultEngine().Evaluate(conflict.EvaluationInput{
		CompanyID:   companyID,
		EvaluatedAt: evalAt,
	}, snapshot)

	depts, err := s.repo.ListCompanyDepartments(ctx, companyID)
	if err != nil {
		return nil, err
	}
	deptRows := make([]validation.DepartmentRow, 0, len(depts))
	for _, d := range depts {
		deptRows = append(deptRows, validation.DepartmentRow{DepartmentID: d.DepartmentID, Name: d.Name})
	}

	adminCount, err := s.repo.CountAdminsInCompany(ctx, companyID)
	if err != nil {
		return nil, err
	}

	canonicalCount := 0
	rules, err := s.repo.ListNotificationRules(ctx, companyID)
	if err != nil {
		return nil, err
	}
	for _, rule := range rules {
		if rule.RuleCode == AlertChannelPrefsRuleCode {
			canonicalCount++
		}
	}

	result := validation.Run(validation.Input{
		CompanyID:                    companyID,
		ValidatedAt:                  evalAt,
		Snapshot:                     snapshot,
		ConflictOutput:               conflictOut,
		Departments:                  deptRows,
		CompanyAdminCount:            adminCount,
		CanonicalAlertPrefsRuleCount: canonicalCount,
		RuntimeConsumerEnabled:       s.notificationRulesConsumerEnabled,
		SubscriptionTierEnforced:     s.subscriptionTierEnforcementEnabled,
	})
	if len(req.Suites) > 0 {
		result = validation.FilterSuites(result, req.Suites)
	}
	return &result, nil
}
