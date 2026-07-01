package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
)

func (h *AdminHandler) registerDelegationRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/admin/delegations", h.createDelegation)
	mux.HandleFunc("GET /api/v1/admin/delegations", h.listDelegations)
	mux.HandleFunc("GET /api/v1/admin/delegations/{delegation_id}", h.getDelegation)
	mux.HandleFunc("PATCH /api/v1/admin/delegations/{delegation_id}", h.patchDelegation)
	mux.HandleFunc("POST /api/v1/admin/delegations/{delegation_id}/revoke", h.revokeDelegation)
}

func (h *AdminHandler) createDelegation(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var body struct {
		DelegateeMembershipID string   `json:"delegatee_membership_id"`
		ScopeType             string   `json:"scope_type"`
		ScopeID               string   `json:"scope_id"`
		PermissionSet         []string `json:"permission_set"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	out, err := h.svc.CreateDelegation(r.Context(), caapp.CreateDelegationRequest{
		Subject:               sub,
		DelegateeMembershipID: body.DelegateeMembershipID,
		ScopeType:             body.ScopeType,
		ScopeID:               body.ScopeID,
		PermissionSet:         body.PermissionSet,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "delegated.admin.granted", "delegation", out.DelegationID)
	httpx.WriteJSON(w, http.StatusCreated, out)
}

func (h *AdminHandler) listDelegations(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	out, err := h.svc.ListDelegations(r.Context(), caapp.ListDelegationsRequest{
		Subject:               sub,
		Status:                r.URL.Query().Get("status"),
		DelegateeMembershipID: r.URL.Query().Get("delegatee_membership_id"),
		ScopeID:               r.URL.Query().Get("scope_id"),
		Limit:                 limit,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *AdminHandler) getDelegation(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	out, err := h.svc.GetDelegation(r.Context(), caapp.GetDelegationRequest{
		Subject:      sub,
		DelegationID: r.PathValue("delegation_id"),
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *AdminHandler) patchDelegation(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var body struct {
		PermissionSet []string `json:"permission_set"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	out, err := h.svc.PatchDelegation(r.Context(), caapp.PatchDelegationRequest{
		Subject:       sub,
		DelegationID:  r.PathValue("delegation_id"),
		PermissionSet: body.PermissionSet,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "delegated.admin.updated", "delegation", out.DelegationID)
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *AdminHandler) revokeDelegation(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	id := strings.TrimSpace(r.PathValue("delegation_id"))
	out, err := h.svc.RevokeDelegation(r.Context(), caapp.RevokeDelegationRequest{
		Subject:      sub,
		DelegationID: id,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, "delegated.admin.revoked", "delegation", out.DelegationID)
	httpx.WriteJSON(w, http.StatusOK, out)
}
