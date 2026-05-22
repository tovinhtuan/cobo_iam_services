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

	invitationsByUser map[string][]string // stacked token hashes for in-mem sanity (minimal)
	directPermissions map[string]caapp.DirectPermissionView // key: membershipID:permCode

	companies map[string]*caapp.PlatformCompanyDetail
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
		companies:               map[string]*caapp.PlatformCompanyDetail{},
	}
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
	return []caapp.DepartmentView{}, nil
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
