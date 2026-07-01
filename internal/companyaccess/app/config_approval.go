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

func (s *adminService) authorizeConfigApprovalRead(ctx context.Context, sub AdminSubject) error {
	return s.authorizeConfigurationHealth(ctx, sub)
}

func (s *adminService) authorizeConfigApprovalDecide(ctx context.Context, sub AdminSubject) error {
	ok, err := s.hasPermission(ctx, sub, "system.settings")
	if err != nil {
		return err
	}
	if !ok {
		return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "access denied", nil)
	}
	return nil
}

func isCriticalPermissionCode(code string) bool {
	_, ok := criticalPermissionCodes[code]
	return ok
}

func requiresApprovalForDirectRemove(code string) bool {
	if isCriticalPermissionCode(code) {
		return true
	}
	risk := PermissionRiskLevel(code)
	return (risk == "high" || risk == "critical") && !IsGrantablePermission(code)
}

func (s *adminService) currentLiveVersionNo(ctx context.Context, companyID, aggregateType, aggregateID string) (int, error) {
	switch aggregateType {
	case configversion.AggregateNotificationRule:
		return s.repo.GetMaxNotificationRuleVersionNo(ctx, companyID, aggregateID)
	case configversion.AggregateRBACMatrix:
		return s.repo.GetMaxRBACMatrixVersionNo(ctx, companyID)
	default:
		return 0, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid aggregate_type", nil)
	}
}

func (s *adminService) checkStaleProposal(ctx context.Context, row *PendingAdminChange) error {
	if row.BaseLiveVersionNo == nil {
		return nil
	}
	current, err := s.currentLiveVersionNo(ctx, row.CompanyID, row.AggregateType, row.AggregateID)
	if err != nil {
		return err
	}
	if current != *row.BaseLiveVersionNo {
		return &perr.HTTPError{
			Code:       perr.CodeStaleProposal,
			Message:    "proposal is stale; live version changed",
			HTTPStatus: http.StatusConflict,
			Details: map[string]any{
				"base_live_version_no": *row.BaseLiveVersionNo,
				"current_version_no":   current,
			},
		}
	}
	return nil
}

func (s *adminService) queueConfigApproval(ctx context.Context, sub AdminSubject, in InsertPendingAdminChangeInput) (*PendingAdminChangeSummary, error) {
	if s.repo == nil {
		return nil, perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeServiceUnavailable, "approval queue unavailable", nil)
	}
	row, err := s.repo.InsertPendingAdminChange(ctx, in)
	if err != nil {
		return nil, err
	}
	s.appendApprovalAudit(ctx, sub, "admin.config.approval.requested", row, map[string]any{
		"approval_id":          row.ID,
		"aggregate_type":       row.AggregateType,
		"aggregate_id":         row.AggregateID,
		"change_type":          row.ChangeType,
		"base_live_version_no": row.BaseLiveVersionNo,
		"requested_by":         row.RequestedBy,
		"status":               row.Status,
		"reason":               strings.TrimSpace(row.Reason),
		"source":               "approval_queue",
	})
	summary := s.safeApprovalSummary(row)
	out := ToPendingSummary(*row, summary)
	return &out, nil
}

func (s *adminService) routeApprovalRouted(sub AdminSubject, summary *PendingAdminChangeSummary) error {
	return &perr.HTTPError{
		Code:       perr.CodeApprovalRouted,
		Message:    "change routed to approval queue",
		HTTPStatus: http.StatusAccepted,
		Details: map[string]any{
			"approval_id": summary.ApprovalID,
			"status":      summary.Status,
		},
	}
}

func (s *adminService) buildProposedNotificationSnapshot(ctx context.Context, companyID, ruleID string, patch map[string]any, status *string) ([]byte, error) {
	raw, err := s.repo.BuildNotificationRuleSnapshotJSON(ctx, companyID, ruleID)
	if err != nil {
		return nil, err
	}
	var snap configversion.NotificationRuleSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return nil, err
	}
	if len(patch) > 0 {
		base := snap.Payload
		if base == nil {
			base = map[string]any{}
		} else {
			base = cloneMap(base)
		}
		mergePayloadMaps(base, patch)
		snap.Payload = base
	}
	if status != nil && strings.TrimSpace(*status) != "" {
		snap.Status = strings.TrimSpace(*status)
	}
	return json.Marshal(snap)
}

func (s *adminService) buildProposedRBACSnapshotAfterRolePermRemove(ctx context.Context, companyID, roleID, permissionID string) ([]byte, error) {
	raw, err := s.repo.BuildRBACMatrixSnapshotJSON(ctx, companyID)
	if err != nil {
		return nil, err
	}
	var snap configversion.RBACMatrixSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return nil, err
	}
	filtered := make([]configversion.RolePermissionEntry, 0, len(snap.RolePermissions))
	for _, e := range snap.RolePermissions {
		if e.RoleID == roleID && e.PermissionID == permissionID {
			continue
		}
		filtered = append(filtered, e)
	}
	snap.RolePermissions = filtered
	return json.Marshal(snap)
}

func (s *adminService) buildProposedRBACSnapshotAfterDirectPermRemove(ctx context.Context, companyID, membershipID, permissionCode string) ([]byte, error) {
	raw, err := s.repo.BuildRBACMatrixSnapshotJSON(ctx, companyID)
	if err != nil {
		return nil, err
	}
	var snap configversion.RBACMatrixSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return nil, err
	}
	filtered := make([]configversion.DirectPermissionEntry, 0, len(snap.DirectPermissions))
	for _, e := range snap.DirectPermissions {
		if e.MembershipID == membershipID && e.PermissionCode == permissionCode {
			continue
		}
		filtered = append(filtered, e)
	}
	snap.DirectPermissions = filtered
	return json.Marshal(snap)
}

func (s *adminService) permissionCodeByID(ctx context.Context, permissionID string) (string, error) {
	perms, err := s.repo.ListPermissions(ctx)
	if err != nil {
		return "", err
	}
	for _, p := range perms {
		if p.PermissionID == permissionID {
			return p.PermissionCode, nil
		}
	}
	return "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "permission not found", nil)
}

func (s *adminService) submitNotificationPatchApproval(ctx context.Context, sub AdminSubject, ruleID string, patch map[string]any, status *string, reason string) (*PendingAdminChangeSummary, error) {
	proposed, err := s.buildProposedNotificationSnapshot(ctx, sub.CompanyID, ruleID, patch, status)
	if err != nil {
		return nil, err
	}
	baseVer, err := s.currentLiveVersionNo(ctx, sub.CompanyID, configversion.AggregateNotificationRule, ruleID)
	if err != nil {
		return nil, err
	}
	return s.queueConfigApproval(ctx, sub, InsertPendingAdminChangeInput{
		ID:                   s.idg.NewUUID(),
		CompanyID:            sub.CompanyID,
		ApprovalSubjectType:  configversion.ApprovalSubjectConfigSnapshot,
		AggregateType:        configversion.AggregateNotificationRule,
		AggregateID:          ruleID,
		ChangeType:           configversion.ChangeTypeNotificationPatch,
		ProposedSnapshotJSON: proposed,
		BaseLiveVersionNo:    &baseVer,
		RequestedBy:          sub.MembershipID,
		Reason:               reason,
	})
}

func (s *adminService) submitRBACRolePermRemoveApproval(ctx context.Context, sub AdminSubject, roleID, permissionID, reason string) (*PendingAdminChangeSummary, error) {
	proposed, err := s.buildProposedRBACSnapshotAfterRolePermRemove(ctx, sub.CompanyID, roleID, permissionID)
	if err != nil {
		return nil, err
	}
	baseVer, err := s.currentLiveVersionNo(ctx, sub.CompanyID, configversion.AggregateRBACMatrix, "")
	if err != nil {
		return nil, err
	}
	return s.queueConfigApproval(ctx, sub, InsertPendingAdminChangeInput{
		ID:                   s.idg.NewUUID(),
		CompanyID:            sub.CompanyID,
		ApprovalSubjectType:  configversion.ApprovalSubjectConfigSnapshot,
		AggregateType:        configversion.AggregateRBACMatrix,
		AggregateID:          "",
		ChangeType:           configversion.ChangeTypeRBACPermissionRemove,
		ProposedSnapshotJSON: proposed,
		BaseLiveVersionNo:    &baseVer,
		RequestedBy:          sub.MembershipID,
		Reason:               reason,
	})
}

func (s *adminService) submitRBACDirectPermRemoveApproval(ctx context.Context, sub AdminSubject, membershipID, permissionCode, reason string) (*PendingAdminChangeSummary, error) {
	proposed, err := s.buildProposedRBACSnapshotAfterDirectPermRemove(ctx, sub.CompanyID, membershipID, permissionCode)
	if err != nil {
		return nil, err
	}
	baseVer, err := s.currentLiveVersionNo(ctx, sub.CompanyID, configversion.AggregateRBACMatrix, "")
	if err != nil {
		return nil, err
	}
	return s.queueConfigApproval(ctx, sub, InsertPendingAdminChangeInput{
		ID:                   s.idg.NewUUID(),
		CompanyID:            sub.CompanyID,
		ApprovalSubjectType:  configversion.ApprovalSubjectConfigSnapshot,
		AggregateType:        configversion.AggregateRBACMatrix,
		AggregateID:          "",
		ChangeType:           configversion.ChangeTypeRBACDirectPermRemove,
		ProposedSnapshotJSON: proposed,
		BaseLiveVersionNo:    &baseVer,
		RequestedBy:          sub.MembershipID,
		Reason:               reason,
	})
}

func (s *adminService) SubmitConfigApproval(ctx context.Context, req SubmitConfigApprovalRequest) (*PendingAdminChangeSummary, error) {
	switch req.ChangeType {
	case configversion.ChangeTypeNotificationPatch:
		if err := s.authorize(ctx, req.Subject, "admin.notification_rule.update", ""); err != nil {
			return nil, err
		}
		if strings.TrimSpace(req.AggregateID) == "" {
			return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "aggregate_id required", nil)
		}
		return s.submitNotificationPatchApproval(ctx, req.Subject, req.AggregateID, req.Proposed, nil, req.Reason)
	case configversion.ChangeTypeRBACPermissionRemove:
		roleID, _ := req.Proposed["role_id"].(string)
		if err := s.authorize(ctx, req.Subject, "admin.role.permission.remove", roleID); err != nil {
			return nil, err
		}
		permID, _ := req.Proposed["permission_id"].(string)
		if roleID == "" || permID == "" {
			return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "role_id and permission_id required", nil)
		}
		return s.submitRBACRolePermRemoveApproval(ctx, req.Subject, roleID, permID, req.Reason)
	case configversion.ChangeTypeRBACDirectPermRemove:
		if err := s.requireRbacManage(ctx, req.Subject); err != nil {
			return nil, err
		}
		memID, _ := req.Proposed["membership_id"].(string)
		code, _ := req.Proposed["permission_code"].(string)
		if memID == "" || code == "" {
			return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "membership_id and permission_code required", nil)
		}
		return s.submitRBACDirectPermRemoveApproval(ctx, req.Subject, memID, code, req.Reason)
	default:
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid change_type", nil)
	}
}

func (s *adminService) ListConfigApprovals(ctx context.Context, req ListConfigApprovalsRequest) (*ConfigApprovalListView, error) {
	if err := s.authorizeConfigApprovalRead(ctx, req.Subject); err != nil {
		return nil, err
	}
	rows, err := s.repo.ListPendingAdminChanges(ctx, req.Subject.CompanyID, req.Status, req.AggregateType, req.Limit)
	if err != nil {
		return nil, err
	}
	items := make([]PendingAdminChangeSummary, 0, len(rows))
	for _, row := range rows {
		summary := s.safeApprovalSummary(&row)
		items = append(items, ToPendingSummary(row, summary))
	}
	return &ConfigApprovalListView{Items: items}, nil
}

func (s *adminService) GetConfigApproval(ctx context.Context, req GetConfigApprovalRequest) (*PendingAdminChangeSummary, error) {
	if err := s.authorizeConfigApprovalRead(ctx, req.Subject); err != nil {
		return nil, err
	}
	row, err := s.repo.GetPendingAdminChange(ctx, req.Subject.CompanyID, req.ApprovalID)
	if err != nil {
		return nil, err
	}
	summary := s.safeApprovalSummary(row)
	out := ToPendingSummary(*row, summary)
	return &out, nil
}

func (s *adminService) ApproveConfigApproval(ctx context.Context, req ApproveConfigApprovalRequest) (*PendingAdminChangeSummary, error) {
	if err := s.authorizeConfigApprovalDecide(ctx, req.Subject); err != nil {
		return nil, err
	}
	row, err := s.repo.GetPendingAdminChange(ctx, req.Subject.CompanyID, req.ApprovalID)
	if err != nil {
		return nil, err
	}
	if row.Status != configversion.ApprovalStatusPending {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeApprovalNotPending, "approval is not pending", nil)
	}
	if row.RequestedBy == req.Subject.MembershipID {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodeSelfApprovalNotAllowed, "requester cannot approve own request", nil)
	}
	if err := s.checkStaleProposal(ctx, row); err != nil {
		return nil, err
	}
	val, err := s.ValidateConfiguration(ctx, ValidateConfigurationRequest{Subject: req.Subject})
	if err != nil {
		return nil, err
	}
	if val != nil && !val.Passed {
		return nil, &perr.HTTPError{
			Code:       perr.CodeDeferredValidationBlock,
			Message:    "configuration validation failed",
			HTTPStatus: http.StatusUnprocessableEntity,
			Details: map[string]any{
				"blocking": val.Summary.Blocking,
			},
		}
	}
	result, err := s.repo.ApplyPendingApprovalInTx(ctx, ApplyPendingApprovalInput{
		ApprovalID:   req.ApprovalID,
		CompanyID:    req.Subject.CompanyID,
		ReviewedBy:   req.Subject.MembershipID,
		ActorUserID:  req.Subject.UserID,
		VersionRowID: s.idg.NewUUID(),
		CreatedBy:    req.Subject.MembershipID,
	}, *row)
	if err != nil {
		return nil, err
	}
	if row.AggregateType == configversion.AggregateRBACMatrix {
		s.invalidateEffectiveAccessForCompany(ctx, req.Subject.CompanyID)
	}
	updated, err := s.repo.GetPendingAdminChange(ctx, req.Subject.CompanyID, req.ApprovalID)
	if err != nil {
		return nil, err
	}
	s.appendApprovalAudit(ctx, req.Subject, "admin.config.approval.approved", updated, map[string]any{
		"approval_id":    updated.ID,
		"aggregate_type": updated.AggregateType,
		"version_no":     result.PostApplyVersionNo,
		"reviewed_by":    req.Subject.MembershipID,
		"status":         updated.Status,
	})
	s.appendVersionAudit(ctx, req.Subject, "admin.version."+versionAuditSuffix(updated.AggregateType)+".snapshot_created",
		updated.AggregateType, versionAuditResourceID(updated), map[string]any{
			"version_no":     result.PostApplyVersionNo,
			"aggregate_type": updated.AggregateType,
			"source":         configversion.SourceApprovalApply,
		})
	summary := s.safeApprovalSummary(updated)
	out := ToPendingSummary(*updated, summary)
	return &out, nil
}

func versionAuditSuffix(aggregateType string) string {
	if aggregateType == configversion.AggregateRBACMatrix {
		return "rbac"
	}
	return "notification"
}

func versionAuditResourceID(row *PendingAdminChange) string {
	if row.AggregateType == configversion.AggregateNotificationRule {
		return row.AggregateID
	}
	return row.CompanyID
}

func (s *adminService) RejectConfigApproval(ctx context.Context, req RejectConfigApprovalRequest) (*PendingAdminChangeSummary, error) {
	if err := s.authorizeConfigApprovalDecide(ctx, req.Subject); err != nil {
		return nil, err
	}
	row, err := s.repo.GetPendingAdminChange(ctx, req.Subject.CompanyID, req.ApprovalID)
	if err != nil {
		return nil, err
	}
	if row.RequestedBy == req.Subject.MembershipID {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodeSelfApprovalNotAllowed, "requester cannot reject own request", nil)
	}
	updated, err := s.repo.UpdatePendingAdminChangeDecision(ctx, req.Subject.CompanyID, req.ApprovalID,
		configversion.ApprovalStatusRejected, req.Subject.MembershipID, req.RejectReason)
	if err != nil {
		return nil, err
	}
	s.appendApprovalAudit(ctx, req.Subject, "admin.config.approval.rejected", updated, map[string]any{
		"approval_id":    updated.ID,
		"aggregate_type": updated.AggregateType,
		"reviewed_by":    req.Subject.MembershipID,
		"status":         updated.Status,
		"reject_reason":  strings.TrimSpace(req.RejectReason),
	})
	summary := s.safeApprovalSummary(updated)
	out := ToPendingSummary(*updated, summary)
	return &out, nil
}

func (s *adminService) CancelConfigApproval(ctx context.Context, req CancelConfigApprovalRequest) (*PendingAdminChangeSummary, error) {
	row, err := s.repo.GetPendingAdminChange(ctx, req.Subject.CompanyID, req.ApprovalID)
	if err != nil {
		return nil, err
	}
	if row.RequestedBy != req.Subject.MembershipID {
		if err := s.authorizeConfigApprovalDecide(ctx, req.Subject); err != nil {
			return nil, err
		}
	}
	updated, err := s.repo.UpdatePendingAdminChangeDecision(ctx, req.Subject.CompanyID, req.ApprovalID,
		configversion.ApprovalStatusCancelled, req.Subject.MembershipID, "")
	if err != nil {
		return nil, err
	}
	s.appendApprovalAudit(ctx, req.Subject, "admin.config.approval.cancelled", updated, map[string]any{
		"approval_id": updated.ID,
		"status":      updated.Status,
		"reviewed_by": req.Subject.MembershipID,
	})
	summary := s.safeApprovalSummary(updated)
	out := ToPendingSummary(*updated, summary)
	return &out, nil
}

func (s *adminService) CompareConfigApproval(ctx context.Context, req CompareConfigApprovalRequest) (*CompareConfigApprovalView, error) {
	if err := s.authorizeConfigApprovalRead(ctx, req.Subject); err != nil {
		return nil, err
	}
	row, err := s.repo.GetPendingAdminChange(ctx, req.Subject.CompanyID, req.ApprovalID)
	if err != nil {
		return nil, err
	}
	currentVer, err := s.currentLiveVersionNo(ctx, row.CompanyID, row.AggregateType, row.AggregateID)
	if err != nil {
		return nil, err
	}
	var currentJSON []byte
	switch row.AggregateType {
	case configversion.AggregateNotificationRule:
		if currentVer > 0 {
			detail, err := s.repo.GetNotificationRuleVersion(ctx, row.CompanyID, row.AggregateID, currentVer)
			if err != nil {
				return nil, err
			}
			currentJSON = detail.SnapshotJSON
		} else {
			currentJSON, err = s.repo.BuildNotificationRuleSnapshotJSON(ctx, row.CompanyID, row.AggregateID)
			if err != nil {
				return nil, err
			}
		}
	case configversion.AggregateRBACMatrix:
		if currentVer > 0 {
			detail, err := s.repo.GetRBACMatrixVersion(ctx, row.CompanyID, currentVer)
			if err != nil {
				return nil, err
			}
			currentJSON = detail.SnapshotJSON
		} else {
			currentJSON, err = s.repo.BuildRBACMatrixSnapshotJSON(ctx, row.CompanyID)
			if err != nil {
				return nil, err
			}
		}
	}
	cmp, err := configversion.CompareJSON(currentJSON, row.ProposedSnapshotJSON, currentVer, currentVer+1)
	if err != nil {
		return nil, err
	}
	return &CompareConfigApprovalView{
		ApprovalID:        row.ID,
		AggregateType:     row.AggregateType,
		AggregateID:       row.AggregateID,
		BaseLiveVersionNo: row.BaseLiveVersionNo,
		CurrentVersionNo:  currentVer,
		Compare: &CompareVersionsView{
			FromVersionNo: cmp.FromVersionNo,
			ToVersionNo:   cmp.ToVersionNo,
			ChangedKeys:   cmp.ChangedKeys,
			Equal:         cmp.Equal,
			Summary:       cmp.Details,
		},
		Summary: s.safeApprovalSummary(row),
	}, nil
}

func (s *adminService) safeApprovalSummary(row *PendingAdminChange) map[string]any {
	out := map[string]any{
		"change_type": row.ChangeType,
	}
	switch row.AggregateType {
	case configversion.AggregateNotificationRule:
		var snap configversion.NotificationRuleSnapshot
		if err := json.Unmarshal(row.ProposedSnapshotJSON, &snap); err == nil {
			out["rule_code"] = snap.RuleCode
			if snap.Payload != nil {
				if ch, ok := snap.Payload["channels"].(map[string]any); ok {
					out["channel_count"] = len(ch)
				}
			}
		}
	case configversion.AggregateRBACMatrix:
		var snap configversion.RBACMatrixSnapshot
		if err := json.Unmarshal(row.ProposedSnapshotJSON, &snap); err == nil {
			out["role_permission_count"] = len(snap.RolePermissions)
			out["direct_permission_count"] = len(snap.DirectPermissions)
		}
	}
	return out
}

func (s *adminService) appendApprovalAudit(ctx context.Context, sub AdminSubject, action string, row *PendingAdminChange, meta map[string]any) {
	if s.auditRepo == nil || row == nil {
		return
	}
	_ = s.auditRepo.Append(ctx, auditapp.Entry{
		ActorUserID:       sub.UserID,
		ActorMembershipID: sub.MembershipID,
		CompanyID:         sub.CompanyID,
		Action:            action,
		ResourceType:      "config_approval",
		ResourceID:        row.ID,
		Decision:          "allow",
		Metadata:          meta,
	})
}
