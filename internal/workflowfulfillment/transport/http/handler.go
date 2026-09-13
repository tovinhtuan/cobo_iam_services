package http

import (
	"io"
	"log/slog"
	"net/http"
	"strings"

	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
	wff "github.com/cobo/cobo_iam_services/internal/workflowfulfillment"
)

const maxMultipartMemory = 32 << 20

// Handler serves B2 document requirement fulfillment routes.
type Handler struct {
	log       *slog.Logger
	svc       *wff.Service
	inspector iamapp.TokenInspector
}

func NewHandler(log *slog.Logger, svc *wff.Service, inspector iamapp.TokenInspector) *Handler {
	return &Handler{log: log, svc: svc, inspector: inspector}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/company/deadlines/{record_id}/steps/{step_code}/document-requirements", h.listRequirements)
	mux.HandleFunc("POST /api/v1/company/deadlines/{record_id}/steps/{step_code}/document-requirements/{requirement_snapshot_id}/files", h.uploadFile)
	mux.HandleFunc("GET /api/v1/company/deadlines/{record_id}/steps/{step_code}/document-requirements/files/{file_id}/content", h.downloadFile)
	mux.HandleFunc("DELETE /api/v1/company/deadlines/{record_id}/steps/{step_code}/document-requirements/files/{file_id}", h.deleteFile)
	mux.HandleFunc("POST /api/v1/company/deadlines/{record_id}/steps/{step_code}/document-requirements/files/{file_id}/replace", h.replaceFile)
}

func (h *Handler) subject(r *http.Request) (wff.Subject, error) {
	claims, err := h.inspector.InspectAccessToken(r.Context(), bearerToken(r.Header.Get("Authorization")))
	if err != nil {
		return wff.Subject{}, err
	}
	return wff.Subject{
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

func (h *Handler) listRequirements(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	resp, err := h.svc.ListRequirements(r.Context(), sub, r.PathValue("record_id"), r.PathValue("step_code"))
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	if resp.Requirements == nil {
		resp.Requirements = []wff.RequirementDTO{}
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) uploadFile(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	fileName, contentType, body, size, err := parseMultipartFile(r)
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	defer body.Close()
	result, err := h.svc.Upload(
		r.Context(), sub,
		r.PathValue("record_id"),
		r.PathValue("step_code"),
		r.PathValue("requirement_snapshot_id"),
		fileName, contentType, body, size,
	)
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, result)
}

func (h *Handler) downloadFile(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	file, data, err := h.svc.Download(r.Context(), sub, r.PathValue("record_id"), r.PathValue("step_code"), r.PathValue("file_id"))
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	w.Header().Set("Content-Type", file.MimeType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+wff.EscapeContentDispositionFileName(file.OriginalFileName)+`"`)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h *Handler) deleteFile(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	if err := h.svc.Delete(r.Context(), sub, r.PathValue("record_id"), r.PathValue("step_code"), r.PathValue("file_id")); err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) replaceFile(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	fileName, contentType, body, size, err := parseMultipartFile(r)
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	defer body.Close()
	result, err := h.svc.Replace(
		r.Context(), sub,
		r.PathValue("record_id"),
		r.PathValue("step_code"),
		r.PathValue("file_id"),
		fileName, contentType, body, size,
	)
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, result)
}

func parseMultipartFile(r *http.Request) (fileName, contentType string, body io.ReadCloser, size int64, err error) {
	if err := r.ParseMultipartForm(maxMultipartMemory); err != nil {
		return "", "", nil, 0, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid multipart form", err)
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		return "", "", nil, 0, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "file is required", err)
	}
	fileName = header.Filename
	if v := strings.TrimSpace(r.FormValue("file_name")); v != "" {
		fileName = v
	}
	contentType = strings.TrimSpace(header.Header.Get("Content-Type"))
	if v := strings.TrimSpace(r.FormValue("content_type")); v != "" {
		contentType = v
	}
	size = header.Size
	if size <= 0 {
		size = wff.MaxSizeBytes
	}
	return fileName, contentType, file, size, nil
}
