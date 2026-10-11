package http

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"reflect"
	"strings"

	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
	platformcmsapp "github.com/cobo/cobo_iam_services/internal/platformcms/app"
)

func alertConfigAuditSnapshot(deadlineEnabled bool, deadlineKey string, stepEnabled bool, stepKey string) map[string]any {
	return map[string]any{
		"deadline":      map[string]any{"enabled": deadlineEnabled, "template_key": deadlineKey},
		"workflow_step": map[string]any{"enabled": stepEnabled, "template_key": stepKey},
	}
}

func (h *Handler) getAlertConfig(w http.ResponseWriter, r *http.Request) {
	if h.alertConfigSvc == nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeServiceUnavailable, "alert config service unavailable", nil))
		return
	}
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireCMSAccess(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	typeID := strings.TrimSpace(r.PathValue("typeId"))
	if typeID == "" {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "typeId is required", nil))
		return
	}
	dto, err := h.alertConfigSvc.GetAlertConfig(r.Context(), typeID)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	writeEnvelope(w, http.StatusOK, map[string]any{
		"typeId": dto.TypeID,
		"deadline": map[string]any{
			"enabled":     dto.Deadline.Enabled,
			"templateKey": dto.Deadline.TemplateKey,
		},
		"workflowStep": map[string]any{
			"enabled":     dto.WorkflowStep.Enabled,
			"templateKey": dto.WorkflowStep.TemplateKey,
		},
	}, nil)
}

func (h *Handler) putAlertConfig(w http.ResponseWriter, r *http.Request) {
	if h.alertConfigSvc == nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeServiceUnavailable, "alert config service unavailable", nil))
		return
	}
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	// PUT requires platform.cms.view AND a template-config capability (same set as the
	// disclosure requireCMSTemplateConfigWrite gate; rbac.manage kept as legacy alias).
	if _, err := h.requireCMSAccess(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireAnyPermission(r.Context(), sub.MembershipID, sub.CompanyID,
		"cms.template.config.write", "cms.template.write", "rbac.manage"); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	typeID := strings.TrimSpace(r.PathValue("typeId"))
	if typeID == "" {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "typeId is required", nil))
		return
	}
	var body struct {
		Deadline struct {
			Enabled     bool   `json:"enabled"`
			TemplateKey string `json:"templateKey"`
		} `json:"deadline"`
		WorkflowStep struct {
			Enabled     bool   `json:"enabled"`
			TemplateKey string `json:"templateKey"`
		} `json:"workflowStep"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid JSON body", err))
		return
	}
	req := platformcmsapp.UpsertAlertConfigRequest{
		TypeID:  typeID,
		ActorID: sub.Sub,
		Deadline: platformcmsapp.AlertKindConfigInput{
			Enabled:     body.Deadline.Enabled,
			TemplateKey: body.Deadline.TemplateKey,
		},
		WorkflowStep: platformcmsapp.AlertKindConfigInput{
			Enabled:     body.WorkflowStep.Enabled,
			TemplateKey: body.WorkflowStep.TemplateKey,
		},
	}
	var before map[string]any
	if prev, prevErr := h.alertConfigSvc.GetAlertConfig(r.Context(), typeID); prevErr == nil && prev != nil {
		before = alertConfigAuditSnapshot(prev.Deadline.Enabled, prev.Deadline.TemplateKey, prev.WorkflowStep.Enabled, prev.WorkflowStep.TemplateKey)
	}
	if err := h.alertConfigSvc.UpsertAlertConfig(r.Context(), req); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	after := alertConfigAuditSnapshot(body.Deadline.Enabled, body.Deadline.TemplateKey, body.WorkflowStep.Enabled, body.WorkflowStep.TemplateKey)
	// The editor re-asserts defaults on every save; only real changes are audited.
	if h.auditSvc != nil && !reflect.DeepEqual(before, after) {
		if err := h.auditSvc.AppendAuditLog(r.Context(), auditapp.AppendAuditLogRequest{
			ActorUserID:       sub.Sub,
			ActorMembershipID: sub.MembershipID,
			CompanyID:         sub.CompanyID,
			Action:            cmsActionTemplateAlertConfigUpdate,
			ResourceType:      "disclosure_type",
			ResourceID:        typeID,
			Decision:          "allow",
			RequestID:         httpx.RequestIDFromContext(r.Context()),
			IP:                r.RemoteAddr,
			UserAgent:         r.UserAgent(),
			Metadata: map[string]any{
				"before": before,
				"after":  after,
			},
		}); err != nil {
			slog.Error("audit append failed", "action", cmsActionTemplateAlertConfigUpdate, "resource_id", typeID, "err", err.Error())
		}
	}
	writeEnvelope(w, http.StatusOK, map[string]any{"ok": true}, nil)
}
