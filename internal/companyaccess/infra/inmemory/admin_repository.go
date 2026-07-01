package inmemory

import (
	"context"
	"fmt"
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

	invitationsByUser map[string][]string                   // stacked token hashes for in-mem sanity (minimal)
	directPermissions map[string]caapp.DirectPermissionView // key: membershipID:permCode

	departments map[string]caapp.DepartmentView // department_id -> view

	companies map[string]*caapp.PlatformCompanyDetail

	companyFounder    map[string]string
	companyProvSource map[string]string
	companyTaxCodes   map[string]string

	notificationVersions []notificationVersionRow
	rbacVersions         []rbacVersionRow
	pendingApprovals     []pendingRow
	delegatedGrants      map[string]*delegationGrantRow
	emergencyGrants      map[string]*emergencyGrantRow
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
		directPermissions:       map[string]caapp.DirectPermissionView{},
		departments:             map[string]caapp.DepartmentView{},
		companies:               map[string]*caapp.PlatformCompanyDetail{},
		companyFounder:          map[string]string{},
		companyProvSource:       map[string]string{},
		companyTaxCodes:         map[string]string{},
		delegatedGrants:         map[string]*delegationGrantRow{},
		emergencyGrants:         map[string]*emergencyGrantRow{},
	}
}

// SeedDepartment registers a department for invite-scope tests.
func (r *AdminRepository) SeedDepartment(d caapp.DepartmentView) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.departments[d.DepartmentID] = d
}

// SeedCompany adds a company to the in-memory store for testing.
func (r *AdminRepository) SeedCompany(c caapp.PlatformCompanyDetail) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := c
	r.companies[c.CompanyID] = &cp
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
		if strings.TrimSpace(opts.InitialRoleID) != "" {
			addSet(r.rolesByMembership, m.MembershipID, strings.TrimSpace(opts.InitialRoleID))
		}
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
			RoleID:             "r_invite_" + code,
			RoleCode:           code,
			RoleName:           code,
			DefaultPermissions: []string{},
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
			if roleIDs, ok := r.rolesByMembership[m.MembershipID]; ok {
				for id := range roleIDs {
					code := strings.TrimPrefix(id, "r_invite_")
					cp.Roles = append(cp.Roles, caapp.RoleView{RoleID: id, RoleCode: code, RoleName: code})
				}
			}
			if deptIDs, ok := r.departmentsByMembership[m.MembershipID]; ok {
				for id := range deptIDs {
					deptName := id
					if dept, found := r.departments[id]; found {
						if strings.TrimSpace(dept.DepartmentName) != "" {
							deptName = dept.DepartmentName
						} else if strings.TrimSpace(dept.Name) != "" {
							deptName = dept.Name
						}
					}
					cp.Departments = append(cp.Departments, caapp.DepartmentView{DepartmentID: id, DepartmentName: deptName, Name: deptName})
				}
			}
			if titleIDs, ok := r.titlesByMembership[m.MembershipID]; ok {
				for id := range titleIDs {
					cp.Titles = append(cp.Titles, caapp.TitleView{TitleID: id, TitleName: id, Name: id})
				}
			}
			out = append(out, cp)
		}
	}
	return out, nil
}

func (r *AdminRepository) CountMembershipsForUser(_ context.Context, userID string) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	n := 0
	for _, m := range r.memberships {
		if m.UserID == userID {
			n++
		}
	}
	return n, nil
}

func (r *AdminRepository) CountEligibleMembershipsForUser(_ context.Context, userID string) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	n := 0
	for _, m := range r.memberships {
		if m.UserID != userID {
			continue
		}
		st := strings.TrimSpace(m.Status)
		if st == "active" || st == "invited" {
			n++
		}
	}
	return n, nil
}

func (r *AdminRepository) GetUserProvisioningGate(_ context.Context, userID string) (string, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.users[userID]
	if !ok {
		return "", false, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "user not found", nil)
	}
	verified := !strings.Contains(strings.ToLower(u.LoginID), "unverified")
	return u.AccountStatus, verified, nil
}

func (r *AdminRepository) countSelfProvisionedLocked(userID string) int {
	n := 0
	for cid, founder := range r.companyFounder {
		if founder != userID {
			continue
		}
		src := r.companyProvSource[cid]
		if src == caapp.ProvisioningSourceSelfServiceInitialize || src == caapp.ProvisioningSourceSelfServiceCreate {
			n++
		}
	}
	return n
}

func (r *AdminRepository) BootstrapSelfServiceCompanyTx(_ context.Context, in caapp.BootstrapSelfServiceInput) (*caapp.BootstrapSelfServiceResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	eligible := 0
	for _, m := range r.memberships {
		if m.UserID != in.UserID {
			continue
		}
		st := strings.TrimSpace(m.Status)
		if st == "active" || st == "invited" {
			eligible++
		}
	}
	switch in.Mode {
	case caapp.BootstrapModeInitialize:
		if eligible > 0 {
			return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "COMPANY_ALREADY_EXISTS", nil)
		}
	case caapp.BootstrapModeCreate:
		if eligible < 1 {
			return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "COMPANY_MEMBERSHIP_REQUIRED", nil)
		}
		if in.QuotaLimit > 0 {
			count := r.countSelfProvisionedLocked(in.UserID)
			if count >= in.QuotaLimit {
				he := perr.NewHTTPError(http.StatusPaymentRequired, perr.CodeQuotaExceeded, "subscription quota exceeded", nil)
				he.Details = map[string]any{
					"limit":   in.QuotaLimit,
					"current": count,
					"tier":    in.QuotaTier,
				}
				return nil, he
			}
		}
	default:
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid bootstrap mode", nil)
	}

	name := strings.TrimSpace(in.CompanyName)
	if name == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "company_name is required", nil)
	}
	tax := strings.TrimSpace(in.TaxCode)
	if tax != "" {
		if _, exists := r.companyTaxCodes[tax]; exists {
			return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "COMPANY_ALREADY_EXISTS", nil)
		}
	}

	selfBefore := r.countSelfProvisionedLocked(in.UserID)
	provSource := caapp.ProvisioningSourceSelfServiceInitialize
	if in.Mode == caapp.BootstrapModeCreate {
		provSource = caapp.ProvisioningSourceSelfServiceCreate
	}

	companyID := "c_" + in.MembershipID
	companyCode := "co_" + in.MembershipID
	r.companies[companyID] = &caapp.PlatformCompanyDetail{
		CompanyID: companyID, CompanyCode: companyCode, CompanyName: name, Status: "active",
	}
	r.companyFounder[companyID] = in.UserID
	r.companyProvSource[companyID] = provSource
	if tax != "" {
		r.companyTaxCodes[tax] = companyID
	}
	r.memberships[in.MembershipID] = caapp.MembershipView{
		MembershipID:   in.MembershipID,
		UserID:         in.UserID,
		CompanyID:      companyID,
		CompanyName:    name,
		Status:         "active",
		IsPrimaryAdmin: true,
	}
	return &caapp.BootstrapSelfServiceResult{
		CompanyID:            companyID,
		CompanyCode:          companyCode,
		CompanyName:          name,
		MembershipID:         in.MembershipID,
		SelfProvisionedCount: selfBefore + 1,
	}, nil
}

func (r *AdminRepository) RollbackBootstrapSelfServiceCompany(_ context.Context, companyID, userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	memCount := 0
	var memID string
	for id, m := range r.memberships {
		if m.CompanyID == companyID {
			memCount++
			memID = id
		}
	}
	if memCount != 1 {
		return perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "rollback not allowed: unexpected membership count", nil)
	}
	delete(r.memberships, memID)
	delete(r.companies, companyID)
	delete(r.companyFounder, companyID)
	delete(r.companyProvSource, companyID)
	for tax, cid := range r.companyTaxCodes {
		if cid == companyID {
			delete(r.companyTaxCodes, tax)
		}
	}
	return nil
}

func (r *AdminRepository) ListUsersWithNoMembership(_ context.Context) ([]caapp.MembershipView, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	hasMem := map[string]struct{}{}
	for _, m := range r.memberships {
		hasMem[m.UserID] = struct{}{}
	}
	ids := make([]string, 0, len(r.users))
	for uid := range r.users {
		if _, ok := hasMem[uid]; !ok {
			ids = append(ids, uid)
		}
	}
	sort.Strings(ids)
	out := make([]caapp.MembershipView, 0, len(ids))
	for _, uid := range ids {
		u := r.users[uid]
		out = append(out, caapp.MembershipView{
			UserID:        uid,
			LoginID:       u.LoginID,
			FullName:      u.FullName,
			AccountStatus: u.AccountStatus,
		})
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

func (r *AdminRepository) ListPermissions(_ context.Context) ([]caapp.PermissionListItem, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]caapp.PermissionListItem, 0, len(r.permissions))
	for code := range r.permissions {
		out = append(out, caapp.PermissionListItem{
			PermissionID:   code,
			PermissionCode: code,
			PermissionName: code,
			ModuleName:     "general",
			RiskLevel:      caapp.PermissionRiskLevel(code),
			IsGrantable:    caapp.IsGrantablePermission(code),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PermissionCode < out[j].PermissionCode })
	return out, nil
}

func (r *AdminRepository) ListRoles(_ context.Context, companyID string) ([]caapp.RoleListItem, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]caapp.RoleListItem, 0, len(r.roles))
	now := time.Now().UTC()
	for roleID := range r.roles {
		memberCount := 0
		for _, roleSet := range r.rolesByMembership {
			if _, ok := roleSet[roleID]; ok {
				memberCount++
			}
		}
		permCount := 0
		if perms, ok := r.rolePermissions[roleID]; ok {
			permCount = len(perms)
		}
		out = append(out, caapp.RoleListItem{
			RoleID:          roleID,
			RoleCode:        roleID,
			RoleName:        roleID,
			Status:          "active",
			Scope:           "global",
			IsBuiltin:       true,
			PermissionCount: permCount,
			MemberCount:     memberCount,
			CreatedAt:       now,
			UpdatedAt:       now,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RoleCode < out[j].RoleCode })
	_ = companyID
	return out, nil
}

func (r *AdminRepository) RoleAccessibleByCompany(_ context.Context, _, roleID string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.roles[roleID]
	return ok, nil
}

func (r *AdminRepository) ListRolePermissions(_ context.Context, _, roleID string) (*caapp.RolePermissionsView, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.roles[roleID]; !ok {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "role not found", nil)
	}
	perms := make([]caapp.PermissionListItem, 0)
	if set, ok := r.rolePermissions[roleID]; ok {
		for code := range set {
			perms = append(perms, caapp.PermissionListItem{
				PermissionID:   code,
				PermissionCode: code,
				PermissionName: code,
				ModuleName:     "general",
				RiskLevel:      caapp.PermissionRiskLevel(code),
				IsGrantable:    caapp.IsGrantablePermission(code),
			})
		}
	}
	sort.Slice(perms, func(i, j int) bool { return perms[i].PermissionCode < perms[j].PermissionCode })
	return &caapp.RolePermissionsView{RoleID: roleID, Permissions: perms}, nil
}

func (r *AdminRepository) GetNotificationRuleByCode(_ context.Context, companyID, ruleCode string) (*caapp.NotificationRuleView, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, rule := range r.notificationRules {
		cid, _ := strFromAnyMap(rule, "company_id")
		code, _ := strFromAnyMap(rule, "rule_code")
		if cid == companyID && code == ruleCode {
			id, _ := strFromAnyMap(rule, "notification_rule_id")
			payload := cloneAnyMap(rule)
			for _, k := range []string{"notification_rule_id", "company_id", "rule_code", "status"} {
				delete(payload, k)
			}
			return &caapp.NotificationRuleView{
				NotificationRuleID: id,
				RuleCode:           code,
				Status:             "active",
				Payload:            payload,
				UpdatedAt:          time.Now().UTC(),
			}, nil
		}
	}
	return nil, nil
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
	if rule == nil {
		rule = map[string]any{}
	}
	rule = cloneAnyMap(rule)
	if _, ok := rule["notification_rule_id"]; !ok {
		rule["notification_rule_id"] = fmt.Sprintf("nr_%d", len(r.notificationRules)+1)
	}
	r.notificationRules = append(r.notificationRules, rule)
	return nil
}

func cloneAnyMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func strFromAnyMap(m map[string]any, key string) (string, bool) {
	v, ok := m[key]
	if !ok || v == nil {
		return "", false
	}
	s, ok := v.(string)
	return strings.TrimSpace(s), ok && strings.TrimSpace(s) != ""
}

func (r *AdminRepository) ListNotificationRules(_ context.Context, companyID string) ([]caapp.NotificationRuleView, error) {
	companyID = strings.TrimSpace(companyID)
	if companyID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "company context required", nil)
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]caapp.NotificationRuleView, 0)
	for _, rule := range r.notificationRules {
		cid, ok := strFromAnyMap(rule, "company_id")
		if !ok || cid != companyID {
			continue
		}
		id, _ := strFromAnyMap(rule, "notification_rule_id")
		code, _ := strFromAnyMap(rule, "rule_code")
		st, _ := strFromAnyMap(rule, "status")
		if st == "" {
			st = "active"
		}
		payload := map[string]any{}
		if p, ok := rule["payload"].(map[string]any); ok {
			payload = cloneAnyMap(p)
		} else {
			for k, v := range rule {
				if k == "company_id" || k == "rule_code" || k == "status" || k == "notification_rule_id" {
					continue
				}
				payload[k] = v
			}
		}
		out = append(out, caapp.NotificationRuleView{
			NotificationRuleID: id,
			RuleCode:           code,
			Status:             st,
			Payload:            payload,
			UpdatedAt:          time.Now().UTC(),
		})
	}
	return out, nil
}

func (r *AdminRepository) UpdateNotificationRuleMerged(_ context.Context, companyID, ruleID string, payloadPatch map[string]any, status *string) error {
	companyID = strings.TrimSpace(companyID)
	ruleID = strings.TrimSpace(ruleID)
	if companyID == "" || ruleID == "" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "company_id and notification_rule_id required", nil)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, rule := range r.notificationRules {
		cid, ok := strFromAnyMap(rule, "company_id")
		if !ok || cid != companyID {
			continue
		}
		id, _ := strFromAnyMap(rule, "notification_rule_id")
		if id != ruleID {
			continue
		}
		base := map[string]any{}
		if p, ok := rule["payload"].(map[string]any); ok {
			base = cloneAnyMap(p)
		} else {
			for k, v := range rule {
				if k == "company_id" || k == "rule_code" || k == "status" || k == "notification_rule_id" || k == "payload" {
					continue
				}
				base[k] = v
			}
		}
		if len(payloadPatch) > 0 {
			mergeJSONObjectsInMem(base, payloadPatch)
		}
		rule["payload"] = base
		if status != nil && strings.TrimSpace(*status) != "" {
			rule["status"] = strings.TrimSpace(*status)
		}
		r.notificationRules[i] = rule
		return nil
	}
	return perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "notification rule not found", nil)
}

func mergeJSONObjectsInMem(dst map[string]any, src map[string]any) {
	for k, v := range src {
		dstMap, ok1 := dst[k].(map[string]any)
		srcMap, ok2 := v.(map[string]any)
		if ok1 && ok2 {
			mergeJSONObjectsInMem(dstMap, srcMap)
			continue
		}
		dst[k] = v
	}
}

func (r *AdminRepository) DeleteNotificationRule(_ context.Context, companyID, ruleID string) error {
	companyID = strings.TrimSpace(companyID)
	ruleID = strings.TrimSpace(ruleID)
	if companyID == "" || ruleID == "" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "company_id and notification_rule_id required", nil)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	next := r.notificationRules[:0]
	found := false
	for _, rule := range r.notificationRules {
		cid, ok := strFromAnyMap(rule, "company_id")
		if ok && cid == companyID {
			id, _ := strFromAnyMap(rule, "notification_rule_id")
			if id == ruleID {
				found = true
				continue
			}
		}
		next = append(next, rule)
	}
	if !found {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "notification rule not found", nil)
	}
	r.notificationRules = next
	return nil
}

func (r *AdminRepository) GetAdminAccountSettings(_ context.Context, userID string) (*caapp.AdminAccountSettingsView, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.users[strings.TrimSpace(userID)]
	if !ok {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "user not found", nil)
	}
	return &caapp.AdminAccountSettingsView{
		UserID:        u.UserID,
		LoginID:       u.LoginID,
		FullName:      u.FullName,
		Email:         u.Email,
		Phone:         u.Phone,
		AccountStatus: u.AccountStatus,
	}, nil
}

func (r *AdminRepository) PatchAdminAccountSettings(_ context.Context, userID string, fullName, email, phone *string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "user_id required", nil)
	}
	if fullName == nil && email == nil && phone == nil {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "no fields to update", nil)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.users[userID]
	if !ok {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "user not found", nil)
	}
	if fullName != nil {
		u.FullName = strings.TrimSpace(*fullName)
	}
	if email != nil {
		u.Email = strings.TrimSpace(strings.ToLower(*email))
	}
	if phone != nil {
		u.Phone = strings.TrimSpace(*phone)
	}
	r.users[userID] = u
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

func (r *AdminRepository) GetCompanyPlatform(_ context.Context, companyID string) (*caapp.PlatformCompanyDetail, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if c, ok := r.companies[companyID]; ok {
		cp := *c
		return &cp, nil
	}
	return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "company not found", nil)
}

func (r *AdminRepository) UpdateCompanyPlatform(_ context.Context, req caapp.UpdatePlatformCompanyRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.companies[req.CompanyID]
	if !ok {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "company not found", nil)
	}
	if req.CompanyName != nil {
		c.CompanyName = *req.CompanyName
	}
	if req.TaxCode != nil {
		c.TaxCode = *req.TaxCode
	}
	if req.RegistrationNumber != nil {
		c.RegistrationNumber = *req.RegistrationNumber
	}
	if req.Address != nil {
		c.Address = *req.Address
	}
	if req.Phone != nil {
		c.Phone = *req.Phone
	}
	if req.ContactEmail != nil {
		c.ContactEmail = *req.ContactEmail
	}
	if req.RepresentativeName != nil {
		c.RepresentativeName = *req.RepresentativeName
	}
	if req.IsListed != nil {
		c.IsListed = *req.IsListed
	}
	if req.IsLargePublic != nil {
		c.IsLargePublic = *req.IsLargePublic
	}
	if req.IsNonLargePublic != nil {
		c.IsNonLargePublic = *req.IsNonLargePublic
	}
	if req.HasSubsidiaries != nil {
		c.HasSubsidiaries = *req.HasSubsidiaries
	}
	if req.HasSubordinateAccountingUnits != nil {
		c.HasSubordinateAccountingUnits = *req.HasSubordinateAccountingUnits
	}
	if req.BusinessSector != nil {
		c.BusinessSector = strings.TrimSpace(*req.BusinessSector)
	}
	return nil
}

func (r *AdminRepository) SetCompanyStatusPlatform(_ context.Context, _, _ string) error {
	return nil
}

func (r *AdminRepository) InsertDirectPermission(_ context.Context, membershipID, _, permCode, grantedBy string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := membershipID + ":" + permCode
	r.directPermissions[key] = caapp.DirectPermissionView{
		PermissionCode: permCode,
		GrantedBy:      grantedBy,
		GrantedAt:      "now",
	}
	return nil
}

func (r *AdminRepository) RevokeDirectPermission(_ context.Context, membershipID, permCode, _ string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := membershipID + ":" + permCode
	if _, ok := r.directPermissions[key]; !ok {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "active direct permission grant not found", nil)
	}
	delete(r.directPermissions, key)
	return nil
}

func (r *AdminRepository) ListActiveDirectPermissions(_ context.Context, membershipID string) ([]caapp.DirectPermissionView, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []caapp.DirectPermissionView
	for key, v := range r.directPermissions {
		if strings.HasPrefix(key, membershipID+":") {
			out = append(out, v)
		}
	}
	if out == nil {
		out = []caapp.DirectPermissionView{}
	}
	return out, nil
}

func (r *AdminRepository) MembershipHasPermissionFromRole(_ context.Context, membershipID, _ string, permissionCode string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for roleID := range r.rolesByMembership[membershipID] {
		if perms, ok := r.rolePermissions[roleID]; ok {
			if _, has := perms[permissionCode]; has {
				return true, nil
			}
		}
	}
	return false, nil
}

func (r *AdminRepository) HasActiveDirectPermission(_ context.Context, membershipID, permissionCode string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.directPermissions[membershipID+":"+permissionCode]
	return ok, nil
}

func (r *AdminRepository) ListDepartmentIDsByHeadMembership(_ context.Context, companyID, headMembershipID string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []string
	for id, d := range r.departments {
		_ = companyID
		if d.HeadMembershipID != nil && *d.HeadMembershipID == headMembershipID {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out, nil
}

func (r *AdminRepository) MembershipInAnyDepartment(_ context.Context, membershipID string, departmentIDs []string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	depts := r.departmentsByMembership[membershipID]
	for _, want := range departmentIDs {
		if _, ok := depts[want]; ok {
			return true, nil
		}
	}
	return false, nil
}

func (r *AdminRepository) GetMembershipIDForUserCompany(_ context.Context, userID, companyID string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for id, m := range r.memberships {
		if m.UserID == userID && m.CompanyID == companyID {
			return id, nil
		}
	}
	return "", nil
}

// Department CRUD — in-memory stubs (used by unit tests; MySQL impl is the production path).

func (r *AdminRepository) MembershipBelongsToCompany(_ context.Context, membershipID, companyID string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.memberships[membershipID]
	if !ok {
		return false, nil
	}
	return m.CompanyID == companyID, nil
}

func (r *AdminRepository) ListCompanyDepartments(_ context.Context, _ string) ([]caapp.DepartmentView, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]caapp.DepartmentView, 0, len(r.departments))
	for _, d := range r.departments {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DepartmentID < out[j].DepartmentID })
	return out, nil
}

func (r *AdminRepository) ListDepartmentTeams(_ context.Context, _, _ string) ([]caapp.TeamView, error) {
	return []caapp.TeamView{}, nil
}

func (r *AdminRepository) CreateTeamRow(_ context.Context, companyID, departmentID, teamID, name string) (*caapp.TeamView, error) {
	return &caapp.TeamView{TeamID: teamID, DepartmentID: departmentID, Name: name, Status: "active"}, nil
}

func (r *AdminRepository) PatchTeamRow(_ context.Context, _, teamID string, name *string, status *string) (*caapp.TeamView, error) {
	v := &caapp.TeamView{TeamID: teamID, Status: "active"}
	if name != nil {
		v.Name = *name
	}
	if status != nil {
		v.Status = *status
	}
	return v, nil
}

func (r *AdminRepository) DeleteTeamRow(_ context.Context, _, _ string) error { return nil }

func (r *AdminRepository) CountTeamsInDepartment(_ context.Context, _ string) (int, error) {
	return 0, nil
}

func (r *AdminRepository) AddTeamMember(_ context.Context, _, _, _ string) error { return nil }

func (r *AdminRepository) RemoveTeamMember(_ context.Context, _, _, _ string) error { return nil }

func (r *AdminRepository) MemberBelongsToDepartment(_ context.Context, _, _ string) (bool, error) {
	return true, nil
}

func (r *AdminRepository) CreateDepartmentRow(_ context.Context, companyID, deptID, _, name string, headMembershipID *string, sortOrder int) (*caapp.DepartmentView, error) {
	v := &caapp.DepartmentView{
		DepartmentID:     deptID,
		DepartmentName:   name,
		Name:             name,
		HeadMembershipID: headMembershipID,
		Status:           "active",
		SortOrder:        sortOrder,
	}
	return v, nil
}

func (r *AdminRepository) PatchDepartmentRow(_ context.Context, _, deptID string, name *string, headMembershipID *string, clearHead bool, sortOrder *int, status *string) (*caapp.DepartmentView, error) {
	v := &caapp.DepartmentView{DepartmentID: deptID, Status: "active"}
	if name != nil {
		v.Name = *name
		v.DepartmentName = *name
	}
	if !clearHead && headMembershipID != nil {
		v.HeadMembershipID = headMembershipID
	}
	if sortOrder != nil {
		v.SortOrder = *sortOrder
	}
	if status != nil {
		v.Status = *status
	}
	return v, nil
}

func (r *AdminRepository) SoftDeleteDepartment(_ context.Context, _, _ string) error {
	return nil
}

func (r *AdminRepository) CountDepartmentMembers(_ context.Context, _ string) (int, error) {
	return 0, nil
}

// Title CRUD — in-memory stubs (used by unit tests; MySQL impl is the production path).

func (r *AdminRepository) ListCompanyTitles(_ context.Context, _ string) ([]caapp.TitleView, error) {
	return []caapp.TitleView{}, nil
}

func (r *AdminRepository) CreateTitleRow(_ context.Context, _, titleID, _, name string, sortOrder int) (*caapp.TitleView, error) {
	v := &caapp.TitleView{
		TitleID:   titleID,
		TitleName: name,
		Name:      name,
		Status:    "active",
		SortOrder: sortOrder,
	}
	return v, nil
}

func (r *AdminRepository) PatchTitleRow(_ context.Context, _, titleID string, name *string, sortOrder *int, status *string) (*caapp.TitleView, error) {
	v := &caapp.TitleView{TitleID: titleID, Status: "active"}
	if name != nil {
		v.Name = *name
		v.TitleName = *name
	}
	if sortOrder != nil {
		v.SortOrder = *sortOrder
	}
	if status != nil {
		v.Status = *status
	}
	return v, nil
}

func (r *AdminRepository) SoftDeleteTitle(_ context.Context, _, _ string) error {
	return nil
}

func (r *AdminRepository) CountTitleMembers(_ context.Context, _ string) (int, error) {
	return 0, nil
}

func (r *AdminRepository) SetMembershipPrimaryAdmin(_ context.Context, membershipID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if m, ok := r.memberships[membershipID]; ok {
		m.IsPrimaryAdmin = true
		r.memberships[membershipID] = m
	}
	return nil
}

func (r *AdminRepository) ClearMembershipPrimaryAdmin(_ context.Context, membershipID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if m, ok := r.memberships[membershipID]; ok {
		m.IsPrimaryAdmin = false
		r.memberships[membershipID] = m
	}
	return nil
}

func (r *AdminRepository) GetMembershipByID(_ context.Context, membershipID string) (*caapp.MembershipView, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.memberships[membershipID]
	if !ok {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "membership not found", nil)
	}
	cp := m
	return &cp, nil
}

func (r *AdminRepository) CountAdminsInCompany(_ context.Context, companyID string) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	count := 0
	for _, m := range r.memberships {
		if m.CompanyID != companyID || m.Status != "active" {
			continue
		}
		if roles, ok := r.rolesByMembership[m.MembershipID]; ok {
			for id := range roles {
				if strings.Contains(id, "company_admin") {
					count++
					break
				}
			}
		}
	}
	return count, nil
}
