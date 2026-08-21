package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

const (
	creationModeTemplateClone = "TEMPLATE_CLONE"
)

// CloneTypeFromActiveRequest creates a new global CMS template draft by one-time
// deep-copy of the source ACTIVE publication (Model A TEMPLATE_PINNED).
type CloneTypeFromActiveRequest struct {
	Subject                 Subject
	SourceTypeID            string `json:"-"`
	TargetTypeID            string `json:"target_type_id"`
	TargetName              string `json:"target_name"`
	ExpectedSourceVersionNo int    `json:"expected_source_version_no"`
}

// CloneTypeFromActiveResponse is returned to FE for editor navigation.
type CloneTypeFromActiveResponse struct {
	TypeID          string    `json:"type_id"`
	VersionNo       int       `json:"version_no"`
	IsActive        bool      `json:"is_active"`
	SourceTypeID    string    `json:"source_type_id"`
	SourceVersionNo int       `json:"source_version_no"`
	UpdatedBy       string    `json:"updated_by"`
	ActivatedAt     time.Time `json:"activated_at"`
}

// CloneTypeFromActive materializes source ACTIVE publication into a new root + draft v1.
// Does not auto-publish. Does not copy runtime, company overrides, or Global Workflow rows.
func (s *service) CloneTypeFromActive(ctx context.Context, req CloneTypeFromActiveRequest) (*CloneTypeFromActiveResponse, error) {
	if err := s.requireCMSTemplateWrite(ctx, req.Subject); err != nil {
		return nil, err
	}
	if err := s.requireCMSTemplateRead(ctx, req.Subject); err != nil {
		return nil, err
	}

	req.SourceTypeID = strings.TrimSpace(req.SourceTypeID)
	req.TargetTypeID = strings.TrimSpace(req.TargetTypeID)
	req.TargetName = strings.TrimSpace(req.TargetName)
	if req.SourceTypeID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "source_type_id is required", nil)
	}
	if req.TargetTypeID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "target_type_id is required", nil)
	}
	if req.TargetName == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "target_name is required", nil)
	}
	if req.ExpectedSourceVersionNo <= 0 {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "expected_source_version_no must be > 0", nil)
	}
	if req.SourceTypeID == req.TargetTypeID {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "target_type_id must differ from source_type_id", nil)
	}

	exists, err := s.repo.TypeExists(ctx, req.TargetTypeID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, &perr.HTTPError{
			Code:       perr.CodeStateConflict,
			Message:    "target_type_id already exists",
			HTTPStatus: http.StatusConflict,
			Details:    map[string]any{"target_type_id": req.TargetTypeID},
		}
	}

	source, sourceVersionNo, err := s.resolveCloneSourceActivePublication(ctx, req.Subject, req.SourceTypeID, req.ExpectedSourceVersionNo)
	if err != nil {
		return nil, err
	}

	upsert, err := s.materializeCloneUpsert(ctx, req, source)
	if err != nil {
		return nil, err
	}

	resp, err := s.UpsertTypeVersion(ctx, upsert)
	if err != nil {
		return nil, err
	}
	if resp.VersionNo != 1 || resp.IsActive {
		return nil, &perr.HTTPError{
			Code:       perr.CodeStateConflict,
			Message:    "clone target must be draft v1 and not active",
			HTTPStatus: http.StatusConflict,
			Details: map[string]any{
				"type_id":     resp.TypeID,
				"version_no":  resp.VersionNo,
				"is_active":   resp.IsActive,
				"expectation": "draft_v1",
			},
		}
	}

	return &CloneTypeFromActiveResponse{
		TypeID:          resp.TypeID,
		VersionNo:       resp.VersionNo,
		IsActive:        false,
		SourceTypeID:    req.SourceTypeID,
		SourceVersionNo: sourceVersionNo,
		UpdatedBy:       resp.UpdatedBy,
		ActivatedAt:     resp.ActivatedAt,
	}, nil
}

// resolveCloneSourceActivePublication loads the exact ACTIVE publication (unredacted).
// Ignores drafts and legacy Global Workflow / enterprise fallback.
func (s *service) resolveCloneSourceActivePublication(
	ctx context.Context,
	sub Subject,
	sourceTypeID string,
	expectedVersionNo int,
) (*DisclosureTypeDTO, int, error) {
	versions, err := s.repo.ListTypeVersions(ctx, sub.CompanyID, sourceTypeID)
	if err != nil {
		return nil, 0, err
	}
	if len(versions) == 0 {
		return nil, 0, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "source template not found", nil)
	}
	activeNo := 0
	for _, v := range versions {
		if v.IsActive {
			activeNo = v.VersionNo
			break
		}
	}
	if activeNo <= 0 {
		return nil, 0, &perr.HTTPError{
			Code:       "TEMPLATE_NO_ACTIVE_PUBLICATION",
			Message:    "source template has no active publication",
			HTTPStatus: http.StatusUnprocessableEntity,
			Details:    map[string]any{"source_type_id": sourceTypeID},
		}
	}
	if activeNo != expectedVersionNo {
		return nil, 0, &perr.HTTPError{
			Code:       perr.CodeStateConflict,
			Message:    "source active version changed; reload and retry",
			HTTPStatus: http.StatusConflict,
			Details: map[string]any{
				"source_type_id":             sourceTypeID,
				"expected_source_version_no": expectedVersionNo,
				"actual_source_version_no":   activeNo,
			},
		}
	}

	detail, err := s.repo.GetTypeVersionDetail(ctx, sub.CompanyID, sourceTypeID, activeNo)
	if err != nil {
		return nil, 0, err
	}
	ApplyLegalBasisReadCompat(ctx, detail, s.legalBasisLegacyFallbackEnabled, s.legalBasisDivergenceWarningEnabled)

	if !strings.EqualFold(strings.TrimSpace(detail.Scope), templateScopeGlobal) && strings.TrimSpace(detail.Scope) != "" {
		return nil, 0, &perr.HTTPError{
			Code:       "TEMPLATE_CLONE_SCOPE_UNSUPPORTED",
			Message:    "MVP clone supports global CMS templates only",
			HTTPStatus: http.StatusUnprocessableEntity,
			Details:    map[string]any{"source_type_id": sourceTypeID, "scope": detail.Scope},
		}
	}
	if strings.TrimSpace(detail.OwnerCompanyID) != "" {
		return nil, 0, &perr.HTTPError{
			Code:       "TEMPLATE_CLONE_SCOPE_UNSUPPORTED",
			Message:    "MVP clone supports global CMS templates only",
			HTTPStatus: http.StatusUnprocessableEntity,
			Details:    map[string]any{"source_type_id": sourceTypeID, "owner_company_id": detail.OwnerCompanyID},
		}
	}

	if err := validateCloneSourceWorkflow(detail); err != nil {
		return nil, 0, err
	}
	return detail, activeNo, nil
}

// validateCloneSourceWorkflow implements OD1:
// EMPTY + TEMPLATE_PINNED + explicit empty manifest => ALLOW (NO_WORKFLOW)
// non-empty invalid steps => REJECT 422
func validateCloneSourceWorkflow(detail *DisclosureTypeDTO) error {
	if detail == nil {
		return &perr.HTTPError{
			Code:       "TEMPLATE_CLONE_SOURCE_INVALID",
			Message:    "source publication is missing",
			HTTPStatus: http.StatusUnprocessableEntity,
		}
	}
	if detail.WorkflowAuthorityMode != WorkflowAuthorityTemplatePinned || detail.WorkflowManifest == nil {
		return &perr.HTTPError{
			Code:       "TEMPLATE_WORKFLOW_NOT_PINNED",
			Message:    "source active publication workflow is not TEMPLATE_PINNED",
			HTTPStatus: http.StatusUnprocessableEntity,
			Details: map[string]any{
				"type_id":                 detail.TypeID,
				"version_no":              detail.VersionNo,
				"workflow_authority_mode": detail.WorkflowAuthorityMode,
			},
		}
	}
	steps := workflowStepsFromDetail(detail)
	if len(steps) == 0 {
		// Explicit NO_WORKFLOW: TEMPLATE_PINNED + empty steps. No Global fallback.
		return nil
	}
	if err := ValidateWorkflowStepsForActivation(steps); err != nil {
		return &perr.HTTPError{
			Code:       "TEMPLATE_CLONE_SOURCE_WORKFLOW_INVALID",
			Message:    err.Error(),
			HTTPStatus: http.StatusUnprocessableEntity,
			Details: map[string]any{
				"type_id":    detail.TypeID,
				"version_no": detail.VersionNo,
			},
		}
	}
	return nil
}

func (s *service) materializeCloneUpsert(ctx context.Context, req CloneTypeFromActiveRequest, source *DisclosureTypeDTO) (UpsertTypeVersionRequest, error) {
	steps := sanitizeWorkflowStepsForClone(workflowStepsFromDetail(source))
	blocks, err := cloneTemplateBlocksForTarget(source.Blocks, steps, s.idg)
	if err != nil {
		return UpsertTypeVersionRequest{}, &perr.HTTPError{
			Code:       "TEMPLATE_CLONE_MATERIALIZE_FAILED",
			Message:    err.Error(),
			HTTPStatus: http.StatusUnprocessableEntity,
		}
	}

	legalBases, legalFlat, _ := PrepareLegalBasesForNewVersion(
		ctx, req.TargetTypeID, source.LegalBases, source.LegalBasis, nil, "", true, s.idg,
	)

	checklist := make([]ChecklistItemDTO, len(source.Checklist))
	copy(checklist, source.Checklist)

	tags := append([]string(nil), source.Tags...)
	displayGroups := append([]string(nil), source.DisplayGroupCodes...)

	var deadlineCfg *TemplateDeadlineConfig
	if source.DeadlineConfig != nil {
		cp := *source.DeadlineConfig
		deadlineCfg = &cp
	}

	upsert := UpsertTypeVersionRequest{
		Subject:               req.Subject,
		TypeID:                req.TargetTypeID,
		Scope:                 templateScopeGlobal,
		GroupID:               source.GroupID,
		Name:                  req.TargetName,
		Category:              source.Category,
		TemplateCategory:      source.TemplateCategory,
		DeadlineStrategy:      source.DeadlineStrategy,
		Description:           source.Description,
		LegalBasis:            legalFlat,
		Applicability:         source.Applicability,
		ImplementationContent: source.ImplementationContent,
		ImplementationNotes:   source.ImplementationNotes,
		SpecialCases:          source.SpecialCases,
		ReportContent:         source.ReportContent,
		RequiredDocs:          source.RequiredDocs,
		DeadlineRule:          source.DeadlineRule,
		Periodicity:           source.Periodicity,
		ChannelsText:          source.ChannelsText,
		Beneficiaries:         source.Beneficiaries,
		ReceivingAuthorities:  source.ReceivingAuthorities,
		Format:                source.Format,
		LegalRisksText:        source.LegalRisksText,
		GeneralInfo:           source.GeneralInfo,
		LegalBases:            legalBases,
		LegalBasesProvided:    true,
		Checklist:             checklist,
		Tags:                  tags,
		DeadlineConfig:        deadlineCfg,
		Blocks:                blocks,
		DisplayGroupCodes:     displayGroups,
		ChangeNote:            fmt.Sprintf("Cloned from %s v%d", req.SourceTypeID, req.ExpectedSourceVersionNo),
		ApplicabilityRules:    source.ApplicabilityRules,
		CreateOnly:            true,
		ClearWorkflow:         len(steps) == 0,
	}
	return upsert, nil
}

func sanitizeWorkflowStepsForClone(steps []WorkflowStepDTO) []WorkflowStepDTO {
	out := cloneWorkflowStepDTOs(steps)
	for i := range out {
		out[i].AssigneeMembershipID = ""
		out[i].AssigneeMembershipIDs = nil
	}
	return out
}

func cloneTemplateBlocksForTarget(sourceBlocks []TemplateBlockDTO, steps []WorkflowStepDTO, idg interface{ NewUUID() string }) ([]TemplateBlockDTO, error) {
	copied := make([]TemplateBlockDTO, 0, len(sourceBlocks))
	for _, block := range sourceBlocks {
		next := block
		if idg != nil {
			next.BlockID = idg.NewUUID()
		}
		next.Config = cloneAnyMapShallow(block.Config)
		next.Validation = cloneAnyMapShallow(block.Validation)
		if strings.EqualFold(strings.TrimSpace(block.BlockKey), "enterprise_workflow") {
			next.Description = ""
		}
		copied = append(copied, next)
	}
	return replaceTemplateWorkflowSteps(copied, steps)
}

func cloneAnyMapShallow(src map[string]any) map[string]any {
	if len(src) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

// CloneAuditMetadata builds audit metadata for the HTTP handler (audit-only provenance).
func CloneAuditMetadata(resp *CloneTypeFromActiveResponse) map[string]any {
	if resp == nil {
		return map[string]any{"creation_mode": creationModeTemplateClone}
	}
	return map[string]any{
		"creation_mode":     creationModeTemplateClone,
		"source_type_id":    resp.SourceTypeID,
		"source_version_no": resp.SourceVersionNo,
		"target_type_id":    resp.TypeID,
		"target_version_no": resp.VersionNo,
		"cloned_at":         time.Now().UTC().Format(time.RFC3339Nano),
	}
}
