package inmemory

import (
	"context"
	"strings"

	"github.com/cobo/cobo_iam_services/internal/companyaccess/conflict"
)

func (r *AdminRepository) ListStaleWorkflowOverridesByCompany(_ context.Context, _ string) ([]conflict.StaleWorkflowOverrideRow, error) {
	return nil, nil
}

func (r *AdminRepository) ListWorkflowAssigneeRulesByCompany(_ context.Context, companyID string) ([]conflict.WorkflowAssigneeRuleRow, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	companyID = strings.TrimSpace(companyID)
	var out []conflict.WorkflowAssigneeRuleRow
	for i, rule := range r.workflowAssigneeRules {
		cid, _ := rule["company_id"].(string)
		if cid != companyID {
			continue
		}
		id, _ := rule["workflow_assignee_rule_id"].(string)
		if id == "" {
			id = rule["rule_id"].(string)
		}
		if id == "" {
			id = strings.TrimSpace(rule["rule_code"].(string))
			if id == "" {
				id = "war_" + string(rune('a'+i))
			}
		}
		code, _ := rule["rule_code"].(string)
		payload := cloneAnyMap(rule)
		out = append(out, conflict.WorkflowAssigneeRuleRow{
			RuleID:   id,
			RuleCode: code,
			Payload:  payload,
		})
	}
	return out, nil
}

func (r *AdminRepository) ListInactiveDepartmentsWithMembers(_ context.Context, companyID string) ([]conflict.InactiveDepartmentRow, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	companyID = strings.TrimSpace(companyID)
	var out []conflict.InactiveDepartmentRow
	for _, d := range r.departments {
		if d.Status != "inactive" {
			continue
		}
		count := 0
		for mid, depts := range r.departmentsByMembership {
			m, ok := r.memberships[mid]
			if !ok || m.CompanyID != companyID {
				continue
			}
			if _, ok := depts[d.DepartmentID]; ok {
				count++
			}
		}
		if count > 0 {
			name := d.Name
			if name == "" {
				name = d.DepartmentName
			}
			out = append(out, conflict.InactiveDepartmentRow{
				DepartmentID:   d.DepartmentID,
				DepartmentName: name,
				MemberCount:    count,
			})
		}
	}
	return out, nil
}

func (r *AdminRepository) ListActiveDirectPermissionsByCompany(_ context.Context, companyID string) ([]conflict.DirectPermissionRow, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	companyID = strings.TrimSpace(companyID)
	var out []conflict.DirectPermissionRow
	for key, dp := range r.directPermissions {
		parts := strings.SplitN(key, ":", 2)
		if len(parts) != 2 {
			continue
		}
		membershipID := parts[0]
		m, ok := r.memberships[membershipID]
		if !ok || m.CompanyID != companyID {
			continue
		}
		out = append(out, conflict.DirectPermissionRow{
			MembershipID:   membershipID,
			PermissionCode: dp.PermissionCode,
		})
	}
	return out, nil
}
