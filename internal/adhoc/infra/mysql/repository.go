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
			 proposed_deadline_date, change_note, created_by)
		VALUES (?, ?, ?, ?, CAST(? AS JSON), ?, ?, ?, ?)
	`, p.ProposalID, p.CompanyID, p.TypeID, p.Status, string(overridesJSON),
		nullableStr(p.ProposedT0Date), nullableStr(p.ProposedDeadlineDate),
		nullIfBlank(p.ChangeNote), p.CreatedBy)
	if err != nil {
		return nil, fmt.Errorf("insert ad_hoc_proposal: %w", err)
	}
	return r.FindByID(ctx, p.CompanyID, p.ProposalID)
}

func (r *Repository) FindByID(ctx context.Context, companyID, proposalID string) (*adhocapp.ProposalDTO, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT proposal_id, company_id, type_id, status, proposed_workflow_json,
		       proposed_t0_date, proposed_deadline_date, change_note,
		       focal_approved_by, focal_approved_at, admin_approved_by, admin_approved_at,
		       rejected_by, rejected_at, reject_reason,
		       record_id, workflow_instance_id, created_by, created_at, updated_at
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
		     record_id = NULLIF(?, ''), workflow_instance_id = NULLIF(?, ''), updated_at = ?
		     WHERE proposal_id = ? AND company_id = ?`
		args = []any{upd.Status, upd.ActorMembershipID, now, upd.RecordID, upd.WorkflowInstanceID, now, upd.ProposalID, upd.CompanyID}
	default:
		return nil, fmt.Errorf("unknown status transition: %s", upd.Status)
	}

	// Handle pending_focal → set focal_approved_by
	if upd.Status == adhocapp.StatusPendingAdminApproval {
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
		       focal_approved_by, focal_approved_at, admin_approved_by, admin_approved_at,
		       rejected_by, rejected_at, reject_reason,
		       record_id, workflow_instance_id, created_by, created_at, updated_at
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
	var t0Date, dlDate, focalBy, adminBy, rejectedBy sql.NullString
	var focalAt, adminAt, rejectedAt sql.NullTime
	var recordID, wfiID sql.NullString
	var changeNote, rejectReason sql.NullString

	if err := row.Scan(
		&p.ProposalID, &p.CompanyID, &p.TypeID, &p.Status, &overridesRaw,
		&t0Date, &dlDate, &changeNote,
		&focalBy, &focalAt, &adminBy, &adminAt,
		&rejectedBy, &rejectedAt, &rejectReason,
		&recordID, &wfiID, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt,
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
	return &p, nil
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
