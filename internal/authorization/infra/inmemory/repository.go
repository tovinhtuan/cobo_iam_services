package inmemory

import (
	"context"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
)

type Repository struct {
	Permissions      map[string][]string
	Departments      map[string][]authapp.DepartmentScope
	Assignments      map[string][]authapp.ResourceAssignment
	Responsibilities map[string][]string
	PositionCodes    map[string][]string
	OrgUnitIDs       map[string][]string
	OrgSubtreeUnitIDs map[string][]string
	Policies         map[string]authapp.ActionPolicy
}

func NewRepository() *Repository {
	return &Repository{
		Permissions: map[string][]string{
			"m_001@c_001": {
				"company.view", "view_disclosure", "approve_disclosure", "view_dashboard",
				"create_disclosure", "update_disclosure", "submit_disclosure",
				"create_workflow", "review_workflow_task", "confirm_workflow_task",
				"enqueue_notification", "dispatch_notification",
				"admin_manage_access",
				"disclosure.view", "disclosure.approve", "disclosure.create", "disclosure.edit",
			},
			"m_002@c_002": {"view_disclosure"},
			"m_010@c_010": {"company.view", "view_dashboard"},
			"m_admin_001@c_001": {
				"company.view", "company.edit",
				"recipient.view", "recipient.manage",
				"deadline.view", "deadline.manage", "deadline.create", "deadline.assign",
				"alert.channels.manage",
				"disclosure.view", "disclosure.create", "disclosure.edit", "disclosure.publish", "disclosure.delete", "disclosure.approve",
				"user.view", "user.edit",
				"workflow.step.confirm", "workflow.step.override",
				"auth.session.manage", "audit.view",
				"system.settings", "rbac.manage",
				"view_disclosure", "approve_disclosure", "view_dashboard",
				"create_disclosure", "update_disclosure", "submit_disclosure",
				"create_workflow", "review_workflow_task", "confirm_workflow_task",
				"enqueue_notification", "dispatch_notification",
				"admin_manage_access",
			},
		},
		Departments: map[string][]authapp.DepartmentScope{
			"m_001@c_001": {
				{DepartmentID: "d_legal", DepartmentName: "Legal"},
				{DepartmentID: "d_ir", DepartmentName: "IR"},
			},
			"m_admin_001@c_001": {
				{DepartmentID: "d_legal", DepartmentName: "Legal"},
			},
		},
		Assignments: map[string][]authapp.ResourceAssignment{
			"m_001@c_001": {{ResourceType: "disclosure_record", ResourceID: "r_1001"}},
			"m_admin_001@c_001": {{ResourceType: "disclosure_record", ResourceID: "r_1001"}},
		},
		Responsibilities: map[string][]string{
			"m_001@c_001": {"workflow_approver:disclosure", "notification_recipient:disclosure"},
			"m_admin_001@c_001": {"workflow_approver:disclosure", "notification_recipient:disclosure"},
		},
		PositionCodes: map[string][]string{
			"m_001@c_001":       {"truong_phong"},
			"m_admin_001@c_001": {"admin_dn"},
		},
		OrgUnitIDs: map[string][]string{
			"m_001@c_001":       {"ou_legal"},
			"m_admin_001@c_001": {"ou_root"},
		},
		OrgSubtreeUnitIDs: map[string][]string{
			"m_001@c_001":       {"ou_legal", "ou_legal_team_a", "ou_legal_team_b"},
			"m_admin_001@c_001": {"ou_root", "ou_legal", "ou_legal_team_a", "ou_legal_team_b", "ou_ir"},
		},
		Policies: defaultPolicies(),
	}
}

func (r *Repository) ListPermissionCodes(_ context.Context, membershipID, companyID string) ([]string, error) {
	return append([]string(nil), r.Permissions[key(membershipID, companyID)]...), nil
}

func (r *Repository) ListDepartmentScopes(_ context.Context, membershipID, companyID string) ([]authapp.DepartmentScope, error) {
	items := r.Departments[key(membershipID, companyID)]
	out := make([]authapp.DepartmentScope, len(items))
	copy(out, items)
	return out, nil
}

func (r *Repository) ListAssignments(_ context.Context, membershipID, companyID string) ([]authapp.ResourceAssignment, error) {
	items := r.Assignments[key(membershipID, companyID)]
	out := make([]authapp.ResourceAssignment, len(items))
	copy(out, items)
	return out, nil
}

func (r *Repository) ListResponsibilities(_ context.Context, membershipID, companyID string) ([]string, error) {
	return append([]string(nil), r.Responsibilities[key(membershipID, companyID)]...), nil
}

func (r *Repository) ListPositionCodes(_ context.Context, membershipID, companyID string) ([]string, error) {
	return append([]string(nil), r.PositionCodes[key(membershipID, companyID)]...), nil
}

func (r *Repository) ListOrgUnitIDs(_ context.Context, membershipID, companyID string) ([]string, error) {
	return append([]string(nil), r.OrgUnitIDs[key(membershipID, companyID)]...), nil
}

func (r *Repository) ListOrgSubtreeUnitIDs(_ context.Context, membershipID, companyID string) ([]string, error) {
	return append([]string(nil), r.OrgSubtreeUnitIDs[key(membershipID, companyID)]...), nil
}

func (r *Repository) GetActionPolicy(_ context.Context, _ string, action string) (*authapp.ActionPolicy, error) {
	if p, ok := r.Policies[action]; ok {
		cp := p
		return &cp, nil
	}
	p := authapp.ActionPolicy{
		ActionCode:         action,
		RequiredPermission: "system.settings",
		ScopeType:          "*",
		WorkflowState:      "*",
		EligibleActor:      "*",
		EffectType:         "allow",
		DenyReasonCode:     "permission_denied",
	}
	return &p, nil
}

func key(membershipID, companyID string) string { return membershipID + "@" + companyID }

func defaultPolicies() map[string]authapp.ActionPolicy {
	return map[string]authapp.ActionPolicy{
		"disclosure.view":    {ActionCode: "disclosure.view", RequiredPermission: "disclosure.view", ScopeType: "org_unit_subtree|assigned_only|owner_only|company_wide", WorkflowState: "*", EligibleActor: "*", EffectType: "allow", DenyReasonCode: "scope_denied"},
		"disclosure.create":  {ActionCode: "disclosure.create", RequiredPermission: "disclosure.create", ScopeType: "*", WorkflowState: "*", EligibleActor: "*", EffectType: "allow", DenyReasonCode: "permission_denied"},
		"disclosure.update":  {ActionCode: "disclosure.update", RequiredPermission: "disclosure.edit", ScopeType: "*", WorkflowState: "*", EligibleActor: "*", EffectType: "allow", DenyReasonCode: "permission_denied"},
		"disclosure.submit":  {ActionCode: "disclosure.submit", RequiredPermission: "submit_disclosure", ScopeType: "*", WorkflowState: "*", EligibleActor: "*", EffectType: "allow", DenyReasonCode: "permission_denied"},
		"disclosure.approve": {ActionCode: "disclosure.approve", RequiredPermission: "disclosure.approve", ScopeType: "*", WorkflowState: "*", EligibleActor: "*", EffectType: "allow", DenyReasonCode: "responsibility_required"},
		"workflow.review":    {ActionCode: "workflow.review", RequiredPermission: "review_workflow_task", ScopeType: "*", WorkflowState: "*", EligibleActor: "*", EffectType: "allow", DenyReasonCode: "permission_denied"},
	}
}
