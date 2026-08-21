package http

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
	"github.com/cobo/cobo_iam_services/internal/platform/idempotency"
)

type Handler struct {
	svc       disclosureapp.Service
	inspector iamapp.TokenInspector
	idem      idempotency.Store
	audit     auditapp.Service
}

func NewHandler(svc disclosureapp.Service, inspector iamapp.TokenInspector, idem idempotency.Store, audit auditapp.Service) *Handler {
	return &Handler{svc: svc, inspector: inspector, idem: idem, audit: audit}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/disclosures", h.createRecord)
	mux.HandleFunc("GET /api/v1/disclosures", h.listRecords)
	mux.HandleFunc("GET /api/v1/disclosures/{record_id}", h.getRecord)
	mux.HandleFunc("PATCH /api/v1/disclosures/{record_id}", h.updateRecord)
	mux.HandleFunc("POST /api/v1/disclosures/{record_id}/submit", h.submitRecord)
	mux.HandleFunc("POST /api/v1/disclosures/{record_id}/confirm", h.confirmRecord)
	mux.HandleFunc("GET /api/v1/disclosure-groups", h.listTypeGroups)
	mux.HandleFunc("GET /api/v1/disclosure-types/display-groups", h.listDisplayGroups)
	mux.HandleFunc("GET /api/v1/disclosure-types/filter-options", h.listTypeFilterOptions)
	mux.HandleFunc("GET /api/v1/disclosure-types", h.listTypes)
	mux.HandleFunc("GET /api/v1/disclosure-types/{type_id}", h.getTypeDetail)
	mux.HandleFunc("GET /api/v1/admin/disclosure-types/reference-data", h.getTemplateReferenceData)
	mux.HandleFunc("PUT /api/v1/admin/disclosure-types/{type_id}", h.upsertTypeVersion)
	mux.HandleFunc("POST /api/v1/admin/disclosure-types/{type_id}/clone", h.cloneTypeFromActive)
	mux.HandleFunc("GET /api/v1/admin/disclosure-types/{type_id}/versions", h.listTypeVersions)
	mux.HandleFunc("GET /api/v1/admin/disclosure-types/{type_id}/versions/{version_no}", h.getTypeVersionDetail)
	mux.HandleFunc("POST /api/v1/admin/disclosure-types/{type_id}/activate", h.activateTypeVersion)
	mux.HandleFunc("GET /api/v1/company/disclosure-types/{type_id}/workflow-override", h.getCompanyWorkflowOverride)
	mux.HandleFunc("PUT /api/v1/company/disclosure-types/{type_id}/workflow-override/draft", h.upsertCompanyWorkflowOverrideDraft)
	mux.HandleFunc("POST /api/v1/company/disclosure-types/{type_id}/workflow-override/approve", h.approveCompanyWorkflowOverride)
	mux.HandleFunc("DELETE /api/v1/company/disclosure-types/{type_id}/workflow-override/draft/{version_no}", h.deleteCompanyWorkflowOverrideDraft)
	mux.HandleFunc("DELETE /api/v1/company/disclosure-types/{type_id}/workflow-override/active", h.resetCompanyWorkflowOverrideActive)
	mux.HandleFunc("GET /api/v1/company/disclosure-types/{type_id}/workflow-override/versions", h.listCompanyWorkflowOverrideVersions)
	mux.HandleFunc("GET /api/v1/company/disclosure-types/{type_id}/workflow-override/draft/reminder-preview", h.getCompanyWorkflowOverrideDraftReminderPreview)
	mux.HandleFunc("PUT /api/v1/company/disclosure-types/{type_id}/workflow-override/draft/steps/{step_id}/groups", h.updateWorkflowOverrideStepGroups)
	// Sprint 3 / Batch 2 — Workflow Override Staleness Detection.
	mux.HandleFunc("GET /api/v1/company/disclosure-types/{type_id}/workflow/override/status", h.getWorkflowOverrideStatus)
	mux.HandleFunc("POST /api/v1/company/disclosure-types/{type_id}/workflow/override/rebase-check", h.rebaseCheckWorkflowOverride)
	// Sprint 3 / Batch 3 — Workflow Override Rebase Preview. Read-only; zero DB writes.
	mux.HandleFunc("GET /api/v1/company/disclosure-types/{type_id}/workflow/override/rebase-preview", h.getWorkflowOverrideRebasePreview)
	// Sprint 3 / Batch 4 — Workflow Override Conflict Detection. Writes ONLY workflow_override_conflicts.
	mux.HandleFunc("POST /api/v1/company/disclosure-types/{type_id}/workflow/override/conflicts/{conflict_id}/resolve", h.resolveWorkflowOverrideConflict)
	// Sprint 3 / Batch 5 — Workflow Override Rebase Apply. The only Sprint 3 endpoint that can
	// change what GetEffectiveWorkflow subsequently returns.
	mux.HandleFunc("POST /api/v1/company/disclosure-types/{type_id}/workflow/override/rebase-apply", h.applyWorkflowOverrideRebase)
	mux.HandleFunc("GET /api/v1/company/groups", h.listCompanyGroups)
	mux.HandleFunc("GET /api/v1/disclosure-types/{type_id}/effective-workflow", h.getEffectiveWorkflow)
	mux.HandleFunc("GET /api/v1/admin/disclosure-types/{type_id}/config", h.getTemplateDeadlineConfig)
	mux.HandleFunc("PUT /api/v1/admin/disclosure-types/{type_id}/config", h.updateTemplateDeadlineConfig)
	mux.HandleFunc("GET /api/v1/company/disclosure-types/{type_id}/preferences", h.getCompanyTypePreference)
	mux.HandleFunc("PATCH /api/v1/company/disclosure-types/{type_id}/preferences", h.upsertCompanyTypePreference)
	// BE-004A: company-defined template create/update (portal path)
	mux.HandleFunc("POST /api/v1/company/disclosure-types", h.createCompanyTemplate)
	mux.HandleFunc("PUT /api/v1/company/disclosure-types/{type_id}", h.updateCompanyTemplate)
	// BE-004B: lifecycle transitions
	mux.HandleFunc("POST /api/v1/company/disclosure-types/{type_id}/lifecycle/{action}", h.transitionCompanyTemplateLifecycle)

	// Platform CMS — platform admin only (gate: platform.cms.view)
	mux.HandleFunc("POST /api/v1/platform/cms/templates/{type_id}/archive", h.cmsArchiveTemplate)
	mux.HandleFunc("GET /api/v1/platform/cms/templates/{type_id}/workflow", h.cmsGetGlobalWorkflow)
	mux.HandleFunc("PUT /api/v1/platform/cms/templates/{type_id}/workflow", h.cmsUpsertGlobalWorkflow)
	mux.HandleFunc("DELETE /api/v1/platform/cms/templates/{type_id}/workflow", h.cmsDeleteGlobalWorkflow)
	mux.HandleFunc("GET /api/v1/platform/cms/display-groups", h.cmsListDisplayGroups)
	mux.HandleFunc("POST /api/v1/platform/cms/display-groups", h.cmsCreateDisplayGroup)
	mux.HandleFunc("PATCH /api/v1/platform/cms/display-groups/{code}", h.cmsUpdateDisplayGroup)
	mux.HandleFunc("DELETE /api/v1/platform/cms/display-groups/{code}", h.cmsDeleteDisplayGroup)
	mux.HandleFunc("GET /api/v1/platform/cms/workflow/template-departments", h.cmsListTemplateDepartments)
	mux.HandleFunc("POST /api/v1/platform/cms/workflow/template-departments", h.cmsCreateTemplateDepartment)
	mux.HandleFunc("GET /api/v1/platform/cms/deadline-rules", h.cmsListDeadlineRules)
	mux.HandleFunc("POST /api/v1/platform/cms/deadline-rules", h.cmsCreateDeadlineRule)
	mux.HandleFunc("PATCH /api/v1/platform/cms/deadline-rules/{rule_id}", h.cmsUpdateDeadlineRule)
	mux.HandleFunc("DELETE /api/v1/platform/cms/deadline-rules/{rule_id}", h.cmsDeleteDeadlineRule)
}

func (h *Handler) getTemplateReferenceData(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	resp, err := h.svc.GetTemplateReferenceData(r.Context(), disclosureapp.GetTemplateReferenceDataRequest{
		Subject: sub,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) createRecord(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var payload disclosureapp.RecordPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	resp, err := h.svc.CreateRecord(r.Context(), disclosureapp.CreateRecordRequest{Subject: sub, Payload: payload})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, resp)
}

func (h *Handler) updateRecord(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	recordID := r.PathValue("record_id")
	var payload disclosureapp.RecordPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	resp, err := h.svc.UpdateRecord(r.Context(), disclosureapp.UpdateRecordRequest{Subject: sub, RecordID: recordID, Payload: payload})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) submitRecord(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	recordID := r.PathValue("record_id")
	idemKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	var res idempotency.Result
	if idemKey != "" && h.idem != nil {
		hash := disclosureRequestHash(sub.CompanyID, recordID, sub.UserID, "submit")
		res, err = h.idem.TryReserve(r.Context(), idempotency.Params{
			CompanyID: sub.CompanyID, Scope: "disclosure.submit", Key: idemKey, RequestHash: hash,
		})
		if err != nil {
			httpx.WriteError(w, nil, err)
			return
		}
		if res.Replay {
			httpx.WriteJSONRaw(w, res.ReplayHTTPStatus, res.ReplayBody)
			return
		}
		if res.Conflict {
			httpx.WriteJSON(w, http.StatusConflict, idempotencyConflictBody("idempotency conflict or request in progress"))
			return
		}
	}
	resp, err := h.svc.SubmitRecord(r.Context(), disclosureapp.SubmitRecordRequest{Subject: sub, RecordID: recordID})
	if res.ReservationID != "" && h.idem != nil {
		if err != nil {
			_ = h.idem.Abandon(r.Context(), res.ReservationID)
		} else {
			body, _ := json.Marshal(resp)
			env := idempotency.Envelope{HTTPStatus: http.StatusOK, Body: body}
			envBytes, _ := json.Marshal(&env)
			_ = h.idem.Complete(r.Context(), res.ReservationID, envBytes)
		}
	}
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) confirmRecord(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	recordID := r.PathValue("record_id")
	idemKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	var res idempotency.Result
	if idemKey != "" && h.idem != nil {
		hash := disclosureRequestHash(sub.CompanyID, recordID, sub.UserID, "confirm")
		res, err = h.idem.TryReserve(r.Context(), idempotency.Params{
			CompanyID: sub.CompanyID, Scope: "disclosure.confirm", Key: idemKey, RequestHash: hash,
		})
		if err != nil {
			httpx.WriteError(w, nil, err)
			return
		}
		if res.Replay {
			httpx.WriteJSONRaw(w, res.ReplayHTTPStatus, res.ReplayBody)
			return
		}
		if res.Conflict {
			httpx.WriteJSON(w, http.StatusConflict, idempotencyConflictBody("idempotency conflict or request in progress"))
			return
		}
	}
	resp, err := h.svc.ConfirmRecord(r.Context(), disclosureapp.ConfirmRecordRequest{Subject: sub, RecordID: recordID})
	if res.ReservationID != "" && h.idem != nil {
		if err != nil {
			_ = h.idem.Abandon(r.Context(), res.ReservationID)
		} else {
			body, _ := json.Marshal(resp)
			env := idempotency.Envelope{HTTPStatus: http.StatusOK, Body: body}
			envBytes, _ := json.Marshal(&env)
			_ = h.idem.Complete(r.Context(), res.ReservationID, envBytes)
		}
	}
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) listRecords(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	resp, err := h.svc.ListRecords(r.Context(), disclosureapp.ListRecordsRequest{Subject: sub})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) getRecord(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	recordID := r.PathValue("record_id")
	resp, err := h.svc.GetRecord(r.Context(), disclosureapp.GetRecordRequest{Subject: sub, RecordID: recordID})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) listDisplayGroups(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	resp, err := h.svc.ListDisplayGroups(r.Context(), disclosureapp.ListDisplayGroupsRequest{Subject: sub})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) listTypeGroups(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	resp, err := h.svc.ListTypeGroups(r.Context(), disclosureapp.ListTypeGroupsRequest{Subject: sub})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) listTypes(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	pageRaw := strings.TrimSpace(r.URL.Query().Get("page"))
	pageSizeRaw := strings.TrimSpace(r.URL.Query().Get("page_size"))
	page, _ := strconv.Atoi(pageRaw)
	pageSize, _ := strconv.Atoi(pageSizeRaw)
	var hasOpenDraft *bool
	if raw := strings.TrimSpace(r.URL.Query().Get("has_open_draft")); raw != "" {
		switch strings.ToLower(raw) {
		case "true", "1":
			v := true
			hasOpenDraft = &v
		case "false", "0":
			v := false
			hasOpenDraft = &v
		default:
			httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "has_open_draft must be true or false", nil))
			return
		}
	}
	resp, err := h.svc.ListTypes(r.Context(), disclosureapp.ListTypesRequest{
		Subject:          sub,
		GroupID:          strings.TrimSpace(r.URL.Query().Get("group_id")),
		DisplayGroupCode: strings.TrimSpace(r.URL.Query().Get("display_group_code")),
		Query:            strings.TrimSpace(r.URL.Query().Get("q")),
		Scope:            strings.TrimSpace(r.URL.Query().Get("scope")),
		Tags:             disclosureapp.ParseTagQuery(r.URL.Query().Get("tag_ids")),
		Periodicity:      strings.TrimSpace(r.URL.Query().Get("frequency")),
		DepartmentID:     strings.TrimSpace(r.URL.Query().Get("department_id")),
		Page:             page,
		PageSize:         pageSize,
		PageProvided:     pageRaw != "",
		PageSizeProvided: pageSizeRaw != "",
		SortBy:           strings.TrimSpace(r.URL.Query().Get("sort_by")),
		SortDir:          strings.TrimSpace(r.URL.Query().Get("sort_dir")),
		ListMode:         strings.TrimSpace(r.URL.Query().Get("list_mode")),
		PortalState:      strings.TrimSpace(r.URL.Query().Get("portal_state")),
		HasOpenDraft:     hasOpenDraft,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) listTypeFilterOptions(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	resp, err := h.svc.ListTypeFilterOptions(r.Context(), disclosureapp.ListTypeFilterOptionsRequest{Subject: sub})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) getTypeDetail(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	resp, err := h.svc.GetTypeDetail(r.Context(), disclosureapp.GetTypeDetailRequest{
		Subject: sub,
		TypeID:  strings.TrimSpace(r.PathValue("type_id")),
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) upsertTypeVersion(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var payload disclosureapp.UpsertTypeVersionRequest
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var rawKeys map[string]json.RawMessage
	if err := json.Unmarshal(rawBody, &rawKeys); err == nil {
		if _, ok := rawKeys["legal_bases"]; ok {
			payload.LegalBasesProvided = true
		}
	}
	payload.Subject = sub
	payload.TypeID = strings.TrimSpace(r.PathValue("type_id"))
	prev, _ := h.svc.GetTypeDetail(r.Context(), disclosureapp.GetTypeDetailRequest{Subject: sub, TypeID: payload.TypeID})
	resp, err := h.svc.UpsertTypeVersion(r.Context(), payload)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	prevVersion := 0
	if prev != nil {
		prevVersion = prev.VersionNo
	}
	h.auditLog(r, sub, "disclosure.type.version.upsert", "disclosure_type", payload.TypeID, map[string]any{
		"old_version_no": prevVersion,
		"new_version_no": resp.VersionNo,
		"change_note":    strings.TrimSpace(payload.ChangeNote),
	})
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) cloneTypeFromActive(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var payload disclosureapp.CloneTypeFromActiveRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	payload.Subject = sub
	payload.SourceTypeID = strings.TrimSpace(r.PathValue("type_id"))
	resp, err := h.svc.CloneTypeFromActive(r.Context(), payload)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, sub, "disclosure.type.clone", "disclosure_type", resp.TypeID, disclosureapp.CloneAuditMetadata(resp))
	httpx.WriteJSON(w, http.StatusCreated, resp)
}

func (h *Handler) listTypeVersions(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	resp, err := h.svc.ListTypeVersions(r.Context(), disclosureapp.ListTypeVersionsRequest{
		Subject: sub,
		TypeID:  strings.TrimSpace(r.PathValue("type_id")),
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) getTypeVersionDetail(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	versionNo, err := strconv.Atoi(strings.TrimSpace(r.PathValue("version_no")))
	if err != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "version_no must be integer", nil))
		return
	}
	resp, err := h.svc.GetTypeVersionDetail(r.Context(), disclosureapp.GetTypeVersionDetailRequest{
		Subject:   sub,
		TypeID:    strings.TrimSpace(r.PathValue("type_id")),
		VersionNo: versionNo,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) activateTypeVersion(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var payload struct {
		VersionNo int    `json:"version_no"`
		Reason    string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	typeID := strings.TrimSpace(r.PathValue("type_id"))
	prev, _ := h.svc.GetTypeDetail(r.Context(), disclosureapp.GetTypeDetailRequest{
		Subject: sub,
		TypeID:  typeID,
	})
	resp, err := h.svc.ActivateTypeVersion(r.Context(), disclosureapp.ActivateTypeVersionRequest{
		Subject:   sub,
		TypeID:    typeID,
		VersionNo: payload.VersionNo,
		Reason:    strings.TrimSpace(payload.Reason),
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	prevVersion := 0
	if prev != nil {
		prevVersion = prev.VersionNo
	}
	h.auditLog(r, sub, "disclosure.type.version.activate", "disclosure_type", typeID, map[string]any{
		"old_version_no": prevVersion,
		"new_version_no": resp.VersionNo,
		"reason":         strings.TrimSpace(payload.Reason),
	})
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) getCompanyWorkflowOverride(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	resp, err := h.svc.GetCompanyWorkflowOverride(r.Context(), disclosureapp.GetCompanyWorkflowOverrideRequest{
		Subject: sub,
		TypeID:  strings.TrimSpace(r.PathValue("type_id")),
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

// Sprint 3 / Batch 2 — Workflow Override Staleness Detection.

func (h *Handler) getWorkflowOverrideStatus(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	resp, err := h.svc.GetWorkflowOverrideStatus(r.Context(), disclosureapp.GetWorkflowOverrideStatusRequest{
		Subject: sub,
		TypeID:  strings.TrimSpace(r.PathValue("type_id")),
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) rebaseCheckWorkflowOverride(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	resp, err := h.svc.RebaseCheckWorkflowOverride(r.Context(), disclosureapp.RebaseCheckWorkflowOverrideRequest{
		Subject: sub,
		TypeID:  strings.TrimSpace(r.PathValue("type_id")),
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) getWorkflowOverrideRebasePreview(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	resp, err := h.svc.GetWorkflowOverrideRebasePreview(r.Context(), disclosureapp.GetWorkflowOverrideRebasePreviewRequest{
		Subject: sub,
		TypeID:  strings.TrimSpace(r.PathValue("type_id")),
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) resolveWorkflowOverrideConflict(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var body struct {
		Resolution      string `json:"resolution"`
		ResolutionValue any    `json:"resolution_value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	resp, err := h.svc.ResolveWorkflowOverrideConflict(r.Context(), disclosureapp.ResolveWorkflowOverrideConflictRequest{
		Subject:         sub,
		TypeID:          strings.TrimSpace(r.PathValue("type_id")),
		ConflictID:      strings.TrimSpace(r.PathValue("conflict_id")),
		Resolution:      body.Resolution,
		ResolutionValue: body.ResolutionValue,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

// applyWorkflowOverrideRebase — Sprint 3 / Batch 5. Idempotency-Key is MANDATORY here (deviating
// from submit/confirm's optional convention — IDEMPOTENCY_CONTRACT.md's documented, deliberate
// exception): this is the only Sprint 3 write that changes runtime, so an unkeyed retry must never
// be allowed to risk a duplicate apply.
func (h *Handler) applyWorkflowOverrideRebase(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	typeID := strings.TrimSpace(r.PathValue("type_id"))
	var body struct {
		PreviewID string `json:"preview_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	idemKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idemKey == "" || h.idem == nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "Idempotency-Key header is required for rebase-apply", nil))
		return
	}
	hash := disclosureRequestHash(sub.CompanyID, typeID, sub.UserID, "rebase-apply:"+strings.TrimSpace(body.PreviewID))
	res, err := h.idem.TryReserve(r.Context(), idempotency.Params{
		CompanyID: sub.CompanyID, Scope: "workflow.override.rebase-apply", Key: idemKey, RequestHash: hash,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if res.Replay {
		httpx.WriteJSONRaw(w, res.ReplayHTTPStatus, res.ReplayBody)
		return
	}
	if res.Conflict {
		httpx.WriteJSON(w, http.StatusConflict, idempotencyConflictBody("idempotency conflict: same key with a different preview, or a request already in progress"))
		return
	}

	resp, applyErr := h.svc.ApplyWorkflowOverrideRebase(r.Context(), disclosureapp.ApplyWorkflowOverrideRebaseRequest{
		Subject:   sub,
		TypeID:    typeID,
		PreviewID: strings.TrimSpace(body.PreviewID),
	})
	if res.ReservationID != "" {
		if applyErr != nil {
			_ = h.idem.Abandon(r.Context(), res.ReservationID)
		} else {
			respBody, _ := json.Marshal(resp)
			env := idempotency.Envelope{HTTPStatus: http.StatusOK, Body: respBody}
			envBytes, _ := json.Marshal(&env)
			_ = h.idem.Complete(r.Context(), res.ReservationID, envBytes)
		}
	}
	if applyErr != nil {
		httpx.WriteError(w, nil, applyErr)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) getCompanyWorkflowOverrideDraftReminderPreview(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	resp, err := h.svc.GetCompanyWorkflowOverrideDraftReminderPreview(r.Context(), disclosureapp.GetCompanyWorkflowOverrideDraftReminderPreviewRequest{
		Subject: sub,
		TypeID:  strings.TrimSpace(r.PathValue("type_id")),
		T0Date:  strings.TrimSpace(r.URL.Query().Get("t0")),
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": resp.Data})
}

func (h *Handler) upsertCompanyWorkflowOverrideDraft(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var payload disclosureapp.UpsertCompanyWorkflowOverrideDraftRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	payload.Subject = sub
	payload.TypeID = strings.TrimSpace(r.PathValue("type_id"))
	resp, err := h.svc.UpsertCompanyWorkflowOverrideDraft(r.Context(), payload)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	auditAction := "disclosure.workflow_override.draft_saved"
	if payload.Publish {
		auditAction = "disclosure.workflow_override.applied"
	}
	h.auditLog(r, sub, auditAction, "disclosure_type", payload.TypeID, map[string]any{
		"draft_version_no": resp.DraftVersionNo,
		"state":            resp.State,
	})
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) approveCompanyWorkflowOverride(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var payload disclosureapp.ApproveCompanyWorkflowOverrideRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	payload.Subject = sub
	payload.TypeID = strings.TrimSpace(r.PathValue("type_id"))
	resp, err := h.svc.ApproveCompanyWorkflowOverride(r.Context(), payload)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, sub, "disclosure.workflow_override.approved", "disclosure_type", payload.TypeID, map[string]any{
		"active_version_no": resp.ActiveVersionNo,
		"reason":            strings.TrimSpace(payload.Reason),
	})
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) deleteCompanyWorkflowOverrideDraft(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	versionNo, err := strconv.Atoi(strings.TrimSpace(r.PathValue("version_no")))
	if err != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "version_no must be integer", nil))
		return
	}
	resp, err := h.svc.DeleteCompanyWorkflowOverrideDraft(r.Context(), disclosureapp.DeleteCompanyWorkflowOverrideDraftRequest{
		Subject:   sub,
		TypeID:    strings.TrimSpace(r.PathValue("type_id")),
		VersionNo: versionNo,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, sub, "disclosure.workflow_override.draft_deleted", "disclosure_type", strings.TrimSpace(r.PathValue("type_id")), map[string]any{
		"version_no": versionNo,
	})
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) resetCompanyWorkflowOverrideActive(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var payload struct {
		Reason string `json:"reason"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&payload)
	}
	resp, err := h.svc.ResetCompanyWorkflowOverrideActive(r.Context(), disclosureapp.ResetCompanyWorkflowOverrideActiveRequest{
		Subject: sub,
		TypeID:  strings.TrimSpace(r.PathValue("type_id")),
		Reason:  strings.TrimSpace(payload.Reason),
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, sub, "disclosure.workflow_override.reset", "disclosure_type", strings.TrimSpace(r.PathValue("type_id")), map[string]any{
		"reason": strings.TrimSpace(payload.Reason),
	})
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) listCompanyWorkflowOverrideVersions(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	page, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("page")))
	pageSize, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("page_size")))
	resp, err := h.svc.ListCompanyWorkflowOverrideVersions(r.Context(), disclosureapp.ListCompanyWorkflowOverrideVersionsRequest{
		Subject:  sub,
		TypeID:   strings.TrimSpace(r.PathValue("type_id")),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) getEffectiveWorkflow(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	resp, err := h.svc.GetEffectiveWorkflow(r.Context(), disclosureapp.GetEffectiveWorkflowRequest{
		Subject: sub,
		TypeID:  strings.TrimSpace(r.PathValue("type_id")),
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) getTemplateDeadlineConfig(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	resp, err := h.svc.GetTemplateDeadlineConfig(r.Context(), disclosureapp.GetTemplateDeadlineConfigRequest{
		Subject: sub,
		TypeID:  strings.TrimSpace(r.PathValue("type_id")),
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) updateTemplateDeadlineConfig(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var req disclosureapp.UpdateTemplateDeadlineConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	req.Subject = sub
	req.TypeID = strings.TrimSpace(r.PathValue("type_id"))
	resp, err := h.svc.UpdateTemplateDeadlineConfig(r.Context(), req)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, sub, "disclosure.type.config.updated", "disclosure_type", req.TypeID, map[string]any{
		"t0_policy":       req.DeadlineConfig.T0Policy,
		"deadline_days":   req.DeadlineConfig.DeadlineDays,
		"processing_days": req.DeadlineConfig.ProcessingDays,
	})
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) auditLog(r *http.Request, sub disclosureapp.Subject, action, resourceType, resourceID string, metadata map[string]any) {
	if h.audit == nil {
		return
	}
	_ = h.audit.AppendAuditLog(r.Context(), auditapp.AppendAuditLogRequest{
		ActorUserID:       sub.UserID,
		ActorMembershipID: sub.MembershipID,
		CompanyID:         sub.CompanyID,
		Action:            action,
		ResourceType:      resourceType,
		ResourceID:        resourceID,
		Decision:          "allow",
		RequestID:         httpx.RequestIDFromContext(r.Context()),
		IP:                r.RemoteAddr,
		UserAgent:         r.UserAgent(),
		Metadata:          metadata,
	})
}

func (h *Handler) subjectFromToken(r *http.Request) (disclosureapp.Subject, error) {
	tok := bearerToken(r.Header.Get("Authorization"))
	claims, err := h.inspector.InspectAccessToken(r.Context(), tok)
	if err != nil {
		return disclosureapp.Subject{}, err
	}
	return disclosureapp.Subject{UserID: claims.Sub, MembershipID: claims.MembershipID, CompanyID: claims.CompanyID}, nil
}

func disclosureRequestHash(companyID, recordID, userID, op string) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%s", companyID, recordID, userID, op)))
	return hex.EncodeToString(h[:])
}

func idempotencyConflictBody(msg string) map[string]any {
	return map[string]any{
		"error": map[string]any{
			"code":    "IDEMPOTENCY_CONFLICT",
			"message": msg,
		},
	}
}

func bearerToken(h string) string {
	h = strings.TrimSpace(h)
	if h == "" {
		return ""
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
		return strings.TrimSpace(parts[1])
	}
	return h
}

func (h *Handler) listCompanyGroups(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	departmentID := strings.TrimSpace(r.URL.Query().Get("department_id"))
	var isActive *bool
	if raw := strings.TrimSpace(r.URL.Query().Get("is_active")); raw != "" {
		v := raw == "true"
		isActive = &v
	}
	resp, err := h.svc.ListCompanyGroups(r.Context(), disclosureapp.ListCompanyGroupsRequest{
		Subject:      sub,
		DepartmentID: departmentID,
		IsActive:     isActive,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) updateWorkflowOverrideStepGroups(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	typeID := strings.TrimSpace(r.PathValue("type_id"))
	stepID := strings.TrimSpace(r.PathValue("step_id"))
	var body struct {
		BaseEtag string                                      `json:"base_etag"`
		Groups   []disclosureapp.WorkflowStepGroupWriteInput `json:"groups"`
		ClearAll bool                                        `json:"clear_all_groups"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	resp, err := h.svc.UpdateWorkflowOverrideStepGroups(r.Context(), disclosureapp.UpdateWorkflowOverrideStepGroupsRequest{
		Subject:  sub,
		TypeID:   typeID,
		StepID:   stepID,
		BaseEtag: body.BaseEtag,
		Groups:   body.Groups,
		ClearAll: body.ClearAll,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, sub, "disclosure.workflow_override.step_groups_updated", "disclosure_type", typeID, map[string]any{
		"step_id":     stepID,
		"group_count": len(resp.Groups),
	})
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) getCompanyTypePreference(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	typeID := strings.TrimSpace(r.PathValue("type_id"))
	resp, err := h.svc.GetCompanyTypePreference(r.Context(), disclosureapp.GetCompanyTypePreferenceRequest{
		Subject: sub,
		TypeID:  typeID,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) upsertCompanyTypePreference(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	typeID := strings.TrimSpace(r.PathValue("type_id"))
	var body struct {
		AutoCreateEnabled bool `json:"auto_create_enabled"`
		CycleAnchorMonth  int  `json:"cycle_anchor_month,omitempty"`
		CycleAnchorDay    int  `json:"cycle_anchor_day,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	resp, err := h.svc.UpsertCompanyTypePreference(r.Context(), disclosureapp.UpsertCompanyTypePreferenceRequest{
		Subject:           sub,
		TypeID:            typeID,
		AutoCreateEnabled: body.AutoCreateEnabled,
		CycleAnchorMonth:  body.CycleAnchorMonth,
		CycleAnchorDay:    body.CycleAnchorDay,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

// BE-004A handlers.

func (h *Handler) createCompanyTemplate(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var body disclosureapp.CreateCompanyTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	body.Subject = sub
	resp, err := h.svc.CreateCompanyTemplate(r.Context(), body)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, sub, "disclosure.company_template.created", "disclosure_type", resp.TypeID, map[string]any{
		"template_category": resp.TemplateCategory,
		"review_status":     resp.ReviewStatus,
		"change_note":       strings.TrimSpace(body.ChangeNote),
	})
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"data": resp})
}

func (h *Handler) updateCompanyTemplate(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	typeID := strings.TrimSpace(r.PathValue("type_id"))
	var body disclosureapp.UpdateCompanyTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	body.Subject = sub
	body.TypeID = typeID
	resp, err := h.svc.UpdateCompanyTemplate(r.Context(), body)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, sub, "disclosure.company_template.updated", "disclosure_type", resp.TypeID, map[string]any{
		"template_category": resp.TemplateCategory,
		"review_status":     resp.ReviewStatus,
		"change_note":       strings.TrimSpace(body.ChangeNote),
	})
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": resp})
}

// BE-004B handler.

func (h *Handler) transitionCompanyTemplateLifecycle(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	typeID := strings.TrimSpace(r.PathValue("type_id"))
	action := strings.TrimSpace(r.PathValue("action"))
	var body struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	resp, err := h.svc.TransitionCompanyTemplateLifecycle(r.Context(), disclosureapp.TransitionCompanyTemplateLifecycleRequest{
		Subject: sub,
		TypeID:  typeID,
		Action:  action,
		Reason:  body.Reason,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.auditLog(r, sub, "disclosure.company_template.lifecycle_transition", "disclosure_type", typeID, map[string]any{
		"action":        action,
		"reason":        strings.TrimSpace(body.Reason),
		"review_status": resp.ReviewStatus,
	})
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": resp})
}
