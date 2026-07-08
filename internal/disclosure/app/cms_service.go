package app

import (
	"context"
	"net/http"
	"strings"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// CmsArchiveTemplate soft-archives a global system template (OQ-004=A, OQ-006=A).
// In-flight disclosure records are unaffected; the template simply stops appearing
// in the portal list (which filters published only).
func (s *service) CmsArchiveTemplate(ctx context.Context, req CmsArchiveTemplateRequest) (*CmsArchiveTemplateResponse, error) {
	if err := s.requireCMSTemplateArchive(ctx, req.Subject); err != nil {
		return nil, err
	}
	req.TypeID = strings.TrimSpace(req.TypeID)
	if req.TypeID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "type_id is required", nil)
	}
	if err := s.repo.ArchiveGlobalTemplate(ctx, req.TypeID, req.Subject.UserID); err != nil {
		return nil, err
	}
	return &CmsArchiveTemplateResponse{TypeID: req.TypeID, Status: "archived"}, nil
}

// CmsGetGlobalWorkflow returns the active global workflow for a template type.
func (s *service) CmsGetGlobalWorkflow(ctx context.Context, req CmsGetGlobalWorkflowRequest) (*CmsGetGlobalWorkflowResponse, error) {
	if err := s.requireCMSTemplateRead(ctx, req.Subject); err != nil {
		return nil, err
	}
	req.TypeID = strings.TrimSpace(req.TypeID)
	if req.TypeID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "type_id is required", nil)
	}
	wf, err := s.repo.GetGlobalWorkflow(ctx, req.TypeID)
	if err != nil {
		return nil, err
	}
	return &CmsGetGlobalWorkflowResponse{Data: wf}, nil
}

// CmsUpsertGlobalWorkflow creates or replaces the global workflow for a template type.
func (s *service) CmsUpsertGlobalWorkflow(ctx context.Context, req CmsUpsertGlobalWorkflowRequest) (*GlobalWorkflowDTO, error) {
	if err := s.requireCMSTemplateWrite(ctx, req.Subject); err != nil {
		return nil, err
	}
	req.TypeID = strings.TrimSpace(req.TypeID)
	if req.TypeID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "type_id is required", nil)
	}
	if len(req.Steps) == 0 {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "workflow must have at least one step", nil)
	}
	const maxWorkflowStepInstructionsLen = 2000
	for i, step := range req.Steps {
		if len(step.Instructions) > maxWorkflowStepInstructionsLen {
			return nil, &perr.HTTPError{
				HTTPStatus: http.StatusBadRequest, Code: perr.CodeInvalidRequest,
				Message: "workflow step instructions must be at most 2000 characters",
				Details: map[string]any{"step_index": i, "field": "instructions"},
			}
		}
		if strings.TrimSpace(step.Stage) == "" {
			return nil, &perr.HTTPError{
				HTTPStatus: http.StatusBadRequest, Code: perr.CodeInvalidRequest,
				Message: "workflow step stage is required",
				Details: map[string]any{"step_index": i},
			}
		}
		if len(step.AssigneeRoleIds) == 0 {
			return nil, &perr.HTTPError{
				HTTPStatus: http.StatusBadRequest, Code: perr.CodeInvalidRequest,
				Message: "workflow step assignee_role_ids is required",
				Details: map[string]any{"step_index": i},
			}
		}
		if step.ProcessingDays <= 0 {
			return nil, &perr.HTTPError{
				HTTPStatus: http.StatusBadRequest, Code: perr.CodeInvalidRequest,
				Message: "workflow step processing_days must be > 0",
				Details: map[string]any{"step_index": i},
			}
		}
	}
	workflowID := s.idg.NewUUID()
	return s.repo.UpsertGlobalWorkflow(ctx, req, workflowID)
}

// CmsDeleteGlobalWorkflow removes the global workflow for a template type.
func (s *service) CmsDeleteGlobalWorkflow(ctx context.Context, req CmsDeleteGlobalWorkflowRequest) error {
	if err := s.requireCMSTemplateWrite(ctx, req.Subject); err != nil {
		return err
	}
	req.TypeID = strings.TrimSpace(req.TypeID)
	if req.TypeID == "" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "type_id is required", nil)
	}
	return s.repo.DeleteGlobalWorkflow(ctx, req.TypeID)
}

// CmsListDisplayGroupsCatalog returns all display groups for CMS management.
func (s *service) CmsListDisplayGroupsCatalog(ctx context.Context, req ListDisplayGroupsRequest) (*ListDisplayGroupsResponse, error) {
	if err := s.requireCMSTemplateRead(ctx, req.Subject); err != nil {
		return nil, err
	}
	items, err := s.repo.ListDisplayGroups(ctx)
	if err != nil {
		return nil, err
	}
	return &ListDisplayGroupsResponse{Items: items}, nil
}

// CmsCreateDisplayGroup creates a new display group.
func (s *service) CmsCreateDisplayGroup(ctx context.Context, req CmsDisplayGroupCreateRequest) (*DisplayGroupDTO, error) {
	if err := s.requireCMSTemplateWrite(ctx, req.Subject); err != nil {
		return nil, err
	}
	req.Code = strings.TrimSpace(req.Code)
	req.NameVI = strings.TrimSpace(req.NameVI)
	if req.Code == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "code is required", nil)
	}
	if req.NameVI == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "name_vi is required", nil)
	}
	return s.repo.CreateDisplayGroup(ctx, req)
}

// CmsUpdateDisplayGroup updates an existing display group.
func (s *service) CmsUpdateDisplayGroup(ctx context.Context, req CmsDisplayGroupUpdateRequest) (*DisplayGroupDTO, error) {
	if err := s.requireCMSTemplateWrite(ctx, req.Subject); err != nil {
		return nil, err
	}
	req.Code = strings.TrimSpace(req.Code)
	if req.Code == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "code is required", nil)
	}
	return s.repo.UpdateDisplayGroup(ctx, req)
}

// CmsDeleteDisplayGroup removes a display group by code.
func (s *service) CmsDeleteDisplayGroup(ctx context.Context, req CmsDisplayGroupDeleteRequest) error {
	if err := s.requireCMSTemplateWrite(ctx, req.Subject); err != nil {
		return err
	}
	req.Code = strings.TrimSpace(req.Code)
	if req.Code == "" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "code is required", nil)
	}
	return s.repo.DeleteDisplayGroup(ctx, req.Code)
}

// CmsListTemplateDepartmentsCatalog lists template-level default departments for global workflows.
func (s *service) CmsListTemplateDepartmentsCatalog(ctx context.Context, req ListDisplayGroupsRequest) (*ListTemplateDepartmentsResponse, error) {
	if err := s.requireCMSTemplateRead(ctx, req.Subject); err != nil {
		return nil, err
	}
	items, err := s.repo.ListTemplateDepartments(ctx)
	if err != nil {
		return nil, err
	}
	return &ListTemplateDepartmentsResponse{Items: items}, nil
}

// CmsCreateTemplateDepartment adds a template-level default department option.
func (s *service) CmsCreateTemplateDepartment(ctx context.Context, req CmsTemplateDepartmentCreateRequest) (*TemplateDepartmentDTO, error) {
	if err := s.requireCMSTemplateWrite(ctx, req.Subject); err != nil {
		return nil, err
	}
	req.Code = strings.TrimSpace(req.Code)
	if err := s.resolveTemplateDepartmentCreate(ctx, &req); err != nil {
		return nil, err
	}
	return s.repo.CreateTemplateDepartment(ctx, req)
}

// CmsListDeadlineRules lists all deadline rules from the catalog.
func (s *service) CmsListDeadlineRules(ctx context.Context, req GetTemplateReferenceDataRequest) ([]CmsDeadlineRuleDTO, error) {
	if err := s.requireCMSTemplateRead(ctx, req.Subject); err != nil {
		return nil, err
	}
	return s.repo.ListCmsDeadlineRules(ctx)
}

// CmsCreateDeadlineRule creates a new deadline rule in the catalog.
func (s *service) CmsCreateDeadlineRule(ctx context.Context, req CmsDeadlineRuleCreateRequest) (*CmsDeadlineRuleDTO, error) {
	if err := s.requireCMSTemplateWrite(ctx, req.Subject); err != nil {
		return nil, err
	}
	req.Code = strings.TrimSpace(req.Code)
	req.LabelVI = strings.TrimSpace(req.LabelVI)
	req.Pattern = strings.TrimSpace(req.Pattern)
	if req.Code == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "code is required", nil)
	}
	if req.LabelVI == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "label_vi is required", nil)
	}
	if req.Pattern == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "pattern is required", nil)
	}
	ruleID := s.idg.NewUUID()
	return s.repo.CreateDeadlineRule(ctx, req, ruleID)
}

// CmsUpdateDeadlineRule updates an existing deadline rule.
func (s *service) CmsUpdateDeadlineRule(ctx context.Context, req CmsDeadlineRuleUpdateRequest) (*CmsDeadlineRuleDTO, error) {
	if err := s.requireCMSTemplateWrite(ctx, req.Subject); err != nil {
		return nil, err
	}
	req.RuleID = strings.TrimSpace(req.RuleID)
	if req.RuleID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "rule_id is required", nil)
	}
	return s.repo.UpdateDeadlineRule(ctx, req)
}

// CmsDeleteDeadlineRule removes a deadline rule from the catalog.
func (s *service) CmsDeleteDeadlineRule(ctx context.Context, req CmsDeadlineRuleDeleteRequest) error {
	if err := s.requireCMSTemplateWrite(ctx, req.Subject); err != nil {
		return err
	}
	req.RuleID = strings.TrimSpace(req.RuleID)
	if req.RuleID == "" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "rule_id is required", nil)
	}
	return s.repo.DeleteDeadlineRule(ctx, req.RuleID)
}
