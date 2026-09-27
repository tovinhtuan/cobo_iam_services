package http

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
)

func (h *Handler) cmsListTemplateImportHistory(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	limit, err := parseHistoryLimit(r.URL.Query().Get("limit"))
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	from, err := parseHistoryTime(r.URL.Query().Get("from"))
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	to, err := parseHistoryTime(r.URL.Query().Get("to"))
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	resp, err := h.svc.ListTemplateImportHistory(r.Context(), disclosureapp.ListTemplateImportHistoryRequest{
		Subject:    sub,
		Status:     r.URL.Query().Get("status"),
		Filename:   r.URL.Query().Get("filename"),
		FileSHA256: r.URL.Query().Get("file_sha256"),
		From:       from,
		To:         to,
		Cursor:     r.URL.Query().Get("cursor"),
		Limit:      limit,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var next any
	if resp.NextCursor != "" {
		next = resp.NextCursor
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{"items": resp.Items},
		"meta": map[string]any{"limit": resp.Limit, "next_cursor": next},
	})
}

func (h *Handler) cmsGetTemplateImportHistory(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	detail, err := h.svc.GetTemplateImportHistory(r.Context(), disclosureapp.GetTemplateImportHistoryRequest{
		Subject: sub,
		ID:      r.PathValue("id"),
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": detail})
}

func parseHistoryLimit(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 20, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > 100 {
		return 0, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "limit must be between 1 and 100", nil)
	}
	return n, nil
}

func parseHistoryTime(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, nil
	}
	ts, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid datetime filter", nil)
	}
	return ts, nil
}
