package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cobo/cobo_iam_services/internal/companyaccess/configversion"
)

// rbacRestorePlans computes, from the live state, the role plan and the direct grant plan of
// restoring raw (a snapshot or a proposal). Everything that decides, shows or fingerprints a
// restore reads it from here.
func (s *adminService) rbacRestorePlans(ctx context.Context, companyID string, raw []byte) (RBACRestorePlan, RBACDirectRestorePlan, error) {
	plan, err := BuildRBACRestorePlan(ctx, s.repo, companyID, raw)
	if err != nil {
		return RBACRestorePlan{}, RBACDirectRestorePlan{}, err
	}
	var target configversion.RBACMatrixSnapshot
	if err := json.Unmarshal(raw, &target); err != nil {
		return RBACRestorePlan{}, RBACDirectRestorePlan{}, err
	}
	currentRaw, err := s.repo.BuildRBACMatrixSnapshotJSON(ctx, companyID)
	if err != nil {
		return RBACRestorePlan{}, RBACDirectRestorePlan{}, err
	}
	var current configversion.RBACMatrixSnapshot
	if err := json.Unmarshal(currentRaw, &current); err != nil {
		return RBACRestorePlan{}, RBACDirectRestorePlan{}, err
	}
	return plan, RBACDirectPlanFor(current.DirectPermissions, target), nil
}

// stampRBACPlanDigest stores the digest of the plan a proposal has right now inside the proposal,
// so that approving it later applies only what was queued.
func (s *adminService) stampRBACPlanDigest(ctx context.Context, companyID string, raw []byte) ([]byte, error) {
	plan, direct, err := s.rbacRestorePlans(ctx, companyID, raw)
	if err != nil {
		return nil, err
	}
	var snap configversion.RBACMatrixSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return nil, err
	}
	snap.PlanDigest = RBACRestorePlanDigest(plan, direct)
	return json.Marshal(snap)
}

// rbacApprovalChanges lists what restoring raw (an enterprise-filtered RBAC snapshot or
// proposal) would change in the company right now: the plan the restore would execute, for roles
// and for direct grants. It is the single source for the approval decision, the compare view and
// the refusal of an approval that would change nothing.
func (s *adminService) rbacApprovalChanges(ctx context.Context, companyID string, raw []byte) ([]ApprovalChange, error) {
	plan, direct, err := s.rbacRestorePlans(ctx, companyID, raw)
	if err != nil {
		return nil, err
	}
	roles, err := s.repo.ListRoles(ctx, companyID)
	if err != nil {
		return nil, err
	}
	roleCode := make(map[string]string, len(roles))
	for _, r := range roles {
		roleCode[r.RoleID] = r.RoleCode
	}

	out := make([]ApprovalChange, 0, len(plan.Ops)+len(direct.Revoke)+len(direct.Grant))
	for _, op := range plan.Ops {
		action := "remove"
		if op.Add {
			action = "add"
		}
		out = append(out, ApprovalChange{
			Kind: "role_permission", Action: action, RoleID: op.RoleID, RoleCode: roleCode[op.RoleID],
			PermissionCode: op.PermissionCode, Critical: isCriticalForRestore(op.PermissionCode),
		})
	}
	for _, d := range direct.Revoke {
		out = append(out, ApprovalChange{
			Kind: "direct_permission", Action: "remove", MembershipID: d.MembershipID, PermissionCode: d.PermissionCode,
			Critical: isCriticalForRestore(d.PermissionCode) || requiresApprovalForDirectRemove(d.PermissionCode),
		})
	}
	for _, d := range direct.Grant {
		out = append(out, ApprovalChange{
			Kind: "direct_permission", Action: "add", MembershipID: d.MembershipID, PermissionCode: d.PermissionCode,
			Critical: isCriticalForRestore(d.PermissionCode),
		})
	}
	return out, nil
}

// rbacRestoreImpact describes what restoring raw would do to the company.
type rbacRestoreImpact struct {
	// Changes is true when at least one role permission or direct grant would change.
	Changes bool
	// Critical is true when a critical permission would be added or removed, on a role the
	// restore may change or as a direct grant.
	Critical bool
}

func (s *adminService) rbacRestoreImpact(ctx context.Context, companyID string, raw []byte) (rbacRestoreImpact, error) {
	changes, err := s.rbacApprovalChanges(ctx, companyID, raw)
	if err != nil {
		return rbacRestoreImpact{}, err
	}
	impact := rbacRestoreImpact{Changes: len(changes) > 0}
	for _, c := range changes {
		if c.Critical {
			impact.Critical = true
		}
	}
	return impact, nil
}

// routeRBACRollbackToApproval queues the rollback and returns the 202 APPROVAL_ROUTED error.
// Nothing is changed until another person approves it.
func (s *adminService) routeRBACRollbackToApproval(ctx context.Context, sub AdminSubject, targetVersionNo int, reason string, sanitized []byte) error {
	// Granting or revoking the invite permission is reserved to the primary admin on the
	// direct-permission routes; a rollback that does the same is held to the same rule. Checked
	// where every rollback is queued, so it also covers the in-transaction fallback.
	changes, err := s.rbacApprovalChanges(ctx, sub.CompanyID, sanitized)
	if err != nil {
		return err
	}
	for _, c := range changes {
		if c.Kind == "direct_permission" && c.PermissionCode == permissionInvite {
			if err := s.assertCanGrantInvitePermission(ctx, sub); err != nil {
				return err
			}
			break
		}
	}
	baseVer, err := s.currentLiveVersionNo(ctx, sub.CompanyID, configversion.AggregateRBACMatrix, "")
	if err != nil {
		return err
	}
	text := fmt.Sprintf("rollback to version %d: %s", targetVersionNo, strings.TrimSpace(reason))
	if runes := []rune(text); len(runes) > 500 { // by rune: a byte cut could split a multi-byte character
		text = string(runes[:500])
	}
	summary, err := s.queueConfigApproval(ctx, sub, InsertPendingAdminChangeInput{
		ID:                   s.idg.NewUUID(),
		CompanyID:            sub.CompanyID,
		ApprovalSubjectType:  configversion.ApprovalSubjectConfigSnapshot,
		AggregateType:        configversion.AggregateRBACMatrix,
		AggregateID:          "",
		ChangeType:           configversion.ChangeTypeRBACMatrixRollback,
		ProposedSnapshotJSON: sanitized,
		BaseLiveVersionNo:    &baseVer,
		RequestedBy:          sub.MembershipID,
		Reason:               text,
	})
	if err != nil {
		return err
	}
	return s.routeApprovalRouted(sub, summary)
}
