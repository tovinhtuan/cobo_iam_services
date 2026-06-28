package app

import (
	"context"
	"errors"
	"net/http"
	"time"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// Sprint 3 / Batch 5 — Workflow Override Rebase Apply.
//
// This is the only file in all of Sprint 3 whose service method can change what
// GetEffectiveWorkflow returns (PREFLIGHT_AUDIT.md §8). Every other read along the way
// (computeRebasePreview) is reused verbatim from Batch 3/4 and performs ZERO writes; the single
// write happens inside s.repo.ApplyWorkflowOverrideRebase, behind one DB transaction
// (PATCH_APPLY_ENGINE_REPORT.md / DB_WRITE_BOUNDARY_REPORT.md).

type ApplyWorkflowOverrideRebaseRequest struct {
	Subject   Subject
	TypeID    string
	PreviewID string
}

type ApplyWorkflowOverrideRebaseDTO struct {
	OverrideID     string    `json:"override_id"`
	NewVersionNo   int       `json:"new_version_no"`
	State          string    `json:"state"`
	AppliedAt      time.Time `json:"applied_at"`
	WorkflowSource string    `json:"workflow_source"`
}

type ApplyWorkflowOverrideRebaseResponse struct {
	Data ApplyWorkflowOverrideRebaseDTO `json:"data"`
}

// ApplyWorkflowOverrideRebaseParams is what the repository transaction actually needs — already
// fully validated and computed by the service layer; the repository's only remaining
// responsibility is the atomic write plus one final race guard (PATCH_APPLY_ENGINE_REPORT.md).
type ApplyWorkflowOverrideRebaseParams struct {
	CompanyID               string
	TypeID                  string
	ExpectedActiveVersionNo int // race guard: fails the write if it no longer matches at commit time
	NewSnapshot             []WorkflowStepDTO
	NewBaseVersionNo        int
	UserID                  string
	Now                     time.Time
}

type ApplyWorkflowOverrideRebaseResult struct {
	OverrideID   string
	NewVersionNo int
	AppliedAt    time.Time
}

// ApplyWorkflowOverrideRebase implements the gating order the task itself enumerates
// (Apply Preconditions 1-10), reusing computeRebasePreview for everything reuse-able rather than
// re-deriving staleness/manifest logic a second time:
//
//  1. type exists                         -> computeRebasePreview (404 NOT_FOUND, reused)
//  2. override exists                     -> computeRebasePreview (404 OVERRIDE_NOT_FOUND, reused)
//  3. override is stale                   -> computeRebasePreview (409 NOT_STALE if NOT stale, reused)
//  4/5. preview target/base still current -> compared against the cached preview entry below
//  6/7. all conflicts resolved            -> scanned from the freshly recomputed conflict list
//  8. Rule 8 deferred-validation blocker   -> RULE_8_APPLY_POLICY.md's exact check
//  9. patch application preserves fields  -> ApplyPatchOperations' own design guarantee
//  10. idempotency key valid              -> enforced at the HTTP handler, before this is ever called
func (s *service) ApplyWorkflowOverrideRebase(ctx context.Context, req ApplyWorkflowOverrideRebaseRequest) (*ApplyWorkflowOverrideRebaseResponse, error) {
	typeID, err := validateStalenessTypeID(req.TypeID)
	if err != nil {
		return nil, err
	}
	if req.PreviewID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "preview_id is required", nil)
	}
	if err := s.authorize(ctx, req.Subject, "template.workflow.override.write", authapp.ResourceRef{
		Type: "disclosure_type",
		ID:   typeID,
	}); err != nil {
		return nil, err
	}

	cached, found, expired := globalPreviewCache.Get(req.PreviewID)
	if !found {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodePreviewRequired, "no preview found for this preview_id; call rebase-preview first", nil)
	}
	if expired {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodePreviewExpired, "preview has expired; call rebase-preview again", nil)
	}
	// Tenant boundary: a preview_id minted for a different company/type can never be used here —
	// company_id comes from the JWT subject only, never trusted from the request, and this
	// preview_id was minted FOR a specific company/type at preview time (PREFLIGHT_AUDIT.md §2).
	if cached.CompanyID != req.Subject.CompanyID || cached.TypeID != typeID {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodePreviewRequired, "preview_id does not belong to this company/type", nil)
	}

	computed, err := s.computeRebasePreview(ctx, req.Subject, typeID)
	if err != nil {
		return nil, err // reuses 404 NOT_FOUND / 404 OVERRIDE_NOT_FOUND / 409 NOT_STALE / 410 PREVIEW_UNAVAILABLE as-is
	}

	// Preconditions 4/5: the preview the caller is applying must still describe the CURRENT
	// base/target pair. If the global workflow activated a newer version, or the override's own
	// base moved, between preview and apply, the preview is stale — reject, never silently apply
	// against state the caller never actually saw.
	if computed.BaseVersionNo != cached.BaseVersionNo || computed.TargetVersionNo != cached.TargetVersionNo {
		return nil, perr.NewHTTPError(http.StatusGone, perr.CodePreviewUnavailable, "preview is stale relative to current base/target versions; re-run rebase-preview", nil)
	}

	// Preconditions 6/7: every conflict detected for this preview must be resolved. Build the
	// resolution lookup the patch engine needs at the same time (ConflictResolutionLookup keyed by
	// step_key|field_path, the same key opFieldPath/ApplyPatchOperations uses).
	resolutions := map[string]ConflictResolutionLookup{}
	unresolvedCount := 0
	for _, c := range computed.PersistedConflicts {
		// Precondition 7's exact wording: only resolution_status=unresolved blocks apply.
		// resolved and ignored (the latter currently unreachable via any code path, but checked
		// explicitly rather than by exclusion so this stays correct if that ever changes) both
		// let apply proceed.
		if c.ResolutionStatus == ResolutionStatusUnresolved {
			unresolvedCount++
			continue
		}
		resolution := ""
		if c.Resolution != nil {
			resolution = *c.Resolution
		}
		resolutions[c.StepKey+"|"+c.FieldPath] = ConflictResolutionLookup{
			Resolution:      resolution,
			ResolutionValue: c.ResolutionValue,
		}
	}
	if unresolvedCount > 0 {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeConflictsUnresolved, "all conflicts must be resolved before apply", nil)
	}

	// Bridge resolved FIELD-level conflicts (stage/department_id/documents/groups/
	// assignee_role_ids/due_rule/reminder_config — Rule 1) into real ops: the diff engine never
	// emits an op for a field it ALSO flagged as conflicted (workflow_override_patch_apply.go's
	// fieldConflictResolutionToOp doc comment explains why), so without this, accept_global/
	// merge_manual on a field conflict would have nothing to apply. Existence conflicts
	// (__step_existence__, Rule 2/6) are intentionally excluded — those are already handled by
	// ApplyPatchOperations' own resolution-aware add_step/remove_step branches.
	allOps := computed.Ops
	for _, c := range computed.PersistedConflicts {
		if c.FieldPath == "__step_existence__" {
			continue
		}
		resolution := ""
		if c.Resolution != nil {
			resolution = *c.Resolution
		}
		if op := fieldConflictResolutionToOp(c.StepKey, c.FieldPath, resolution, c.GlobalNew, c.ResolutionValue); op != nil {
			allOps = append(allOps, *op)
		}
	}

	// Precondition 8 / RULE_8_APPLY_POLICY.md: block if the patch touches department_id anywhere
	// — Rule 8 (department validity) is deferred; Batch 5 never silently applies a department
	// reassignment it cannot prove is still valid.
	for _, op := range allOps {
		if op.Op == OpUpdateStepField && op.FieldPath == "department_id" {
			return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeDeferredValidationBlock, "apply blocked: step "+op.StepKey+" changes department_id, which Rule 8 validation does not yet cover", nil)
		}
	}

	newSnapshot, err := ApplyPatchOperations(computed.CompanySteps, allOps, resolutions, computed.KeyOf)
	if err != nil {
		if errors.Is(err, ErrInvalidPatch) {
			return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidPatch, err.Error(), err)
		}
		return nil, err
	}
	if err := ValidateCompanyWorkflowOverrideSteps(newSnapshot); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	result, err := s.repo.ApplyWorkflowOverrideRebase(ctx, ApplyWorkflowOverrideRebaseParams{
		CompanyID:               req.Subject.CompanyID,
		TypeID:                  typeID,
		ExpectedActiveVersionNo: computed.OverrideVersionNo,
		NewSnapshot:             newSnapshot,
		NewBaseVersionNo:        computed.TargetVersionNo,
		UserID:                  req.Subject.UserID,
		Now:                     now,
	})
	if err != nil {
		return nil, err
	}

	return &ApplyWorkflowOverrideRebaseResponse{Data: ApplyWorkflowOverrideRebaseDTO{
		OverrideID:     result.OverrideID,
		NewVersionNo:   result.NewVersionNo,
		State:          "approved",
		AppliedAt:      result.AppliedAt,
		WorkflowSource: "company_override",
	}}, nil
}
