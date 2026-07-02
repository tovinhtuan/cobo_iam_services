package app

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
	"github.com/cobo/cobo_iam_services/internal/companyaccess/configversion"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// EffectiveAccessCache invalidates cached effective-access projections (ADR-025).
type EffectiveAccessCache interface {
	InvalidateMemberships(ctx context.Context, companyID string, membershipIDs []string)
}

func (s *adminService) authorizeConfigVersioning(ctx context.Context, sub AdminSubject) error {
	return s.authorizeConfigurationHealth(ctx, sub)
}

func (s *adminService) captureNotificationRuleVersion(ctx context.Context, sub AdminSubject, ruleID, source, reason string) error {
	if s.repo == nil {
		return nil
	}
	raw, err := s.repo.BuildNotificationRuleSnapshotJSON(ctx, sub.CompanyID, ruleID)
	if err != nil {
		return err
	}
	row, err := s.repo.InsertNotificationRuleVersion(ctx, InsertNotificationRuleVersionInput{
		ID:           s.idg.NewUUID(),
		CompanyID:    sub.CompanyID,
		RuleID:       ruleID,
		SnapshotJSON: raw,
		CreatedBy:    sub.MembershipID,
		Reason:       reason,
		Source:       source,
	})
	if err != nil {
		return err
	}
	s.appendVersionAudit(ctx, sub, "admin.version.notification.snapshot_created", configversion.AggregateNotificationRule, ruleID, map[string]any{
		"version_no":     row.VersionNo,
		"aggregate_type": configversion.AggregateNotificationRule,
		"source":         source,
		"reason":         strings.TrimSpace(reason),
	})
	return nil
}

func (s *adminService) captureRBACMatrixVersion(ctx context.Context, sub AdminSubject, source, reason string) error {
	raw, err := s.repo.BuildRBACMatrixSnapshotJSON(ctx, sub.CompanyID)
	if err != nil {
		return err
	}
	raw, err = filterEnterpriseRBACSnapshotJSON(ctx, s.repo.ListPermissions, raw)
	if err != nil {
		return err
	}
	row, err := s.repo.InsertRBACMatrixSnapshot(ctx, InsertRBACMatrixSnapshotInput{
		ID:           s.idg.NewUUID(),
		CompanyID:    sub.CompanyID,
		SnapshotJSON: raw,
		CreatedBy:    sub.MembershipID,
		Reason:       reason,
		Source:       source,
	})
	if err != nil {
		return err
	}
	s.appendVersionAudit(ctx, sub, "admin.version.rbac.snapshot_created", configversion.AggregateRBACMatrix, sub.CompanyID, map[string]any{
		"version_no":     row.VersionNo,
		"aggregate_type": configversion.AggregateRBACMatrix,
		"source":         source,
		"reason":         strings.TrimSpace(reason),
	})
	return nil
}

func (s *adminService) appendVersionAudit(ctx context.Context, sub AdminSubject, action, resourceType, resourceID string, meta map[string]any) {
	if s.auditRepo == nil {
		return
	}
	_ = s.auditRepo.Append(ctx, auditapp.Entry{
		ActorUserID:       sub.UserID,
		ActorMembershipID: sub.MembershipID,
		CompanyID:         sub.CompanyID,
		Action:            action,
		ResourceType:      resourceType,
		ResourceID:        resourceID,
		Decision:          "allow",
		Metadata:          meta,
	})
}

func (s *adminService) invalidateEffectiveAccessForCompany(ctx context.Context, companyID string) {
	if s.effectiveAccessCache == nil {
		return
	}
	members, err := s.repo.ListMembershipsByCompany(ctx, companyID)
	if err != nil {
		return
	}
	ids := make([]string, 0, len(members))
	for _, m := range members {
		ids = append(ids, m.MembershipID)
	}
	s.effectiveAccessCache.InvalidateMemberships(ctx, companyID, ids)
}

func (s *adminService) ListNotificationRuleVersions(ctx context.Context, req ListNotificationRuleVersionsRequest) (*ConfigVersionListView, error) {
	if err := s.authorizeConfigVersioning(ctx, req.Subject); err != nil {
		return nil, err
	}
	items, err := s.repo.ListNotificationRuleVersions(ctx, req.Subject.CompanyID, req.RuleID, req.Limit)
	if err != nil {
		return nil, err
	}
	return &ConfigVersionListView{Items: items, Meta: map[string]any{"limit": req.Limit}}, nil
}

func (s *adminService) GetNotificationRuleVersion(ctx context.Context, req GetNotificationRuleVersionRequest) (*ConfigVersionDetail, error) {
	if err := s.authorizeConfigVersioning(ctx, req.Subject); err != nil {
		return nil, err
	}
	if req.VersionNo <= 0 {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid version_no", nil)
	}
	detail, err := s.repo.GetNotificationRuleVersion(ctx, req.Subject.CompanyID, req.RuleID, req.VersionNo)
	if err != nil {
		return nil, err
	}
	var snap map[string]any
	_ = json.Unmarshal(detail.SnapshotJSON, &snap)
	return detail, nil
}

func (s *adminService) CompareNotificationRuleVersions(ctx context.Context, req CompareNotificationRuleVersionsRequest) (*CompareVersionsView, error) {
	if err := s.authorizeConfigVersioning(ctx, req.Subject); err != nil {
		return nil, err
	}
	if req.FromVersionNo <= 0 || req.ToVersionNo <= 0 {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid version_no", nil)
	}
	from, err := s.repo.GetNotificationRuleVersion(ctx, req.Subject.CompanyID, req.RuleID, req.FromVersionNo)
	if err != nil {
		return nil, err
	}
	to, err := s.repo.GetNotificationRuleVersion(ctx, req.Subject.CompanyID, req.RuleID, req.ToVersionNo)
	if err != nil {
		return nil, err
	}
	sum, err := configversion.CompareJSON(from.SnapshotJSON, to.SnapshotJSON, req.FromVersionNo, req.ToVersionNo)
	if err != nil {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "compare failed", nil)
	}
	return &CompareVersionsView{
		FromVersionNo: sum.FromVersionNo,
		ToVersionNo:   sum.ToVersionNo,
		Equal:         sum.Equal,
		ChangedKeys:   sum.ChangedKeys,
		Summary:       sum.Details,
	}, nil
}

func (s *adminService) RollbackNotificationRuleVersion(ctx context.Context, req RollbackNotificationRuleVersionRequest) (*ConfigVersionRow, error) {
	if err := s.authorizeConfigVersioning(ctx, req.Subject); err != nil {
		return nil, err
	}
	if req.VersionNo <= 0 {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid version_no", nil)
	}
	target, err := s.repo.GetNotificationRuleVersion(ctx, req.Subject.CompanyID, req.RuleID, req.VersionNo)
	if err != nil {
		return nil, err
	}
	if err := s.repo.RestoreNotificationRuleFromSnapshot(ctx, req.Subject.CompanyID, target.SnapshotJSON); err != nil {
		return nil, err
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = "rollback"
	}
	if err := s.captureNotificationRuleVersion(ctx, req.Subject, req.RuleID, configversion.SourceRollback, reason); err != nil {
		return nil, err
	}
	s.appendVersionAudit(ctx, req.Subject, "admin.version.notification.rollback", configversion.AggregateNotificationRule, req.RuleID, map[string]any{
		"target_version_no": req.VersionNo,
		"aggregate_type":    configversion.AggregateNotificationRule,
		"source":            configversion.SourceRollback,
		"reason":            reason,
	})
	items, err := s.repo.ListNotificationRuleVersions(ctx, req.Subject.CompanyID, req.RuleID, 1)
	if err != nil || len(items) == 0 {
		return &ConfigVersionRow{AggregateType: configversion.AggregateNotificationRule, AggregateID: req.RuleID}, nil
	}
	return &items[0], nil
}

func (s *adminService) ListRBACMatrixVersions(ctx context.Context, req ListRBACMatrixVersionsRequest) (*ConfigVersionListView, error) {
	if err := s.authorizeConfigVersioning(ctx, req.Subject); err != nil {
		return nil, err
	}
	items, err := s.repo.ListRBACMatrixVersions(ctx, req.Subject.CompanyID, req.Limit)
	if err != nil {
		return nil, err
	}
	return &ConfigVersionListView{Items: items, Meta: map[string]any{"limit": req.Limit}}, nil
}

func (s *adminService) GetRBACMatrixVersion(ctx context.Context, req GetRBACMatrixVersionRequest) (*ConfigVersionDetail, error) {
	if err := s.authorizeConfigVersioning(ctx, req.Subject); err != nil {
		return nil, err
	}
	if req.VersionNo <= 0 {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid version_no", nil)
	}
	return s.repo.GetRBACMatrixVersion(ctx, req.Subject.CompanyID, req.VersionNo)
}

func (s *adminService) CompareRBACMatrixVersions(ctx context.Context, req CompareRBACMatrixVersionsRequest) (*CompareVersionsView, error) {
	if err := s.authorizeConfigVersioning(ctx, req.Subject); err != nil {
		return nil, err
	}
	if req.FromVersionNo <= 0 || req.ToVersionNo <= 0 {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid version_no", nil)
	}
	from, err := s.repo.GetRBACMatrixVersion(ctx, req.Subject.CompanyID, req.FromVersionNo)
	if err != nil {
		return nil, err
	}
	to, err := s.repo.GetRBACMatrixVersion(ctx, req.Subject.CompanyID, req.ToVersionNo)
	if err != nil {
		return nil, err
	}
	sum, err := configversion.CompareJSON(from.SnapshotJSON, to.SnapshotJSON, req.FromVersionNo, req.ToVersionNo)
	if err != nil {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "compare failed", nil)
	}
	return &CompareVersionsView{
		FromVersionNo: sum.FromVersionNo,
		ToVersionNo:   sum.ToVersionNo,
		Equal:         sum.Equal,
		ChangedKeys:   sum.ChangedKeys,
		Summary:       sum.Details,
	}, nil
}

func (s *adminService) RollbackRBACMatrixVersion(ctx context.Context, req RollbackRBACMatrixVersionRequest) (*ConfigVersionRow, error) {
	if err := s.authorizeConfigVersioning(ctx, req.Subject); err != nil {
		return nil, err
	}
	if req.VersionNo <= 0 {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid version_no", nil)
	}
	target, err := s.repo.GetRBACMatrixVersion(ctx, req.Subject.CompanyID, req.VersionNo)
	if err != nil {
		return nil, err
	}
	sanitized, err := filterEnterpriseRBACSnapshotJSON(ctx, s.repo.ListPermissions, target.SnapshotJSON)
	if err != nil {
		return nil, err
	}
	if err := s.repo.RestoreRBACMatrixFromSnapshot(ctx, req.Subject.CompanyID, req.Subject.UserID, sanitized); err != nil {
		return nil, err
	}
	s.invalidateEffectiveAccessForCompany(ctx, req.Subject.CompanyID)
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = "rollback"
	}
	if err := s.captureRBACMatrixVersion(ctx, req.Subject, configversion.SourceRollback, reason); err != nil {
		return nil, err
	}
	s.appendVersionAudit(ctx, req.Subject, "admin.version.rbac.rollback", configversion.AggregateRBACMatrix, req.Subject.CompanyID, map[string]any{
		"target_version_no": req.VersionNo,
		"aggregate_type":    configversion.AggregateRBACMatrix,
		"source":            configversion.SourceRollback,
		"reason":            reason,
	})
	items, err := s.repo.ListRBACMatrixVersions(ctx, req.Subject.CompanyID, 1)
	if err != nil || len(items) == 0 {
		return &ConfigVersionRow{AggregateType: configversion.AggregateRBACMatrix}, nil
	}
	return &items[0], nil
}
