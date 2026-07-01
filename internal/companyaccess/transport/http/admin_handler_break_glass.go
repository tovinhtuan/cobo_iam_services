package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
)

func (h *AdminHandler) registerEmergencyAccessRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/admin/emergency-access/requests", h.createEmergencyAccessRequest)
	mux.HandleFunc("GET /api/v1/admin/emergency-access/requests", h.listEmergencyAccessRequests)
	mux.HandleFunc("GET /api/v1/admin/emergency-access/requests/{session_id}", h.getEmergencyAccessRequest)
	mux.HandleFunc("POST /api/v1/admin/emergency-access/requests/{session_id}/approve", h.approveEmergencyAccessRequest)
	mux.HandleFunc("POST /api/v1/admin/emergency-access/requests/{session_id}/deny", h.denyEmergencyAccessRequest)
	mux.HandleFunc("POST /api/v1/admin/emergency-access/requests/{session_id}/cancel", h.cancelEmergencyAccessRequest)
	mux.HandleFunc("POST /api/v1/admin/emergency-access/requests/{session_id}/revoke", h.revokeEmergencyAccessRequest)
	mux.HandleFunc("GET /api/v1/admin/emergency-access/requests/{session_id}/timeline", h.getEmergencyAccessTimeline)
}

func (h *AdminHandler) createEmergencyAccessRequest(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var body struct {
		TargetMembershipID       string `json:"target_membership_id"`
		Reason                   string `json:"reason"`
		RequestedDurationSeconds int    `json:"requested_duration_seconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	out, err := h.svc.CreateEmergencyAccessRequest(r.Context(), caapp.CreateEmergencyAccessRequest{
		Subject:                  sub,
		TargetMembershipID:       body.TargetMembershipID,
		Reason:                   body.Reason,
		RequestedDurationSeconds: body.RequestedDurationSeconds,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "breakglass.session.created", "break_glass_session", out.SessionID)
	httpx.WriteJSON(w, http.StatusCreated, out)
}

func (h *AdminHandler) listEmergencyAccessRequests(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	out, err := h.svc.ListEmergencyAccessRequests(r.Context(), caapp.ListEmergencyAccessRequests{
		Subject:            sub,
		Status:             r.URL.Query().Get("status"),
		TargetMembershipID: r.URL.Query().Get("target_membership_id"),
		Limit:              limit,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *AdminHandler) getEmergencyAccessRequest(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	out, err := h.svc.GetEmergencyAccessRequest(r.Context(), caapp.GetEmergencyAccessRequest{
		Subject:   sub,
		SessionID: r.PathValue("session_id"),
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *AdminHandler) approveEmergencyAccessRequest(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	id := strings.TrimSpace(r.PathValue("session_id"))
	out, err := h.svc.ApproveEmergencyAccessRequest(r.Context(), caapp.ApproveEmergencyAccessRequest{
		Subject:   sub,
		SessionID: id,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if out.Status == caapp.EmergencyStatusPendingSecond {
		h.auditLog(r, "breakglass.session.approved", "break_glass_session", out.SessionID)
	} else if out.Status == caapp.EmergencyStatusActive {
		h.auditLog(r, "breakglass.session.approved", "break_glass_session", out.SessionID)
		h.auditLog(r, "breakglass.session.activated", "break_glass_session", out.SessionID)
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *AdminHandler) denyEmergencyAccessRequest(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	id := strings.TrimSpace(r.PathValue("session_id"))
	out, err := h.svc.DenyEmergencyAccessRequest(r.Context(), caapp.DenyEmergencyAccessRequest{
		Subject:   sub,
		SessionID: id,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "breakglass.session.denied", "break_glass_session", out.SessionID)
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *AdminHandler) cancelEmergencyAccessRequest(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	id := strings.TrimSpace(r.PathValue("session_id"))
	out, err := h.svc.CancelEmergencyAccessRequest(r.Context(), caapp.CancelEmergencyAccessRequest{
		Subject:   sub,
		SessionID: id,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *AdminHandler) revokeEmergencyAccessRequest(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	id := strings.TrimSpace(r.PathValue("session_id"))
	out, err := h.svc.RevokeEmergencyAccessRequest(r.Context(), caapp.RevokeEmergencyAccessRequest{
		Subject:   sub,
		SessionID: id,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "breakglass.session.revoked", "break_glass_session", out.SessionID)
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *AdminHandler) getEmergencyAccessTimeline(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	out, err := h.svc.GetEmergencyAccessTimeline(r.Context(), caapp.GetEmergencyAccessTimelineRequest{
		Subject:   sub,
		SessionID: r.PathValue("session_id"),
		Limit:     limit,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}
