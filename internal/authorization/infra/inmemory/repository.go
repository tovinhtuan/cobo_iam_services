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
}

func NewRepository() *Repository {
	return &Repository{
		Permissions: map[string][]string{
			"m_001@c_001": {
				"view_disclosure", "approve_disclosure", "view_dashboard",
				"create_disclosure", "update_disclosure", "submit_disclosure",
				"create_workflow", "review_workflow_task", "confirm_workflow_task",
				"enqueue_notification", "dispatch_notification",
				"admin_manage_access",
			},
			"m_002@c_002": {"view_disclosure"},
			"m_010@c_010": {"view_dashboard"},
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

func key(membershipID, companyID string) string { return membershipID + "@" + companyID }
