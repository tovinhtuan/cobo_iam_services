package http

import (
	"net/http"
	"strconv"
	"strings"

	companyaccessapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
)

func (h *Handler) listCMSCompanies(w http.ResponseWriter, r *http.Request) {
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
	q := r.URL.Query()
	page, _ := strconv.Atoi(strings.TrimSpace(q.Get("page")))
	limit, _ := strconv.Atoi(strings.TrimSpace(q.Get("limit")))
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	out, err := h.adminSvc.ListPlatformCompanies(r.Context(), companyaccessapp.ListPlatformCompaniesRequest{
		Subject: companyaccessapp.AdminSubject{
			UserID: sub.Sub, MembershipID: sub.MembershipID, CompanyID: sub.CompanyID,
		},
		Q:                  strings.TrimSpace(q.Get("q")),
		Status:             strings.TrimSpace(q.Get("status")),
		VerificationStatus: strings.TrimSpace(q.Get("verification_status")),
		Page:               page,
		Limit:              limit,
		SortBy:             strings.TrimSpace(q.Get("sort_by")),
		SortOrder:          strings.TrimSpace(q.Get("sort_order")),
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	writeEnvelope(w, http.StatusOK, map[string]any{"items": out.Items}, map[string]any{
		"total": out.Total, "page": page, "limit": limit,
	})
}

func (h *Handler) getCMSCompany(w http.ResponseWriter, r *http.Request) {
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
	cid := strings.TrimSpace(r.PathValue("company_id"))
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
	writeEnvelope(w, http.StatusOK, detail, nil)
}

func (h *Handler) patchCMSCompany(w http.ResponseWriter, r *http.Request) {
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
	var p struct {
		CompanyName        *string `json:"company_name"`
		TaxCode            *string `json:"tax_code"`
		RegistrationNumber *string `json:"registration_number"`
		Address            *string `json:"address"`
		Phone              *string `json:"phone"`
		ContactEmail       *string `json:"contact_email"`
		RepresentativeName *string `json:"representative_name"`
		VerificationStatus *string `json:"verification_status"`
		ExpectedRevision   string  `json:"expected_revision"`
	}
	if err := decodeCMSCompanyBody(w, r, &p, cmsCompanyBodyLimit, false); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	req := companyaccessapp.UpdatePlatformCompanyRequest{
		Subject: companyaccessapp.AdminSubject{
			UserID: sub.Sub, MembershipID: sub.MembershipID, CompanyID: sub.CompanyID,
		},
		CompanyID:          strings.TrimSpace(r.PathValue("company_id")),
		CompanyName:        p.CompanyName,
		TaxCode:            p.TaxCode,
		RegistrationNumber: p.RegistrationNumber,
		Address:            p.Address,
		Phone:              p.Phone,
		ContactEmail:       p.ContactEmail,
		RepresentativeName: p.RepresentativeName,
		VerificationStatus: p.VerificationStatus,
		ExpectedRevision:   p.ExpectedRevision,
		RequestID:          httpx.RequestIDFromContext(r.Context()),
	}

	if err := h.adminSvc.UpdatePlatformCompany(r.Context(), req); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	detail, err := h.adminSvc.GetPlatformCompany(r.Context(), companyaccessapp.GetPlatformCompanyRequest{
		Subject: req.Subject, CompanyID: req.CompanyID,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	writeEnvelope(w, http.StatusOK, detail, nil)
}

func (h *Handler) postCMSCompanyDeactivate(w http.ResponseWriter, r *http.Request) {
	h.setCMSCompanyStatus(w, r, "inactive")
}

func (h *Handler) postCMSCompanyActivate(w http.ResponseWriter, r *http.Request) {
	h.setCMSCompanyStatus(w, r, "active")
}

// postCMSCompanySuspend sets "Tạm ngưng": members keep read and export access only.
func (h *Handler) postCMSCompanySuspend(w http.ResponseWriter, r *http.Request) {
	h.setCMSCompanyStatus(w, r, "suspended")
}

func (h *Handler) setCMSCompanyStatus(w http.ResponseWriter, r *http.Request, status string) {
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
		ReasonCode         string `json:"reason_code"`
		ReasonNote         string `json:"reason_note"`
		ExpectedRevision   string `json:"expected_revision"`
		ConfirmCompanyCode string `json:"confirm_company_code"`
	}
	if err := decodeCMSCompanyBody(w, r, &body, cmsCompanyStatusBodyLimit, true); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	cid := strings.TrimSpace(r.PathValue("company_id"))
	change, err := h.adminSvc.SetPlatformCompanyStatus(r.Context(), companyaccessapp.SetPlatformCompanyStatusRequest{
		Subject: companyaccessapp.AdminSubject{
			UserID: sub.Sub, MembershipID: sub.MembershipID, CompanyID: sub.CompanyID,
		},
		CompanyID:          cid,
		Status:             status,
		ReasonCode:         body.ReasonCode,
		ReasonNote:         body.ReasonNote,
		ExpectedRevision:   body.ExpectedRevision,
		ConfirmCompanyCode: body.ConfirmCompanyCode,
		RequestID:          httpx.RequestIDFromContext(r.Context()),
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
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
	writeEnvelope(w, http.StatusOK, detail, map[string]any{"status_change": change})
}

// getCMSCompanyHistory lists audit entries for one company (status, verification, profile, plan).
func (h *Handler) getCMSCompanyHistory(w http.ResponseWriter, r *http.Request) {
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
	q := r.URL.Query()
	limit, _ := strconv.Atoi(strings.TrimSpace(q.Get("limit")))
	out, err := h.adminSvc.ListPlatformCompanyHistory(r.Context(), companyaccessapp.ListPlatformCompanyHistoryRequest{
		Subject: companyaccessapp.AdminSubject{
			UserID: sub.Sub, MembershipID: sub.MembershipID, CompanyID: sub.CompanyID,
		},
		CompanyID: strings.TrimSpace(r.PathValue("company_id")),
		Cursor:    strings.TrimSpace(q.Get("cursor")),
		Limit:     limit,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var next any
	if out.NextCursor != "" {
		next = out.NextCursor
	}
	writeEnvelope(w, http.StatusOK, map[string]any{"items": out.Items}, map[string]any{"limit": out.Limit, "next_cursor": next})
}
