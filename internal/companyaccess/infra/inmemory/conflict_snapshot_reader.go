package inmemory

import (
	"context"

	"github.com/cobo/cobo_iam_services/internal/companyaccess/conflict"
)

// ConflictSnapshotReader adapts in-memory AdminRepository to conflict.SnapshotReader.
type ConflictSnapshotReader struct {
	Repo *AdminRepository
}

func NewConflictSnapshotReader(repo *AdminRepository) conflict.SnapshotReader {
	if repo == nil {
		return nil
	}
	return ConflictSnapshotReader{Repo: repo}
}

func (a ConflictSnapshotReader) GetNotificationRuleByCode(ctx context.Context, companyID, ruleCode string) (*conflict.NotificationRuleSnapshot, error) {
	v, err := a.Repo.GetNotificationRuleByCode(ctx, companyID, ruleCode)
	if err != nil || v == nil {
		return nil, err
	}
	return &conflict.NotificationRuleSnapshot{RuleID: v.NotificationRuleID, Payload: v.Payload}, nil
}

func (a ConflictSnapshotReader) ListRoles(ctx context.Context, companyID string) ([]conflict.RoleSnapshot, error) {
	rows, err := a.Repo.ListRoles(ctx, companyID)
	if err != nil {
		return nil, err
	}
	out := make([]conflict.RoleSnapshot, 0, len(rows))
	for _, r := range rows {
		out = append(out, conflict.RoleSnapshot{
			RoleID: r.RoleID, RoleCode: r.RoleCode, RoleName: r.RoleName,
			MemberCount: r.MemberCount, Status: r.Status,
		})
	}
	return out, nil
}

func (a ConflictSnapshotReader) ListRolePermissionCodes(ctx context.Context, companyID, roleID string) ([]string, error) {
	v, err := a.Repo.ListRolePermissions(ctx, companyID, roleID)
	if err != nil || v == nil {
		return nil, err
	}
	codes := make([]string, 0, len(v.Permissions))
	for _, p := range v.Permissions {
		codes = append(codes, p.PermissionCode)
	}
	return codes, nil
}

func (a ConflictSnapshotReader) ListStaleWorkflowOverridesByCompany(ctx context.Context, companyID string) ([]conflict.StaleWorkflowOverrideRow, error) {
	return a.Repo.ListStaleWorkflowOverridesByCompany(ctx, companyID)
}

func (a ConflictSnapshotReader) ListWorkflowAssigneeRulesByCompany(ctx context.Context, companyID string) ([]conflict.WorkflowAssigneeRuleRow, error) {
	return a.Repo.ListWorkflowAssigneeRulesByCompany(ctx, companyID)
}

func (a ConflictSnapshotReader) ListInactiveDepartmentsWithMembers(ctx context.Context, companyID string) ([]conflict.InactiveDepartmentRow, error) {
	return a.Repo.ListInactiveDepartmentsWithMembers(ctx, companyID)
}

func (a ConflictSnapshotReader) ListActiveDirectPermissionsByCompany(ctx context.Context, companyID string) ([]conflict.DirectPermissionRow, error) {
	return a.Repo.ListActiveDirectPermissionsByCompany(ctx, companyID)
}

var _ conflict.SnapshotReader = ConflictSnapshotReader{}
