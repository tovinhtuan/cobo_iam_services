package app

import (
	"context"
	"net/http"
	"strings"
	"time"

	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func (s *adminService) CreateEmergencyAccessRequest(ctx context.Context, req CreateEmergencyAccessRequest) (*EmergencyAccessGrant, error) {
	if err := s.requireActiveCompanyMember(ctx, req.Subject); err != nil {
		return nil, err
	}
	targetID := strings.TrimSpace(req.TargetMembershipID)
	reason := strings.TrimSpace(req.Reason)
	if targetID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "target_membership_id is required", nil)
	}
	if reason == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "reason is required", nil)
	}
	duration := req.RequestedDurationSeconds
	if duration <= 0 {
		duration = defaultEmergencyDurationSeconds
	}
	if duration < minEmergencyDurationSeconds || duration > maxEmergencyDurationSeconds {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "requested_duration_seconds out of allowed range", nil)
	}
	if err := s.requireMembershipInCompany(ctx, targetID, req.Subject.CompanyID); err != nil {
		return nil, err
	}
	target, err := s.repo.GetMembershipByID(ctx, targetID)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(strings.TrimSpace(target.Status), "active") {
		return nil, perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, "target membership must be active", nil)
	}
	sessionID := s.idg.NewUUID()
	row, err := s.repo.InsertEmergencyAccessGrant(ctx, InsertEmergencyAccessGrantInput{
		SessionID:             sessionID,
		CompanyID:             req.Subject.CompanyID,
		TargetMembershipID:    targetID,
		RequesterMembershipID: req.Subject.MembershipID,
		Reason:                reason,
		Scope:                 EmergencyScopeCompany,
		CapabilitySet:         append([]string(nil), DefaultBreakGlassCapabilities...),
		RequestedDurationSec:  duration,
	})
	if err != nil {
		return nil, err
	}
	s.appendBreakGlassAudit(ctx, req.Subject, "breakglass.session.created", row, nil)
	return row, nil
}

func (s *adminService) ListEmergencyAccessRequests(ctx context.Context, req ListEmergencyAccessRequests) (*EmergencyAccessListView, error) {
	if err := s.requireEmergencyAccessRead(ctx, req.Subject); err != nil {
		return nil, err
	}
	_, _ = s.repo.ExpireDueEmergencyGrants(ctx, req.Subject.CompanyID)
	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}
	items, err := s.repo.ListEmergencyAccessGrants(ctx, req.Subject.CompanyID, req.Status, req.TargetMembershipID, limit)
	if err != nil {
		return nil, err
	}
	return &EmergencyAccessListView{Items: items, Total: len(items)}, nil
}

func (s *adminService) GetEmergencyAccessRequest(ctx context.Context, req GetEmergencyAccessRequest) (*EmergencyAccessGrant, error) {
	row, err := s.repo.GetEmergencyAccessGrant(ctx, req.Subject.CompanyID, strings.TrimSpace(req.SessionID))
	if err != nil {
		return nil, err
	}
	if err := s.requireEmergencyGrantRead(ctx, req.Subject, row); err != nil {
		return nil, err
	}
	return row, nil
}

func (s *adminService) ApproveEmergencyAccessRequest(ctx context.Context, req ApproveEmergencyAccessRequest) (*EmergencyAccessGrant, error) {
	if err := s.requireEmergencyApprover(ctx, req.Subject); err != nil {
		return nil, err
	}
	sessionID := strings.TrimSpace(req.SessionID)
	row, err := s.repo.GetEmergencyAccessGrant(ctx, req.Subject.CompanyID, sessionID)
	if err != nil {
		return nil, err
	}
	if row.RequesterMembershipID == req.Subject.MembershipID {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "self-approval is not allowed", nil)
	}
	var out *EmergencyAccessGrant
	switch row.Status {
	case EmergencyStatusPendingFirst:
		if row.ApproverMembershipID1 == req.Subject.MembershipID {
			return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "duplicate approver", nil)
		}
		out, err = s.repo.RecordEmergencyFirstApproval(ctx, req.Subject.CompanyID, sessionID, req.Subject.MembershipID)
		if err != nil {
			return nil, err
		}
		s.appendBreakGlassAudit(ctx, req.Subject, "breakglass.session.approved", out, map[string]any{"approval_step": 1})
	case EmergencyStatusPendingSecond:
		if row.ApproverMembershipID1 == req.Subject.MembershipID {
			return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "duplicate approver", nil)
		}
		if row.ApproverMembershipID2 == req.Subject.MembershipID {
			return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "duplicate approver", nil)
		}
		duration := row.RequestedDurationSeconds
		if duration <= 0 {
			duration = defaultEmergencyDurationSeconds
		}
		expiresAt := time.Now().UTC().Add(time.Duration(duration) * time.Second)
		out, err = s.repo.ActivateEmergencyGrant(ctx, req.Subject.CompanyID, sessionID, req.Subject.MembershipID, expiresAt)
		if err != nil {
			return nil, err
		}
		s.appendBreakGlassAudit(ctx, req.Subject, "breakglass.session.approved", out, map[string]any{"approval_step": 2})
		s.appendBreakGlassAudit(ctx, req.Subject, "breakglass.session.activated", out, nil)
	default:
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "grant is not pending approval", nil)
	}
	return out, nil
}

func (s *adminService) DenyEmergencyAccessRequest(ctx context.Context, req DenyEmergencyAccessRequest) (*EmergencyAccessGrant, error) {
	if err := s.requireEmergencyApprover(ctx, req.Subject); err != nil {
		return nil, err
	}
	sessionID := strings.TrimSpace(req.SessionID)
	row, err := s.repo.GetEmergencyAccessGrant(ctx, req.Subject.CompanyID, sessionID)
	if err != nil {
		return nil, err
	}
	if row.RequesterMembershipID == req.Subject.MembershipID {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "requester cannot deny own request", nil)
	}
	out, err := s.repo.DenyEmergencyGrant(ctx, req.Subject.CompanyID, sessionID)
	if err != nil {
		return nil, err
	}
	s.appendBreakGlassAudit(ctx, req.Subject, "breakglass.session.denied", out, nil)
	return out, nil
}

func (s *adminService) CancelEmergencyAccessRequest(ctx context.Context, req CancelEmergencyAccessRequest) (*EmergencyAccessGrant, error) {
	sessionID := strings.TrimSpace(req.SessionID)
	row, err := s.repo.GetEmergencyAccessGrant(ctx, req.Subject.CompanyID, sessionID)
	if err != nil {
		return nil, err
	}
	if row.RequesterMembershipID != req.Subject.MembershipID {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "only requester can cancel", nil)
	}
	out, err := s.repo.CancelEmergencyGrant(ctx, req.Subject.CompanyID, sessionID)
	if err != nil {
		return nil, err
	}
	s.appendBreakGlassAudit(ctx, req.Subject, "breakglass.session.denied", out, map[string]any{"cancelled_by_requester": true})
	return out, nil
}

func (s *adminService) RevokeEmergencyAccessRequest(ctx context.Context, req RevokeEmergencyAccessRequest) (*EmergencyAccessGrant, error) {
	if err := s.requireEmergencyApprover(ctx, req.Subject); err != nil {
		return nil, err
	}
	sessionID := strings.TrimSpace(req.SessionID)
	out, err := s.repo.RevokeEmergencyGrant(ctx, req.Subject.CompanyID, sessionID)
	if err != nil {
		return nil, err
	}
	s.appendBreakGlassAudit(ctx, req.Subject, "breakglass.session.revoked", out, nil)
	return out, nil
}

func (s *adminService) GetEmergencyAccessTimeline(ctx context.Context, req GetEmergencyAccessTimelineRequest) (*ChangeTimelineView, error) {
	sessionID := strings.TrimSpace(req.SessionID)
	row, err := s.repo.GetEmergencyAccessGrant(ctx, req.Subject.CompanyID, sessionID)
	if err != nil {
		return nil, err
	}
	if err := s.requireEmergencyGrantRead(ctx, req.Subject, row); err != nil {
		return nil, err
	}
	return s.ListChangeTimeline(ctx, ListChangeTimelineRequest{
		Subject:      req.Subject,
		Limit:        req.Limit,
		ResourceType: "break_glass_session",
		ResourceID:   sessionID,
		Action:       "breakglass.session.",
		ActionPrefix: true,
	})
}

func (s *adminService) requireActiveCompanyMember(ctx context.Context, sub AdminSubject) error {
	if sub.CompanyID == "" || sub.MembershipID == "" {
		return perrNewBadRequest("company context required")
	}
	m, err := s.repo.GetMembershipByID(ctx, sub.MembershipID)
	if err != nil {
		return err
	}
	if m.CompanyID != sub.CompanyID {
		return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "cross-company access denied", nil)
	}
	if !strings.EqualFold(strings.TrimSpace(m.Status), "active") {
		return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "membership is not active", nil)
	}
	return nil
}

func (s *adminService) requireEmergencyApprover(ctx context.Context, sub AdminSubject) error {
	if err := s.requireActiveCompanyMember(ctx, sub); err != nil {
		return err
	}
	ok, err := s.hasPermissionRBACOnly(ctx, sub, "rbac.manage")
	if err != nil {
		return err
	}
	if !ok {
		return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "rbac.manage required", nil)
	}
	return nil
}

func (s *adminService) requireEmergencyAccessRead(ctx context.Context, sub AdminSubject) error {
	if err := s.requireActiveCompanyMember(ctx, sub); err != nil {
		return err
	}
	ok, err := s.hasPermissionRBACOnly(ctx, sub, "rbac.manage")
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "access denied", nil)
}

func (s *adminService) requireEmergencyGrantRead(ctx context.Context, sub AdminSubject, row *EmergencyAccessGrant) error {
	if row == nil {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "emergency access grant not found", nil)
	}
	if row.CompanyID != sub.CompanyID {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "emergency access grant not found", nil)
	}
	if err := s.requireActiveCompanyMember(ctx, sub); err != nil {
		return err
	}
	if sub.MembershipID == row.RequesterMembershipID ||
		sub.MembershipID == row.TargetMembershipID ||
		sub.MembershipID == row.ApproverMembershipID1 ||
		sub.MembershipID == row.ApproverMembershipID2 {
		return nil
	}
	ok, err := s.hasPermissionRBACOnly(ctx, sub, "rbac.manage")
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "access denied", nil)
}

func (s *adminService) hasPermissionRBACOnly(ctx context.Context, sub AdminSubject, permission string) (bool, error) {
	eff, err := s.auth.GetEffectiveAccess(ctx, sub.MembershipID, sub.CompanyID)
	if err != nil {
		return false, err
	}
	for _, p := range eff.Permissions {
		if p == permission {
			return true, nil
		}
	}
	return false, nil
}

func (s *adminService) hasBreakGlassPermissionOverlay(ctx context.Context, sub AdminSubject, permission string) (bool, error) {
	if sub.CompanyID == "" || sub.MembershipID == "" {
		return false, nil
	}
	_, _ = s.repo.ExpireDueEmergencyGrants(ctx, sub.CompanyID)
	grant, err := s.repo.GetActiveEmergencyGrantForTarget(ctx, sub.CompanyID, sub.MembershipID)
	if err != nil {
		return false, err
	}
	if grant == nil {
		return false, nil
	}
	for _, cap := range grant.CapabilitySet {
		for _, perm := range breakGlassCapabilityPermissions[cap] {
			if perm == permission {
				return true, nil
			}
		}
	}
	return false, nil
}

func (s *adminService) appendBreakGlassAudit(ctx context.Context, sub AdminSubject, action string, row *EmergencyAccessGrant, extra map[string]any) {
	if s.auditRepo == nil || row == nil {
		return
	}
	meta := map[string]any{
		"session_id":              row.SessionID,
		"target_membership_id":    row.TargetMembershipID,
		"requester_membership_id": row.RequesterMembershipID,
		"status":                  row.Status,
		"capability_set":          row.CapabilitySet,
		"reason":                  row.Reason,
	}
	if row.ExpiresAt != nil {
		meta["expires_at"] = row.ExpiresAt.UTC().Format(time.RFC3339)
	}
	for k, v := range extra {
		meta[k] = v
	}
	_ = s.auditRepo.Append(ctx, auditapp.Entry{
		ActorUserID:       sub.UserID,
		ActorMembershipID: sub.MembershipID,
		CompanyID:         sub.CompanyID,
		Action:            action,
		ResourceType:      "break_glass_session",
		ResourceID:        row.SessionID,
		Decision:          "allow",
		Metadata:          meta,
	})
}

// appendBreakGlassExpiredAudit is called when lazy expire detects overdue session.
func (s *adminService) appendBreakGlassExpiredAudit(ctx context.Context, companyID string, row *EmergencyAccessGrant) {
	if s.auditRepo == nil || row == nil {
		return
	}
	meta := map[string]any{
		"session_id":           row.SessionID,
		"target_membership_id": row.TargetMembershipID,
		"status":               EmergencyStatusExpired,
	}
	_ = s.auditRepo.Append(ctx, auditapp.Entry{
		CompanyID:    companyID,
		Action:       "breakglass.session.expired",
		ResourceType: "break_glass_session",
		ResourceID:   row.SessionID,
		Decision:     "allow",
		Metadata:     meta,
	})
}
