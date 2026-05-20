package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	adhocapp "github.com/cobo/cobo_iam_services/internal/adhoc/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Insert(ctx context.Context, p adhocapp.ProposalDTO) (*adhocapp.ProposalDTO, error) {
	overridesJSON, err := json.Marshal(p.StepOverrides)
	if err != nil {
		return nil, fmt.Errorf("marshal step_overrides: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO ad_hoc_proposals
			(proposal_id, company_id, type_id, status, proposed_workflow_json, proposed_t0_date,
			 proposed_deadline_date, change_note, created_by, process_controller_id)
		VALUES (?, ?, ?, ?, CAST(? AS JSON), ?, ?, ?, ?, ?)
	`, p.ProposalID, p.CompanyID, p.TypeID, p.Status, string(overridesJSON),
		nullableStr(p.ProposedT0Date), nullableStr(p.ProposedDeadlineDate),
		nullIfBlank(p.ChangeNote), p.CreatedBy, nullIfBlank(p.ProcessControllerID))
	if err != nil {
		return nil, fmt.Errorf("insert ad_hoc_proposal: %w", err)
	}
	return r.FindByID(ctx, p.CompanyID, p.ProposalID)
}

func (r *Repository) FindByID(ctx context.Context, companyID, proposalID string) (*adhocapp.ProposalDTO, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT proposal_id, company_id, type_id, status, proposed_workflow_json,
		       proposed_t0_date, proposed_deadline_date, change_note,
		       final_t0_date, final_deadline_date, adjustment_note,
		       focal_approved_by, focal_approved_at, admin_approved_by, admin_approved_at,
		       rejected_by, rejected_at, reject_reason,
		       record_id, workflow_instance_id, created_by, process_controller_id, created_at, updated_at
		FROM ad_hoc_proposals
		WHERE proposal_id = ? AND company_id = ?
	`, proposalID, companyID)
	return scanProposal(row)
}

func (r *Repository) UpdateStatus(ctx context.Context, upd adhocapp.StatusUpdate) (*adhocapp.ProposalDTO, error) {
	now := time.Now().UTC()
	var q string
	var args []any

	switch upd.Status {
	case adhocapp.StatusPendingFocalApproval, adhocapp.StatusPendingAdminApproval, adhocapp.StatusCancelled:
		q = `UPDATE ad_hoc_proposals SET status = ?, updated_at = ? WHERE proposal_id = ? AND company_id = ?`
		args = []any{upd.Status, now, upd.ProposalID, upd.CompanyID}
	case adhocapp.StatusRejected:
		q = `UPDATE ad_hoc_proposals SET status = ?, rejected_by = ?, rejected_at = ?, reject_reason = ?, updated_at = ?
		     WHERE proposal_id = ? AND company_id = ?`
		args = []any{upd.Status, upd.ActorMembershipID, now, upd.RejectReason, now, upd.ProposalID, upd.CompanyID}
	case adhocapp.StatusApproved:
		q = `UPDATE ad_hoc_proposals SET status = ?, admin_approved_by = ?, admin_approved_at = ?,
		     record_id = NULLIF(?, ''), workflow_instance_id = NULLIF(?, ''),
		     final_t0_date = ?, final_deadline_date = ?, adjustment_note = ?, updated_at = ?
		     WHERE proposal_id = ? AND company_id = ?`
		args = []any{upd.Status, upd.ActorMembershipID, now, upd.RecordID, upd.WorkflowInstanceID, nullableStr(upd.FinalT0Date), nullableStr(upd.FinalDeadlineDate), nullIfBlank(upd.AdjustmentNote), now, upd.ProposalID, upd.CompanyID}
	default:
		return nil, fmt.Errorf("unknown status transition: %s", upd.Status)
	}

	// Only persist focal approval metadata when the transition came from a real focal review.
	if upd.Status == adhocapp.StatusPendingAdminApproval && upd.SetFocalApprovalMetadata {
		q = `UPDATE ad_hoc_proposals SET status = ?, focal_approved_by = ?, focal_approved_at = ?, updated_at = ?
		     WHERE proposal_id = ? AND company_id = ?`
		args = []any{upd.Status, upd.ActorMembershipID, now, now, upd.ProposalID, upd.CompanyID}
	}

	res, err := r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("update ad_hoc_proposal status: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "proposal not found", nil)
	}
	return r.FindByID(ctx, upd.CompanyID, upd.ProposalID)
}

func (r *Repository) ReserveAdminApproval(ctx context.Context, in adhocapp.ReserveAdminApprovalInput) (*adhocapp.AdminApprovalReservation, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx, `
		SELECT proposal_id, company_id, type_id, status, proposed_workflow_json,
		       proposed_t0_date, proposed_deadline_date, change_note,
		       final_t0_date, final_deadline_date, adjustment_note,
		       focal_approved_by, focal_approved_at, admin_approved_by, admin_approved_at,
		       rejected_by, rejected_at, reject_reason,
		       record_id, workflow_instance_id, created_by, process_controller_id, created_at, updated_at,
		       approval_idempotency_key, approval_record_id, approval_workflow_instance_id
		FROM ad_hoc_proposals
		WHERE proposal_id = ? AND company_id = ?
		FOR UPDATE
	`, in.ProposalID, in.CompanyID)
	p, approvalKey, progressRecordID, progressWorkflowID, err := scanProposalWithApprovalState(row)
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

func (r *Repository) CompleteAdminApproval(ctx context.Context, upd adhocapp.StatusUpdate, idemKey string) (*adhocapp.ProposalDTO, error) {
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
		return nil, fmt.Errorf("complete admin approval: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		cur, findErr := r.FindByID(ctx, upd.CompanyID, upd.ProposalID)
		if findErr != nil {
			return nil, findErr
		}
		if cur.Status == adhocapp.StatusApproved {
			return cur, nil
		}
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "proposal cannot be approved in current state", nil)
	}
	return r.FindByID(ctx, upd.CompanyID, upd.ProposalID)
}

func (r *Repository) List(ctx context.Context, companyID string, statusFilter []string, page, pageSize int) ([]adhocapp.ProposalDTO, int, error) {
	var whereExtra string
	var extraArgs []any
	if len(statusFilter) > 0 {
		placeholders := strings.Repeat("?,", len(statusFilter))
		placeholders = placeholders[:len(placeholders)-1]
		whereExtra = " AND status IN (" + placeholders + ")"
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

	offset := (page - 1) * pageSize
	listArgs := append(countArgs, pageSize, offset)
	rows, err := r.db.QueryContext(ctx, `
		SELECT proposal_id, company_id, type_id, status, proposed_workflow_json,
		       proposed_t0_date, proposed_deadline_date, change_note,
		       final_t0_date, final_deadline_date, adjustment_note,
		       focal_approved_by, focal_approved_at, admin_approved_by, admin_approved_at,
		       rejected_by, rejected_at, reject_reason,
		       record_id, workflow_instance_id, created_by, process_controller_id, created_at, updated_at
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
		p, err := scanProposalRow(rows)
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

func scanProposal(row *sql.Row) (*adhocapp.ProposalDTO, error) {
	return scanProposalRow(row)
}

func scanProposalRow(row rowScanner) (*adhocapp.ProposalDTO, error) {
	var p adhocapp.ProposalDTO
	var overridesRaw string
	var t0Date, dlDate, finalT0Date, finalDeadlineDate, focalBy, adminBy, rejectedBy sql.NullString
	var focalAt, adminAt, rejectedAt sql.NullTime
	var recordID, wfiID, processControllerID sql.NullString
	var changeNote, adjustmentNote, rejectReason sql.NullString

	if err := row.Scan(
		&p.ProposalID, &p.CompanyID, &p.TypeID, &p.Status, &overridesRaw,
		&t0Date, &dlDate, &changeNote,
		&finalT0Date, &finalDeadlineDate, &adjustmentNote,
		&focalBy, &focalAt, &adminBy, &adminAt,
		&rejectedBy, &rejectedAt, &rejectReason,
		&recordID, &wfiID, &p.CreatedBy, &processControllerID, &p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "proposal not found", nil)
		}
		return nil, fmt.Errorf("scan proposal: %w", err)
	}

	if overridesRaw != "" && overridesRaw != "null" {
		if err := json.Unmarshal([]byte(overridesRaw), &p.StepOverrides); err != nil {
			return nil, fmt.Errorf("unmarshal step_overrides: %w", err)
		}
	}
	if t0Date.Valid {
		p.ProposedT0Date = &t0Date.String
	}
	if dlDate.Valid {
		p.ProposedDeadlineDate = &dlDate.String
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

func scanProposalWithApprovalState(row rowScanner) (*adhocapp.ProposalDTO, string, string, string, error) {
	var p adhocapp.ProposalDTO
	var overridesRaw string
	var t0Date, dlDate, finalT0Date, finalDeadlineDate, focalBy, adminBy, rejectedBy sql.NullString
	var focalAt, adminAt, rejectedAt sql.NullTime
	var recordID, wfiID, processControllerID sql.NullString
	var changeNote, adjustmentNote, rejectReason sql.NullString
	var approvalKey, approvalRecordID, approvalWorkflowID sql.NullString

	if err := row.Scan(
		&p.ProposalID, &p.CompanyID, &p.TypeID, &p.Status, &overridesRaw,
		&t0Date, &dlDate, &changeNote,
		&finalT0Date, &finalDeadlineDate, &adjustmentNote,
		&focalBy, &focalAt, &adminBy, &adminAt,
		&rejectedBy, &rejectedAt, &rejectReason,
		&recordID, &wfiID, &p.CreatedBy, &processControllerID, &p.CreatedAt, &p.UpdatedAt,
		&approvalKey, &approvalRecordID, &approvalWorkflowID,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, "", "", "", perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "proposal not found", nil)
		}
		return nil, "", "", "", fmt.Errorf("scan proposal: %w", err)
	}
	if overridesRaw != "" && overridesRaw != "null" {
		if err := json.Unmarshal([]byte(overridesRaw), &p.StepOverrides); err != nil {
			return nil, "", "", "", fmt.Errorf("unmarshal step_overrides: %w", err)
		}
	}
	if t0Date.Valid {
		p.ProposedT0Date = &t0Date.String
	}
	if dlDate.Valid {
		p.ProposedDeadlineDate = &dlDate.String
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
	return &p, approvalKey.String, approvalRecordID.String, approvalWorkflowID.String, nil
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
