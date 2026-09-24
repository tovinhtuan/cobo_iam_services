package app

import (
	"context"
	"net/http"
	"strings"
	"time"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

type PermissionChecker func(ctx context.Context, membershipID, companyID string, permissions ...string) error

type Service struct {
	repo   Repository
	idg    IDGenerator
	clock  Clock
	check  PermissionChecker
}

func NewGlobalRecordsService(repo Repository, idg IDGenerator, clock Clock, check PermissionChecker) *Service {
	if clock == nil {
		clock = systemClock{}
	}
	return &Service{repo: repo, idg: idg, clock: clock, check: check}
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

func (s *Service) require(ctx context.Context, actor Actor, perms ...string) error {
	if s.check == nil {
		return nil
	}
	return s.check(ctx, actor.MembershipID, actor.CompanyID, perms...)
}

func (s *Service) Create(ctx context.Context, actor Actor, templateID string, title, summary, content, cycleKey string) (*GlobalRecord, error) {
	if err := s.require(ctx, actor, PermPlatformCMSView, PermRecordWrite); err != nil {
		return nil, err
	}
	templateID = strings.TrimSpace(templateID)
	cycleKey = strings.TrimSpace(cycleKey)
	title = strings.TrimSpace(title)
	if templateID == "" || title == "" || strings.TrimSpace(content) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "template_id, title, and content are required", nil)
	}
	if !ValidateCycleKey(cycleKey) {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid cycle_key for phase 1", nil)
	}
	tpl, err := s.repo.GetTemplateInfo(ctx, templateID)
	if err != nil {
		return nil, err
	}
	if tpl == nil {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "template not found", nil)
	}
	if strings.EqualFold(tpl.Scope, "company") {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "only global/system templates support Global CMS Records", nil)
	}
	if strings.EqualFold(tpl.PortalState, "archived") || strings.EqualFold(tpl.ReviewStatus, "archived") {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "template is archived", nil)
	}
	if tpl.ActiveVersionNo <= 0 && !strings.EqualFold(tpl.PortalState, "active") {
		// Allow create when portal_state active OR active_version_no > 0
		if strings.TrimSpace(tpl.PortalState) != "" && !strings.EqualFold(tpl.PortalState, "active") {
			return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "template must be active/published before creating Global CMS Records", nil)
		}
	}
	if existing, _ := s.repo.GetByTemplateCycle(ctx, templateID, cycleKey); existing != nil {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "global record already exists for template_id+cycle_key", nil)
	}
	now := s.clock.Now()
	rec := GlobalRecord{
		ID:         s.idg.NewID("cms_rec"),
		TemplateID: templateID,
		CycleKey:   cycleKey,
		Title:      title,
		Summary:    strings.TrimSpace(summary),
		Content:    content,
		Status:     StatusDraft,
		CreatedBy:  actor.UserID,
		UpdatedBy:  actor.UserID,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := s.repo.Create(ctx, rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

func (s *Service) Update(ctx context.Context, actor Actor, id string, title, summary, content string) (*GlobalRecord, error) {
	if err := s.require(ctx, actor, PermPlatformCMSView, PermRecordWrite); err != nil {
		return nil, err
	}
	cur, err := s.repo.GetByID(ctx, strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}
	if cur == nil {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "global record not found", nil)
	}
	if !strings.EqualFold(cur.Status, StatusDraft) {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "only Draft global records can be updated", nil)
	}
	title = strings.TrimSpace(title)
	if title == "" || strings.TrimSpace(content) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "title and content are required", nil)
	}
	cur.Title = title
	cur.Summary = strings.TrimSpace(summary)
	cur.Content = content
	cur.UpdatedBy = actor.UserID
	cur.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, *cur); err != nil {
		return nil, err
	}
	return cur, nil
}

func (s *Service) Get(ctx context.Context, actor Actor, id string) (*GlobalRecord, error) {
	if err := s.require(ctx, actor, PermPlatformCMSView, PermRecordRead); err != nil {
		return nil, err
	}
	rec, err := s.repo.GetByID(ctx, strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}
	if rec == nil {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "global record not found", nil)
	}
	if counts, cErr := s.repo.CountCompaniesByCMSRecord(ctx, rec.ID); cErr == nil {
		rec.CompanyCounts = counts
	}
	return rec, nil
}

func (s *Service) ListByTemplate(ctx context.Context, actor Actor, f ListGlobalRecordsFilter) (ListGlobalRecordsResult, error) {
	if err := s.require(ctx, actor, PermPlatformCMSView, PermRecordRead); err != nil {
		return ListGlobalRecordsResult{}, err
	}
	if f.Page <= 0 {
		f.Page = 1
	}
	if f.PageSize <= 0 {
		f.PageSize = 20
	}
	if f.PageSize > 100 {
		f.PageSize = 100
	}
	res, err := s.repo.ListByTemplate(ctx, f)
	if err != nil {
		return ListGlobalRecordsResult{}, err
	}
	for i := range res.Items {
		if counts, cErr := s.repo.CountCompaniesByCMSRecord(ctx, res.Items[i].ID); cErr == nil {
			res.Items[i].CompanyCounts = counts
		}
	}
	return res, nil
}

func (s *Service) Publish(ctx context.Context, actor Actor, id string) (*GlobalRecord, error) {
	if err := s.require(ctx, actor, PermPlatformCMSView, PermRecordPublish); err != nil {
		return nil, err
	}
	cur, err := s.repo.GetByID(ctx, strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}
	if cur == nil {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "global record not found", nil)
	}
	if strings.EqualFold(cur.Status, StatusPublished) {
		return cur, nil // idempotent 200 no-op
	}
	if strings.EqualFold(cur.Status, StatusArchived) {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "archived global record cannot be published", nil)
	}
	if !strings.EqualFold(cur.Status, StatusDraft) {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "only Draft global records can be published", nil)
	}
	if strings.TrimSpace(cur.Title) == "" || strings.TrimSpace(cur.Content) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "title and content are required before publish", nil)
	}
	tpl, err := s.repo.GetTemplateInfo(ctx, cur.TemplateID)
	if err != nil {
		return nil, err
	}
	if tpl == nil {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "template not found", nil)
	}
	if strings.EqualFold(tpl.PortalState, "archived") || strings.EqualFold(tpl.ReviewStatus, "archived") {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "template is archived", nil)
	}
	now := s.clock.Now()
	cur.Status = StatusPublished
	cur.PublishedBy = actor.UserID
	cur.PublishedAt = &now
	cur.UpdatedBy = actor.UserID
	cur.UpdatedAt = now
	if tpl.ActiveVersionNo > 0 {
		v := tpl.ActiveVersionNo
		cur.TemplateVersionNo = &v
	}
	if err := s.repo.Update(ctx, *cur); err != nil {
		return nil, err
	}
	return cur, nil
}

func (s *Service) Archive(ctx context.Context, actor Actor, id string) (*GlobalRecord, error) {
	if err := s.require(ctx, actor, PermPlatformCMSView, PermRecordPublish); err != nil {
		return nil, err
	}
	cur, err := s.repo.GetByID(ctx, strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}
	if cur == nil {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "global record not found", nil)
	}
	if strings.EqualFold(cur.Status, StatusArchived) {
		return cur, nil
	}
	now := s.clock.Now()
	// Free UNIQUE(template_id, cycle_key) for a new Draft of the same business cycle.
	cur.CycleKey = cur.CycleKey + "#archived:" + cur.ID
	cur.Status = StatusArchived
	cur.ArchivedBy = actor.UserID
	cur.ArchivedAt = &now
	cur.UpdatedBy = actor.UserID
	cur.UpdatedAt = now
	if err := s.repo.Update(ctx, *cur); err != nil {
		return nil, err
	}
	return cur, nil
}

func (s *Service) ListCompanyChildren(ctx context.Context, actor Actor, cmsRecordID, status string, page, pageSize int) ([]CompanyChild, int, error) {
	if err := s.require(ctx, actor, PermPlatformCMSView, PermRecordRead); err != nil {
		return nil, 0, err
	}
	rec, err := s.repo.GetByID(ctx, strings.TrimSpace(cmsRecordID))
	if err != nil {
		return nil, 0, err
	}
	if rec == nil {
		return nil, 0, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "global record not found", nil)
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	return s.repo.ListCompanyChildren(ctx, rec.ID, status, page, pageSize)
}

func (s *Service) Materialize(ctx context.Context, actor Actor, cmsRecordID string, req MaterializeRequest) (*MaterializeResult, error) {
	if err := s.require(ctx, actor, PermPlatformCMSView, PermRecordMaterialize); err != nil {
		return nil, err
	}
	rec, err := s.repo.GetByID(ctx, strings.TrimSpace(cmsRecordID))
	if err != nil {
		return nil, err
	}
	if rec == nil {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "global record not found", nil)
	}
	if !strings.EqualFold(rec.Status, StatusPublished) {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "only Published global records can be materialized", nil)
	}
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if mode == "" {
		mode = "full"
	}
	if mode != "full" && mode != "incremental" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "mode must be full or incremental", nil)
	}

	decisions, err := s.repo.EvaluateEligibility(ctx, rec.TemplateID, displayCycleKey(rec.CycleKey), rec.ID)
	if err != nil {
		return nil, err
	}

	requestedFilter := map[string]struct{}{}
	filterRequested := len(req.CompanyIDs) > 0
	for _, id := range req.CompanyIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			requestedFilter[id] = struct{}{}
		}
	}

	runID := s.idg.NewID("cms_mat")
	out := &MaterializeResult{
		RunID:       runID,
		DryRun:      req.DryRun,
		Mode:        mode,
		Status:      "completed",
		Results:     make([]MaterializeItemResult, 0, len(decisions)),
		Eligibility: make([]EligibilityDecision, 0, len(decisions)),
	}

	for _, d := range decisions {
		if filterRequested {
			if _, ok := requestedFilter[d.CompanyID]; !ok {
				continue
			}
		}
		out.Eligibility = append(out.Eligibility, d)
		out.Requested++

		item := MaterializeItemResult{
			CompanyID:   d.CompanyID,
			CompanyName: d.CompanyName,
			ReasonCode:  d.ReasonCode,
		}

		if d.AlreadyMaterialized {
			item.Outcome = "exists"
			item.CompanyRecordID = d.ExistingCompanyRecID
			item.ReasonCode = ReasonAlreadyMaterialized
			item.ErrorMessage = ReasonMessageVI(ReasonAlreadyMaterialized)
			out.Exists++
			out.Results = append(out.Results, item)
			continue
		}
		if !d.Eligible {
			item.Outcome = "skipped"
			item.ErrorCode = d.ReasonCode
			item.ErrorMessage = d.ReasonMessage
			if item.ErrorMessage == "" {
				item.ErrorMessage = ReasonMessageVI(d.ReasonCode)
			}
			out.Skipped++
			out.Results = append(out.Results, item)
			continue
		}
		out.EligibleCount++
		if mode == "incremental" && d.AlreadyMaterialized {
			item.Outcome = "exists"
			item.CompanyRecordID = d.ExistingCompanyRecID
			out.Exists++
			out.Results = append(out.Results, item)
			continue
		}
		if req.DryRun {
			item.Outcome = "created"
			item.ReasonCode = ReasonEligible
			out.Created++
			out.Results = append(out.Results, item)
			continue
		}
		recordID, cErr := s.repo.CreateCompanyProcessingRecord(ctx, rec.ID, d.CompanyID, rec.TemplateID, rec.Title, rec.Summary, rec.Content, actor.UserID)
		if cErr != nil {
			if existingID, found2, _ := s.repo.FindCompanyRecordLink(ctx, rec.ID, d.CompanyID); found2 {
				item.Outcome = "exists"
				item.CompanyRecordID = existingID
				item.ReasonCode = ReasonAlreadyMaterialized
				out.Exists++
			} else {
				item.Outcome = "failed"
				item.ErrorCode = "CREATE_FAILED"
				item.ErrorMessage = cErr.Error()
				out.Failed++
			}
			out.Results = append(out.Results, item)
			continue
		}
		item.Outcome = "created"
		item.CompanyRecordID = recordID
		item.ReasonCode = ReasonEligible
		out.Created++
		out.Results = append(out.Results, item)
	}

	// Client-requested IDs that were not in evaluation set at all (unknown company).
	if filterRequested {
		seen := map[string]struct{}{}
		for _, r := range out.Results {
			seen[r.CompanyID] = struct{}{}
		}
		for id := range requestedFilter {
			if _, ok := seen[id]; ok {
				continue
			}
			out.Requested++
			out.Skipped++
			out.Results = append(out.Results, MaterializeItemResult{
				CompanyID:    id,
				Outcome:      "skipped",
				ErrorCode:    ReasonCompanyInactive,
				ErrorMessage: ReasonMessageVI(ReasonCompanyInactive),
				ReasonCode:   ReasonCompanyInactive,
			})
		}
	}

	if !req.DryRun {
		_ = s.repo.SaveMaterializationRun(ctx, *out, actor, rec.ID)
	}
	return out, nil
}

func displayCycleKey(cycleKey string) string {
	if i := strings.Index(cycleKey, "#archived:"); i >= 0 {
		return cycleKey[:i]
	}
	return cycleKey
}
