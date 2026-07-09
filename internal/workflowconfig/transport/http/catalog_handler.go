package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
	wfcapp "github.com/cobo/cobo_iam_services/internal/workflowconfig/app"
)

type assigneeRoleItem struct {
	ID          string `json:"id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Assignable  bool   `json:"assignable"`
	Creatable   bool   `json:"creatable"`
}

func preferredRoleID(d wfcapp.RoleDefinition) string {
	if len(d.Aliases) > 0 {
		return d.Aliases[0]
	}
	if d.Code != "" {
		return d.Code
	}
	return d.RoleID
}

func (h *Handler) assigneeRoles(w http.ResponseWriter, r *http.Request) {
	if _, err := h.actor(r); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	switch r.Method {
	case http.MethodGet:
		h.listAssigneeRoles(w, r)
	case http.MethodPost:
		h.createAssigneeRole(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) listAssigneeRoles(w http.ResponseWriter, r *http.Request) {
	reg := wfcapp.DefaultRoleRegistry()
	if h.catalog != nil {
		if merged, err := h.catalog.MergedRegistry(r.Context()); err == nil && merged != nil {
			reg = merged
		}
	}
	items := make([]assigneeRoleItem, 0, len(reg.ListRoles()))
	for _, d := range reg.ListRoles() {
		assignable := d.Class != wfcapp.RoleClassSystemOwned
		creatable := false
		if assignable && d.Class == wfcapp.RoleClassStandard && strings.HasPrefix(d.Code, "wf_role_") {
			creatable = true
		}
		items = append(items, assigneeRoleItem{
			ID:          preferredRoleID(d),
			Code:        d.Code,
			Name:        d.Name,
			Description: strings.TrimSpace(d.Description),
			Assignable:  assignable,
			Creatable:   creatable,
		})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"items": items,
			"note":  "Workflow assignee roles are template metadata only; not IAM Roles & Permissions.",
		},
	})
}

func (h *Handler) createAssigneeRole(w http.ResponseWriter, r *http.Request) {
	if h.catalog == nil {
		httpx.WriteError(w, nil, http.ErrNotSupported)
		return
	}
	var req wfcapp.CreateAssigneeRoleRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	created, err := h.catalog.Create(r.Context(), req)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{
		"data": map[string]any{
			"id":        created.RoleCode,
			"code":      created.RoleCode,
			"role_code": created.RoleCode,
			"name":      created.RoleName,
			"role_name": created.RoleName,
		},
	})
}
