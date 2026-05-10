package http

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	companyaccessapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	holidayapp "github.com/cobo/cobo_iam_services/internal/holiday/app"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
)

type Handler struct {
	inspector          iamapp.TokenInspector
	authorizer         authapp.Service
	adminSvc           companyaccessapp.AdminService
	iamSvc             iamapp.Service
	auditSvc           auditapp.Service
	auditRepo          auditapp.Repository
	disclosureSvc      disclosureapp.Service
	disclosures        disclosureapp.Repository
	holidaySvc         holidayapp.Service
	mediaRepo          cmsMediaRepository
	mediaSigner        *cmsMediaSigner
	mediaStorage       *cmsMediaDiskStorage
	mediaPublicBaseURL string
	metrics            *cmsMetrics
}

type MediaOptions struct {
	DB                  *sql.DB
	UploadSigningSecret string
	UploadURLTTL        time.Duration
	StorageDir          string
	PublicAPIBaseURL    string
}

func NewHandler(inspector iamapp.TokenInspector, authorizer authapp.Service, adminSvc companyaccessapp.AdminService, iamSvc iamapp.Service, auditSvc auditapp.Service, auditRepo auditapp.Repository, disclosureSvc disclosureapp.Service, disclosures disclosureapp.Repository, holidaySvc holidayapp.Service, mediaOpts MediaOptions) *Handler {
	mediaStorage, err := newCMSMediaDiskStorage(mediaOpts.StorageDir)
	if err != nil {
		panic(err)
	}
	return &Handler{
		inspector:          inspector,
		authorizer:         authorizer,
		adminSvc:           adminSvc,
		iamSvc:             iamSvc,
		auditSvc:           auditSvc,
		auditRepo:          auditRepo,
		disclosureSvc:      disclosureSvc,
		disclosures:        disclosures,
		holidaySvc:         holidaySvc,
		mediaRepo:          newCMSMediaRepository(mediaOpts.DB),
		mediaSigner:        newCMSMediaSigner(mediaOpts.UploadSigningSecret, mediaOpts.UploadURLTTL),
		mediaStorage:       mediaStorage,
		mediaPublicBaseURL: strings.TrimSpace(mediaOpts.PublicAPIBaseURL),
		metrics:            newCMSMetrics(),
	}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/platform/cms/dashboard/summary", h.observe("cms.dashboard.summary", h.dashboardSummary))
	mux.HandleFunc("GET /api/v1/platform/cms/collections", h.observe("cms.collections.list", h.collections))
	mux.HandleFunc("GET /api/v1/platform/cms/collections/{collection_id}", h.observe("cms.collections.detail", h.collectionDetail))
	mux.HandleFunc("GET /api/v1/platform/cms/entries", h.observe("cms.entries.list", h.entries))
	mux.HandleFunc("GET /api/v1/platform/cms/entries/{entry_id}", h.observe("cms.entries.detail", h.entryDetail))
	mux.HandleFunc("POST /api/v1/platform/cms/entries", h.observe("cms.entries.create", h.createEntry))
	mux.HandleFunc("PUT /api/v1/platform/cms/entries/{entry_id}", h.observe("cms.entries.update", h.updateEntry))
	mux.HandleFunc("POST /api/v1/platform/cms/media/upload", h.observe("cms.media.upload.intent", h.createMediaUploadIntent))
	mux.HandleFunc("PUT /api/v1/platform/cms/media/upload/{asset_id}", h.observe("cms.media.upload.binary", h.uploadMediaBinary))
	mux.HandleFunc("POST /api/v1/platform/cms/media/{asset_id}/complete", h.observe("cms.media.upload.complete", h.completeMediaUpload))
	mux.HandleFunc("GET /api/v1/platform/cms/media", h.observe("cms.media.list", h.listMedia))
	mux.HandleFunc("DELETE /api/v1/platform/cms/media/{asset_id}", h.observe("cms.media.delete", h.deleteMedia))
	mux.HandleFunc("GET /api/v1/platform/cms/reviews", h.observe("cms.reviews.list", h.reviews))
	mux.HandleFunc("POST /api/v1/platform/cms/reviews/{entry_id}", h.observe("cms.reviews.action", h.reviewAction))
	mux.HandleFunc("GET /api/v1/platform/cms/schedules", h.observe("cms.schedules.list", h.schedules))
	mux.HandleFunc("POST /api/v1/platform/cms/schedules", h.observe("cms.schedules.create", h.createSchedule))
	mux.HandleFunc("DELETE /api/v1/platform/cms/schedules/{entry_id}", h.observe("cms.schedules.delete", h.deleteSchedule))
	mux.HandleFunc("GET /api/v1/platform/cms/releases", h.observe("cms.releases.list", h.releases))
	mux.HandleFunc("GET /api/v1/platform/cms/admin/users", h.observe("cms.admin.users.list", h.adminUsers))
	mux.HandleFunc("GET /api/v1/platform/cms/admin/companies", h.observe("cms.admin.companies.list", h.listCMSCompanies))
	mux.HandleFunc("GET /api/v1/platform/cms/admin/companies/{company_id}", h.observe("cms.admin.companies.get", h.getCMSCompany))
	mux.HandleFunc("PATCH /api/v1/platform/cms/admin/companies/{company_id}", h.observe("cms.admin.companies.patch", h.patchCMSCompany))
	mux.HandleFunc("POST /api/v1/platform/cms/admin/companies/{company_id}/deactivate", h.observe("cms.admin.companies.deactivate", h.postCMSCompanyDeactivate))
	mux.HandleFunc("POST /api/v1/platform/cms/admin/companies/{company_id}/activate", h.observe("cms.admin.companies.activate", h.postCMSCompanyActivate))
	mux.HandleFunc("POST /api/v1/platform/cms/admin/companies", h.observe("cms.admin.companies.create", h.createCMSCompany))
	mux.HandleFunc("POST /api/v1/platform/cms/admin/users", h.observe("cms.admin.users.create", h.createAdminUser))
	mux.HandleFunc("POST /api/v1/platform/cms/admin/users/invite", h.observe("cms.admin.users.invite", h.inviteAdminUser))
	mux.HandleFunc("POST /api/v1/platform/cms/admin/users/{user_id}/resend-invitation", h.observe("cms.admin.users.invite.resend", h.resendInvitation))
	mux.HandleFunc("POST /api/v1/platform/cms/admin/users/{user_id}/request-password-reset", h.observe("cms.admin.users.password_reset", h.requestAdminPasswordReset))
	mux.HandleFunc("GET /api/v1/platform/cms/admin/roles", h.observe("cms.admin.roles.list", h.roles))
	mux.HandleFunc("GET /api/v1/platform/cms/admin/rules", h.observe("cms.admin.rules.list", h.rules))
	mux.HandleFunc("POST /api/v1/platform/cms/admin/rules/validate", h.observe("cms.admin.rules.validate", h.validateRule))
	mux.HandleFunc("GET /api/v1/platform/cms/ops/audit", h.observe("cms.ops.audit.list", h.auditLogs))
	mux.HandleFunc("GET /api/v1/platform/cms/ops/sessions", h.observe("cms.ops.sessions.list", h.sessions))
	mux.HandleFunc("POST /api/v1/platform/cms/ops/sessions/{session_id}/revoke", h.observe("cms.ops.sessions.revoke", h.revokeSession))
	mux.HandleFunc("GET /api/v1/platform/cms/ops/health", h.observe("cms.ops.health", h.systemHealth))
	mux.HandleFunc("GET /api/v1/platform/cms/ops/metrics", h.observe("cms.ops.metrics", h.systemMetrics))
	if h.holidaySvc != nil {
		mux.HandleFunc("GET /api/v1/platform/cms/holiday-calendars/{year}", h.observe("cms.holiday.get", h.holidayCalendarGet))
		mux.HandleFunc("POST /api/v1/platform/cms/holiday-calendars/{year}/preview", h.observe("cms.holiday.preview", h.holidayCalendarPreview))
		mux.HandleFunc("PUT /api/v1/platform/cms/holiday-calendars/{year}", h.observe("cms.holiday.replace", h.holidayCalendarReplace))
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (h *Handler) observe(route string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next(rec, r)
		requestID := httpx.RequestIDFromContext(r.Context())
		h.metrics.record(route, requestID, rec.status, time.Since(start))
	}
}

func (h *Handler) dashboardSummary(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireCMSAccess(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	items, err := h.disclosures.List(r.Context(), sub.CompanyID)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var draft, published, completed int
	for _, item := range items {
		switch strings.ToLower(strings.TrimSpace(item.Status)) {
		case "draft":
			draft++
		case "published", "submitted":
			published++
		case "completed", "confirmed":
			completed++
		}
	}
	writeEnvelope(w, http.StatusOK, map[string]any{
		"total":        len(items),
		"draft":        draft,
		"published":    published,
		"completed":    completed,
		"platform_cms": true,
	}, nil)
}

func (h *Handler) collections(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireCMSAccess(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireAnyPermission(r.Context(), sub.MembershipID, sub.CompanyID, "disclosure.view", "disclosure.create", "disclosure.edit", "rbac.manage", "system.settings"); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	items, err := h.disclosures.List(r.Context(), sub.CompanyID)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	type collection struct {
		CollectionID    string `json:"collection_id"`
		Name            string `json:"name"`
		EntryCount      int    `json:"entry_count"`
		LatestUpdatedAt string `json:"latest_updated_at,omitempty"`
	}
	agg := map[string]*collection{}
	for _, item := range items {
		key := strings.TrimSpace(item.TypeID)
		if key == "" {
			key = "general"
		}
		cur := agg[key]
		if cur == nil {
			cur = &collection{CollectionID: key, Name: key}
			agg[key] = cur
		}
		cur.EntryCount++
		updated := item.UpdatedAt.UTC().Format(timeLayout)
		if cur.LatestUpdatedAt == "" || updated > cur.LatestUpdatedAt {
			cur.LatestUpdatedAt = updated
		}
	}
	out := make([]collection, 0, len(agg))
	for _, item := range agg {
		out = append(out, *item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].EntryCount == out[j].EntryCount {
			return out[i].CollectionID < out[j].CollectionID
		}
		return out[i].EntryCount > out[j].EntryCount
	})
	writeEnvelope(w, http.StatusOK, map[string]any{"items": out}, map[string]any{"total": len(out)})
}

func (h *Handler) collectionDetail(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireCMSAccess(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireAnyPermission(r.Context(), sub.MembershipID, sub.CompanyID, "disclosure.view", "disclosure.create", "disclosure.edit", "rbac.manage", "system.settings"); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	collectionID := strings.TrimSpace(r.PathValue("collection_id"))
	if collectionID == "" {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "collection_id is required", nil))
		return
	}
	items, err := h.disclosures.List(r.Context(), sub.CompanyID)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	filtered := make([]map[string]any, 0)
	for _, item := range items {
		typeID := strings.TrimSpace(item.TypeID)
		if typeID == "" {
			typeID = "general"
		}
		if !strings.EqualFold(typeID, collectionID) {
			continue
		}
		filtered = append(filtered, map[string]any{
			"entry_id":   item.RecordID,
			"title":      item.Title,
			"status":     item.Status,
			"type_id":    typeID,
			"updated_at": item.UpdatedAt.UTC().Format(timeLayout),
		})
	}
	writeEnvelope(w, http.StatusOK, map[string]any{
		"collection_id": collectionID,
		"name":          collectionID,
		"items":         filtered,
	}, map[string]any{"total": len(filtered)})
}

func (h *Handler) entries(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireCMSAccess(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireAnyPermission(r.Context(), sub.MembershipID, sub.CompanyID, "disclosure.view", "disclosure.create", "disclosure.edit", "rbac.manage", "system.settings"); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	items, err := h.disclosures.List(r.Context(), sub.CompanyID)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		typeID := strings.TrimSpace(item.TypeID)
		if typeID == "" {
			typeID = "general"
		}
		out = append(out, map[string]any{
			"entry_id":   item.RecordID,
			"title":      item.Title,
			"status":     item.Status,
			"type_id":    typeID,
			"updated_at": item.UpdatedAt.UTC().Format(timeLayout),
			"company_id": item.CompanyID,
			"record_id":  item.RecordID,
		})
	}
	writeEnvelope(w, http.StatusOK, map[string]any{"items": out}, map[string]any{"total": len(out)})
}

func (h *Handler) entryDetail(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireCMSAccess(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireAnyPermission(r.Context(), sub.MembershipID, sub.CompanyID, "disclosure.view", "disclosure.create", "disclosure.edit", "rbac.manage", "system.settings"); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	entryID := strings.TrimSpace(r.PathValue("entry_id"))
	if entryID == "" {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "entry_id is required", nil))
		return
	}
	rec, err := h.disclosures.FindByID(r.Context(), sub.CompanyID, entryID)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	writeEnvelope(w, http.StatusOK, map[string]any{
		"entry_id":       rec.RecordID,
		"title":          rec.Title,
		"summary":        rec.Summary,
		"content":        rec.Content,
		"status":         rec.Status,
		"type_id":        rec.TypeID,
		"planned_date":   rec.PlannedDate,
		"published_date": rec.PublishedDate,
		"updated_at":     rec.UpdatedAt.UTC().Format(timeLayout),
	}, nil)
}

func (h *Handler) createEntry(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireCMSAccess(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var payload disclosureapp.RecordPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid JSON payload", err))
		return
	}
	rec, err := h.disclosureSvc.CreateRecord(r.Context(), disclosureapp.CreateRecordRequest{
		Subject: disclosureapp.Subject{
			UserID:       sub.Sub,
			MembershipID: sub.MembershipID,
			CompanyID:    sub.CompanyID,
		},
		Payload: payload,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	_ = h.auditSvc.AppendAuditLog(r.Context(), auditapp.AppendAuditLogRequest{
		ActorUserID:       sub.Sub,
		ActorMembershipID: sub.MembershipID,
		CompanyID:         sub.CompanyID,
		Action:            cmsActionEntryCreate,
		ResourceType:      "disclosure_record",
		ResourceID:        rec.RecordID,
		Decision:          "allow",
		Metadata: map[string]any{
			"type_id": rec.TypeID,
			"title":   rec.Title,
		},
	})
	writeEnvelope(w, http.StatusCreated, map[string]any{
		"entry_id":   rec.RecordID,
		"record_id":  rec.RecordID,
		"title":      rec.Title,
		"status":     rec.Status,
		"type_id":    rec.TypeID,
		"updated_at": rec.UpdatedAt.UTC().Format(timeLayout),
	}, nil)
}

func (h *Handler) updateEntry(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireCMSAccess(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	entryID := strings.TrimSpace(r.PathValue("entry_id"))
	if entryID == "" {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "entry_id is required", nil))
		return
	}
	var payload disclosureapp.RecordPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid JSON payload", err))
		return
	}
	rec, err := h.disclosureSvc.UpdateRecord(r.Context(), disclosureapp.UpdateRecordRequest{
		Subject: disclosureapp.Subject{
			UserID:       sub.Sub,
			MembershipID: sub.MembershipID,
			CompanyID:    sub.CompanyID,
		},
		RecordID: entryID,
		Payload:  payload,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	_ = h.auditSvc.AppendAuditLog(r.Context(), auditapp.AppendAuditLogRequest{
		ActorUserID:       sub.Sub,
		ActorMembershipID: sub.MembershipID,
		CompanyID:         sub.CompanyID,
		Action:            cmsActionEntryUpdate,
		ResourceType:      "disclosure_record",
		ResourceID:        rec.RecordID,
		Decision:          "allow",
		Metadata: map[string]any{
			"type_id": rec.TypeID,
			"title":   rec.Title,
		},
	})
	writeEnvelope(w, http.StatusOK, map[string]any{
		"entry_id":   rec.RecordID,
		"record_id":  rec.RecordID,
		"title":      rec.Title,
		"status":     rec.Status,
		"type_id":    rec.TypeID,
		"updated_at": rec.UpdatedAt.UTC().Format(timeLayout),
	}, nil)
}

func (h *Handler) reviews(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireCMSAccess(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireAnyPermission(r.Context(), sub.MembershipID, sub.CompanyID, "disclosure.approve", "workflow.step.confirm", "rbac.manage", "system.settings"); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	items, err := h.disclosures.List(r.Context(), sub.CompanyID)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	out := make([]map[string]any, 0)
	for _, item := range items {
		state := strings.ToLower(strings.TrimSpace(item.Status))
		if state != "published" && state != "submitted" {
			continue
		}
		out = append(out, map[string]any{
			"entry_id":   item.RecordID,
			"title":      item.Title,
			"status":     item.Status,
			"type_id":    item.TypeID,
			"updated_at": item.UpdatedAt.UTC().Format(timeLayout),
		})
	}
	writeEnvelope(w, http.StatusOK, map[string]any{"items": out}, map[string]any{"total": len(out)})
}

func (h *Handler) reviewAction(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireCMSAccess(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	entryID := strings.TrimSpace(r.PathValue("entry_id"))
	if entryID == "" {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "entry_id is required", nil))
		return
	}
	var payload struct {
		Decision string `json:"decision"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid JSON payload", err))
		return
	}
	decision := strings.ToLower(strings.TrimSpace(payload.Decision))
	if decision == "" {
		decision = "approve"
	}
	switch decision {
	case "approve":
		rec, err := h.disclosureSvc.ConfirmRecord(r.Context(), disclosureapp.ConfirmRecordRequest{
			Subject: disclosureapp.Subject{
				UserID:       sub.Sub,
				MembershipID: sub.MembershipID,
				CompanyID:    sub.CompanyID,
			},
			RecordID: entryID,
		})
		if err != nil {
			httpx.WriteError(w, nil, err)
			return
		}
		_ = h.auditSvc.AppendAuditLog(r.Context(), auditapp.AppendAuditLogRequest{
			ActorUserID:       sub.Sub,
			ActorMembershipID: sub.MembershipID,
			CompanyID:         sub.CompanyID,
			Action:            cmsActionReviewApprove,
			ResourceType:      "disclosure_record",
			ResourceID:        rec.RecordID,
			Decision:          "allow",
		})
		writeEnvelope(w, http.StatusOK, map[string]any{
			"entry_id": rec.RecordID,
			"status":   rec.Status,
			"decision": "approve",
		}, nil)
	case "reject":
		rec, err := h.disclosures.FindByID(r.Context(), sub.CompanyID, entryID)
		if err != nil {
			httpx.WriteError(w, nil, err)
			return
		}
		if _, err := h.requireAnyPermission(r.Context(), sub.MembershipID, sub.CompanyID, "disclosure.approve", "workflow.step.confirm", "rbac.manage", "system.settings"); err != nil {
			httpx.WriteError(w, nil, err)
			return
		}
		rec.Status = "Draft"
		rec.UpdatedBy = sub.Sub
		next, err := h.disclosures.Update(r.Context(), *rec)
		if err != nil {
			httpx.WriteError(w, nil, err)
			return
		}
		_ = h.auditSvc.AppendAuditLog(r.Context(), auditapp.AppendAuditLogRequest{
			ActorUserID:       sub.Sub,
			ActorMembershipID: sub.MembershipID,
			CompanyID:         sub.CompanyID,
			Action:            cmsActionReviewReject,
			ResourceType:      "disclosure_record",
			ResourceID:        next.RecordID,
			Decision:          "allow",
		})
		writeEnvelope(w, http.StatusOK, map[string]any{
			"entry_id": next.RecordID,
			"status":   next.Status,
			"decision": "reject",
		}, nil)
	default:
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "decision must be approve or reject", nil))
	}
}

func (h *Handler) schedules(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireCMSAccess(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireAnyPermission(r.Context(), sub.MembershipID, sub.CompanyID, "disclosure.publish", "rbac.manage", "system.settings"); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	items, err := h.disclosures.List(r.Context(), sub.CompanyID)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	out := make([]map[string]any, 0)
	for _, item := range items {
		if strings.TrimSpace(item.PlannedDate) == "" {
			continue
		}
		out = append(out, map[string]any{
			"entry_id":   item.RecordID,
			"publish_at": item.PlannedDate,
			"status":     item.Status,
			"title":      item.Title,
			"updated_at": item.UpdatedAt.UTC().Format(timeLayout),
		})
	}
	writeEnvelope(w, http.StatusOK, map[string]any{"items": out}, map[string]any{"total": len(out)})
}

func (h *Handler) createSchedule(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireCMSAccess(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireAnyPermission(r.Context(), sub.MembershipID, sub.CompanyID, "disclosure.publish", "rbac.manage", "system.settings"); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var payload struct {
		EntryID   string `json:"entry_id"`
		PublishAt string `json:"publish_at"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid JSON payload", err))
		return
	}
	payload.EntryID = strings.TrimSpace(payload.EntryID)
	payload.PublishAt = strings.TrimSpace(payload.PublishAt)
	if payload.EntryID == "" || payload.PublishAt == "" {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "entry_id and publish_at are required", nil))
		return
	}
	if _, err := time.Parse("2006-01-02", payload.PublishAt); err != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "publish_at must be YYYY-MM-DD", err))
		return
	}
	rec, err := h.disclosures.FindByID(r.Context(), sub.CompanyID, payload.EntryID)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	rec.PlannedDate = payload.PublishAt
	rec.UpdatedBy = sub.Sub
	next, err := h.disclosures.Update(r.Context(), *rec)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	_ = h.auditSvc.AppendAuditLog(r.Context(), auditapp.AppendAuditLogRequest{
		ActorUserID:       sub.Sub,
		ActorMembershipID: sub.MembershipID,
		CompanyID:         sub.CompanyID,
		Action:            cmsActionScheduleCreate,
		ResourceType:      "disclosure_record",
		ResourceID:        next.RecordID,
		Decision:          "allow",
		Metadata: map[string]any{
			"publish_at": next.PlannedDate,
		},
	})
	writeEnvelope(w, http.StatusCreated, map[string]any{
		"entry_id":   next.RecordID,
		"publish_at": next.PlannedDate,
		"status":     next.Status,
	}, nil)
}

func (h *Handler) deleteSchedule(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireCMSAccess(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireAnyPermission(r.Context(), sub.MembershipID, sub.CompanyID, "disclosure.publish", "rbac.manage", "system.settings"); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	entryID := strings.TrimSpace(r.PathValue("entry_id"))
	if entryID == "" {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "entry_id is required", nil))
		return
	}
	rec, err := h.disclosures.FindByID(r.Context(), sub.CompanyID, entryID)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	rec.PlannedDate = ""
	rec.UpdatedBy = sub.Sub
	next, err := h.disclosures.Update(r.Context(), *rec)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	_ = h.auditSvc.AppendAuditLog(r.Context(), auditapp.AppendAuditLogRequest{
		ActorUserID:       sub.Sub,
		ActorMembershipID: sub.MembershipID,
		CompanyID:         sub.CompanyID,
		Action:            cmsActionScheduleDelete,
		ResourceType:      "disclosure_record",
		ResourceID:        next.RecordID,
		Decision:          "allow",
	})
	writeEnvelope(w, http.StatusOK, map[string]any{
		"entry_id":   next.RecordID,
		"publish_at": next.PlannedDate,
		"status":     next.Status,
	}, nil)
}

func (h *Handler) releases(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireCMSAccess(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireAnyPermission(r.Context(), sub.MembershipID, sub.CompanyID, "disclosure.publish", "rbac.manage", "system.settings"); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}

	entryIDFilter := strings.TrimSpace(r.URL.Query().Get("entry_id"))
	items, err := h.disclosures.List(r.Context(), sub.CompanyID)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}

	out := make([]map[string]any, 0)
	for _, item := range items {
		state := strings.ToLower(strings.TrimSpace(item.Status))
		if state != "published" && state != "completed" && state != "confirmed" {
			continue
		}
		if entryIDFilter != "" && !strings.EqualFold(entryIDFilter, item.RecordID) {
			continue
		}
		out = append(out, map[string]any{
			"entry_id":       item.RecordID,
			"title":          item.Title,
			"status":         item.Status,
			"type_id":        item.TypeID,
			"published_date": strings.TrimSpace(item.PublishedDate),
			"updated_at":     item.UpdatedAt.UTC().Format(timeLayout),
		})
	}

	sort.Slice(out, func(i, j int) bool {
		left := strings.TrimSpace(anyToString(out[i]["published_date"]))
		right := strings.TrimSpace(anyToString(out[j]["published_date"]))
		if left == right {
			return anyToString(out[i]["entry_id"]) < anyToString(out[j]["entry_id"])
		}
		return left > right
	})
	writeEnvelope(w, http.StatusOK, map[string]any{"items": out}, map[string]any{"total": len(out)})
}

func anyToString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func (h *Handler) roles(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireCMSAccess(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}

	targetCompany := strings.TrimSpace(r.URL.Query().Get("company_id"))
	if targetCompany != "" {
		// Assignable roles from IAM `roles` table for invite/update-membership — must match invite permission level.
		if _, err := h.requireAnyPermission(r.Context(), sub.MembershipID, sub.CompanyID, "rbac.manage", "system.settings"); err != nil {
			httpx.WriteError(w, nil, err)
			return
		}
		opts, err := h.adminSvc.ListInviteRoles(r.Context(), companyaccessapp.ListInviteRolesRequest{
			Subject: companyaccessapp.AdminSubject{
				UserID:       sub.Sub,
				MembershipID: sub.MembershipID,
				CompanyID:    sub.CompanyID,
			},
			CompanyID: targetCompany,
		})
		if err != nil {
			httpx.WriteError(w, nil, err)
			return
		}
		roleItems := make([]map[string]any, 0, len(opts))
		for _, o := range opts {
			roleItems = append(roleItems, map[string]any{
				"role_id":   o.RoleID,
				"role_code": o.RoleCode,
				"role_name": o.RoleName,
			})
		}
		writeEnvelope(w, http.StatusOK, map[string]any{
			"items":       roleItems,
			"permissions": []string{},
		}, map[string]any{"total": len(roleItems)})
		return
	}

	if _, err := h.requireAnyPermission(r.Context(), sub.MembershipID, sub.CompanyID, "rbac.manage", "system.settings", "platform.cms.view"); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	eff, err := h.authorizer.GetEffectiveAccess(r.Context(), sub.MembershipID, sub.CompanyID)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	roleItems := make([]map[string]any, 0, len(eff.Responsibilities))
	for _, roleCode := range eff.Responsibilities {
		roleItems = append(roleItems, map[string]any{
			"role_code": roleCode,
			"role_name": roleCode,
		})
	}
	sort.Slice(roleItems, func(i, j int) bool {
		return anyToString(roleItems[i]["role_code"]) < anyToString(roleItems[j]["role_code"])
	})
	writeEnvelope(w, http.StatusOK, map[string]any{
		"items":       roleItems,
		"permissions": eff.Permissions,
	}, map[string]any{"total": len(roleItems)})
}

func (h *Handler) rules(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireCMSAccess(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireAnyPermission(r.Context(), sub.MembershipID, sub.CompanyID, "system.settings", "platform.cms.view"); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	eff, err := h.authorizer.GetEffectiveAccess(r.Context(), sub.MembershipID, sub.CompanyID)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	items := []map[string]any{
		{
			"rule_id":     "cms-entry-gate",
			"name":        "CMS Entry Gate",
			"description": "Require explicit platform.cms.view at route entry-level",
			"enabled":     true,
			"permissions": []string{"platform.cms.view"},
		},
		{
			"rule_id":     "cms-role-sync",
			"name":        "Role Sync",
			"description": "Keep CMS actions bounded by effective access permissions",
			"enabled":     true,
			"permissions": eff.Permissions,
		},
	}
	writeEnvelope(w, http.StatusOK, map[string]any{"items": items}, map[string]any{"total": len(items)})
}

func (h *Handler) validateRule(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireCMSAccess(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireAnyPermission(r.Context(), sub.MembershipID, sub.CompanyID, "system.settings", "platform.cms.view"); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var payload struct {
		Name        string   `json:"name"`
		Permissions []string `json:"permissions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid JSON payload", err))
		return
	}
	name := strings.TrimSpace(payload.Name)
	if name == "" {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "name is required", nil))
		return
	}
	if len(payload.Permissions) == 0 {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "permissions are required", nil))
		return
	}
	_ = h.auditSvc.AppendAuditLog(r.Context(), auditapp.AppendAuditLogRequest{
		ActorUserID:       sub.Sub,
		ActorMembershipID: sub.MembershipID,
		CompanyID:         sub.CompanyID,
		Action:            cmsActionRuleValidate,
		ResourceType:      "cms_rule",
		ResourceID:        name,
		Decision:          "allow",
		Metadata: map[string]any{
			"permission_count": len(payload.Permissions),
		},
	})
	writeEnvelope(w, http.StatusOK, map[string]any{
		"valid":       true,
		"name":        name,
		"permissions": payload.Permissions,
	}, nil)
}

func (h *Handler) auditLogs(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireCMSAccess(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireAnyPermission(r.Context(), sub.MembershipID, sub.CompanyID, "rbac.manage", "system.settings", "platform.cms.view"); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	limit := parsePositiveInt(r.URL.Query().Get("limit"), 50, 200)
	action := strings.TrimSpace(r.URL.Query().Get("action"))
	resourceType := strings.TrimSpace(r.URL.Query().Get("resource_type"))
	resourceID := strings.TrimSpace(r.URL.Query().Get("resource_id"))
	fromOccurredAt := strings.TrimSpace(r.URL.Query().Get("from"))
	toOccurredAt := strings.TrimSpace(r.URL.Query().Get("to"))
	cursor := strings.TrimSpace(r.URL.Query().Get("cursor"))
	if action != "" {
		if _, ok := cmsKnownActions[action]; !ok {
			httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "unsupported action filter", nil))
			return
		}
	}
	entries, err := h.auditRepo.ListByCompany(r.Context(), sub.CompanyID, action, resourceType, resourceID, fromOccurredAt, toOccurredAt, cursor, limit)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	events := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		events = append(events, map[string]any{
			"event_id":      entry.EventID,
			"action":        entry.Action,
			"actor":         entry.ActorUserID,
			"company_id":    entry.CompanyID,
			"resource_type": entry.ResourceType,
			"resource_id":   entry.ResourceID,
			"decision":      entry.Decision,
			"request_id":    entry.RequestID,
			"created_at":    entry.OccurredAt,
			"metadata":      entry.Metadata,
		})
	}
	var nextCursor any
	if len(events) == limit {
		if last, ok := events[len(events)-1]["created_at"].(string); ok && strings.TrimSpace(last) != "" {
			nextCursor = last
		}
	}
	writeEnvelope(w, http.StatusOK, map[string]any{"items": events}, map[string]any{
		"total":       len(events),
		"limit":       limit,
		"next_cursor": nextCursor,
	})
}

func (h *Handler) systemMetrics(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireCMSAccess(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireAnyPermission(r.Context(), sub.MembershipID, sub.CompanyID, "system.settings", "platform.cms.view"); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	writeEnvelope(w, http.StatusOK, map[string]any{
		"routes":       h.metrics.snapshot(),
		"action_codes": mapsKeysSorted(cmsKnownActions),
		"trace_header": httpx.RequestIDHeader,
	}, nil)
}

func (h *Handler) sessions(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireCMSAccess(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireAnyPermission(r.Context(), sub.MembershipID, sub.CompanyID, "rbac.manage", "system.settings", "platform.cms.view"); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	list, err := h.iamSvc.ListSessions(r.Context(), iamapp.ListSessionsRequest{
		UserID:           sub.Sub,
		CurrentSessionID: sub.SessionID,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	items := make([]map[string]any, 0, len(list.Items))
	for _, session := range list.Items {
		status := "active"
		if session.Revoked {
			status = "revoked"
		}
		items = append(items, map[string]any{
			"session_id":   session.SessionID,
			"user_id":      sub.Sub,
			"company_id":   firstNonEmpty(session.CurrentCompanyID, sub.CompanyID),
			"status":       status,
			"last_seen_at": time.Now().UTC().Format(timeLayout),
		})
	}
	writeEnvelope(w, http.StatusOK, map[string]any{"items": items}, map[string]any{"total": len(items)})
}

func (h *Handler) revokeSession(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireCMSAccess(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireAnyPermission(r.Context(), sub.MembershipID, sub.CompanyID, "rbac.manage", "system.settings", "platform.cms.view"); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	sessionID := strings.TrimSpace(r.PathValue("session_id"))
	if sessionID == "" {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "session_id is required", nil))
		return
	}
	if _, err := h.iamSvc.RevokeSession(r.Context(), iamapp.RevokeSessionRequest{
		UserID:    sub.Sub,
		SessionID: sessionID,
	}); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	_ = h.auditSvc.AppendAuditLog(r.Context(), auditapp.AppendAuditLogRequest{
		ActorUserID:       sub.Sub,
		ActorMembershipID: sub.MembershipID,
		CompanyID:         sub.CompanyID,
		Action:            cmsActionSessionRevoke,
		ResourceType:      "session",
		ResourceID:        sessionID,
		Decision:          "allow",
	})
	writeEnvelope(w, http.StatusOK, map[string]any{"session_id": sessionID, "status": "revoked"}, nil)
}

func (h *Handler) systemHealth(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireCMSAccess(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireAnyPermission(r.Context(), sub.MembershipID, sub.CompanyID, "system.settings", "platform.cms.view"); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	apiStatus := "healthy"
	dbStatus := "healthy"
	cacheStatus := "degraded"
	if _, err := h.authorizer.GetEffectiveAccess(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		apiStatus = "degraded"
	}
	if _, err := h.disclosures.List(r.Context(), sub.CompanyID); err != nil {
		dbStatus = "degraded"
	}
	items := []map[string]any{{"component": "api", "status": apiStatus}, {"component": "db", "status": dbStatus}, {"component": "cache", "status": cacheStatus}}
	items = append(items, map[string]any{
		"component": "cms_observability",
		"status":    "healthy",
		"meta": map[string]any{
			"trace_header": httpx.RequestIDHeader,
			"route_count":  len(h.metrics.snapshot()),
		},
	})
	writeEnvelope(w, http.StatusOK, map[string]any{"items": items}, map[string]any{"total": len(items)})
}

func parsePositiveInt(raw string, fallback, max int) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n <= 0 {
		return fallback
	}
	if n > max {
		return max
	}
	return n
}

func firstNonEmpty(primary, fallback string) string {
	if strings.TrimSpace(primary) != "" {
		return primary
	}
	return fallback
}

func mapsKeysSorted(items map[string]struct{}) []string {
	out := make([]string, 0, len(items))
	for key := range items {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func (h *Handler) createCMSCompany(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireCMSAccess(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireAnyPermission(r.Context(), sub.MembershipID, sub.CompanyID, "rbac.manage", "system.settings"); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var p struct {
		CompanyName          string  `json:"company_name"`
		TaxCode              string  `json:"tax_code"`
		RegistrationNumber   string  `json:"registration_number"`
		Address              string  `json:"address"`
		Phone                string  `json:"phone"`
		ContactEmail         string  `json:"contact_email"`
		RepresentativeName   string  `json:"representative_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid JSON payload", err))
		return
	}
	resp, err := h.adminSvc.CreateCompany(r.Context(), companyaccessapp.CreateCompanyRequest{
		Subject: companyaccessapp.AdminSubject{
			UserID:       sub.Sub,
			MembershipID: sub.MembershipID,
			CompanyID:    sub.CompanyID,
		},
		CompanyName:        p.CompanyName,
		TaxCode:              p.TaxCode,
		RegistrationNumber:   p.RegistrationNumber,
		Address:              p.Address,
		Phone:                p.Phone,
		ContactEmail:         p.ContactEmail,
		RepresentativeName:   p.RepresentativeName,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	writeEnvelope(w, http.StatusCreated, resp, nil)
}

func (h *Handler) adminUsers(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireCMSAccess(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireAnyPermission(r.Context(), sub.MembershipID, sub.CompanyID, "rbac.manage", "system.settings"); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	companyID := strings.TrimSpace(r.URL.Query().Get("company_id"))
	if companyID == "" {
		companyID = sub.CompanyID
	}
	items, err := h.adminSvc.ListCompanyMemberships(r.Context(), companyaccessapp.ListCompanyMembershipsRequest{
		Subject: companyaccessapp.AdminSubject{
			UserID:       sub.Sub,
			MembershipID: sub.MembershipID,
			CompanyID:    sub.CompanyID,
		},
		CompanyID: companyID,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	writeEnvelope(w, http.StatusOK, map[string]any{"items": items}, map[string]any{"total": len(items)})
}

func (h *Handler) createAdminUser(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireCMSAccess(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireAnyPermission(r.Context(), sub.MembershipID, sub.CompanyID, "rbac.manage", "system.settings"); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var p struct {
		LoginID          string `json:"login_id"`
		Password         string `json:"password"`
		FullName         string `json:"full_name"`
		Email            string `json:"email"`
		Phone            string `json:"phone"`
		AccountStatus    string `json:"account_status"`
		CompanyID        string `json:"company_id"`
		MembershipStatus string `json:"membership_status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid JSON payload", err))
		return
	}
	resp, err := h.adminSvc.CreateUser(r.Context(), companyaccessapp.CreateUserRequest{
		Subject: companyaccessapp.AdminSubject{
			UserID:       sub.Sub,
			MembershipID: sub.MembershipID,
			CompanyID:    sub.CompanyID,
		},
		LoginID:          p.LoginID,
		Password:         p.Password,
		FullName:         p.FullName,
		Email:            p.Email,
		Phone:            p.Phone,
		AccountStatus:    p.AccountStatus,
		CompanyID:        p.CompanyID,
		MembershipStatus: p.MembershipStatus,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	writeEnvelope(w, http.StatusCreated, resp, nil)
}

func (h *Handler) inviteAdminUser(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireCMSAccess(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireAnyPermission(r.Context(), sub.MembershipID, sub.CompanyID, "rbac.manage", "system.settings"); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var p struct {
		Email            string `json:"email"`
		FullName         string `json:"full_name"`
		CompanyID        string `json:"company_id"`
		MembershipStatus string `json:"membership_status"`
		RoleID           string `json:"role_id"`
		RoleCode         string `json:"role_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid JSON payload", err))
		return
	}
	resp, err := h.adminSvc.InviteUser(r.Context(), companyaccessapp.InviteUserRequest{
		Subject: companyaccessapp.AdminSubject{
			UserID:       sub.Sub,
			MembershipID: sub.MembershipID,
			CompanyID:    sub.CompanyID,
		},
		Email:            p.Email,
		FullName:         p.FullName,
		CompanyID:        p.CompanyID,
		MembershipStatus: p.MembershipStatus,
		CreatedByUserID:  sub.Sub,
		RoleID:           p.RoleID,
		RoleCode:         p.RoleCode,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	_ = h.auditSvc.AppendAuditLog(r.Context(), auditapp.AppendAuditLogRequest{
		ActorUserID:       sub.Sub,
		ActorMembershipID: sub.MembershipID,
		CompanyID:         sub.CompanyID,
		Action:            "cms.admin.users.invite",
		ResourceType:      "user",
		ResourceID:        resp.UserID,
		Decision:          "allow",
	})
	writeEnvelope(w, http.StatusCreated, resp, nil)
}

func (h *Handler) resendInvitation(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireCMSAccess(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireAnyPermission(r.Context(), sub.MembershipID, sub.CompanyID, "rbac.manage", "system.settings"); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	userID := strings.TrimSpace(r.PathValue("user_id"))
	companyID := strings.TrimSpace(r.URL.Query().Get("company_id"))
	if companyID == "" {
		companyID = sub.CompanyID
	}
	if err := h.adminSvc.ResendUserInvitation(r.Context(), companyaccessapp.ResendUserInvitationRequest{
		Subject: companyaccessapp.AdminSubject{
			UserID:       sub.Sub,
			MembershipID: sub.MembershipID,
			CompanyID:    sub.CompanyID,
		},
		UserID:    userID,
		CompanyID: companyID,
	}); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	_ = h.auditSvc.AppendAuditLog(r.Context(), auditapp.AppendAuditLogRequest{
		ActorUserID:       sub.Sub,
		ActorMembershipID: sub.MembershipID,
		CompanyID:         sub.CompanyID,
		Action:            "cms.admin.users.invite.resend",
		ResourceType:      "user",
		ResourceID:        userID,
		Decision:          "allow",
	})
	writeEnvelope(w, http.StatusOK, map[string]any{"success": true}, nil)
}

func (h *Handler) requestAdminPasswordReset(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireCMSAccess(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireAnyPermission(r.Context(), sub.MembershipID, sub.CompanyID, "rbac.manage", "system.settings"); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	target := strings.TrimSpace(r.PathValue("user_id"))
	if err := h.iamSvc.AdminRequestPasswordReset(r.Context(), target); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	_ = h.auditSvc.AppendAuditLog(r.Context(), auditapp.AppendAuditLogRequest{
		ActorUserID:       sub.Sub,
		ActorMembershipID: sub.MembershipID,
		CompanyID:         sub.CompanyID,
		Action:            "cms.admin.users.password_reset",
		ResourceType:      "user",
		ResourceID:        target,
		Decision:          "allow",
	})
	writeEnvelope(w, http.StatusOK, map[string]any{"success": true}, nil)
}

func (h *Handler) subject(r *http.Request) (iamapp.AccessTokenClaims, error) {
	claims, err := h.inspector.InspectAccessToken(r.Context(), bearerToken(r.Header.Get("Authorization")))
	if err != nil {
		return iamapp.AccessTokenClaims{}, err
	}
	if claims == nil {
		return iamapp.AccessTokenClaims{}, perr.NewHTTPError(http.StatusUnauthorized, perr.CodeSessionExpired, "invalid token", nil)
	}
	return *claims, nil
}

func (h *Handler) requireAnyPermission(ctx context.Context, membershipID, companyID string, permissions ...string) ([]string, error) {
	eff, err := h.authorizer.GetEffectiveAccess(ctx, membershipID, companyID)
	if err != nil {
		return nil, err
	}
	if !hasAnyPermission(eff.Permissions, permissions...) {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "access denied", nil)
	}
	return eff.Permissions, nil
}

func (h *Handler) requireCMSAccess(ctx context.Context, membershipID, companyID string) ([]string, error) {
	return h.requireAnyPermission(ctx, membershipID, companyID, "platform.cms.view")
}

func hasAnyPermission(items []string, expected ...string) bool {
	for _, candidate := range expected {
		for _, item := range items {
			if strings.EqualFold(strings.TrimSpace(item), strings.TrimSpace(candidate)) {
				return true
			}
		}
	}
	return false
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

const timeLayout = "2006-01-02T15:04:05Z"

func writeEnvelope(w http.ResponseWriter, status int, data any, meta map[string]any) {
	body := map[string]any{
		"data": data,
	}
	if len(meta) > 0 {
		body["meta"] = meta
	}
	httpx.WriteJSON(w, status, body)
}
