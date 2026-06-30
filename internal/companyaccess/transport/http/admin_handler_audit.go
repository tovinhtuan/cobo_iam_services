package http

import (
	"net/http"
	"strconv"
	"strings"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
)

func (h *AdminHandler) registerAuditRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/admin/audit-logs", h.listAuditLogs)
	mux.HandleFunc("GET /api/v1/admin/change-timeline", h.listChangeTimeline)
}

func (h *AdminHandler) listAuditLogs(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	limit := parseAuditLimit(r.URL.Query().Get("limit"))
	if limit < 0 {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid limit", nil))
		return
	}
	actionPrefix := strings.TrimSpace(r.URL.Query().Get("action_prefix")) == "1" || strings.EqualFold(r.URL.Query().Get("action_prefix"), "true")
	out, err := h.svc.ListAuditLogs(r.Context(), caapp.ListAuditLogsRequest{
		Subject:          sub,
		Limit:            limit,
		Cursor:           strings.TrimSpace(r.URL.Query().Get("cursor")),
		Action:           strings.TrimSpace(r.URL.Query().Get("action")),
		ActionPrefix:     actionPrefix,
		ResourceType:     strings.TrimSpace(r.URL.Query().Get("resource_type")),
		ResourceID:       strings.TrimSpace(r.URL.Query().Get("resource_id")),
		FromOccurredAt: strings.TrimSpace(r.URL.Query().Get("from")),
		ToOccurredAt:   strings.TrimSpace(r.URL.Query().Get("to")),
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"items": out.Items,
		"meta": map[string]any{
			"total":       out.Total,
			"limit":       out.Limit,
			"next_cursor": nullableCursor(out.NextCursor),
		},
	})
}

func (h *AdminHandler) listChangeTimeline(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	limit := parseAuditLimit(r.URL.Query().Get("limit"))
	if limit < 0 {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid limit", nil))
		return
	}
	actionPrefix := strings.TrimSpace(r.URL.Query().Get("action_prefix")) == "1" || strings.EqualFold(r.URL.Query().Get("action_prefix"), "true")
	out, err := h.svc.ListChangeTimeline(r.Context(), caapp.ListChangeTimelineRequest{
		Subject:          sub,
		Limit:            limit,
		Cursor:           strings.TrimSpace(r.URL.Query().Get("cursor")),
		Action:           strings.TrimSpace(r.URL.Query().Get("action")),
		ActionPrefix:     actionPrefix,
		Domain:           strings.TrimSpace(r.URL.Query().Get("domain")),
		ResourceType:     strings.TrimSpace(r.URL.Query().Get("resource_type")),
		ResourceID:       strings.TrimSpace(r.URL.Query().Get("resource_id")),
		FromOccurredAt: strings.TrimSpace(r.URL.Query().Get("from")),
		ToOccurredAt:   strings.TrimSpace(r.URL.Query().Get("to")),
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func parseAuditLimit(raw string) int {
	if strings.TrimSpace(raw) == "" {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n < 0 {
		return -1
	}
	return n
}

func nullableCursor(cursor string) any {
	if strings.TrimSpace(cursor) == "" {
		return nil
	}
	return cursor
}
