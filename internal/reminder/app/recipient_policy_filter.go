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
		emails, err := policyAllowedEmails(ctx, querier, taskReader, stepReader, companyID, policy, c)
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
		if querier == nil {
			return nil, nil
		}
		deptID := ""
		if c.ScopeType == ScopeTypeWorkflowStep && stepReader != nil {
			step, err := stepReader.GetStepByID(ctx, extractStepID(c.ScopeID))
			if err != nil {
				return nil, err
			}
			if step != nil {
				deptID = strings.TrimSpace(step.DepartmentID)
			}
		}
		if deptID == "" {
			return nil, nil
		}
		return querier.EmailsByDepartments(ctx, companyID, []string{deptID})
	default:
		return nil, nil
	}
}
