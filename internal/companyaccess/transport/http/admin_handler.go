package http

import (
	"encoding/json"
	"net/http"
	"strings"

	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
)

type AdminHandler struct {
	svc       caapp.AdminService
	inspector iamapp.TokenInspector
	audit     auditapp.Service
}

func NewAdminHandler(svc caapp.AdminService, inspector iamapp.TokenInspector, audit auditapp.Service) *AdminHandler {
	return &AdminHandler{svc: svc, inspector: inspector, audit: audit}
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
	mux.HandleFunc("POST /api/v1/admin/roles/{role_id}/permissions", h.assignRolePermission)
	mux.HandleFunc("DELETE /api/v1/admin/roles/{role_id}/permissions/{permission_id}", h.removeRolePermission)
	mux.HandleFunc("POST /api/v1/admin/resource-scope-rules", h.createResourceScopeRule)
	mux.HandleFunc("POST /api/v1/admin/workflow-assignee-rules", h.createWorkflowAssigneeRule)
	mux.HandleFunc("POST /api/v1/admin/notification-rules", h.createNotificationRule)
	mux.HandleFunc("GET /api/v1/admin/notification-rules", h.listNotificationRules)
	mux.HandleFunc("PATCH /api/v1/admin/notification-rules/{notification_rule_id}", h.patchNotificationRule)
	mux.HandleFunc("DELETE /api/v1/admin/notification-rules/{notification_rule_id}", h.deleteNotificationRule)
	mux.HandleFunc("GET /api/v1/admin/account/settings", h.getAdminAccountSettings)
	mux.HandleFunc("PATCH /api/v1/admin/account/settings", h.patchAdminAccountSettings)
	mux.HandleFunc("GET /api/v1/admin/company", h.getOwnCompany)
	mux.HandleFunc("PATCH /api/v1/admin/company", h.patchOwnCompany)
	mux.HandleFunc("POST /api/v1/admin/users/invite", h.inviteUser)
	mux.HandleFunc("GET /api/v1/admin/invite-roles", h.listInviteRoles)
	mux.HandleFunc("POST /api/v1/admin/users/{user_id}/resend-invitation", h.resendInvitation)
	mux.HandleFunc("POST /api/v1/admin/memberships/{membership_id}/permissions", h.addMemberPermission)
	mux.HandleFunc("DELETE /api/v1/admin/memberships/{membership_id}/permissions/{permission_code}", h.removeMemberPermission)
	mux.HandleFunc("GET /api/v1/admin/memberships/{membership_id}/permissions", h.listMemberPermissions)
}

func (h *AdminHandler) createUser(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var p struct {
		LoginID       string `json:"login_id"`
		Password      string `json:"password"`
		FullName      string `json:"full_name"`
		Email         string `json:"email"`
		Phone         string `json:"phone"`
		AccountStatus string `json:"account_status"`
		CompanyID        string `json:"company_id"`        // optional: empty = user without membership (when caller has rbac.manage)
		MembershipStatus string `json:"membership_status"` // when company_id set
	}
	_ = json.NewDecoder(r.Body).Decode(&p)
	resp, err := h.svc.CreateUser(r.Context(), caapp.CreateUserRequest{
		Subject:       sub,
		LoginID:       p.LoginID,
		Password:      p.Password,
		FullName:      p.FullName,
		Email:         p.Email,
		Phone:         p.Phone,
		AccountStatus: p.AccountStatus,
		CompanyID:        p.CompanyID,
		MembershipStatus: p.MembershipStatus,
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

func (h *AdminHandler) listMemberships(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	cid := r.PathValue("company_id")
	items, err := h.svc.ListCompanyMemberships(r.Context(), caapp.ListCompanyMembershipsRequest{Subject: sub, CompanyID: cid})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
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
		CompanyName        *string `json:"company_name"`
		TaxCode            *string `json:"tax_code"`
		RegistrationNumber *string `json:"registration_number"`
		Address            *string `json:"address"`
		Phone              *string `json:"phone"`
		ContactEmail       *string `json:"contact_email"`
		RepresentativeName *string `json:"representative_name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	out, err := h.svc.PatchOwnCompany(r.Context(), caapp.PatchOwnCompanyRequest{
		Subject:            sub,
		CompanyName:        body.CompanyName,
		TaxCode:            body.TaxCode,
		RegistrationNumber: body.RegistrationNumber,
		Address:            body.Address,
		Phone:              body.Phone,
		ContactEmail:       body.ContactEmail,
		RepresentativeName: body.RepresentativeName,
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
		Email       string   `json:"email"`
		FullName    string   `json:"full_name"`
		RoleID      string   `json:"role_id"`
		RoleCode    string   `json:"role_code"`
		Permissions []string `json:"permissions"`
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
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.user.invite", "user", resp.UserID)
	httpx.WriteJSON(w, http.StatusOK, resp)
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
