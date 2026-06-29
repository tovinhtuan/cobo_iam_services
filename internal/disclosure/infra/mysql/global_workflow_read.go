package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

// activeGlobalWorkflowManifestStep mirrors the JSON shape workflowconfig writes into
// global_workflow_versions.steps_manifest_json (workflowconfig/app.ManifestStep). Read directly
// via SQL here rather than importing workflowconfig, per ADR_WORKFLOW_DATA_SOURCE_ALIGNMENT.md —
// disclosure already reads global_workflows/global_workflow_steps directly (cms_repository.go);
// this keeps the same no-new-cross-module-dependency pattern.
type activeGlobalWorkflowManifestStep struct {
	StepID         string `json:"step_id"`
	StepKey        string `json:"step_key"`
	Stage          string `json:"stage"`
	Name           string `json:"name"`
	Instructions   string `json:"instructions,omitempty"`
	Role           string `json:"role"`
	DepartmentID   string `json:"department_id"`
	DueRule        string `json:"due_rule"`
	ProcessingDays int    `json:"processing_days"`
	DisplayOrder   int    `json:"display_order"`
}

// activeGlobalWorkflowManifest mirrors workflowconfig/app.Manifest's JSON shape — the full
// envelope persisted in steps_manifest_json, not a bare step array.
type activeGlobalWorkflowManifest struct {
	TypeID     string                             `json:"type_id"`
	WorkflowID string                             `json:"workflow_id"`
	VersionNo  int                                `json:"version_no"`
	Steps      []activeGlobalWorkflowManifestStep `json:"steps"`
}

// loadActiveGlobalWorkflow returns the steps of the ACTIVE global_workflow_versions row for
// typeID (if any), converted to disclosureapp.WorkflowStepDTO, plus its version number. ok=false
// means no active global workflow version exists for this type — caller must fall back to the
// legacy enterprise_workflow content block (ADR target behavior, step 2 of 4).
func (r *Repository) loadActiveGlobalWorkflow(ctx context.Context, typeID string) ([]disclosureapp.WorkflowStepDTO, int, bool, error) {
	var versionNo int
	var manifestJSON []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT v.version_no, v.steps_manifest_json
		FROM global_workflows w
		JOIN global_workflow_versions v ON v.type_id = w.type_id AND v.version_no = w.active_version_no
		WHERE w.type_id = ? AND w.status = 'active' AND w.active_version_no IS NOT NULL
		LIMIT 1
	`, typeID).Scan(&versionNo, &manifestJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, 0, false, nil
		}
		return nil, 0, false, fmt.Errorf("load active global workflow: %w", err)
	}

	manifestSteps, err := decodeGlobalWorkflowManifestSteps(manifestJSON)
	if err != nil {
		return nil, 0, false, fmt.Errorf("unmarshal active global workflow manifest (type=%s, version=%d): %w", typeID, versionNo, err)
	}

	steps := make([]disclosureapp.WorkflowStepDTO, 0, len(manifestSteps))
	for _, ms := range manifestSteps {
		dueRule := ms.DueRule
		if dueRule == "" && ms.ProcessingDays > 0 {
			dueRule = fmt.Sprintf("T+%d", ms.ProcessingDays)
		}
		var roleIDs []string
		if ms.Role != "" {
			roleIDs = []string{ms.Role}
		}
		steps = append(steps, disclosureapp.WorkflowStepDTO{
			StepID:          ms.StepID,
			Stage:           ms.Stage,
			Instructions:    ms.Instructions,
			DepartmentID:    ms.DepartmentID,
			AssigneeRoleIds: roleIDs,
			DueRule:         dueRule,
			ProcessingDays:  ms.ProcessingDays,
			DisplayOrder:    ms.DisplayOrder,
		})
	}
	return steps, versionNo, true, nil
}

// GetGlobalWorkflowVersionManifest is Sprint 3 / Batch 3's read for an ARBITRARY version_no
// (not just the active one) — needed for the rebase-preview's three-way comparison, which must
// read BOTH base_version_no and the current active version. Unlike loadActiveGlobalWorkflow,
// this preserves step_key (PREFLIGHT_AUDIT.md §5/§6 — the only existing global-side read path
// that does). Read-only: a single SELECT, no write of any kind. ok=false means no such
// (typeID, versionNo) row exists — caller returns 410 PREVIEW_UNAVAILABLE, never a partial result.
func (r *Repository) GetGlobalWorkflowVersionManifest(ctx context.Context, typeID string, versionNo int) ([]disclosureapp.GlobalWorkflowStepInput, bool, error) {
	var manifestJSON []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT steps_manifest_json FROM global_workflow_versions WHERE type_id = ? AND version_no = ?
	`, typeID, versionNo).Scan(&manifestJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("load global workflow version manifest (type=%s, version=%d): %w", typeID, versionNo, err)
	}

	manifestSteps, err := decodeGlobalWorkflowManifestSteps(manifestJSON)
	if err != nil {
		return nil, false, fmt.Errorf("unmarshal global workflow version manifest (type=%s, version=%d): %w", typeID, versionNo, err)
	}

	steps := make([]disclosureapp.GlobalWorkflowStepInput, 0, len(manifestSteps))
	for _, ms := range manifestSteps {
		dueRule := ms.DueRule
		if dueRule == "" && ms.ProcessingDays > 0 {
			dueRule = fmt.Sprintf("T+%d", ms.ProcessingDays)
		}
		var roleIDs []string
		if ms.Role != "" {
			roleIDs = []string{ms.Role}
		}
		steps = append(steps, disclosureapp.GlobalWorkflowStepInput{
			StepKey:         ms.StepKey,
			StepID:          ms.StepID,
			Stage:           ms.Stage,
			Instructions:    ms.Instructions,
			DepartmentID:    ms.DepartmentID,
			AssigneeRoleIds: roleIDs,
			DueRule:         dueRule,
			ProcessingDays:  ms.ProcessingDays,
			DisplayOrder:    ms.DisplayOrder,
		})
	}
	return steps, true, nil
}

// decodeGlobalWorkflowManifestSteps tolerates two on-disk shapes of steps_manifest_json:
//   - the envelope workflowconfig's Publish() writes: {"type_id":..,"workflow_id":..,"version_no":..,"steps":[...]}
//   - a bare step array: [...] — the shape migrations/0101's backfill wrote directly via
//     JSON_ARRAYAGG for the original 4 pre-Sprint-0-2 types (confirmed live on DEV: type
//     dt-sys-board-resolution's active v1 row is a bare array, not the envelope). Versions
//     created since via the real Publish() flow use the envelope; trying it first matches the
//     common case, falling back to the bare array keeps both readable without a data migration.
func decodeGlobalWorkflowManifestSteps(raw []byte) ([]activeGlobalWorkflowManifestStep, error) {
	var envelope activeGlobalWorkflowManifest
	if err := json.Unmarshal(raw, &envelope); err == nil && envelope.Steps != nil {
		return envelope.Steps, nil
	}
	var bareSteps []activeGlobalWorkflowManifestStep
	if err := json.Unmarshal(raw, &bareSteps); err != nil {
		return nil, err
	}
	return bareSteps, nil
}
