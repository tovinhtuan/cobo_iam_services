package inmemory

import (
	"context"
	"strings"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	"github.com/cobo/cobo_iam_services/internal/companyaccess/dependency"
)

// DependencyReader adapts in-memory AdminRepository to dependency.Reader.
type DependencyReader struct {
	Repo *AdminRepository
}

func NewDependencyReader(repo *AdminRepository) dependency.Reader {
	if repo == nil {
		return nil
	}
	return DependencyReader{Repo: repo}
}

func (d DependencyReader) DepartmentBelongsToCompany(_ context.Context, companyID, departmentID string) (bool, error) {
	d.Repo.mu.RLock()
	defer d.Repo.mu.RUnlock()
	dept, ok := d.Repo.departments[departmentID]
	if !ok || dept.Status == "inactive" {
		return false, nil
	}
	_ = companyID
	return true, nil
}

func (d DependencyReader) RoleAccessibleByCompany(ctx context.Context, companyID, roleID string) (bool, error) {
	return d.Repo.RoleAccessibleByCompany(ctx, companyID, roleID)
}

func (d DependencyReader) ListDepartmentMemberSamples(_ context.Context, companyID, departmentID string, limit int) ([]dependency.Sample, int, error) {
	d.Repo.mu.RLock()
	defer d.Repo.mu.RUnlock()
	var samples []dependency.Sample
	total := 0
	for mid, depts := range d.Repo.departmentsByMembership {
		if _, ok := depts[departmentID]; !ok {
			continue
		}
		m, ok := d.Repo.memberships[mid]
		if !ok || m.CompanyID != companyID || strings.EqualFold(m.Status, "deleted") {
			continue
		}
		total++
		if len(samples) < limit {
			name := ""
			if u, ok := d.Repo.users[m.UserID]; ok {
				name = u.FullName
			}
			samples = append(samples, dependency.Sample{
				"membership_id": mid,
				"display_name":  name,
			})
		}
	}
	return samples, total, nil
}

func (d DependencyReader) ListRoleMembershipSamples(_ context.Context, companyID, roleID string, limit int) ([]dependency.Sample, int, error) {
	d.Repo.mu.RLock()
	defer d.Repo.mu.RUnlock()
	var samples []dependency.Sample
	total := 0
	for mid, roles := range d.Repo.rolesByMembership {
		if _, ok := roles[roleID]; !ok {
			continue
		}
		m, ok := d.Repo.memberships[mid]
		if !ok || m.CompanyID != companyID || strings.EqualFold(m.Status, "deleted") {
			continue
		}
		total++
		if len(samples) < limit {
			name := ""
			if u, ok := d.Repo.users[m.UserID]; ok {
				name = u.FullName
			}
			samples = append(samples, dependency.Sample{
				"membership_id": mid,
				"display_name":  name,
			})
		}
	}
	return samples, total, nil
}

func (d DependencyReader) ListRolePermissionSamples(_ context.Context, companyID, roleID string, limit int) ([]dependency.Sample, int, error) {
	ok, err := d.Repo.RoleAccessibleByCompany(context.Background(), companyID, roleID)
	if err != nil || !ok {
		return nil, 0, err
	}
	d.Repo.mu.RLock()
	defer d.Repo.mu.RUnlock()
	set := d.Repo.rolePermissions[roleID]
	total := len(set)
	if total == 0 {
		return nil, 0, nil
	}
	samples := make([]dependency.Sample, 0, limit)
	for code := range set {
		if len(samples) >= limit {
			break
		}
		samples = append(samples, dependency.Sample{
			"permission_code": code,
			"permission_name": code,
		})
	}
	return samples, total, nil
}

func (d DependencyReader) ListWorkflowOverrideStepsForDepartment(_ context.Context, _, _ string, _ int) ([]dependency.Sample, int, bool, error) {
	return nil, 0, false, nil
}

// SeedDepartmentMember links a membership to a department for dependency tests.
func (r *AdminRepository) SeedDepartmentMember(companyID, departmentID, membershipID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.memberships[membershipID]; !ok {
		r.memberships[membershipID] = caapp.MembershipView{
			MembershipID: membershipID,
			CompanyID:    companyID,
			Status:       "active",
		}
	}
	if r.departmentsByMembership[membershipID] == nil {
		r.departmentsByMembership[membershipID] = map[string]struct{}{}
	}
	r.departmentsByMembership[membershipID][departmentID] = struct{}{}
	if d, ok := r.departments[departmentID]; ok {
		d.MemberCount++
		r.departments[departmentID] = d
	} else {
		r.departments[departmentID] = caapp.DepartmentView{
			DepartmentID: departmentID,
			Name:         departmentID,
			Status:       "active",
		}
	}
	_ = companyID
}

var _ dependency.Reader = DependencyReader{}
