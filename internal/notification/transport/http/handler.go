package http

import (
	"encoding/json"
	"net/http"
	"strings"

	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	notificationapp "github.com/cobo/cobo_iam_services/internal/notification/app"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
)

type Handler struct {
	svc       notificationapp.Service
	inspector iamapp.TokenInspector
}

func NewHandler(svc notificationapp.Service, inspector iamapp.TokenInspector) *Handler {
	return &Handler{svc: svc, inspector: inspector}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/notifications/resolve-recipients", h.resolveRecipients)
	mux.HandleFunc("POST /api/v1/notifications/enqueue", h.enqueue)
	mux.HandleFunc("POST /api/v1/notifications/dispatch", h.dispatch)
}

func (h *Handler) resolveRecipients(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var req notificationapp.ResolveRecipientsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	req.Subject = sub
	resp, err := h.svc.ResolveRecipients(r.Context(), req)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) enqueue(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var req notificationapp.EnqueueNotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	req.Subject = sub
	resp, err := h.svc.EnqueueNotification(r.Context(), req)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, resp)
}

func (h *Handler) dispatch(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var req notificationapp.DispatchPendingRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	req.Subject = sub
	resp, err := h.svc.DispatchPending(r.Context(), req)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) subjectFromToken(r *http.Request) (notificationapp.Subject, error) {
	tok := bearerToken(r.Header.Get("Authorization"))
	claims, err := h.inspector.InspectAccessToken(r.Context(), tok)
	if err != nil {
		return notificationapp.Subject{}, err
	}
	return notificationapp.Subject{UserID: claims.Sub, MembershipID: claims.MembershipID, CompanyID: claims.CompanyID}, nil
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
