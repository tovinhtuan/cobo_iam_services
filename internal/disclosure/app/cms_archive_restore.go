package app

import (
	"context"
	"net/http"
	"strings"
	"time"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// CmsArchiveTemplate soft-archives a global system template.
// In-flight disclosure records stay open obligations (shown with "Mẫu đã ngừng áp dụng", still
// reminded; PO decision Q1-B); the template stops appearing in Portal catalog lists, and its
// periodic cycles without a record are SUSPENDED in the same transaction.
func (s *service) CmsArchiveTemplate(ctx context.Context, req CmsArchiveTemplateRequest) (*CmsArchiveTemplateResponse, error) {
	if err := s.requireCMSTemplateArchive(ctx, req.Subject); err != nil {
		return nil, err
	}
	req.TypeID = strings.TrimSpace(req.TypeID)
	req.Reason = strings.TrimSpace(req.Reason)
	if req.TypeID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "type_id is required", nil)
	}
	result, err := s.repo.ArchiveGlobalTemplate(ctx, ArchiveGlobalTemplateParams{
		TypeID:    req.TypeID,
		UpdatedBy: req.Subject.UserID,
		Reason:    req.Reason,
	})
	if err != nil {
		return nil, err
	}
	return &CmsArchiveTemplateResponse{
		TypeID:                result.TypeID,
		Status:                result.AfterStatus,
		AlreadyArchived:       result.AlreadyArchived,
		ArchivedFromVersionNo: result.ArchivedFromVersionNo,
		ActiveVersionNo:       result.AfterActiveVersionNo,
		BeforeStatus:          result.BeforeStatus,
		BeforeActiveVersionNo: result.BeforeActiveVersionNo,
		SuspendedCycles:       result.SuspendedCycles,
	}, nil
}

// CmsRestoreTemplate restores a soft-archived global template to draft or active using archived_from_version_no.
func (s *service) CmsRestoreTemplate(ctx context.Context, req CmsRestoreTemplateRequest) (*CmsRestoreTemplateResponse, error) {
	if err := s.requireCMSTemplateArchive(ctx, req.Subject); err != nil {
		return nil, err
	}
	req.TypeID = strings.TrimSpace(req.TypeID)
	req.Reason = strings.TrimSpace(req.Reason)
	if req.TypeID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "type_id is required", nil)
	}

	life, err := s.lookupTypeLifecycle(ctx, req.TypeID)
	if err != nil {
		return nil, err
	}
	if life == nil {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "global template not found", nil)
	}
	if strings.TrimSpace(life.CompanyID) != "" {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "global template not found", nil)
	}
	if !strings.EqualFold(strings.TrimSpace(life.Status), "archived") {
		return nil, &perr.HTTPError{
			HTTPStatus: http.StatusConflict,
			Code:       perr.CodeStateConflict,
			Message:    "template is not archived; restore is not applicable",
			Details:    map[string]any{"type_id": req.TypeID, "status": life.Status},
		}
	}

	backfill, err := normalizeBackfillCycleIDs(req.BackfillCycleIDs)
	if err != nil {
		return nil, err
	}
	params := RestoreGlobalTemplateParams{
		TypeID: req.TypeID, UpdatedBy: req.Subject.UserID, BackfillCycleIDs: backfill,
		TodayHCM: time.Now().In(asiaHoChiMinh()).Format("2006-01-02"),
	}
	if life.ArchivedFromVersionNo == nil {
		// Draft restore — no activate validation.
		params.ExpectedFromVersionNo = nil
		params.RestoreActiveVersionNo = 0
	} else {
		versionNo := *life.ArchivedFromVersionNo
		if err := s.validateVersionForRestore(ctx, req.Subject, req.TypeID, versionNo); err != nil {
			return nil, err
		}
		params.ExpectedFromVersionNo = &versionNo
		params.RestoreActiveVersionNo = versionNo
	}

	result, err := s.repo.RestoreGlobalTemplate(ctx, params)
	if err != nil {
		return nil, err
	}
	return &CmsRestoreTemplateResponse{
		TypeID:                result.TypeID,
		Status:                result.AfterStatus,
		ActiveVersionNo:       result.AfterActiveVersionNo,
		RestoredMode:          result.RestoredMode,
		BeforeStatus:          result.BeforeStatus,
		BeforeActiveVersionNo: result.BeforeActiveVersionNo,
		ArchivedFromVersionNo: result.ArchivedFromVersionNo,
		BackfilledCycles:      result.BackfilledCycles,
		SkippedCycles:         result.SkippedCycles,
		ResumedCycles:         result.ResumedCycles,
		BackfillCycleIDs:      backfill,
	}, nil
}

// MaxBackfillCycleIDs bounds one restore request (and the missed cycles the archive impact lists).
const MaxBackfillCycleIDs = 1000

// normalizeBackfillCycleIDs trims, drops blanks and duplicates. The repository checks under the
// template lock that every id is a SUSPENDED cycle of the template.
func normalizeBackfillCycleIDs(ids []string) ([]string, error) {
	if len(ids) > MaxBackfillCycleIDs {
		return nil, BackfillCycleIDsError("too many backfill_cycle_ids")
	}
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}

// BackfillCycleIDsError is the 422 for backfill_cycle_ids that are not suspended cycles of the
// template.
func BackfillCycleIDsError(message string) error {
	he := perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, message, nil)
	he.Details = map[string]any{"field": "backfill_cycle_ids"}
	return he
}

// CmsTemplateArchiveImpact reports, for a global template, the open obligations per company that
// stay open after archiving and the periodic cycles that archiving suspends or a restore can
// backfill. Same permission as archive/restore.
func (s *service) CmsTemplateArchiveImpact(ctx context.Context, req CmsTemplateArchiveImpactRequest) (*CmsTemplateArchiveImpactResponse, error) {
	if err := s.requireCMSTemplateArchive(ctx, req.Subject); err != nil {
		return nil, err
	}
	typeID := strings.TrimSpace(req.TypeID)
	if typeID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "type_id is required", nil)
	}
	todayHCM := time.Now().In(asiaHoChiMinh()).Format("2006-01-02")
	return s.repo.GetTemplateArchiveImpact(ctx, typeID, todayHCM)
}

// validateVersionForRestore reuses ActivateTypeVersion validation rules without mutating state.
func (s *service) validateVersionForRestore(ctx context.Context, sub Subject, typeID string, versionNo int) error {
	versionDetail, err := s.repo.GetTypeVersionDetail(ctx, sub.CompanyID, typeID, versionNo)
	if err != nil {
		return err
	}
	if versionDetail.WorkflowAuthorityMode != WorkflowAuthorityTemplatePinned || versionDetail.WorkflowManifest == nil {
		return &perr.HTTPError{
			Code: "TEMPLATE_WORKFLOW_NOT_PINNED", Message: "template version workflow publication is not pinned",
			HTTPStatus: http.StatusUnprocessableEntity,
			Details:    map[string]any{"type_id": typeID, "version_no": versionNo},
		}
	}
	published := ResolveTemplatePublicationWorkflow(typeID, versionNo, *versionDetail.WorkflowManifest)
	if err := ValidateWorkflowStepsForActivation(published.Workflow); err != nil {
		code := perr.Code("TEMPLATE_WORKFLOW_INVALID")
		if len(published.Workflow) == 0 {
			code = perr.Code("TEMPLATE_NO_WORKFLOW")
		}
		return &perr.HTTPError{
			Code: code, Message: err.Error(), HTTPStatus: http.StatusUnprocessableEntity,
			Details: map[string]any{"type_id": typeID, "version_no": versionNo},
		}
	}
	if descBlockers := CollectWorkflowStepDescriptionActivationBlockers(published.Workflow); len(descBlockers) > 0 {
		return &perr.HTTPError{
			Code:       perr.Code(ActivationBlockerWorkflowStepDescriptionRequired),
			Message:    descBlockers[0].Message,
			HTTPStatus: http.StatusUnprocessableEntity,
			Details:    map[string]any{"type_id": typeID, "version_no": versionNo, "activation_blockers": descBlockers},
		}
	}
	if err := validatePortalDeadlineRule(versionDetail.DeadlineRule, s.loadDeadlineRuleCatalog(ctx)); err != nil {
		return err
	}
	return nil
}
