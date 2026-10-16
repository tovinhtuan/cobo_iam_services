package mysql

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	adhocapp "github.com/cobo/cobo_iam_services/internal/adhoc/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/pagination"
)

type Repository struct {
	db *sql.DB

	// finalizeLockWaitSeconds overrides finalizeLockWaitSeconds (tests).
	finalizeLockWaitSeconds *int

	// CF-16: deadlineDaysCol* cache the result of detecting whether
	// ad_hoc_proposals.proposed_deadline_days exists. Only a *successful*
	// information_schema lookup is cached (deadlineDaysColCached=true);
	// a transient failure must not permanently poison subsequent callers.
	// See hasProposedDeadlineDaysColumn in schema_caps.go.
	deadlineDaysColMu     sync.RWMutex
	deadlineDaysColCached bool
	deadlineDaysColOK     bool

	deadlineDayTypeColMu     sync.RWMutex
	deadlineDayTypeColCached bool
	deadlineDayTypeColOK     bool
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Insert(ctx context.Context, p adhocapp.ProposalDTO) (*adhocapp.ProposalDTO, error) {
	caps, err := r.loadProposalSchemaCaps(ctx)
	if err != nil {
		return nil, fmt.Errorf("check ad_hoc_proposals schema: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	embedDays := p.ProposedDeadlineDays
	dayTypeVal := nullableDayType(p.ProposedDeadlineDayType)
	switch {
	case caps.includeDeadlineDays && caps.includeDayType:
		overridesJSON, mErr := marshalProposalWorkflowPayload(p, nil)
		if mErr != nil {
			return nil, fmt.Errorf("marshal proposed_workflow_json: %w", mErr)
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO ad_hoc_proposals
				(proposal_id, company_id, type_id, status, proposed_workflow_json, proposed_t0_date,
				 proposed_deadline_date, proposed_deadline_days, proposed_deadline_day_type, change_note, created_by, process_controller_id)
			VALUES (?, ?, ?, ?, CAST(? AS JSON), ?, ?, ?, ?, ?, ?, ?)
		`, p.ProposalID, p.CompanyID, p.TypeID, p.Status, overridesJSON,
			nullableStr(p.ProposedT0Date), nullableStr(p.ProposedDeadlineDate), nullableInt(p.ProposedDeadlineDays), dayTypeVal,
			nullIfBlank(p.ChangeNote), p.CreatedBy, nullIfBlank(p.ProcessControllerID))
	case caps.includeDeadlineDays:
		overridesJSON, mErr := marshalProposalWorkflowPayload(p, nil)
		if mErr != nil {
			return nil, fmt.Errorf("marshal proposed_workflow_json: %w", mErr)
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO ad_hoc_proposals
				(proposal_id, company_id, type_id, status, proposed_workflow_json, proposed_t0_date,
				 proposed_deadline_date, proposed_deadline_days, change_note, created_by, process_controller_id)
			VALUES (?, ?, ?, ?, CAST(? AS JSON), ?, ?, ?, ?, ?, ?)
		`, p.ProposalID, p.CompanyID, p.TypeID, p.Status, overridesJSON,
			nullableStr(p.ProposedT0Date), nullableStr(p.ProposedDeadlineDate), nullableInt(p.ProposedDeadlineDays),
			nullIfBlank(p.ChangeNote), p.CreatedBy, nullIfBlank(p.ProcessControllerID))
	default:
		overridesJSON, mErr := marshalProposalWorkflowPayload(p, embedDays)
		if mErr != nil {
			return nil, fmt.Errorf("marshal proposed_workflow_json: %w", mErr)
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO ad_hoc_proposals
				(proposal_id, company_id, type_id, status, proposed_workflow_json, proposed_t0_date,
				 proposed_deadline_date, change_note, created_by, process_controller_id)
			VALUES (?, ?, ?, ?, CAST(? AS JSON), ?, ?, ?, ?, ?)
		`, p.ProposalID, p.CompanyID, p.TypeID, p.Status, overridesJSON,
			nullableStr(p.ProposedT0Date), nullableStr(p.ProposedDeadlineDate),
			nullIfBlank(p.ChangeNote), p.CreatedBy, nullIfBlank(p.ProcessControllerID))
	}
	if err != nil {
		return nil, fmt.Errorf("insert ad_hoc_proposal: %w", err)
	}

	for i, membershipID := range p.ReviewerMembershipIDs {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO ad_hoc_proposal_reviewers (proposal_id, company_id, membership_id, sort_order)
			VALUES (?, ?, ?, ?)
		`, p.ProposalID, p.CompanyID, membershipID, i); err != nil {
			return nil, fmt.Errorf("insert ad_hoc_proposal_reviewer: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.FindByID(ctx, p.CompanyID, p.ProposalID)
}

func (r *Repository) FindByID(ctx context.Context, companyID, proposalID string) (*adhocapp.ProposalDTO, error) {
	cols, caps, err := r.proposalDetailColumns(ctx)
	if err != nil {
		return nil, err
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT `+cols+`
		FROM ad_hoc_proposals
		WHERE proposal_id = ? AND company_id = ?
	`, proposalID, companyID)
	return scanProposalRow(row, caps)
}

func (r *Repository) UpdateDraft(ctx context.Context, upd adhocapp.DraftUpdate) (*adhocapp.ProposalDTO, error) {
	caps, err := r.loadProposalSchemaCaps(ctx)
	if err != nil {
		return nil, fmt.Errorf("check ad_hoc_proposals schema: %w", err)
	}

	var workflowJSON string
	switch {
	case upd.Workflow != nil:
		raw, mErr := json.Marshal(upd.Workflow)
		if mErr != nil {
			return nil, fmt.Errorf("marshal workflow snapshot: %w", mErr)
		}
		workflowJSON = string(raw)
	case upd.UseLegacyOverrides:
		raw, mErr := marshalProposedWorkflowJSON(upd.LegacyStepOverrides, nil)
		if mErr != nil {
			return nil, fmt.Errorf("marshal legacy overrides: %w", mErr)
		}
		workflowJSON = raw
	default:
		return nil, fmt.Errorf("UpdateDraft requires Workflow or UseLegacyOverrides")
	}

	now := time.Now().UTC()
	var res sql.Result
	dayTypeVal := nullableDayType(upd.ProposedDeadlineDayType)
	switch {
	case caps.includeDeadlineDays && caps.includeDayType:
		res, err = r.db.ExecContext(ctx, `
			UPDATE ad_hoc_proposals
			SET type_id = ?, change_note = ?, proposed_t0_date = ?, proposed_deadline_date = ?,
			    proposed_deadline_days = ?, proposed_deadline_day_type = ?, proposed_workflow_json = CAST(? AS JSON), updated_at = ?
			WHERE proposal_id = ? AND company_id = ? AND status = ?
		`, upd.TypeID, nullIfBlank(upd.ChangeNote), nullableStr(upd.ProposedT0Date), nullableStr(upd.ProposedDeadlineDate),
			nullableInt(upd.ProposedDeadlineDays), dayTypeVal, workflowJSON, now,
			upd.ProposalID, upd.CompanyID, upd.FromStatus)
	case caps.includeDeadlineDays:
		res, err = r.db.ExecContext(ctx, `
			UPDATE ad_hoc_proposals
			SET type_id = ?, change_note = ?, proposed_t0_date = ?, proposed_deadline_date = ?,
			    proposed_deadline_days = ?, proposed_workflow_json = CAST(? AS JSON), updated_at = ?
			WHERE proposal_id = ? AND company_id = ? AND status = ?
		`, upd.TypeID, nullIfBlank(upd.ChangeNote), nullableStr(upd.ProposedT0Date), nullableStr(upd.ProposedDeadlineDate),
			nullableInt(upd.ProposedDeadlineDays), workflowJSON, now,
			upd.ProposalID, upd.CompanyID, upd.FromStatus)
	default:
		// Embed days into JSON when column missing — keep snapshot authority for v2.
		if upd.Workflow != nil {
			res, err = r.db.ExecContext(ctx, `
				UPDATE ad_hoc_proposals
				SET type_id = ?, change_note = ?, proposed_t0_date = ?, proposed_deadline_date = ?,
				    proposed_workflow_json = CAST(? AS JSON), updated_at = ?
				WHERE proposal_id = ? AND company_id = ? AND status = ?
			`, upd.TypeID, nullIfBlank(upd.ChangeNote), nullableStr(upd.ProposedT0Date), nullableStr(upd.ProposedDeadlineDate),
				workflowJSON, now, upd.ProposalID, upd.CompanyID, upd.FromStatus)
		} else {
			raw, mErr := marshalProposedWorkflowJSON(upd.LegacyStepOverrides, upd.ProposedDeadlineDays)
			if mErr != nil {
				return nil, fmt.Errorf("marshal legacy overrides: %w", mErr)
			}
			res, err = r.db.ExecContext(ctx, `
				UPDATE ad_hoc_proposals
				SET type_id = ?, change_note = ?, proposed_t0_date = ?, proposed_deadline_date = ?,
				    proposed_workflow_json = CAST(? AS JSON), updated_at = ?
				WHERE proposal_id = ? AND company_id = ? AND status = ?
			`, upd.TypeID, nullIfBlank(upd.ChangeNote), nullableStr(upd.ProposedT0Date), nullableStr(upd.ProposedDeadlineDate),
				raw, now, upd.ProposalID, upd.CompanyID, upd.FromStatus)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("update ad_hoc_proposal draft: %w", err)
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		cur, findErr := r.FindByID(ctx, upd.CompanyID, upd.ProposalID)
		if findErr != nil {
			return nil, findErr
		}
		if cur.Status != adhocapp.StatusDraft {
			return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "proposal is not in draft state", nil)
		}
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "proposal draft was not updated", nil)
	}
	return r.FindByID(ctx, upd.CompanyID, upd.ProposalID)
}

func (r *Repository) UpdateStatus(ctx context.Context, upd adhocapp.StatusUpdate) (*adhocapp.ProposalDTO, bool, error) {
	now := time.Now().UTC()
	var q string
	var args []any

	switch upd.Status {
	case adhocapp.StatusPendingFocalApproval, adhocapp.StatusPendingAdminApproval, adhocapp.StatusCancelled:
		if upd.Status == adhocapp.StatusPendingFocalApproval && upd.Workflow != nil {
			raw, mErr := json.Marshal(upd.Workflow)
			if mErr != nil {
				return nil, false, fmt.Errorf("marshal workflow snapshot: %w", mErr)
			}
			if upd.PersistProposedDeadlineDayType {
				hasDayType, capErr := r.hasProposedDeadlineDayTypeColumn(ctx)
				if capErr != nil {
					return nil, false, fmt.Errorf("check ad_hoc_proposals schema: %w", capErr)
				}
				if hasDayType {
					q = `UPDATE ad_hoc_proposals SET status = ?, proposed_workflow_json = CAST(? AS JSON), proposed_deadline_day_type = ?, updated_at = ? WHERE proposal_id = ? AND company_id = ? AND status = ?`
					args = []any{upd.Status, string(raw), nullableDayType(upd.ProposedDeadlineDayType), now, upd.ProposalID, upd.CompanyID, upd.FromStatus}
				} else {
					q = `UPDATE ad_hoc_proposals SET status = ?, proposed_workflow_json = CAST(? AS JSON), updated_at = ? WHERE proposal_id = ? AND company_id = ? AND status = ?`
					args = []any{upd.Status, string(raw), now, upd.ProposalID, upd.CompanyID, upd.FromStatus}
				}
			} else {
				q = `UPDATE ad_hoc_proposals SET status = ?, proposed_workflow_json = CAST(? AS JSON), updated_at = ? WHERE proposal_id = ? AND company_id = ? AND status = ?`
				args = []any{upd.Status, string(raw), now, upd.ProposalID, upd.CompanyID, upd.FromStatus}
			}
		} else if upd.Status == adhocapp.StatusPendingFocalApproval && upd.PersistProposedDeadlineDayType {
			hasDayType, capErr := r.hasProposedDeadlineDayTypeColumn(ctx)
			if capErr != nil {
				return nil, false, fmt.Errorf("check ad_hoc_proposals schema: %w", capErr)
			}
			if hasDayType {
				q = `UPDATE ad_hoc_proposals SET status = ?, proposed_deadline_day_type = ?, updated_at = ? WHERE proposal_id = ? AND company_id = ? AND status = ?`
				args = []any{upd.Status, nullableDayType(upd.ProposedDeadlineDayType), now, upd.ProposalID, upd.CompanyID, upd.FromStatus}
			} else {
				q = `UPDATE ad_hoc_proposals SET status = ?, updated_at = ? WHERE proposal_id = ? AND company_id = ? AND status = ?`
				args = []any{upd.Status, now, upd.ProposalID, upd.CompanyID, upd.FromStatus}
			}
		} else {
			q = `UPDATE ad_hoc_proposals SET status = ?, updated_at = ? WHERE proposal_id = ? AND company_id = ? AND status = ?`
			args = []any{upd.Status, now, upd.ProposalID, upd.CompanyID, upd.FromStatus}
		}
	case adhocapp.StatusRejected:
		q = `UPDATE ad_hoc_proposals SET status = ?, rejected_by = ?, rejected_at = ?, reject_reason = ?, updated_at = ?
		     WHERE proposal_id = ? AND company_id = ? AND status = ?`
		args = []any{upd.Status, upd.ActorMembershipID, now, upd.RejectReason, now, upd.ProposalID, upd.CompanyID, upd.FromStatus}
	case adhocapp.StatusApproved:
		q = `UPDATE ad_hoc_proposals SET status = ?, admin_approved_by = ?, admin_approved_at = ?,
		     record_id = NULLIF(?, ''), workflow_instance_id = NULLIF(?, ''),
		     final_t0_date = ?, final_deadline_date = ?, adjustment_note = ?, updated_at = ?
		     WHERE proposal_id = ? AND company_id = ? AND status = ?`
		args = []any{upd.Status, upd.ActorMembershipID, now, upd.RecordID, upd.WorkflowInstanceID, nullableStr(upd.FinalT0Date), nullableStr(upd.FinalDeadlineDate), nullIfBlank(upd.AdjustmentNote), now, upd.ProposalID, upd.CompanyID, upd.FromStatus}
	default:
		return nil, false, fmt.Errorf("unknown status transition: %s", upd.Status)
	}

	// Only persist focal approval metadata when the transition came from a real focal review.
	if upd.Status == adhocapp.StatusPendingAdminApproval && upd.SetFocalApprovalMetadata {
		q = `UPDATE ad_hoc_proposals SET status = ?, focal_approved_by = ?, focal_approved_at = ?, updated_at = ?
		     WHERE proposal_id = ? AND company_id = ? AND status = ?`
		args = []any{upd.Status, upd.ActorMembershipID, now, now, upd.ProposalID, upd.CompanyID, upd.FromStatus}
	}

	var execer interface {
		ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	} = r.db
	var tx *sql.Tx
	if upd.RefuseIfAllApproved {
		var err error
		if tx, err = r.db.BeginTx(ctx, nil); err != nil {
			return nil, false, err
		}
		defer func() { _ = tx.Rollback() }()
		if err := refuseIfAllApproved(ctx, tx, upd.CompanyID, upd.ProposalID); err != nil {
			return nil, false, err
		}
		execer = tx
	}
	res, err := execer.ExecContext(ctx, q, args...)
	if err != nil {
		return nil, false, fmt.Errorf("update ad_hoc_proposal status: %w", err)
	}
	if tx != nil {
		if err := tx.Commit(); err != nil {
			return nil, false, err
		}
	}
	n, _ := res.RowsAffected()
	applied := n == 1
	if applied {
		dto, err := r.FindByID(ctx, upd.CompanyID, upd.ProposalID)
		return dto, true, err
	}

	// ADR-2 RowsAffected decision tree (n == 0): the WHERE ... AND status = ?
	// guard matched no row. Re-read by proposal_id to disambiguate:
	//   - row not found            -> 404 CodeNotFound
	//   - current_status == target -> idempotent replay success (EV-2)
	//   - otherwise                -> 409 CodeStateConflict (lost-update guard, CF-02)
	cur, findErr := r.FindByID(ctx, upd.CompanyID, upd.ProposalID)
	if findErr != nil {
		if httpErr, ok := perr.AsHTTPError(findErr); ok && httpErr.HTTPStatus == http.StatusNotFound {
			return nil, false, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "proposal not found", nil)
		}
		return nil, false, findErr
	}
	if cur.Status == upd.Status {
		return cur, false, nil
	}
	conflictErr := perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "proposal status changed concurrently", nil)
	conflictErr.Details = map[string]any{
		"proposal_id":             upd.ProposalID,
		"current_status":          cur.Status,
		"expected_from_status":    upd.FromStatus,
		"attempted_target_status": upd.Status,
	}
	return nil, false, conflictErr
}

func (r *Repository) ReserveAdminApproval(ctx context.Context, in adhocapp.ReserveAdminApprovalInput) (*adhocapp.AdminApprovalReservation, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	cols, caps, err := r.proposalDetailColumns(ctx)
	if err != nil {
		return nil, err
	}
	row := tx.QueryRowContext(ctx, `
		SELECT `+cols+`,
		       approval_idempotency_key, approval_record_id, approval_workflow_instance_id
		FROM ad_hoc_proposals
		WHERE proposal_id = ? AND company_id = ?
		FOR UPDATE
	`, in.ProposalID, in.CompanyID)
	p, approvalKey, progressRecordID, progressWorkflowID, err := scanProposalWithApprovalState(row, caps)
	if err != nil {
		return nil, err
	}
	res := &adhocapp.AdminApprovalReservation{
		Proposal:           p,
		ProgressRecordID:   progressRecordID,
		ProgressWorkflowID: progressWorkflowID,
	}
	if p.Status == adhocapp.StatusApproved {
		res.ReplayApproved = true
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return res, nil
	}
	if p.Status != adhocapp.StatusPendingAdminApproval {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "proposal is not pending admin approval", nil)
	}

	incomingKey := strings.TrimSpace(in.IdempotencyKey)
	switch {
	case approvalKey == "":
		_, err = tx.ExecContext(ctx, `
			UPDATE ad_hoc_proposals
			SET approval_idempotency_key = ?, approval_started_at = ?, approval_last_error = NULL
			WHERE proposal_id = ? AND company_id = ?
		`, nullIfBlank(incomingKey), time.Now().UTC(), in.ProposalID, in.CompanyID)
		if err != nil {
			return nil, fmt.Errorf("reserve admin approval: %w", err)
		}
	case incomingKey != "" && approvalKey == incomingKey:
		// Same attempt: safe to resume.
	case progressRecordID != "":
		// Prior attempt already materialized a disclosure record. Reuse and finalize.
	default:
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "admin approval is already in progress", nil)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return res, nil
}

func (r *Repository) SaveAdminApprovalProgress(ctx context.Context, companyID, proposalID, idemKey, recordID, workflowID, lastError string) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE ad_hoc_proposals
		SET approval_record_id = COALESCE(approval_record_id, NULLIF(?, '')),
		    approval_workflow_instance_id = COALESCE(approval_workflow_instance_id, NULLIF(?, '')),
		    approval_last_error = NULLIF(?, ''),
		    approval_started_at = COALESCE(approval_started_at, ?)
		WHERE proposal_id = ? AND company_id = ? AND status = ?
		  AND (approval_idempotency_key = ? OR ? = '')
	`, recordID, workflowID, nullIfBlank(lastError), time.Now().UTC(), proposalID, companyID, adhocapp.StatusPendingAdminApproval, idemKey, idemKey)
	if err != nil {
		return fmt.Errorf("save admin approval progress: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "proposal cannot save admin approval progress in current state", nil)
	}
	return nil
}

func (r *Repository) CompleteAdminApproval(ctx context.Context, upd adhocapp.StatusUpdate, idemKey string) (*adhocapp.ProposalDTO, bool, error) {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, `
		UPDATE ad_hoc_proposals
		SET status = ?, admin_approved_by = ?, admin_approved_at = ?,
		    record_id = NULLIF(COALESCE(?, approval_record_id), ''),
		    workflow_instance_id = NULLIF(COALESCE(?, approval_workflow_instance_id), ''),
		    final_t0_date = ?, final_deadline_date = ?, adjustment_note = ?, updated_at = ?,
		    approval_last_error = NULL
		WHERE proposal_id = ? AND company_id = ? AND status = ?
		  AND (approval_idempotency_key = ? OR ? = '' OR approval_record_id IS NOT NULL)
	`, upd.Status, upd.ActorMembershipID, now, nullIfBlank(upd.RecordID), nullIfBlank(upd.WorkflowInstanceID),
		nullableStr(upd.FinalT0Date), nullableStr(upd.FinalDeadlineDate), nullIfBlank(upd.AdjustmentNote), now,
		upd.ProposalID, upd.CompanyID, adhocapp.StatusPendingAdminApproval, idemKey, idemKey)
	if err != nil {
		return nil, false, fmt.Errorf("complete admin approval: %w", err)
	}
	n, _ := res.RowsAffected()
	applied := n != 0
	if !applied {
		// EV-2/EV-3 disambiguation: the guarded UPDATE matched no row — re-read to
		// distinguish an idempotent replay (already approved) from a genuine conflict.
		cur, findErr := r.FindByID(ctx, upd.CompanyID, upd.ProposalID)
		if findErr != nil {
			return nil, false, findErr
		}
		if cur.Status == adhocapp.StatusApproved {
			return cur, false, nil
		}
		return nil, false, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "proposal cannot be approved in current state", nil)
	}
	dto, err := r.FindByID(ctx, upd.CompanyID, upd.ProposalID)
	return dto, true, err
}

// ReserveVote is Phase A of §6.5: a single FOR UPDATE transaction that records
// one reviewer's vote (idempotently — a repeat vote from the same reviewer is
// not double-counted) and reports whether this was the last outstanding vote.
func (r *Repository) ReserveVote(ctx context.Context, in adhocapp.ReserveVoteInput) (*adhocapp.VoteReservation, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	cols, caps, err := r.proposalDetailColumns(ctx)
	if err != nil {
		return nil, err
	}
	row := tx.QueryRowContext(ctx, `
		SELECT `+cols+`
		FROM ad_hoc_proposals
		WHERE proposal_id = ? AND company_id = ?
		FOR UPDATE
	`, in.ProposalID, in.CompanyID)
	p, err := scanProposalRow(row, caps)
	if err != nil {
		return nil, err
	}

	if p.Status != adhocapp.StatusApproved && p.Status != adhocapp.StatusPendingFocalApproval {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "proposal is not pending focal approval", nil)
	}

	var assigned int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(1) FROM ad_hoc_proposal_reviewers WHERE proposal_id = ? AND company_id = ? AND membership_id = ?
	`, in.ProposalID, in.CompanyID, in.ActorMembershipID).Scan(&assigned); err != nil {
		return nil, fmt.Errorf("check assigned reviewer: %w", err)
	}
	if assigned == 0 {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "actor is not an assigned reviewer for this proposal", nil)
	}

	if p.Status == adhocapp.StatusApproved {
		// Already finalized — idempotent replay (EV-2). Caller distinguishes via
		// Proposal.Status; Required/Completed are best-effort context only.
		required, completed, cErr := countReviewersAndApprovals(ctx, tx, in.ProposalID, in.CompanyID)
		if cErr != nil {
			return nil, cErr
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return &adhocapp.VoteReservation{Proposal: p, Required: required, Completed: completed, ActorMembershipID: in.ActorMembershipID, ActorUserID: in.ActorUserID}, nil
	}

	// Once finalize created the record from the current final values, a vote may not change them:
	// the workflow already runs on them.
	if in.FinalizeRecordID != "" && (in.FinalT0Date != nil || in.FinalDeadlineDate != nil || strings.TrimSpace(in.AdjustmentNote) != "") {
		if err := refuseFinalValueChange(ctx, tx, in); err != nil {
			return nil, err
		}
	}

	// Insert-if-not-voted. A repeat vote from the same reviewer keeps its first approval time, and a
	// blank field keeps the value entered before it (v1.2 §3.2).
	_, err = tx.ExecContext(ctx, `
		INSERT INTO ad_hoc_proposal_approvals
			(proposal_id, company_id, membership_id, approved_at, final_t0_date, final_deadline_date, adjustment_note, comment)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			final_t0_date = COALESCE(VALUES(final_t0_date), final_t0_date),
			final_deadline_date = COALESCE(VALUES(final_deadline_date), final_deadline_date),
			adjustment_note = COALESCE(VALUES(adjustment_note), adjustment_note),
			comment = COALESCE(VALUES(comment), comment)
	`, in.ProposalID, in.CompanyID, in.ActorMembershipID, time.Now().UTC(),
		nullableStr(in.FinalT0Date), nullableStr(in.FinalDeadlineDate), nullIfBlank(in.AdjustmentNote), nullIfBlank(in.Comment))
	if err != nil {
		return nil, fmt.Errorf("insert ad_hoc_proposal_approval: %w", err)
	}

	// The proposal carries the current final values (v1.2 §3.2), in vote order (this row lock): per
	// field, the value entered last wins and a blank field keeps the one before it. A proposal voted
	// on before this rule starts from the latest value a reviewer entered.
	if _, err := tx.ExecContext(ctx, `
		UPDATE ad_hoc_proposals p SET
			final_t0_date = COALESCE(?, p.final_t0_date, (
				SELECT a.final_t0_date FROM ad_hoc_proposal_approvals a
				WHERE a.proposal_id = p.proposal_id AND a.company_id = p.company_id AND a.final_t0_date IS NOT NULL
				ORDER BY a.approved_at DESC, a.membership_id DESC LIMIT 1)),
			final_deadline_date = COALESCE(?, p.final_deadline_date, (
				SELECT a.final_deadline_date FROM ad_hoc_proposal_approvals a
				WHERE a.proposal_id = p.proposal_id AND a.company_id = p.company_id AND a.final_deadline_date IS NOT NULL
				ORDER BY a.approved_at DESC, a.membership_id DESC LIMIT 1)),
			adjustment_note = COALESCE(?, NULLIF(p.adjustment_note, ''), (
				SELECT a.adjustment_note FROM ad_hoc_proposal_approvals a
				WHERE a.proposal_id = p.proposal_id AND a.company_id = p.company_id AND COALESCE(a.adjustment_note, '') <> ''
				ORDER BY a.approved_at DESC, a.membership_id DESC LIMIT 1))
		WHERE p.proposal_id = ? AND p.company_id = ?
	`, nullableStr(in.FinalT0Date), nullableStr(in.FinalDeadlineDate), nullIfBlank(in.AdjustmentNote), in.ProposalID, in.CompanyID); err != nil {
		return nil, fmt.Errorf("update ad_hoc_proposal current final values: %w", err)
	}
	// Read them back as YYYY-MM-DD for finalize (a DATE scanned as a string follows the DSN's
	// parseTime), and the proposal as FindByID returns it.
	var curT0, curDeadline sql.NullString
	var curNote string
	if err := tx.QueryRowContext(ctx, `
		SELECT DATE_FORMAT(final_t0_date, '%Y-%m-%d'), DATE_FORMAT(final_deadline_date, '%Y-%m-%d'), COALESCE(adjustment_note, '')
		FROM ad_hoc_proposals WHERE proposal_id = ? AND company_id = ?
	`, in.ProposalID, in.CompanyID).Scan(&curT0, &curDeadline, &curNote); err != nil {
		return nil, fmt.Errorf("read ad_hoc_proposal current final values: %w", err)
	}
	if p, err = scanProposalRow(tx.QueryRowContext(ctx, `
		SELECT `+cols+` FROM ad_hoc_proposals WHERE proposal_id = ? AND company_id = ?
	`, in.ProposalID, in.CompanyID), caps); err != nil {
		return nil, err
	}

	required, completed, err := countReviewersAndApprovals(ctx, tx, in.ProposalID, in.CompanyID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &adhocapp.VoteReservation{
		Proposal:              p,
		IsLastVote:            completed >= required,
		Required:              required,
		Completed:             completed,
		ActorMembershipID:     in.ActorMembershipID,
		ActorUserID:           in.ActorUserID,
		LastFinalT0Date:       nullStringPtr(curT0),
		LastFinalDeadlineDate: nullStringPtr(curDeadline),
		LastAdjustmentNote:    curNote,
	}, nil
}

func nullStringPtr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	return &v.String
}

// refuseIfAllApproved takes the proposal row lock that votes take (ReserveVote), then refuses with
// 409 when the proposal is pending and every assigned reviewer has approved (v1.2 §3.2). Nothing
// before the lock reads plainly, so the counts are as of the lock.
func refuseIfAllApproved(ctx context.Context, tx *sql.Tx, companyID, proposalID string) error {
	var status string
	err := tx.QueryRowContext(ctx, `
		SELECT status FROM ad_hoc_proposals WHERE proposal_id = ? AND company_id = ? FOR UPDATE
	`, proposalID, companyID).Scan(&status)
	if err == sql.ErrNoRows {
		return nil // the guarded UPDATE reports it
	}
	if err != nil {
		return fmt.Errorf("lock ad_hoc_proposal: %w", err)
	}
	if status != adhocapp.StatusPendingFocalApproval {
		return nil
	}
	required, completed, err := countReviewersAndApprovals(ctx, tx, proposalID, companyID)
	if err != nil {
		return err
	}
	if required > 0 && completed >= required {
		return adhocapp.AllReviewersApprovedError()
	}
	return nil
}

// ReleaseVote removes a reviewer's vote under the row lock votes take, while the proposal is still
// pending_focal_approval and finalize has not created its record: the compensation for a finalize
// that failed before any side effect. Once the record exists the votes stay (reject/cancel stay
// closed) and a retry finishes the proposal, instead of leaving the record behind a rejection.
func (r *Repository) ReleaseVote(ctx context.Context, companyID, proposalID, membershipID, recordID string) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	var status string
	err = tx.QueryRowContext(ctx, `
		SELECT status FROM ad_hoc_proposals WHERE proposal_id = ? AND company_id = ? FOR UPDATE
	`, proposalID, companyID).Scan(&status)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("lock ad_hoc_proposal: %w", err)
	}
	if status != adhocapp.StatusPendingFocalApproval {
		return false, tx.Commit()
	}
	exists, err := disclosureRecordExists(ctx, tx, companyID, recordID)
	if err != nil {
		return false, err
	}
	if exists {
		return false, tx.Commit()
	}
	res, err := tx.ExecContext(ctx, `
		DELETE FROM ad_hoc_proposal_approvals WHERE proposal_id = ? AND company_id = ? AND membership_id = ?
	`, proposalID, companyID, membershipID)
	if err != nil {
		return false, fmt.Errorf("release ad_hoc_proposal vote: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, tx.Commit()
}

func disclosureRecordExists(ctx context.Context, tx *sql.Tx, companyID, recordID string) (bool, error) {
	if strings.TrimSpace(recordID) == "" {
		return false, nil
	}
	var n int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM disclosure_records WHERE record_id = ? AND company_id = ?
	`, recordID, companyID).Scan(&n); err != nil {
		return false, fmt.Errorf("check finalize record: %w", err)
	}
	return n > 0, nil
}

// refuseFinalValueChange returns 409 when the finalize record exists and the vote enters a final
// value that differs from the current one. Runs under the proposal row lock.
func refuseFinalValueChange(ctx context.Context, tx *sql.Tx, in adhocapp.ReserveVoteInput) error {
	exists, err := disclosureRecordExists(ctx, tx, in.CompanyID, in.FinalizeRecordID)
	if err != nil || !exists {
		return err
	}
	var curT0, curDeadline sql.NullString
	var curNote string
	if err := tx.QueryRowContext(ctx, `
		SELECT DATE_FORMAT(final_t0_date, '%Y-%m-%d'), DATE_FORMAT(final_deadline_date, '%Y-%m-%d'), COALESCE(adjustment_note, '')
		FROM ad_hoc_proposals WHERE proposal_id = ? AND company_id = ?
	`, in.ProposalID, in.CompanyID).Scan(&curT0, &curDeadline, &curNote); err != nil {
		return fmt.Errorf("read ad_hoc_proposal final values: %w", err)
	}
	changes := func(in *string, cur sql.NullString) bool { return in != nil && (!cur.Valid || *in != cur.String) }
	note := strings.TrimSpace(in.AdjustmentNote)
	if changes(in.FinalT0Date, curT0) || changes(in.FinalDeadlineDate, curDeadline) || (note != "" && note != curNote) {
		return adhocapp.FinalValuesLockedError()
	}
	return nil
}

const finalizeLockWaitSeconds = 5

func finalizeLockName(companyID, proposalID string) string {
	sum := sha256.Sum256([]byte(companyID + ":" + proposalID))
	return "cobo_iam:adhoc_finalize:" + hex.EncodeToString(sum[:])[:40]
}

// LockFinalize takes a MySQL named lock on a dedicated connection, so one finalize runs at a time
// per proposal across API instances (same pattern as LockCompanyAdmins). The finalize writes run on
// other connections and commit before the lock is released.
func (r *Repository) LockFinalize(ctx context.Context, companyID, proposalID string) (func(), error) {
	conn, err := r.db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("lock ad-hoc finalize: %w", err)
	}
	name := finalizeLockName(companyID, proposalID)
	var got sql.NullInt64
	if err := conn.QueryRowContext(ctx, `SELECT GET_LOCK(?, ?)`, name, r.finalizeLockWait()).Scan(&got); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("lock ad-hoc finalize: %w", err)
	}
	if !got.Valid || got.Int64 != 1 {
		_ = conn.Close()
		return nil, adhocapp.FinalizeInProgressError()
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
			defer cancel()
			if _, err := conn.ExecContext(rctx, `DO RELEASE_LOCK(?)`, name); err != nil {
				// Never return a session that may still hold the lock to the pool: closing the
				// session releases it.
				_ = conn.Raw(func(any) error { return driver.ErrBadConn })
			}
			_ = conn.Close()
		})
	}, nil
}

func (r *Repository) finalizeLockWait() int {
	if r.finalizeLockWaitSeconds != nil {
		return *r.finalizeLockWaitSeconds
	}
	return finalizeLockWaitSeconds
}

func countReviewersAndApprovals(ctx context.Context, tx *sql.Tx, proposalID, companyID string) (required, completed int, err error) {
	if err = tx.QueryRowContext(ctx, `
		SELECT COUNT(1) FROM ad_hoc_proposal_reviewers WHERE proposal_id = ? AND company_id = ?
	`, proposalID, companyID).Scan(&required); err != nil {
		return 0, 0, fmt.Errorf("count reviewers: %w", err)
	}
	if err = tx.QueryRowContext(ctx, `
		SELECT COUNT(1) FROM ad_hoc_proposal_approvals WHERE proposal_id = ? AND company_id = ?
	`, proposalID, companyID).Scan(&completed); err != nil {
		return 0, 0, fmt.Errorf("count approvals: %w", err)
	}
	return required, completed, nil
}

// CompleteFinalize is Phase C of §6.5 — mirrors CompleteAdminApproval but
// keyed on FromStatus=pending_focal_approval with no idempotency-key complexity
// (finalize-retry safety instead comes from the deterministic RecordID, ADR-1B).
func (r *Repository) CompleteFinalize(ctx context.Context, upd adhocapp.StatusUpdate) (*adhocapp.ProposalDTO, bool, error) {
	now := time.Now().UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = tx.Rollback() }()
	// A failed finalize releases its vote (ReleaseVote); a concurrent finalize must not then approve
	// without every vote. Votes and releases take this row lock.
	var status string
	err = tx.QueryRowContext(ctx, `
		SELECT status FROM ad_hoc_proposals WHERE proposal_id = ? AND company_id = ? FOR UPDATE
	`, upd.ProposalID, upd.CompanyID).Scan(&status)
	if err != nil && err != sql.ErrNoRows {
		return nil, false, fmt.Errorf("lock ad_hoc_proposal: %w", err)
	}
	if err == nil && status == upd.FromStatus {
		required, completed, cErr := countReviewersAndApprovals(ctx, tx, upd.ProposalID, upd.CompanyID)
		if cErr != nil {
			return nil, false, cErr
		}
		if required > 0 && completed < required {
			return nil, false, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "proposal no longer has every approval", nil)
		}
	}
	res, err := tx.ExecContext(ctx, `
		UPDATE ad_hoc_proposals
		SET status = ?, admin_approved_by = ?, admin_approved_at = ?,
		    record_id = NULLIF(?, ''), workflow_instance_id = NULLIF(?, ''),
		    final_t0_date = ?, final_deadline_date = ?, adjustment_note = ?, updated_at = ?
		WHERE proposal_id = ? AND company_id = ? AND status = ?
	`, upd.Status, upd.ActorMembershipID, now, upd.RecordID, upd.WorkflowInstanceID,
		nullableStr(upd.FinalT0Date), nullableStr(upd.FinalDeadlineDate), nullIfBlank(upd.AdjustmentNote), now,
		upd.ProposalID, upd.CompanyID, upd.FromStatus)
	if err != nil {
		return nil, false, fmt.Errorf("complete finalize: %w", err)
	}
	n, _ := res.RowsAffected()
	if err := tx.Commit(); err != nil {
		return nil, false, err
	}
	if n == 0 {
		// EV-2/EV-3 disambiguation: guarded UPDATE matched no row — re-read to
		// distinguish an idempotent replay (already approved by a concurrent
		// finalize) from a genuine conflict.
		cur, findErr := r.FindByID(ctx, upd.CompanyID, upd.ProposalID)
		if findErr != nil {
			return nil, false, findErr
		}
		if cur.Status == adhocapp.StatusApproved {
			return cur, false, nil
		}
		return nil, false, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "proposal cannot be finalized in current state", nil)
	}
	dto, err := r.FindByID(ctx, upd.CompanyID, upd.ProposalID)
	return dto, true, err
}

func (r *Repository) IsAssignedReviewer(ctx context.Context, companyID, proposalID, membershipID string) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(1) FROM ad_hoc_proposal_reviewers WHERE proposal_id = ? AND company_id = ? AND membership_id = ?
	`, proposalID, companyID, membershipID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check assigned reviewer: %w", err)
	}
	return count > 0, nil
}

func (r *Repository) ListReviewers(ctx context.Context, companyID, proposalID string) ([]adhocapp.ReviewerDTO, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT rv.membership_id, COALESCE(u.full_name, ''), COALESCE(NULLIF(TRIM(u.email), ''), u.login_id)
		FROM ad_hoc_proposal_reviewers rv
		LEFT JOIN memberships m ON m.membership_id = rv.membership_id
		LEFT JOIN users u ON u.user_id = m.user_id
		WHERE rv.proposal_id = ? AND rv.company_id = ?
		ORDER BY rv.sort_order, rv.membership_id
	`, proposalID, companyID)
	if err != nil {
		return nil, fmt.Errorf("list reviewers: %w", err)
	}
	defer rows.Close()

	var out []adhocapp.ReviewerDTO
	for rows.Next() {
		var d adhocapp.ReviewerDTO
		if err := rows.Scan(&d.MembershipID, &d.FullName, &d.Email); err != nil {
			return nil, fmt.Errorf("scan reviewer: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *Repository) ListApprovals(ctx context.Context, companyID, proposalID string) ([]adhocapp.ApprovalDTO, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT membership_id, approved_at, final_t0_date, final_deadline_date, COALESCE(adjustment_note, ''), COALESCE(comment, '')
		FROM ad_hoc_proposal_approvals
		WHERE proposal_id = ? AND company_id = ?
		ORDER BY approved_at
	`, proposalID, companyID)
	if err != nil {
		return nil, fmt.Errorf("list approvals: %w", err)
	}
	defer rows.Close()

	var out []adhocapp.ApprovalDTO
	for rows.Next() {
		var d adhocapp.ApprovalDTO
		var finalT0, finalDeadline sql.NullString
		if err := rows.Scan(&d.MembershipID, &d.ApprovedAt, &finalT0, &finalDeadline, &d.AdjustmentNote, &d.Comment); err != nil {
			return nil, fmt.Errorf("scan approval: %w", err)
		}
		if finalT0.Valid {
			d.FinalT0Date = &finalT0.String
		}
		if finalDeadline.Valid {
			d.FinalDeadlineDate = &finalDeadline.String
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *Repository) List(ctx context.Context, companyID string, statusFilter []string, createdByMembershipID string, page, pageSize int) ([]adhocapp.ProposalDTO, int, error) {
	var whereExtra string
	var extraArgs []any
	if createdBy := strings.TrimSpace(createdByMembershipID); createdBy != "" {
		whereExtra += " AND created_by = ?"
		extraArgs = append(extraArgs, createdBy)
	}
	if len(statusFilter) > 0 {
		placeholders := strings.Repeat("?,", len(statusFilter))
		placeholders = placeholders[:len(placeholders)-1]
		whereExtra += " AND status IN (" + placeholders + ")"
		for _, s := range statusFilter {
			extraArgs = append(extraArgs, s)
		}
	}

	var total int
	countArgs := append([]any{companyID}, extraArgs...)
	if err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM ad_hoc_proposals WHERE company_id = ?"+whereExtra,
		countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := pagination.Offset(page, pageSize)
	listArgs := append(countArgs, pageSize, offset)
	cols, caps, err := r.proposalDetailColumns(ctx)
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+cols+`
		FROM ad_hoc_proposals
		WHERE company_id = ?`+whereExtra+`
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []adhocapp.ProposalDTO
	for rows.Next() {
		p, err := scanProposalRow(rows, caps)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *p)
	}
	return out, total, rows.Err()
}

// ── scan helpers ─────────────────────────────────────────────────────────────

type rowScanner interface {
	Scan(dest ...any) error
}

func scanProposalRow(row rowScanner, caps proposalSchemaCaps) (*adhocapp.ProposalDTO, error) {
	p, err := scanProposalCore(row, caps, false)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func scanProposalWithApprovalState(row rowScanner, caps proposalSchemaCaps) (*adhocapp.ProposalDTO, string, string, string, error) {
	var approvalKey, approvalRecordID, approvalWorkflowID sql.NullString
	p, err := scanProposalCore(row, caps, true, &approvalKey, &approvalRecordID, &approvalWorkflowID)
	if err != nil {
		return nil, "", "", "", err
	}
	return p, approvalKey.String, approvalRecordID.String, approvalWorkflowID.String, nil
}

func scanProposalCore(row rowScanner, caps proposalSchemaCaps, withApproval bool, approvalDest ...*sql.NullString) (*adhocapp.ProposalDTO, error) {
	var p adhocapp.ProposalDTO
	var overridesRaw string
	var t0Date, dlDate, finalT0Date, finalDeadlineDate, focalBy, adminBy, rejectedBy sql.NullString
	var dlDays sql.NullInt32
	var dayType sql.NullString
	var focalAt, adminAt, rejectedAt sql.NullTime
	var recordID, wfiID, processControllerID sql.NullString
	var changeNote, adjustmentNote, rejectReason sql.NullString

	dest := []any{
		&p.ProposalID, &p.CompanyID, &p.TypeID, &p.Status, &overridesRaw,
		&t0Date, &dlDate,
	}
	if caps.includeDeadlineDays {
		dest = append(dest, &dlDays)
	}
	if caps.includeDayType {
		dest = append(dest, &dayType)
	}
	dest = append(dest,
		&changeNote,
		&finalT0Date, &finalDeadlineDate, &adjustmentNote,
		&focalBy, &focalAt, &adminBy, &adminAt,
		&rejectedBy, &rejectedAt, &rejectReason,
		&recordID, &wfiID, &p.CreatedBy, &processControllerID, &p.CreatedAt, &p.UpdatedAt,
	)
	if withApproval {
		if len(approvalDest) != 3 {
			return nil, fmt.Errorf("scanProposalCore: approval destinations required")
		}
		dest = append(dest, approvalDest[0], approvalDest[1], approvalDest[2])
	}

	if err := row.Scan(dest...); err != nil {
		if err == sql.ErrNoRows {
			return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "proposal not found", nil)
		}
		return nil, fmt.Errorf("scan proposal: %w", err)
	}

	applyProposedWorkflowFields(&p, overridesRaw, dlDays)
	if t0Date.Valid {
		p.ProposedT0Date = &t0Date.String
	}
	if dlDate.Valid {
		p.ProposedDeadlineDate = &dlDate.String
	}
	if dayType.Valid && strings.TrimSpace(dayType.String) != "" {
		v := adhocapp.ProposalDeadlineDayType(strings.TrimSpace(dayType.String))
		p.ProposedDeadlineDayType = &v
	}
	if changeNote.Valid {
		p.ChangeNote = changeNote.String
	}
	if finalT0Date.Valid {
		p.FinalT0Date = &finalT0Date.String
	}
	if finalDeadlineDate.Valid {
		p.FinalDeadlineDate = &finalDeadlineDate.String
	}
	if adjustmentNote.Valid {
		p.AdjustmentNote = adjustmentNote.String
	}
	if focalBy.Valid {
		p.FocalApprovedBy = focalBy.String
	}
	if focalAt.Valid {
		t := focalAt.Time
		p.FocalApprovedAt = &t
	}
	if adminBy.Valid {
		p.AdminApprovedBy = adminBy.String
	}
	if adminAt.Valid {
		t := adminAt.Time
		p.AdminApprovedAt = &t
	}
	if rejectedBy.Valid {
		p.RejectedBy = rejectedBy.String
	}
	if rejectedAt.Valid {
		t := rejectedAt.Time
		p.RejectedAt = &t
	}
	if rejectReason.Valid {
		p.RejectReason = rejectReason.String
	}
	if recordID.Valid {
		p.RecordID = recordID.String
	}
	if wfiID.Valid {
		p.WorkflowInstanceID = wfiID.String
	}
	if processControllerID.Valid {
		p.ProcessControllerID = processControllerID.String
	}
	return &p, nil
}

func applyProposedWorkflowFields(p *adhocapp.ProposalDTO, overridesRaw string, dlDays sql.NullInt32) {
	steps, embeddedDays, snap, err := decodeProposalWorkflowPayload(overridesRaw)
	if err != nil {
		p.StepOverrides = []adhocapp.WorkflowStepOverride{}
		p.Workflow = nil
	} else {
		p.StepOverrides = steps
		p.Workflow = snap
		if embeddedDays != nil {
			p.ProposedDeadlineDays = embeddedDays
		}
	}
	if dlDays.Valid {
		v := int(dlDays.Int32)
		p.ProposedDeadlineDays = &v
	}
}

func nullableStr(s *string) any {
	if s == nil || *s == "" {
		return nil
	}
	return *s
}

func nullIfBlank(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func nullableInt(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullableDayType(v *adhocapp.ProposalDeadlineDayType) any {
	if v == nil || strings.TrimSpace(string(*v)) == "" {
		return nil
	}
	return string(*v)
}
