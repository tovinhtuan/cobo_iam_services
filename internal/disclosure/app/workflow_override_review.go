package app

import (
	"context"
	"net/http"
	"strings"
	"unicode/utf8"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// MR-4 (PO decision Q5-B): maker-checker for company workflow overrides by permission. A holder of
// template.workflow.override.approve activates a version directly, their own included (audited as
// self_approved); a writer saves drafts and submits them; an approver approves or rejects a
// submission. Only the latest version can be submitted, approved or rejected.

const (
	// maxOverrideReviewNoteRunes bounds the submit note and the rejection reason (decision_note).
	maxOverrideReviewNoteRunes = 500
	// maxPendingOverrides bounds the pending-approval list of one company.
	maxPendingOverrides = 100
	// maxWorkflowStageRunes bounds a merge_manual stage label.
	maxWorkflowStageRunes = 255
)

func (s *service) SubmitCompanyWorkflowOverride(ctx context.Context, req SubmitCompanyWorkflowOverrideRequest) (*SubmitCompanyWorkflowOverrideResponse, error) {
	req.TypeID = strings.TrimSpace(req.TypeID)
	req.Note = strings.TrimSpace(req.Note)
	if err := normalizeOverrideReviewVersion(&req.VersionNo, req.BaseEtag, req.TypeID); err != nil {
		return nil, err
	}
	if utf8.RuneCountInString(req.Note) > maxOverrideReviewNoteRunes {
		return nil, overrideReviewFieldError("note", "note must be at most 500 characters")
	}
	if err := s.authorize(ctx, req.Subject, "template.workflow.override.write", authapp.ResourceRef{
		Type: "disclosure_type",
		ID:   req.TypeID,
	}); err != nil {
		return nil, err
	}
	draft, err := s.latestOverrideDraft(ctx, req.Subject.CompanyID, req.TypeID, req.VersionNo)
	if err != nil {
		return nil, err
	}
	if draft.State != OverrideVersionStateDraft {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "only a draft version can be submitted for approval", nil)
	}
	// An approver must receive a version they can activate.
	if err := ValidateCompanyWorkflowOverrideSteps(draft.Workflow); err != nil {
		return nil, err
	}
	if err := ValidateCompanyWorkflowOverrideStepDescriptionsForActivation(draft.Workflow); err != nil {
		return nil, err
	}
	v, err := s.repo.SubmitCompanyWorkflowOverride(ctx, req)
	if err != nil {
		return nil, err
	}
	v.DraftEtag = WorkflowDraftEtagFromVersion(v.VersionNo)
	return &SubmitCompanyWorkflowOverrideResponse{Data: *v}, nil
}

func (s *service) RejectCompanyWorkflowOverride(ctx context.Context, req RejectCompanyWorkflowOverrideRequest) (*RejectCompanyWorkflowOverrideResponse, error) {
	req.TypeID = strings.TrimSpace(req.TypeID)
	req.Reason = strings.TrimSpace(req.Reason)
	if err := normalizeOverrideReviewVersion(&req.VersionNo, req.BaseEtag, req.TypeID); err != nil {
		return nil, err
	}
	if req.Reason == "" || utf8.RuneCountInString(req.Reason) > maxOverrideReviewNoteRunes {
		return nil, overrideReviewFieldError("reason", "reason is required (at most 500 characters)")
	}
	if err := s.authorize(ctx, req.Subject, "template.workflow.override.approve", authapp.ResourceRef{
		Type: "disclosure_type",
		ID:   req.TypeID,
	}); err != nil {
		return nil, err
	}
	draft, err := s.latestOverrideDraft(ctx, req.Subject.CompanyID, req.TypeID, req.VersionNo)
	if err != nil {
		return nil, err
	}
	if draft.State != OverrideVersionStatePendingApproval {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "only a version pending approval can be rejected", nil)
	}
	v, err := s.repo.RejectCompanyWorkflowOverride(ctx, req)
	if err != nil {
		return nil, err
	}
	v.DraftEtag = WorkflowDraftEtagFromVersion(v.VersionNo)
	return &RejectCompanyWorkflowOverrideResponse{Data: *v}, nil
}

func (s *service) ListPendingCompanyWorkflowOverrides(ctx context.Context, req ListPendingCompanyWorkflowOverridesRequest) (*ListPendingCompanyWorkflowOverridesResponse, error) {
	if err := s.authorize(ctx, req.Subject, "template.workflow.override.approve", authapp.ResourceRef{
		Type: "disclosure_type",
	}); err != nil {
		return nil, err
	}
	items, err := s.repo.ListPendingCompanyWorkflowOverrides(ctx, req.Subject.CompanyID, maxPendingOverrides)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []PendingCompanyWorkflowOverrideItem{}
	}
	for i := range items {
		items[i].DraftEtag = WorkflowDraftEtagFromVersion(items[i].VersionNo)
	}
	return &ListPendingCompanyWorkflowOverridesResponse{Items: items, Total: len(items)}, nil
}

// latestOverrideDraft returns the latest open version of the override when it is versionNo: 404
// without one, 409 STALE_ETAG when a newer version exists, 409 STATE_CONFLICT when versionNo is
// already the active version.
func (s *service) latestOverrideDraft(ctx context.Context, companyID, typeID string, versionNo int) (*CompanyWorkflowOverrideVersionDTO, error) {
	view, err := s.repo.GetCompanyWorkflowOverride(ctx, companyID, typeID)
	if err != nil {
		return nil, err
	}
	if view.DraftVersion == nil {
		// No open version: an older version below the active one (e.g. a draft a rebase-apply
		// superseded, WF-08c) is stale; anything else does not exist.
		if view.Override != nil && versionNo < view.Override.ActiveVersionNo {
			return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStaleEtag, "a newer workflow override version exists; reload and retry", nil)
		}
		// The version is already the active one (e.g. approved twice, MR-4 review DB-03).
		if view.Override != nil && versionNo == view.Override.ActiveVersionNo {
			return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "workflow override version is already active", nil)
		}
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "workflow override draft version not found", nil)
	}
	if view.DraftVersion.VersionNo != versionNo {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStaleEtag, "a newer workflow override version exists; reload and retry", nil)
	}
	return view.DraftVersion, nil
}

func normalizeOverrideReviewVersion(versionNo *int, baseEtag, typeID string) error {
	if typeID == "" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "type_id is required", nil)
	}
	if *versionNo <= 0 {
		*versionNo = ResolveWorkflowBaseVersionNo(*versionNo, strings.TrimSpace(baseEtag))
	}
	if *versionNo <= 0 {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "version_no or base_etag is required", nil)
	}
	return nil
}

func overrideReviewFieldError(field, message string) error {
	he := perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, message, nil)
	he.Details = map[string]any{"field": field}
	return he
}

// ValidateMergeManualValue checks the value a merge_manual resolution brings for a conflicting
// field, so a rebase can neither switch a step's reminders off with {} nor silently keep the
// company value (MR-4, Q5-B). Field paths that merge_manual cannot change are rejected:
// documents and groups reference company data the value would have to be checked against, and
// the Portal has no input for them, so they take accept_global or keep_company only.
func ValidateMergeManualValue(fieldPath string, value any) error {
	invalid := func(message string) error {
		he := perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, message, nil)
		he.Details = map[string]any{"field": "resolution_value", "field_path": fieldPath}
		return he
	}
	switch fieldPath {
	case "due_rule":
		s, ok := value.(string)
		if !ok || parseDueRuleProcessingDays(s) <= 0 {
			return invalid("merge_manual due_rule must be T+N with N >= 1")
		}
	case "assignee_role_ids":
		roles, ok := decodeStringSlice(value)
		if !ok || len(roles) == 0 {
			return invalid("merge_manual assignee_role_ids must be a non-empty list")
		}
		for _, r := range roles {
			if strings.TrimSpace(r) == "" {
				return invalid("merge_manual assignee_role_ids must not contain an empty role")
			}
		}
		if raw, isList := value.([]any); isList && len(raw) != len(roles) {
			return invalid("merge_manual assignee_role_ids must only contain role ids")
		}
	case "reminder_config":
		m, ok := value.(map[string]any)
		if !ok {
			return invalid("merge_manual reminder_config must be an object")
		}
		enabled, ok := m["enabled"].(bool)
		if !ok {
			return invalid("merge_manual reminder_config.enabled is required")
		}
		if enabled {
			raw, ok := m["days_before"].([]any)
			if !ok || len(raw) == 0 {
				return invalid("merge_manual reminder_config.days_before must list at least one day")
			}
			for _, item := range raw {
				f, ok := item.(float64)
				if !ok || f != float64(int(f)) || f < 1 || f > 365 {
					return invalid("merge_manual reminder_config.days_before must be whole days between 1 and 365")
				}
			}
		}
	case "stage":
		s, ok := value.(string)
		if !ok || strings.TrimSpace(s) == "" || utf8.RuneCountInString(strings.TrimSpace(s)) > maxWorkflowStageRunes {
			return invalid("merge_manual stage must be a non-empty string of at most 255 characters")
		}
	case "__step_existence__":
		// An unclear step identity takes the step the company wants (the patch engine adds it as
		// given); the whole rebased workflow is validated again before it is written.
		step, ok := decodeStepFromAny(value)
		if !ok || strings.TrimSpace(step.StepID) == "" || strings.TrimSpace(step.Stage) == "" {
			return invalid("merge_manual for a step needs the step (step_id and stage)")
		}
	default: // documents, groups, department_id (Rule 8), unknown fields
		return invalid("merge_manual is not available for this field")
	}
	return nil
}
