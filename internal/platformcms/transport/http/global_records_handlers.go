package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	platformcmsapp "github.com/cobo/cobo_iam_services/internal/platformcms/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
)

func (h *Handler) globalRecordsSvcOrErr(w http.ResponseWriter) *platformcmsapp.Service {
	if h.globalRecordsSvc == nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeServiceUnavailable, "global cms records service unavailable", nil))
		return nil
	}
	return h.globalRecordsSvc
}

func (h *Handler) listTemplateGlobalRecords(w http.ResponseWriter, r *http.Request) {
	svc := h.globalRecordsSvcOrErr(w)
	if svc == nil {
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
	templateID := strings.TrimSpace(r.PathValue("template_id"))
	if templateID == "" {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "template_id is required", nil))
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	res, err := svc.ListByTemplate(r.Context(), platformcmsapp.Actor{
		UserID: sub.Sub, MembershipID: sub.MembershipID, CompanyID: sub.CompanyID,
	}, platformcmsapp.ListGlobalRecordsFilter{
		TemplateID: templateID,
		Status:     strings.TrimSpace(r.URL.Query().Get("status")),
		CycleKey:   strings.TrimSpace(r.URL.Query().Get("cycle_key")),
		Query:      strings.TrimSpace(r.URL.Query().Get("q")),
		Page:       page,
		PageSize:   pageSize,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	writeEnvelope(w, http.StatusOK, map[string]any{"items": res.Items}, map[string]any{
		"page": res.Page, "page_size": res.PageSize, "total": res.Total,
	})
}

func (h *Handler) createTemplateGlobalRecord(w http.ResponseWriter, r *http.Request) {
	svc := h.globalRecordsSvcOrErr(w)
	if svc == nil {
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
	templateID := strings.TrimSpace(r.PathValue("template_id"))
	if templateID == "" {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "template_id is required", nil))
		return
	}
	var body struct {
		Title    string `json:"title"`
		Summary  string `json:"summary"`
		Content  string `json:"content"`
		CycleKey string `json:"cycle_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid JSON payload", err))
		return
	}
	actor := platformcmsapp.Actor{UserID: sub.Sub, MembershipID: sub.MembershipID, CompanyID: sub.CompanyID}
	rec, err := svc.Create(r.Context(), actor, templateID, body.Title, body.Summary, body.Content, body.CycleKey)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.appendGlobalRecordAudit(r, sub, platformcmsapp.AuditCreate, rec.ID, map[string]any{
		"template_id": rec.TemplateID, "cycle_key": rec.CycleKey, "status": rec.Status,
	})
	writeEnvelope(w, http.StatusCreated, rec, nil)
}

func (h *Handler) getGlobalRecord(w http.ResponseWriter, r *http.Request) {
	svc := h.globalRecordsSvcOrErr(w)
	if svc == nil {
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
	recordID := strings.TrimSpace(r.PathValue("record_id"))
	rec, err := svc.Get(r.Context(), platformcmsapp.Actor{
		UserID: sub.Sub, MembershipID: sub.MembershipID, CompanyID: sub.CompanyID,
	}, recordID)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	writeEnvelope(w, http.StatusOK, rec, nil)
}

func (h *Handler) updateGlobalRecord(w http.ResponseWriter, r *http.Request) {
	svc := h.globalRecordsSvcOrErr(w)
	if svc == nil {
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
	recordID := strings.TrimSpace(r.PathValue("record_id"))
	var body struct {
		Title   string `json:"title"`
		Summary string `json:"summary"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid JSON payload", err))
		return
	}
	actor := platformcmsapp.Actor{UserID: sub.Sub, MembershipID: sub.MembershipID, CompanyID: sub.CompanyID}
	rec, err := svc.Update(r.Context(), actor, recordID, body.Title, body.Summary, body.Content)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.appendGlobalRecordAudit(r, sub, platformcmsapp.AuditUpdate, rec.ID, map[string]any{
		"template_id": rec.TemplateID, "cycle_key": rec.CycleKey, "status": rec.Status,
	})
	writeEnvelope(w, http.StatusOK, rec, nil)
}

func (h *Handler) publishGlobalRecord(w http.ResponseWriter, r *http.Request) {
	svc := h.globalRecordsSvcOrErr(w)
	if svc == nil {
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
	recordID := strings.TrimSpace(r.PathValue("record_id"))
	actor := platformcmsapp.Actor{UserID: sub.Sub, MembershipID: sub.MembershipID, CompanyID: sub.CompanyID}
	beforeStatus := ""
	if before, gErr := svc.Get(r.Context(), actor, recordID); gErr == nil && before != nil {
		beforeStatus = before.Status
	}
	rec, err := svc.Publish(r.Context(), actor, recordID)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.appendGlobalRecordAudit(r, sub, platformcmsapp.AuditPublish, rec.ID, map[string]any{
		"template_id": rec.TemplateID, "cycle_key": displayCycleKey(rec.CycleKey),
		"before_status": beforeStatus, "after_status": rec.Status,
		"template_version_no": rec.TemplateVersionNo,
	})
	writeEnvelope(w, http.StatusOK, rec, nil)
}

func (h *Handler) archiveGlobalRecord(w http.ResponseWriter, r *http.Request) {
	svc := h.globalRecordsSvcOrErr(w)
	if svc == nil {
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
	recordID := strings.TrimSpace(r.PathValue("record_id"))
	actor := platformcmsapp.Actor{UserID: sub.Sub, MembershipID: sub.MembershipID, CompanyID: sub.CompanyID}
	beforeStatus := ""
	if before, gErr := svc.Get(r.Context(), actor, recordID); gErr == nil && before != nil {
		beforeStatus = before.Status
	}
	rec, err := svc.Archive(r.Context(), actor, recordID)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	h.appendGlobalRecordAudit(r, sub, platformcmsapp.AuditArchive, rec.ID, map[string]any{
		"template_id": rec.TemplateID, "cycle_key": displayCycleKey(rec.CycleKey),
		"before_status": beforeStatus, "after_status": rec.Status,
	})
	writeEnvelope(w, http.StatusOK, rec, nil)
}

func (h *Handler) listGlobalRecordCompanyRecords(w http.ResponseWriter, r *http.Request) {
	svc := h.globalRecordsSvcOrErr(w)
	if svc == nil {
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
	recordID := strings.TrimSpace(r.PathValue("record_id"))
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	items, total, err := svc.ListCompanyChildren(r.Context(), platformcmsapp.Actor{
		UserID: sub.Sub, MembershipID: sub.MembershipID, CompanyID: sub.CompanyID,
	}, recordID, status, page, pageSize)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	writeEnvelope(w, http.StatusOK, map[string]any{"items": items}, map[string]any{
		"page": pageOrDefault(page, 1), "page_size": pageOrDefault(pageSize, 20), "total": total,
	})
}

func (h *Handler) materializeGlobalRecord(w http.ResponseWriter, r *http.Request) {
	svc := h.globalRecordsSvcOrErr(w)
	if svc == nil {
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
	recordID := strings.TrimSpace(r.PathValue("record_id"))
	var body platformcmsapp.MaterializeRequest
	decErr := json.NewDecoder(r.Body).Decode(&body)
	if decErr != nil && !errors.Is(decErr, io.EOF) {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid JSON payload", decErr))
		return
	}
	actor := platformcmsapp.Actor{UserID: sub.Sub, MembershipID: sub.MembershipID, CompanyID: sub.CompanyID}
	res, err := svc.Materialize(r.Context(), actor, recordID, body)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	action := platformcmsapp.AuditMatRun
	if body.DryRun {
		action = platformcmsapp.AuditMatPreview
	}
	h.appendGlobalRecordAudit(r, sub, action, recordID, map[string]any{
		"run_id": res.RunID, "mode": res.Mode, "dry_run": res.DryRun,
		"requested": res.Requested, "created": res.Created, "exists": res.Exists,
		"skipped": res.Skipped, "failed": res.Failed,
	})
	writeEnvelope(w, http.StatusOK, res, nil)
}

func (h *Handler) appendGlobalRecordAudit(r *http.Request, sub iamapp.AccessTokenClaims, action, resourceID string, metadata map[string]any) {
	if h.auditSvc == nil {
		return
	}
	_ = h.auditSvc.AppendAuditLog(r.Context(), auditapp.AppendAuditLogRequest{
		ActorUserID:       sub.Sub,
		ActorMembershipID: sub.MembershipID,
		CompanyID:         sub.CompanyID,
		Action:            action,
		ResourceType:      "cms_global_record",
		ResourceID:        resourceID,
		Decision:          "allow",
		Metadata:          metadata,
	})
}

func displayCycleKey(cycleKey string) string {
	if i := strings.Index(cycleKey, "#archived:"); i >= 0 {
		return cycleKey[:i]
	}
	return cycleKey
}

func pageOrDefault(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}
