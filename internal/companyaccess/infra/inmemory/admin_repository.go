package inmemory

import (
	"context"
	"sync"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
)

type AdminRepository struct {
	mu sync.RWMutex

	memberships             map[string]caapp.MembershipView
	rolesByMembership       map[string]map[string]struct{}
	departmentsByMembership map[string]map[string]struct{}
	titlesByMembership      map[string]map[string]struct{}

	permissions     map[string]struct{}
	roles           map[string]struct{}
	rolePermissions map[string]map[string]struct{}

	resourceScopeRules    []map[string]any
	workflowAssigneeRules []map[string]any
	notificationRules     []map[string]any
}

func NewAdminRepository() *AdminRepository {
	return &AdminRepository{
		memberships:             map[string]caapp.MembershipView{},
		rolesByMembership:       map[string]map[string]struct{}{},
		departmentsByMembership: map[string]map[string]struct{}{},
		titlesByMembership:      map[string]map[string]struct{}{},
		permissions:             map[string]struct{}{"view_dashboard": {}, "view_disclosure": {}, "approve_disclosure": {}, "admin_manage_access": {}},
		roles:                   map[string]struct{}{"company_admin": {}, "disclosure_approver": {}, "department_staff": {}},
		rolePermissions:         map[string]map[string]struct{}{},
		resourceScopeRules:      []map[string]any{},
		workflowAssigneeRules:   []map[string]any{},
		notificationRules:       []map[string]any{},
	}
}

func (r *AdminRepository) CreateMembership(_ context.Context, m caapp.MembershipView) (*caapp.MembershipView, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.memberships[m.MembershipID] = m
	cp := m
	return &cp, nil
}
func (r *AdminRepository) UpdateMembershipStatus(_ context.Context, membershipID, status string) (*caapp.MembershipView, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m := r.memberships[membershipID]
	m.Status = status
	r.memberships[membershipID] = m
	cp := m
	return &cp, nil
}
func (r *AdminRepository) DeleteMembership(_ context.Context, membershipID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.memberships, membershipID)
	return nil
}
func (r *AdminRepository) ListMembershipsByCompany(_ context.Context, companyID string) ([]caapp.MembershipView, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []caapp.MembershipView{}
	for _, m := range r.memberships {
		if m.CompanyID == companyID {
			out = append(out, m)
		}
	}
	return out, nil
}

func addSet(m map[string]map[string]struct{}, k, v string) {
	if m[k] == nil {
		m[k] = map[string]struct{}{}
	}
	m[k][v] = struct{}{}
}
func delSet(m map[string]map[string]struct{}, k, v string) {
	if m[k] == nil {
		return
	}
	delete(m[k], v)
}

func (r *AdminRepository) AddRole(_ context.Context, membershipID, roleID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	addSet(r.rolesByMembership, membershipID, roleID)
	return nil
}
func (r *AdminRepository) RemoveRole(_ context.Context, membershipID, roleID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delSet(r.rolesByMembership, membershipID, roleID)
	return nil
}
func (r *AdminRepository) AddDepartment(_ context.Context, membershipID, departmentID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	addSet(r.departmentsByMembership, membershipID, departmentID)
	return nil
}
func (r *AdminRepository) RemoveDepartment(_ context.Context, membershipID, departmentID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delSet(r.departmentsByMembership, membershipID, departmentID)
	return nil
}
func (r *AdminRepository) AddTitle(_ context.Context, membershipID, titleID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	addSet(r.titlesByMembership, membershipID, titleID)
	return nil
}
func (r *AdminRepository) RemoveTitle(_ context.Context, membershipID, titleID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delSet(r.titlesByMembership, membershipID, titleID)
	return nil
}

func (r *AdminRepository) ListPermissions(_ context.Context) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []string{}
	for p := range r.permissions {
		out = append(out, p)
	}
	return out, nil
}
func (r *AdminRepository) ListRoles(_ context.Context) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []string{}
	for p := range r.roles {
		out = append(out, p)
	}
	return out, nil
}
func (r *AdminRepository) AddRolePermission(_ context.Context, roleID, permissionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	addSet(r.rolePermissions, roleID, permissionID)
	r.permissions[permissionID] = struct{}{}
	r.roles[roleID] = struct{}{}
	return nil
}
func (r *AdminRepository) RemoveRolePermission(_ context.Context, roleID, permissionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delSet(r.rolePermissions, roleID, permissionID)
	return nil
}

func (r *AdminRepository) AddResourceScopeRule(_ context.Context, rule map[string]any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.resourceScopeRules = append(r.resourceScopeRules, rule)
	return nil
}
func (r *AdminRepository) AddWorkflowAssigneeRule(_ context.Context, rule map[string]any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.workflowAssigneeRules = append(r.workflowAssigneeRules, rule)
	return nil
}
func (r *AdminRepository) AddNotificationRule(_ context.Context, rule map[string]any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.notificationRules = append(r.notificationRules, rule)
	return nil
}
