package http

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	platformcmsapp "github.com/cobo/cobo_iam_services/internal/platformcms/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
)

func (h *Handler) getSubscriptionUpgradePayment(w http.ResponseWriter, r *http.Request) {
	if h.subscriptionUpgradeSvc == nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeServiceUnavailable, "subscription upgrade config unavailable", nil))
		return
	}
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireCMSAccess(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	dto, err := h.subscriptionUpgradeSvc.GetCMS(r.Context())
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	writeEnvelope(w, http.StatusOK, dto, nil)
}

func (h *Handler) putSubscriptionUpgradePayment(w http.ResponseWriter, r *http.Request) {
	if h.subscriptionUpgradeSvc == nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeServiceUnavailable, "subscription upgrade config unavailable", nil))
		return
	}
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
	var body struct {
		DescriptionVI        string `json:"description_vi"`
		DescriptionEN        string `json:"description_en"`
		Hotline              string `json:"hotline"`
		BankName             string `json:"bank_name"`
		AccountName          string `json:"account_name"`
		AccountNumber        string `json:"account_number"`
		TransferNoteTemplate string `json:"transfer_note_template"`
		IsActive             bool   `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid JSON body", err))
		return
	}
	dto, err := h.subscriptionUpgradeSvc.UpdateCMS(r.Context(), platformcmsapp.UpdateSubscriptionUpgradePaymentRequest{
		DescriptionVI:        body.DescriptionVI,
		DescriptionEN:        body.DescriptionEN,
		Hotline:              body.Hotline,
		BankName:             body.BankName,
		AccountName:          body.AccountName,
		AccountNumber:        body.AccountNumber,
		TransferNoteTemplate: body.TransferNoteTemplate,
		IsActive:             body.IsActive,
		ActorID:              sub.Sub,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.appendCMSAuditLog(r.Context(), sub, cmsActionSubscriptionUpgradeUpdate, "subscription_upgrade_payment", "global")
	writeEnvelope(w, http.StatusOK, dto, nil)
}

func (h *Handler) uploadSubscriptionUpgradeQR(w http.ResponseWriter, r *http.Request) {
	if h.subscriptionUpgradeSvc == nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeServiceUnavailable, "subscription upgrade config unavailable", nil))
		return
	}
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
	if err := r.ParseMultipartForm(platformcmsapp.SubscriptionUpgradeQRMaxBytes + 512); err != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid multipart form", err))
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "file is required", err))
		return
	}
	defer file.Close()
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	// Detect from filename extension if browser sent generic type.
	lowerName := strings.ToLower(header.Filename)
	switch {
	case strings.HasSuffix(lowerName, ".png"):
		contentType = "image/png"
	case strings.HasSuffix(lowerName, ".jpg"), strings.HasSuffix(lowerName, ".jpeg"):
		contentType = "image/jpeg"
	case strings.HasSuffix(lowerName, ".webp"):
		contentType = "image/webp"
	}
	dto, err := h.subscriptionUpgradeSvc.UploadQR(r.Context(), sub.Sub, contentType, header.Filename, file, header.Size)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.appendCMSAuditLog(r.Context(), sub, cmsActionSubscriptionUpgradeQRUpload, "subscription_upgrade_payment", "global")
	writeEnvelope(w, http.StatusOK, dto, nil)
}

func (h *Handler) deleteSubscriptionUpgradeQR(w http.ResponseWriter, r *http.Request) {
	if h.subscriptionUpgradeSvc == nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeServiceUnavailable, "subscription upgrade config unavailable", nil))
		return
	}
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
	dto, err := h.subscriptionUpgradeSvc.DeleteQR(r.Context(), sub.Sub)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.appendCMSAuditLog(r.Context(), sub, cmsActionSubscriptionUpgradeQRDelete, "subscription_upgrade_payment", "global")
	writeEnvelope(w, http.StatusOK, dto, nil)
}

func (h *Handler) getCMSSubscriptionUpgradeQR(w http.ResponseWriter, r *http.Request) {
	if h.subscriptionUpgradeSvc == nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeServiceUnavailable, "subscription upgrade config unavailable", nil))
		return
	}
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireCMSAccess(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	ct, fn, data, err := h.subscriptionUpgradeSvc.ReadQR(r.Context())
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", "inline; filename=\""+fn+"\"")
	w.Header().Set("Cache-Control", "private, max-age=60")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h *Handler) getAdminSubscriptionUpgradeInstruction(w http.ResponseWriter, r *http.Request) {
	if h.subscriptionUpgradeSvc == nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeServiceUnavailable, "subscription upgrade config unavailable", nil))
		return
	}
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireAnyPermission(r.Context(), sub.MembershipID, sub.CompanyID, "rbac.manage", "company.view", "company.edit", "system.settings"); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	lang := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("lang")))
	if lang != "en" {
		lang = "vi"
	}
	tier := h.lookupUserSubscriptionTier(r.Context(), sub.Sub)
	companyCode := h.lookupCompanyCode(r.Context(), sub.CompanyID)
	dto, err := h.subscriptionUpgradeSvc.PortalInstruction(
		r.Context(),
		lang,
		tier,
		companyCode,
		"/api/v1/admin/subscription-upgrade/qr",
	)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	writeEnvelope(w, http.StatusOK, dto, nil)
}

func (h *Handler) getAdminSubscriptionUpgradeQR(w http.ResponseWriter, r *http.Request) {
	if h.subscriptionUpgradeSvc == nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeServiceUnavailable, "subscription upgrade config unavailable", nil))
		return
	}
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireAnyPermission(r.Context(), sub.MembershipID, sub.CompanyID, "rbac.manage", "company.view", "company.edit", "system.settings"); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	cms, err := h.subscriptionUpgradeSvc.GetCMS(r.Context())
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if !cms.IsActive || !cms.QRConfigured {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "qr not available", nil))
		return
	}
	ct, fn, data, err := h.subscriptionUpgradeSvc.ReadQR(r.Context())
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", "inline; filename=\""+fn+"\"")
	w.Header().Set("Cache-Control", "private, max-age=60")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h *Handler) lookupUserSubscriptionTier(ctx context.Context, userID string) string {
	if h.upgradePaymentDB == nil || strings.TrimSpace(userID) == "" {
		return "Free"
	}
	var tier string
	err := h.upgradePaymentDB.QueryRowContext(ctx, `
		SELECT subscription_tier FROM user_subscription_tiers
		WHERE user_id = ? AND (effective_to IS NULL OR effective_to > UTC_TIMESTAMP())
		LIMIT 1
	`, userID).Scan(&tier)
	if err != nil {
		return "Free"
	}
	return tier
}

func (h *Handler) lookupCompanyCode(ctx context.Context, companyID string) string {
	if h.upgradePaymentDB == nil || strings.TrimSpace(companyID) == "" {
		return ""
	}
	var code sql.NullString
	err := h.upgradePaymentDB.QueryRowContext(ctx, `
		SELECT code FROM companies WHERE company_id = ? LIMIT 1
	`, companyID).Scan(&code)
	if err != nil || !code.Valid || strings.TrimSpace(code.String) == "" {
		return strings.ReplaceAll(companyID, "-", "")
	}
	return code.String
}
