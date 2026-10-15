package app

import (
	"context"
	"strings"
)

// applyRecipientPolicies filters resolver candidates per CQ-03 (policies never create recipients).
func applyRecipientPolicies(
	ctx context.Context,
	querier MembershipEmailQuerier,
	taskReader WorkflowTaskAssigneeReader,
	stepReader WorkflowStepReader,
	instanceReader InstanceStepReader,
	companyID string,
	candidates []string,
	policies []string,
	c DispatchCandidate,
) ([]string, error) {
	if len(policies) == 0 {
		return candidates, nil
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	companyID = strings.TrimSpace(companyID)
	if companyID == "" {
		return nil, nil
	}

	allowed := make(map[string]struct{})
	for _, policy := range policies {
		emails, err := policyAllowedEmails(ctx, querier, taskReader, stepReader, instanceReader, companyID, policy, c)
		if err != nil {
			return nil, err
		}
		for _, e := range emails {
			el := strings.ToLower(strings.TrimSpace(e))
			if el != "" {
				allowed[el] = struct{}{}
			}
		}
	}

	out := make([]string, 0, len(candidates))
	for _, e := range candidates {
		el := strings.ToLower(strings.TrimSpace(e))
		if _, ok := allowed[el]; ok {
			out = append(out, e)
		}
	}
	return deduplicateEmails(out), nil
}

func policyAllowedEmails(
	ctx context.Context,
	querier MembershipEmailQuerier,
	taskReader WorkflowTaskAssigneeReader,
	stepReader WorkflowStepReader,
	instanceReader InstanceStepReader,
	companyID, policy string,
	c DispatchCandidate,
) ([]string, error) {
	switch strings.TrimSpace(strings.ToLower(policy)) {
	case "company_admin":
		if querier == nil {
			return nil, nil
		}
		return querier.AdminEmailsByCompany(ctx, companyID)
	case "assignee":
		if taskReader == nil || c.ScopeType != ScopeTypeWorkflowStep {
			return nil, nil
		}
		stepCode := extractStepID(c.ScopeID)
		return taskReader.AssigneeEmailsByStep(ctx, companyID, c.WorkflowInstanceID, stepCode)
	case "department_focal":
		if querier == nil || c.ScopeType != ScopeTypeWorkflowStep {
			return nil, nil
		}
		stepID := extractStepID(c.ScopeID)
		var step *WorkflowStepConfig
		// RMC-08: the instance snapshot carries the company's own step first; the global step only
		// names a template department, which must be mapped onto the company's department.
		if instanceReader != nil && strings.TrimSpace(c.WorkflowInstanceID) != "" {
			st, err := instanceReader.GetStepByInstance(ctx, companyID, c.WorkflowInstanceID, stepID)
			if err != nil {
				return nil, err
			}
			step = st
		}
		if step == nil && stepReader != nil {
			st, err := stepReader.GetStepByID(ctx, stepID)
			if err != nil {
				return nil, err
			}
			step = st
		}
		if step == nil || strings.TrimSpace(step.DepartmentID) == "" {
			return nil, nil
		}
		deptID, ok, err := querier.ResolveCompanyDepartment(ctx, companyID, strings.TrimSpace(step.DepartmentID))
		if err != nil {
			return nil, err
		}
		if !ok || deptID == "" {
			return nil, nil
		}
		return querier.EmailsByDepartments(ctx, companyID, []string{deptID})
	default:
		return nil, nil
	}
}
