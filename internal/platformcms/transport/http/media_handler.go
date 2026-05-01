package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
	"github.com/google/uuid"
)

const (
	cmsMediaMaxSizeBytes int64 = 20 * 1024 * 1024
)

var cmsMediaAllowedContentTypes = map[string]struct{}{
	"image/png":       {},
	"image/jpeg":      {},
	"image/webp":      {},
	"image/gif":       {},
	"application/pdf": {},
	"text/plain":      {},
}

func (h *Handler) createMediaUploadIntent(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireCMSAccess(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireAnyPermission(r.Context(), sub.MembershipID, sub.CompanyID, "rbac.manage", "system.settings"); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}

	var payload struct {
		FileName    string `json:"file_name"`
		ContentType string `json:"content_type"`
		SizeBytes   int64  `json:"size_bytes"`
		Context     string `json:"context"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid JSON payload", err))
		return
	}
	fileName := sanitizeFileName(payload.FileName)
	contentType := strings.ToLower(strings.TrimSpace(payload.ContentType))
	if fileName == "" || contentType == "" || payload.SizeBytes <= 0 {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "file_name, content_type, size_bytes are required", nil))
		return
	}
	if payload.SizeBytes > cmsMediaMaxSizeBytes {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusRequestEntityTooLarge, perr.CodeInvalidRequest, "file too large", nil))
		return
	}
	if _, ok := cmsMediaAllowedContentTypes[contentType]; !ok {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "unsupported content_type", nil))
		return
	}

	assetID := "asset_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	objectKey := fmt.Sprintf("company/%s/cms/media/%s/%s", sub.CompanyID, assetID, fileName)
	now := time.Now().UTC()
	exp := now.Add(h.mediaSigner.ttl)
	expUnix := exp.Unix()
	sig := h.mediaSigner.sign(assetID, sub.CompanyID, http.MethodPut, contentType, payload.SizeBytes, expUnix)
	uploadURL := fmt.Sprintf("%s/api/v1/platform/cms/media/upload/%s?exp=%d&sig=%s&company_id=%s&method=%s&content_type=%s&size_bytes=%d",
		resolveRequestBaseURL(r, h.mediaPublicBaseURL),
		url.PathEscape(assetID),
		expUnix,
		url.QueryEscape(sig),
		url.QueryEscape(sub.CompanyID),
		http.MethodPut,
		url.QueryEscape(contentType),
		payload.SizeBytes,
	)
	item := cmsMediaAsset{
		AssetID:      assetID,
		CompanyID:    sub.CompanyID,
		FileName:     fileName,
		ContentType:  contentType,
		SizeBytes:    payload.SizeBytes,
		Context:      strings.TrimSpace(payload.Context),
		ObjectKey:    objectKey,
		UploadURL:    uploadURL,
		UploadMethod: http.MethodPut,
		State:        "pending_upload",
		CreatedAt:    now,
		UpdatedBy:    sub.Sub,
		ExpiresAt:    exp,
	}
	if err := h.mediaRepo.CreateIntent(r.Context(), item); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}

	_ = h.auditSvc.AppendAuditLog(r.Context(), auditapp.AppendAuditLogRequest{
		ActorUserID:       sub.Sub,
		ActorMembershipID: sub.MembershipID,
		CompanyID:         sub.CompanyID,
		Action:            cmsActionMediaUploadIntent,
		ResourceType:      "cms_media_asset",
		ResourceID:        assetID,
		Decision:          "allow",
		Metadata: map[string]any{
			"content_type": contentType,
			"size_bytes":   payload.SizeBytes,
			"object_key":   objectKey,
		},
	})

	writeEnvelope(w, http.StatusCreated, map[string]any{
		"asset_id":    assetID,
		"object_key":  objectKey,
		"content_type": contentType,
		"size_bytes":  payload.SizeBytes,
		"state":       item.State,
		"upload": map[string]any{
			"method":     item.UploadMethod,
			"url":        uploadURL,
			"headers":    map[string]string{"content-type": contentType},
			"expires_at": exp.Format(timeLayout),
		},
		"constraints": map[string]any{
			"max_size_bytes":       cmsMediaMaxSizeBytes,
			"allowed_content_type": sortedAllowedMediaTypes(),
		},
	}, nil)
}

func (h *Handler) uploadMediaBinary(w http.ResponseWriter, r *http.Request) {
	assetID := strings.TrimSpace(r.PathValue("asset_id"))
	if assetID == "" {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "asset_id is required", nil))
		return
	}
	query := r.URL.Query()
	expUnix, err := strconv.ParseInt(strings.TrimSpace(query.Get("exp")), 10, 64)
	if err != nil || expUnix <= 0 {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid exp query", nil))
		return
	}
	companyID := strings.TrimSpace(query.Get("company_id"))
	expectedMethod := strings.ToUpper(strings.TrimSpace(query.Get("method")))
	contentType := strings.ToLower(strings.TrimSpace(query.Get("content_type")))
	sizeBytes, err := strconv.ParseInt(strings.TrimSpace(query.Get("size_bytes")), 10, 64)
	if err != nil || sizeBytes <= 0 {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid size_bytes query", nil))
		return
	}
	sig := strings.TrimSpace(query.Get("sig"))
	if companyID == "" || expectedMethod == "" || contentType == "" || sig == "" {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "missing signed query fields", nil))
		return
	}
	if expectedMethod != http.MethodPut || r.Method != http.MethodPut {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusMethodNotAllowed, perr.CodeInvalidRequest, "method not allowed for signed upload", nil))
		return
	}
	now := time.Now().UTC().Unix()
	if now > expUnix {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusUnauthorized, perr.CodeSessionExpired, "signed upload url expired", nil))
		return
	}
	if !h.mediaSigner.verify(sig, assetID, companyID, expectedMethod, contentType, sizeBytes, expUnix) {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusUnauthorized, perr.CodeSessionExpired, "invalid upload signature", nil))
		return
	}
	item, err := h.mediaRepo.GetByAssetID(r.Context(), assetID)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if item.CompanyID != companyID {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "company mismatch for signed upload", nil))
		return
	}
	if item.State != "pending_upload" {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusConflict, perr.CodeInvalidRequest, "media asset is not pending upload", nil))
		return
	}
	if time.Now().UTC().After(item.ExpiresAt) {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusUnauthorized, perr.CodeSessionExpired, "signed upload url expired", nil))
		return
	}
	body := http.MaxBytesReader(w, r.Body, sizeBytes)
	defer body.Close()
	written, err := h.mediaStorage.Write(item.ObjectKey, body)
	if err != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "failed to persist media upload", err))
		return
	}
	if written != sizeBytes {
		_ = h.mediaStorage.Delete(item.ObjectKey)
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "uploaded payload size mismatch", nil))
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "ok")
}

func (h *Handler) completeMediaUpload(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireCMSAccess(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireAnyPermission(r.Context(), sub.MembershipID, sub.CompanyID, "rbac.manage", "system.settings"); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	assetID := strings.TrimSpace(r.PathValue("asset_id"))
	if assetID == "" {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "asset_id is required", nil))
		return
	}
	var payload struct {
		ETag      string `json:"etag"`
		Checksum  string `json:"checksum"`
		SizeBytes int64  `json:"size_bytes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid JSON payload", err))
		return
	}
	item, err := h.mediaRepo.GetByCompany(r.Context(), sub.CompanyID, assetID)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if !h.mediaStorage.Exists(item.ObjectKey) {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusConflict, perr.CodeInvalidRequest, "media binary not uploaded yet", nil))
		return
	}
	item, err = h.mediaRepo.MarkComplete(r.Context(), sub.CompanyID, assetID, sub.Sub, payload.ETag, payload.Checksum, payload.SizeBytes)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}

	_ = h.auditSvc.AppendAuditLog(r.Context(), auditapp.AppendAuditLogRequest{
		ActorUserID:       sub.Sub,
		ActorMembershipID: sub.MembershipID,
		CompanyID:         sub.CompanyID,
		Action:            cmsActionMediaUploadComplete,
		ResourceType:      "cms_media_asset",
		ResourceID:        item.AssetID,
		Decision:          "allow",
		Metadata: map[string]any{
			"etag":      strings.TrimSpace(payload.ETag),
			"checksum":  strings.TrimSpace(payload.Checksum),
			"object_key": item.ObjectKey,
		},
	})

	writeEnvelope(w, http.StatusOK, map[string]any{
		"asset_id":     item.AssetID,
		"object_key":   item.ObjectKey,
		"content_type": item.ContentType,
		"size_bytes":   item.SizeBytes,
		"state":        item.State,
		"completed_at": item.CompletedAt.Format(timeLayout),
	}, nil)
}

func (h *Handler) listMedia(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireCMSAccess(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireAnyPermission(r.Context(), sub.MembershipID, sub.CompanyID, "rbac.manage", "system.settings"); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	limit := parsePositiveInt(r.URL.Query().Get("limit"), 50, 200)
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	typeFilter := strings.TrimSpace(r.URL.Query().Get("type"))
	cursor := strings.TrimSpace(r.URL.Query().Get("cursor"))

	items, err := h.mediaRepo.List(r.Context(), sub.CompanyID, typeFilter, query, cursor, limit)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		var completedAt any
		if item.CompletedAt != nil {
			completedAt = item.CompletedAt.UTC().Format(timeLayout)
		}
		out = append(out, map[string]any{
			"asset_id":      item.AssetID,
			"file_name":     item.FileName,
			"content_type":  item.ContentType,
			"size_bytes":    item.SizeBytes,
			"state":         item.State,
			"object_key":    item.ObjectKey,
			"context":       item.Context,
			"created_at":    item.CreatedAt.UTC().Format(timeLayout),
			"completed_at":  completedAt,
			"upload_method": item.UploadMethod,
		})
	}
	var nextCursor any
	if len(out) == limit {
		if last, ok := out[len(out)-1]["created_at"].(string); ok && strings.TrimSpace(last) != "" {
			nextCursor = last
		}
	}
	writeEnvelope(w, http.StatusOK, map[string]any{"items": out}, map[string]any{
		"total":       len(out),
		"limit":       limit,
		"next_cursor": nextCursor,
	})
}

func (h *Handler) deleteMedia(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireCMSAccess(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireAnyPermission(r.Context(), sub.MembershipID, sub.CompanyID, "rbac.manage", "system.settings"); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	assetID := strings.TrimSpace(r.PathValue("asset_id"))
	if assetID == "" {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "asset_id is required", nil))
		return
	}
	current, err := h.mediaRepo.GetByCompany(r.Context(), sub.CompanyID, assetID)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if err := h.mediaStorage.Delete(current.ObjectKey); err != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "failed to delete media binary", err))
		return
	}
	item, err := h.mediaRepo.MarkDeleted(r.Context(), sub.CompanyID, assetID, sub.Sub)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	_ = h.auditSvc.AppendAuditLog(r.Context(), auditapp.AppendAuditLogRequest{
		ActorUserID:       sub.Sub,
		ActorMembershipID: sub.MembershipID,
		CompanyID:         sub.CompanyID,
		Action:            cmsActionMediaDelete,
		ResourceType:      "cms_media_asset",
		ResourceID:        item.AssetID,
		Decision:          "allow",
		Metadata: map[string]any{
			"object_key": item.ObjectKey,
		},
	})
	writeEnvelope(w, http.StatusOK, map[string]any{
		"asset_id":   item.AssetID,
		"state":      item.State,
		"deleted_at": item.DeletedAt.UTC().Format(timeLayout),
	}, nil)
}

func sanitizeFileName(name string) string {
	base := strings.TrimSpace(filepath.Base(name))
	base = strings.ReplaceAll(base, " ", "_")
	base = strings.ReplaceAll(base, "..", "")
	if base == "." || base == "/" || base == `\` {
		return ""
	}
	return base
}

func sortedAllowedMediaTypes() []string {
	items := make([]string, 0, len(cmsMediaAllowedContentTypes))
	for item := range cmsMediaAllowedContentTypes {
		items = append(items, item)
	}
	sort.Strings(items)
	return items
}

func resolveRequestBaseURL(r *http.Request, configuredBaseURL string) string {
	if base := strings.TrimSpace(configuredBaseURL); base != "" {
		return strings.TrimRight(base, "/")
	}
	scheme := "http"
	if strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https") || r.TLS != nil {
		scheme = "https"
	}
	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = strings.TrimSpace(r.Host)
	}
	return fmt.Sprintf("%s://%s", scheme, host)
}
