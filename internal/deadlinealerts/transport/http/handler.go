package http

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	deadlinealertsapp "github.com/cobo/cobo_iam_services/internal/deadlinealerts/app"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
)

type Handler struct {
	log       *slog.Logger
	svc       deadlinealertsapp.Service
	inspector iamapp.TokenInspector
}

func NewHandler(log *slog.Logger, svc deadlinealertsapp.Service, inspector iamapp.TokenInspector) *Handler {
	return &Handler{log: log, svc: svc, inspector: inspector}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/company/deadline-alerts", h.listDeadlineAlerts)
	mux.HandleFunc("POST /api/v1/company/deadline-alerts/{id}/confirm", h.confirmDeadlineAlert)
}

func (h *Handler) listDeadlineAlerts(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	page, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("page")))
	pageSize, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("page_size")))
	resp, err := h.svc.ListDeadlineAlerts(r.Context(), deadlinealertsapp.ListDeadlineAlertsRequest{
		Subject:   sub,
		Status:    strings.TrimSpace(r.URL.Query().Get("status")),
		Query:     strings.TrimSpace(r.URL.Query().Get("q")),
		StartDate: strings.TrimSpace(r.URL.Query().Get("start_date")),
		EndDate:   strings.TrimSpace(r.URL.Query().Get("end_date")),
		Page:      page,
		PageSize:  pageSize,
	})
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	if resp.Items == nil {
		resp.Items = []deadlinealertsapp.DeadlineAlertDTO{}
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

type confirmDeadlineAlertBody struct {
	Note           string `json:"note"`
	IdempotencyKey string `json:"idempotency_key"`
}

func (h *Handler) confirmDeadlineAlert(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	recordID := strings.TrimSpace(r.PathValue("id"))
	if recordID == "" {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]any{
			"error": map[string]any{
				"code":    "invalid_request",
				"message": "record_id is required",
			},
		})
		return
	}
	var body confirmDeadlineAlertBody
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			httpx.WriteJSON(w, http.StatusBadRequest, map[string]any{
				"error": map[string]any{
					"code":    "invalid_request",
					"message": "invalid JSON body",
				},
			})
			return
		}
	}
	resp, err := h.svc.ConfirmDeadlineAlert(r.Context(), deadlinealertsapp.ConfirmDeadlineAlertRequest{
		Subject:        sub,
		RecordID:       recordID,
		Note:           strings.TrimSpace(body.Note),
		IdempotencyKey: strings.TrimSpace(body.IdempotencyKey),
	})
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) subjectFromToken(r *http.Request) (deadlinealertsapp.Subject, error) {
	tok := bearerToken(r.Header.Get("Authorization"))
	claims, err := h.inspector.InspectAccessToken(r.Context(), tok)
	if err != nil {
		return deadlinealertsapp.Subject{}, err
	}
	return deadlinealertsapp.Subject{
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
	return ""
}
