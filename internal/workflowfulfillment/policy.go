package workflowfulfillment

import (
	"net/http"
	"path/filepath"
	"strings"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// Allowed MIME types — same allowlist as workflow document templates.
var allowedContentTypes = map[string]struct{}{
	"application/pdf":    {},
	"application/msword": {},
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": {},
	"application/vnd.ms-excel": {},
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": {},
	"text/csv":   {},
	"text/plain": {},
	"image/png":  {},
	"image/jpeg": {},
	"image/webp": {},
	"image/gif":  {},
}

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

// SanitizeFileName strips path components and injection characters for metadata / Content-Disposition.
// Must never be used alone as a storage path authority.
func SanitizeFileName(name string) string {
	name = strings.TrimSpace(name)
	// Normalize Windows separators before filepath.Base (Linux Base does not strip `\`).
	name = strings.ReplaceAll(name, "\\", "/")
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, "..", "")
	name = strings.ReplaceAll(name, "\n", "")
	name = strings.ReplaceAll(name, "\r", "")
	name = strings.ReplaceAll(name, `"`, "")
	name = strings.ReplaceAll(name, "/", "")
	name = strings.ReplaceAll(name, "\\", "")
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == string(filepath.Separator) {
		return ""
	}
	return name
}

// ValidateUploadMeta enforces size + MIME/extension allowlist.
func ValidateUploadMeta(fileName, contentType string, sizeBytes int64) error {
	fileName = SanitizeFileName(fileName)
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if fileName == "" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "file_name is required", nil)
	}
	if contentType == "" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeDocumentFulfillmentFileTypeInvalid, "content_type is required", nil)
	}
	if sizeBytes <= 0 {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "empty file is not allowed", nil)
	}
	if sizeBytes > MaxSizeBytes {
		return perr.NewHTTPError(http.StatusRequestEntityTooLarge, perr.CodeDocumentFulfillmentFileTooLarge, "file too large", nil)
	}
	if _, ok := allowedContentTypes[contentType]; !ok {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeDocumentFulfillmentFileTypeInvalid, "unsupported content_type", nil)
	}
	ext := strings.ToLower(filepath.Ext(fileName))
	if ext == "" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeDocumentFulfillmentFileTypeInvalid, "file extension required", nil)
	}
	allowedForExt, ok := extensionAllowedMIME[ext]
	if !ok {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeDocumentFulfillmentFileTypeInvalid, "unsupported file extension", nil)
	}
	if _, ok := allowedForExt[contentType]; !ok {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeDocumentFulfillmentFileTypeInvalid, "content_type does not match file extension", nil)
	}
	return nil
}

// EscapeContentDispositionFileName sanitizes a filename for Content-Disposition.
func EscapeContentDispositionFileName(name string) string {
	name = SanitizeFileName(name)
	if name == "" {
		return "download"
	}
	return name
}

// GuessContentType maps extension to MIME (multipart fallback).
func GuessContentType(fileName string) string {
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
