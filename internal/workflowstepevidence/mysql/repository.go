package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	wse "github.com/cobo/cobo_iam_services/internal/workflowstepevidence"
)

// Repository persists workflow_step_evidence_files with step_state locking.
type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListActiveByStep(ctx context.Context, owner wse.OwnerContext) ([]wse.EvidenceFile, error) {
	owner = trimOwner(owner)
	if owner.CompanyID == "" || owner.WorkflowInstanceID == "" || owner.StepCode == "" {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, company_id, disclosure_record_id, workflow_instance_id, step_code,
		       storage_key, original_file_name, mime_type, file_size,
		       uploaded_by, uploaded_at, lifecycle_status,
		       supersedes_file_id, superseded_by_file_id, deleted_at, deleted_by
		FROM workflow_step_evidence_files
		WHERE company_id = ?
		  AND disclosure_record_id = ?
		  AND workflow_instance_id = ?
		  AND step_code = ?
		  AND lifecycle_status = ?
		ORDER BY uploaded_at ASC, id ASC
	`, owner.CompanyID, owner.DisclosureRecordID, owner.WorkflowInstanceID, owner.StepCode, wse.LifecycleActive)
	if err != nil {
		return nil, fmt.Errorf("list active evidence by step: %w", err)
	}
	defer rows.Close()
	return scanFiles(rows)
}

func (r *Repository) CountActiveByStep(ctx context.Context, owner wse.OwnerContext) (int, error) {
	owner = trimOwner(owner)
	var n int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM workflow_step_evidence_files
		WHERE company_id = ?
		  AND disclosure_record_id = ?
		  AND workflow_instance_id = ?
		  AND step_code = ?
		  AND lifecycle_status = ?
	`, owner.CompanyID, owner.DisclosureRecordID, owner.WorkflowInstanceID, owner.StepCode, wse.LifecycleActive).Scan(&n)
	return n, err
}

func (r *Repository) GetByIDInContext(ctx context.Context, owner wse.OwnerContext, fileID string) (*wse.EvidenceFile, error) {
	owner = trimOwner(owner)
	fileID = strings.TrimSpace(fileID)
	row := r.db.QueryRowContext(ctx, `
		SELECT id, company_id, disclosure_record_id, workflow_instance_id, step_code,
		       storage_key, original_file_name, mime_type, file_size,
		       uploaded_by, uploaded_at, lifecycle_status,
		       supersedes_file_id, superseded_by_file_id, deleted_at, deleted_by
		FROM workflow_step_evidence_files
		WHERE company_id = ?
		  AND disclosure_record_id = ?
		  AND workflow_instance_id = ?
		  AND step_code = ?
		  AND id = ?
	`, owner.CompanyID, owner.DisclosureRecordID, owner.WorkflowInstanceID, owner.StepCode, fileID)
	f, err := scanFile(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return f, err
}

func (r *Repository) GetActiveByIDInContext(ctx context.Context, owner wse.OwnerContext, fileID string) (*wse.EvidenceFile, error) {
	f, err := r.GetByIDInContext(ctx, owner, fileID)
	if err != nil || f == nil {
		return f, err
	}
	if f.LifecycleStatus != wse.LifecycleActive {
		return nil, nil
	}
	return f, nil
}

func (r *Repository) CreateActiveInTx(ctx context.Context, in wse.CreateTxInput) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	owner := trimOwner(in.Owner)
	if in.RequireNotCompleted {
		completed, err := lockStepCompleted(ctx, tx, owner.CompanyID, owner.WorkflowInstanceID, owner.StepCode)
		if err != nil {
			return err
		}
		if completed {
			return wse.ErrStepCompleted{}
		}
	}
	if !in.File.MatchesOwner(owner) {
		return wse.ErrNotActive{Reason: "file owner mismatch"}
	}
	var count int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM workflow_step_evidence_files
		WHERE company_id = ?
		  AND disclosure_record_id = ?
		  AND workflow_instance_id = ?
		  AND step_code = ?
		  AND lifecycle_status = ?
	`, owner.CompanyID, owner.DisclosureRecordID, owner.WorkflowInstanceID, owner.StepCode, wse.LifecycleActive).Scan(&count); err != nil {
		return err
	}
	maxActive := in.MaxActive
	if maxActive <= 0 {
		maxActive = wse.MaxActiveFilesPerStep
	}
	if count >= maxActive {
		return wse.ErrActiveLimitReached{}
	}
	if err := wse.ValidateInitialLifecycle(in.File.LifecycleStatus); err != nil {
		return err
	}
	if err := insertFile(ctx, tx, in.File); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) ReplaceInTx(ctx context.Context, in wse.ReplaceTxInput) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	owner := trimOwner(in.Owner)
	if in.RequireNotCompleted {
		completed, err := lockStepCompleted(ctx, tx, owner.CompanyID, owner.WorkflowInstanceID, owner.StepCode)
		if err != nil {
			return err
		}
		if completed {
			return wse.ErrStepCompleted{}
		}
	}
	old, err := lockActiveFileInContext(ctx, tx, owner, in.OldFileID)
	if err != nil {
		return err
	}
	if old == nil {
		return wse.ErrNotActive{Reason: "file not active"}
	}
	if err := wse.ValidateTransition(old.LifecycleStatus, wse.LifecycleSuperseded); err != nil {
		return err
	}
	if err := wse.ValidateInitialLifecycle(in.NewFile.LifecycleStatus); err != nil {
		return err
	}
	if !in.NewFile.MatchesOwner(owner) {
		return wse.ErrNotActive{Reason: "new file owner mismatch"}
	}
	// REPLACE_AT_ACTIVE_LIMIT_ALLOWED: no count gate (net ACTIVE unchanged).

	in.NewFile.SupersedesFileID = old.ID
	if err := insertFile(ctx, tx, in.NewFile); err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, `
		UPDATE workflow_step_evidence_files
		SET lifecycle_status = ?, superseded_by_file_id = ?, updated_at = CURRENT_TIMESTAMP(3)
		WHERE id = ?
		  AND company_id = ?
		  AND disclosure_record_id = ?
		  AND workflow_instance_id = ?
		  AND step_code = ?
		  AND lifecycle_status = ?
	`, wse.LifecycleSuperseded, in.NewFile.ID, old.ID,
		owner.CompanyID, owner.DisclosureRecordID, owner.WorkflowInstanceID, owner.StepCode,
		wse.LifecycleActive)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return wse.ErrNotActive{Reason: "replace lost race"}
	}
	return tx.Commit()
}

func (r *Repository) DeleteInTx(ctx context.Context, in wse.DeleteTxInput) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	owner := trimOwner(in.Owner)
	if in.RequireNotCompleted {
		completed, err := lockStepCompleted(ctx, tx, owner.CompanyID, owner.WorkflowInstanceID, owner.StepCode)
		if err != nil {
			return err
		}
		if completed {
			return wse.ErrStepCompleted{}
		}
	}
	old, err := lockActiveFileInContext(ctx, tx, owner, in.FileID)
	if err != nil {
		return err
	}
	if old == nil {
		return wse.ErrNotActive{Reason: "file not active"}
	}
	if err := wse.ValidateTransition(old.LifecycleStatus, wse.LifecycleDeleted); err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, `
		UPDATE workflow_step_evidence_files
		SET lifecycle_status = ?, deleted_at = ?, deleted_by = ?, updated_at = CURRENT_TIMESTAMP(3)
		WHERE id = ?
		  AND company_id = ?
		  AND disclosure_record_id = ?
		  AND workflow_instance_id = ?
		  AND step_code = ?
		  AND lifecycle_status = ?
	`, wse.LifecycleDeleted, in.DeletedAt, in.DeletedBy, in.FileID,
		owner.CompanyID, owner.DisclosureRecordID, owner.WorkflowInstanceID, owner.StepCode,
		wse.LifecycleActive)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return wse.ErrNotActive{Reason: "delete lost race"}
	}
	return tx.Commit()
}

func lockStepCompleted(ctx context.Context, tx *sql.Tx, companyID, workflowInstanceID, stepCode string) (bool, error) {
	var completedAt sql.NullTime
	err := tx.QueryRowContext(ctx, `
		SELECT completed_at FROM workflow_instance_step_states
		WHERE workflow_instance_id = ? AND step_code = ?
		FOR UPDATE
	`, workflowInstanceID, stepCode).Scan(&completedAt)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO workflow_instance_step_states (
				company_id, workflow_instance_id, step_code
			) VALUES (?, ?, ?)
		`, companyID, workflowInstanceID, stepCode)
		if err != nil {
			err = tx.QueryRowContext(ctx, `
				SELECT completed_at FROM workflow_instance_step_states
				WHERE workflow_instance_id = ? AND step_code = ?
				FOR UPDATE
			`, workflowInstanceID, stepCode).Scan(&completedAt)
			if err != nil {
				return false, err
			}
			return completedAt.Valid, nil
		}
		err = tx.QueryRowContext(ctx, `
			SELECT completed_at FROM workflow_instance_step_states
			WHERE workflow_instance_id = ? AND step_code = ?
			FOR UPDATE
		`, workflowInstanceID, stepCode).Scan(&completedAt)
		if err != nil {
			return false, err
		}
	} else if err != nil {
		return false, err
	}
	return completedAt.Valid, nil
}

func lockActiveFileInContext(ctx context.Context, tx *sql.Tx, owner wse.OwnerContext, fileID string) (*wse.EvidenceFile, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT id, company_id, disclosure_record_id, workflow_instance_id, step_code,
		       storage_key, original_file_name, mime_type, file_size,
		       uploaded_by, uploaded_at, lifecycle_status,
		       supersedes_file_id, superseded_by_file_id, deleted_at, deleted_by
		FROM workflow_step_evidence_files
		WHERE company_id = ?
		  AND disclosure_record_id = ?
		  AND workflow_instance_id = ?
		  AND step_code = ?
		  AND id = ?
		  AND lifecycle_status = ?
		FOR UPDATE
	`, owner.CompanyID, owner.DisclosureRecordID, owner.WorkflowInstanceID, owner.StepCode, fileID, wse.LifecycleActive)
	f, err := scanFile(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return f, err
}

func insertFile(ctx context.Context, tx *sql.Tx, f wse.EvidenceFile) error {
	var supersedes, supersededBy, deletedBy any
	if strings.TrimSpace(f.SupersedesFileID) != "" {
		supersedes = f.SupersedesFileID
	}
	if strings.TrimSpace(f.SupersededByFileID) != "" {
		supersededBy = f.SupersededByFileID
	}
	if strings.TrimSpace(f.DeletedBy) != "" {
		deletedBy = f.DeletedBy
	}
	var deletedAt any
	if f.DeletedAt != nil {
		deletedAt = *f.DeletedAt
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO workflow_step_evidence_files (
			id, company_id, disclosure_record_id, workflow_instance_id, step_code,
			storage_key, original_file_name, mime_type, file_size,
			uploaded_by, uploaded_at, lifecycle_status,
			supersedes_file_id, superseded_by_file_id, deleted_at, deleted_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, f.ID, f.CompanyID, f.DisclosureRecordID, f.WorkflowInstanceID, f.StepCode,
		f.StorageKey, f.OriginalFileName, f.MimeType, f.FileSize,
		f.UploadedBy, f.UploadedAt, f.LifecycleStatus,
		supersedes, supersededBy, deletedAt, deletedBy)
	return err
}

type scannable interface {
	Scan(dest ...any) error
}

func scanFile(row scannable) (*wse.EvidenceFile, error) {
	var f wse.EvidenceFile
	var supersedes, supersededBy, deletedBy sql.NullString
	var deletedAt sql.NullTime
	err := row.Scan(
		&f.ID, &f.CompanyID, &f.DisclosureRecordID, &f.WorkflowInstanceID, &f.StepCode,
		&f.StorageKey, &f.OriginalFileName, &f.MimeType, &f.FileSize,
		&f.UploadedBy, &f.UploadedAt, &f.LifecycleStatus,
		&supersedes, &supersededBy, &deletedAt, &deletedBy,
	)
	if err != nil {
		return nil, err
	}
	if supersedes.Valid {
		f.SupersedesFileID = supersedes.String
	}
	if supersededBy.Valid {
		f.SupersededByFileID = supersededBy.String
	}
	if deletedAt.Valid {
		t := deletedAt.Time
		f.DeletedAt = &t
	}
	if deletedBy.Valid {
		f.DeletedBy = deletedBy.String
	}
	return &f, nil
}

func scanFiles(rows *sql.Rows) ([]wse.EvidenceFile, error) {
	out := make([]wse.EvidenceFile, 0)
	for rows.Next() {
		f, err := scanFile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *f)
	}
	return out, rows.Err()
}

func trimOwner(o wse.OwnerContext) wse.OwnerContext {
	return wse.OwnerContext{
		CompanyID:          strings.TrimSpace(o.CompanyID),
		DisclosureRecordID: strings.TrimSpace(o.DisclosureRecordID),
		WorkflowInstanceID: strings.TrimSpace(o.WorkflowInstanceID),
		StepCode:           strings.TrimSpace(o.StepCode),
	}
}
