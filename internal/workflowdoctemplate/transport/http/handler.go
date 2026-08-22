package http

import (
	"context"
	"net/http"
	"path/filepath"
	"strings"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
	wdt "github.com/cobo/cobo_iam_services/internal/workflowdoctemplate"
)

const maxMultipartMemory = 32 << 20 // 32MiB form buffer; file size still capped at 20MiB

// Handler serves CMS + Company workflow document template upload/download.
type Handler struct {
	svc        *wdt.Service
	inspector  iamapp.TokenInspector
	authorizer authapp.Service
}

// NewHandler wires HTTP transport.
func NewHandler(svc *wdt.Service, inspector iamapp.TokenInspector, authorizer authapp.Service) *Handler {
	return &Handler{svc: svc, inspector: inspector, authorizer: authorizer}
}

// Register mounts routes on mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/platform/cms/workflow-document-templates/upload", h.uploadCMS)
	mux.HandleFunc("GET /api/v1/platform/cms/workflow-document-templates/{file_id}/content", h.downloadCMS)
	mux.HandleFunc("POST /api/v1/company/workflow-document-templates/upload", h.uploadCompany)
	mux.HandleFunc("GET /api/v1/company/workflow-document-templates/{file_id}/content", h.downloadCompany)
}

type subject struct {
	UserID       string
	MembershipID string
	CompanyID    string
}

func (h *Handler) subject(r *http.Request) (subject, error) {
	claims, err := h.inspector.InspectAccessToken(r.Context(), bearerToken(r.Header.Get("Authorization")))
	if err != nil {
		return subject{}, err
	}
	return subject{UserID: claims.Sub, MembershipID: claims.MembershipID, CompanyID: claims.CompanyID}, nil
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

func (h *Handler) requireAnyPermission(ctx context.Context, membershipID, companyID string, permissions ...string) error {
	if h.authorizer == nil {
		return perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeInternal, "authorizer unavailable", nil)
	}
	eff, err := h.authorizer.GetEffectiveAccess(ctx, membershipID, companyID)
	if err != nil {
		return err
	}
	if !hasAnyPermission(eff.Permissions, permissions...) {
		return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "access denied", nil)
	}
	return nil
}

func hasAnyPermission(items []string, expected ...string) bool {
	for _, candidate := range expected {
		for _, item := range items {
			if strings.EqualFold(strings.TrimSpace(item), strings.TrimSpace(candidate)) {
				return true
			}
		}
	}
	return false
}

func (h *Handler) requireCMSEditor(ctx context.Context, membershipID, companyID string) error {
	if err := h.requireAnyPermission(ctx, membershipID, companyID, "platform.cms.view"); err != nil {
		return err
	}
	return h.requireAnyPermission(ctx, membershipID, companyID, "rbac.manage", "system.settings")
}

func (h *Handler) uploadCMS(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if err := h.requireCMSEditor(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.handleUpload(w, r, wdt.OwnerScopeCMS, sub)
}

func (h *Handler) uploadCompany(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if err := h.requireAnyPermission(r.Context(), sub.MembershipID, sub.CompanyID, "template.workflow.override.write"); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.handleUpload(w, r, wdt.OwnerScopeCompany, sub)
}

func (h *Handler) handleUpload(w http.ResponseWriter, r *http.Request, ownerScope string, sub subject) {
	if err := r.ParseMultipartForm(maxMultipartMemory); err != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid multipart form", err))
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "file is required", err))
		return
	}
	defer file.Close()

	fileName := header.Filename
	if v := strings.TrimSpace(r.FormValue("file_name")); v != "" {
		fileName = v
	}
	contentType := strings.TrimSpace(header.Header.Get("Content-Type"))
	if v := strings.TrimSpace(r.FormValue("content_type")); v != "" {
		contentType = v
	}
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = guessContentType(fileName)
	}
	sizeBytes := header.Size
	if sizeBytes <= 0 {
		sizeBytes = wdt.MaxSizeBytes
	}

	result, err := h.svc.UploadMultipart(r.Context(), ownerScope, sub.CompanyID, sub.UserID, fileName, contentType, file, sizeBytes)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, result)
}

func (h *Handler) downloadCMS(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if err := h.requireCMSEditor(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.handleDownload(w, r, sub, true)
}

func (h *Handler) downloadCompany(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if err := h.requireAnyPermission(r.Context(), sub.MembershipID, sub.CompanyID,
		"template.workflow.override.read",
		"template.workflow.override.write",
		"disclosure.view",
	); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.handleDownload(w, r, sub, false)
}

func (h *Handler) handleDownload(w http.ResponseWriter, r *http.Request, sub subject, isCMSAdmin bool) {
	fileID := strings.TrimSpace(r.PathValue("file_id"))
	asset, data, err := h.svc.ReadContent(r.Context(), fileID)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if !wdt.CanDownload(asset, sub.CompanyID, isCMSAdmin) {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "permission denied", nil))
		return
	}
	w.Header().Set("Content-Type", asset.ContentType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+escapeFileName(asset.FileName)+`"`)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func escapeFileName(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, `"`, "")
	name = strings.ReplaceAll(name, "\n", "")
	name = strings.ReplaceAll(name, "\r", "")
	if name == "" {
		return "download"
	}
	return name
}

func guessContentType(fileName string) string {
	switch strings.ToLower(filepath.Ext(fileName)) {
	case ".pdf":
		return "application/pdf"
	case ".doc":
		return "application/msword"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".xls":
		return "application/vnd.ms-excel"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".csv":
		return "text/csv"
	case ".txt":
		return "text/plain"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return ""
	}
}
