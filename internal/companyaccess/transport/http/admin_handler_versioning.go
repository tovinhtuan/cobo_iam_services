package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
)

func (h *AdminHandler) registerVersioningRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/admin/notification-rules/versions", h.listNotificationRuleVersions)
	mux.HandleFunc("GET /api/v1/admin/notification-rules/versions/compare", h.compareNotificationRuleVersions)
	mux.HandleFunc("GET /api/v1/admin/notification-rules/versions/{version_no}", h.getNotificationRuleVersion)
	mux.HandleFunc("POST /api/v1/admin/notification-rules/versions/{version_no}/rollback", h.rollbackNotificationRuleVersion)

	mux.HandleFunc("GET /api/v1/admin/rbac/matrix/versions", h.listRBACMatrixVersions)
	mux.HandleFunc("GET /api/v1/admin/rbac/matrix/versions/compare", h.compareRBACMatrixVersions)
	mux.HandleFunc("GET /api/v1/admin/rbac/matrix/versions/{version_no}", h.getRBACMatrixVersion)
	mux.HandleFunc("POST /api/v1/admin/rbac/matrix/versions/{version_no}/rollback", h.rollbackRBACMatrixVersion)
}

func (h *AdminHandler) listNotificationRuleVersions(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	limit := parseVersionLimit(r.URL.Query().Get("limit"))
	out, err := h.svc.ListNotificationRuleVersions(r.Context(), caapp.ListNotificationRuleVersionsRequest{
		Subject: sub,
		RuleID:  strings.TrimSpace(r.URL.Query().Get("rule_id")),
		Limit:   limit,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *AdminHandler) getNotificationRuleVersion(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	versionNo, err := parsePathVersionNo(r.PathValue("version_no"))
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	ruleID := strings.TrimSpace(r.URL.Query().Get("rule_id"))
	if ruleID == "" {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "rule_id query required", nil))
		return
	}
	detail, err := h.svc.GetNotificationRuleVersion(r.Context(), caapp.GetNotificationRuleVersionRequest{
		Subject: sub, RuleID: ruleID, VersionNo: versionNo,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var snapshot any
	_ = json.Unmarshal(detail.SnapshotJSON, &snapshot)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"id":             detail.ID,
		"company_id":     detail.CompanyID,
		"aggregate_type": detail.AggregateType,
		"rule_id":        detail.AggregateID,
		"version_no":     detail.VersionNo,
		"created_by":     detail.CreatedBy,
		"created_at":     detail.CreatedAt,
		"reason":         detail.Reason,
		"source":         detail.Source,
		"snapshot":       snapshot,
	})
}

func (h *AdminHandler) compareNotificationRuleVersions(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	ruleID := strings.TrimSpace(r.URL.Query().Get("rule_id"))
	from, to, err := parseCompareVersions(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if ruleID == "" {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "rule_id query required", nil))
		return
	}
	out, err := h.svc.CompareNotificationRuleVersions(r.Context(), caapp.CompareNotificationRuleVersionsRequest{
		Subject: sub, RuleID: ruleID, FromVersionNo: from, ToVersionNo: to,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *AdminHandler) rollbackNotificationRuleVersion(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	versionNo, err := parsePathVersionNo(r.PathValue("version_no"))
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var body struct {
		RuleID string `json:"rule_id"`
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	ruleID := strings.TrimSpace(body.RuleID)
	if ruleID == "" {
		ruleID = strings.TrimSpace(r.URL.Query().Get("rule_id"))
	}
	if ruleID == "" {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "rule_id required", nil))
		return
	}
	row, err := h.svc.RollbackNotificationRuleVersion(r.Context(), caapp.RollbackNotificationRuleVersionRequest{
		Subject: sub, RuleID: ruleID, VersionNo: versionNo, Reason: body.Reason,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditVersionLog(r, sub, "admin.version.notification.rollback", "notification_rule", ruleID, map[string]any{
		"target_version_no": versionNo,
		"new_version_no":    row.VersionNo,
	})
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"rolled_back_from": versionNo, "new_version": row})
}

func (h *AdminHandler) listRBACMatrixVersions(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	out, err := h.svc.ListRBACMatrixVersions(r.Context(), caapp.ListRBACMatrixVersionsRequest{
		Subject: sub,
		Limit:   parseVersionLimit(r.URL.Query().Get("limit")),
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *AdminHandler) getRBACMatrixVersion(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	versionNo, err := parsePathVersionNo(r.PathValue("version_no"))
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	detail, err := h.svc.GetRBACMatrixVersion(r.Context(), caapp.GetRBACMatrixVersionRequest{
		Subject: sub, VersionNo: versionNo,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var snapshot any
	_ = json.Unmarshal(detail.SnapshotJSON, &snapshot)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"id":             detail.ID,
		"company_id":     detail.CompanyID,
		"aggregate_type": detail.AggregateType,
		"version_no":     detail.VersionNo,
		"created_by":     detail.CreatedBy,
		"created_at":     detail.CreatedAt,
		"reason":         detail.Reason,
		"source":         detail.Source,
		"snapshot":       snapshot,
	})
}

func (h *AdminHandler) compareRBACMatrixVersions(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	from, to, err := parseCompareVersions(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	out, err := h.svc.CompareRBACMatrixVersions(r.Context(), caapp.CompareRBACMatrixVersionsRequest{
		Subject: sub, FromVersionNo: from, ToVersionNo: to,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *AdminHandler) rollbackRBACMatrixVersion(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	versionNo, err := parsePathVersionNo(r.PathValue("version_no"))
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	row, err := h.svc.RollbackRBACMatrixVersion(r.Context(), caapp.RollbackRBACMatrixVersionRequest{
		Subject: sub, VersionNo: versionNo, Reason: body.Reason,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditVersionLog(r, sub, "admin.version.rbac.rollback", "rbac_matrix", sub.CompanyID, map[string]any{
		"target_version_no": versionNo,
		"new_version_no":    row.VersionNo,
	})
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"rolled_back_from": versionNo, "new_version": row})
}

func (h *AdminHandler) auditVersionLog(r *http.Request, sub caapp.AdminSubject, action, resourceType, resourceID string, meta map[string]any) {
	if h.audit == nil {
		return
	}
	_ = h.audit.AppendAuditLog(r.Context(), auditapp.AppendAuditLogRequest{
		ActorUserID: sub.UserID, ActorMembershipID: sub.MembershipID, CompanyID: sub.CompanyID,
		Action: action, ResourceType: resourceType, ResourceID: resourceID, Decision: "allow",
		RequestID: httpx.RequestIDFromContext(r.Context()), IP: r.RemoteAddr, UserAgent: r.UserAgent(),
		Metadata: meta,
	})
}

func parsePathVersionNo(raw string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n <= 0 {
		return 0, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid version_no", nil)
	}
	return n, nil
}

func parseCompareVersions(r *http.Request) (from, to int, err error) {
	from, err = strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("from")))
	if err != nil || from <= 0 {
		return 0, 0, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid from version", nil)
	}
	to, err = strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("to")))
	if err != nil || to <= 0 {
		return 0, 0, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid to version", nil)
	}
	return from, to, nil
}

func parseVersionLimit(raw string) int {
	if raw == "" {
		return 50
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 50
	}
	if n > 100 {
		return 100
	}
	return n
}
