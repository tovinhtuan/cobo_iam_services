// Package http exposes the global workflow versioning lifecycle (publish ≠ activate).
// Routes are registered only when WORKFLOW_VERSIONING_ENABLED is true (server wiring), so the
// existing GET/PUT workflow APIs are unaffected when the flag is OFF.
package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
	wfcapp "github.com/cobo/cobo_iam_services/internal/workflowconfig/app"
)

type Handler struct {
	svc       *wfcapp.VersionService
	cfg       *wfcapp.ConfigService
	catalog   *wfcapp.AssigneeRoleCatalogService
	inspector iamapp.TokenInspector
}

func NewHandler(svc *wfcapp.VersionService, cfg *wfcapp.ConfigService, catalog *wfcapp.AssigneeRoleCatalogService, inspector iamapp.TokenInspector) *Handler {
	return &Handler{svc: svc, cfg: cfg, catalog: catalog, inspector: inspector}
}

// RegisterAssigneeRoleCatalog wires GET/POST assignee role catalog routes (independent of workflow versioning).
func RegisterAssigneeRoleCatalog(mux *http.ServeMux, catalog *wfcapp.AssigneeRoleCatalogService, inspector iamapp.TokenInspector) {
	h := &Handler{catalog: catalog, inspector: inspector}
	mux.HandleFunc("GET /api/v1/platform/cms/workflow/assignee-roles", h.assigneeRoles)
	mux.HandleFunc("POST /api/v1/platform/cms/workflow/assignee-roles", h.assigneeRoles)
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/platform/cms/templates/{type_id}/workflow/configuration", h.configuration)
	mux.HandleFunc("GET /api/v1/platform/cms/templates/{type_id}/workflow/readiness", h.readiness)
	mux.HandleFunc("POST /api/v1/platform/cms/templates/{type_id}/workflow/validate", h.validate)
	mux.HandleFunc("GET /api/v1/platform/cms/templates/{type_id}/workflow/lifecycle", h.lifecycle)
	mux.HandleFunc("GET /api/v1/platform/cms/templates/{type_id}/workflow/versions", h.listVersions)
	mux.HandleFunc("GET /api/v1/platform/cms/templates/{type_id}/workflow/versions/{version_no}", h.versionDetail)
	mux.HandleFunc("POST /api/v1/platform/cms/templates/{type_id}/workflow/publish", h.publish)
	mux.HandleFunc("POST /api/v1/platform/cms/templates/{type_id}/workflow/versions/{version_no}/activate", h.activate)
}

func (h *Handler) configuration(w http.ResponseWriter, r *http.Request) {
	if _, err := h.actor(r); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	data, err := h.cfg.Configuration(r.Context(), r.PathValue("type_id"))
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (h *Handler) readiness(w http.ResponseWriter, r *http.Request) {
	if _, err := h.actor(r); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	data, err := h.cfg.Readiness(r.Context(), r.PathValue("type_id"))
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (h *Handler) validate(w http.ResponseWriter, r *http.Request) {
	if _, err := h.actor(r); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	data, err := h.cfg.Validate(r.Context(), r.PathValue("type_id"))
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (h *Handler) lifecycle(w http.ResponseWriter, r *http.Request) {
	if _, err := h.actor(r); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	data, err := h.cfg.Lifecycle(r.Context(), r.PathValue("type_id"))
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (h *Handler) versionDetail(w http.ResponseWriter, r *http.Request) {
	if _, err := h.actor(r); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	versionNo, convErr := strconv.Atoi(r.PathValue("version_no"))
	if convErr != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "version_no must be an integer", nil))
		return
	}
	rec, err := h.svc.GetPublishedVersion(r.Context(), r.PathValue("type_id"), versionNo)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if rec == nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "version not found", nil))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": rec})
}

func (h *Handler) actor(r *http.Request) (string, error) {
	tok := bearer(r.Header.Get("Authorization"))
	claims, err := h.inspector.InspectAccessToken(r.Context(), tok)
	if err != nil {
		return "", err
	}
	return claims.Sub, nil
}

func (h *Handler) listVersions(w http.ResponseWriter, r *http.Request) {
	if _, err := h.actor(r); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	versions, err := h.svc.ListVersions(r.Context(), r.PathValue("type_id"))
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": versions})
}

func (h *Handler) publish(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actor(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var payload struct {
		ChangeNote string `json:"change_note"`
	}
	if r.Body != nil && r.ContentLength != 0 {
		_ = json.NewDecoder(r.Body).Decode(&payload)
	}
	info, err := h.svc.PublishVersion(r.Context(), r.PathValue("type_id"), payload.ChangeNote, actor)
	if err != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, err.Error(), nil))
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"data": info})
}

func (h *Handler) activate(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actor(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	versionNo, convErr := strconv.Atoi(r.PathValue("version_no"))
	if convErr != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "version_no must be an integer", nil))
		return
	}
	info, err := h.svc.ActivateVersion(r.Context(), r.PathValue("type_id"), versionNo, actor)
	if err != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, err.Error(), nil))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": info})
}

func bearer(h string) string {
	h = strings.TrimSpace(h)
	parts := strings.SplitN(h, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
		return strings.TrimSpace(parts[1])
	}
	return h
}
