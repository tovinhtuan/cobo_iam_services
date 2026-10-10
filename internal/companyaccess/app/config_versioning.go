package app

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
	"github.com/cobo/cobo_iam_services/internal/companyaccess/configversion"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// EffectiveAccessCache invalidates cached effective-access projections (ADR-025).
type EffectiveAccessCache interface {
	// InvalidateCompany drops the cached effective access of every membership of the company.
	InvalidateCompany(ctx context.Context, companyID string) error
}

// effectiveAccessInvalidateTimeout bounds the cache call made after a change has committed.
const effectiveAccessInvalidateTimeout = 2 * time.Second

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

// invalidateEffectiveAccessOnSuccess is deferred by mutations that change what a membership
// may do or see (roles, permissions, status, departments, titles, teams). It drops the
// company's cached effective access when the mutation succeeded (H17).
func (s *adminService) invalidateEffectiveAccessOnSuccess(ctx context.Context, companyID string, err *error) {
	if *err == nil {
		s.invalidateEffectiveAccessForCompany(ctx, companyID)
	}
}

// invalidateEffectiveAccessIfWritten is the variant for mutations made of several repository
// writes without a shared transaction: once *wrote is set, a later failure still leaves
// committed changes behind, so the cache is dropped whatever the outcome.
func (s *adminService) invalidateEffectiveAccessIfWritten(ctx context.Context, companyID string, err *error, wrote *bool) {
	if *err == nil || *wrote {
		s.invalidateEffectiveAccessForCompany(ctx, companyID)
	}
}

func (s *adminService) invalidateEffectiveAccessForCompany(ctx context.Context, companyID string) {
	if s.effectiveAccessCache == nil || companyID == "" {
		return
	}
	// The change has already committed: finish even if the client has gone away.
	ictx, cancel := context.WithTimeout(context.WithoutCancel(ctx), effectiveAccessInvalidateTimeout)
	defer cancel()
	if err := s.effectiveAccessCache.InvalidateCompany(ictx, companyID); err != nil {
		slog.WarnContext(ctx, "effective access cache invalidation failed; stale access lasts until the cache TTL",
			slog.String("company_id", companyID), slog.String("error", err.Error()))
	}
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
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = "rollback"
	}
	var snap configversion.NotificationRuleSnapshot
	if err := json.Unmarshal(target.SnapshotJSON, &snap); err == nil && snap.RuleCode == AlertChannelPrefsRuleCode {
		return nil, s.submitNotificationPrefsRollbackApproval(ctx, req.Subject, req.RuleID, target.SnapshotJSON, snap, reason)
	}
	if err := s.repo.RestoreNotificationRuleFromSnapshot(ctx, req.Subject.CompanyID, target.SnapshotJSON); err != nil {
		return nil, err
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

// submitNotificationPrefsRollbackApproval (ROLE-12): rolling the alert channel preferences
// back changes them like any other edit, so it is checked against the payload rules and the
// company's plan and waits for a second admin, as UpdateNotificationRule does.
func (s *adminService) submitNotificationPrefsRollbackApproval(ctx context.Context, sub AdminSubject, ruleID string, raw []byte, snap configversion.NotificationRuleSnapshot, reason string) error {
	prefs := prefsDocumentFromRulePayload(deepCloneMap(snap.Payload))
	if valid, issues := ValidateAlertChannelPrefsPayload(prefs); !valid {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, strings.Join(issues, "; "), nil)
	}
	if err := s.entitlementChecker().ValidateAlertChannelPrefsMutation(ctx, sub.UserID, prefs); err != nil {
		return err
	}
	baseVer, err := s.currentLiveVersionNo(ctx, sub.CompanyID, configversion.AggregateNotificationRule, ruleID)
	if err != nil {
		return err
	}
	summary, err := s.queueConfigApproval(ctx, sub, InsertPendingAdminChangeInput{
		ID:                   s.idg.NewUUID(),
		CompanyID:            sub.CompanyID,
		ApprovalSubjectType:  configversion.ApprovalSubjectConfigSnapshot,
		AggregateType:        configversion.AggregateNotificationRule,
		AggregateID:          ruleID,
		ChangeType:           configversion.ChangeTypeNotificationPatch,
		ProposedSnapshotJSON: raw,
		BaseLiveVersionNo:    &baseVer,
		RequestedBy:          sub.MembershipID,
		Reason:               reason,
	})
	if err != nil {
		return err
	}
	return s.routeApprovalRouted(sub, summary)
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
	// A stored version is a state to converge to: it never carries the instructions of an approval
	// proposal (explicit revokes, plan digest), so none are honoured even if one were present.
	if sanitized, err = stripApprovalInstructions(sanitized); err != nil {
		return nil, err
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = "rollback"
	}
	// A rollback that adds or removes a critical permission needs a second person, like
	// RemoveRolePermission does for the same permissions.
	impact, err := s.rbacRestoreImpact(ctx, req.Subject.CompanyID, sanitized)
	if err != nil {
		return nil, err
	}
	// The role-permission and direct-permission routes need rbac.manage, and what a rollback
	// changes is decided under the restore's locks, so the rollback itself always needs it
	// (system.settings alone is enough only to read versions).
	if err := s.requireRbacManage(ctx, req.Subject); err != nil {
		return nil, err
	}
	if impact.Critical {
		return nil, s.routeRBACRollbackToApproval(ctx, req.Subject, req.VersionNo, reason, sanitized)
	}
	if err := s.repo.RestoreRBACMatrixFromSnapshot(ctx, req.Subject.CompanyID, req.Subject.UserID, sanitized, RBACRestoreOptions{}); err != nil {
		// The state changed after the pre-check and the restore now touches a critical permission.
		if errors.Is(err, ErrRBACRestoreNeedsApproval) {
			return nil, s.routeRBACRollbackToApproval(ctx, req.Subject, req.VersionNo, reason, sanitized)
		}
		return nil, err
	}
	s.invalidateEffectiveAccessForCompany(ctx, req.Subject.CompanyID)
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

// stripApprovalInstructions removes the fields only an approval proposal carries.
func stripApprovalInstructions(raw []byte) ([]byte, error) {
	var snap configversion.RBACMatrixSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return nil, err
	}
	snap.Explicit = false
	snap.RoleRevokes = nil
	snap.DirectRevokes = nil
	snap.PlanDigest = ""
	return json.Marshal(snap)
}
