package http

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	"github.com/cobo/cobo_iam_services/internal/platform/events"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
	"github.com/cobo/cobo_iam_services/internal/platform/outbox"
)

type Handler struct {
	log       *slog.Logger
	svc       iamapp.Service
	inspector iamapp.TokenInspector
	audit     auditapp.Service
	outbox    outbox.Publisher
	idgen     idgen.Generator
}

func NewHandler(log *slog.Logger, svc iamapp.Service, inspector iamapp.TokenInspector, audit auditapp.Service, outbox outbox.Publisher, idgen idgen.Generator) *Handler {
	return &Handler{log: log, svc: svc, inspector: inspector, audit: audit, outbox: outbox, idgen: idgen}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/login", h.login)
	mux.HandleFunc("POST /api/v1/auth/refresh", h.refresh)
	mux.HandleFunc("POST /api/v1/auth/logout", h.logout)
	mux.HandleFunc("POST /api/v1/auth/select-company", h.selectCompany)
	mux.HandleFunc("POST /api/v1/auth/switch-company", h.switchCompany)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req iamapp.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	req.IP = r.RemoteAddr
	req.UserAgent = r.UserAgent()
	resp, err := h.svc.Login(r.Context(), req)
	if err != nil {
		h.auditEvent(r, "login_failure", "deny", "", "", map[string]any{"login_id": req.LoginID})
		httpx.WriteError(w, h.log, err)
		return
	}
	h.auditEvent(r, "login_success", "allow", resp.User.UserID, contextMembership(resp), map[string]any{"next_action": resp.NextAction})
	h.publishEvent(r, "iam.session.login", resp.User.UserID, map[string]any{"next_action": resp.NextAction})
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	var req iamapp.RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	resp, err := h.svc.Refresh(r.Context(), req)
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	var req iamapp.LogoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	resp, err := h.svc.Logout(r.Context(), req)
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	h.auditEvent(r, "logout", "allow", "", "", nil)
	h.publishEvent(r, "iam.session.logout", "", map[string]any{})
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) selectCompany(w http.ResponseWriter, r *http.Request) {
	bearer := bearerToken(r.Header.Get("Authorization"))
	claims, err := h.inspector.InspectPreCompanyToken(r.Context(), bearer)
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	var req iamapp.SelectCompanyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	req.UserID = claims.Sub
	req.SessionID = claims.SessionID
	resp, err := h.svc.SelectCompany(r.Context(), req)
	if err != nil {
		h.auditEvent(r, "select_company_failure", "deny", claims.Sub, "", map[string]any{"company_id": req.CompanyID})
		httpx.WriteError(w, h.log, err)
		return
	}
	h.auditEvent(r, "select_company", "allow", claims.Sub, resp.CurrentContext.MembershipID, map[string]any{"company_id": resp.CurrentContext.CompanyID})
	h.publishEvent(r, "iam.company.selected", claims.Sub, map[string]any{"company_id": resp.CurrentContext.CompanyID, "membership_id": resp.CurrentContext.MembershipID})
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) switchCompany(w http.ResponseWriter, r *http.Request) {
	bearer := bearerToken(r.Header.Get("Authorization"))
	claims, err := h.inspector.InspectAccessToken(r.Context(), bearer)
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	var req iamapp.SwitchCompanyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	req.UserID = claims.Sub
	req.SessionID = claims.SessionID
	resp, err := h.svc.SwitchCompany(r.Context(), req)
	if err != nil {
		h.auditEvent(r, "switch_company_failure", "deny", claims.Sub, claims.MembershipID, map[string]any{"company_id": req.CompanyID})
		httpx.WriteError(w, h.log, err)
		return
	}
	h.auditEvent(r, "switch_company", "allow", claims.Sub, resp.CurrentContext.MembershipID, map[string]any{"from_company_id": claims.CompanyID, "to_company_id": resp.CurrentContext.CompanyID})
	h.publishEvent(r, "iam.company.switched", claims.Sub, map[string]any{"from_company_id": claims.CompanyID, "to_company_id": resp.CurrentContext.CompanyID, "membership_id": resp.CurrentContext.MembershipID})
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) auditEvent(r *http.Request, action, decision, userID, membershipID string, metadata map[string]any) {
	if h.audit == nil {
		return
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	_ = h.audit.AppendAuditLog(r.Context(), auditapp.AppendAuditLogRequest{
		ActorUserID:       userID,
		ActorMembershipID: membershipID,
		Action:            action,
		Decision:          decision,
		RequestID:         httpx.RequestIDFromContext(r.Context()),
		IP:                r.RemoteAddr,
		UserAgent:         r.UserAgent(),
		Metadata:          metadata,
	})
}

func (h *Handler) publishEvent(r *http.Request, eventType, aggregateID string, payload map[string]any) {
	if h.outbox == nil {
		return
	}
	if payload == nil {
		payload = map[string]any{}
	}
	payload["request_id"] = httpx.RequestIDFromContext(r.Context())
	_ = h.outbox.Publish(r.Context(), events.Event{
		EventID:       h.idgen.NewUUID(),
		AggregateType: "iam_session",
		AggregateID:   aggregateID,
		EventType:     eventType,
		Payload:       payload,
		OccurredAt:    nowUTC(),
	})
}

func nowUTC() (t time.Time) { return time.Now().UTC() }

func contextMembership(resp *iamapp.LoginResponse) string {
	if resp != nil && resp.CurrentContext != nil {
		return resp.CurrentContext.MembershipID
	}
	return ""
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
