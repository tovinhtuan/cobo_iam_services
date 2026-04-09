package http

import (
	"encoding/json"
	"net/http"
	"strings"

	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
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
