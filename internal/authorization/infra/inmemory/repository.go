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
		},
		Departments: map[string][]authapp.DepartmentScope{
			"m_001@c_001": {
				{DepartmentID: "d_legal", DepartmentName: "Legal"},
				{DepartmentID: "d_ir", DepartmentName: "IR"},
			},
		},
		Assignments: map[string][]authapp.ResourceAssignment{
			"m_001@c_001": {{ResourceType: "disclosure_record", ResourceID: "r_1001"}},
		},
		Responsibilities: map[string][]string{
			"m_001@c_001": {"workflow_approver:disclosure", "notification_recipient:disclosure"},
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
