package inmemory

import (
	"context"
	"strings"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

type Checker struct{}

func NewChecker() *Checker { return &Checker{} }

func (c *Checker) Check(_ context.Context, req authapp.AuthorizeRequest, effective *authapp.EffectiveAccessSummary) (*authapp.AuthorizeDecision, error) {
	if req.Subject.CompanyID != effective.CompanyID || req.Subject.MembershipID != effective.MembershipID {
		code := perr.CodeCompanyScopeMismatch
		return &authapp.AuthorizeDecision{Decision: authapp.DecisionDeny, MatchedPermissions: []string{}, ScopeReasons: []string{}, ResponsibilityReasons: []string{}, DenyReasonCode: &code}, nil
	}

	mapped := mapActionToPermission(req.Action)
	if mapped == "" || !contains(effective.Permissions, mapped) {
		code := perr.CodePermissionDenied
		return &authapp.AuthorizeDecision{Decision: authapp.DecisionDeny, MatchedPermissions: []string{}, ScopeReasons: []string{}, ResponsibilityReasons: []string{}, DenyReasonCode: &code}, nil
	}

	scopeReasons := []string{}
	if effective.DataScope.HasCompanyWideAccess {
		scopeReasons = append(scopeReasons, "company_wide_access")
	} else {
		if depID, _ := req.Resource.Attributes["department_id"].(string); depID != "" {
			if containsDepartment(effective.DataScope.Departments, depID) {
				scopeReasons = append(scopeReasons, "department_membership:"+depID)
			}
		}
		if containsAssignment(effective.DataScope.RecordAssignments, req.Resource.Type, req.Resource.ID) {
			scopeReasons = append(scopeReasons, "assignment:"+req.Resource.Type+":"+req.Resource.ID)
		}
	}
	if len(scopeReasons) == 0 {
		code := perr.CodeDataScopeDenied
		return &authapp.AuthorizeDecision{Decision: authapp.DecisionDeny, MatchedPermissions: []string{mapped}, ScopeReasons: []string{}, ResponsibilityReasons: []string{}, DenyReasonCode: &code}, nil
	}

	respReasons := []string{}
	if strings.Contains(req.Action, "approve") {
		if containsResponsibility(effective.Responsibilities, "workflow_approver:disclosure") {
			respReasons = append(respReasons, "workflow_assignee_rule:legal_approval")
		} else {
			code := perr.CodeResponsibilityRequired
			return &authapp.AuthorizeDecision{Decision: authapp.DecisionDeny, MatchedPermissions: []string{mapped}, ScopeReasons: scopeReasons, ResponsibilityReasons: []string{}, DenyReasonCode: &code}, nil
		}
	}

	return &authapp.AuthorizeDecision{
		Decision:              authapp.DecisionAllow,
		MatchedPermissions:    []string{mapped},
		ScopeReasons:          scopeReasons,
		ResponsibilityReasons: respReasons,
		DenyReasonCode:        nil,
	}, nil
}

func mapActionToPermission(action string) string {
	switch strings.TrimSpace(action) {
	case "disclosure.approve":
		return "approve_disclosure"
	case "disclosure.view":
		return "view_disclosure"
	case "disclosure.create":
		return "create_disclosure"
	case "disclosure.update":
		return "update_disclosure"
	case "disclosure.submit":
		return "submit_disclosure"
	case "workflow.create":
		return "create_workflow"
	case "workflow.review":
		return "review_workflow_task"
	case "workflow.confirm":
		return "confirm_workflow_task"
	case "workflow.resolve_assignees":
		return "create_workflow"
	case "notification.enqueue":
		return "enqueue_notification"
	case "notification.dispatch":
		return "dispatch_notification"
	case "notification.resolve_recipients":
		return "enqueue_notification"
	case "dashboard.view":
		return "view_dashboard"
	case "admin.membership.create",
		"admin.membership.update",
		"admin.membership.delete",
		"admin.membership.list",
		"admin.membership.role.assign",
		"admin.membership.role.remove",
		"admin.membership.department.assign",
		"admin.membership.department.remove",
		"admin.membership.title.assign",
		"admin.membership.title.remove",
		"admin.permissions.list",
		"admin.roles.list",
		"admin.role.permission.assign",
		"admin.role.permission.remove",
		"admin.resource_scope_rule.create",
		"admin.workflow_assignee_rule.create",
		"admin.notification_rule.create":
		return "admin_manage_access"
	default:
		return ""
	}
}

func contains(items []string, target string) bool {
	for _, it := range items {
		if strings.EqualFold(it, target) {
			return true
		}
	}
	return false
}

func containsDepartment(items []authapp.DepartmentScope, depID string) bool {
	for _, it := range items {
		if it.DepartmentID == depID {
			return true
		}
	}
	return false
}

func containsAssignment(items []authapp.ResourceAssignment, resourceType, resourceID string) bool {
	for _, it := range items {
		if it.ResourceType == resourceType && it.ResourceID == resourceID {
			return true
		}
	}
	return false
}

func containsResponsibility(items []string, target string) bool {
	for _, it := range items {
		if strings.EqualFold(it, target) {
			return true
		}
	}
	return false
}
