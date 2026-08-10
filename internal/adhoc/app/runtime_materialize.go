package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

const (
	// MaterializationModeV2Snapshot is the observability label for frozen proposal workflow v2.
	MaterializationModeV2Snapshot = "v2_snapshot"
	// MaterializationModeV3Snapshot is the observability label for frozen proposal workflow v3.
	MaterializationModeV3Snapshot = "v3_snapshot"
	// MaterializationModeLegacy is the late-resolve template path.
	MaterializationModeLegacy = "legacy"
)

// ValidateFrozenProposalWorkflowForRuntime enforces defensive gates before frozen materialization (v2|v3).
// Invalid snapshots must FAIL with no template fallback.
func ValidateFrozenProposalWorkflowForRuntime(snap *ProposalWorkflowSnapshot) error {
	if snap == nil {
		return newAdHocFieldError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, "workflow", "proposal workflow snapshot is required for frozen materialization")
	}
	if snap.SchemaVersion != ProposalWorkflowSchemaV2 && snap.SchemaVersion != ProposalWorkflowSchemaV3 {
		return newAdHocFieldError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, "workflow.schema_version", "schema_version must be 2 or 3 for frozen snapshot materialization")
	}
	if !snap.Frozen {
		return newAdHocFieldError(http.StatusConflict, perr.CodeStateConflict, "workflow.frozen", "proposal workflow must be frozen before materialization")
	}
	if len(snap.Steps) < MinProposalWorkflowSteps {
		return newAdHocFieldError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, "workflow.steps", "frozen workflow must contain at least one step")
	}
	if len(snap.Steps) > MaxProposalWorkflowSteps {
		return newAdHocFieldError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, "workflow.steps", fmt.Sprintf("frozen workflow exceeds technical maximum of %d steps", MaxProposalWorkflowSteps))
	}
	seenOrders := make(map[int]struct{}, len(snap.Steps))
	for i, step := range snap.Steps {
		prefix := fmt.Sprintf("workflow.steps[%d]", i)
		if strings.TrimSpace(step.ID) == "" {
			return newAdHocFieldError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, prefix+".id", "proposal step id is required")
		}
		if strings.TrimSpace(step.Name) == "" {
			return newAdHocFieldError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, prefix+".name", "name is required")
		}
		if step.ProcessingDays < 0 {
			return newAdHocFieldError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, prefix+".processing_days", "processing_days must be >= 0")
		}
		wantOrder := i + 1
		if step.Order != wantOrder {
			return newAdHocFieldError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, prefix+".order", fmt.Sprintf("order must be contiguous starting at 1 (want %d got %d)", wantOrder, step.Order))
		}
		if _, dup := seenOrders[step.Order]; dup {
			return newAdHocFieldError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, prefix+".order", "duplicate step order")
		}
		seenOrders[step.Order] = struct{}{}
	}
	return nil
}

// ValidateDirectAssigneeRequired locks runtime/submit model A for schema v2:
// every step must have department_id + assignee_membership_id (no creator/approver fallback,
// no department queue).
func ValidateDirectAssigneeRequired(snap *ProposalWorkflowSnapshot) error {
	if snap == nil {
		return newAdHocFieldError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, "workflow", "proposal workflow snapshot is required")
	}
	for i, step := range snap.Steps {
		dept := strings.TrimSpace(step.DepartmentID)
		assignee := strings.TrimSpace(step.AssigneeMembershipID)
		if dept == "" {
			return newAdHocFieldError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, fmt.Sprintf("workflow_steps[%d].department_id", i), "department_required: department_id is required")
		}
		if assignee == "" {
			return newAdHocFieldError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, fmt.Sprintf("workflow_steps[%d].assignee_membership_id", i), "assignee_required: assignee_membership_id is required")
		}
	}
	return nil
}

// ValidateV3AssigneesRequired locks runtime materialization for schema v3:
// every step must have department_id + non-empty frozen assignee_membership_ids.
func ValidateV3AssigneesRequired(snap *ProposalWorkflowSnapshot) error {
	if snap == nil {
		return newAdHocFieldError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, "workflow", "proposal workflow snapshot is required")
	}
	for i, step := range snap.Steps {
		dept := strings.TrimSpace(step.DepartmentID)
		if dept == "" {
			return newAdHocFieldError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, fmt.Sprintf("workflow_steps[%d].department_id", i), "department_required: department_id is required")
		}
		ids := EffectiveAssigneeMembershipIDs(step, ProposalWorkflowSchemaV3)
		if len(ids) == 0 {
			return newAdHocFieldError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, fmt.Sprintf("workflow_steps[%d].assignee_membership_ids", i), "assignee_required: assignee_membership_ids must be non-empty for v3 materialization")
		}
	}
	return nil
}

// ValidateWorkflowForSubmit enforces assignment rules before freeze/status transition.
// Draft/PATCH may remain incomplete; submit must not.
func ValidateWorkflowForSubmit(ctx context.Context, org OrgDirectory, companyID string, snap *ProposalWorkflowSnapshot) error {
	if snap == nil {
		return nil
	}
	switch snap.SchemaVersion {
	case ProposalWorkflowSchemaV2:
		if err := ValidateDirectAssigneeRequired(snap); err != nil {
			return err
		}
		if org == nil {
			return perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "org directory is required for v2 submit validation", nil)
		}
		return ValidateWorkflowStepOrgRefs(ctx, org, companyID, snap.Steps)
	case ProposalWorkflowSchemaV3:
		// Head resolution + non-empty freeze is owned by NormalizeAndValidateWorkflowForSubmitV3.
		return nil
	default:
		return nil
	}
}

// PrepareV2Materialization validates frozen snapshot + model A assignment and re-checks org refs.
// Call before CreateAndSubmitRecordWithOpts for schema_version=2 proposals.
func PrepareV2Materialization(ctx context.Context, org OrgDirectory, companyID string, snap *ProposalWorkflowSnapshot) error {
	if err := ValidateFrozenProposalWorkflowForRuntime(snap); err != nil {
		return err
	}
	if snap.SchemaVersion != ProposalWorkflowSchemaV2 {
		return newAdHocFieldError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, "workflow.schema_version", "PrepareV2Materialization requires schema_version=2")
	}
	if err := ValidateDirectAssigneeRequired(snap); err != nil {
		return err
	}
	if org == nil {
		return perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "org directory is required for v2 materialization", nil)
	}
	if err := ValidateWorkflowStepOrgRefs(ctx, org, companyID, snap.Steps); err != nil {
		return err
	}
	return nil
}

// PrepareV3Materialization validates frozen v3 snapshot + non-empty assignees and re-checks org refs.
func PrepareV3Materialization(ctx context.Context, org OrgDirectory, companyID string, snap *ProposalWorkflowSnapshot) error {
	if err := ValidateFrozenProposalWorkflowForRuntime(snap); err != nil {
		return err
	}
	if snap.SchemaVersion != ProposalWorkflowSchemaV3 {
		return newAdHocFieldError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, "workflow.schema_version", "PrepareV3Materialization requires schema_version=3")
	}
	if err := ValidateV3AssigneesRequired(snap); err != nil {
		return err
	}
	if org == nil {
		return perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "org directory is required for v3 materialization", nil)
	}
	if err := ValidateWorkflowStepOrgRefs(ctx, org, companyID, snap.Steps); err != nil {
		return err
	}
	return nil
}

// FirstStepAssigneeMembershipID returns the direct assignee for the first ordered step (v2 singular).
// Must not be used as a v3 first-assignee fallback for task materialization.
func FirstStepAssigneeMembershipID(snap *ProposalWorkflowSnapshot) string {
	if snap == nil || len(snap.Steps) == 0 {
		return ""
	}
	return strings.TrimSpace(snap.Steps[0].AssigneeMembershipID)
}

// FirstStepAssigneeMembershipIDs returns frozen v3 assignees for the first ordered step.
func FirstStepAssigneeMembershipIDs(snap *ProposalWorkflowSnapshot) []string {
	if snap == nil || len(snap.Steps) == 0 {
		return nil
	}
	return EffectiveAssigneeMembershipIDs(snap.Steps[0], ProposalWorkflowSchemaV3)
}

// BuildCreateRecordOptsForFinalize selects frozen snapshot (v2|v3) vs legacy step_overrides.
func BuildCreateRecordOptsForFinalize(recordID string, proposal *ProposalDTO) (CreateRecordOpts, string, error) {
	opts := CreateRecordOpts{RecordID: recordID}
	if proposal == nil {
		return opts, MaterializationModeLegacy, nil
	}
	ver := ResolveProposalWorkflowContractVersion(proposal.Workflow, proposal.StepOverrides)
	switch ver {
	case ProposalWorkflowSchemaV3:
		opts.ProposalWorkflow = proposal.Workflow
		opts.StepOverrides = nil
		return opts, MaterializationModeV3Snapshot, nil
	case ProposalWorkflowSchemaV2:
		opts.ProposalWorkflow = proposal.Workflow
		opts.StepOverrides = nil
		return opts, MaterializationModeV2Snapshot, nil
	default:
		opts.StepOverrides = append([]WorkflowStepOverride(nil), proposal.StepOverrides...)
		return opts, MaterializationModeLegacy, nil
	}
}
