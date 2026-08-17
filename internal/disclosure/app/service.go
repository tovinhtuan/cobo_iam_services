package app

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

type service struct {
	repo                               Repository
	auth                               authapp.Service
	idg                                idgen.Generator
	calculator                         *DeadlineCalculator
	holidayProvider                    HolidayCalendarProvider
	deadlineEngineAdapter              DeadlineEngineAdapter
	shadowRunner                       *deadlineEngineShadowRunner
	workflowGroupsEnabled              bool
	templateApplicabilityStrictFilter  bool
	deadlineEngineV2Shadow             bool
	legalBasisStructuredWriteEnabled   bool
	legalBasisLegacyFallbackEnabled    bool
	legalBasisDivergenceWarningEnabled bool
	tierLookup                         func(ctx context.Context, userID string) string
	workflowBootstrap                  WorkflowBootstrapper
}

// ServiceOption configures disclosure service construction.
type ServiceOption func(*service)

// WithHolidayCalendarProvider overrides the default JSON file holiday provider (e.g. DB-backed composite).
func WithHolidayCalendarProvider(p HolidayCalendarProvider) ServiceOption {
	return func(s *service) {
		if p != nil {
			s.calculator = NewDeadlineCalculator(p)
			s.holidayProvider = p
		}
	}
}

// WithDeadlineEngineV2Shadow enables Batch 5B shadow-compute: Deadline Engine
// V2 (Source C) is computed alongside the existing runtime for Portal
// Preview, Periodic Worker, and Manual Create, compared against the existing
// result, and logged (event=deadline_engine_shadow). Default false
// (DEADLINE_ENGINE_V2_SHADOW). Shadow compute never affects DB writes, API
// responses, or worker output — see deadlineengine_shadow.go.
func WithDeadlineEngineV2Shadow(enabled bool) ServiceOption {
	return func(s *service) {
		s.deadlineEngineV2Shadow = enabled
	}
}

// WithWorkflowGroupsEnabled enables the WORKFLOW_GROUPS_ENABLED feature flag.
func WithWorkflowGroupsEnabled(enabled bool) ServiceOption {
	return func(s *service) {
		s.workflowGroupsEnabled = enabled
	}
}

// WithTemplateApplicabilityStrictFilter enables strict global template applicability filtering.
func WithTemplateApplicabilityStrictFilter(enabled bool) ServiceOption {
	return func(s *service) {
		s.templateApplicabilityStrictFilter = enabled
	}
}

// WithLegalBasisStructuredWriteEnabled gates Phase 12.2 structured write precedence
// (LEGAL_BASIS_STRUCTURED_WRITE_ENABLED). Default false.
func WithLegalBasisStructuredWriteEnabled(enabled bool) ServiceOption {
	return func(s *service) {
		s.legalBasisStructuredWriteEnabled = enabled
	}
}

// WithLegalBasisLegacyFallbackEnabled gates OD-2 synthesize on read
// (LEGAL_BASIS_LEGACY_FALLBACK_ENABLED). Default true.
func WithLegalBasisLegacyFallbackEnabled(enabled bool) ServiceOption {
	return func(s *service) {
		s.legalBasisLegacyFallbackEnabled = enabled
	}
}

// WithLegalBasisDivergenceWarningEnabled gates divergence warning logs
// (LEGAL_BASIS_DIVERGENCE_WARNING_ENABLED). Default true.
func WithLegalBasisDivergenceWarningEnabled(enabled bool) ServiceOption {
	return func(s *service) {
		s.legalBasisDivergenceWarningEnabled = enabled
	}
}

// WithSubscriptionTierLookup injects a function that returns the subscription tier for a user.
// Used for server-side template quota enforcement. If not set, quota is not enforced.
func WithSubscriptionTierLookup(fn func(ctx context.Context, userID string) string) ServiceOption {
	return func(s *service) {
		s.tierLookup = fn
	}
}

// WithWorkflowBootstrap wires workflow instance creation on disclosure submit.
func WithWorkflowBootstrap(b WorkflowBootstrapper) ServiceOption {
	return func(s *service) {
		s.workflowBootstrap = b
	}
}

// SetWorkflowBootstrap allows late binding after workflow service construction.
func (s *service) SetWorkflowBootstrap(b WorkflowBootstrapper) {
	s.workflowBootstrap = b
}

func companyTemplateQuotaLimit(tier string) int {
	if tier == "Free" || tier == "" {
		return 5
	}
	return 100
}

const (
	templateScopeGlobal  = "global"
	templateScopeCompany = "company"
)

func NewService(repo Repository, auth authapp.Service, idg idgen.Generator, opts ...ServiceOption) Service {
	holidayProvider := NewHolidayCalendarFileProvider(filepath.Join("configs", "non_trading_days"))
	s := &service{
		repo:                               repo,
		auth:                               auth,
		idg:                                idg,
		calculator:                         NewDeadlineCalculator(holidayProvider),
		holidayProvider:                    holidayProvider,
		legalBasisLegacyFallbackEnabled:    true,
		legalBasisDivergenceWarningEnabled: true,
	}
	for _, o := range opts {
		o(s)
	}
	s.deadlineEngineAdapter = NewDeadlineEngineAdapter(s.holidayProvider)
	s.shadowRunner = newDeadlineEngineShadowRunner(s.deadlineEngineAdapter, s.deadlineEngineV2Shadow)
	return s
}

// READY_FOR_5B (Manual Create): plannedDate is client-supplied and passed
// through as-is (no server-side T0/N computation on this path today). 5B+
// may offer DeadlineEngineAdapter.ResolveDeadline as a hint/default for the
// client, but MUST NOT override a client-supplied planned_date. Behavior
// unchanged in Batch 5A.
func (s *service) CreateRecord(ctx context.Context, req CreateRecordRequest) (*RecordDTO, error) {
	if strings.TrimSpace(req.Payload.Title) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "title is required", nil)
	}
	if strings.TrimSpace(req.Payload.Content) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "content is required", nil)
	}
	plannedDate, err := normalizeDate(req.Payload.PlannedDate)
	if err != nil {
		return nil, err
	}
	departmentID := strings.TrimSpace(req.Payload.DepartmentID)
	if departmentID == "" {
		departmentID = "general"
	}
	if err := s.authorize(ctx, req.Subject, "disclosure.create", authapp.ResourceRef{
		Type: "disclosure_record",
		ID:   "",
		Attributes: map[string]any{
			"department_id":       departmentID,
			"owner_membership_id": req.Subject.MembershipID,
			"workflow_state":      "draft",
		},
	}); err != nil {
		return nil, err
	}
	// BE-002: Enforce has_workflow gate. A disclosure record cannot be created
	// unless the selected template has an effective active workflow.
	// TODO: BE-002 — this gate is wired in but requires global_workflows (migration 0053)
	// to be populated. Until CMS seeds data via the new schema, use feature-flag guard.
	if typeID := strings.TrimSpace(req.Payload.TypeID); typeID != "" {
		if err := s.enforceHasWorkflowGate(ctx, req.Subject.CompanyID, typeID); err != nil {
			return nil, err
		}
		if err := s.enforceTemplateApplicability(ctx, req.Subject.CompanyID, typeID); err != nil {
			return nil, err
		}
		if err := s.enforceStructureDeadlineOnCreate(ctx, req.Subject.CompanyID, typeID); err != nil {
			return nil, err
		}
	}
	recordID := strings.TrimSpace(req.RecordID)
	if recordID == "" {
		recordID = s.idg.NewUUID()
	}
	rec := RecordDTO{
		RecordID:     recordID,
		CompanyID:    req.Subject.CompanyID,
		TypeID:       strings.TrimSpace(req.Payload.TypeID),
		DepartmentID: departmentID,
		Title:        strings.TrimSpace(req.Payload.Title),
		Summary:      strings.TrimSpace(req.Payload.Summary),
		Content:      strings.TrimSpace(req.Payload.Content),
		PlannedDate:  plannedDate,
		Status:       "Draft",
		Attachments:  sanitizeAttachments(req.Payload.Attachments),
		EvidenceLink: strings.TrimSpace(req.Payload.EvidenceLink),
		CreatedBy:    req.Subject.UserID,
		UpdatedBy:    req.Subject.UserID,
	}
	created, err := s.repo.Create(ctx, rec)
	if err != nil {
		return created, err
	}
	// Batch 5B Phase C (shadow only, see deadlineengine_shadow.go): audit-only,
	// does not mutate the request or the persisted record. Guarded so the
	// default (DEADLINE_ENGINE_V2_SHADOW=false) issues zero extra repo calls.
	if rec.TypeID != "" && plannedDate != "" && s.shadowRunner != nil && s.shadowRunner.enabled {
		s.shadowManualCreate(ctx, req.Subject.CompanyID, rec.TypeID, plannedDate, time.Now())
	}
	return created, nil
}

// shadowManualCreate fetches the type's deadline config and company context
// (best-effort) and delegates to shadowRunner.manualCreate. Any error here is
// swallowed — shadow compute must never affect CreateRecord's response.
func (s *service) shadowManualCreate(ctx context.Context, companyID, typeID, plannedDate string, now time.Time) {
	item, err := s.repo.GetTypeDetail(ctx, companyID, typeID)
	if err != nil || item == nil || item.DeadlineConfig == nil {
		return
	}
	companyCtx, err := s.repo.GetCompanyTypeDeadlineContext(ctx, companyID, typeID)
	if err != nil {
		return
	}
	profile, err := s.repo.GetCompanyApplicabilityProfile(ctx, companyID)
	if err != nil {
		return
	}
	s.shadowRunner.manualCreate(ctx, companyID, typeID,
		item.DeadlineConfig, item.ApplicabilityRules, item.TemplateCategory,
		companyCtx, profile, plannedDate, now)
}

func (s *service) UpdateRecord(ctx context.Context, req UpdateRecordRequest) (*RecordDTO, error) {
	if strings.TrimSpace(req.RecordID) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "record_id is required", nil)
	}
	cur, err := s.repo.FindByID(ctx, req.Subject.CompanyID, req.RecordID)
	if err != nil {
		return nil, err
	}
	plannedDate, err := normalizeDate(req.Payload.PlannedDate)
	if err != nil {
		return nil, err
	}
	departmentID := strings.TrimSpace(req.Payload.DepartmentID)
	if departmentID == "" {
		departmentID = cur.DepartmentID
	}
	if err := s.authorize(ctx, req.Subject, "disclosure.update", authapp.ResourceRef{
		Type: "disclosure_record",
		ID:   req.RecordID,
		Attributes: map[string]any{
			"department_id":       cur.DepartmentID,
			"owner_membership_id": req.Subject.MembershipID,
			"workflow_state":      strings.ToLower(cur.Status),
		},
	}); err != nil {
		return nil, err
	}
	cur.TypeID = strings.TrimSpace(req.Payload.TypeID)
	cur.DepartmentID = departmentID
	cur.Title = strings.TrimSpace(req.Payload.Title)
	cur.Summary = strings.TrimSpace(req.Payload.Summary)
	cur.Content = strings.TrimSpace(req.Payload.Content)
	cur.PlannedDate = plannedDate
	cur.Attachments = sanitizeAttachments(req.Payload.Attachments)
	cur.EvidenceLink = strings.TrimSpace(req.Payload.EvidenceLink)
	cur.UpdatedBy = req.Subject.UserID
	return s.repo.Update(ctx, *cur)
}

func (s *service) SubmitRecord(ctx context.Context, req SubmitRecordRequest) (*RecordDTO, error) {
	if strings.TrimSpace(req.RecordID) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "record_id is required", nil)
	}
	cur, err := s.repo.FindByID(ctx, req.Subject.CompanyID, req.RecordID)
	if err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, req.Subject, "disclosure.submit", authapp.ResourceRef{
		Type: "disclosure_record",
		ID:   req.RecordID,
		Attributes: map[string]any{
			"department_id":       cur.DepartmentID,
			"owner_membership_id": req.Subject.MembershipID,
			"workflow_state":      strings.ToLower(cur.Status),
		},
	}); err != nil {
		return nil, err
	}
	if strings.EqualFold(cur.Status, "Draft") {
		cur.Status = "PendingReview"
	} else {
		cur.Status = "In Progress"
	}
	cur.UpdatedBy = req.Subject.UserID
	updated, err := s.repo.Update(ctx, *cur)
	if err != nil {
		return nil, err
	}
	if s.workflowBootstrap != nil && strings.TrimSpace(updated.WorkflowInstanceID) == "" {
		if wfID, wfErr := s.workflowBootstrap.EnsureOnSubmit(ctx, req.Subject, *updated); wfErr == nil && wfID != "" {
			updated.WorkflowInstanceID = wfID
		}
	}
	return updated, nil
}

func (s *service) ConfirmRecord(ctx context.Context, req ConfirmRecordRequest) (*RecordDTO, error) {
	if strings.TrimSpace(req.RecordID) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "record_id is required", nil)
	}
	cur, err := s.repo.FindByID(ctx, req.Subject.CompanyID, req.RecordID)
	if err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, req.Subject, "disclosure.approve", authapp.ResourceRef{
		Type: "disclosure_record",
		ID:   req.RecordID,
		Attributes: map[string]any{
			"department_id":       cur.DepartmentID,
			"owner_membership_id": req.Subject.MembershipID,
			"workflow_state":      strings.ToLower(cur.Status),
		},
	}); err != nil {
		return nil, err
	}
	status := strings.ToLower(strings.TrimSpace(cur.Status))
	if status != "published" && status != "approved" {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "record is not in approved state", nil)
	}
	cur.Status = "Completed"
	cur.UpdatedBy = req.Subject.UserID
	// Forward-only outcome timestamp for personal-ops on_time_rate (no historical backfill).
	StampCompletedAtIfNeeded(cur, "confirm_record", time.Now().UTC())
	return s.repo.Update(ctx, *cur)
}

func (s *service) ListRecords(ctx context.Context, req ListRecordsRequest) (*ListRecordsResponse, error) {
	if err := s.authorize(ctx, req.Subject, "disclosure.view", authapp.ResourceRef{
		Type: "disclosure_record",
		ID:   "",
		Attributes: map[string]any{
			"workflow_state": "*",
		},
	}); err != nil {
		return nil, err
	}
	items, err := s.repo.List(ctx, req.Subject.CompanyID)
	if err != nil {
		return nil, err
	}
	return &ListRecordsResponse{Items: items}, nil
}

func (s *service) GetRecord(ctx context.Context, req GetRecordRequest) (*RecordDTO, error) {
	if strings.TrimSpace(req.RecordID) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "record_id is required", nil)
	}
	cur, err := s.repo.FindByID(ctx, req.Subject.CompanyID, req.RecordID)
	if err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, req.Subject, "disclosure.view", authapp.ResourceRef{
		Type: "disclosure_record",
		ID:   req.RecordID,
		Attributes: map[string]any{
			"department_id":       cur.DepartmentID,
			"owner_membership_id": req.Subject.MembershipID,
			"workflow_state":      strings.ToLower(cur.Status),
		},
	}); err != nil {
		return nil, err
	}
	return cur, nil
}

func (s *service) ListTypeGroups(ctx context.Context, req ListTypeGroupsRequest) (*ListTypeGroupsResponse, error) {
	if err := s.requireDisclosureCatalogRead(ctx, req.Subject); err != nil {
		return nil, err
	}
	out, err := s.repo.ListTypeGroups(ctx, req.Subject.CompanyID)
	if err != nil {
		return nil, err
	}
	return &ListTypeGroupsResponse{Items: out}, nil
}

func (s *service) ListDisplayGroups(ctx context.Context, req ListDisplayGroupsRequest) (*ListDisplayGroupsResponse, error) {
	if err := s.requireDisclosureCatalogRead(ctx, req.Subject); err != nil {
		return nil, err
	}
	items, err := s.repo.ListDisplayGroups(ctx)
	if err != nil {
		return nil, err
	}
	return &ListDisplayGroupsResponse{Items: items}, nil
}

var allowedSortBy = map[string]bool{"name": true, "created_at": true}
var allowedSortDir = map[string]bool{"asc": true, "desc": true}

// listTypesLightweightChunkSize caps each MySQL round-trip for applicability filtering.
// Keeps result sets small when max_allowed_packet is misconfigured on legacy DEV MySQL.
const listTypesLightweightChunkSize = 50

func (s *service) ListTypes(ctx context.Context, req ListTypesRequest) (*ListTypesResponse, error) {
	if err := s.requireDisclosureCatalogRead(ctx, req.Subject); err != nil {
		return nil, err
	}
	sortBy := strings.ToLower(strings.TrimSpace(req.SortBy))
	sortDir := strings.ToLower(strings.TrimSpace(req.SortDir))
	if sortBy == "" {
		sortBy = "created_at"
	} else if !allowedSortBy[sortBy] {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "sort_by must be one of: name, created_at", nil)
	}
	if sortDir == "" {
		sortDir = "desc"
	} else if !allowedSortDir[sortDir] {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "sort_dir must be one of: asc, desc", nil)
	}
	scope := strings.ToLower(strings.TrimSpace(req.Scope))
	if scope != "" && scope != templateScopeGlobal && scope != templateScopeCompany {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "scope must be one of: global, company", nil)
	}
	page := req.Page
	pageSize := req.PageSize
	if req.PageProvided && page <= 0 {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "page must be a positive integer", nil)
	}
	if !req.PageProvided {
		page = 1
	}
	if req.PageSizeProvided && (pageSize <= 0 || pageSize > 100) {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "page_size must be between 1 and 100", nil)
	}
	if !req.PageSizeProvided {
		pageSize = 20
	}

	// Phase 1: lightweight rows for applicability filtering without loading the full catalog payload.
	light, err := s.listTypesLightweightCatalog(ctx, ListTypesParams{
		CompanyID:        req.Subject.CompanyID,
		GroupID:          req.GroupID,
		DisplayGroupCode: req.DisplayGroupCode,
		Query:            req.Query,
		Scope:            scope,
		Tags:             req.Tags,
		Periodicity:      NormalizePeriodicityFilter(req.Periodicity),
		DepartmentID:     strings.TrimSpace(req.DepartmentID),
		SortBy:           sortBy,
		SortDir:          sortDir,
		LightweightOnly:  true,
	})
	if err != nil {
		return nil, err
	}
	filtered, err := s.filterTypesByApplicability(ctx, req.Subject.CompanyID, light)
	if err != nil {
		return nil, err
	}
	sortTypeSummaries(filtered, sortBy, sortDir)
	total := len(filtered)
	start := (page - 1) * pageSize
	if start >= total {
		return &ListTypesResponse{Items: []DisclosureTypeSummaryDTO{}, Total: total, Page: page, PageSize: pageSize}, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	pageSlice := filtered[start:end]
	pageIDs := make([]string, 0, len(pageSlice))
	for _, item := range pageSlice {
		pageIDs = append(pageIDs, item.TypeID)
	}

	// Phase 2: load full summary fields only for the current page.
	out, _, err := s.repo.ListTypes(ctx, ListTypesParams{
		CompanyID: req.Subject.CompanyID,
		Scope:     scope,
		TypeIDs:   pageIDs,
		SortBy:    sortBy,
		SortDir:   sortDir,
	})
	if err != nil {
		return nil, err
	}
	byID := make(map[string]DisclosureTypeSummaryDTO, len(out))
	for _, item := range out {
		byID[item.TypeID] = item
	}
	ordered := make([]DisclosureTypeSummaryDTO, 0, len(pageIDs))
	for _, id := range pageIDs {
		if item, ok := byID[id]; ok {
			ordered = append(ordered, item)
		}
	}
	catalog := s.loadDeadlineRuleCatalog(ctx)
	for i := range ordered {
		enrichDeadlineRuleDisplaySummary(&ordered[i], catalog)
	}
	return &ListTypesResponse{Items: ordered, Total: total, Page: page, PageSize: pageSize}, nil
}

func (s *service) ListTypeFilterOptions(ctx context.Context, req ListTypeFilterOptionsRequest) (*ListTypeFilterOptionsResponse, error) {
	if err := s.requireDisclosureCatalogRead(ctx, req.Subject); err != nil {
		return nil, err
	}
	resp, err := s.repo.ListTypeFilterOptions(ctx, req.Subject.CompanyID)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		resp = &ListTypeFilterOptionsResponse{}
	}
	if resp.Tags == nil {
		resp.Tags = []TypeFilterOptionDTO{}
	}
	if resp.Departments == nil {
		resp.Departments = []TypeFilterOptionDTO{}
	}
	if len(resp.Frequencies) == 0 {
		resp.Frequencies = DefaultFrequencyFilterOptions()
	}
	return resp, nil
}

func (s *service) listTypesLightweightCatalog(ctx context.Context, base ListTypesParams) ([]DisclosureTypeSummaryDTO, error) {
	base.LightweightOnly = true
	base.PageSize = listTypesLightweightChunkSize
	var all []DisclosureTypeSummaryDTO
	for page := 1; ; page++ {
		base.Page = page
		chunk, total, err := s.repo.ListTypes(ctx, base)
		if err != nil {
			return nil, err
		}
		all = append(all, chunk...)
		if len(chunk) == 0 || len(all) >= total {
			break
		}
	}
	return all, nil
}

func sortTypeSummaries(items []DisclosureTypeSummaryDTO, sortBy, sortDir string) {
	less := func(i, j int) bool {
		switch sortBy {
		case "name":
			if strings.EqualFold(items[i].Name, items[j].Name) {
				return items[i].TypeID < items[j].TypeID
			}
			lessName := strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
			if sortDir == "asc" {
				return lessName
			}
			return !lessName
		default:
			if items[i].CreatedAt.Equal(items[j].CreatedAt) {
				return items[i].TypeID < items[j].TypeID
			}
			before := items[i].CreatedAt.Before(items[j].CreatedAt)
			if sortDir == "asc" {
				return before
			}
			return !before
		}
	}
	sort.Slice(items, less)
}

func (s *service) GetTypeDetail(ctx context.Context, req GetTypeDetailRequest) (*DisclosureTypeDTO, error) {
	if strings.TrimSpace(req.TypeID) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "type_id is required", nil)
	}
	if err := s.requireDisclosureCatalogRead(ctx, req.Subject); err != nil {
		return nil, err
	}
	item, err := s.repo.GetTypeDetail(ctx, req.Subject.CompanyID, req.TypeID)
	if err != nil {
		return nil, err
	}
	ApplyLegalBasisReadCompat(ctx, item, s.legalBasisLegacyFallbackEnabled, s.legalBasisDivergenceWarningEnabled)
	enrichDeadlineRuleDisplay(item, s.loadDeadlineRuleCatalog(ctx))

	var companyProfile applicability.CompanyApplicabilityProfile
	haveCompanyProfile := false
	if item.ApplicabilityRules != nil {
		if profile, profileErr := s.repo.GetCompanyApplicabilityProfile(ctx, req.Subject.CompanyID); profileErr == nil {
			companyProfile = profile
			haveCompanyProfile = true
			item.ResolvedDeadlineRule = buildResolvedDeadlineRuleDTO(
				item.ApplicabilityRules, companyProfile, item.Periodicity, item.DeadlineConfig,
			)
		}
	}

	if item.DeadlineConfig == nil {
		return item, nil
	}
	companyCtx, err := s.repo.GetCompanyTypeDeadlineContext(ctx, req.Subject.CompanyID, req.TypeID)
	if err != nil {
		item.DeadlineSummary = &DeadlineSummaryDTO{
			DeadlineMode:    item.DeadlineConfig.DeadlineMode,
			Status:          "UNKNOWN",
			RuleDescription: ptrString("Không lấy được ngữ cảnh doanh nghiệp để tính deadline."),
			Timezone:        ptrString("Asia/Ho_Chi_Minh"),
		}
		return item, nil
	}
	now := time.Now()
	deadlineCfg := item.DeadlineConfig
	if item.ApplicabilityRules != nil && deadlineCfg != nil && deadlineCfg.DeadlineMode == DeadlineModePeriodic {
		cfgCopy := *deadlineCfg
		if haveCompanyProfile {
			if days, ok := applicability.ResolveDeadlineDays(item.ApplicabilityRules, companyProfile); ok {
				cfgCopy.DeadlineDays = days
			}
			cfgCopy.DeadlineDurationType = applicability.ResolveDeadlineDurationType(item.ApplicabilityRules)
		}
		deadlineCfg = &cfgCopy
	}
	summary, err := s.calculator.CalculateDeadlineSummary(ctx, deadlineCfg, companyCtx, now)
	if err != nil {
		item.DeadlineSummary = &DeadlineSummaryDTO{
			DeadlineMode:    item.DeadlineConfig.DeadlineMode,
			Status:          "UNKNOWN",
			RuleDescription: ptrString("Không thể tính deadline từ cấu hình hiện tại."),
			Timezone:        ptrString("Asia/Ho_Chi_Minh"),
		}
		return item, nil
	}
	item.DeadlineSummary = summary
	attachResolvedDueDate(item.ResolvedDeadlineRule, summary)
	// Batch 5B Phase A (shadow only, see deadlineengine_shadow.go): does not
	// modify item/summary or the API response. Guarded so the default
	// (DEADLINE_ENGINE_V2_SHADOW=false) issues zero extra repo calls.
	if summary != nil && s.shadowRunner != nil && s.shadowRunner.enabled {
		if haveCompanyProfile {
			s.shadowRunner.portalPreview(ctx, req.Subject.CompanyID, req.TypeID,
				item.DeadlineConfig, item.ApplicabilityRules, item.TemplateCategory,
				companyCtx, companyProfile, summary.StartDate, summary.DeadlineDate, now)
		} else if profile, profileErr := s.repo.GetCompanyApplicabilityProfile(ctx, req.Subject.CompanyID); profileErr == nil {
			s.shadowRunner.portalPreview(ctx, req.Subject.CompanyID, req.TypeID,
				item.DeadlineConfig, item.ApplicabilityRules, item.TemplateCategory,
				companyCtx, profile, summary.StartDate, summary.DeadlineDate, now)
		}
	}
	return item, nil
}

func (s *service) GetTypeVersionDetail(ctx context.Context, req GetTypeVersionDetailRequest) (*DisclosureTypeDTO, error) {
	if err := s.requireCMSTemplateRead(ctx, req.Subject); err != nil {
		return nil, err
	}
	req.TypeID = strings.TrimSpace(req.TypeID)
	if req.TypeID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "type_id is required", nil)
	}
	if req.VersionNo <= 0 {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "version_no must be > 0", nil)
	}
	item, err := s.repo.GetTypeVersionDetail(ctx, req.Subject.CompanyID, req.TypeID, req.VersionNo)
	if err != nil {
		return nil, err
	}
	ApplyLegalBasisReadCompat(ctx, item, s.legalBasisLegacyFallbackEnabled, s.legalBasisDivergenceWarningEnabled)
	return item, nil
}

func (s *service) GetTemplateReferenceData(ctx context.Context, req GetTemplateReferenceDataRequest) (*GetTemplateReferenceDataResponse, error) {
	if err := s.requireCMSTemplateRead(ctx, req.Subject); err != nil {
		return nil, err
	}
	return &GetTemplateReferenceDataResponse{
		Data: TemplateReferenceDataDTO{
			TemplateCategories: []string{
				TemplateCategoryPeriodic,
				TemplateCategoryIrregular,
				TemplateCategoryCustom,
			},
			Periodicities: []string{
				PeriodicityMonthly,
				PeriodicityQuarterly,
				PeriodicityYearly,
				PeriodicityDaily,
				PeriodicityWeekly,
				PeriodicityEventBased,
				PeriodicityAdHoc,
			},
			DeadlineStrategies: []string{
				DeadlineStrategyFixedCycleDays,
				DeadlineStrategyEventHours,
				DeadlineStrategyConfigurable,
			},
			DeadlineRuleCatalog: s.loadDeadlineRuleCatalog(ctx),
			MatrixRules: map[string][]string{
				TemplateCategoryPeriodic: {
					"periodicity in [monthly, quarterly, yearly, daily, weekly]",
					"deadline_strategy must be fixed_cycle_days",
				},
				TemplateCategoryIrregular: {
					"periodicity must be event_based",
					"deadline_strategy must be event_relative_hours",
				},
				TemplateCategoryCustom: {
					"periodicity in [monthly, quarterly, yearly, ad_hoc]",
					"deadline_strategy must be configurable",
				},
			},
		},
	}, nil
}

func (s *service) UpsertTypeVersion(ctx context.Context, req UpsertTypeVersionRequest) (*UpsertTypeVersionResponse, error) {
	if err := s.requireCMSTemplateWrite(ctx, req.Subject); err != nil {
		return nil, err
	}
	isPlat := s.hasPermission(ctx, req.Subject, permissionPlatformCMSView)
	req.TypeID = strings.TrimSpace(req.TypeID)
	req.Scope = strings.ToLower(strings.TrimSpace(req.Scope))
	req.GroupID = strings.TrimSpace(req.GroupID)
	req.Name = strings.TrimSpace(req.Name)
	req.Category = strings.TrimSpace(req.Category)
	req.TemplateCategory = strings.TrimSpace(req.TemplateCategory)
	req.DeadlineStrategy = strings.TrimSpace(req.DeadlineStrategy)
	req.DeadlineRule = strings.TrimSpace(req.DeadlineRule)
	req.Periodicity = strings.TrimSpace(req.Periodicity)
	req.Checklist = sanitizeChecklist(req.Checklist)
	if req.TypeID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "type_id is required", nil)
	}
	if req.GroupID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "group_id is required", nil)
	}
	if req.Name == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "name is required", nil)
	}
	if err := validateTemplateDescription(req.Description); err != nil {
		return nil, err
	}
	if req.Scope == "" {
		if s.hasPermission(ctx, req.Subject, permissionPlatformCMSView) {
			req.Scope = templateScopeGlobal
		} else {
			req.Scope = templateScopeCompany
		}
	}
	if req.Scope != templateScopeGlobal && req.Scope != templateScopeCompany {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "scope must be global or company", nil)
	}
	if req.Scope == templateScopeGlobal && !isPlat {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "global scope requires platform admin permission", nil)
	}
	if req.Scope == templateScopeCompany && !isCompanyCreatableTemplateCategory(req.TemplateCategory) {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "company scope only supports custom template category", nil)
	}
	ApplyTemplateFlatBlockSync(&req, s.idg)
	if err := ResolveLegalBasisWrite(ctx, &req, s.legalBasisStructuredWriteEnabled, s.idg); err != nil {
		return nil, err
	}
	syncLegalBasisBlockDescriptionFromFlat(&req)
	HydrateTemplateBlocksBilingualForPersistence(req.Blocks)
	validateFn := validateTemplateMatrix
	if req.Scope == templateScopeGlobal {
		validateFn = validatePortalTemplateMatrix
	}
	if err := validateFn(&req); err != nil {
		return nil, err
	}
	if err := validatePortalDeadlineRule(req.DeadlineRule, s.loadDeadlineRuleCatalog(ctx)); err != nil {
		return nil, err
	}
	req.DisplayGroupCodes = normalizeDisplayGroupCodes(req.DisplayGroupCodes)
	if len(req.DisplayGroupCodes) == 0 {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "display_group_codes is required (at least one Portal catalog group)", nil)
	}
	if err := validateDisplayGroupCodesExist(ctx, s.repo, req.DisplayGroupCodes); err != nil {
		return nil, err
	}
	if req.Scope == templateScopeGlobal {
		isPeriodic := strings.EqualFold(req.TemplateCategory, TemplateCategoryPeriodic)
		if err := applicability.ValidateRules(req.ApplicabilityRules, isPeriodic); err != nil {
			return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, err.Error(), nil)
		}
	}
	resp, err := s.repo.UpsertTypeVersion(ctx, req)
	if err != nil {
		return nil, mapRepositoryUpsertError(err)
	}
	return resp, nil
}

func normalizeDisplayGroupCodes(codes []string) []string {
	if len(codes) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(codes))
	out := make([]string, 0, len(codes))
	for _, code := range codes {
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		out = append(out, code)
	}
	return out
}

func validateDisplayGroupCodesExist(ctx context.Context, repo Repository, codes []string) error {
	catalog, err := repo.ListDisplayGroups(ctx)
	if err != nil {
		return err
	}
	allowed := make(map[string]struct{}, len(catalog))
	for _, item := range catalog {
		allowed[strings.TrimSpace(item.DisplayGroupCode)] = struct{}{}
	}
	for _, code := range codes {
		if _, ok := allowed[code]; !ok {
			return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid display_group_code: "+code, nil)
		}
	}
	return nil
}

func isCompanyCreatableTemplateCategory(category string) bool {
	return strings.EqualFold(strings.TrimSpace(category), TemplateCategoryCustom)
}

func (s *service) ListTypeVersions(ctx context.Context, req ListTypeVersionsRequest) (*ListTypeVersionsResponse, error) {
	if err := s.requireCMSTemplateRead(ctx, req.Subject); err != nil {
		return nil, err
	}
	req.TypeID = strings.TrimSpace(req.TypeID)
	if req.TypeID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "type_id is required", nil)
	}
	items, err := s.repo.ListTypeVersions(ctx, req.Subject.CompanyID, req.TypeID)
	if err != nil {
		return nil, err
	}
	return &ListTypeVersionsResponse{Items: items}, nil
}

func (s *service) ActivateTypeVersion(ctx context.Context, req ActivateTypeVersionRequest) (*ActivateTypeVersionResponse, error) {
	if err := s.requireCMSTemplateActivate(ctx, req.Subject); err != nil {
		return nil, err
	}
	req.TypeID = strings.TrimSpace(req.TypeID)
	req.Reason = strings.TrimSpace(req.Reason)
	if req.TypeID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "type_id is required", nil)
	}
	if req.VersionNo <= 0 {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "version_no must be > 0", nil)
	}
	versionDetail, err := s.repo.GetTypeVersionDetail(ctx, req.Subject.CompanyID, req.TypeID, req.VersionNo)
	if err != nil {
		return nil, err
	}
	if err := ValidateTemplateWorkflowForActivation(versionDetail.Blocks); err != nil {
		return nil, &perr.HTTPError{
			Code:       "TEMPLATE_NO_WORKFLOW",
			Message:    err.Error(),
			HTTPStatus: http.StatusUnprocessableEntity,
			Details: map[string]any{
				"type_id":      req.TypeID,
				"version_no":   req.VersionNo,
				"field_errors": map[string]string{"enterprise_workflow": err.Error()},
			},
		}
	}
	if err := validatePortalDeadlineRule(versionDetail.DeadlineRule, s.loadDeadlineRuleCatalog(ctx)); err != nil {
		return nil, err
	}
	if versionDetail.Scope == templateScopeGlobal || versionDetail.Scope == "" {
		isPeriodic := strings.EqualFold(versionDetail.TemplateCategory, TemplateCategoryPeriodic)
		if err := applicability.ValidateRules(versionDetail.ApplicabilityRules, isPeriodic); err != nil {
			return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, err.Error(), nil)
		}
	}
	return s.repo.ActivateTypeVersion(ctx, req)
}

func (s *service) GetCompanyWorkflowOverride(ctx context.Context, req GetCompanyWorkflowOverrideRequest) (*GetCompanyWorkflowOverrideResponse, error) {
	req.TypeID = strings.TrimSpace(req.TypeID)
	if req.TypeID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "type_id is required", nil)
	}
	if err := s.authorize(ctx, req.Subject, "template.workflow.override.read", authapp.ResourceRef{
		Type: "disclosure_type",
		ID:   req.TypeID,
	}); err != nil {
		return nil, err
	}
	view, err := s.repo.GetCompanyWorkflowOverride(ctx, req.Subject.CompanyID, req.TypeID)
	if err != nil {
		return nil, err
	}
	decorateWorkflowOverrideViewEtags(view)
	return &GetCompanyWorkflowOverrideResponse{Data: *view}, nil
}

func decorateWorkflowOverrideViewEtags(view *CompanyWorkflowOverrideViewDTO) {
	if view == nil {
		return
	}
	if view.DraftVersion != nil {
		view.DraftVersion.DraftEtag = WorkflowDraftEtagFromVersion(view.DraftVersion.VersionNo)
	}
	if view.ActiveVersion != nil && view.ActiveVersion.DraftEtag == "" {
		view.ActiveVersion.DraftEtag = WorkflowDraftEtagFromVersion(view.ActiveVersion.VersionNo)
	}
}

func (s *service) UpsertCompanyWorkflowOverrideDraft(ctx context.Context, req UpsertCompanyWorkflowOverrideDraftRequest) (*UpsertCompanyWorkflowOverrideDraftResponse, error) {
	req.TypeID = strings.TrimSpace(req.TypeID)
	req.ChangeNote = strings.TrimSpace(req.ChangeNote)
	if req.BaseVersionNo <= 0 {
		req.BaseVersionNo = ResolveWorkflowBaseVersionNo(req.BaseVersionNo, req.BaseEtag)
	}
	if req.TypeID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "type_id is required", nil)
	}
	if err := ValidateCompanyWorkflowOverrideSteps(req.Workflow); err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, req.Subject, "template.workflow.override.write", authapp.ResourceRef{
		Type: "disclosure_type",
		ID:   req.TypeID,
	}); err != nil {
		return nil, err
	}
	for i := range req.Workflow {
		req.Workflow[i].DepartmentID = strings.TrimSpace(req.Workflow[i].DepartmentID)
		req.Workflow[i].DueRule = strings.TrimSpace(req.Workflow[i].DueRule)
		// Auto-fill active groups when flag is on, step has a department, and
		// groups field was absent in the request (nil != explicit empty slice []).
		if s.workflowGroupsEnabled && req.Workflow[i].DepartmentID != "" && req.Workflow[i].Groups == nil {
			isActive := true
			fetched, ferr := s.repo.ListCompanyGroups(ctx, req.Subject.CompanyID, req.Workflow[i].DepartmentID, &isActive)
			if ferr != nil {
				return nil, ferr
			}
			filled := make([]WorkflowStepGroupDTO, 0, len(fetched))
			for j, g := range fetched {
				filled = append(filled, WorkflowStepGroupDTO{
					GroupID:        g.GroupID,
					GroupName:      g.GroupName,
					DepartmentID:   g.DepartmentID,
					DepartmentName: g.DepartmentName,
					Source:         "auto_fill",
					DurationMode:   "inherit",
					DisplayOrder:   j + 1,
					IsActive:       g.IsActive,
				})
			}
			req.Workflow[i].Groups = filled
		}
	}
	resp, err := s.repo.UpsertCompanyWorkflowOverrideDraft(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp != nil {
		resp.DraftEtag = WorkflowDraftEtagFromVersion(resp.DraftVersionNo)
		resp.VersionNo = resp.DraftVersionNo
		resp.Workflow = req.Workflow
	}
	if req.Publish && resp != nil {
		if err2 := s.authorize(ctx, req.Subject, "template.workflow.override.approve", authapp.ResourceRef{
			Type: "disclosure_type",
			ID:   req.TypeID,
		}); err2 != nil {
			return nil, err2
		}
		if _, err2 := s.repo.ApproveCompanyWorkflowOverride(ctx, ApproveCompanyWorkflowOverrideRequest{
			Subject:               req.Subject,
			TypeID:                req.TypeID,
			VersionNo:             resp.DraftVersionNo,
			Reason:                "apply",
			SkipSelfApprovalCheck: true,
		}); err2 != nil {
			return nil, err2
		}
		resp.State = "approved"
	}
	return resp, nil
}

func (s *service) ApproveCompanyWorkflowOverride(ctx context.Context, req ApproveCompanyWorkflowOverrideRequest) (*ApproveCompanyWorkflowOverrideResponse, error) {
	req.TypeID = strings.TrimSpace(req.TypeID)
	req.Reason = strings.TrimSpace(req.Reason)
	req.BaseEtag = strings.TrimSpace(req.BaseEtag)
	if req.TypeID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "type_id is required", nil)
	}
	// Normalize version from base_etag if version_no not provided directly.
	if req.VersionNo <= 0 {
		req.VersionNo = ResolveWorkflowBaseVersionNo(req.VersionNo, req.BaseEtag)
	}
	if req.VersionNo <= 0 {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "version_no or base_etag is required", nil)
	}
	if err := s.authorize(ctx, req.Subject, "template.workflow.override.approve", authapp.ResourceRef{
		Type: "disclosure_type",
		ID:   req.TypeID,
	}); err != nil {
		return nil, err
	}
	view, err := s.repo.GetCompanyWorkflowOverride(ctx, req.Subject.CompanyID, req.TypeID)
	if err != nil {
		return nil, err
	}
	if view.DraftVersion == nil || view.DraftVersion.VersionNo != req.VersionNo {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "workflow override draft version not found", nil)
	}
	if err := ValidateCompanyWorkflowOverrideSteps(view.DraftVersion.Workflow); err != nil {
		return nil, err
	}
	return s.repo.ApproveCompanyWorkflowOverride(ctx, req)
}

func (s *service) DeleteCompanyWorkflowOverrideDraft(ctx context.Context, req DeleteCompanyWorkflowOverrideDraftRequest) (*DeleteCompanyWorkflowOverrideDraftResponse, error) {
	req.TypeID = strings.TrimSpace(req.TypeID)
	if req.TypeID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "type_id is required", nil)
	}
	if req.VersionNo <= 0 {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "version_no must be > 0", nil)
	}
	if err := s.authorize(ctx, req.Subject, "template.workflow.override.write", authapp.ResourceRef{
		Type: "disclosure_type",
		ID:   req.TypeID,
	}); err != nil {
		return nil, err
	}
	return s.repo.DeleteCompanyWorkflowOverrideDraft(ctx, req)
}

func (s *service) ResetCompanyWorkflowOverrideActive(ctx context.Context, req ResetCompanyWorkflowOverrideActiveRequest) (*ResetCompanyWorkflowOverrideActiveResponse, error) {
	req.TypeID = strings.TrimSpace(req.TypeID)
	req.Reason = strings.TrimSpace(req.Reason)
	if req.TypeID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "type_id is required", nil)
	}
	if err := s.authorize(ctx, req.Subject, "template.workflow.override.reset", authapp.ResourceRef{
		Type: "disclosure_type",
		ID:   req.TypeID,
	}); err != nil {
		return nil, err
	}
	view, err := s.repo.GetCompanyWorkflowOverride(ctx, req.Subject.CompanyID, req.TypeID)
	if err != nil {
		return nil, err
	}
	if view.EffectiveSource == "global_template" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "no active override to reset — effective source is already global template", nil)
	}
	return s.repo.ResetCompanyWorkflowOverrideActive(ctx, req)
}

func (s *service) ListCompanyWorkflowOverrideVersions(ctx context.Context, req ListCompanyWorkflowOverrideVersionsRequest) (*ListCompanyWorkflowOverrideVersionsResponse, error) {
	req.TypeID = strings.TrimSpace(req.TypeID)
	if req.TypeID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "type_id is required", nil)
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 || req.PageSize > 100 {
		req.PageSize = 20
	}
	if err := s.authorize(ctx, req.Subject, "template.workflow.override.read", authapp.ResourceRef{
		Type: "disclosure_type",
		ID:   req.TypeID,
	}); err != nil {
		return nil, err
	}
	items, total, err := s.repo.ListCompanyWorkflowOverrideVersions(ctx, req.Subject.CompanyID, req.TypeID, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}
	resp := &ListCompanyWorkflowOverrideVersionsResponse{Items: items}
	resp.Meta.Page = req.Page
	resp.Meta.PageSize = req.PageSize
	resp.Meta.Total = total
	return resp, nil
}

func (s *service) GetCompanyWorkflowOverrideDraftReminderPreview(
	ctx context.Context,
	req GetCompanyWorkflowOverrideDraftReminderPreviewRequest,
) (*GetCompanyWorkflowOverrideDraftReminderPreviewResponse, error) {
	req.TypeID = strings.TrimSpace(req.TypeID)
	if req.TypeID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "type_id is required", nil)
	}
	if err := s.authorize(ctx, req.Subject, "template.workflow.override.read", authapp.ResourceRef{
		Type: "disclosure_type",
		ID:   req.TypeID,
	}); err != nil {
		return nil, err
	}
	view, err := s.repo.GetCompanyWorkflowOverride(ctx, req.Subject.CompanyID, req.TypeID)
	if err != nil {
		return nil, err
	}
	decorateWorkflowOverrideViewEtags(view)
	if view.DraftVersion == nil || len(view.DraftVersion.Workflow) == 0 {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "workflow override draft is required for reminder preview", nil)
	}
	steps := append([]WorkflowStepDTO(nil), view.DraftVersion.Workflow...)
	sortWorkflowSteps(steps)

	loc := CompanyLocation(defaultCompanyTimezone)
	t0Local := time.Now().In(loc)
	if raw := strings.TrimSpace(req.T0Date); raw != "" {
		parsed, parseErr := time.ParseInLocation("2006-01-02", raw, loc)
		if parseErr != nil {
			return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "t0 must be YYYY-MM-DD", parseErr)
		}
		t0Local = parsed
	}
	t0Local = time.Date(t0Local.Year(), t0Local.Month(), t0Local.Day(), 0, 0, 0, 0, loc)

	typeDefaultDays := 1
	if _, cfg, cfgErr := s.repo.GetActiveVersionDeadlineConfig(ctx, req.TypeID); cfgErr == nil && cfg != nil {
		if cfg.StepDefaultSlaDays > 0 {
			typeDefaultDays = cfg.StepDefaultSlaDays
		} else if cfg.ProcessingDays > 0 {
			typeDefaultDays = cfg.ProcessingDays // legacy fallback
		}
	}

	timelines, err := ComputeStepTimelines(t0Local, defaultCompanyTimezone, steps, typeDefaultDays)
	if err != nil {
		return nil, err
	}
	cfgByStep := make(map[string]*WorkflowStepReminderConfig, len(steps))
	for i := range steps {
		cfgByStep[steps[i].StepID] = steps[i].ReminderConfig
	}
	previewInstanceID := "preview-" + req.TypeID
	milestones := make([]WorkflowOverrideReminderPreviewMilestoneDTO, 0, len(timelines)*4)
	for _, tl := range timelines {
		for _, row := range GenerateConfiguredReminderPreviewCandidates(tl, t0Local, req.Subject.CompanyID, previewInstanceID, cfgByStep[tl.StepID], s.idg.NewUUID) {
			milestones = append(milestones, WorkflowOverrideReminderPreviewMilestoneDTO{
				StepID:        row.StepID,
				StepOrder:     row.StepOrder,
				MilestoneType: string(row.MilestoneType),
				ScheduledDate: row.ScheduledDate.Format("2006-01-02"),
			})
		}
	}
	source := view.EffectiveSource
	if source == "" {
		source = "company_override"
	}
	return &GetCompanyWorkflowOverrideDraftReminderPreviewResponse{
		Data: WorkflowOverrideReminderPreviewDTO{
			TypeID:     req.TypeID,
			CompanyID:  req.Subject.CompanyID,
			T0Date:     t0Local.Format("2006-01-02"),
			Timezone:   defaultCompanyTimezone,
			Source:     source,
			Milestones: milestones,
		},
	}, nil
}

func sortWorkflowSteps(steps []WorkflowStepDTO) {
	for i := 0; i < len(steps); i++ {
		for j := i + 1; j < len(steps); j++ {
			left := steps[i].DisplayOrder
			right := steps[j].DisplayOrder
			if left <= 0 {
				left = i + 1
			}
			if right <= 0 {
				right = j + 1
			}
			if right < left {
				steps[i], steps[j] = steps[j], steps[i]
			}
		}
	}
}

func (s *service) GetEffectiveWorkflow(ctx context.Context, req GetEffectiveWorkflowRequest) (*GetEffectiveWorkflowResponse, error) {
	req.TypeID = strings.TrimSpace(req.TypeID)
	if req.TypeID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "type_id is required", nil)
	}
	if err := s.requireDisclosureCatalogRead(ctx, req.Subject); err != nil {
		return nil, err
	}
	out, err := s.repo.GetEffectiveWorkflow(ctx, req.Subject.CompanyID, req.TypeID)
	if err != nil {
		return nil, err
	}
	return &GetEffectiveWorkflowResponse{Data: *out}, nil
}

func (s *service) GetTemplateDeadlineConfig(ctx context.Context, req GetTemplateDeadlineConfigRequest) (*GetTemplateDeadlineConfigResponse, error) {
	req.TypeID = strings.TrimSpace(req.TypeID)
	if req.TypeID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "type_id is required", nil)
	}
	if err := s.requireCMSTemplateRead(ctx, req.Subject); err != nil {
		return nil, err
	}
	versionNo, cfg, err := s.repo.GetActiveVersionDeadlineConfig(ctx, req.TypeID)
	if err != nil {
		return nil, err
	}
	out := &GetTemplateDeadlineConfigResponse{TypeID: req.TypeID, VersionNo: versionNo}
	if cfg != nil {
		out.DeadlineConfig = *cfg
	}
	return out, nil
}

func (s *service) UpdateTemplateDeadlineConfig(ctx context.Context, req UpdateTemplateDeadlineConfigRequest) (*UpdateTemplateDeadlineConfigResponse, error) {
	req.TypeID = strings.TrimSpace(req.TypeID)
	if req.TypeID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "type_id is required", nil)
	}
	if err := s.requireCMSTemplateConfigWrite(ctx, req.Subject); err != nil {
		return nil, err
	}
	if req.DeadlineConfig.T0Policy != "" {
		switch req.DeadlineConfig.T0Policy {
		case "system_date", "event_date", "user_defined":
		default:
			return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "t0_policy must be system_date | event_date | user_defined", nil)
		}
	}
	if req.DeadlineConfig.DeadlineDays < 0 {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "deadline_days must be >= 0", nil)
	}
	if req.DeadlineConfig.ProcessingDays < 0 {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "processing_days must be >= 0", nil)
	}
	if err := s.repo.UpdateActiveVersionDeadlineConfig(ctx, req.TypeID, req.DeadlineConfig, req.Subject.UserID); err != nil {
		return nil, err
	}
	versionNo, cfg, err := s.repo.GetActiveVersionDeadlineConfig(ctx, req.TypeID)
	if err != nil {
		return nil, err
	}
	out := &UpdateTemplateDeadlineConfigResponse{TypeID: req.TypeID, VersionNo: versionNo, UpdatedBy: req.Subject.UserID}
	if cfg != nil {
		out.DeadlineConfig = *cfg
	}
	return out, nil
}

func (s *service) requireDisclosureCatalogRead(ctx context.Context, sub Subject) error {
	if err := s.authorize(ctx, sub, "disclosure.create", authapp.ResourceRef{
		Type: "disclosure_record",
		ID:   "",
		Attributes: map[string]any{
			"department_id":       "general",
			"owner_membership_id": sub.MembershipID,
			"workflow_state":      "draft",
		},
	}); err != nil {
		return err
	}
	return nil
}

func (s *service) hasPermission(ctx context.Context, sub Subject, permission string) bool {
	if s.auth == nil {
		return true
	}
	eff, err := s.auth.GetEffectiveAccess(ctx, sub.MembershipID, sub.CompanyID)
	if err != nil {
		return false
	}
	for _, p := range eff.Permissions {
		if p == permission {
			return true
		}
	}
	return false
}

func (s *service) ListCompanyGroups(ctx context.Context, req ListCompanyGroupsRequest) (*ListCompanyGroupsResponse, error) {
	if !s.workflowGroupsEnabled {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodeFeatureNotEnabled, "workflow groups feature is not enabled", nil)
	}
	if err := s.authorize(ctx, req.Subject, "template.workflow.override.read", authapp.ResourceRef{
		Type: "company",
		ID:   req.Subject.CompanyID,
	}); err != nil {
		return nil, err
	}
	items, err := s.repo.ListCompanyGroups(ctx, req.Subject.CompanyID, req.DepartmentID, req.IsActive)
	if err != nil {
		return nil, err
	}
	return &ListCompanyGroupsResponse{Items: items}, nil
}

func (s *service) UpdateWorkflowOverrideStepGroups(ctx context.Context, req UpdateWorkflowOverrideStepGroupsRequest) (*UpdateWorkflowOverrideStepGroupsResponse, error) {
	if !s.workflowGroupsEnabled {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodeFeatureNotEnabled, "workflow groups feature is not enabled", nil)
	}
	req.TypeID = strings.TrimSpace(req.TypeID)
	req.StepID = strings.TrimSpace(req.StepID)
	req.BaseEtag = strings.TrimSpace(req.BaseEtag)
	if req.TypeID == "" || req.StepID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "type_id and step_id are required", nil)
	}
	if req.BaseEtag == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "base_etag is required", nil)
	}
	if err := s.authorize(ctx, req.Subject, "template.workflow.override.write", authapp.ResourceRef{
		Type: "disclosure_type",
		ID:   req.TypeID,
	}); err != nil {
		return nil, err
	}
	for i, g := range req.Groups {
		if g.DurationMode != "inherit" && g.DurationMode != "custom" {
			return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest,
				fmt.Sprintf("groups[%d].duration_mode must be inherit or custom", i), nil)
		}
		if g.DurationMode == "custom" && g.ProcessingDays == nil {
			return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest,
				fmt.Sprintf("groups[%d].processing_days is required when duration_mode=custom", i), nil)
		}
	}
	return s.repo.UpdateWorkflowOverrideStepGroups(ctx, req)
}

func normalizeDate(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if _, err := time.Parse("2006-01-02", raw); err != nil {
		return "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "planned_date must be YYYY-MM-DD", nil)
	}
	return raw, nil
}

func sanitizeAttachments(items []AttachmentDTO) []AttachmentDTO {
	if len(items) == 0 {
		return []AttachmentDTO{}
	}
	out := make([]AttachmentDTO, 0, len(items))
	for _, it := range items {
		name := strings.TrimSpace(it.Name)
		if name == "" {
			continue
		}
		out = append(out, AttachmentDTO{
			ID:         strings.TrimSpace(it.ID),
			Name:       name,
			Type:       strings.TrimSpace(it.Type),
			UploadedAt: strings.TrimSpace(it.UploadedAt),
		})
	}
	return out
}

func sanitizeStringList(items []string) []string {
	if len(items) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func sanitizeLegalBases(items []LegalBasisDTO) []LegalBasisDTO {
	if len(items) == 0 {
		return []LegalBasisDTO{}
	}
	out := make([]LegalBasisDTO, 0, len(items))
	for _, item := range items {
		title := strings.TrimSpace(item.Title)
		summary := strings.TrimSpace(item.Summary)
		if title == "" && summary == "" {
			continue
		}
		out = append(out, LegalBasisDTO{
			ID:        strings.TrimSpace(item.ID),
			Title:     title,
			Code:      strings.TrimSpace(item.Code),
			Authority: strings.TrimSpace(item.Authority),
			IssueDate: strings.TrimSpace(item.IssueDate),
			Summary:   summary,
			Link:      strings.TrimSpace(item.Link),
		})
	}
	return out
}

func sanitizeChecklist(items []ChecklistItemDTO) []ChecklistItemDTO {
	if len(items) == 0 {
		return []ChecklistItemDTO{}
	}
	out := make([]ChecklistItemDTO, 0, len(items))
	for _, item := range items {
		title := strings.TrimSpace(item.Title)
		if title == "" {
			continue
		}
		status := strings.TrimSpace(item.Status)
		if status == "" {
			status = "Pending"
		}
		out = append(out, ChecklistItemDTO{
			ID:      strings.TrimSpace(item.ID),
			Title:   title,
			Owner:   strings.TrimSpace(item.Owner),
			DueDate: strings.TrimSpace(item.DueDate),
			Status:  status,
		})
	}
	return out
}

// SeedPeriodicCycles computes expected cycles for the current tick and upserts them.
// Idempotent — safe to call on every worker tick.
func (s *service) SeedPeriodicCycles(ctx context.Context, now time.Time) (int, error) {
	return seedPeriodicCycles(ctx, now, s.repo, s.idg, s.calculator, s.templateApplicabilityStrictFilter, s.shadowRunner)
}

// MaterializePeriodicDisclosures picks up pending cycles and creates disclosure records.
func (s *service) MaterializePeriodicDisclosures(ctx context.Context, now time.Time, creator PeriodicRecordCreator) (int, error) {
	return materializePeriodicDisclosures(ctx, now, s.repo, creator)
}

// GetCompanyTypePreference returns auto_create preference for a (company, type) pair.
// Defaults to enabled when no row exists.
func (s *service) GetCompanyTypePreference(ctx context.Context, req GetCompanyTypePreferenceRequest) (*CompanyTypePreferenceDTO, error) {
	if err := s.authorize(ctx, req.Subject, "disclosure.view", authapp.ResourceRef{
		Type: "disclosure_type",
		ID:   req.TypeID,
	}); err != nil {
		return nil, err
	}
	pref, err := s.repo.GetCompanyTypePreference(ctx, req.Subject.CompanyID, req.TypeID)
	if err != nil {
		return nil, err
	}
	return companyTypePreferenceDTO(req.Subject.CompanyID, req.TypeID, pref), nil
}

// UpsertCompanyTypePreference sets auto_create_enabled for a (company, type) pair.
func (s *service) UpsertCompanyTypePreference(ctx context.Context, req UpsertCompanyTypePreferenceRequest) (*CompanyTypePreferenceDTO, error) {
	if err := s.authorize(ctx, req.Subject, "disclosure.auto_create.manage", authapp.ResourceRef{
		Type: "disclosure_type",
		ID:   req.TypeID,
	}); err != nil {
		return nil, err
	}
	if err := s.repo.UpsertCompanyTypePreference(ctx, CompanyTypePreference{
		CompanyID:         req.Subject.CompanyID,
		TypeID:            req.TypeID,
		AutoCreateEnabled: req.AutoCreateEnabled,
		UpdatedBy:         req.Subject.MembershipID,
		CycleAnchorMonth:  req.CycleAnchorMonth,
		CycleAnchorDay:    req.CycleAnchorDay,
	}); err != nil {
		return nil, err
	}
	return companyTypePreferenceDTO(req.Subject.CompanyID, req.TypeID, &CompanyTypePreference{
		CompanyID:         req.Subject.CompanyID,
		TypeID:            req.TypeID,
		AutoCreateEnabled: req.AutoCreateEnabled,
		CycleAnchorMonth:  req.CycleAnchorMonth,
		CycleAnchorDay:    req.CycleAnchorDay,
	}), nil
}

func companyTypePreferenceDTO(companyID, typeID string, pref *CompanyTypePreference) *CompanyTypePreferenceDTO {
	dto := &CompanyTypePreferenceDTO{
		TypeID:            typeID,
		CompanyID:         companyID,
		AutoCreateEnabled: true, // default when no row
	}
	if pref != nil {
		dto.AutoCreateEnabled = pref.AutoCreateEnabled
		dto.CycleAnchorMonth = pref.CycleAnchorMonth
		dto.CycleAnchorDay = pref.CycleAnchorDay
	}
	return dto
}

// companyTemplateLifecycleTransitions defines valid (from → to) pairs.
var companyTemplateLifecycleTransitions = map[string]string{
	"submit-review": "in_review",
	"publish":       "published",
	"reject":        "rejected",
	"archive":       "archived",
}

// validFromStatus lists states from which each action is allowed.
var companyTemplateActionFromStatus = map[string]string{
	"submit-review": "draft",
	"publish":       "in_review",
	"reject":        "in_review",
	"archive":       "published",
}

func (s *service) CreateCompanyTemplate(ctx context.Context, req CreateCompanyTemplateRequest) (*CompanyTemplateWriteResponse, error) {
	if req.Subject.CompanyID == "" {
		return nil, perr.NewHTTPError(http.StatusUnauthorized, perr.CodeCompanyContextRequired, "company context required", nil)
	}
	if !s.hasPermission(ctx, req.Subject, permissionLegacyTemplateManage) {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "access denied", nil)
	}
	req.Name = strings.TrimSpace(req.Name)
	req.TemplateCategory = strings.ToLower(strings.TrimSpace(req.TemplateCategory))
	if req.Name == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "name is required", nil)
	}
	if err := validatePortalTemplateMatrix(&UpsertTypeVersionRequest{
		Name:             req.Name,
		TemplateCategory: req.TemplateCategory,
		DeadlineRule:     strings.TrimSpace(req.DeadlineRule),
		Periodicity:      strings.TrimSpace(req.Periodicity),
	}); err != nil {
		return nil, err
	}
	if err := validatePortalDeadlineRule(req.DeadlineRule, s.loadDeadlineRuleCatalog(ctx)); err != nil {
		return nil, err
	}
	if s.tierLookup != nil {
		tier := s.tierLookup(ctx, req.Subject.UserID)
		limit := companyTemplateQuotaLimit(tier)
		count, err := s.repo.CountCompanyTemplatesByCompanyID(ctx, req.Subject.CompanyID)
		if err == nil && count >= limit {
			return nil, perr.NewHTTPError(http.StatusPaymentRequired, "QUOTA_EXCEEDED",
				fmt.Sprintf("subscription quota exceeded: company has %d of %d allowed templates", count, limit), nil)
		}
	}
	return s.repo.CreateCompanyTemplate(ctx, req)
}

func (s *service) UpdateCompanyTemplate(ctx context.Context, req UpdateCompanyTemplateRequest) (*CompanyTemplateWriteResponse, error) {
	if req.Subject.CompanyID == "" {
		return nil, perr.NewHTTPError(http.StatusUnauthorized, perr.CodeCompanyContextRequired, "company context required", nil)
	}
	if !s.hasPermission(ctx, req.Subject, permissionLegacyTemplateManage) {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "access denied", nil)
	}
	req.TypeID = strings.TrimSpace(req.TypeID)
	req.Name = strings.TrimSpace(req.Name)
	req.TemplateCategory = strings.ToLower(strings.TrimSpace(req.TemplateCategory))
	if req.TypeID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "type_id is required", nil)
	}
	if req.Name == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "name is required", nil)
	}
	current, err := s.repo.GetCompanyTemplateForLifecycle(ctx, req.Subject.CompanyID, req.TypeID)
	if err != nil {
		return nil, err
	}
	effectiveCategory := req.TemplateCategory
	if effectiveCategory == "" {
		effectiveCategory = current.TemplateCategory
	}
	effectiveDeadlineRule := strings.TrimSpace(req.DeadlineRule)
	if effectiveDeadlineRule == "" {
		effectiveDeadlineRule = strings.TrimSpace(current.DeadlineRule)
	}
	effectivePeriodicity := strings.TrimSpace(req.Periodicity)
	if effectivePeriodicity == "" {
		effectivePeriodicity = strings.TrimSpace(current.Periodicity)
	}
	if err := validatePortalTemplateMatrix(&UpsertTypeVersionRequest{
		Name:             req.Name,
		TemplateCategory: effectiveCategory,
		DeadlineRule:     effectiveDeadlineRule,
		Periodicity:      effectivePeriodicity,
	}); err != nil {
		return nil, err
	}
	if err := validatePortalDeadlineRule(effectiveDeadlineRule, s.loadDeadlineRuleCatalog(ctx)); err != nil {
		return nil, err
	}
	if current.ReviewStatus != "draft" && current.ReviewStatus != "rejected" {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "template can only be edited in draft or rejected status", nil)
	}
	return s.repo.UpdateCompanyTemplate(ctx, req)
}

// companyTemplateLifecycleCapability returns the required capability for a lifecycle action.
// submit-review is a maker action (disclosure_type.manage).
// publish, reject, archive are checker actions (disclosure_type.publish) — enforcing maker-checker separation.
func companyTemplateLifecycleCapability(action string) string {
	switch action {
	case "publish", "reject", "archive":
		return "disclosure_type.publish"
	default:
		return "disclosure_type.manage"
	}
}

func (s *service) TransitionCompanyTemplateLifecycle(ctx context.Context, req TransitionCompanyTemplateLifecycleRequest) (*CompanyTemplateWriteResponse, error) {
	if req.Subject.CompanyID == "" {
		return nil, perr.NewHTTPError(http.StatusUnauthorized, perr.CodeCompanyContextRequired, "company context required", nil)
	}
	req.TypeID = strings.TrimSpace(req.TypeID)
	req.Action = strings.ToLower(strings.TrimSpace(req.Action))
	if !s.hasPermission(ctx, req.Subject, companyTemplateLifecycleCapability(req.Action)) {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "access denied", nil)
	}
	newStatus, valid := companyTemplateLifecycleTransitions[req.Action]
	if !valid {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "unknown lifecycle action", nil)
	}
	current, err := s.repo.GetCompanyTemplateForLifecycle(ctx, req.Subject.CompanyID, req.TypeID)
	if err != nil {
		return nil, err
	}
	requiredFrom := companyTemplateActionFromStatus[req.Action]
	if current.ReviewStatus != requiredFrom {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict,
			fmt.Sprintf("action %q requires status %q; current status is %q", req.Action, requiredFrom, current.ReviewStatus), nil)
	}
	if err := s.repo.TransitionCompanyTemplateReviewStatus(ctx, req.Subject.CompanyID, req.TypeID, newStatus, req.Subject.MembershipID); err != nil {
		return nil, err
	}
	return s.repo.GetCompanyTemplateForLifecycle(ctx, req.Subject.CompanyID, req.TypeID)
}

// enforceHasWorkflowGate aligns with Portal FE has_workflow (PO D5-A):
// active version must have ≥1 step in enterprise_workflow block.
// global_workflows does not satisfy the gate (PO D6-B).
func (s *service) enforceHasWorkflowGate(ctx context.Context, companyID, typeID string) error {
	hasWorkflow, err := s.repo.HasActiveEnterpriseWorkflow(ctx, companyID, typeID)
	if err != nil {
		// Workflow lookup failure must not silently block disclosure creation.
		return nil
	}
	if !hasWorkflow {
		return perr.NewHTTPError(http.StatusUnprocessableEntity, "TEMPLATE_NO_WORKFLOW",
			"template has no active enterprise workflow; disclosure cannot be created", nil)
	}
	return nil
}

func (s *service) authorize(ctx context.Context, sub Subject, action string, resource authapp.ResourceRef) error {
	if s.auth == nil {
		// Worker mode: no user-facing requests; trust the caller.
		return nil
	}
	decision, err := s.auth.Authorize(ctx, authapp.AuthorizeRequest{Subject: authapp.SubjectRef{UserID: sub.UserID, MembershipID: sub.MembershipID, CompanyID: sub.CompanyID}, Action: action, Resource: resource})
	if err != nil {
		return fmt.Errorf("authorize disclosure action: %w", err)
	}
	if decision.Decision != authapp.DecisionAllow {
		code := perr.CodePermissionDenied
		if decision.DenyReasonCode != nil {
			code = *decision.DenyReasonCode
		}
		return perr.NewHTTPError(http.StatusForbidden, code, "access denied", nil)
	}
	return nil
}
