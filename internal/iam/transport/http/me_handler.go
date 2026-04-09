package http

import (
	"net/http"
	"sort"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
)

type MeHandler struct {
	h          *Handler
	identities iamapp.IdentityQueryService
	members    caapp.MembershipQueryService
	authorizer authapp.Service
}

func NewMeHandler(base *Handler, identities iamapp.IdentityQueryService, members caapp.MembershipQueryService, authorizer authapp.Service) *MeHandler {
	return &MeHandler{h: base, identities: identities, members: members, authorizer: authorizer}
}

func (m *MeHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/me", m.me)
	mux.HandleFunc("GET /api/v1/me/companies", m.companies)
	mux.HandleFunc("GET /api/v1/me/effective-access", m.effectiveAccess)
	mux.HandleFunc("GET /api/v1/me/capabilities", m.capabilities)
	mux.HandleFunc("GET /api/v1/me/membership", m.membership)
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
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"user": map[string]any{
			"user_id":   user.UserID,
			"login_id":  user.LoginID,
			"full_name": user.FullName,
		},
		"current_context": map[string]any{
			"company_id":    claims.CompanyID,
			"membership_id": claims.MembershipID,
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
		out = append(out, map[string]any{
			"company_id":        it.CompanyID,
			"membership_id":     it.MembershipID,
			"company_name":      it.CompanyName,
			"membership_status": it.Status,
		})
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
			"dashboard":             hasPermission(eff.Permissions, "view_dashboard"),
			"user_management":       hasPermission(eff.Permissions, "manage_users"),
			"department_management": hasPermission(eff.Permissions, "manage_departments"),
			"disclosure":            hasPermission(eff.Permissions, "view_disclosure"),
			"workflow_approval":     hasPermission(eff.Permissions, "approve_disclosure"),
			"notification_config":   hasPermission(eff.Permissions, "manage_notification_rules"),
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
