package app

import (
	"context"
	"net/http"
	"strings"
	"time"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func (s *service) prepareConfirmAttempt(ctx context.Context, req ConfirmTemplateImportRequest, tokenHash, actualHash string) (*TemplateImportAttempt, error) {
	if !ImportHistoryEnabled() {
		return nil, nil
	}
	if strings.TrimSpace(req.ImportAttemptID) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeImportAttemptRequired, "import_attempt_id is required", nil)
	}
	store, ok := importAttemptStore(s.repo)
	if !ok {
		return nil, historyUnavailable()
	}
	row, err := store.GetImportAttemptByID(ctx, strings.TrimSpace(req.ImportAttemptID))
	if err != nil {
		return nil, err
	}
	if row == nil || row.CreatedAt.Before(templateImportRetentionCutoff(time.Now())) {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeImportAttemptNotFound, "import attempt not found", nil)
	}
	if row.CompanyID != req.Subject.CompanyID || row.ActorUserID != req.Subject.UserID {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodeImportAttemptForbidden, "import attempt is not owned by this actor", nil)
	}
	switch row.Status {
	case ImportAttemptStatusConfirmed, ImportAttemptStatusConfirmFailed:
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeImportAttemptAlreadyConfirmed, "import attempt already confirmed", nil)
	case ImportAttemptStatusConfirming:
		if confirmLeaseActive(row.LeaseExpiresAt, importConfirmNow()) {
			return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeImportAttemptAlreadyConfirmed, "import attempt confirm is in progress", nil)
		}
	case ImportAttemptStatusValidated, ImportAttemptStatusMappingRequired:
	default:
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeImportAttemptAlreadyConfirmed, "import attempt cannot be confirmed", nil)
	}
	if row.ValidationTokenSHA256 == "" || !hashesEqual(row.ValidationTokenSHA256, importSHA256Hex(req.ValidationToken)) {
		return nil, perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeImportAttemptTokenMismatch, "validation token does not match this import attempt", nil)
	}
	if row.CanonicalPayloadSHA256 == "" || !hashesEqual(row.CanonicalPayloadSHA256, actualHash) || !hashesEqual(row.CanonicalPayloadSHA256, tokenHash) {
		return nil, perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeImportAttemptPayloadMismatch, "normalized template does not match this import attempt", nil)
	}
	if err := s.ensureAttemptMappings(ctx, row, req.DepartmentMappings); err != nil {
		return nil, err
	}
	return row, nil
}

func (s *service) ensureAttemptMappings(ctx context.Context, row *TemplateImportAttempt, mappings map[string]string) error {
	if row.RequiredMappingCount == 0 {
		return nil
	}
	depts, err := s.repo.ListTemplateDepartments(ctx)
	if err != nil {
		return err
	}
	live := map[string]struct{}{}
	for _, d := range depts {
		code := strings.ToLower(strings.TrimSpace(d.DepartmentCode))
		if code != "" {
			live[code] = struct{}{}
		}
	}
	resolved := 0
	seen := map[string]struct{}{}
	for _, item := range row.MappingSummary {
		source := strings.TrimSpace(item.SourceID)
		if source == "" {
			continue
		}
		seen[source] = struct{}{}
		target := strings.TrimSpace(mappings[source])
		if target == "" {
			target = strings.TrimSpace(item.TargetID)
		}
		if target == "" {
			return &perr.HTTPError{HTTPStatus: http.StatusBadRequest, Code: perr.CodeImportAttemptMappingIncomplete, Message: "required department mapping is incomplete", Details: map[string]any{"source_id": source}}
		}
		if _, ok := live[strings.ToLower(target)]; !ok {
			return &perr.HTTPError{HTTPStatus: http.StatusConflict, Code: perr.CodeImportAttemptStaleReference, Message: "mapping target department is not in the active catalog", Details: map[string]any{"target_department_code": target}}
		}
		resolved++
	}
	for source := range mappings {
		if _, ok := seen[strings.TrimSpace(source)]; !ok {
			return &perr.HTTPError{HTTPStatus: http.StatusBadRequest, Code: perr.CodeImportAttemptMappingIncomplete, Message: "mapping source is not part of this import attempt", Details: map[string]any{"source_id": source}}
		}
	}
	if resolved < row.RequiredMappingCount {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeImportAttemptMappingIncomplete, "resolved mappings are fewer than required mappings", nil)
	}
	return nil
}

func (s *service) claimConfirmAttempt(ctx context.Context, row *TemplateImportAttempt, targetTypeID string) error {
	if row == nil {
		return nil
	}
	store, ok := importAttemptStore(s.repo)
	if !ok {
		return historyUnavailable()
	}
	version, err := store.ClaimImportAttempt(ctx, row.ID, row.CompanyID, row.ActorUserID, strings.TrimSpace(targetTypeID), row.RowVersion, templateImportRetentionCutoff(importConfirmNow()), importConfirmNow())
	if err != nil {
		return err
	}
	if version == 0 {
		return perr.NewHTTPError(http.StatusConflict, perr.CodeImportAttemptAlreadyConfirmed, "import attempt already confirmed", nil)
	}
	row.RowVersion = version
	return nil
}

func (s *service) finishConfirmAttempt(ctx context.Context, row *TemplateImportAttempt, createdTypeID, targetTypeID string) error {
	if row == nil {
		return nil
	}
	store, ok := importAttemptStore(s.repo)
	if !ok {
		return historyUnavailable()
	}
	return store.MarkImportAttemptConfirmed(ctx, row.ID, row.RowVersion, createdTypeID, targetTypeID, time.Now().UTC())
}

func (s *service) failConfirmAttempt(ctx context.Context, row *TemplateImportAttempt, code perr.Code, targetTypeID string) error {
	if row == nil || row.RowVersion < 2 {
		return nil
	}
	store, ok := importAttemptStore(s.repo)
	if !ok {
		return nil
	}
	return store.MarkImportAttemptFailed(ctx, row.ID, row.RowVersion, string(code), targetTypeID, importConfirmNow().UTC())
}

func confirmLeaseActive(expires, now time.Time) bool {
	return ConfirmLeaseActive(expires, now)
}

// ConfirmLeaseActive reports whether a confirm claim still blocks other requests.
// A zero expiry is not an active lease, so an older CONFIRMING row can be recovered.
func ConfirmLeaseActive(expires, now time.Time) bool {
	if expires.IsZero() {
		return false
	}
	return !now.After(expires)
}

// recoverExpiredConfirm finishes a CONFIRMING row whose lease has ended.
// An existing draft is linked and no second draft is created. A missing target id fails closed.
func (s *service) recoverExpiredConfirm(ctx context.Context, req ConfirmTemplateImportRequest, row *TemplateImportAttempt) (*ConfirmTemplateImportResponse, bool, error) {
	if row == nil || row.Status != ImportAttemptStatusConfirming {
		return nil, false, nil
	}
	target := strings.TrimSpace(row.TargetTypeID)
	if target == "" || target != strings.TrimSpace(req.TargetTypeID) {
		return nil, false, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "import attempt recovery target does not match", nil)
	}
	exists, err := s.repo.TypeExists(ctx, target)
	if err != nil {
		return nil, false, err
	}
	if !exists {
		return nil, false, nil
	}
	store, ok := importAttemptStore(s.repo)
	if !ok {
		return nil, false, historyUnavailable()
	}
	if err := store.ReconcileExpiredImportAttempt(ctx, row.ID, row.RowVersion, target, importConfirmNow().UTC()); err != nil {
		return nil, false, err
	}
	return &ConfirmTemplateImportResponse{
		TypeID:            target,
		VersionNo:         1,
		IsActive:          false,
		IsReleased:        false,
		PortalState:       "not_active",
		RootStatus:        "active",
		Name:              req.TargetName,
		CreatedAt:         importConfirmNow().UTC(),
		HistoryReconciled: true,
	}, true, nil
}
