package dependency

import (
	"context"
	"strings"
	"time"

	"github.com/cobo/cobo_iam_services/internal/companyaccess/conflict"
)

// Provider builds reverse dependency lookup from snapshot + reader (read-only).
type Provider struct {
	Loader conflict.SnapshotLoader
	Reader Reader
}

// Resolve returns dependencies for one department or role (ADR-043 MVP).
func (p *Provider) Resolve(ctx context.Context, q Query) (*Result, error) {
	if p == nil || p.Reader == nil {
		return emptyResult(q), nil
	}
	limit := q.SampleLimit
	if limit <= 0 {
		limit = DefaultSampleLimit
	}
	if limit > MaxSampleLimit {
		limit = MaxSampleLimit
	}
	evalAt := q.EvaluatedAt
	if evalAt.IsZero() {
		evalAt = time.Now().UTC()
	}

	objectType := strings.TrimSpace(strings.ToLower(q.ObjectType))
	objectID := strings.TrimSpace(q.ObjectID)
	companyID := strings.TrimSpace(q.CompanyID)

	switch objectType {
	case ObjectTypeDepartment:
		ok, err := p.Reader.DepartmentBelongsToCompany(ctx, companyID, objectID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrObjectNotFound
		}
	case ObjectTypeRole:
		ok, err := p.Reader.RoleAccessibleByCompany(ctx, companyID, objectID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrObjectNotFound
		}
	default:
		return nil, ErrInvalidObjectType
	}

	snapshot, err := p.Loader.Load(ctx, companyID)
	if err != nil {
		return nil, err
	}

	var groups []Group
	truncated := false
	switch objectType {
	case ObjectTypeDepartment:
		groups, truncated = p.departmentDependencies(ctx, companyID, objectID, snapshot, limit, q.IncludeCounts)
	case ObjectTypeRole:
		groups, truncated = p.roleDependencies(ctx, companyID, objectID, snapshot, limit, q.IncludeCounts)
	}

	total := 0
	for _, g := range groups {
		total += g.Count
	}
	return &Result{
		ObjectType:      objectType,
		ObjectID:        objectID,
		CompanyID:       companyID,
		Dependencies:    groups,
		TotalReferences: total,
		Truncated:       truncated,
		Source:          Source,
		EvaluatedAt:     evalAt,
	}, nil
}

func (p *Provider) departmentDependencies(ctx context.Context, companyID, deptID string, snap *conflict.ConfigurationSnapshot, limit int, includeCounts bool) ([]Group, bool) {
	var groups []Group
	truncated := false

	if includeCounts {
		samples, count, err := p.Reader.ListDepartmentMemberSamples(ctx, companyID, deptID, limit)
		if err == nil && count > 0 {
			groups = append(groups, Group{
				Domain:   "membership",
				Relation: RelationAssignedMember,
				Count:    count,
				Samples:  samples,
			})
		}
	}

	assigneeSamples, assigneeCount := scanAssigneeRulesForDepartment(snap, deptID, limit)
	if assigneeCount > 0 {
		groups = append(groups, Group{
			Domain:   "workflow",
			Relation: RelationAssigneeRuleReference,
			Count:    assigneeCount,
			Samples:  assigneeSamples,
		})
	}

	stepSamples, stepCount, stepTrunc, err := p.Reader.ListWorkflowOverrideStepsForDepartment(ctx, companyID, deptID, MaxOverrideScan)
	if err == nil && stepCount > 0 {
		groups = append(groups, Group{
			Domain:   "workflow",
			Relation: RelationWorkflowStepDepartment,
			Count:    stepCount,
			Samples:  stepSamples,
		})
		if stepTrunc {
			truncated = true
		}
	}

	if prefsSamples, prefsCount := scanNotificationPrefsForDepartment(snap, deptID); prefsCount > 0 {
		groups = append(groups, Group{
			Domain:   "notification",
			Relation: RelationNotificationRecipient,
			Count:    prefsCount,
			Samples:  prefsSamples,
		})
	}

	if groups == nil {
		groups = []Group{}
	}
	return groups, truncated
}

func (p *Provider) roleDependencies(ctx context.Context, companyID, roleID string, snap *conflict.ConfigurationSnapshot, limit int, includeCounts bool) ([]Group, bool) {
	var groups []Group

	if includeCounts {
		samples, count, err := p.Reader.ListRoleMembershipSamples(ctx, companyID, roleID, limit)
		if err == nil && count > 0 {
			groups = append(groups, Group{
				Domain:   "membership",
				Relation: RelationAssignedMembershipRole,
				Count:    count,
				Samples:  samples,
			})
		}
		permSamples, permCount, err := p.Reader.ListRolePermissionSamples(ctx, companyID, roleID, limit)
		if err == nil && permCount > 0 {
			groups = append(groups, Group{
				Domain:   "rbac",
				Relation: RelationDirectPermission,
				Count:    permCount,
				Samples:  permSamples,
			})
		}
	}

	assigneeSamples, assigneeCount := scanAssigneeRulesForRole(snap, roleID, limit)
	if assigneeCount > 0 {
		groups = append(groups, Group{
			Domain:   "workflow",
			Relation: RelationAssigneeRuleReference,
			Count:    assigneeCount,
			Samples:  assigneeSamples,
		})
	}

	if groups == nil {
		groups = []Group{}
	}
	return groups, false
}

func scanAssigneeRulesForDepartment(snap *conflict.ConfigurationSnapshot, deptID string, limit int) ([]Sample, int) {
	if snap == nil {
		return nil, 0
	}
	var samples []Sample
	count := 0
	for _, rule := range snap.WorkflowAssigneeRules {
		if !payloadReferencesDepartment(rule.Payload, deptID) {
			continue
		}
		count++
		if len(samples) < limit {
			samples = append(samples, Sample{
				"rule_id":           rule.RuleID,
				"rule_code":         rule.RuleCode,
				"disclosure_type_id": payloadString(rule.Payload, "disclosure_type_id"),
			})
		}
	}
	return samples, count
}

func scanAssigneeRulesForRole(snap *conflict.ConfigurationSnapshot, roleID string, limit int) ([]Sample, int) {
	if snap == nil {
		return nil, 0
	}
	var samples []Sample
	count := 0
	for _, rule := range snap.WorkflowAssigneeRules {
		matched := false
		for _, id := range extractRoleIDs(rule.Payload) {
			if id == roleID {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		count++
		if len(samples) < limit {
			samples = append(samples, Sample{
				"rule_id":   rule.RuleID,
				"rule_code": rule.RuleCode,
			})
		}
	}
	return samples, count
}

func scanNotificationPrefsForDepartment(snap *conflict.ConfigurationSnapshot, deptID string) ([]Sample, int) {
	if snap == nil || !snap.AlertChannelPrefsExists {
		return nil, 0
	}
	policies, ok := snap.AlertChannelPrefsPayload["recipient_policies"].([]any)
	if !ok {
		return nil, 0
	}
	for _, item := range policies {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if id, _ := m["department_id"].(string); strings.TrimSpace(id) == deptID {
			return []Sample{{
				"rule_id":        snap.AlertChannelPrefsRuleID,
				"department_id":  deptID,
				"recipient_policy": m,
			}}, 1
		}
	}
	return nil, 0
}

func payloadReferencesDepartment(payload map[string]any, deptID string) bool {
	if payload == nil {
		return false
	}
	if id, _ := payload["department_id"].(string); strings.TrimSpace(id) == deptID {
		return true
	}
	if ids, ok := payload["department_ids"].([]any); ok {
		for _, item := range ids {
			if s, ok := item.(string); ok && strings.TrimSpace(s) == deptID {
				return true
			}
		}
	}
	return false
}

func extractRoleIDs(payload map[string]any) []string {
	if payload == nil {
		return nil
	}
	var ids []string
	if s, ok := payload["role_id"].(string); ok && s != "" {
		ids = append(ids, s)
	}
	if s, ok := payload["assignee_role_id"].(string); ok && s != "" {
		ids = append(ids, s)
	}
	if arr, ok := payload["assignee_role_ids"].([]any); ok {
		for _, item := range arr {
			if s, ok := item.(string); ok && s != "" {
				ids = append(ids, s)
			}
		}
	}
	return ids
}

func payloadString(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	if s, ok := payload[key].(string); ok {
		return s
	}
	return ""
}

func emptyResult(q Query) *Result {
	evalAt := q.EvaluatedAt
	if evalAt.IsZero() {
		evalAt = time.Now().UTC()
	}
	return &Result{
		ObjectType:      q.ObjectType,
		ObjectID:        q.ObjectID,
		CompanyID:       q.CompanyID,
		Dependencies:    []Group{},
		TotalReferences: 0,
		Truncated:       false,
		Source:          Source,
		EvaluatedAt:     evalAt,
	}
}
