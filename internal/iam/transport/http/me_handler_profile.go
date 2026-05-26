package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	iamregmysql "github.com/cobo/cobo_iam_services/internal/iam/registrationmysql"
	"github.com/cobo/cobo_iam_services/internal/iam/loginpassword"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
)

// userAccountProfileRepo is the shared persistence surface for self-service and admin account profile patches.
type userAccountProfileRepo interface {
	GetAdminAccountSettings(ctx context.Context, userID string) (*caapp.AdminAccountSettingsView, error)
	PatchAdminAccountSettings(ctx context.Context, userID string, fullName, email, phone *string) error
}

func (m *MeHandler) patchProfile(w http.ResponseWriter, r *http.Request) {
	claims, err := m.h.inspector.InspectAccessToken(r.Context(), bearerToken(r.Header.Get("Authorization")))
	if err != nil {
		httpx.WriteError(w, m.h.log, err)
		return
	}
	if m.profiles == nil {
		httpx.WriteError(w, m.h.log, perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeInternal, "profile update not configured", nil))
		return
	}
	var body struct {
		FullName *string `json:"full_name"`
		Email    *string `json:"email"`
		Phone    *string `json:"phone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, m.h.log, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid json body", err))
		return
	}
	if body.FullName == nil && body.Email == nil && body.Phone == nil {
		httpx.WriteError(w, m.h.log, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "no fields to update", nil))
		return
	}
	current, err := m.profiles.GetAdminAccountSettings(r.Context(), claims.Sub)
	if err != nil {
		httpx.WriteError(w, m.h.log, err)
		return
	}
	if isAccountLocked(current.AccountStatus) {
		httpx.WriteError(w, m.h.log, perr.NewHTTPError(http.StatusForbidden, perr.CodeInvalidRequest, "account is locked", nil))
		return
	}
	if err := m.profiles.PatchAdminAccountSettings(r.Context(), claims.Sub, body.FullName, body.Email, body.Phone); err != nil {
		httpx.WriteError(w, m.h.log, err)
		return
	}
	m.auditLog(r, claims, "user.profile.update", "user", claims.Sub)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (m *MeHandler) changePassword(w http.ResponseWriter, r *http.Request) {
	claims, err := m.h.inspector.InspectAccessToken(r.Context(), bearerToken(r.Header.Get("Authorization")))
	if err != nil {
		httpx.WriteError(w, m.h.log, err)
		return
	}
	if m.iamSvc == nil {
		httpx.WriteError(w, m.h.log, perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeInternal, "password change not configured", nil))
		return
	}
	if m.loginPWD == nil {
		httpx.WriteError(w, m.h.log, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "password_cipher is required (RSA not configured)", nil))
		return
	}
	var body struct {
		CurrentPasswordCipher *iamapp.LoginPasswordCipher `json:"current_password_cipher"`
		NewPasswordCipher     *iamapp.LoginPasswordCipher `json:"new_password_cipher"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, m.h.log, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid json body", err))
		return
	}
	current, err := m.decryptPasswordCipher(body.CurrentPasswordCipher)
	if err != nil {
		httpx.WriteError(w, m.h.log, err)
		return
	}
	newPwd, err := m.decryptPasswordCipher(body.NewPasswordCipher)
	if err != nil {
		httpx.WriteError(w, m.h.log, err)
		return
	}
	if _, err := m.iamSvc.ChangeAccountPassword(r.Context(), iamapp.ChangeAccountPasswordRequest{
		UserID:          claims.Sub,
		CurrentPassword: current,
		NewPassword:     newPwd,
	}); err != nil {
		httpx.WriteError(w, m.h.log, err)
		return
	}
	m.auditLog(r, claims, "user.password_change", "user", claims.Sub)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (m *MeHandler) decryptPasswordCipher(c *iamapp.LoginPasswordCipher) (string, error) {
	if c == nil || strings.TrimSpace(c.CiphertextB64) == "" {
		return "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "password_cipher required", nil)
	}
	alg := strings.TrimSpace(c.Alg)
	if alg != loginpassword.AlgRSAOAEP256 {
		return "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "unsupported password_cipher.alg", nil)
	}
	if strings.TrimSpace(c.KID) != "" && c.KID != m.loginPWD.KeyID() {
		return "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "unknown password_cipher.kid", nil)
	}
	plain, err := m.loginPWD.DecryptOAEP256(c.CiphertextB64)
	if err != nil {
		return "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid password_cipher", err)
	}
	return plain, nil
}

func (m *MeHandler) auditLog(r *http.Request, claims *iamapp.AccessTokenClaims, action, resourceType, resourceID string) {
	if m.h.audit == nil {
		return
	}
	_ = m.h.audit.AppendAuditLog(r.Context(), auditapp.AppendAuditLogRequest{
		ActorUserID:       claims.Sub,
		ActorMembershipID: claims.MembershipID,
		CompanyID:         claims.CompanyID,
		Action:            action,
		ResourceType:      resourceType,
		ResourceID:        resourceID,
		Decision:          "allow",
		RequestID:         httpx.RequestIDFromContext(r.Context()),
		IP:                r.RemoteAddr,
		UserAgent:         r.UserAgent(),
	})
}

func isAccountLocked(status string) bool {
	s := strings.ToLower(strings.TrimSpace(status))
	return s == "locked" || s == "suspended" || s == "disabled"
}

func (m *MeHandler) loadContactBlock(ctx context.Context, userID string) map[string]any {
	out := map[string]any{
		"email":           "",
		"phone":           "",
		"email_verified": false,
	}
	if m.profiles != nil {
		if prof, err := m.profiles.GetAdminAccountSettings(ctx, userID); err == nil {
			out["email"] = prof.Email
			out["phone"] = prof.Phone
		}
	}
	if m.regDB != nil {
		if ev, err := iamregmysql.EmailVerifiedSnapshot(ctx, m.regDB, userID); err == nil {
			out["email_verified"] = ev
		}
	}
	return out
}

func (m *MeHandler) companyAddress(ctx context.Context, companyID string) string {
	if m.regDB == nil || strings.TrimSpace(companyID) == "" {
		return ""
	}
	var addr string
	err := m.regDB.QueryRowContext(ctx, `
		SELECT COALESCE(address, '') FROM companies WHERE company_id = ?
	`, companyID).Scan(&addr)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(addr)
}
