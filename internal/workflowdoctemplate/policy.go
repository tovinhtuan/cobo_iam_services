package workflowdoctemplate

import (
	"net/http"
	"path/filepath"
	"strings"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// Allowed MIME types for workflow document templates (purpose-scoped; does not widen CMS media).
var allowedContentTypes = map[string]struct{}{
	"application/pdf": {},
	"application/msword": {},
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": {},
	"application/vnd.ms-excel": {},
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": {},
	"text/csv":    {},
	"text/plain":  {},
	"image/png":   {},
	"image/jpeg":  {},
	"image/webp":  {},
	"image/gif":   {},
}

// extension → allowed MIME set (must match declared content-type).
var extensionAllowedMIME = map[string]map[string]struct{}{
	".pdf":  {"application/pdf": {}},
	".doc":  {"application/msword": {}},
	".docx": {"application/vnd.openxmlformats-officedocument.wordprocessingml.document": {}},
	".xls":  {"application/vnd.ms-excel": {}},
	".xlsx": {"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": {}},
	".csv":  {"text/csv": {}, "text/plain": {}},
	".txt":  {"text/plain": {}},
	".png":  {"image/png": {}},
	".jpg":  {"image/jpeg": {}},
	".jpeg": {"image/jpeg": {}},
	".webp": {"image/webp": {}},
	".gif":  {"image/gif": {}},
}

func sanitizeFileName(name string) string {
	name = strings.TrimSpace(name)
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, "..", "")
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == string(filepath.Separator) {
		return ""
	}
	return name
}

func validateUploadMeta(fileName, contentType string, sizeBytes int64) error {
	fileName = sanitizeFileName(fileName)
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if fileName == "" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "file_name is required", nil)
	}
	if contentType == "" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "content_type is required", nil)
	}
	if sizeBytes <= 0 {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "size_bytes must be positive", nil)
	}
	if sizeBytes > MaxSizeBytes {
		return perr.NewHTTPError(http.StatusRequestEntityTooLarge, perr.CodeInvalidRequest, "file too large", nil)
	}
	if _, ok := allowedContentTypes[contentType]; !ok {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "unsupported content_type", nil)
	}
	ext := strings.ToLower(filepath.Ext(fileName))
	if ext == "" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "file extension required", nil)
	}
	allowedForExt, ok := extensionAllowedMIME[ext]
	if !ok {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "unsupported file extension", nil)
	}
	if _, ok := allowedForExt[contentType]; !ok {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "content_type does not match file extension", nil)
	}
	return nil
}

// CanDownload reports whether actorCompanyID may download the asset.
func CanDownload(asset *Asset, actorCompanyID string, isCMSAdmin bool) bool {
	if asset == nil || asset.State != StateReady {
		return false
	}
	if isCMSAdmin {
		return true
	}
	actorCompanyID = strings.TrimSpace(actorCompanyID)
	if actorCompanyID == "" {
		return false
	}
	if asset.OwnerScope == OwnerScopeCMS {
		return true // CMS template files are readable by company tenants that see the workflow
	}
	return asset.OwnerScope == OwnerScopeCompany && asset.CompanyID == actorCompanyID
}

// CanBindForWorkflowSave validates that a file_id may be referenced from a workflow snapshot.
// bindScope is OwnerScopeCMS or OwnerScopeCompany; bindCompanyID is the workflow owner's company.
func CanBindForWorkflowSave(asset *Asset, bindScope, bindCompanyID string) bool {
	if asset == nil || asset.State != StateReady {
		return false
	}
	bindCompanyID = strings.TrimSpace(bindCompanyID)
	switch bindScope {
	case OwnerScopeCMS:
		return asset.OwnerScope == OwnerScopeCMS
	case OwnerScopeCompany:
		if asset.OwnerScope == OwnerScopeCMS {
			return true // company may keep inherited CMS file reference
		}
		return asset.OwnerScope == OwnerScopeCompany && asset.CompanyID == bindCompanyID
	default:
		return false
	}
}
