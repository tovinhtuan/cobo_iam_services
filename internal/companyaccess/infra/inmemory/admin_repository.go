package inmemory

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

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

	invitationsByUser map[string][]string // stacked token hashes for in-mem sanity (minimal)
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
		permissions:             map[string]struct{}{"dashboard.view": {}, "disclosure.view": {}, "disclosure.approve": {}, "rbac.manage": {}, "system.settings": {}},
		roles:                   map[string]struct{}{"company_admin": {}, "disclosure_approver": {}, "department_staff": {}},
		rolePermissions:         map[string]map[string]struct{}{},
		resourceScopeRules:      []map[string]any{},
		workflowAssigneeRules:   []map[string]any{},
		notificationRules:       []map[string]any{},
		invitationsByUser:       map[string][]string{},
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

func (r *AdminRepository) LookupUserByLoginID(_ context.Context, loginID string) (string, string, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.usersByLoginID[strings.ToLower(strings.TrimSpace(loginID))]
	if !ok || id == "" {
		return "", "", false, nil
	}
	u := r.users[id]
	return u.UserID, u.AccountStatus, true, nil
}

func (r *AdminRepository) GetUserProfile(_ context.Context, userID string) (loginID, email, fullName, accountStatus string, err error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.users[strings.TrimSpace(userID)]
	if !ok {
		return "", "", "", "", perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "user not found", nil)
	}
	return u.LoginID, u.Email, u.FullName, u.AccountStatus, nil
}

func (r *AdminRepository) MembershipExistsForUserCompany(_ context.Context, userID, companyID string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, m := range r.memberships {
		if m.UserID == userID && m.CompanyID == companyID {
			return true, nil
		}
	}
	return false, nil
}

func (r *AdminRepository) GetCompanyName(_ context.Context, companyID string) (string, error) {
	return strings.TrimSpace(companyID), nil
}

func (r *AdminRepository) InviteUserWithMembership(_ context.Context, u caapp.UserView, opts caapp.CreateUserOptions, invitationID, tokenHash, _ string, _ time.Time) (*caapp.UserView, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := strings.ToLower(strings.TrimSpace(u.LoginID))
	if existingID, ok := r.usersByLoginID[key]; ok && existingID != "" {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "login_id already exists", nil)
	}
	r.users[u.UserID] = u
	r.usersByLoginID[key] = u.UserID
	if opts.CompanyID != "" {
		ms := opts.MembershipStatus
		if ms == "" {
			ms = "active"
		}
		m := caapp.MembershipView{
			MembershipID: opts.MembershipID,
			UserID:       u.UserID,
			CompanyID:    opts.CompanyID,
			CompanyName:  opts.CompanyID,
			Status:       ms,
		}
		r.memberships[m.MembershipID] = m
		u.MembershipID = m.MembershipID
		u.MembershipStatus = m.Status
		u.CompanyID = m.CompanyID
		u.CompanyName = m.CompanyName
		if strings.TrimSpace(opts.InitialRoleID) != "" {
			addSet(r.rolesByMembership, m.MembershipID, strings.TrimSpace(opts.InitialRoleID))
		}
	}
	r.invitationsByUser[u.UserID] = append(r.invitationsByUser[u.UserID], invitationID+":"+tokenHash)
	cp := u
	return &cp, nil
}

func (r *AdminRepository) LookupRoleIDForInvite(_ context.Context, companyID, preferRoleID, preferRoleCode, defaultRoleCode string) (string, error) {
	_ = companyID
	if strings.TrimSpace(preferRoleID) != "" {
		return strings.TrimSpace(preferRoleID), nil
	}
	code := strings.TrimSpace(preferRoleCode)
	if code == "" {
		code = strings.TrimSpace(defaultRoleCode)
	}
	if code == "" {
		return "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invite role required", nil)
	}
	return "r_invite_" + code, nil
}

func (r *AdminRepository) ListInviteRolesForCompany(_ context.Context, companyID string) ([]caapp.InviteRoleOption, error) {
	_ = companyID
	r.mu.RLock()
	defer r.mu.RUnlock()
	codes := make([]string, 0, len(r.roles))
	for c := range r.roles {
		codes = append(codes, c)
	}
	sort.Strings(codes)
	out := make([]caapp.InviteRoleOption, 0, len(codes))
	for _, code := range codes {
		out = append(out, caapp.InviteRoleOption{
			RoleID:   "r_invite_" + code,
			RoleCode: code,
			RoleName: code,
		})
	}
	return out, nil
}

func (r *AdminRepository) InviteUserWithoutCompany(_ context.Context, u caapp.UserView, invitationID, tokenHash, _ string, _ time.Time) (*caapp.UserView, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := strings.ToLower(strings.TrimSpace(u.LoginID))
	if existingID, ok := r.usersByLoginID[key]; ok && existingID != "" {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "login_id already exists", nil)
	}
	r.users[u.UserID] = u
	r.usersByLoginID[key] = u.UserID
	r.invitationsByUser[u.UserID] = append(r.invitationsByUser[u.UserID], invitationID+":"+tokenHash)
	cp := u
	return &cp, nil
}

func (r *AdminRepository) ReplaceUserInvitation(_ context.Context, userID, invitationID, tokenHash, _ string, _ time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.users[userID]; !ok {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "user not found", nil)
	}
	r.invitationsByUser[userID] = append(r.invitationsByUser[userID], invitationID+":"+tokenHash)
	return nil
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
			cp := m
			if u, ok := r.users[m.UserID]; ok {
				cp.LoginID = u.LoginID
				cp.FullName = u.FullName
				cp.AccountStatus = u.AccountStatus
			}
			out = append(out, cp)
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

func (r *AdminRepository) CreateStandaloneCompany(_ context.Context, displayName string, _ caapp.CreateCompanyBootstrap) (string, string, error) {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return "", "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "company_name is required", nil)
	}
	return "", "", perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "CreateStandaloneCompany is not implemented for in-memory admin repository", nil)
}

func (r *AdminRepository) ListCompaniesPlatform(_ context.Context, _ caapp.ListPlatformCompaniesRequest) (*caapp.ListPlatformCompaniesResult, error) {
	return &caapp.ListPlatformCompaniesResult{Items: []caapp.PlatformCompanySummary{}, Total: 0}, nil
}

func (r *AdminRepository) GetCompanyPlatform(_ context.Context, _ string) (*caapp.PlatformCompanyDetail, error) {
	return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "company not found", nil)
}

func (r *AdminRepository) UpdateCompanyPlatform(_ context.Context, _ caapp.UpdatePlatformCompanyRequest) error {
	return nil
}

func (r *AdminRepository) SetCompanyStatusPlatform(_ context.Context, _, _ string) error {
	return nil
}
