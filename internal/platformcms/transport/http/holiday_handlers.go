package http

import (
	"io"
	"net/http"
	"strconv"
	"strings"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
)

func (h *Handler) holidayCalendarGet(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireCMSAccess(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	year, err := parsePathYear(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	out, err := h.holidaySvc.Get(r.Context(), year)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	writeEnvelope(w, http.StatusOK, out, nil)
}

func (h *Handler) holidayCalendarPreview(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireCMSAccess(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	year, err := parsePathYear(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if err := r.ParseMultipartForm(15 << 20); err != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "multipart form required", err))
		return
	}
	f, header, err := r.FormFile("file")
	if err != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "file field required", err))
		return
	}
	defer func() { _ = f.Close() }()
	raw, err := io.ReadAll(io.LimitReader(f, 15<<20))
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	name := ""
	if header != nil {
		name = header.Filename
	}
	preview, err := h.holidaySvc.Preview(r.Context(), year, name, raw)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	writeEnvelope(w, http.StatusOK, preview, nil)
}

func (h *Handler) holidayCalendarReplace(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireCMSAccess(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireAnyPermission(r.Context(), sub.MembershipID, sub.CompanyID, "system.settings", "rbac.manage"); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	year, err := parsePathYear(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if err := r.ParseMultipartForm(15 << 20); err != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "multipart form required", err))
		return
	}
	f, header, err := r.FormFile("file")
	if err != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "file field required", err))
		return
	}
	defer func() { _ = f.Close() }()
	raw, err := io.ReadAll(io.LimitReader(f, 15<<20))
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	name := ""
	if header != nil {
		name = header.Filename
	}
	desc := strings.TrimSpace(r.FormValue("description"))
	if err := h.holidaySvc.Replace(r.Context(), year, name, sub.Sub, desc, raw); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	writeEnvelope(w, http.StatusOK, map[string]any{"calendar_year": year, "status": "replaced"}, nil)
}

func parsePathYear(r *http.Request) (int, error) {
	ys := strings.TrimSpace(r.PathValue("year"))
	y, err := strconv.Atoi(ys)
	if err != nil || y < 2000 || y > 2100 {
		return 0, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid year", err)
	}
	return y, nil
}
