package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
	"github.com/cobo/cobo_iam_services/internal/platform/mediaupload"
)

func (m *MeHandler) avatarUploadIntent(w http.ResponseWriter, r *http.Request) {
	claims, err := m.h.inspector.InspectAccessToken(r.Context(), bearerToken(r.Header.Get("Authorization")))
	if err != nil {
		httpx.WriteError(w, m.h.log, err)
		return
	}
	if m.avatars == nil {
		httpx.WriteError(w, m.h.log, perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeInternal, "avatar upload not configured", nil))
		return
	}
	if err := m.requireUnlockedAccount(r, claims.Sub); err != nil {
		httpx.WriteError(w, m.h.log, err)
		return
	}
	var body struct {
		FileName    string `json:"file_name"`
		ContentType string `json:"content_type"`
		SizeBytes   int64  `json:"size_bytes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, m.h.log, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid json body", err))
		return
	}
	baseURL := strings.TrimSpace(m.publicAPIBaseURL)
	if baseURL == "" {
		baseURL = resolveMeRequestBaseURL(r)
	}
	out, err := m.avatars.CreateUploadIntent(r.Context(), claims.Sub, baseURL, iamapp.AvatarUploadIntentRequest{
		FileName:    body.FileName,
		ContentType: body.ContentType,
		SizeBytes:   body.SizeBytes,
	})
	if err != nil {
		httpx.WriteError(w, m.h.log, err)
		return
	}
	m.auditLog(r, claims, "user.avatar.upload.intent", "user", claims.Sub, map[string]any{
		"asset_id": out.AssetID,
	})
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{
		"asset_id":   out.AssetID,
		"upload_url": out.UploadURL,
		"expires_at": out.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

func (m *MeHandler) avatarUploadBinary(w http.ResponseWriter, r *http.Request) {
	if m.avatars == nil {
		httpx.WriteError(w, m.h.log, perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeInternal, "avatar upload not configured", nil))
		return
	}
	assetID := strings.TrimSpace(r.PathValue("asset_id"))
	if assetID == "" {
		httpx.WriteError(w, m.h.log, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "asset_id is required", nil))
		return
	}
	q := r.URL.Query()
	expUnix, err := strconv.ParseInt(strings.TrimSpace(q.Get("exp")), 10, 64)
	if err != nil || expUnix <= 0 {
		httpx.WriteError(w, m.h.log, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid exp query", nil))
		return
	}
	userID := strings.TrimSpace(q.Get("user_id"))
	expectedMethod := strings.ToUpper(strings.TrimSpace(q.Get("method")))
	contentType := strings.ToLower(strings.TrimSpace(q.Get("content_type")))
	sizeBytes, err := strconv.ParseInt(strings.TrimSpace(q.Get("size_bytes")), 10, 64)
	if err != nil || sizeBytes <= 0 {
		httpx.WriteError(w, m.h.log, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid size_bytes query", nil))
		return
	}
	sig := strings.TrimSpace(q.Get("sig"))
	if userID == "" || expectedMethod == "" || contentType == "" || sig == "" {
		httpx.WriteError(w, m.h.log, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "missing signed query fields", nil))
		return
	}
	if expectedMethod != http.MethodPut || r.Method != http.MethodPut {
		httpx.WriteError(w, m.h.log, perr.NewHTTPError(http.StatusMethodNotAllowed, perr.CodeInvalidRequest, "method not allowed for signed upload", nil))
		return
	}
	exp := time.Unix(expUnix, 0).UTC()
	signIn := mediaupload.SignInput{
		Purpose:     mediaupload.PurposeUserAvatarUpload,
		OwnerID:     userID,
		AssetID:     assetID,
		Method:      http.MethodPut,
		Path:        "/api/v1/me/avatar/upload/" + url.PathEscape(assetID),
		ContentType: contentType,
		SizeBytes:   sizeBytes,
		ExpiresAt:   exp,
	}
	if !m.avatars.VerifySignature(sig, signIn) {
		httpx.WriteError(w, m.h.log, perr.NewHTTPError(http.StatusUnauthorized, perr.CodeSessionExpired, "invalid upload signature", nil))
		return
	}
	asset, err := m.avatars.GetAssetForSignedUpload(r.Context(), userID, assetID)
	if err != nil {
		httpx.WriteError(w, m.h.log, err)
		return
	}
	if asset.ContentType != contentType || asset.SizeBytes != sizeBytes {
		httpx.WriteError(w, m.h.log, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "signed upload metadata mismatch", nil))
		return
	}
	body := http.MaxBytesReader(w, r.Body, sizeBytes+1)
	defer body.Close()
	if err := m.avatars.WriteSignedUpload(r.Context(), asset, contentType, sizeBytes, body); err != nil {
		httpx.WriteError(w, m.h.log, err)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "ok")
}

func (m *MeHandler) avatarComplete(w http.ResponseWriter, r *http.Request) {
	claims, err := m.h.inspector.InspectAccessToken(r.Context(), bearerToken(r.Header.Get("Authorization")))
	if err != nil {
		httpx.WriteError(w, m.h.log, err)
		return
	}
	if m.avatars == nil {
		httpx.WriteError(w, m.h.log, perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeInternal, "avatar upload not configured", nil))
		return
	}
	assetID := strings.TrimSpace(r.PathValue("asset_id"))
	if assetID == "" {
		httpx.WriteError(w, m.h.log, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "asset_id is required", nil))
		return
	}
	out, err := m.avatars.Complete(r.Context(), claims.Sub, assetID)
	if err != nil {
		httpx.WriteError(w, m.h.log, err)
		return
	}
	m.auditLog(r, claims, "user.avatar.upload.complete", "user_avatar_asset", out.AssetID, map[string]any{
		"user_id": claims.Sub,
	})
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"asset_id":     out.AssetID,
		"object_key":   out.ObjectKey,
		"content_type": out.ContentType,
		"size_bytes":   out.SizeBytes,
		"state":        out.State,
	})
}

func (m *MeHandler) avatarDelete(w http.ResponseWriter, r *http.Request) {
	claims, err := m.h.inspector.InspectAccessToken(r.Context(), bearerToken(r.Header.Get("Authorization")))
	if err != nil {
		httpx.WriteError(w, m.h.log, err)
		return
	}
	if m.avatars == nil {
		httpx.WriteError(w, m.h.log, perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeInternal, "avatar upload not configured", nil))
		return
	}
	if err := m.requireUnlockedAccount(r, claims.Sub); err != nil {
		httpx.WriteError(w, m.h.log, err)
		return
	}
	if err := m.avatars.Delete(r.Context(), claims.Sub); err != nil {
		httpx.WriteError(w, m.h.log, err)
		return
	}
	m.auditLog(r, claims, "user.avatar.delete", "user", claims.Sub, nil)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (m *MeHandler) avatarContent(w http.ResponseWriter, r *http.Request) {
	if m.avatars == nil {
		httpx.WriteError(w, m.h.log, perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeInternal, "avatar content not configured", nil))
		return
	}
	userID, err := m.resolveAvatarContentUserID(r)
	if err != nil {
		httpx.WriteError(w, m.h.log, err)
		return
	}
	data, contentType, err := m.avatars.GetContentBytes(r.Context(), userID)
	if err != nil {
		httpx.WriteError(w, m.h.log, err)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, max-age=60")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (m *MeHandler) resolveAvatarContentUserID(r *http.Request) (string, error) {
	q := r.URL.Query()
	sig := strings.TrimSpace(q.Get("sig"))
	expRaw := strings.TrimSpace(q.Get("exp"))
	userID := strings.TrimSpace(q.Get("user_id"))
	if sig != "" || expRaw != "" || userID != "" {
		expUnix, err := strconv.ParseInt(expRaw, 10, 64)
		if err != nil || expUnix <= 0 {
			return "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid exp query", nil)
		}
		expectedMethod := strings.ToUpper(strings.TrimSpace(q.Get("method")))
		if userID == "" || sig == "" || expectedMethod != http.MethodGet {
			return "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "missing signed query fields", nil)
		}
		if r.Method != http.MethodGet {
			return "", perr.NewHTTPError(http.StatusMethodNotAllowed, perr.CodeInvalidRequest, "method not allowed", nil)
		}
		exp := time.Unix(expUnix, 0).UTC()
		signIn := m.avatars.BuildContentSignInput(userID, exp)
		if !m.avatars.VerifySignature(sig, signIn) {
			return "", perr.NewHTTPError(http.StatusUnauthorized, perr.CodeSessionExpired, "invalid content signature", nil)
		}
		return userID, nil
	}
	claims, err := m.h.inspector.InspectAccessToken(r.Context(), bearerToken(r.Header.Get("Authorization")))
	if err != nil {
		return "", err
	}
	return claims.Sub, nil
}

func (m *MeHandler) requireUnlockedAccount(r *http.Request, userID string) error {
	if m.profiles == nil {
		return nil
	}
	current, err := m.profiles.GetAdminAccountSettings(r.Context(), userID)
	if err != nil {
		return err
	}
	if isAccountLocked(current.AccountStatus) {
		return perr.NewHTTPError(http.StatusForbidden, perr.CodeInvalidRequest, "account is locked", nil)
	}
	return nil
}

func resolveMeRequestBaseURL(r *http.Request) string {
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
