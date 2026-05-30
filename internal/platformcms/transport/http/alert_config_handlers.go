package http

import (
	"encoding/json"
	"net/http"
	"strings"

	platformcmsapp "github.com/cobo/cobo_iam_services/internal/platformcms/app"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

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
	// PUT requires platform.cms.view AND rbac.manage.
	if _, err := h.requireCMSAccess(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireAnyPermission(r.Context(), sub.MembershipID, sub.CompanyID, "rbac.manage"); err != nil {
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
	if err := h.alertConfigSvc.UpsertAlertConfig(r.Context(), req); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	writeEnvelope(w, http.StatusOK, map[string]any{"ok": true}, nil)
}
