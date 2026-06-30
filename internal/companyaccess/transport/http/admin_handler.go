package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	"github.com/cobo/cobo_iam_services/internal/iam/loginpassword"
	marketapp "github.com/cobo/cobo_iam_services/internal/marketreference/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
	"github.com/cobo/cobo_iam_services/internal/platform/idempotency"
)

type AdminHandler struct {
	svc                         caapp.AdminService
	inspector                   iamapp.TokenInspector
	tokenIssuer                 iamapp.TokenIssuer
	sessions                    iamapp.SessionRepository
	audit                       auditapp.Service
	iamSvc                      iamapp.Service
	loginPWD                    *loginpassword.Service
	idem                        idempotency.Store
	requireProvisionIdempotency bool
	selfCreateEnabled           bool
	listedLookup                *marketapp.Service // nil → 503 on lookup endpoint
}

func NewAdminHandler(svc caapp.AdminService, inspector iamapp.TokenInspector, audit auditapp.Service) *AdminHandler {
	return &AdminHandler{svc: svc, inspector: inspector, audit: audit}
}

// WithTokenIssuer wires token issuing for POST /api/v1/company/initialize.
func (h *AdminHandler) WithTokenIssuer(t iamapp.TokenIssuer, s iamapp.SessionRepository) {
	h.tokenIssuer = t
	h.sessions = s
}

// WithAccountPasswordChange wires IAM change-password (RSA cipher required when loginPWD set).
func (h *AdminHandler) WithAccountPasswordChange(iam iamapp.Service, lp *loginpassword.Service) {
	h.iamSvc = iam
	h.loginPWD = lp
}

// WithIdempotency wires idempotency store for company self-service provision.
func (h *AdminHandler) WithIdempotency(store idempotency.Store, requireProvisionIdempotency bool) {
	h.idem = store
	h.requireProvisionIdempotency = requireProvisionIdempotency
}

// WithSelfCreateEnabled gates POST /api/v1/company/create (COMPANY_SELF_CREATE_ENABLED).
func (h *AdminHandler) WithSelfCreateEnabled(enabled bool) {
	h.selfCreateEnabled = enabled
}

// WithListedCompaniesLookup wires the vnstock lookup service for GET /api/v1/company/listed-lookup.
// When nil or not called, the endpoint returns 503.
func (h *AdminHandler) WithListedCompaniesLookup(svc *marketapp.Service) {
	h.listedLookup = svc
}

func (h *AdminHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/admin/users", h.createUser)
	mux.HandleFunc("POST /api/v1/admin/memberships", h.createMembership)
	mux.HandleFunc("PATCH /api/v1/admin/memberships/{membership_id}", h.updateMembership)
	mux.HandleFunc("DELETE /api/v1/admin/memberships/{membership_id}", h.deleteMembership)
	mux.HandleFunc("GET /api/v1/admin/companies/{company_id}/memberships", h.listMemberships)
	mux.HandleFunc("POST /api/v1/admin/memberships/{membership_id}/roles", h.assignRole)
	mux.HandleFunc("DELETE /api/v1/admin/memberships/{membership_id}/roles/{role_id}", h.removeRole)
	mux.HandleFunc("POST /api/v1/admin/memberships/{membership_id}/departments", h.assignDepartment)
	mux.HandleFunc("DELETE /api/v1/admin/memberships/{membership_id}/departments/{department_id}", h.removeDepartment)
	mux.HandleFunc("POST /api/v1/admin/memberships/{membership_id}/titles", h.assignTitle)
	mux.HandleFunc("DELETE /api/v1/admin/memberships/{membership_id}/titles/{title_id}", h.removeTitle)
	mux.HandleFunc("GET /api/v1/admin/permissions", h.listPermissions)
	mux.HandleFunc("GET /api/v1/admin/roles", h.listRoles)
	mux.HandleFunc("GET /api/v1/admin/roles/{role_id}/permissions", h.listRolePermissions)
	mux.HandleFunc("POST /api/v1/admin/roles/{role_id}/permissions", h.assignRolePermission)
	mux.HandleFunc("DELETE /api/v1/admin/roles/{role_id}/permissions/{permission_id}", h.removeRolePermission)
	mux.HandleFunc("POST /api/v1/admin/resource-scope-rules", h.createResourceScopeRule)
	mux.HandleFunc("POST /api/v1/admin/workflow-assignee-rules", h.createWorkflowAssigneeRule)
	mux.HandleFunc("POST /api/v1/admin/notification-rules", h.createNotificationRule)
	mux.HandleFunc("POST /api/v1/admin/notification-rules/simulate", h.simulateNotificationRule)
	mux.HandleFunc("GET /api/v1/admin/notification-rules/status", h.getNotificationRuleStatus)
	mux.HandleFunc("GET /api/v1/admin/notification-rules", h.listNotificationRules)
	mux.HandleFunc("PATCH /api/v1/admin/notification-rules/{notification_rule_id}", h.patchNotificationRule)
	mux.HandleFunc("DELETE /api/v1/admin/notification-rules/{notification_rule_id}", h.deleteNotificationRule)
	mux.HandleFunc("GET /api/v1/admin/account/settings", h.getAdminAccountSettings)
	mux.HandleFunc("PATCH /api/v1/admin/account/settings", h.patchAdminAccountSettings)
	mux.HandleFunc("POST /api/v1/admin/account/change-password", h.changeAccountPassword)
	mux.HandleFunc("GET /api/v1/admin/company", h.getOwnCompany)
	mux.HandleFunc("PATCH /api/v1/admin/company", h.patchOwnCompany)
	mux.HandleFunc("POST /api/v1/admin/users/invite", h.inviteUser)
	mux.HandleFunc("GET /api/v1/admin/users/invite-scope", h.getInviteScope)
	mux.HandleFunc("GET /api/v1/admin/invite-roles", h.listInviteRoles)
	mux.HandleFunc("POST /api/v1/admin/users/{user_id}/resend-invitation", h.resendInvitation)
	mux.HandleFunc("POST /api/v1/admin/memberships/{membership_id}/permissions", h.addMemberPermission)
	mux.HandleFunc("DELETE /api/v1/admin/memberships/{membership_id}/permissions/{permission_code}", h.removeMemberPermission)
	mux.HandleFunc("GET /api/v1/admin/memberships/{membership_id}/permissions", h.listMemberPermissions)

	// Department CRUD
	mux.HandleFunc("GET /api/v1/admin/departments", h.listDepartments)
	mux.HandleFunc("POST /api/v1/admin/departments", h.createDepartment)
	mux.HandleFunc("PATCH /api/v1/admin/departments/{department_id}", h.updateDepartment)
	mux.HandleFunc("DELETE /api/v1/admin/departments/{department_id}", h.deleteDepartment)
	mux.HandleFunc("POST /api/v1/admin/departments/{department_id}/members", h.addDeptMember)
	mux.HandleFunc("DELETE /api/v1/admin/departments/{department_id}/members/{membership_id}", h.removeDeptMember)

	// Team CRUD (org_units)
	mux.HandleFunc("GET /api/v1/admin/departments/{department_id}/teams", h.listTeams)
	mux.HandleFunc("POST /api/v1/admin/departments/{department_id}/teams", h.createTeam)
	mux.HandleFunc("PATCH /api/v1/admin/teams/{team_id}", h.updateTeam)
	mux.HandleFunc("DELETE /api/v1/admin/teams/{team_id}", h.deleteTeam)
	mux.HandleFunc("POST /api/v1/admin/teams/{team_id}/members", h.addTeamMember)
	mux.HandleFunc("DELETE /api/v1/admin/teams/{team_id}/members/{membership_id}", h.removeTeamMember)

	// Title CRUD
	mux.HandleFunc("GET /api/v1/admin/titles", h.listTitles)
	mux.HandleFunc("POST /api/v1/admin/titles", h.createTitle)
	mux.HandleFunc("PATCH /api/v1/admin/titles/{title_id}", h.updateTitle)
	mux.HandleFunc("DELETE /api/v1/admin/titles/{title_id}", h.deleteTitle)
	mux.HandleFunc("POST /api/v1/admin/titles/{title_id}/members", h.addTitleMember)
	mux.HandleFunc("DELETE /api/v1/admin/titles/{title_id}/members/{membership_id}", h.removeTitleMember)

	// Company admin assignment and transfer-ownership
	mux.HandleFunc("POST /api/v1/admin/company/admins", h.assignCompanyAdmin)
	mux.HandleFunc("DELETE /api/v1/admin/company/admins/{membership_id}", h.revokeCompanyAdmin)
	mux.HandleFunc("POST /api/v1/admin/company/transfer-ownership", h.transferOwnership)

	// Company self-service provision (no rbac.manage required)
	mux.HandleFunc("POST /api/v1/company/initialize", h.initializeCompany)
	mux.HandleFunc("POST /api/v1/company/create", h.createSelfServiceCompany)

	// Public lookup — no auth required
	mux.HandleFunc("GET /api/v1/company/listed-lookup", h.listedLookupByBusinessCode)
}

func (h *AdminHandler) createUser(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var p struct {
		LoginID          string   `json:"login_id"`
		Password         string   `json:"password"`
		FullName         string   `json:"full_name"`
		Email            string   `json:"email"`
		Phone            string   `json:"phone"`
		AccountStatus    string   `json:"account_status"`
		CompanyID        string   `json:"company_id"`        // optional: empty = user without membership (when caller has rbac.manage)
		MembershipStatus string   `json:"membership_status"` // when company_id set
		RoleID           string   `json:"role_id"`
		RoleCode         string   `json:"role_code"`
		Permissions      []string `json:"permissions"`
		DepartmentID     string   `json:"department_id"`
		TitleID          string   `json:"title_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&p)
	resp, err := h.svc.CreateUser(r.Context(), caapp.CreateUserRequest{
		Subject:          sub,
		LoginID:          p.LoginID,
		Password:         p.Password,
		FullName:         p.FullName,
		Email:            p.Email,
		Phone:            p.Phone,
		AccountStatus:    p.AccountStatus,
		CompanyID:        p.CompanyID,
		MembershipStatus: p.MembershipStatus,
		RoleID:           p.RoleID,
		RoleCode:         p.RoleCode,
		Permissions:      p.Permissions,
		DepartmentID:     p.DepartmentID,
		TitleID:          p.TitleID,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.user.create", "user", resp.UserID)
	if resp.MembershipID != "" {
		h.auditLog(r, "admin.membership.create", "membership", resp.MembershipID)
	}
	httpx.WriteJSON(w, http.StatusCreated, resp)
}

func (h *AdminHandler) subject(r *http.Request) (caapp.AdminSubject, error) {
	claims, err := h.inspector.InspectAccessToken(r.Context(), bearerToken(r.Header.Get("Authorization")))
	if err != nil {
		return caapp.AdminSubject{}, err
	}
	return caapp.AdminSubject{UserID: claims.Sub, MembershipID: claims.MembershipID, CompanyID: claims.CompanyID}, nil
}

func (h *AdminHandler) auditLog(r *http.Request, action, resourceType, resourceID string) {
	if h.audit == nil {
		return
	}
	sub, _ := h.subject(r)
	_ = h.audit.AppendAuditLog(r.Context(), auditapp.AppendAuditLogRequest{ActorUserID: sub.UserID, ActorMembershipID: sub.MembershipID, CompanyID: sub.CompanyID, Action: action, ResourceType: resourceType, ResourceID: resourceID, Decision: "allow", RequestID: httpx.RequestIDFromContext(r.Context()), IP: r.RemoteAddr, UserAgent: r.UserAgent()})
}

func (h *AdminHandler) createMembership(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var p struct {
		UserID    string `json:"user_id"`
		CompanyID string `json:"company_id"`
		Status    string `json:"status"`
	}
	_ = json.NewDecoder(r.Body).Decode(&p)
	resp, err := h.svc.CreateMembership(r.Context(), caapp.CreateMembershipRequest{Subject: sub, UserID: p.UserID, CompanyID: p.CompanyID, Status: p.Status})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.membership.create", "membership", resp.MembershipID)
	httpx.WriteJSON(w, http.StatusCreated, resp)
}

func (h *AdminHandler) updateMembership(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var p struct {
		Status string `json:"status"`
	}
	_ = json.NewDecoder(r.Body).Decode(&p)
	id := r.PathValue("membership_id")
	resp, err := h.svc.UpdateMembership(r.Context(), caapp.UpdateMembershipRequest{Subject: sub, MembershipID: id, Status: p.Status})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.membership.update", "membership", id)
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *AdminHandler) deleteMembership(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	id := r.PathValue("membership_id")
	if err := h.svc.DeleteMembership(r.Context(), caapp.DeleteMembershipRequest{Subject: sub, MembershipID: id}); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.membership.delete", "membership", id)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *AdminHandler) issueProvisionSession(r *http.Request, claims *iamapp.AccessTokenClaims, membershipID, companyID string) (map[string]any, error) {
	resp := map[string]any{}
	if h.tokenIssuer == nil || h.sessions == nil || claims.SessionID == "" {
		return nil, perr.NewHTTPError(http.StatusInternalServerError, perr.CodeSessionContextUpdateFailed, "session services not configured", nil)
	}
	if err := h.sessions.UpdateContext(r.Context(), claims.SessionID, membershipID, companyID); err != nil {
		return nil, perr.NewHTTPError(http.StatusInternalServerError, perr.CodeSessionContextUpdateFailed, "failed to update session context", err)
	}
	access, exp, err := h.tokenIssuer.IssueAccessToken(r.Context(), iamapp.AccessTokenClaims{
		Sub: claims.Sub, SessionID: claims.SessionID,
		MembershipID: membershipID, CompanyID: companyID,
	})
	if err != nil {
		return nil, perr.NewHTTPError(http.StatusInternalServerError, perr.CodeSessionContextUpdateFailed, "failed to issue access token", err)
	}
	if strings.TrimSpace(access) == "" {
		return nil, perr.NewHTTPError(http.StatusInternalServerError, perr.CodeSessionContextUpdateFailed, "empty access token", nil)
	}
	refresh, _ := h.tokenIssuer.IssueRefreshToken(r.Context(), claims.SessionID, claims.Sub)
	if refresh != "" {
		_ = h.sessions.RotateRefreshToken(r.Context(), claims.SessionID, refresh)
	}
	resp["session"] = map[string]any{
		"access_token":  access,
		"refresh_token": refresh,
		"expires_in":    exp,
	}
	return resp, nil
}

func (h *AdminHandler) auditProvision(r *http.Request, actorUserID, membershipID, companyID, action string) {
	if h.audit == nil {
		return
	}
	_ = h.audit.AppendAuditLog(r.Context(), auditapp.AppendAuditLogRequest{
		ActorUserID:       actorUserID,
		ActorMembershipID: membershipID,
		CompanyID:         companyID,
		Action:            action,
		ResourceType:      "company",
		ResourceID:        companyID,
		Decision:          "allow",
		RequestID:         httpx.RequestIDFromContext(r.Context()),
		IP:                r.RemoteAddr,
		UserAgent:         r.UserAgent(),
	})
}

func (h *AdminHandler) listMemberships(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	cid := r.PathValue("company_id")
	page, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("page")))
	pageSize, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("page_size")))
	items, err := h.svc.ListCompanyMemberships(r.Context(), caapp.ListCompanyMembershipsRequest{
		Subject:   sub,
		CompanyID: cid,
		Page:      page,
		PageSize:  pageSize,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"items":     items.Items,
		"total":     items.Total,
		"page":      items.Page,
		"page_size": items.PageSize,
	})
}

func (h *AdminHandler) assignRole(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	mid := r.PathValue("membership_id")
	var p struct {
		RoleID string `json:"role_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&p)
	if err := h.svc.AssignRole(r.Context(), caapp.AssignRoleRequest{Subject: sub, MembershipID: mid, RoleID: p.RoleID}); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.membership.role.assign", "membership", mid)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *AdminHandler) removeRole(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	mid := r.PathValue("membership_id")
	rid := r.PathValue("role_id")
	if err := h.svc.RemoveRole(r.Context(), caapp.RemoveRoleRequest{Subject: sub, MembershipID: mid, RoleID: rid}); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.membership.role.remove", "membership", mid)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *AdminHandler) assignDepartment(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	mid := r.PathValue("membership_id")
	var p struct {
		DepartmentID string `json:"department_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&p)
	if err := h.svc.AssignDepartment(r.Context(), caapp.AssignDepartmentRequest{Subject: sub, MembershipID: mid, DepartmentID: p.DepartmentID}); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.membership.department.assign", "membership", mid)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *AdminHandler) removeDepartment(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	mid := r.PathValue("membership_id")
	did := r.PathValue("department_id")
	if err := h.svc.RemoveDepartment(r.Context(), caapp.RemoveDepartmentRequest{Subject: sub, MembershipID: mid, DepartmentID: did}); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.membership.department.remove", "membership", mid)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *AdminHandler) assignTitle(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	mid := r.PathValue("membership_id")
	var p struct {
		TitleID string `json:"title_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&p)
	if err := h.svc.AssignTitle(r.Context(), caapp.AssignTitleRequest{Subject: sub, MembershipID: mid, TitleID: p.TitleID}); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.membership.title.assign", "membership", mid)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *AdminHandler) removeTitle(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	mid := r.PathValue("membership_id")
	tid := r.PathValue("title_id")
	if err := h.svc.RemoveTitle(r.Context(), caapp.RemoveTitleRequest{Subject: sub, MembershipID: mid, TitleID: tid}); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.membership.title.remove", "membership", mid)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *AdminHandler) listPermissions(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	items, err := h.svc.ListPermissions(r.Context(), caapp.AdminSubjectRequest{Subject: sub})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *AdminHandler) listRoles(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	items, err := h.svc.ListRoles(r.Context(), caapp.AdminSubjectRequest{Subject: sub})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *AdminHandler) listRolePermissions(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	rid := strings.TrimSpace(r.PathValue("role_id"))
	if rid == "" {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "role_id required", nil))
		return
	}
	out, err := h.svc.ListRolePermissions(r.Context(), caapp.ListRolePermissionsRequest{Subject: sub, RoleID: rid})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *AdminHandler) assignRolePermission(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	rid := r.PathValue("role_id")
	var p struct {
		PermissionID string `json:"permission_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&p)
	if err := h.svc.AssignRolePermission(r.Context(), caapp.AssignRolePermissionRequest{Subject: sub, RoleID: rid, PermissionID: p.PermissionID}); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.role.permission.assign", "role", rid)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *AdminHandler) removeRolePermission(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	rid := r.PathValue("role_id")
	pid := r.PathValue("permission_id")
	if err := h.svc.RemoveRolePermission(r.Context(), caapp.RemoveRolePermissionRequest{Subject: sub, RoleID: rid, PermissionID: pid}); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.role.permission.remove", "role", rid)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *AdminHandler) createResourceScopeRule(w http.ResponseWriter, r *http.Request) {
	h.createRule(w, r, "admin.resource_scope_rule.create", func(sub caapp.AdminSubject, payload map[string]any) error {
		return h.svc.CreateResourceScopeRule(r.Context(), caapp.CreateResourceScopeRuleRequest{Subject: sub, Payload: payload})
	})
}
func (h *AdminHandler) createWorkflowAssigneeRule(w http.ResponseWriter, r *http.Request) {
	h.createRule(w, r, "admin.workflow_assignee_rule.create", func(sub caapp.AdminSubject, payload map[string]any) error {
		return h.svc.CreateWorkflowAssigneeRule(r.Context(), caapp.CreateWorkflowAssigneeRuleRequest{Subject: sub, Payload: payload})
	})
}
func (h *AdminHandler) createNotificationRule(w http.ResponseWriter, r *http.Request) {
	h.createRule(w, r, "admin.notification_rule.create", func(sub caapp.AdminSubject, payload map[string]any) error {
		return h.svc.CreateNotificationRule(r.Context(), caapp.CreateNotificationRuleRequest{Subject: sub, Payload: payload})
	})
}

func (h *AdminHandler) listNotificationRules(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	items, err := h.svc.ListNotificationRules(r.Context(), caapp.ListNotificationRulesRequest{Subject: sub})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *AdminHandler) getNotificationRuleStatus(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	out, err := h.svc.GetNotificationRuleStatus(r.Context(), caapp.GetNotificationRuleStatusRequest{Subject: sub})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *AdminHandler) simulateNotificationRule(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var raw map[string]any
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid JSON body", nil))
		return
	}
	var body caapp.SimulateNotificationRuleBody
	rawBytes, _ := json.Marshal(raw)
	if err := json.Unmarshal(rawBytes, &body); err != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid request body", nil))
		return
	}
	out, err := h.svc.SimulateNotificationRule(r.Context(), caapp.SimulateNotificationRuleRequest{
		Subject: sub,
		Body:    body,
	}, raw)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.notification_rule.simulate", "notification_rule", "simulate")
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *AdminHandler) patchNotificationRule(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var body struct {
		Payload map[string]any `json:"payload"`
		Status  *string        `json:"status"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	ruleID := strings.TrimSpace(r.PathValue("notification_rule_id"))
	if ruleID == "" {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "notification_rule_id required", nil))
		return
	}
	if err := h.svc.UpdateNotificationRule(r.Context(), caapp.UpdateNotificationRuleRequest{
		Subject:      sub,
		RuleID:       ruleID,
		PayloadPatch: body.Payload,
		Status:       body.Status,
	}); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.notification_rule.update", "notification_rule", ruleID)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *AdminHandler) deleteNotificationRule(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	ruleID := strings.TrimSpace(r.PathValue("notification_rule_id"))
	if ruleID == "" {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "notification_rule_id required", nil))
		return
	}
	if err := h.svc.DeleteNotificationRule(r.Context(), caapp.DeleteNotificationRuleRequest{Subject: sub, RuleID: ruleID}); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.notification_rule.delete", "notification_rule", ruleID)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *AdminHandler) getAdminAccountSettings(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	out, err := h.svc.GetAdminAccountSettings(r.Context(), caapp.GetAdminAccountSettingsRequest{Subject: sub})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *AdminHandler) patchAdminAccountSettings(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var body struct {
		FullName *string `json:"full_name"`
		Email    *string `json:"email"`
		Phone    *string `json:"phone"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if err := h.svc.PatchAdminAccountSettings(r.Context(), caapp.PatchAdminAccountSettingsRequest{
		Subject:  sub,
		FullName: body.FullName,
		Email:    body.Email,
		Phone:    body.Phone,
	}); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.account.settings.patch", "user", sub.UserID)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *AdminHandler) changeAccountPassword(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if h.iamSvc == nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeInternal, "password change not configured", nil))
		return
	}
	if h.loginPWD == nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "password_cipher is required (RSA not configured)", nil))
		return
	}
	var body struct {
		CurrentPasswordCipher *iamapp.LoginPasswordCipher `json:"current_password_cipher"`
		NewPasswordCipher     *iamapp.LoginPasswordCipher `json:"new_password_cipher"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid json body", err))
		return
	}
	current, err := h.decryptPasswordCipher(body.CurrentPasswordCipher)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	newPwd, err := h.decryptPasswordCipher(body.NewPasswordCipher)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.iamSvc.ChangeAccountPassword(r.Context(), iamapp.ChangeAccountPasswordRequest{
		UserID:          sub.UserID,
		CurrentPassword: current,
		NewPassword:     newPwd,
	}); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.account.password_change", "user", sub.UserID)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *AdminHandler) decryptPasswordCipher(c *iamapp.LoginPasswordCipher) (string, error) {
	if c == nil || strings.TrimSpace(c.CiphertextB64) == "" {
		return "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "password_cipher required", nil)
	}
	alg := strings.TrimSpace(c.Alg)
	if alg != loginpassword.AlgRSAOAEP256 {
		return "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "unsupported password_cipher.alg", nil)
	}
	if strings.TrimSpace(c.KID) != "" && c.KID != h.loginPWD.KeyID() {
		return "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "unknown password_cipher.kid", nil)
	}
	plain, err := h.loginPWD.DecryptOAEP256(c.CiphertextB64)
	if err != nil {
		return "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid password_cipher", err)
	}
	return plain, nil
}

func (h *AdminHandler) createRule(w http.ResponseWriter, r *http.Request, action string, fn func(sub caapp.AdminSubject, payload map[string]any) error) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	payload := map[string]any{}
	_ = json.NewDecoder(r.Body).Decode(&payload)
	if err := fn(sub, payload); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, action, "rule", "")
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"success": true})
}

func (h *AdminHandler) getOwnCompany(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	out, err := h.svc.GetOwnCompany(r.Context(), caapp.GetOwnCompanyRequest{Subject: sub})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *AdminHandler) patchOwnCompany(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var body struct {
		CompanyName                   *string `json:"company_name"`
		TaxCode                       *string `json:"tax_code"`
		RegistrationNumber            *string `json:"registration_number"`
		Address                       *string `json:"address"`
		Phone                         *string `json:"phone"`
		ContactEmail                  *string `json:"contact_email"`
		RepresentativeName            *string `json:"representative_name"`
		IsListed                      *bool   `json:"is_listed"`
		IsLargePublic                 *bool   `json:"is_large_public"`
		IsNonLargePublic              *bool   `json:"is_non_large_public"`
		HasSubsidiaries               *bool   `json:"has_subsidiaries"`
		HasSubordinateAccountingUnits *bool   `json:"has_subordinate_accounting_units"`
		BusinessSector                *string `json:"business_sector"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	out, err := h.svc.PatchOwnCompany(r.Context(), caapp.PatchOwnCompanyRequest{
		Subject:                       sub,
		CompanyName:                   body.CompanyName,
		TaxCode:                       body.TaxCode,
		RegistrationNumber:            body.RegistrationNumber,
		Address:                       body.Address,
		Phone:                         body.Phone,
		ContactEmail:                  body.ContactEmail,
		RepresentativeName:            body.RepresentativeName,
		IsListed:                      body.IsListed,
		IsLargePublic:                 body.IsLargePublic,
		IsNonLargePublic:              body.IsNonLargePublic,
		HasSubsidiaries:               body.HasSubsidiaries,
		HasSubordinateAccountingUnits: body.HasSubordinateAccountingUnits,
		BusinessSector:                body.BusinessSector,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.company.patch", "company", sub.CompanyID)
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *AdminHandler) inviteUser(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var p struct {
		Email        string   `json:"email"`
		FullName     string   `json:"full_name"`
		RoleID       string   `json:"role_id"`
		RoleCode     string   `json:"role_code"`
		Permissions  []string `json:"permissions"`
		DepartmentID string   `json:"department_id"`
		TitleID      string   `json:"title_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&p)
	resp, err := h.svc.InviteUser(r.Context(), caapp.InviteUserRequest{
		Subject:         sub,
		Email:           p.Email,
		FullName:        p.FullName,
		CompanyID:       sub.CompanyID,
		CreatedByUserID: sub.UserID,
		RoleID:          p.RoleID,
		RoleCode:        p.RoleCode,
		Permissions:     p.Permissions,
		DepartmentID:    p.DepartmentID,
		TitleID:         p.TitleID,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.user.invite", "user", resp.UserID)
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *AdminHandler) getInviteScope(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	out, err := h.svc.GetInviteScope(r.Context(), caapp.GetInviteScopeRequest{Subject: sub})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *AdminHandler) listInviteRoles(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	items, err := h.svc.ListInviteRoles(r.Context(), caapp.ListInviteRolesRequest{Subject: sub, CompanyID: sub.CompanyID})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *AdminHandler) resendInvitation(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	userID := r.PathValue("user_id")
	if err := h.svc.ResendUserInvitation(r.Context(), caapp.ResendUserInvitationRequest{
		Subject:   sub,
		UserID:    userID,
		CompanyID: sub.CompanyID,
	}); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.user.resend_invitation", "user", userID)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *AdminHandler) addMemberPermission(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	mid := r.PathValue("membership_id")
	var p struct {
		PermissionCode string `json:"permission_code"`
	}
	_ = json.NewDecoder(r.Body).Decode(&p)
	if err := h.svc.AddDirectPermission(r.Context(), caapp.AddDirectPermissionRequest{
		Subject:        sub,
		MembershipID:   mid,
		PermissionCode: p.PermissionCode,
	}); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.membership.permission.add", "membership", mid)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *AdminHandler) removeMemberPermission(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	mid := r.PathValue("membership_id")
	permCode := r.PathValue("permission_code")
	if err := h.svc.RemoveDirectPermission(r.Context(), caapp.RemoveDirectPermissionRequest{
		Subject:        sub,
		MembershipID:   mid,
		PermissionCode: permCode,
	}); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.membership.permission.remove", "membership", mid)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *AdminHandler) listMemberPermissions(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	mid := r.PathValue("membership_id")
	items, err := h.svc.ListDirectPermissions(r.Context(), caapp.ListDirectPermissionsRequest{
		Subject:      sub,
		MembershipID: mid,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *AdminHandler) listDepartments(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	items, err := h.svc.ListDepartments(r.Context(), caapp.ListDepartmentsRequest{Subject: sub})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *AdminHandler) createDepartment(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var p struct {
		Name             string  `json:"name"`
		HeadMembershipID *string `json:"head_membership_id"`
		SortOrder        int     `json:"sort_order"`
	}
	_ = json.NewDecoder(r.Body).Decode(&p)
	out, err := h.svc.CreateDepartment(r.Context(), caapp.CreateDepartmentRequest{
		Subject:          sub,
		Name:             p.Name,
		HeadMembershipID: p.HeadMembershipID,
		SortOrder:        p.SortOrder,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.department.create", "department", out.DepartmentID)
	httpx.WriteJSON(w, http.StatusCreated, out)
}

func (h *AdminHandler) updateDepartment(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	deptID := r.PathValue("department_id")
	var p struct {
		Name             *string `json:"name"`
		HeadMembershipID *string `json:"head_membership_id"`
		SortOrder        *int    `json:"sort_order"`
		Status           *string `json:"status"`
	}
	_ = json.NewDecoder(r.Body).Decode(&p)
	out, err := h.svc.UpdateDepartment(r.Context(), caapp.UpdateDepartmentRequest{
		Subject:          sub,
		DepartmentID:     deptID,
		Name:             p.Name,
		HeadMembershipID: p.HeadMembershipID,
		SortOrder:        p.SortOrder,
		Status:           p.Status,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.department.update", "department", deptID)
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *AdminHandler) deleteDepartment(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	deptID := r.PathValue("department_id")
	if err := h.svc.DeleteDepartment(r.Context(), caapp.DeleteDepartmentRequest{
		Subject:      sub,
		DepartmentID: deptID,
	}); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.department.delete", "department", deptID)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *AdminHandler) addDeptMember(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	deptID := r.PathValue("department_id")
	var p struct {
		MembershipID string `json:"membership_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&p)
	if err := h.svc.AddDeptMember(r.Context(), caapp.AddDeptMemberRequest{
		Subject:      sub,
		DepartmentID: deptID,
		MembershipID: p.MembershipID,
	}); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.department.member.add", "department", deptID)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *AdminHandler) removeDeptMember(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	deptID := r.PathValue("department_id")
	membershipID := r.PathValue("membership_id")
	if err := h.svc.RemoveDeptMember(r.Context(), caapp.RemoveDeptMemberRequest{
		Subject:      sub,
		DepartmentID: deptID,
		MembershipID: membershipID,
	}); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.department.member.remove", "department", deptID)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *AdminHandler) listTitles(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	items, err := h.svc.ListTitles(r.Context(), caapp.ListTitlesRequest{Subject: sub})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *AdminHandler) createTitle(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var p struct {
		Name      string `json:"name"`
		SortOrder int    `json:"sort_order"`
	}
	_ = json.NewDecoder(r.Body).Decode(&p)
	out, err := h.svc.CreateTitle(r.Context(), caapp.CreateTitleRequest{
		Subject:   sub,
		Name:      p.Name,
		SortOrder: p.SortOrder,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.title.create", "title", out.TitleID)
	httpx.WriteJSON(w, http.StatusCreated, out)
}

func (h *AdminHandler) updateTitle(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	titleID := r.PathValue("title_id")
	var p struct {
		Name      *string `json:"name"`
		SortOrder *int    `json:"sort_order"`
		Status    *string `json:"status"`
	}
	_ = json.NewDecoder(r.Body).Decode(&p)
	out, err := h.svc.UpdateTitle(r.Context(), caapp.UpdateTitleRequest{
		Subject:   sub,
		TitleID:   titleID,
		Name:      p.Name,
		SortOrder: p.SortOrder,
		Status:    p.Status,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.title.update", "title", titleID)
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *AdminHandler) deleteTitle(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	titleID := r.PathValue("title_id")
	if err := h.svc.DeleteTitle(r.Context(), caapp.DeleteTitleRequest{
		Subject: sub,
		TitleID: titleID,
	}); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.title.delete", "title", titleID)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *AdminHandler) addTitleMember(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	titleID := r.PathValue("title_id")
	var p struct {
		MembershipID string `json:"membership_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&p)
	if err := h.svc.AddTitleMember(r.Context(), caapp.AddTitleMemberRequest{
		Subject:      sub,
		TitleID:      titleID,
		MembershipID: p.MembershipID,
	}); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.title.member.add", "title", titleID)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *AdminHandler) removeTitleMember(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	titleID := r.PathValue("title_id")
	membershipID := r.PathValue("membership_id")
	if err := h.svc.RemoveTitleMember(r.Context(), caapp.RemoveTitleMemberRequest{
		Subject:      sub,
		TitleID:      titleID,
		MembershipID: membershipID,
	}); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.title.member.remove", "title", titleID)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *AdminHandler) listTeams(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	deptID := r.PathValue("department_id")
	items, err := h.svc.ListDepartmentTeams(r.Context(), caapp.ListDepartmentTeamsRequest{Subject: sub, DepartmentID: deptID})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *AdminHandler) createTeam(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	deptID := r.PathValue("department_id")
	var p struct {
		Name string `json:"name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&p)
	out, err := h.svc.CreateTeam(r.Context(), caapp.CreateTeamRequest{Subject: sub, DepartmentID: deptID, Name: p.Name})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.team.create", "team", out.TeamID)
	httpx.WriteJSON(w, http.StatusCreated, out)
}

func (h *AdminHandler) updateTeam(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	teamID := r.PathValue("team_id")
	var p struct {
		Name   *string `json:"name"`
		Status *string `json:"status"`
	}
	_ = json.NewDecoder(r.Body).Decode(&p)
	out, err := h.svc.UpdateTeam(r.Context(), caapp.UpdateTeamRequest{Subject: sub, TeamID: teamID, Name: p.Name, Status: p.Status})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.team.update", "team", teamID)
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *AdminHandler) deleteTeam(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	teamID := r.PathValue("team_id")
	if err := h.svc.DeleteTeam(r.Context(), caapp.DeleteTeamRequest{Subject: sub, TeamID: teamID}); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.team.delete", "team", teamID)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *AdminHandler) addTeamMember(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	teamID := r.PathValue("team_id")
	var p struct {
		MembershipID string `json:"membership_id"`
		DepartmentID string `json:"department_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&p)
	if err := h.svc.AddTeamMember(r.Context(), caapp.AddTeamMemberRequest{
		Subject:      sub,
		TeamID:       teamID,
		DepartmentID: p.DepartmentID,
		MembershipID: p.MembershipID,
	}); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.team.member.add", "team", teamID)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *AdminHandler) removeTeamMember(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	teamID := r.PathValue("team_id")
	membershipID := r.PathValue("membership_id")
	if err := h.svc.RemoveTeamMember(r.Context(), caapp.RemoveTeamMemberRequest{
		Subject:      sub,
		TeamID:       teamID,
		MembershipID: membershipID,
	}); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.team.member.remove", "team", teamID)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *AdminHandler) assignCompanyAdmin(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var body struct {
		MembershipID string `json:"membership_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if err := h.svc.AssignCompanyAdmin(r.Context(), caapp.AssignCompanyAdminRequest{
		Subject:      sub,
		MembershipID: body.MembershipID,
	}); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.company.admin.assign", "membership", body.MembershipID)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *AdminHandler) revokeCompanyAdmin(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	membershipID := r.PathValue("membership_id")
	if err := h.svc.RevokeCompanyAdmin(r.Context(), caapp.RevokeCompanyAdminRequest{
		Subject:      sub,
		MembershipID: membershipID,
	}); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.company.admin.revoke", "membership", membershipID)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *AdminHandler) transferOwnership(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var body struct {
		TargetMembershipID string `json:"target_membership_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if err := h.svc.TransferOwnership(r.Context(), caapp.TransferOwnershipRequest{
		Subject:            sub,
		TargetMembershipID: body.TargetMembershipID,
	}); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.company.ownership.transfer", "membership", body.TargetMembershipID)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func bearerToken(h string) string {
	h = strings.TrimSpace(h)
	if h == "" {
		return ""
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
		return strings.TrimSpace(parts[1])
	}
	return h
}
