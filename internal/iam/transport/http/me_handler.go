package http

import (
	"database/sql"
	"net/http"
	"sort"
	"strings"
	"time"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	"github.com/cobo/cobo_iam_services/internal/iam/loginpassword"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
)

type MeHandler struct {
	h          *Handler
	identities iamapp.IdentityQueryService
	members    caapp.MembershipQueryService
	authorizer authapp.Service
	profiles   userAccountProfileRepo
	regDB      *sql.DB
	iamSvc     iamapp.Service
	loginPWD   *loginpassword.Service
}

func NewMeHandler(
	base *Handler,
	identities iamapp.IdentityQueryService,
	members caapp.MembershipQueryService,
	authorizer authapp.Service,
	profiles userAccountProfileRepo,
	regDB *sql.DB,
	iamSvc iamapp.Service,
	loginPWD *loginpassword.Service,
) *MeHandler {
	return &MeHandler{
		h:          base,
		identities: identities,
		members:    members,
		authorizer: authorizer,
		profiles:   profiles,
		regDB:      regDB,
		iamSvc:     iamSvc,
		loginPWD:   loginPWD,
	}
}

func (m *MeHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/me", m.me)
	// Alias for frontend contract compatibility.
	mux.HandleFunc("GET /api/v1/me/profile", m.me)
	mux.HandleFunc("GET /api/v1/me/companies", m.companies)
	// Alias for frontend contract compatibility.
	mux.HandleFunc("GET /api/v1/me/authorized-companies", m.companies)
	mux.HandleFunc("GET /api/v1/me/effective-access", m.effectiveAccess)
	mux.HandleFunc("GET /api/v1/me/capabilities", m.capabilities)
	mux.HandleFunc("GET /api/v1/me/membership", m.membership)
	mux.HandleFunc("PATCH /api/v1/me/profile", m.patchProfile)
	mux.HandleFunc("POST /api/v1/me/change-password", m.changePassword)
}

func (m *MeHandler) me(w http.ResponseWriter, r *http.Request) {
	claims, err := m.h.inspector.InspectAccessToken(r.Context(), bearerToken(r.Header.Get("Authorization")))
	if err != nil {
		httpx.WriteError(w, m.h.log, err)
		return
	}
	user, err := m.identities.GetByUserID(r.Context(), claims.Sub)
	if err != nil {
		httpx.WriteError(w, m.h.log, err)
		return
	}
	memberships, err := m.members.GetMembershipsByUser(r.Context(), claims.Sub)
	if err != nil {
		httpx.WriteError(w, m.h.log, err)
		return
	}
	activeMemberships := make([]map[string]any, 0, len(memberships))
	for _, ms := range memberships {
		if strings.EqualFold(strings.TrimSpace(ms.Status), "active") {
			activeMemberships = append(activeMemberships, map[string]any{
				"company_id":    ms.CompanyID,
				"company_name":  ms.CompanyName,
				"membership_id": ms.MembershipID,
			})
		}
	}
	hasCompany := len(activeMemberships) > 0
	activeCompanyID := any(nil)
	if claims.CompanyID != "" {
		activeCompanyID = claims.CompanyID
	}
	userPayload := map[string]any{
		"user_id":           user.UserID,
		"login_id":          user.LoginID,
		"full_name":         user.FullName,
		"subscription_tier": user.SubscriptionTier,
	}
	if user.SubscriptionExpiresAt != nil {
		userPayload["subscription_expires_at"] = user.SubscriptionExpiresAt.UTC().Format(time.RFC3339)
	} else {
		userPayload["subscription_expires_at"] = nil
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"user":                    userPayload,
		"contact":                 m.loadContactBlock(r.Context(), claims.Sub),
		"profile_schema_version":  1,
		"current_context": map[string]any{
			"company_id":    claims.CompanyID,
			"membership_id": claims.MembershipID,
		},
		"company_context": map[string]any{
			"has_company":       hasCompany,
			"active_company_id": activeCompanyID,
			"companies":         activeMemberships,
		},
	})
}

func (m *MeHandler) companies(w http.ResponseWriter, r *http.Request) {
	claims, err := m.h.inspector.InspectAccessToken(r.Context(), bearerToken(r.Header.Get("Authorization")))
	if err != nil {
		httpx.WriteError(w, m.h.log, err)
		return
	}
	items, err := m.members.GetMembershipsByUser(r.Context(), claims.Sub)
	if err != nil {
		httpx.WriteError(w, m.h.log, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		entry := map[string]any{
			"company_id":        it.CompanyID,
			"membership_id":     it.MembershipID,
			"company_name":      it.CompanyName,
			"membership_status": it.Status,
			"roles":             []string{},
			"titles":            []string{},
			"address":           m.companyAddress(r.Context(), it.CompanyID),
		}
		if roles, err := m.members.GetMembershipRoles(r.Context(), it.MembershipID); err == nil {
			entry["roles"] = roles
		}
		if titles, err := m.members.GetMembershipTitles(r.Context(), it.MembershipID); err == nil {
			entry["titles"] = titles
		}
		out = append(out, entry)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (m *MeHandler) effectiveAccess(w http.ResponseWriter, r *http.Request) {
	claims, err := m.h.inspector.InspectAccessToken(r.Context(), bearerToken(r.Header.Get("Authorization")))
	if err != nil {
		httpx.WriteError(w, m.h.log, err)
		return
	}
	resp, err := m.authorizer.GetEffectiveAccess(r.Context(), claims.MembershipID, claims.CompanyID)
	if err != nil {
		httpx.WriteError(w, m.h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (m *MeHandler) capabilities(w http.ResponseWriter, r *http.Request) {
	claims, err := m.h.inspector.InspectAccessToken(r.Context(), bearerToken(r.Header.Get("Authorization")))
	if err != nil {
		httpx.WriteError(w, m.h.log, err)
		return
	}
	eff, err := m.authorizer.GetEffectiveAccess(r.Context(), claims.MembershipID, claims.CompanyID)
	if err != nil {
		httpx.WriteError(w, m.h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"modules": map[string]bool{
			"platform_cms": hasAnyPermission(eff.Permissions,
				"platform.cms.view",
			),
			"dashboard": hasAnyPermission(eff.Permissions,
				"dashboard.view",
			),
			"user_management": hasAnyPermission(eff.Permissions,
				"user.edit",
				"rbac.manage",
				"system.settings",
			),
			"department_management": hasAnyPermission(eff.Permissions,
				"recipient.manage",
				"user.edit",
				"rbac.manage",
			),
			"disclosure": hasAnyPermission(eff.Permissions,
				"disclosure.view",
				"disclosure.create",
				"disclosure.edit",
			),
			"workflow_approval": hasAnyPermission(eff.Permissions,
				"disclosure.approve",
				"workflow.step.confirm",
				"workflow.step.override",
			),
			"notification_config": hasAnyPermission(eff.Permissions,
				"alert.channels.manage",
			),
		},
	})
}

func (m *MeHandler) membership(w http.ResponseWriter, r *http.Request) {
	claims, err := m.h.inspector.InspectAccessToken(r.Context(), bearerToken(r.Header.Get("Authorization")))
	if err != nil {
		httpx.WriteError(w, m.h.log, err)
		return
	}
	roles, err := m.members.GetMembershipRoles(r.Context(), claims.MembershipID)
	if err != nil {
		httpx.WriteError(w, m.h.log, err)
		return
	}
	deps, err := m.members.GetMembershipDepartments(r.Context(), claims.MembershipID)
	if err != nil {
		httpx.WriteError(w, m.h.log, err)
		return
	}
	titles, err := m.members.GetMembershipTitles(r.Context(), claims.MembershipID)
	if err != nil {
		httpx.WriteError(w, m.h.log, err)
		return
	}
	depNames := make([]string, 0, len(deps))
	for _, d := range deps {
		depNames = append(depNames, d.DepartmentName)
	}
	sort.Strings(depNames)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"company_id":    claims.CompanyID,
		"membership_id": claims.MembershipID,
		"roles":         roles,
		"departments":   depNames,
		"titles":        titles,
	})
}

func hasPermission(items []string, target string) bool {
	for _, it := range items {
		if it == target {
			return true
		}
	}
	return false
}

func hasAnyPermission(items []string, targets ...string) bool {
	for _, t := range targets {
		if hasPermission(items, t) {
			return true
		}
	}
	return false
}
