package app

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// Sprint 3 / Batch 2 — Workflow Override Staleness Detection.
//
// This file owns ONLY: the staleness comparator (pure function, no DB) and the
// status/rebase-check request/response contracts. It does not call, and is not called by,
// GetEffectiveWorkflow — see docs/ai-cache/workflow-override-foundation-batch2/PREFLIGHT_AUDIT.md
// §6 for the explicit, structural non-interference statement.

// StaleStatus values. "unknown" is the conservative default whenever equivalence cannot be
// proven — never guessed into "current" or "stale" (see ComputeStaleStatus rules 2/3/4).
const (
	StaleStatusUnknown = "unknown"
	StaleStatusCurrent = "current"
	StaleStatusStale   = "stale"
)

// Valid BaseSource values (mirrors Batch 1's migration 0103 enum).
const (
	BaseSourceGlobalWorkflow = "global_workflow"
	BaseSourceGlobalTemplate = "global_template"
	BaseSourceUnknown        = "unknown"
)

// OverrideStalenessMetadata is what the comparator reads about a single override row — a direct,
// narrow projection of the columns Batch 1 added, never the full override header DTO.
type OverrideStalenessMetadata struct {
	HasOverride   bool // true iff a row exists AND active_version_no > 0 (matches the resolver's
	// own "is this override actually winning" definition — see PREFLIGHT_AUDIT.md §1)
	BaseSource    string
	BaseVersionNo *int
}

// OverrideStalenessRow is the full repository-layer projection of a single override row's Batch 1
// columns — a superset of OverrideStalenessMetadata, carrying the extra fields (BaseWorkflowID,
// BaseHash, StaleStatus, LastRebaseCheckAt) the service layer needs to build the HTTP response,
// even though the comparator itself only consumes HasOverride/BaseSource/BaseVersionNo.
type OverrideStalenessRow struct {
	HasOverride       bool
	BaseSource        string
	BaseWorkflowID    *string
	BaseVersionNo     *int
	BaseHash          *string
	StaleStatus       string
	LastRebaseCheckAt *time.Time
}

// ComputeStaleStatus implements the 5 locked comparator rules from the task spec. It is a pure
// function — no DB access, no side effects — so it can be unit-tested exhaustively without any
// repository or HTTP plumbing. anomaly is non-empty only for Rule 3's "base_version_no greater
// than current" case, which the caller is expected to log (never silently ignored, never treated
// as an error that blocks the response — the comparator still returns a safe "unknown" result).
func ComputeStaleStatus(meta OverrideStalenessMetadata, currentGlobalActiveVersionNo *int) (status string, anomaly string) {
	// Rule 1 — no override.
	if !meta.HasOverride {
		return StaleStatusUnknown, ""
	}

	switch meta.BaseSource {
	case "", BaseSourceUnknown:
		// Rule 2 — unknown base. Do not guess.
		return StaleStatusUnknown, ""

	case BaseSourceGlobalWorkflow:
		// Rule 3 — global workflow base, version known.
		if meta.BaseVersionNo == nil {
			return StaleStatusUnknown, "base_source=global_workflow but base_version_no is nil"
		}
		if currentGlobalActiveVersionNo == nil {
			// The type no longer has ANY active global workflow version (e.g. archived). Cannot
			// prove equivalence either way — conservative unknown, not a guess.
			return StaleStatusUnknown, ""
		}
		switch {
		case *meta.BaseVersionNo == *currentGlobalActiveVersionNo:
			return StaleStatusCurrent, ""
		case *meta.BaseVersionNo < *currentGlobalActiveVersionNo:
			return StaleStatusStale, ""
		default: // *meta.BaseVersionNo > *currentGlobalActiveVersionNo
			return StaleStatusUnknown, "base_version_no > current_global_active_version_no (anomaly)"
		}

	case BaseSourceGlobalTemplate:
		// Rule 4 — legacy base. Conservative: only "stale" when we can PROVE a global workflow
		// now exists where there wasn't one before (a real, detectable change). Otherwise
		// "unknown" — we cannot prove the legacy content is still equivalent (no historical
		// snapshot of legacy content is retained anywhere in this system).
		if currentGlobalActiveVersionNo != nil {
			return StaleStatusStale, ""
		}
		return StaleStatusUnknown, ""

	default:
		return StaleStatusUnknown, "unrecognized base_source: " + meta.BaseSource
	}
}

// LogStalenessAnomaly is the single place an anomaly (Rule 3's impossible-looking
// base_version_no > current case) gets logged — called by the service layer, not the pure
// comparator itself, so the comparator stays side-effect-free and trivially testable.
func LogStalenessAnomaly(ctx context.Context, companyID, typeID, anomaly string) {
	if anomaly == "" {
		return
	}
	slog.WarnContext(ctx, "workflow override staleness anomaly detected",
		slog.String("company_id", companyID),
		slog.String("type_id", typeID),
		slog.String("anomaly", anomaly))
}

// ── Contracts ──────────────────────────────────────────────────────────────────────────────────

type GetWorkflowOverrideStatusRequest struct {
	Subject Subject
	TypeID  string
}

type WorkflowOverrideStatusDTO struct {
	HasOverride                  bool       `json:"has_override"`
	StaleStatus                  string     `json:"stale_status"`
	BaseSource                   *string    `json:"base_source"`
	BaseWorkflowID                *string    `json:"base_workflow_id"`
	BaseVersionNo                 *int       `json:"base_version_no"`
	CurrentGlobalActiveVersionNo  *int       `json:"current_global_active_version_no"`
	LastRebaseCheckAt              *time.Time `json:"last_rebase_check_at"`
}

type GetWorkflowOverrideStatusResponse struct {
	Data WorkflowOverrideStatusDTO `json:"data"`
}

type RebaseCheckWorkflowOverrideRequest struct {
	Subject Subject
	TypeID  string
}

type RebaseCheckWorkflowOverrideResponse struct {
	Data WorkflowOverrideStatusDTO `json:"data"`
}

// validateStalenessTypeID is the shared guard both new service methods use.
func validateStalenessTypeID(typeID string) (string, error) {
	typeID = strings.TrimSpace(typeID)
	if typeID == "" {
		return "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "type_id is required", nil)
	}
	return typeID, nil
}

// resolveWorkflowOverrideStatus is the shared core both GetWorkflowOverrideStatus and
// RebaseCheckWorkflowOverride use to build a WorkflowOverrideStatusDTO — read-only, never writes
// anything. RebaseCheckWorkflowOverride additionally persists the result afterward (see below).
func resolveWorkflowOverrideStatus(ctx context.Context, repo Repository, companyID, typeID string) (*WorkflowOverrideStatusDTO, error) {
	currentGlobalActiveVersionNo, err := repo.GetCurrentGlobalActiveVersionNo(ctx, typeID)
	if err != nil {
		return nil, err
	}

	row, exists, err := repo.GetOverrideStalenessMetadata(ctx, companyID, typeID)
	if err != nil {
		return nil, err
	}
	if !exists || !row.HasOverride {
		// Rule 1 — no override (covers both "no row at all" and "row exists but is a draft-only,
		// never-active override" — neither is a "has_override=true" case for this endpoint, per
		// PREFLIGHT_AUDIT.md §1's definition matching the resolver's own).
		return &WorkflowOverrideStatusDTO{
			HasOverride:                  false,
			StaleStatus:                  StaleStatusUnknown,
			CurrentGlobalActiveVersionNo: currentGlobalActiveVersionNo,
		}, nil
	}

	status, anomaly := ComputeStaleStatus(OverrideStalenessMetadata{
		HasOverride:   row.HasOverride,
		BaseSource:    row.BaseSource,
		BaseVersionNo: row.BaseVersionNo,
	}, currentGlobalActiveVersionNo)
	LogStalenessAnomaly(ctx, companyID, typeID, anomaly)

	baseSource := row.BaseSource
	return &WorkflowOverrideStatusDTO{
		HasOverride:                  true,
		StaleStatus:                  status,
		BaseSource:                   &baseSource,
		BaseWorkflowID:               row.BaseWorkflowID,
		BaseVersionNo:                row.BaseVersionNo,
		CurrentGlobalActiveVersionNo: currentGlobalActiveVersionNo,
		LastRebaseCheckAt:            row.LastRebaseCheckAt,
	}, nil
}

func (s *service) GetWorkflowOverrideStatus(ctx context.Context, req GetWorkflowOverrideStatusRequest) (*GetWorkflowOverrideStatusResponse, error) {
	typeID, err := validateStalenessTypeID(req.TypeID)
	if err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, req.Subject, "template.workflow.override.read", authapp.ResourceRef{
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
	dto, err := resolveWorkflowOverrideStatus(ctx, s.repo, req.Subject.CompanyID, typeID)
	if err != nil {
		return nil, err
	}
	return &GetWorkflowOverrideStatusResponse{Data: *dto}, nil
}

func (s *service) RebaseCheckWorkflowOverride(ctx context.Context, req RebaseCheckWorkflowOverrideRequest) (*RebaseCheckWorkflowOverrideResponse, error) {
	typeID, err := validateStalenessTypeID(req.TypeID)
	if err != nil {
		return nil, err
	}
	// Read-tier permission per API_CONTRACT_PROPOSAL.md: rebase-check only ever refreshes
	// stale_status/last_rebase_check_at (a metadata read-and-record action), it never writes
	// workflow content — so it is gated the same as the read-only status endpoint, not the
	// write-tier permission the actual override-content endpoints require.
	if err := s.authorize(ctx, req.Subject, "template.workflow.override.read", authapp.ResourceRef{
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
	dto, err := resolveWorkflowOverrideStatus(ctx, s.repo, req.Subject.CompanyID, typeID)
	if err != nil {
		return nil, err
	}
	if dto.HasOverride {
		// Rule 1 — no override: "No DB write required." Only write when an override actually
		// exists; never write stale_status/last_rebase_check_at for a (company, type) pair with
		// no override row to begin with.
		if err := s.repo.UpdateOverrideStaleness(ctx, req.Subject.CompanyID, typeID, dto.StaleStatus, time.Now().UTC()); err != nil {
			return nil, err
		}
		now := time.Now().UTC()
		dto.LastRebaseCheckAt = &now
	}
	return &RebaseCheckWorkflowOverrideResponse{Data: *dto}, nil
}

func typeNotFoundErr(typeID string) error {
	return perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "disclosure type not found: "+typeID, nil)
}
