package conflict

import (
	"context"
	"time"
)

// SnapshotReader loads persisted configuration for conflict evaluation (read-only).
type SnapshotReader interface {
	GetNotificationRuleByCode(ctx context.Context, companyID, ruleCode string) (*NotificationRuleSnapshot, error)
	ListRoles(ctx context.Context, companyID string) ([]RoleSnapshot, error)
	ListRolePermissionCodes(ctx context.Context, companyID, roleID string) ([]string, error)
	ListStaleWorkflowOverridesByCompany(ctx context.Context, companyID string) ([]StaleWorkflowOverrideRow, error)
	ListWorkflowAssigneeRulesByCompany(ctx context.Context, companyID string) ([]WorkflowAssigneeRuleRow, error)
	ListInactiveDepartmentsWithMembers(ctx context.Context, companyID string) ([]InactiveDepartmentRow, error)
	ListActiveDirectPermissionsByCompany(ctx context.Context, companyID string) ([]DirectPermissionRow, error)
}

// SnapshotLoader loads a company-scoped configuration snapshot (SELECT only).
type SnapshotLoader struct {
	Reader                   SnapshotReader
	CompanyTierLookup        func(ctx context.Context, companyID string) string
	SubscriptionTierEnforced bool
}

// Load builds a read-only snapshot for the given company.
func (l *SnapshotLoader) Load(ctx context.Context, companyID string) (*ConfigurationSnapshot, error) {
	now := time.Now().UTC()
	snap := &ConfigurationSnapshot{
		CompanyID:                companyID,
		EvaluatedAt:              now,
		RolePermissionCodes:      map[string][]string{},
		SubscriptionTierEnforced: l.SubscriptionTierEnforced,
	}
	if l.Reader == nil {
		return snap, nil
	}
	rule, err := l.Reader.GetNotificationRuleByCode(ctx, companyID, AlertChannelPrefsRuleCode)
	if err != nil {
		return nil, err
	}
	if rule != nil {
		snap.AlertChannelPrefsExists = true
		snap.AlertChannelPrefsRuleID = rule.RuleID
		snap.AlertChannelPrefsPayload = rule.Payload
	}
	roles, err := l.Reader.ListRoles(ctx, companyID)
	if err != nil {
		return nil, err
	}
	snap.Roles = roles
	for _, r := range roles {
		codes, err := l.Reader.ListRolePermissionCodes(ctx, companyID, r.RoleID)
		if err != nil {
			return nil, err
		}
		snap.RolePermissionCodes[r.RoleID] = codes
	}
	if rows, err := l.Reader.ListStaleWorkflowOverridesByCompany(ctx, companyID); err != nil {
		return nil, err
	} else {
		snap.StaleWorkflowOverrides = rows
	}
	if rows, err := l.Reader.ListWorkflowAssigneeRulesByCompany(ctx, companyID); err != nil {
		return nil, err
	} else {
		snap.WorkflowAssigneeRules = rows
	}
	if rows, err := l.Reader.ListInactiveDepartmentsWithMembers(ctx, companyID); err != nil {
		return nil, err
	} else {
		snap.InactiveDepartmentsReferenced = rows
	}
	if rows, err := l.Reader.ListActiveDirectPermissionsByCompany(ctx, companyID); err != nil {
		return nil, err
	} else {
		snap.NonGrantableDirectPermissions = rows
	}
	if l.CompanyTierLookup != nil {
		snap.CompanySubscriptionTier = l.CompanyTierLookup(ctx, companyID)
	}
	return snap, nil
}
