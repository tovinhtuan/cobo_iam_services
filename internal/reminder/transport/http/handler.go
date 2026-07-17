package http

import (
	"crypto/hmac"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
	reminderapp "github.com/cobo/cobo_iam_services/internal/reminder/app"
)

type Handler struct {
	svc          reminderapp.Service
	inspector    iamapp.TokenInspector
	internalToken string
	env          string
}

func NewHandler(svc reminderapp.Service, inspector iamapp.TokenInspector, internalToken, env string) *Handler {
	return &Handler{svc: svc, inspector: inspector, internalToken: internalToken, env: env}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("PUT /api/v1/disclosures/{id}/reminder-config", h.upsertDisclosureReminderConfig)
	mux.HandleFunc("GET /api/v1/disclosures/{id}/reminder-config", h.getDisclosureReminderConfig)
	mux.HandleFunc("PUT /api/v1/disclosures/{id}/workflow/{stepId}/reminder-config", h.upsertWorkflowStepReminderConfig)
	mux.HandleFunc("GET /api/v1/disclosures/{id}/workflow/{stepId}/reminder-config", h.getWorkflowStepReminderConfig)
	mux.HandleFunc("GET /api/v1/disclosures/{id}/reminders/history", h.getReminderHistory)
	mux.HandleFunc("POST /internal/reminders/dispatch", h.dispatchOccurrence)
	if buildAllowsDevSeed() {
		mux.HandleFunc("POST /internal/dev/reminders/seed-occurrence", h.seedOccurrence)
	}
}

func (h *Handler) upsertDisclosureReminderConfig(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	disclosureID := r.PathValue("id")
	var payload reminderapp.ReminderConfigInput
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}

	resp, err := h.svc.UpsertDisclosureReminderConfig(r.Context(), reminderapp.UpsertReminderConfigRequest{
		Subject:      sub,
		DisclosureID: disclosureID,
		Config:       payload,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) getDisclosureReminderConfig(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	disclosureID := r.PathValue("id")
	resp, err := h.svc.GetDisclosureReminderConfig(r.Context(), reminderapp.GetReminderConfigRequest{
		Subject:     sub,
		DisclosureID: disclosureID,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) upsertWorkflowStepReminderConfig(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	disclosureID := r.PathValue("id")
	stepID := r.PathValue("stepId")

	var payload reminderapp.ReminderConfigInput
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	resp, err := h.svc.UpsertWorkflowStepReminderConfig(r.Context(), reminderapp.UpsertReminderConfigRequest{
		Subject:        sub,
		DisclosureID:   disclosureID,
		WorkflowStepID: stepID,
		Config:         payload,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) getWorkflowStepReminderConfig(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	disclosureID := r.PathValue("id")
	stepID := r.PathValue("stepId")
	resp, err := h.svc.GetWorkflowStepReminderConfig(r.Context(), reminderapp.GetReminderConfigRequest{
		Subject:        sub,
		DisclosureID:   disclosureID,
		WorkflowStepID: stepID,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	_ = disclosureID // retained for route parity and future validation.
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) getReminderHistory(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	disclosureID := r.PathValue("id")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	scope := strings.TrimSpace(r.URL.Query().Get("scope"))

	var status reminderapp.ReminderStatus
	if v := strings.TrimSpace(r.URL.Query().Get("status")); v != "" {
		status = reminderapp.ReminderStatus(v)
	}

	var from time.Time
	if v := strings.TrimSpace(r.URL.Query().Get("from")); v != "" {
		from, _ = time.Parse(time.RFC3339, v)
	}
	var to time.Time
	if v := strings.TrimSpace(r.URL.Query().Get("to")); v != "" {
		to, _ = time.Parse(time.RFC3339, v)
	}

	resp, err := h.svc.GetReminderHistory(r.Context(), reminderapp.GetReminderHistoryRequest{
		Subject:      sub,
		DisclosureID: disclosureID,
		Scope:        scope,
		Status:       status,
		From:         from,
		To:           to,
		Page:         page,
		PageSize:     pageSize,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) dispatchOccurrence(w http.ResponseWriter, r *http.Request) {
	if !h.authorizeInternalRequest(r) {
		httpx.WriteJSON(w, http.StatusUnauthorized, map[string]any{
			"error": map[string]any{
				"code":    "UNAUTHORIZED",
				"message": "invalid internal token",
			},
		})
		return
	}

	var req reminderapp.DispatchOccurrenceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	resp, err := h.svc.DispatchOccurrence(r.Context(), req)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusAccepted, resp)
}

func (h *Handler) seedOccurrence(w http.ResponseWriter, r *http.Request) {
	if !strings.EqualFold(h.env, "development") {
		httpx.WriteJSON(w, http.StatusForbidden, map[string]any{
			"error": map[string]any{
				"code":    "FORBIDDEN",
				"message": "seed endpoint is only enabled in development",
			},
		})
		return
	}
	if !h.authorizeInternalRequest(r) {
		httpx.WriteJSON(w, http.StatusUnauthorized, map[string]any{
			"error": map[string]any{
				"code":    "UNAUTHORIZED",
				"message": "invalid internal token",
			},
		})
		return
	}
	var req reminderapp.SeedOccurrenceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	resp, err := h.svc.SeedOccurrence(r.Context(), req)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, resp)
}

func (h *Handler) subjectFromToken(r *http.Request) (reminderapp.Subject, error) {
	tok := bearerToken(r.Header.Get("Authorization"))
	claims, err := h.inspector.InspectAccessToken(r.Context(), tok)
	if err != nil {
		return reminderapp.Subject{}, err
	}
	return reminderapp.Subject{
		UserID:       claims.Sub,
		MembershipID: claims.MembershipID,
		CompanyID:    claims.CompanyID,
	}, nil
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

func (h *Handler) authorizeInternalRequest(r *http.Request) bool {
	token := strings.TrimSpace(r.Header.Get("X-Internal-Token"))
	expected := strings.TrimSpace(h.internalToken)
	if expected == "" || token == "" {
		return false
	}
	return hmac.Equal([]byte(token), []byte(expected))
}
