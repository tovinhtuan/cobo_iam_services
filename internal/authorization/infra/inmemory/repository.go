package inmemory

import (
	"context"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
)

type Repository struct {
	Permissions       map[string][]string
	Departments       map[string][]authapp.DepartmentScope
	Assignments       map[string][]authapp.ResourceAssignment
	Responsibilities  map[string][]string
	PositionCodes     map[string][]string
	OrgUnitIDs        map[string][]string
	OrgSubtreeUnitIDs map[string][]string
	Policies          map[string]authapp.ActionPolicy
}

func NewRepository() *Repository {
	return &Repository{
		Permissions: map[string][]string{
			"m_001@c_001": {
				// Common user fixture: no admin/system permissions.
				"company.view", "dashboard.view",
				"disclosure.view", "disclosure.create", "disclosure.edit", "disclosure.publish",
			},
			"m_002@c_002": {"disclosure.view"},
			"m_010@c_010": {"company.view", "dashboard.view"},
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
				"template.workflow.override.read", "template.workflow.override.write", "template.workflow.override.approve", "template.workflow.override.reset",
			},
			"m_cms_001@c_001": {
				"platform.cms.view",
				"company.view", "company.edit",
				"recipient.view", "recipient.manage",
				"deadline.view", "deadline.manage", "deadline.create", "deadline.assign",
				"alert.channels.manage",
				"disclosure.view", "disclosure.create", "disclosure.edit", "disclosure.publish", "disclosure.delete", "disclosure.approve",
				"user.view", "user.edit",
				"workflow.step.confirm", "workflow.step.override",
				"auth.session.manage", "audit.view",
				"system.settings", "rbac.manage",
				"template.workflow.override.read", "template.workflow.override.write", "template.workflow.override.approve", "template.workflow.override.reset",
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
			"m_cms_001@c_001": {
				{DepartmentID: "d_legal", DepartmentName: "Legal"},
			},
		},
		Assignments: map[string][]authapp.ResourceAssignment{
			"m_001@c_001":       {{ResourceType: "disclosure_record", ResourceID: "r_1001"}},
			"m_admin_001@c_001": {{ResourceType: "disclosure_record", ResourceID: "r_1001"}},
			"m_cms_001@c_001":   {{ResourceType: "disclosure_record", ResourceID: "r_1001"}},
		},
		Responsibilities: map[string][]string{
			"m_001@c_001":       {"workflow_approver:disclosure", "notification_recipient:disclosure"},
			"m_admin_001@c_001": {"workflow_approver:disclosure", "notification_recipient:disclosure"},
			"m_cms_001@c_001":   {"workflow_approver:disclosure", "notification_recipient:disclosure"},
		},
		PositionCodes: map[string][]string{
			"m_001@c_001":       {"truong_phong"},
			"m_admin_001@c_001": {"admin_dn"},
			"m_cms_001@c_001":   {"cms_operator"},
		},
		OrgUnitIDs: map[string][]string{
			"m_001@c_001":       {"ou_legal"},
			"m_admin_001@c_001": {"ou_root"},
			"m_cms_001@c_001":   {"ou_root"},
		},
		OrgSubtreeUnitIDs: map[string][]string{
			"m_001@c_001":       {"ou_legal", "ou_legal_team_a", "ou_legal_team_b"},
			"m_admin_001@c_001": {"ou_root", "ou_legal", "ou_legal_team_a", "ou_legal_team_b", "ou_ir"},
			"m_cms_001@c_001":   {"ou_root", "ou_legal", "ou_legal_team_a", "ou_legal_team_b", "ou_ir"},
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
		"disclosure.view":                    {ActionCode: "disclosure.view", RequiredPermission: "disclosure.view", ScopeType: "org_unit_subtree|assigned_only|owner_only|company_wide", WorkflowState: "*", EligibleActor: "*", EffectType: "allow", DenyReasonCode: "scope_denied"},
		"disclosure.create":                  {ActionCode: "disclosure.create", RequiredPermission: "disclosure.create", ScopeType: "*", WorkflowState: "*", EligibleActor: "*", EffectType: "allow", DenyReasonCode: "permission_denied"},
		"disclosure.update":                  {ActionCode: "disclosure.update", RequiredPermission: "disclosure.edit", ScopeType: "*", WorkflowState: "*", EligibleActor: "*", EffectType: "allow", DenyReasonCode: "permission_denied"},
		"disclosure.submit":                  {ActionCode: "disclosure.submit", RequiredPermission: "disclosure.publish", ScopeType: "*", WorkflowState: "*", EligibleActor: "*", EffectType: "allow", DenyReasonCode: "permission_denied"},
		"disclosure.approve":                 {ActionCode: "disclosure.approve", RequiredPermission: "disclosure.approve", ScopeType: "*", WorkflowState: "*", EligibleActor: "*", EffectType: "allow", DenyReasonCode: "responsibility_required"},
		"workflow.read":                      {ActionCode: "workflow.read", RequiredPermission: "disclosure.view", ScopeType: "*", WorkflowState: "*", EligibleActor: "*", EffectType: "allow", DenyReasonCode: "permission_denied"},
		"workflow.review":                    {ActionCode: "workflow.review", RequiredPermission: "deadline.assign", ScopeType: "*", WorkflowState: "*", EligibleActor: "*", EffectType: "allow", DenyReasonCode: "permission_denied"},
		"workflow.approve":                 {ActionCode: "workflow.approve", RequiredPermission: "disclosure.approve", ScopeType: "*", WorkflowState: "*", EligibleActor: "*", EffectType: "allow", DenyReasonCode: "permission_denied"},
		"template.workflow.override.read":    {ActionCode: "template.workflow.override.read", RequiredPermission: "template.workflow.override.read", ScopeType: "*", WorkflowState: "*", EligibleActor: "*", EffectType: "allow", DenyReasonCode: "permission_denied"},
		"template.workflow.override.write":   {ActionCode: "template.workflow.override.write", RequiredPermission: "template.workflow.override.write", ScopeType: "*", WorkflowState: "*", EligibleActor: "*", EffectType: "allow", DenyReasonCode: "permission_denied"},
		"template.workflow.override.approve": {ActionCode: "template.workflow.override.approve", RequiredPermission: "template.workflow.override.approve", ScopeType: "*", WorkflowState: "*", EligibleActor: "*", EffectType: "allow", DenyReasonCode: "permission_denied"},
		"template.workflow.override.reset":   {ActionCode: "template.workflow.override.reset", RequiredPermission: "template.workflow.override.reset", ScopeType: "*", WorkflowState: "*", EligibleActor: "*", EffectType: "allow", DenyReasonCode: "permission_denied"},
	}
}
