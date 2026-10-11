// Package http exposes the global workflow versioning lifecycle (publish ≠ activate).
// Routes are registered only when WORKFLOW_VERSIONING_ENABLED is true (server wiring), so the
// existing GET/PUT workflow APIs are unaffected when the flag is OFF.
package http

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
	wfcapp "github.com/cobo/cobo_iam_services/internal/workflowconfig/app"
)

type Handler struct {
	svc        *wfcapp.VersionService
	cfg        *wfcapp.ConfigService
	catalog    *wfcapp.AssigneeRoleCatalogService
	inspector  iamapp.TokenInspector
	authorizer AccessResolver
	audit      auditapp.Service
}

func NewHandler(svc *wfcapp.VersionService, cfg *wfcapp.ConfigService, catalog *wfcapp.AssigneeRoleCatalogService, inspector iamapp.TokenInspector, authorizer AccessResolver) *Handler {
	return &Handler{svc: svc, cfg: cfg, catalog: catalog, inspector: inspector, authorizer: authorizer}
}

// WithAudit records publish/activate of global workflow versions (they change every tenant's
// effective workflow). Nil-safe: without it the handler behaves as before.
func (h *Handler) WithAudit(audit auditapp.Service) *Handler {
	h.audit = audit
	return h
}

func (h *Handler) auditLog(r *http.Request, sub subject, action, typeID string, metadata map[string]any) {
	if h.audit == nil {
		return
	}
	if err := h.audit.AppendAuditLog(r.Context(), auditapp.AppendAuditLogRequest{
		ActorUserID:       sub.UserID,
		ActorMembershipID: sub.MembershipID,
		CompanyID:         sub.CompanyID,
		Action:            action,
		ResourceType:      "disclosure_type",
		ResourceID:        typeID,
		Decision:          "allow",
		RequestID:         httpx.RequestIDFromContext(r.Context()),
		IP:                r.RemoteAddr,
		UserAgent:         r.UserAgent(),
		Metadata:          metadata,
	}); err != nil {
		slog.Error("audit append failed", "action", action, "resource_id", typeID, "err", err.Error())
	}
}

// RegisterAssigneeRoleCatalog wires GET/POST assignee role catalog routes (independent of workflow versioning).
func RegisterAssigneeRoleCatalog(mux *http.ServeMux, catalog *wfcapp.AssigneeRoleCatalogService, inspector iamapp.TokenInspector, authorizer AccessResolver) {
	h := &Handler{catalog: catalog, inspector: inspector, authorizer: authorizer}
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
	if _, err := h.requireTemplateRead(r); err != nil {
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
	if _, err := h.requireTemplateRead(r); err != nil {
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
	if _, err := h.requireTemplateRead(r); err != nil {
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
	if _, err := h.requireTemplateRead(r); err != nil {
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
	if _, err := h.requireTemplateRead(r); err != nil {
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

func (h *Handler) listVersions(w http.ResponseWriter, r *http.Request) {
	if _, err := h.requireTemplateRead(r); err != nil {
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
	sub, err := h.requireTemplateWrite(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var payload struct {
		ChangeNote string `json:"change_note"`
	}
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxPublishBodyBytes)).Decode(&payload); err != nil && !errors.Is(err, io.EOF) {
			httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid JSON body", nil))
			return
		}
	}
	info, err := h.svc.PublishVersion(r.Context(), r.PathValue("type_id"), payload.ChangeNote, sub.UserID)
	if err != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, err.Error(), nil))
		return
	}
	h.auditLog(r, sub, "cms_workflow.version.publish", info.TypeID, map[string]any{
		"version_no":  info.VersionNo,
		"state":       info.State,
		"change_note": truncateForAudit(payload.ChangeNote),
	})
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"data": info})
}

func (h *Handler) activate(w http.ResponseWriter, r *http.Request) {
	sub, err := h.requireTemplateActivate(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	versionNo, convErr := strconv.Atoi(r.PathValue("version_no"))
	if convErr != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "version_no must be an integer", nil))
		return
	}
	typeID := r.PathValue("type_id")
	var prevActive any // nil when no version was active
	if h.audit != nil {
		if cur, curErr := h.svc.GetCurrentVersion(r.Context(), typeID); curErr == nil && cur != nil {
			prevActive = cur.VersionNo
		}
	}
	info, err := h.svc.ActivateVersion(r.Context(), typeID, versionNo, sub.UserID)
	if err != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, err.Error(), nil))
		return
	}
	h.auditLog(r, sub, "cms_workflow.version.activate", typeID, map[string]any{
		"version_no":             info.VersionNo,
		"prev_active_version_no": prevActive,
		"state":                  info.State,
	})
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": info})
}

const (
	maxPublishBodyBytes   int64 = 64 << 10
	maxAuditChangeNoteLen       = 500
)

// truncateForAudit keeps audit rows small (free text is stored in full on the version itself).
func truncateForAudit(s string) string {
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if len(runes) <= maxAuditChangeNoteLen {
		return s
	}
	return string(runes[:maxAuditChangeNoteLen]) + "…"
}

func bearer(h string) string {
	h = strings.TrimSpace(h)
	parts := strings.SplitN(h, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
		return strings.TrimSpace(parts[1])
	}
	return h
}
