package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cobo/cobo_iam_services/internal/companyaccess/dependency"
)

// DependencyReader adapts AdminRepository to dependency.Reader.
type DependencyReader struct {
	Repo *AdminRepository
}

func NewDependencyReader(repo *AdminRepository) dependency.Reader {
	if repo == nil {
		return nil
	}
	return DependencyReader{Repo: repo}
}

func (d DependencyReader) DepartmentBelongsToCompany(ctx context.Context, companyID, departmentID string) (bool, error) {
	companyID = strings.TrimSpace(companyID)
	departmentID = strings.TrimSpace(departmentID)
	if companyID == "" || departmentID == "" {
		return false, nil
	}
	var cid string
	err := d.Repo.db.QueryRowContext(ctx,
		`SELECT company_id FROM departments WHERE department_id = ? AND status != 'inactive'`,
		departmentID,
	).Scan(&cid)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return cid == companyID, nil
}

func (d DependencyReader) RoleAccessibleByCompany(ctx context.Context, companyID, roleID string) (bool, error) {
	return d.Repo.RoleAccessibleByCompany(ctx, companyID, roleID)
}

func (d DependencyReader) ListDepartmentMemberSamples(ctx context.Context, companyID, departmentID string, limit int) ([]dependency.Sample, int, error) {
	departmentID = strings.TrimSpace(departmentID)
	if departmentID == "" {
		return nil, 0, nil
	}
	var total int
	if err := d.Repo.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM department_memberships dm
		INNER JOIN memberships m ON m.membership_id = dm.membership_id
		WHERE dm.department_id = ? AND dm.status = 'active'
		  AND LOWER(m.membership_status) != 'deleted'
		  AND m.company_id = ?
	`, departmentID, companyID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count department members: %w", err)
	}
	if total == 0 {
		return nil, 0, nil
	}
	rows, err := d.Repo.db.QueryContext(ctx, `
		SELECT m.membership_id, COALESCE(u.full_name, '')
		FROM department_memberships dm
		INNER JOIN memberships m ON m.membership_id = dm.membership_id
		LEFT JOIN users u ON u.user_id = m.user_id
		WHERE dm.department_id = ? AND dm.status = 'active'
		  AND LOWER(m.membership_status) != 'deleted'
		  AND m.company_id = ?
		ORDER BY u.full_name, m.membership_id
		LIMIT ?
	`, departmentID, companyID, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("list department member samples: %w", err)
	}
	defer rows.Close()
	samples := make([]dependency.Sample, 0, limit)
	for rows.Next() {
		var membershipID, displayName string
		if err := rows.Scan(&membershipID, &displayName); err != nil {
			return nil, 0, err
		}
		samples = append(samples, dependency.Sample{
			"membership_id": membershipID,
			"display_name":  displayName,
		})
	}
	return samples, total, rows.Err()
}

func (d DependencyReader) ListRoleMembershipSamples(ctx context.Context, companyID, roleID string, limit int) ([]dependency.Sample, int, error) {
	roleID = strings.TrimSpace(roleID)
	if roleID == "" {
		return nil, 0, nil
	}
	var total int
	if err := d.Repo.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT mr.membership_id)
		FROM membership_roles mr
		INNER JOIN memberships m ON m.membership_id = mr.membership_id
		WHERE mr.role_id = ? AND mr.status = 'active'
		  AND m.company_id = ? AND LOWER(m.membership_status) != 'deleted'
	`, roleID, companyID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count role memberships: %w", err)
	}
	if total == 0 {
		return nil, 0, nil
	}
	rows, err := d.Repo.db.QueryContext(ctx, `
		SELECT m.membership_id, COALESCE(u.full_name, '')
		FROM membership_roles mr
		INNER JOIN memberships m ON m.membership_id = mr.membership_id
		LEFT JOIN users u ON u.user_id = m.user_id
		WHERE mr.role_id = ? AND mr.status = 'active'
		  AND m.company_id = ? AND LOWER(m.membership_status) != 'deleted'
		ORDER BY u.full_name, m.membership_id
		LIMIT ?
	`, roleID, companyID, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("list role membership samples: %w", err)
	}
	defer rows.Close()
	samples := make([]dependency.Sample, 0, limit)
	for rows.Next() {
		var membershipID, displayName string
		if err := rows.Scan(&membershipID, &displayName); err != nil {
			return nil, 0, err
		}
		samples = append(samples, dependency.Sample{
			"membership_id": membershipID,
			"display_name":  displayName,
		})
	}
	return samples, total, rows.Err()
}

func (d DependencyReader) ListRolePermissionSamples(ctx context.Context, companyID, roleID string, limit int) ([]dependency.Sample, int, error) {
	ok, err := d.Repo.RoleAccessibleByCompany(ctx, companyID, roleID)
	if err != nil || !ok {
		return nil, 0, err
	}
	var total int
	if err := d.Repo.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM role_permissions rp
		WHERE rp.role_id = ? AND rp.status = 'active'
	`, roleID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count role permissions: %w", err)
	}
	if total == 0 {
		return nil, 0, nil
	}
	rows, err := d.Repo.db.QueryContext(ctx, `
		SELECT p.permission_code, p.permission_name
		FROM role_permissions rp
		INNER JOIN permissions p ON p.permission_id = rp.permission_id
		WHERE rp.role_id = ? AND rp.status = 'active'
		ORDER BY p.permission_code
		LIMIT ?
	`, roleID, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("list role permission samples: %w", err)
	}
	defer rows.Close()
	samples := make([]dependency.Sample, 0, limit)
	for rows.Next() {
		var code, name string
		if err := rows.Scan(&code, &name); err != nil {
			return nil, 0, err
		}
		samples = append(samples, dependency.Sample{
			"permission_code": code,
			"permission_name": name,
		})
	}
	return samples, total, rows.Err()
}

func (d DependencyReader) ListWorkflowOverrideStepsForDepartment(ctx context.Context, companyID, departmentID string, scanCap int) ([]dependency.Sample, int, bool, error) {
	companyID = strings.TrimSpace(companyID)
	departmentID = strings.TrimSpace(departmentID)
	if companyID == "" || departmentID == "" || scanCap <= 0 {
		return nil, 0, false, nil
	}
	rows, err := d.Repo.db.QueryContext(ctx, `
		SELECT o.type_id, v.workflow_json
		FROM company_template_workflow_overrides o
		INNER JOIN company_template_workflow_override_versions v
		  ON v.override_id = o.override_id AND v.version_no = o.active_version_no
		WHERE o.company_id = ? AND o.active_version_no > 0
		ORDER BY o.type_id
		LIMIT ?
	`, companyID, scanCap)
	if err != nil {
		return nil, 0, false, fmt.Errorf("list workflow overrides for dependency scan: %w", err)
	}
	defer rows.Close()

	var samples []dependency.Sample
	total := 0
	scanned := 0
	for rows.Next() {
		scanned++
		var typeID string
		var raw []byte
		if err := rows.Scan(&typeID, &raw); err != nil {
			return nil, 0, false, err
		}
		matches := workflowJSONStepsForDepartment(raw, departmentID, typeID)
		total += len(matches)
		for _, m := range matches {
			if len(samples) < dependency.MaxSampleLimit {
				samples = append(samples, m)
			}
		}
	}
	truncated := scanned >= scanCap
	return samples, total, truncated, rows.Err()
}

func workflowJSONStepsForDepartment(raw []byte, departmentID, typeID string) []dependency.Sample {
	if len(raw) == 0 {
		return nil
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil
	}
	steps, ok := doc["steps"].([]any)
	if !ok {
		return nil
	}
	var out []dependency.Sample
	for _, item := range steps {
		step, ok := item.(map[string]any)
		if !ok {
			continue
		}
		stepDept, _ := step["department_id"].(string)
		if strings.TrimSpace(stepDept) != departmentID {
			continue
		}
		stepID, _ := step["step_id"].(string)
		out = append(out, dependency.Sample{
			"disclosure_type_id": typeID,
			"step_id":            stepID,
			"department_id":      departmentID,
		})
	}
	return out
}

var _ dependency.Reader = DependencyReader{}
