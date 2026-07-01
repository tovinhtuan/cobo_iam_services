package inmemory

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"time"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	"github.com/cobo/cobo_iam_services/internal/companyaccess/configversion"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

type pendingRow struct {
	row caapp.PendingAdminChange
}

func (r *AdminRepository) InsertPendingAdminChange(_ context.Context, in caapp.InsertPendingAdminChangeInput) (*caapp.PendingAdminChange, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.pendingApprovals {
		if p.row.CompanyID == in.CompanyID && p.row.AggregateType == in.AggregateType &&
			p.row.AggregateID == in.AggregateID && p.row.Status == configversion.ApprovalStatusPending {
			return nil, perr.NewHTTPError(http.StatusConflict, perr.CodePendingApprovalExists, "pending approval already exists for aggregate stream", nil)
		}
	}
	now := time.Now().UTC()
	item := caapp.PendingAdminChange{
		ID:                   in.ID,
		CompanyID:            in.CompanyID,
		ApprovalSubjectType:  in.ApprovalSubjectType,
		AggregateType:        in.AggregateType,
		AggregateID:          in.AggregateID,
		ChangeType:           in.ChangeType,
		ProposedSnapshotJSON: append([]byte(nil), in.ProposedSnapshotJSON...),
		BaseLiveVersionNo:    in.BaseLiveVersionNo,
		Status:               configversion.ApprovalStatusPending,
		RequestedBy:          in.RequestedBy,
		RequestedAt:          now,
		Reason:               in.Reason,
	}
	r.pendingApprovals = append(r.pendingApprovals, pendingRow{row: item})
	out := item
	return &out, nil
}

func (r *AdminRepository) GetPendingAdminChange(_ context.Context, companyID, approvalID string) (*caapp.PendingAdminChange, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, p := range r.pendingApprovals {
		if p.row.ID == approvalID && p.row.CompanyID == companyID {
			out := p.row
			return &out, nil
		}
	}
	return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "approval not found", nil)
}

func (r *AdminRepository) ListPendingAdminChanges(_ context.Context, companyID, status, aggregateType string, limit int) ([]caapp.PendingAdminChange, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	out := make([]caapp.PendingAdminChange, 0)
	for _, p := range r.pendingApprovals {
		if p.row.CompanyID != companyID {
			continue
		}
		if s := strings.TrimSpace(status); s != "" && p.row.Status != s {
			continue
		}
		if at := strings.TrimSpace(aggregateType); at != "" && p.row.AggregateType != at {
			continue
		}
		out = append(out, p.row)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].RequestedAt.After(out[j].RequestedAt)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *AdminRepository) HasPendingForAggregateStream(_ context.Context, companyID, aggregateType, aggregateID string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, p := range r.pendingApprovals {
		if p.row.CompanyID == companyID && p.row.AggregateType == aggregateType &&
			p.row.AggregateID == aggregateID && p.row.Status == configversion.ApprovalStatusPending {
			return true, nil
		}
	}
	return false, nil
}

func (r *AdminRepository) UpdatePendingAdminChangeDecision(_ context.Context, companyID, approvalID, status, reviewedBy, rejectReason string) (*caapp.PendingAdminChange, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, p := range r.pendingApprovals {
		if p.row.ID != approvalID || p.row.CompanyID != companyID {
			continue
		}
		if p.row.Status != configversion.ApprovalStatusPending {
			return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeApprovalNotPending, "approval is not pending", nil)
		}
		now := time.Now().UTC()
		p.row.Status = status
		p.row.ReviewedBy = reviewedBy
		p.row.ReviewedAt = &now
		p.row.RejectReason = rejectReason
		r.pendingApprovals[i] = p
		out := p.row
		return &out, nil
	}
	return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "approval not found", nil)
}

func (r *AdminRepository) GetMaxNotificationRuleVersionNo(_ context.Context, companyID, ruleID string) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	max := 0
	for _, v := range r.notificationVersions {
		if v.row.CompanyID == companyID && v.rule == ruleID && v.row.VersionNo > max {
			max = v.row.VersionNo
		}
	}
	return max, nil
}

func (r *AdminRepository) GetMaxRBACMatrixVersionNo(_ context.Context, companyID string) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	max := 0
	for _, v := range r.rbacVersions {
		if v.row.CompanyID == companyID && v.row.VersionNo > max {
			max = v.row.VersionNo
		}
	}
	return max, nil
}

func (r *AdminRepository) ApplyPendingApprovalInTx(ctx context.Context, in caapp.ApplyPendingApprovalInput, row caapp.PendingAdminChange) (*caapp.ApplyPendingApprovalResult, error) {
	r.mu.Lock()
	idx := -1
	for i, p := range r.pendingApprovals {
		if p.row.ID == in.ApprovalID && p.row.CompanyID == in.CompanyID {
			idx = i
			break
		}
	}
	if idx < 0 {
		r.mu.Unlock()
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "approval not found", nil)
	}
	if r.pendingApprovals[idx].row.Status != configversion.ApprovalStatusPending {
		r.mu.Unlock()
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeApprovalNotPending, "approval is not pending", nil)
	}
	raw := append([]byte(nil), row.ProposedSnapshotJSON...)
	aggType := row.AggregateType
	aggID := row.AggregateID
	r.mu.Unlock()

	switch aggType {
	case configversion.AggregateNotificationRule:
		if err := r.RestoreNotificationRuleFromSnapshot(ctx, in.CompanyID, raw); err != nil {
			return nil, err
		}
	case configversion.AggregateRBACMatrix:
		if err := r.RestoreRBACMatrixFromSnapshot(ctx, in.CompanyID, in.ActorUserID, raw); err != nil {
			return nil, err
		}
	default:
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "unsupported aggregate_type", nil)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	for i, p := range r.pendingApprovals {
		if p.row.ID != in.ApprovalID || p.row.CompanyID != in.CompanyID {
			continue
		}
		idx = i
		break
	}
	if idx < 0 || r.pendingApprovals[idx].row.Status != configversion.ApprovalStatusPending {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeApprovalNotPending, "approval is not pending", nil)
	}
	var postVersionNo int
	switch aggType {
	case configversion.AggregateNotificationRule:
		postVersionNo = 1
		for _, v := range r.notificationVersions {
			if v.row.CompanyID == in.CompanyID && v.rule == row.AggregateID && v.row.VersionNo >= postVersionNo {
				postVersionNo = v.row.VersionNo + 1
			}
		}
		r.notificationVersions = append(r.notificationVersions, notificationVersionRow{
			row: caapp.ConfigVersionRow{
				ID: in.VersionRowID, CompanyID: in.CompanyID, AggregateType: configversion.AggregateNotificationRule,
				AggregateID: aggID, VersionNo: postVersionNo, CreatedBy: in.CreatedBy,
				CreatedAt: time.Now().UTC(), Reason: "approval apply", Source: configversion.SourceApprovalApply,
			},
			raw: append([]byte(nil), raw...), rule: aggID,
		})
	case configversion.AggregateRBACMatrix:
		postVersionNo = 1
		for _, v := range r.rbacVersions {
			if v.row.CompanyID == in.CompanyID && v.row.VersionNo >= postVersionNo {
				postVersionNo = v.row.VersionNo + 1
			}
		}
		r.rbacVersions = append(r.rbacVersions, rbacVersionRow{
			row: caapp.ConfigVersionRow{
				ID: in.VersionRowID, CompanyID: in.CompanyID, AggregateType: configversion.AggregateRBACMatrix,
				VersionNo: postVersionNo, CreatedBy: in.CreatedBy, CreatedAt: time.Now().UTC(),
				Reason: "approval apply", Source: configversion.SourceApprovalApply,
			},
			raw: append([]byte(nil), raw...),
		})
	}
	now := time.Now().UTC()
	p := r.pendingApprovals[idx]
	p.row.Status = configversion.ApprovalStatusApproved
	p.row.ReviewedBy = in.ReviewedBy
	p.row.ReviewedAt = &now
	r.pendingApprovals[idx] = p
	return &caapp.ApplyPendingApprovalResult{PostApplyVersionNo: postVersionNo}, nil
}
