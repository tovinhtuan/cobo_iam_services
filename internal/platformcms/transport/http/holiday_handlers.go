package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"

	holidayapp "github.com/cobo/cobo_iam_services/internal/holiday/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
)

const (
	holidayUploadLimit = 15 << 20 // .xlsx multipart
	// holidayMultipartOverhead covers multipart headers and the small text fields.
	holidayMultipartOverhead = 1 << 20
	holidayJSONBodyLimit     = 256 << 10 // editor JSON (≤ 400 rows)
	holidayDeleteLimit       = 8 << 10
)

// holidayEditorBody is the JSON body of POST …/preview and PUT …/{year}.
type holidayEditorBody struct {
	Dates            json.RawMessage `json:"dates"`
	Description      string          `json:"description"`
	ExpectedRevision *string         `json:"expected_revision"`
}

// requireHolidayAccess authorizes platform.cms.view, plus system.settings | rbac.manage when
// write is set (PUT/DELETE). Returns the audit actor and the path year.
func (h *Handler) requireHolidayAccess(w http.ResponseWriter, r *http.Request, write bool) (holidayapp.Actor, int, bool) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return holidayapp.Actor{}, 0, false
	}
	if _, err := h.requireCMSAccess(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return holidayapp.Actor{}, 0, false
	}
	if write {
		if _, err := h.requireAnyPermission(r.Context(), sub.MembershipID, sub.CompanyID, "system.settings", "rbac.manage"); err != nil {
			httpx.WriteError(w, nil, err)
			return holidayapp.Actor{}, 0, false
		}
	}
	year, err := parsePathYear(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return holidayapp.Actor{}, 0, false
	}
	return holidayapp.Actor{
		UserID:       sub.Sub,
		MembershipID: sub.MembershipID,
		CompanyID:    sub.CompanyID,
		RequestID:    httpx.RequestIDFromContext(r.Context()),
		IP:           r.RemoteAddr,
		UserAgent:    r.UserAgent(),
	}, year, true
}

func (h *Handler) holidayCalendarGet(w http.ResponseWriter, r *http.Request) {
	_, year, ok := h.requireHolidayAccess(w, r, false)
	if !ok {
		return
	}
	out, err := h.holidaySvc.Get(r.Context(), year)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	writeEnvelope(w, http.StatusOK, out, nil)
}

func (h *Handler) holidayCalendarDefault(w http.ResponseWriter, r *http.Request) {
	_, year, ok := h.requireHolidayAccess(w, r, false)
	if !ok {
		return
	}
	out, err := h.holidaySvc.Default(r.Context(), year)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	writeEnvelope(w, http.StatusOK, out, nil)
}

func (h *Handler) holidayCalendarExport(w http.ResponseWriter, r *http.Request) {
	_, year, ok := h.requireHolidayAccess(w, r, false)
	if !ok {
		return
	}
	raw, err := h.holidaySvc.ExportXLSX(r.Context(), year, strings.TrimSpace(r.URL.Query().Get("source")))
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="lich_khong_giao_dich_%d.xlsx"`, year))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Length", strconv.Itoa(len(raw)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}

func requestMediaType(r *http.Request) string {
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		return ""
	}
	return strings.ToLower(mt)
}

func decodeHolidayEditorBody(w http.ResponseWriter, r *http.Request) (holidayEditorBody, []holidayapp.HolidayInputRow, error) {
	var body holidayEditorBody
	if err := decodeCMSCompanyBody(w, r, &body, holidayJSONBodyLimit, false); err != nil {
		return body, nil, err
	}
	fieldErr := func(msg string) error {
		he := perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, msg, nil)
		he.Details = map[string]any{"field_errors": map[string]string{"dates": msg}}
		return he
	}
	if len(body.Dates) == 0 || string(body.Dates) == "null" {
		return body, nil, fieldErr("dates is required")
	}
	var rows []holidayapp.HolidayInputRow
	if err := json.Unmarshal(body.Dates, &rows); err != nil {
		return body, nil, fieldErr("dates must be an array of {date, name, day_type}")
	}
	return body, rows, nil
}

// readUpload reads the multipart "file" field (unchanged .xlsx flow). The whole body is capped
// (file + form fields); ParseMultipartForm alone only caps memory and spills the rest to disk.
func readUpload(w http.ResponseWriter, r *http.Request) ([]byte, string, error) {
	r.Body = http.MaxBytesReader(w, r.Body, holidayUploadLimit+holidayMultipartOverhead)
	if err := r.ParseMultipartForm(holidayUploadLimit); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return nil, "", perr.NewHTTPError(http.StatusRequestEntityTooLarge, perr.CodeInvalidRequest, "file too large", nil)
		}
		return nil, "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "multipart form required", err)
	}
	f, header, err := r.FormFile("file")
	if err != nil {
		return nil, "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "file field required", err)
	}
	defer func() { _ = f.Close() }()
	raw, err := io.ReadAll(io.LimitReader(f, holidayUploadLimit))
	if err != nil {
		return nil, "", err
	}
	name := ""
	if header != nil {
		name = header.Filename
	}
	return raw, name, nil
}

// holidayCalendarPreview validates an .xlsx (multipart) or editor rows (JSON) without saving.
func (h *Handler) holidayCalendarPreview(w http.ResponseWriter, r *http.Request) {
	_, year, ok := h.requireHolidayAccess(w, r, false)
	if !ok {
		return
	}
	var (
		preview *holidayapp.PreviewResult
		err     error
	)
	if requestMediaType(r) == "application/json" {
		var rows []holidayapp.HolidayInputRow
		if _, rows, err = decodeHolidayEditorBody(w, r); err == nil {
			preview, err = h.holidaySvc.PreviewRows(r.Context(), year, rows)
		}
	} else {
		var (
			raw  []byte
			name string
		)
		if raw, name, err = readUpload(w, r); err == nil {
			preview, err = h.holidaySvc.Preview(r.Context(), year, name, raw)
		}
	}
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	writeEnvelope(w, http.StatusOK, preview, nil)
}

// holidayCalendarReplace publishes the year's calendar from an .xlsx (multipart) or editor rows (JSON).
func (h *Handler) holidayCalendarReplace(w http.ResponseWriter, r *http.Request) {
	actor, year, ok := h.requireHolidayAccess(w, r, true)
	if !ok {
		return
	}
	req := holidayapp.ReplaceRequest{Year: year, Actor: actor}
	if requestMediaType(r) == "application/json" {
		body, rows, err := decodeHolidayEditorBody(w, r)
		if err != nil {
			httpx.WriteError(w, nil, err)
			return
		}
		req.FromEditor = true
		req.Rows = rows
		req.Description = strings.TrimSpace(body.Description)
		req.ExpectedRevision = body.ExpectedRevision
	} else {
		raw, name, err := readUpload(w, r)
		if err != nil {
			httpx.WriteError(w, nil, err)
			return
		}
		req.Raw = raw
		req.SourceName = name
		req.Description = strings.TrimSpace(r.FormValue("description"))
		if vals, present := r.MultipartForm.Value["expected_revision"]; present && len(vals) > 0 {
			rev := strings.TrimSpace(vals[0])
			req.ExpectedRevision = &rev
		}
	}
	out, err := h.holidaySvc.Replace(r.Context(), req)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	writeEnvelope(w, http.StatusOK, out, nil)
}

// holidayCalendarDelete removes the published calendar; the year falls back to the default file.
func (h *Handler) holidayCalendarDelete(w http.ResponseWriter, r *http.Request) {
	actor, year, ok := h.requireHolidayAccess(w, r, true)
	if !ok {
		return
	}
	var body struct {
		ExpectedRevision *string `json:"expected_revision"`
		Reason           string  `json:"reason"`
	}
	if r.Body != nil && r.ContentLength != 0 {
		if err := decodeCMSCompanyBody(w, r, &body, holidayDeleteLimit, true); err != nil {
			httpx.WriteError(w, nil, err)
			return
		}
	}
	out, err := h.holidaySvc.Delete(r.Context(), holidayapp.DeleteRequest{
		Year: year, Reason: body.Reason, ExpectedRevision: body.ExpectedRevision, Actor: actor,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	writeEnvelope(w, http.StatusOK, out, nil)
}

func parsePathYear(r *http.Request) (int, error) {
	ys := strings.TrimSpace(r.PathValue("year"))
	y, err := strconv.Atoi(ys)
	if err != nil || y < 2000 || y > 2100 {
		return 0, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid year", err)
	}
	return y, nil
}
