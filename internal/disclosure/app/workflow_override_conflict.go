package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	wfcapp "github.com/cobo/cobo_iam_services/internal/workflowconfig/app"
)

// Sprint 3 / Batch 4 — Workflow Override Conflict Detection: persistence contracts, the
// conflict_key idempotency function, and the ResolveWorkflowOverrideConflict service method.
// Preview-time persistence orchestration lives in workflow_override_rebase_preview.go (the
// existing Batch 3 entry point, enhanced).

// Resolution status values (task's exact allowed set).
const (
	ResolutionStatusUnresolved = "unresolved"
	ResolutionStatusResolved   = "resolved"
	ResolutionStatusIgnored    = "ignored"
)

// allowedResolutions is the exact 5-value set the resolve endpoint accepts.
var allowedResolutions = map[string]bool{
	ResolutionKeepCompany:       true,
	ResolutionAcceptGlobal:      true,
	ResolutionMergeManual:       true,
	ResolutionCreateNewStep:     true,
	ResolutionMarkNotApplicable: true,
}

// PersistedConflictInput is what the preview-persistence step writes — a PreviewConflict plus
// the row-identity fields needed to build a stable conflict_key and satisfy the table schema.
type PersistedConflictInput struct {
	PreviewConflict
	CompanyID         string
	TypeID            string
	OverrideID        string
	OverrideVersionNo int
	BaseVersionNo     int
	TargetVersionNo   int
	CreatedBy         string
}

// PersistedConflictDTO is the durable, API-facing shape — adds `id`/`resolution_status`/etc. to
// PreviewConflict's fields. Never read by GetEffectiveWorkflow or any runtime path.
type PersistedConflictDTO struct {
	ID                string     `json:"id"`
	PreviewID         *string    `json:"preview_id"`
	CompanyID         string     `json:"company_id"`
	TypeID            string     `json:"type_id"`
	OverrideID        string     `json:"override_id,omitempty"`
	OverrideVersionNo int        `json:"override_version_no,omitempty"`
	BaseVersionNo     int        `json:"base_version_no"`
	TargetVersionNo   int        `json:"target_version_no"`
	StepKey           string     `json:"step_key"`
	FieldPath         string     `json:"field_path"`
	Severity          string     `json:"severity"`
	ConflictType      string     `json:"conflict_type"`
	GlobalOld         any        `json:"global_old"`
	GlobalNew         any        `json:"global_new"`
	CompanyValue      any        `json:"company_value"`
	ResolutionStatus  string     `json:"resolution_status"`
	Resolution        *string    `json:"resolution,omitempty"`
	ResolutionOptions []string   `json:"resolution_options"`
	CreatedAt         time.Time  `json:"created_at"`
	ResolvedBy        *string    `json:"resolved_by,omitempty"`
	ResolvedAt        *time.Time `json:"resolved_at,omitempty"`
}

// BuildConflictKey is the Option B idempotency function (PREFLIGHT_AUDIT.md §8): a deterministic
// key over the fields that define "the same conflict, detected again." A NEW base or target
// version_no naturally produces a NEW key, so a genuinely new staleness situation creates
// genuinely new rows rather than colliding with old, possibly-already-resolved ones.
func BuildConflictKey(companyID, typeID string, baseVersionNo, targetVersionNo int, stepKey, fieldPath, conflictType string) string {
	raw := fmt.Sprintf("%s|%s|%d|%d|%s|%s|%s", companyID, typeID, baseVersionNo, targetVersionNo, stepKey, fieldPath, conflictType)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// ── Resolve endpoint contracts ──────────────────────────────────────────────────────────────────

type ResolveWorkflowOverrideConflictRequest struct {
	Subject         Subject
	TypeID          string
	ConflictID      string
	Resolution      string
	ResolutionValue any
}

type ResolveWorkflowOverrideConflictResponse struct {
	Data PersistedConflictDTO `json:"data"`
}

func (s *service) ResolveWorkflowOverrideConflict(ctx context.Context, req ResolveWorkflowOverrideConflictRequest) (*ResolveWorkflowOverrideConflictResponse, error) {
	typeID, err := validateStalenessTypeID(req.TypeID)
	if err != nil {
		return nil, err
	}
	conflictID := strings.TrimSpace(req.ConflictID)
	if conflictID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "conflict id is required", nil)
	}
	resolution := strings.TrimSpace(req.Resolution)
	if !allowedResolutions[resolution] {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidResolution, "invalid resolution value", nil)
	}
	if err := s.authorize(ctx, req.Subject, "template.workflow.override.write", authapp.ResourceRef{
		Type: "disclosure_type",
		ID:   typeID,
	}); err != nil {
		return nil, err
	}

	existing, err := s.repo.GetWorkflowOverrideConflict(ctx, req.Subject.CompanyID, typeID, conflictID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "conflict not found", nil)
	}

	if existing.ResolutionStatus == ResolutionStatusResolved {
		if existing.Resolution != nil && *existing.Resolution == resolution {
			// Same resolution on an already-resolved conflict: idempotent no-op success.
			return &ResolveWorkflowOverrideConflictResponse{Data: *existing}, nil
		}
		// Different resolution after already resolved: reject per Batch 4's locked
		// recommendation (PREFLIGHT_AUDIT.md / task spec) rather than silently overwriting a
		// prior, possibly-acted-upon decision.
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeConflictAlreadyResolved, "conflict already resolved with a different resolution", nil)
	}

	updated, err := s.repo.ResolveWorkflowOverrideConflict(ctx, req.Subject.CompanyID, typeID, conflictID, resolution, req.ResolutionValue, req.Subject.UserID, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	return &ResolveWorkflowOverrideConflictResponse{Data: *updated}, nil
}

// roleRegistryForConflictDetection is the single, process-wide static instance Rules 5/7 use —
// zero DB, zero per-request construction cost. See PREFLIGHT_AUDIT.md §5 for why this specific
// source was chosen (already used by the global workflow Publish gate, ValidateManifestRoles).
var roleRegistryForConflictDetection = wfcapp.DefaultRoleRegistry()
