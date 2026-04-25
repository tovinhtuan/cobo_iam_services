package inmemory

import (
	"context"
	"net/http"
	"sync"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

type AdminRepository struct {
	mu sync.RWMutex

	users                   map[string]caapp.UserView
	usersByLoginID          map[string]string
	passwordHashByUserID    map[string]string
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
		users:                   map[string]caapp.UserView{},
		usersByLoginID:          map[string]string{},
		passwordHashByUserID:    map[string]string{},
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

func (r *AdminRepository) CreateUser(_ context.Context, u caapp.UserView, passwordHash string, opts caapp.CreateUserOptions) (*caapp.UserView, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if existingID, ok := r.usersByLoginID[u.LoginID]; ok && existingID != "" {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "login_id already exists", nil)
	}
	r.users[u.UserID] = u
	r.usersByLoginID[u.LoginID] = u.UserID
	r.passwordHashByUserID[u.UserID] = passwordHash
	if opts.CompanyID != "" {
		status := opts.MembershipStatus
		if status == "" {
			status = "active"
		}
		m := caapp.MembershipView{
			MembershipID: opts.MembershipID,
			UserID:       u.UserID,
			CompanyID:    opts.CompanyID,
			CompanyName:  opts.CompanyID,
			Status:       status,
		}
		r.memberships[m.MembershipID] = m
		u.MembershipID = m.MembershipID
		u.MembershipStatus = m.Status
		u.CompanyID = m.CompanyID
		u.CompanyName = m.CompanyName
	}
	cp := u
	return &cp, nil
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
func (r *AdminRepository) ListRoles(_ context.Context, _ string) ([]string, error) {
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
