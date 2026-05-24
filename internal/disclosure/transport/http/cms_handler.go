package http

import (
	"encoding/json"
	"net/http"
	"strings"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
)

// ─── CMS Template Management ──────────────────────────────────────────────────

func (h *Handler) cmsArchiveTemplate(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	typeID := strings.TrimSpace(r.PathValue("type_id"))
	var body struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	resp, err := h.svc.CmsArchiveTemplate(r.Context(), disclosureapp.CmsArchiveTemplateRequest{
		Subject: sub,
		TypeID:  typeID,
		Reason:  body.Reason,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, sub, "cms_template.archive", "disclosure_type", typeID, map[string]any{
		"reason": body.Reason,
	})
	httpx.WriteJSON(w, http.StatusOK, resp)
}

// ─── Global Workflow ──────────────────────────────────────────────────────────

func (h *Handler) cmsGetGlobalWorkflow(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	typeID := strings.TrimSpace(r.PathValue("type_id"))
	resp, err := h.svc.CmsGetGlobalWorkflow(r.Context(), disclosureapp.CmsGetGlobalWorkflowRequest{
		Subject: sub,
		TypeID:  typeID,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) cmsUpsertGlobalWorkflow(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	typeID := strings.TrimSpace(r.PathValue("type_id"))
	var req disclosureapp.CmsUpsertGlobalWorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	req.Subject = sub
	req.TypeID = typeID
	resp, err := h.svc.CmsUpsertGlobalWorkflow(r.Context(), req)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, sub, "cms_workflow.upsert", "disclosure_type", typeID, nil)
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) cmsDeleteGlobalWorkflow(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	typeID := strings.TrimSpace(r.PathValue("type_id"))
	if err := h.svc.CmsDeleteGlobalWorkflow(r.Context(), disclosureapp.CmsDeleteGlobalWorkflowRequest{
		Subject: sub,
		TypeID:  typeID,
	}); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, sub, "cms_workflow.delete", "disclosure_type", typeID, nil)
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

// ─── Display Groups ───────────────────────────────────────────────────────────

func (h *Handler) cmsListDisplayGroups(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	resp, err := h.svc.CmsListDisplayGroupsCatalog(r.Context(), disclosureapp.ListDisplayGroupsRequest{Subject: sub})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) cmsCreateDisplayGroup(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var req disclosureapp.CmsDisplayGroupCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	req.Subject = sub
	resp, err := h.svc.CmsCreateDisplayGroup(r.Context(), req)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, sub, "cms_display_group.create", "display_group", resp.DisplayGroupCode, nil)
	httpx.WriteJSON(w, http.StatusCreated, resp)
}

func (h *Handler) cmsUpdateDisplayGroup(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	code := strings.TrimSpace(r.PathValue("code"))
	var req disclosureapp.CmsDisplayGroupUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	req.Subject = sub
	req.Code = code
	resp, err := h.svc.CmsUpdateDisplayGroup(r.Context(), req)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, sub, "cms_display_group.update", "display_group", code, nil)
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) cmsDeleteDisplayGroup(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	code := strings.TrimSpace(r.PathValue("code"))
	if err := h.svc.CmsDeleteDisplayGroup(r.Context(), disclosureapp.CmsDisplayGroupDeleteRequest{
		Subject: sub,
		Code:    code,
	}); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, sub, "cms_display_group.delete", "display_group", code, nil)
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

// ─── Deadline Rules ───────────────────────────────────────────────────────────

func (h *Handler) cmsListDeadlineRules(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	resp, err := h.svc.CmsListDeadlineRules(r.Context(), disclosureapp.GetTemplateReferenceDataRequest{Subject: sub})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": resp})
}

func (h *Handler) cmsCreateDeadlineRule(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var req disclosureapp.CmsDeadlineRuleCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	req.Subject = sub
	resp, err := h.svc.CmsCreateDeadlineRule(r.Context(), req)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, sub, "cms_deadline_rule.create", "deadline_rule", resp.RuleID, nil)
	httpx.WriteJSON(w, http.StatusCreated, resp)
}

func (h *Handler) cmsUpdateDeadlineRule(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	ruleID := strings.TrimSpace(r.PathValue("rule_id"))
	var req disclosureapp.CmsDeadlineRuleUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	req.Subject = sub
	req.RuleID = ruleID
	resp, err := h.svc.CmsUpdateDeadlineRule(r.Context(), req)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, sub, "cms_deadline_rule.update", "deadline_rule", ruleID, nil)
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) cmsDeleteDeadlineRule(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	ruleID := strings.TrimSpace(r.PathValue("rule_id"))
	if err := h.svc.CmsDeleteDeadlineRule(r.Context(), disclosureapp.CmsDeadlineRuleDeleteRequest{
		Subject: sub,
		RuleID:  ruleID,
	}); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, sub, "cms_deadline_rule.delete", "deadline_rule", ruleID, nil)
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}
