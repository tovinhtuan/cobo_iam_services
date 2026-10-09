package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cobo/cobo_iam_services/internal/companyaccess/configversion"
)

// rbacRestoreImpact describes what restoring raw (an enterprise-filtered RBAC snapshot)
// would do to the company.
type rbacRestoreImpact struct {
	// Changes is true when at least one role permission or direct grant would change.
	Changes bool
	// Critical is true when a critical permission would be added or removed, on a role the
	// restore may change or as a direct grant.
	Critical bool
}

func (s *adminService) rbacRestoreImpact(ctx context.Context, companyID string, raw []byte) (rbacRestoreImpact, error) {
	plan, err := BuildRBACRestorePlan(ctx, s.repo, companyID, raw)
	if err != nil {
		return rbacRestoreImpact{}, err
	}
	var target configversion.RBACMatrixSnapshot
	if err := json.Unmarshal(raw, &target); err != nil {
		return rbacRestoreImpact{}, err
	}
	currentRaw, err := s.repo.BuildRBACMatrixSnapshotJSON(ctx, companyID)
	if err != nil {
		return rbacRestoreImpact{}, err
	}
	var current configversion.RBACMatrixSnapshot
	if err := json.Unmarshal(currentRaw, &current); err != nil {
		return rbacRestoreImpact{}, err
	}
	direct := ComputeRBACDirectRestorePlan(current.DirectPermissions, target.DirectPermissions)
	return rbacRestoreImpact{
		Changes:  len(plan.Ops) > 0 || len(direct.Revoke) > 0 || len(direct.Grant) > 0,
		Critical: plan.TouchesCritical || direct.TouchesCritical,
	}, nil
}

// routeRBACRollbackToApproval queues the rollback and returns the 202 APPROVAL_ROUTED error.
// Nothing is changed until another person approves it.
func (s *adminService) routeRBACRollbackToApproval(ctx context.Context, sub AdminSubject, targetVersionNo int, reason string, sanitized []byte) error {
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
