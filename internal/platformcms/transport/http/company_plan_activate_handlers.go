package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
	companyaccessapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
	"github.com/cobo/cobo_iam_services/internal/subscription/companyplan"
)

func (h *Handler) postCMSCompanySubscriptionActivate(w http.ResponseWriter, r *http.Request) {
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
	if h.companyPlanRepo == nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeServiceUnavailable, "company plan service unavailable", nil))
		return
	}
	cid := strings.TrimSpace(r.PathValue("company_id"))
	if cid == "" {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "company_id is required", nil))
		return
	}
	var body struct {
		PlanCode         string `json:"plan_code"`
		VerifiedAmount   string `json:"verified_amount"`
		PaymentReference string `json:"payment_reference"`
		VerificationNote string `json:"verification_note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid JSON body", err))
		return
	}
	code := companyplan.PlanCode(strings.ToUpper(strings.TrimSpace(body.PlanCode)))
	if !companyplan.ValidPaidManualPlanCode(code) {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "plan_code must be PREMIUM or ENTERPRISE", nil))
		return
	}
	if utf8.RuneCountInString(body.VerifiedAmount) > 64 || utf8.RuneCountInString(body.PaymentReference) > 128 || utf8.RuneCountInString(body.VerificationNote) > 500 {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "verification metadata too long", nil))
		return
	}

	detail, err := h.adminSvc.GetPlatformCompany(r.Context(), companyaccessapp.GetPlatformCompanyRequest{
		Subject: companyaccessapp.AdminSubject{
			UserID: sub.Sub, MembershipID: sub.MembershipID, CompanyID: sub.CompanyID,
		},
		CompanyID: cid,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}

	newID := idgen.UUIDv7Generator{}.NewUUID()
	out, err := h.companyPlanRepo.ActivateImmediate(r.Context(), cid, code, companyplan.RecordOriginPlatformAdminManual, newID)
	if err != nil {
		httpx.WriteError(w, nil, mapCompanyPlanActivateError(err))
		return
	}

	h.appendCMSAuditLogMeta(r.Context(), sub, cmsActionCompanyPlanActivate, "company_subscription", out.Plan.ID, cid, map[string]any{
		"target_company_id":   cid,
		"company_code":        detail.CompanyCode,
		"old_plan":            string(out.PreviousCode),
		"new_plan":            string(out.Plan.Code),
		"already_active":      out.AlreadyActive,
		"verified_amount":     strings.TrimSpace(body.VerifiedAmount),
		"payment_reference":   strings.TrimSpace(body.PaymentReference),
		"verification_note":   strings.TrimSpace(body.VerificationNote),
		"origin":              string(out.Plan.Origin),
		"effective_from":      out.Plan.EffectiveFrom.UTC().Format("2006-01-02T15:04:05Z"),
		"human_payment_check": true,
	})

	refreshed, err := h.adminSvc.GetPlatformCompany(r.Context(), companyaccessapp.GetPlatformCompanyRequest{
		Subject: companyaccessapp.AdminSubject{
			UserID: sub.Sub, MembershipID: sub.MembershipID, CompanyID: sub.CompanyID,
		},
		CompanyID: cid,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	writeEnvelope(w, http.StatusOK, map[string]any{
		"company":        refreshed,
		"already_active": out.AlreadyActive,
		"plan":           companyplan.ToPlanDTO(&out.Plan),
	}, nil)
}

func mapCompanyPlanActivateError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, companyplan.ErrCompanyNotFound):
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "company not found", err)
	case errors.Is(err, companyplan.ErrUnsupportedManualPlan), errors.Is(err, companyplan.ErrInvalidPlan):
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid company plan activation", err)
	case errors.Is(err, companyplan.ErrOverlap):
		return perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "company plan window conflict", err)
	default:
		if _, ok := perr.AsHTTPError(err); ok {
			return err
		}
		return perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "company plan activation failed", err)
	}
}

func (h *Handler) appendCMSAuditLogMeta(ctx context.Context, sub iamapp.AccessTokenClaims, action, resourceType, resourceID, targetCompanyID string, meta map[string]any) {
	if h.auditSvc == nil {
		return
	}
	companyID := strings.TrimSpace(targetCompanyID)
	if companyID == "" {
		companyID = sub.CompanyID
	}
	_ = h.auditSvc.AppendAuditLog(ctx, auditapp.AppendAuditLogRequest{
		ActorUserID:       sub.Sub,
		ActorMembershipID: sub.MembershipID,
		CompanyID:         companyID,
		Action:            action,
		ResourceType:      resourceType,
		ResourceID:        resourceID,
		Decision:          "allow",
		Metadata:          meta,
	})
}
