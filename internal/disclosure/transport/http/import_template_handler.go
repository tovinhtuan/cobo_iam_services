package http

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
)

// cmsValidateTemplateImport handles POST /api/v1/platform/cms/templates/import/validate.
// It parses the uploaded multipart .json file with strict transport and file size guards,
// runs the normalization and domain validation engine, and returns stateless preview and HMAC token.
func (h *Handler) cmsValidateTemplateImport(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}

	// 1. Content-Type guard: strictly multipart/form-data. Raw JSON is not accepted.
	contentType := r.Header.Get("Content-Type")
	mediaType, _, _ := mime.ParseMediaType(contentType)
	if mediaType != "multipart/form-data" {
		httpx.WriteError(w, nil, perr.NewHTTPError(
			http.StatusUnsupportedMediaType,
			perr.CodeInvalidRequest,
			"yêu cầu Content-Type multipart/form-data với trường 'file' (không hỗ trợ raw JSON)",
			nil,
		))
		return
	}

	// 2. Safe file reading using http.MaxBytesReader to guard total request body
	// Allow 64 KiB margin for multipart boundary and headers beyond file payload
	r.Body = http.MaxBytesReader(w, r.Body, int64(disclosureapp.MaxTemplateImportFileSizeBytes)+64*1024)

	mr, err := r.MultipartReader()
	if err != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(
			http.StatusBadRequest,
			perr.CodeInvalidRequest,
			"lỗi khởi tạo multipart reader: "+err.Error(),
			nil,
		))
		return
	}

	var fileBytes []byte
	var fileName string
	partCount := 0

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			errStr := strings.ToLower(err.Error())
			if strings.Contains(errStr, "request body too large") || strings.Contains(errStr, "http: request body too large") {
				httpx.WriteError(w, nil, perr.NewHTTPError(
					http.StatusRequestEntityTooLarge,
					perr.CodeInvalidRequest,
					fmt.Sprintf("kích thước tập tin vượt quá giới hạn %d bytes (2 MiB)", disclosureapp.MaxTemplateImportFileSizeBytes),
					nil,
				))
				return
			}
			httpx.WriteError(w, nil, perr.NewHTTPError(
				http.StatusBadRequest,
				perr.CodeInvalidRequest,
				"lỗi đọc dữ liệu multipart: "+err.Error(),
				nil,
			))
			return
		}

		if part.FormName() == "file" {
			partCount++
			if partCount > 1 {
				_ = part.Close()
				httpx.WriteError(w, nil, perr.NewHTTPError(
					http.StatusBadRequest,
					perr.CodeInvalidRequest,
					"yêu cầu chính xác duy nhất 1 tập tin trong trường 'file'",
					nil,
				))
				return
			}

			fileName = strings.TrimSpace(part.FileName())
			if fileName == "" {
				_ = part.Close()
				httpx.WriteError(w, nil, perr.NewHTTPError(
					http.StatusBadRequest,
					perr.CodeInvalidRequest,
					"tên tập tin không được để trống",
					nil,
				))
				return
			}

			// Extension check: case-insensitively .json
			ext := strings.ToLower(filepath.Ext(fileName))
			if ext != ".json" {
				_ = part.Close()
				httpx.WriteError(w, nil, perr.NewHTTPError(
					http.StatusUnsupportedMediaType,
					perr.CodeInvalidRequest,
					fmt.Sprintf("phần mở rộng tập tin %q không được hỗ trợ (chỉ chấp nhận .json)", ext),
					nil,
				))
				return
			}

			// Read with LimitReader (+1 byte detection for exact 2 MiB boundary)
			lr := io.LimitReader(part, int64(disclosureapp.MaxTemplateImportFileSizeBytes)+1)
			data, readErr := io.ReadAll(lr)
			_ = part.Close()
			if readErr != nil {
				httpx.WriteError(w, nil, perr.NewHTTPError(
					http.StatusBadRequest,
					perr.CodeInvalidRequest,
					"lỗi đọc nội dung tập tin: "+readErr.Error(),
					nil,
				))
				return
			}

			if len(data) > disclosureapp.MaxTemplateImportFileSizeBytes {
				httpx.WriteError(w, nil, perr.NewHTTPError(
					http.StatusRequestEntityTooLarge,
					perr.CodeInvalidRequest,
					fmt.Sprintf("kích thước tập tin vượt quá giới hạn %d bytes (2 MiB)", disclosureapp.MaxTemplateImportFileSizeBytes),
					nil,
				))
				return
			}

			if len(data) == 0 {
				httpx.WriteError(w, nil, perr.NewHTTPError(
					http.StatusBadRequest,
					perr.CodeInvalidRequest,
					"tập tin tải lên không có nội dung (rỗng)",
					nil,
				))
				return
			}

			// Sniff first bytes to reject clearly incompatible binary formats
			sniffLen := 512
			if len(data) < sniffLen {
				sniffLen = len(data)
			}
			sniffedMIME := http.DetectContentType(data[:sniffLen])
			if isClearlyIncompatibleBinaryMIME(sniffedMIME) {
				httpx.WriteError(w, nil, perr.NewHTTPError(
					http.StatusUnsupportedMediaType,
					perr.CodeInvalidRequest,
					fmt.Sprintf("nội dung tập tin là binary (%s), không phải định dạng văn bản JSON", sniffedMIME),
					nil,
				))
				return
			}

			fileBytes = data
		} else {
			_ = part.Close()
		}
	}

	if partCount == 0 || len(fileBytes) == 0 {
		httpx.WriteError(w, nil, perr.NewHTTPError(
			http.StatusBadRequest,
			perr.CodeInvalidRequest,
			"thiếu trường tập tin 'file' trong multipart request",
			nil,
		))
		return
	}

	// 3. Delegate to service layer
	resp, err := h.svc.ValidateTemplateImport(r.Context(), disclosureapp.ValidateTemplateImportRequest{
		Subject:   sub,
		Filename:  fileName,
		FileBytes: fileBytes,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, resp)
}

func isClearlyIncompatibleBinaryMIME(mime string) bool {
	m := strings.ToLower(strings.TrimSpace(mime))
	if strings.HasPrefix(m, "application/zip") ||
		strings.HasPrefix(m, "application/pdf") ||
		strings.HasPrefix(m, "image/") ||
		strings.HasPrefix(m, "application/x-executable") ||
		strings.HasPrefix(m, "application/vnd.openxmlformats") ||
		strings.HasPrefix(m, "application/vnd.ms-excel") ||
		strings.HasPrefix(m, "application/gzip") ||
		strings.HasPrefix(m, "application/x-tar") {
		return true
	}
	return false
}

// cmsConfirmTemplateImport handles POST /api/v1/platform/cms/templates/import/confirm.
// It verifies the validation token and payload hash binding, revalidates mutable target references,
// maps departments, and commits the new template root + Draft v1 in an atomic transaction.
func (h *Handler) cmsConfirmTemplateImport(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}

	var req disclosureapp.ConfirmTemplateImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(
			http.StatusBadRequest,
			perr.CodeInvalidRequest,
			"định dạng dữ liệu JSON không hợp lệ: "+err.Error(),
			nil,
		))
		return
	}
	req.Subject = sub

	resp, err := h.svc.ConfirmTemplateImport(r.Context(), req)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}

	// Post-commit best-effort audit log (AUDIT_ATOMICITY_MODEL=POST_COMMIT)
	// Never audits raw validation token, HMAC secret, auth token, or full imported template JSON.
	payloadHash, _ := disclosureapp.ComputeCanonicalTemplatePayloadHash(&req.NormalizedTemplate)
	h.auditLog(r, sub, "disclosure.type.import", "disclosure_type", resp.TypeID, map[string]any{
		"creation_mode":  "TEMPLATE_IMPORT",
		"target_type_id": resp.TypeID,
		"version_no":     resp.VersionNo,
		"schema_version": disclosureapp.TemplateImportSchemaVersion,
		"payload_hash":   payloadHash,
		"actor_id":       sub.UserID,
	})

	httpx.WriteJSON(w, http.StatusCreated, resp)
}

