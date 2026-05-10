package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	companyaccessapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
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
		CompanyName          *string `json:"company_name"`
		TaxCode              *string `json:"tax_code"`
		RegistrationNumber   *string `json:"registration_number"`
		Address              *string `json:"address"`
		Phone                *string `json:"phone"`
		ContactEmail         *string `json:"contact_email"`
		RepresentativeName   *string `json:"representative_name"`
		VerificationStatus   *string `json:"verification_status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid JSON payload", err))
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
	cid := strings.TrimSpace(r.PathValue("company_id"))
	if err := h.adminSvc.SetPlatformCompanyStatus(r.Context(), companyaccessapp.SetPlatformCompanyStatusRequest{
		Subject: companyaccessapp.AdminSubject{
			UserID: sub.Sub, MembershipID: sub.MembershipID, CompanyID: sub.CompanyID,
		},
		CompanyID: cid,
		Status:    status,
	}); err != nil {
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
	writeEnvelope(w, http.StatusOK, detail, nil)
}
