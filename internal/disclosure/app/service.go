package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

type service struct {
	repo Repository
	auth authapp.Service
	idg  idgen.Generator
}

const (
	templateScopeGlobal  = "global"
	templateScopeCompany = "company"
)

func NewService(repo Repository, auth authapp.Service, idg idgen.Generator) Service {
	return &service{repo: repo, auth: auth, idg: idg}
}

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
	rec := RecordDTO{
		RecordID:     s.idg.NewUUID(),
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
	return s.repo.Create(ctx, rec)
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
	cur.Status = "Published"
	cur.PublishedDate = time.Now().UTC().Format("2006-01-02")
	cur.UpdatedBy = req.Subject.UserID
	return s.repo.Update(ctx, *cur)
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
	if strings.ToLower(cur.Status) != "published" {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "record is not in published state", nil)
	}
	cur.Status = "Completed"
	cur.UpdatedBy = req.Subject.UserID
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

func (s *service) ListTypes(ctx context.Context, req ListTypesRequest) (*ListTypesResponse, error) {
	if err := s.requireDisclosureCatalogRead(ctx, req.Subject); err != nil {
		return nil, err
	}
	out, err := s.repo.ListTypes(ctx, req.Subject.CompanyID, req.GroupID, req.Query)
	if err != nil {
		return nil, err
	}
	return &ListTypesResponse{Items: out}, nil
}

func (s *service) GetTypeDetail(ctx context.Context, req GetTypeDetailRequest) (*DisclosureTypeDTO, error) {
	if strings.TrimSpace(req.TypeID) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "type_id is required", nil)
	}
	if err := s.requireDisclosureCatalogRead(ctx, req.Subject); err != nil {
		return nil, err
	}
	return s.repo.GetTypeDetail(ctx, req.Subject.CompanyID, req.TypeID)
}

func (s *service) GetTypeVersionDetail(ctx context.Context, req GetTypeVersionDetailRequest) (*DisclosureTypeDTO, error) {
	if !s.hasPermission(ctx, req.Subject, "rbac.manage") {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "access denied", nil)
	}
	req.TypeID = strings.TrimSpace(req.TypeID)
	if req.TypeID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "type_id is required", nil)
	}
	if req.VersionNo <= 0 {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "version_no must be > 0", nil)
	}
	return s.repo.GetTypeVersionDetail(ctx, req.Subject.CompanyID, req.TypeID, req.VersionNo)
}

func (s *service) GetTemplateReferenceData(ctx context.Context, req GetTemplateReferenceDataRequest) (*GetTemplateReferenceDataResponse, error) {
	if !s.hasPermission(ctx, req.Subject, "rbac.manage") {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "access denied", nil)
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
				PeriodicityEventBased,
				PeriodicityAdHoc,
			},
			DeadlineStrategies: []string{
				DeadlineStrategyFixedCycleDays,
				DeadlineStrategyEventHours,
				DeadlineStrategyConfigurable,
			},
			MatrixRules: map[string][]string{
				TemplateCategoryPeriodic: {
					"periodicity in [monthly, quarterly, yearly]",
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
	if !s.hasPermission(ctx, req.Subject, "rbac.manage") {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "access denied", nil)
	}
	req.TypeID = strings.TrimSpace(req.TypeID)
	req.Scope = strings.ToLower(strings.TrimSpace(req.Scope))
	req.GroupID = strings.TrimSpace(req.GroupID)
	req.Name = strings.TrimSpace(req.Name)
	req.Category = strings.TrimSpace(req.Category)
	req.TemplateCategory = strings.TrimSpace(req.TemplateCategory)
	req.DeadlineStrategy = strings.TrimSpace(req.DeadlineStrategy)
	req.DeadlineRule = strings.TrimSpace(req.DeadlineRule)
	req.Periodicity = strings.TrimSpace(req.Periodicity)
	req.ReminderMilestones = sanitizeStringList(req.ReminderMilestones)
	req.LegalBases = sanitizeLegalBases(req.LegalBases)
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
	if req.Scope == "" {
		if s.hasPermission(ctx, req.Subject, "platform.cms.view") {
			req.Scope = templateScopeGlobal
		} else {
			req.Scope = templateScopeCompany
		}
	}
	if req.Scope != templateScopeGlobal && req.Scope != templateScopeCompany {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "scope must be global or company", nil)
	}
	if req.Scope == templateScopeGlobal && !s.hasPermission(ctx, req.Subject, "platform.cms.view") {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "global scope requires platform admin permission", nil)
	}
	if req.Scope == templateScopeCompany && !isCompanyCreatableTemplateCategory(req.TemplateCategory) {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "company scope only supports custom template category", nil)
	}
	ApplyTemplateFlatBlockSync(&req, s.idg)
	if err := validateTemplateMatrix(&req); err != nil {
		return nil, err
	}
	return s.repo.UpsertTypeVersion(ctx, req)
}

func isCompanyCreatableTemplateCategory(category string) bool {
	return strings.EqualFold(strings.TrimSpace(category), TemplateCategoryCustom)
}

func (s *service) ListTypeVersions(ctx context.Context, req ListTypeVersionsRequest) (*ListTypeVersionsResponse, error) {
	if !s.hasPermission(ctx, req.Subject, "rbac.manage") {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "access denied", nil)
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
	if !s.hasPermission(ctx, req.Subject, "rbac.manage") {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "access denied", nil)
	}
	req.TypeID = strings.TrimSpace(req.TypeID)
	req.Reason = strings.TrimSpace(req.Reason)
	if req.TypeID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "type_id is required", nil)
	}
	if req.VersionNo <= 0 {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "version_no must be > 0", nil)
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
	return &GetCompanyWorkflowOverrideResponse{Data: *view}, nil
}

func (s *service) UpsertCompanyWorkflowOverrideDraft(ctx context.Context, req UpsertCompanyWorkflowOverrideDraftRequest) (*UpsertCompanyWorkflowOverrideDraftResponse, error) {
	req.TypeID = strings.TrimSpace(req.TypeID)
	req.ChangeNote = strings.TrimSpace(req.ChangeNote)
	if req.TypeID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "type_id is required", nil)
	}
	if len(req.Workflow) == 0 {
		return nil, &perr.HTTPError{
			Code:       perr.CodeInvalidRequest,
			Message:    "workflow is required",
			HTTPStatus: http.StatusBadRequest,
			Details:    map[string]any{"field_errors": map[string]string{"workflow": "must contain at least one step"}},
		}
	}
	if err := s.authorize(ctx, req.Subject, "template.workflow.override.write", authapp.ResourceRef{
		Type: "disclosure_type",
		ID:   req.TypeID,
	}); err != nil {
		return nil, err
	}
	for i := range req.Workflow {
		req.Workflow[i].StepID = strings.TrimSpace(req.Workflow[i].StepID)
		req.Workflow[i].Stage = strings.TrimSpace(req.Workflow[i].Stage)
		req.Workflow[i].Department = strings.TrimSpace(req.Workflow[i].Department)
		req.Workflow[i].AssigneeRole = strings.TrimSpace(req.Workflow[i].AssigneeRole)
		req.Workflow[i].DueRule = strings.TrimSpace(req.Workflow[i].DueRule)
		if req.Workflow[i].StepID == "" || req.Workflow[i].Stage == "" {
			return nil, &perr.HTTPError{
				Code:       perr.CodeInvalidRequest,
				Message:    "workflow step is invalid",
				HTTPStatus: http.StatusBadRequest,
				Details:    map[string]any{"field_errors": map[string]string{fmt.Sprintf("workflow[%d]", i): "step_id and stage are required"}},
			}
		}
	}
	return s.repo.UpsertCompanyWorkflowOverrideDraft(ctx, req)
}

func (s *service) ApproveCompanyWorkflowOverride(ctx context.Context, req ApproveCompanyWorkflowOverrideRequest) (*ApproveCompanyWorkflowOverrideResponse, error) {
	req.TypeID = strings.TrimSpace(req.TypeID)
	req.Reason = strings.TrimSpace(req.Reason)
	if req.TypeID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "type_id is required", nil)
	}
	if req.VersionNo <= 0 {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "version_no must be > 0", nil)
	}
	if err := s.authorize(ctx, req.Subject, "template.workflow.override.approve", authapp.ResourceRef{
		Type: "disclosure_type",
		ID:   req.TypeID,
	}); err != nil {
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

func (s *service) GetEffectiveWorkflow(ctx context.Context, req GetEffectiveWorkflowRequest) (*GetEffectiveWorkflowResponse, error) {
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
	out, err := s.repo.GetEffectiveWorkflow(ctx, req.Subject.CompanyID, req.TypeID)
	if err != nil {
		return nil, err
	}
	return &GetEffectiveWorkflowResponse{Data: *out}, nil
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

func (s *service) authorize(ctx context.Context, sub Subject, action string, resource authapp.ResourceRef) error {
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
