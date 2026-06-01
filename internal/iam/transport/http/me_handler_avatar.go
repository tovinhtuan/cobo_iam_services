package http

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
)

const avatarMultipartMaxBytes = int64(2<<20) + 4096 // 2 MB + form overhead

func (m *MeHandler) avatarUploadMultipart(w http.ResponseWriter, r *http.Request) {
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

	r.Body = http.MaxBytesReader(w, r.Body, avatarMultipartMaxBytes)
	if err := r.ParseMultipartForm(avatarMultipartMaxBytes); err != nil {
		if strings.Contains(err.Error(), "too large") {
			httpx.WriteError(w, m.h.log, perr.NewHTTPError(http.StatusRequestEntityTooLarge, perr.CodeInvalidRequest, "file too large", err))
		} else {
			httpx.WriteError(w, m.h.log, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid multipart form", err))
		}
		return
	}

	file, header, err := r.FormFile("avatar")
	if err != nil {
		httpx.WriteError(w, m.h.log, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "avatar field required", err))
		return
	}
	defer file.Close()

	contentType := strings.ToLower(strings.TrimSpace(header.Header.Get("Content-Type")))
	if contentType == "" {
		// sniff from first 512 bytes
		buf := make([]byte, 512)
		n, _ := file.Read(buf)
		contentType = strings.ToLower(http.DetectContentType(buf[:n]))
		// seek back — multipart.File implements io.ReadSeeker
		if s, ok := file.(io.ReadSeeker); ok {
			_, _ = s.Seek(0, io.SeekStart)
		}
	}

	baseURL := strings.TrimSpace(m.publicAPIBaseURL)
	if baseURL == "" {
		baseURL = resolveMeRequestBaseURL(r)
	}

	out, err := m.avatars.UploadMultipart(r.Context(), claims.Sub, baseURL, contentType, file, header.Size)
	if err != nil {
		httpx.WriteError(w, m.h.log, err)
		return
	}

	m.auditLog(r, claims, "user.avatar.upload", "user", claims.Sub, nil)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"avatar_url":        out.AvatarURL,
		"avatar_updated_at": out.AvatarUpdatedAt.UTC().Format(time.RFC3339),
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

