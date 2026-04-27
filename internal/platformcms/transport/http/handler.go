package http

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	companyaccessapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
)

type Handler struct {
	inspector    iamapp.TokenInspector
	authorizer   authapp.Service
	adminSvc     companyaccessapp.AdminService
	disclosureSvc disclosureapp.Service
	disclosures disclosureapp.Repository
}

func NewHandler(inspector iamapp.TokenInspector, authorizer authapp.Service, adminSvc companyaccessapp.AdminService, disclosureSvc disclosureapp.Service, disclosures disclosureapp.Repository) *Handler {
	return &Handler{
		inspector:     inspector,
		authorizer:    authorizer,
		adminSvc:      adminSvc,
		disclosureSvc: disclosureSvc,
		disclosures:   disclosures,
	}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/platform/cms/dashboard/summary", h.dashboardSummary)
	mux.HandleFunc("GET /api/v1/platform/cms/collections", h.collections)
	mux.HandleFunc("GET /api/v1/platform/cms/collections/{collection_id}", h.collectionDetail)
	mux.HandleFunc("GET /api/v1/platform/cms/entries", h.entries)
	mux.HandleFunc("GET /api/v1/platform/cms/entries/{entry_id}", h.entryDetail)
	mux.HandleFunc("POST /api/v1/platform/cms/entries", h.createEntry)
	mux.HandleFunc("PUT /api/v1/platform/cms/entries/{entry_id}", h.updateEntry)
	mux.HandleFunc("GET /api/v1/platform/cms/reviews", h.reviews)
	mux.HandleFunc("POST /api/v1/platform/cms/reviews/{entry_id}", h.reviewAction)
	mux.HandleFunc("GET /api/v1/platform/cms/schedules", h.schedules)
	mux.HandleFunc("POST /api/v1/platform/cms/schedules", h.createSchedule)
	mux.HandleFunc("DELETE /api/v1/platform/cms/schedules/{entry_id}", h.deleteSchedule)
	mux.HandleFunc("GET /api/v1/platform/cms/admin/users", h.adminUsers)
	mux.HandleFunc("POST /api/v1/platform/cms/admin/users", h.createAdminUser)
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
		CollectionID   string `json:"collection_id"`
		Name           string `json:"name"`
		EntryCount     int    `json:"entry_count"`
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
			"entry_id":    item.RecordID,
			"title":       item.Title,
			"status":      item.Status,
			"type_id":     typeID,
			"updated_at":  item.UpdatedAt.UTC().Format(timeLayout),
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
			"entry_id":    item.RecordID,
			"title":       item.Title,
			"status":      item.Status,
			"type_id":     typeID,
			"updated_at":  item.UpdatedAt.UTC().Format(timeLayout),
			"company_id":  item.CompanyID,
			"record_id":   item.RecordID,
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
		"entry_id":      rec.RecordID,
		"title":         rec.Title,
		"summary":       rec.Summary,
		"content":       rec.Content,
		"status":        rec.Status,
		"type_id":       rec.TypeID,
		"planned_date":  rec.PlannedDate,
		"published_date": rec.PublishedDate,
		"updated_at":    rec.UpdatedAt.UTC().Format(timeLayout),
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
	writeEnvelope(w, http.StatusCreated, map[string]any{
		"entry_id":     rec.RecordID,
		"record_id":    rec.RecordID,
		"title":        rec.Title,
		"status":       rec.Status,
		"type_id":      rec.TypeID,
		"updated_at":   rec.UpdatedAt.UTC().Format(timeLayout),
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
	writeEnvelope(w, http.StatusOK, map[string]any{
		"entry_id":     rec.RecordID,
		"record_id":    rec.RecordID,
		"title":        rec.Title,
		"status":       rec.Status,
		"type_id":      rec.TypeID,
		"updated_at":   rec.UpdatedAt.UTC().Format(timeLayout),
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
			"entry_id":    item.RecordID,
			"title":       item.Title,
			"status":      item.Status,
			"type_id":     item.TypeID,
			"updated_at":  item.UpdatedAt.UTC().Format(timeLayout),
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
		writeEnvelope(w, http.StatusOK, map[string]any{
			"entry_id":  rec.RecordID,
			"status":    rec.Status,
			"decision":  "approve",
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
		writeEnvelope(w, http.StatusOK, map[string]any{
			"entry_id":  next.RecordID,
			"status":    next.Status,
			"decision":  "reject",
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
			"entry_id":    item.RecordID,
			"publish_at":  item.PlannedDate,
			"status":      item.Status,
			"title":       item.Title,
			"updated_at":  item.UpdatedAt.UTC().Format(timeLayout),
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
	writeEnvelope(w, http.StatusOK, map[string]any{
		"entry_id":   next.RecordID,
		"publish_at": next.PlannedDate,
		"status":     next.Status,
	}, nil)
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
