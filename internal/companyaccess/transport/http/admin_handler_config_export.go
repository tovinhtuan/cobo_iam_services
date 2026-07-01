package http

import (
	"encoding/json"
	"net/http"
	"strings"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
)

func (h *AdminHandler) registerConfigExportRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/admin/config-export", h.createConfigExport)
	mux.HandleFunc("GET /api/v1/admin/config-export/{export_id}", h.getConfigExport)
	mux.HandleFunc("GET /api/v1/admin/config-export/{export_id}/download", h.downloadConfigExport)
}

func (h *AdminHandler) createConfigExport(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var body struct {
		Modules []string `json:"modules"`
	}
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httpx.WriteError(w, nil, err)
			return
		}
	}
	out, err := h.svc.CreateConfigExport(r.Context(), caapp.CreateConfigExportRequest{
		Subject: sub,
		Modules: body.Modules,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditConfigExportLog(r, sub, "config.export.requested", out.ExportID, caapp.ConfigExportAuditMetadata(out))
	httpx.WriteJSON(w, http.StatusCreated, out)
}

func (h *AdminHandler) getConfigExport(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	exportID := strings.TrimSpace(r.PathValue("export_id"))
	out, err := h.svc.GetConfigExport(r.Context(), caapp.GetConfigExportRequest{
		Subject: sub, ExportID: exportID,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *AdminHandler) downloadConfigExport(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	exportID := strings.TrimSpace(r.PathValue("export_id"))
	raw, err := h.svc.DownloadConfigExport(r.Context(), caapp.DownloadConfigExportRequest{
		Subject: sub, ExportID: exportID,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="enterprise-config-export.json"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}

func (h *AdminHandler) auditConfigExportLog(r *http.Request, sub caapp.AdminSubject, action, resourceID string, meta map[string]any) {
	if h.audit == nil {
		return
	}
	_ = h.audit.AppendAuditLog(r.Context(), auditapp.AppendAuditLogRequest{
		ActorUserID: sub.UserID, ActorMembershipID: sub.MembershipID, CompanyID: sub.CompanyID,
		Action: action, ResourceType: "config_export", ResourceID: resourceID, Decision: "allow",
		RequestID: httpx.RequestIDFromContext(r.Context()), IP: r.RemoteAddr, UserAgent: r.UserAgent(),
		Metadata: meta,
	})
}
