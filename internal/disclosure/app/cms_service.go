package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

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

// CmsGetGlobalWorkflow is a wire-compatible projection of the template-owned
// draft (when present) or active publication. Global tables are history only.
func (s *service) CmsGetGlobalWorkflow(ctx context.Context, req CmsGetGlobalWorkflowRequest) (*CmsGetGlobalWorkflowResponse, error) {
	if err := s.requireCMSTemplateRead(ctx, req.Subject); err != nil {
		return nil, err
	}
	req.TypeID = strings.TrimSpace(req.TypeID)
	if req.TypeID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "type_id is required", nil)
	}
	detail, isDraft, err := s.templateWorkflowCandidateDetail(ctx, req.Subject, req.TypeID)
	if err != nil {
		return nil, err
	}
	wf := projectTemplateWorkflow(detail, isDraft)
	if legacy, legacyErr := s.repo.GetGlobalWorkflow(ctx, req.TypeID); legacyErr == nil && legacy != nil {
		wf.WorkflowID = legacy.WorkflowID
		wf.PublishedVersionNo = legacy.PublishedVersionNo
		wf.ActiveVersionNo = legacy.ActiveVersionNo
		wf.CreatedAt = legacy.CreatedAt
		wf.CreatedBy = legacy.CreatedBy
	}
	return &CmsGetGlobalWorkflowResponse{Data: wf}, nil
}

// CmsUpsertGlobalWorkflow preserves the old request/response shape but writes
// only the template draft's enterprise_workflow projection.
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
		if len(step.Description) > maxWorkflowStepInstructionsLen {
			return nil, &perr.HTTPError{
				HTTPStatus: http.StatusBadRequest, Code: perr.CodeInvalidRequest,
				Message: "workflow step description must be at most 2000 characters",
				Details: map[string]any{"step_index": i, "field": "description"},
			}
		}
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
		if err := ValidateWorkflowStepReminderConfigForPersist(step.ReminderConfig); err != nil {
			return nil, &perr.HTTPError{
				HTTPStatus: http.StatusBadRequest, Code: perr.CodeInvalidRequest,
				Message: err.Error(),
				Details: map[string]any{"step_index": i, "field": "reminder_config"},
			}
		}
		if err := ValidateWorkflowStepDescriptionFormatForPersist(step.DescriptionFormat); err != nil {
			return nil, &perr.HTTPError{
				HTTPStatus: http.StatusBadRequest, Code: perr.CodeInvalidRequest,
				Message: err.Error(),
				Details: map[string]any{"step_index": i, "field": "description_format"},
			}
		}
		if strings.TrimSpace(step.DescriptionFormat) != "" {
			req.Steps[i].DescriptionFormat = NormalizeWorkflowStepDescriptionFormat(step.DescriptionFormat)
			if req.Steps[i].DescriptionFormat == WorkflowStepDescriptionFormatPlainText {
				req.Steps[i].DescriptionFormat = ""
			}
		}
	}
	detail, _, err := s.templateWorkflowCandidateDetail(ctx, req.Subject, req.TypeID)
	if err != nil {
		return nil, err
	}
	if err := rejectDuplicateIncomingIdentities(req.Steps); err != nil {
		return nil, err
	}
	assignFacadeStepKeys(&req, detail)
	if err := rejectDuplicateFacadeStepKeys(req.Steps); err != nil {
		return nil, err
	}
	merged := mergeIncomingWorkflowSteps(workflowStepsFromDetail(detail), req.Steps)
	blocks, err := replaceTemplateWorkflowSteps(detail.Blocks, merged)
	if err != nil {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, err.Error(), nil)
	}
	upsert := upsertRequestFromDetail(req.Subject, detail, blocks, req.ChangeNote)
	upsert.SkipPublicationMatrix = true
	saved, err := s.UpsertTypeVersion(ctx, upsert)
	if err != nil {
		return nil, err
	}
	savedDetail, err := s.repo.GetTypeVersionDetail(ctx, req.Subject.CompanyID, req.TypeID, saved.VersionNo)
	if err != nil {
		return nil, err
	}
	return projectTemplateWorkflow(savedDetail, true), nil
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
	detail, _, err := s.templateWorkflowCandidateDetail(ctx, req.Subject, req.TypeID)
	if err != nil {
		return err
	}
	blocks, err := replaceTemplateWorkflowSteps(detail.Blocks, nil)
	if err != nil {
		return err
	}
	upsert := upsertRequestFromDetail(req.Subject, detail, blocks, "clear workflow draft")
	upsert.SkipPublicationMatrix = true
	upsert.ClearWorkflow = true
	_, err = s.UpsertTypeVersion(ctx, upsert)
	return err
}

func (s *service) templateWorkflowCandidateDetail(ctx context.Context, sub Subject, typeID string) (*DisclosureTypeDTO, bool, error) {
	versions, err := s.repo.ListTypeVersions(ctx, sub.CompanyID, typeID)
	if err != nil {
		return nil, false, err
	}
	target := 0
	isDraft := false
	for _, version := range versions {
		if !version.IsActive && !version.IsReleased && version.VersionNo > target {
			target = version.VersionNo
			isDraft = true
		}
	}
	if target == 0 {
		for _, version := range versions {
			if version.IsActive {
				target = version.VersionNo
				break
			}
		}
	}
	if target == 0 {
		return nil, false, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "template has no workflow candidate version", nil)
	}
	detail, err := s.repo.GetTypeVersionDetail(ctx, sub.CompanyID, typeID, target)
	return detail, isDraft, err
}

func replaceTemplateWorkflowBlock(blocks []TemplateBlockDTO, steps []GlobalWorkflowStepInput) ([]TemplateBlockDTO, error) {
	dtos := make([]WorkflowStepDTO, 0, len(steps))
	for _, step := range steps {
		dtos = append(dtos, workflowStepFromGlobalInput(step, WorkflowStepDTO{}))
	}
	return replaceTemplateWorkflowSteps(blocks, dtos)
}

func replaceTemplateWorkflowSteps(blocks []TemplateBlockDTO, steps []WorkflowStepDTO) ([]TemplateBlockDTO, error) {
	raw, err := json.Marshal(steps)
	if err != nil {
		return nil, fmt.Errorf("marshal workflow projection: %w", err)
	}
	var projected []any
	if err := json.Unmarshal(raw, &projected); err != nil {
		return nil, fmt.Errorf("normalize workflow projection: %w", err)
	}
	out := make([]TemplateBlockDTO, 0, len(blocks)+1)
	replaced := false
	for _, block := range blocks {
		next := block
		if strings.EqualFold(strings.TrimSpace(block.BlockKey), "enterprise_workflow") {
			cfg := map[string]any{"steps": projected}
			if block.Config != nil {
				if maxLen, ok := block.Config["max_length"]; ok {
					cfg["max_length"] = maxLen
				}
				if allowHTML, ok := block.Config["allow_html"]; ok {
					cfg["allow_html"] = allowHTML
				}
			}
			next.Config = cfg
			next.Enabled = true
			replaced = true
		}
		out = append(out, next)
	}
	if !replaced {
		out = append(out, TemplateBlockDTO{
			BlockID: "enterprise-workflow", BlockKey: "enterprise_workflow", BlockType: "workflow",
			Title: "Workflow", Config: map[string]any{"steps": projected},
			Validation: map[string]any{}, DisplayOrder: len(out) + 1, Enabled: true,
		})
	}
	return out, nil
}

func assignFacadeStepKeys(req *CmsUpsertGlobalWorkflowRequest, detail *DisclosureTypeDTO) {
	existingKeyByStepID := map[string]string{}
	existingKeys := map[string]bool{}
	indexFacadeIdentity := func(stepID, stepKey string) {
		key := strings.TrimSpace(stepKey)
		id := strings.TrimSpace(stepID)
		if key == "" {
			key = id
		}
		if key == "" {
			return
		}
		existingKeys[key] = true
		if id != "" {
			existingKeyByStepID[id] = key
		}
	}
	for _, step := range projectGlobalStepsFromDetail(detail) {
		indexFacadeIdentity(step.StepID, step.StepKey)
	}
	for _, step := range projectStepsFromBlocks(detail.Blocks) {
		indexFacadeIdentity(step.StepID, step.StepKey)
	}
	usedKeys := map[string]bool{}
	for i := range req.Steps {
		if strings.TrimSpace(req.Steps[i].StepID) == "" {
			req.Steps[i].StepID = fmt.Sprintf("%s-step-%d", req.TypeID, i+1)
		}
		if req.Steps[i].DisplayOrder <= 0 {
			req.Steps[i].DisplayOrder = i + 1
		}
		req.Steps[i].StepKey = ResolveStepKey(req.Steps[i], existingKeys, existingKeyByStepID, usedKeys)
		usedKeys[req.Steps[i].StepKey] = true
	}
}

func rejectDuplicateIncomingIdentities(steps []GlobalWorkflowStepInput) error {
	seenID := map[string]int{}
	seenKey := map[string]int{}
	for i, step := range steps {
		if id := strings.TrimSpace(step.StepID); id != "" {
			if prev, ok := seenID[id]; ok {
				return &perr.HTTPError{
					HTTPStatus: http.StatusBadRequest, Code: perr.CodeInvalidRequest,
					Message: "workflow step_id must be unique",
					Details: map[string]any{"step_index": i, "duplicate_of": prev, "step_id": id},
				}
			}
			seenID[id] = i
		}
		if key := strings.TrimSpace(step.StepKey); key != "" {
			if prev, ok := seenKey[key]; ok {
				return &perr.HTTPError{
					HTTPStatus: http.StatusBadRequest, Code: perr.CodeInvalidRequest,
					Message: "workflow step_key must be unique",
					Details: map[string]any{"step_index": i, "duplicate_of": prev, "step_key": key},
				}
			}
			seenKey[key] = i
		}
	}
	return nil
}

func rejectDuplicateFacadeStepKeys(steps []GlobalWorkflowStepInput) error {
	seen := map[string]int{}
	for i, step := range steps {
		key := strings.TrimSpace(step.StepKey)
		if key == "" {
			key = strings.TrimSpace(step.StepID)
		}
		if key == "" {
			continue
		}
		if prev, ok := seen[key]; ok {
			return &perr.HTTPError{
				HTTPStatus: http.StatusBadRequest, Code: perr.CodeInvalidRequest,
				Message: "workflow step_key must be unique",
				Details: map[string]any{"step_index": i, "duplicate_of": prev, "step_key": key},
			}
		}
		seen[key] = i
	}
	return nil
}

func projectStepsFromBlocks(blocks []TemplateBlockDTO) []GlobalWorkflowStepInput {
	for _, block := range blocks {
		if !strings.EqualFold(strings.TrimSpace(block.BlockKey), "enterprise_workflow") {
			continue
		}
		raw, err := json.Marshal(block.Config["steps"])
		if err != nil || len(raw) == 0 || string(raw) == "null" {
			return nil
		}
		var steps []GlobalWorkflowStepInput
		if err := json.Unmarshal(raw, &steps); err != nil {
			return nil
		}
		return steps
	}
	return nil
}

func upsertRequestFromDetail(sub Subject, detail *DisclosureTypeDTO, blocks []TemplateBlockDTO, changeNote string) UpsertTypeVersionRequest {
	return UpsertTypeVersionRequest{
		Subject: sub, TypeID: detail.TypeID, Scope: detail.Scope, GroupID: detail.GroupID,
		Name: detail.Name, Category: detail.Category, TemplateCategory: detail.TemplateCategory,
		DeadlineStrategy: detail.DeadlineStrategy, Description: detail.Description,
		LegalBasis: detail.LegalBasis, Applicability: detail.Applicability,
		ImplementationContent: detail.ImplementationContent, ImplementationNotes: detail.ImplementationNotes,
		SpecialCases: detail.SpecialCases, ReportContent: detail.ReportContent, RequiredDocs: detail.RequiredDocs,
		DeadlineRule: detail.DeadlineRule, Periodicity: detail.Periodicity, ChannelsText: detail.ChannelsText,
		Beneficiaries: detail.Beneficiaries, ReceivingAuthorities: detail.ReceivingAuthorities,
		Format: detail.Format, LegalRisksText: detail.LegalRisksText, GeneralInfo: detail.GeneralInfo,
		LegalBases: detail.LegalBases, LegalBasesProvided: true, Checklist: detail.Checklist, Tags: detail.Tags,
		DeadlineConfig: detail.DeadlineConfig, Blocks: blocks, DisplayGroupCodes: detail.DisplayGroupCodes,
		ChangeNote: changeNote, ApplicabilityRules: detail.ApplicabilityRules,
	}
}

func projectTemplateWorkflow(detail *DisclosureTypeDTO, isDraft bool) *GlobalWorkflowDTO {
	steps := projectGlobalStepsFromDetail(detail)
	out := &GlobalWorkflowDTO{
		WorkflowID: fmt.Sprintf("template:%s:%d", detail.TypeID, detail.VersionNo),
		TypeID:     detail.TypeID, Status: "active", Steps: steps,
		UpdatedAt: time.Now().UTC(),
	}
	if isDraft {
		out.Status = "draft"
	}
	return out
}

func workflowStepsFromDetail(detail *DisclosureTypeDTO) []WorkflowStepDTO {
	if detail == nil {
		return nil
	}
	if detail.WorkflowManifest != nil && len(detail.WorkflowManifest.Steps) > 0 {
		out := make([]WorkflowStepDTO, 0, len(detail.WorkflowManifest.Steps))
		for _, step := range detail.WorkflowManifest.Steps {
			dto := step.WorkflowStepDTO
			if strings.TrimSpace(dto.StepID) == "" {
				dto.StepID = strings.TrimSpace(step.StepKey)
			}
			out = append(out, dto)
		}
		return out
	}
	return ExtractTemplateWorkflow(detail.Blocks)
}

func projectGlobalStepsFromDetail(detail *DisclosureTypeDTO) []GlobalWorkflowStepInput {
	if detail == nil {
		return nil
	}
	dtos := workflowStepsFromDetail(detail)
	out := make([]GlobalWorkflowStepInput, 0, len(dtos))
	keyByID := map[string]string{}
	if detail.WorkflowManifest != nil {
		for _, step := range detail.WorkflowManifest.Steps {
			id := strings.TrimSpace(step.StepID)
			key := strings.TrimSpace(step.StepKey)
			if id != "" && key != "" {
				keyByID[id] = key
			}
		}
	}
	for _, step := range dtos {
		key := strings.TrimSpace(keyByID[strings.TrimSpace(step.StepID)])
		if key == "" {
			key = strings.TrimSpace(step.StepID)
		}
		out = append(out, GlobalWorkflowStepInput{
			StepID: step.StepID, StepKey: key, Stage: step.Stage,
			Description: step.Description, DescriptionFormat: step.DescriptionFormat,
			Instructions: step.Instructions,
			DepartmentID: step.DepartmentID, AssigneeRoleIds: append([]string(nil), step.AssigneeRoleIds...),
			DueRule: step.DueRule, ProcessingDays: step.ProcessingDays, DisplayOrder: step.DisplayOrder,
			ReminderConfig: CloneWorkflowStepReminderConfig(step.ReminderConfig),
		})
	}
	return out
}

func mergeIncomingWorkflowSteps(existing []WorkflowStepDTO, incoming []GlobalWorkflowStepInput) []WorkflowStepDTO {
	byID := map[string]WorkflowStepDTO{}
	if existing == nil {
		existing = []WorkflowStepDTO{}
	}
	for _, step := range existing {
		if id := strings.TrimSpace(step.StepID); id != "" {
			byID[id] = step
		}
	}
	out := make([]WorkflowStepDTO, 0, len(incoming))
	for _, in := range incoming {
		prev, ok := byID[strings.TrimSpace(in.StepID)]
		if !ok {
			prev, ok = byID[strings.TrimSpace(in.StepKey)]
		}
		if !ok {
			prev = WorkflowStepDTO{}
		}
		out = append(out, workflowStepFromGlobalInput(in, prev))
	}
	return out
}

func workflowStepFromGlobalInput(in GlobalWorkflowStepInput, prev WorkflowStepDTO) WorkflowStepDTO {
	next := prev
	if id := strings.TrimSpace(in.StepID); id != "" {
		next.StepID = id
	} else if strings.TrimSpace(next.StepID) == "" {
		next.StepID = strings.TrimSpace(in.StepKey)
	}
	next.Stage = in.Stage
	next.Description = in.Description
	next.DescriptionFormat = in.DescriptionFormat
	if strings.TrimSpace(next.DescriptionFormat) != "" {
		next.DescriptionFormat = NormalizeWorkflowStepDescriptionFormat(next.DescriptionFormat)
		if next.DescriptionFormat == WorkflowStepDescriptionFormatPlainText {
			next.DescriptionFormat = ""
		}
	}
	next.Instructions = in.Instructions
	next.DepartmentID = in.DepartmentID
	next.AssigneeRoleIds = append([]string(nil), in.AssigneeRoleIds...)
	next.DueRule = in.DueRule
	next.ProcessingDays = in.ProcessingDays
	next.DisplayOrder = in.DisplayOrder
	next.ReminderConfig = CloneWorkflowStepReminderConfig(in.ReminderConfig)
	return next
}

func redactEnterpriseWorkflowStepsForCMSEditor(item *DisclosureTypeDTO) *DisclosureTypeDTO {
	if item == nil {
		return nil
	}
	if item.WorkflowManifest != nil && len(item.WorkflowManifest.Steps) > 0 {
		item.HasWorkflow = true
	}
	blocks := make([]TemplateBlockDTO, len(item.Blocks))
	for i, block := range item.Blocks {
		next := block
		if strings.EqualFold(strings.TrimSpace(block.BlockKey), "enterprise_workflow") {
			cfg := map[string]any{}
			for k, v := range block.Config {
				if k == "steps" || k == "workflow" {
					continue
				}
				cfg[k] = v
			}
			cfg["steps"] = []any{}
			next.Config = cfg
			// Frozen FE hydrates enterpriseWorkflowSteps from block Description /
			// implementation_content via legacyWorkflowTextToSteps when steps are empty.
			// Those reconstructed rows have stage text but no department_id, so Lưu nháp
			// fails client validation ("N bước chưa chọn phòng/ban") after reload.
			next.Description = ""
		}
		blocks[i] = next
	}
	item.Blocks = blocks
	item.ImplementationContent = ""
	return item
}

func (s *service) preservePinnedWorkflowIfOmitted(ctx context.Context, req *UpsertTypeVersionRequest) error {
	if req == nil || req.ClearWorkflow {
		return nil
	}
	if len(ExtractTemplateWorkflow(req.Blocks)) > 0 {
		return nil
	}
	detail, _, err := s.templateWorkflowCandidateDetail(ctx, req.Subject, req.TypeID)
	if err != nil {
		return nil
	}
	existing := workflowStepsFromDetail(detail)
	if len(existing) == 0 {
		return nil
	}
	blocks, err := replaceTemplateWorkflowSteps(req.Blocks, existing)
	if err != nil {
		return err
	}
	req.Blocks = blocks
	return nil
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
