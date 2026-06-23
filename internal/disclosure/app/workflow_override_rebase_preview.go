package app

import (
	"context"
	"net/http"
	"time"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// Sprint 3 / Batch 3 — Workflow Override Rebase Preview.
//
// This file owns: the request/response contracts, the step-identity bridging logic
// (PREFLIGHT_AUDIT.md §5), and the orchestrating service method. It performs ZERO writes — every
// repository call below is one of the existing read-only methods (Batch 2's staleness metadata
// reads, the existing override-snapshot reader, this batch's new GetGlobalWorkflowVersionManifest).
// The actual diff/conflict computation is delegated entirely to workflow_override_diff.go's pure
// ComputeRebaseDiff — this file never inspects field values itself.

type GetWorkflowOverrideRebasePreviewRequest struct {
	Subject Subject
	TypeID  string
}

type WorkflowOverrideRebasePreviewDTO struct {
	PreviewID       *string           `json:"preview_id"`
	TypeID          string            `json:"type_id"`
	CompanyID       string            `json:"company_id"`
	BaseVersionNo   int               `json:"base_version_no"`
	TargetVersionNo int               `json:"target_version_no"`
	StaleStatus     string            `json:"stale_status"`
	GeneratedAt     time.Time         `json:"generated_at"`
	PatchOperations []PatchOperation  `json:"patch_operations"`
	Conflicts       []PreviewConflict `json:"conflicts"`
}

type GetWorkflowOverrideRebasePreviewResponse struct {
	Data WorkflowOverrideRebasePreviewDTO `json:"data"`
}

// GetWorkflowOverrideRebasePreview is the Batch 3 service method. Gating order: type existence
// (404 NOT_FOUND) -> override existence (404 OVERRIDE_NOT_FOUND) -> staleness (409 NOT_STALE if
// current) -> base/target manifest decodability (410 PREVIEW_UNAVAILABLE if either is missing).
// Permission is template.workflow.override.write per API_CONTRACT_PROPOSAL.md (a preview is a
// prerequisite step toward a future write action, gated at write-tier even though this batch
// itself writes nothing).
func (s *service) GetWorkflowOverrideRebasePreview(ctx context.Context, req GetWorkflowOverrideRebasePreviewRequest) (*GetWorkflowOverrideRebasePreviewResponse, error) {
	typeID, err := validateStalenessTypeID(req.TypeID)
	if err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, req.Subject, "template.workflow.override.write", authapp.ResourceRef{
		Type: "disclosure_type",
		ID:   typeID,
	}); err != nil {
		return nil, err
	}
	exists, err := s.repo.TypeExists(ctx, typeID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, typeNotFoundErr(typeID)
	}

	currentGlobalActiveVersionNo, err := s.repo.GetCurrentGlobalActiveVersionNo(ctx, typeID)
	if err != nil {
		return nil, err
	}
	row, hasRow, err := s.repo.GetOverrideStalenessMetadata(ctx, req.Subject.CompanyID, typeID)
	if err != nil {
		return nil, err
	}
	if !hasRow || !row.HasOverride {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeOverrideNotFound, "no workflow override exists for this company and type", nil)
	}

	staleStatus, anomaly := ComputeStaleStatus(OverrideStalenessMetadata{
		HasOverride:   row.HasOverride,
		BaseSource:    row.BaseSource,
		BaseVersionNo: row.BaseVersionNo,
	}, currentGlobalActiveVersionNo)
	LogStalenessAnomaly(ctx, req.Subject.CompanyID, typeID, anomaly)

	if staleStatus == StaleStatusCurrent {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeNotStale, "override is already current; nothing to preview", nil)
	}
	if row.BaseSource != BaseSourceGlobalWorkflow || row.BaseVersionNo == nil || currentGlobalActiveVersionNo == nil {
		// Unknown/legacy base, or no current active global version at all: there is no global
		// manifest pair to diff against. Do not guess, do not return a partial preview.
		return nil, perr.NewHTTPError(http.StatusGone, perr.CodePreviewUnavailable, "cannot compute a preview: base or current global workflow version is not determinable", nil)
	}
	baseVersionNo := *row.BaseVersionNo
	targetVersionNo := *currentGlobalActiveVersionNo

	baseManifest, baseOK, err := s.repo.GetGlobalWorkflowVersionManifest(ctx, typeID, baseVersionNo)
	if err != nil {
		return nil, err
	}
	targetManifest, targetOK, err := s.repo.GetGlobalWorkflowVersionManifest(ctx, typeID, targetVersionNo)
	if err != nil {
		return nil, err
	}
	if !baseOK || !targetOK {
		return nil, perr.NewHTTPError(http.StatusGone, perr.CodePreviewUnavailable, "base or target global workflow version could not be decoded", nil)
	}

	overrideView, err := s.repo.GetCompanyWorkflowOverride(ctx, req.Subject.CompanyID, typeID)
	if err != nil {
		return nil, err
	}
	var companySteps []WorkflowStepDTO
	if overrideView != nil && overrideView.ActiveVersion != nil {
		companySteps = overrideView.ActiveVersion.Workflow
	}

	baseInputs, targetInputs, companyInputs := buildDiffStepInputs(baseManifest, targetManifest, companySteps)
	ops, conflicts := ComputeRebaseDiff(baseInputs, targetInputs, companyInputs)

	dto := WorkflowOverrideRebasePreviewDTO{
		PreviewID:       nil, // ephemeral, no durable identity in Batch 3 — see SCOPE_DRIFT_GUARD.md
		TypeID:          typeID,
		CompanyID:       req.Subject.CompanyID,
		BaseVersionNo:   baseVersionNo,
		TargetVersionNo: targetVersionNo,
		StaleStatus:     staleStatus,
		GeneratedAt:     time.Now().UTC(),
		PatchOperations: ops,
		Conflicts:       conflicts,
	}
	return &GetWorkflowOverrideRebasePreviewResponse{Data: dto}, nil
}

// buildDiffStepInputs implements the step-identity bridging documented in PREFLIGHT_AUDIT.md §5:
//  1. base manifest steps are keyed by their own step_key (the global side always has one once
//     mig-S1 minted it; falls back to step_id only for the rare pre-mig-S1 row, never silently
//     treated as unclear when a step_id is at least present).
//  2. target manifest steps are keyed the same way — this is what makes a step rename across
//     global versions still match correctly (the whole reason step_key exists).
//  3. company (override) steps are bridged via the BASE manifest's step_id->step_key map (the
//     override's StepID was seeded verbatim from the base global step's StepID at snapshot time —
//     see PREFLIGHT_AUDIT.md §4/§5). A company step whose StepID has no counterpart in the base
//     manifest is treated as company-original — its own StepID is used as its key, since there is
//     nothing to bridge to and it is only ever compared against itself.
func buildDiffStepInputs(baseManifest, targetManifest []GlobalWorkflowStepInput, companySteps []WorkflowStepDTO) (base, target, company []DiffStepInput) {
	baseStepIDToKey := map[string]string{}

	for _, ms := range baseManifest {
		key := resolveManifestStepKey(ms)
		baseStepIDToKey[ms.StepID] = key
		dto := manifestStepToComparable(ms)
		base = append(base, DiffStepInput{Key: key, Step: &dto})
	}
	for _, ms := range targetManifest {
		key := resolveManifestStepKey(ms)
		dto := manifestStepToComparable(ms)
		target = append(target, DiffStepInput{Key: key, Step: &dto})
	}
	for i := range companySteps {
		step := companySteps[i]
		key, bridged := baseStepIDToKey[step.StepID]
		if !bridged {
			key = step.StepID
		}
		company = append(company, DiffStepInput{Key: key, Step: &step})
	}
	return base, target, company
}

// resolveManifestStepKey prefers the manifest's own step_key (mig-S1); a step minted before
// mig-S1 existed has no step_key recorded, so its step_id is used instead — this is a
// determinable identity (not the STEP_IDENTITY_UNCLEAR case, which is reserved for when NEITHER
// is usable, which cannot happen here since step_id is always required/non-empty).
func resolveManifestStepKey(ms GlobalWorkflowStepInput) string {
	if ms.StepKey != "" {
		return ms.StepKey
	}
	return ms.StepID
}

// manifestStepToComparable maps a GlobalWorkflowStepInput onto the shared WorkflowStepDTO shape
// the diff engine compares, so the SAME field-comparison code (workflow_override_diff.go) runs
// for both the global side and the override-snapshot side. Documents/Groups/ReminderConfig are
// left nil/zero — the global manifest schema has no such fields (Baseline §8 "Manifest field
// gap") — the diff engine's own normalization treats nil and [] as equal, so this never produces
// a false "changed" detection purely from the global side's structural absence of these fields.
func manifestStepToComparable(ms GlobalWorkflowStepInput) WorkflowStepDTO {
	return WorkflowStepDTO{
		StepID:          ms.StepID,
		Stage:           ms.Stage,
		DepartmentID:    ms.DepartmentID,
		AssigneeRoleIds: ms.AssigneeRoleIds,
		DueRule:         ms.DueRule,
		ProcessingDays:  ms.ProcessingDays,
		DisplayOrder:    ms.DisplayOrder,
	}
}
