package http

import (
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
	reg := wfcapp.DefaultRoleRegistry()
	items := make([]assigneeRoleItem, 0, len(reg.ListRoles()))
	for _, d := range reg.ListRoles() {
		assignable := d.Class != wfcapp.RoleClassSystemOwned
		items = append(items, assigneeRoleItem{
			ID:          preferredRoleID(d),
			Code:        d.Code,
			Name:        d.Name,
			Description: strings.TrimSpace(d.Description),
			Assignable:  assignable,
			Creatable:   false,
		})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"items": items,
			"note":  "Workflow assignee roles are platform-managed registry entries; custom roles require a platform release.",
		},
	})
}
