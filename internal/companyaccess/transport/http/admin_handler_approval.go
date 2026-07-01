package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
)

func (h *AdminHandler) registerApprovalRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/admin/config-approvals", h.submitConfigApproval)
	mux.HandleFunc("GET /api/v1/admin/config-approvals", h.listConfigApprovals)
	mux.HandleFunc("GET /api/v1/admin/config-approvals/{approval_id}", h.getConfigApproval)
	mux.HandleFunc("GET /api/v1/admin/config-approvals/{approval_id}/compare", h.compareConfigApproval)
	mux.HandleFunc("POST /api/v1/admin/config-approvals/{approval_id}/approve", h.approveConfigApproval)
	mux.HandleFunc("POST /api/v1/admin/config-approvals/{approval_id}/reject", h.rejectConfigApproval)
	mux.HandleFunc("POST /api/v1/admin/config-approvals/{approval_id}/cancel", h.cancelConfigApproval)
}

func writeApprovalRoutedOrError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.Code != perr.CodeApprovalRouted {
		httpx.WriteError(w, nil, err)
		return true
	}
	body := map[string]any{
		"approval_id": "",
		"status":      "pending",
	}
	if he.Details != nil {
		for k, v := range he.Details {
			body[k] = v
		}
	}
	httpx.WriteJSON(w, http.StatusAccepted, body)
	return true
}

func (h *AdminHandler) submitConfigApproval(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var body struct {
		AggregateType string         `json:"aggregate_type"`
		AggregateID   string         `json:"aggregate_id"`
		ChangeType    string         `json:"change_type"`
		Reason        string         `json:"reason"`
		Proposed      map[string]any `json:"proposed"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid json body", err))
		return
	}
	out, err := h.svc.SubmitConfigApproval(r.Context(), caapp.SubmitConfigApprovalRequest{
		Subject:       sub,
		AggregateType: strings.TrimSpace(body.AggregateType),
		AggregateID:   strings.TrimSpace(body.AggregateID),
		ChangeType:    strings.TrimSpace(body.ChangeType),
		Reason:        strings.TrimSpace(body.Reason),
		Proposed:      body.Proposed,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.config.approval.requested", "config_approval", out.ApprovalID)
	httpx.WriteJSON(w, http.StatusCreated, out)
}

func (h *AdminHandler) listConfigApprovals(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	limit := 50
	if v := strings.TrimSpace(r.URL.Query().Get("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	out, err := h.svc.ListConfigApprovals(r.Context(), caapp.ListConfigApprovalsRequest{
		Subject:       sub,
		Status:        strings.TrimSpace(r.URL.Query().Get("status")),
		AggregateType: strings.TrimSpace(r.URL.Query().Get("aggregate_type")),
		Limit:         limit,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *AdminHandler) getConfigApproval(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	id := strings.TrimSpace(r.PathValue("approval_id"))
	out, err := h.svc.GetConfigApproval(r.Context(), caapp.GetConfigApprovalRequest{Subject: sub, ApprovalID: id})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *AdminHandler) compareConfigApproval(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	id := strings.TrimSpace(r.PathValue("approval_id"))
	out, err := h.svc.CompareConfigApproval(r.Context(), caapp.CompareConfigApprovalRequest{Subject: sub, ApprovalID: id})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *AdminHandler) approveConfigApproval(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	id := strings.TrimSpace(r.PathValue("approval_id"))
	out, err := h.svc.ApproveConfigApproval(r.Context(), caapp.ApproveConfigApprovalRequest{Subject: sub, ApprovalID: id})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.config.approval.approved", "config_approval", id)
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *AdminHandler) rejectConfigApproval(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var body struct {
		RejectReason string `json:"reject_reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	id := strings.TrimSpace(r.PathValue("approval_id"))
	out, err := h.svc.RejectConfigApproval(r.Context(), caapp.RejectConfigApprovalRequest{
		Subject: sub, ApprovalID: id, RejectReason: strings.TrimSpace(body.RejectReason),
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.config.approval.rejected", "config_approval", id)
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *AdminHandler) cancelConfigApproval(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	id := strings.TrimSpace(r.PathValue("approval_id"))
	out, err := h.svc.CancelConfigApproval(r.Context(), caapp.CancelConfigApprovalRequest{Subject: sub, ApprovalID: id})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "admin.config.approval.cancelled", "config_approval", id)
	httpx.WriteJSON(w, http.StatusOK, out)
}
