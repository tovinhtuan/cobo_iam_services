package dependency

import "context"

// Reader loads capped dependency samples and counts (SELECT only).
type Reader interface {
	DepartmentBelongsToCompany(ctx context.Context, companyID, departmentID string) (bool, error)
	RoleAccessibleByCompany(ctx context.Context, companyID, roleID string) (bool, error)
	ListDepartmentMemberSamples(ctx context.Context, companyID, departmentID string, limit int) (samples []Sample, total int, err error)
	ListRoleMembershipSamples(ctx context.Context, companyID, roleID string, limit int) (samples []Sample, total int, err error)
	ListRolePermissionSamples(ctx context.Context, companyID, roleID string, limit int) (samples []Sample, total int, err error)
	ListWorkflowOverrideStepsForDepartment(ctx context.Context, companyID, departmentID string, scanCap int) (samples []Sample, total int, truncated bool, err error)
}
