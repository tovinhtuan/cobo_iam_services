package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	wff "github.com/cobo/cobo_iam_services/internal/workflowfulfillment"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListActiveByRequirement(ctx context.Context, companyID, requirementSnapshotID string) ([]wff.FulfillmentFile, error) {
	companyID = strings.TrimSpace(companyID)
	requirementSnapshotID = strings.TrimSpace(requirementSnapshotID)
	if companyID == "" || requirementSnapshotID == "" {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, company_id, disclosure_record_id, workflow_instance_id, step_code,
		       requirement_snapshot_id, storage_key, original_file_name, mime_type, file_size,
		       uploaded_by, uploaded_at, lifecycle_status,
		       supersedes_file_id, superseded_by_file_id, deleted_at, deleted_by
		FROM workflow_step_document_fulfillment_files
		WHERE company_id = ? AND requirement_snapshot_id = ? AND lifecycle_status = ?
		ORDER BY uploaded_at ASC, id ASC
	`, companyID, requirementSnapshotID, wff.LifecycleActive)
	if err != nil {
		return nil, fmt.Errorf("list active fulfillment by requirement: %w", err)
	}
	defer rows.Close()
	return scanFiles(rows)
}

func (r *Repository) ListActiveByInstanceStep(ctx context.Context, companyID, workflowInstanceID, stepCode string) ([]wff.FulfillmentFile, error) {
	companyID = strings.TrimSpace(companyID)
	workflowInstanceID = strings.TrimSpace(workflowInstanceID)
	stepCode = strings.TrimSpace(stepCode)
	if companyID == "" || workflowInstanceID == "" || stepCode == "" {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, company_id, disclosure_record_id, workflow_instance_id, step_code,
		       requirement_snapshot_id, storage_key, original_file_name, mime_type, file_size,
		       uploaded_by, uploaded_at, lifecycle_status,
		       supersedes_file_id, superseded_by_file_id, deleted_at, deleted_by
		FROM workflow_step_document_fulfillment_files
		WHERE company_id = ? AND workflow_instance_id = ? AND step_code = ? AND lifecycle_status = ?
		ORDER BY uploaded_at ASC, id ASC
	`, companyID, workflowInstanceID, stepCode, wff.LifecycleActive)
	if err != nil {
		return nil, fmt.Errorf("list active fulfillment by step: %w", err)
	}
	defer rows.Close()
	return scanFiles(rows)
}

func (r *Repository) GetByID(ctx context.Context, companyID, fileID string) (*wff.FulfillmentFile, error) {
	return r.getByID(ctx, companyID, fileID, "")
}

func (r *Repository) GetActiveByID(ctx context.Context, companyID, fileID string) (*wff.FulfillmentFile, error) {
	return r.getByID(ctx, companyID, fileID, wff.LifecycleActive)
}

func (r *Repository) getByID(ctx context.Context, companyID, fileID, lifecycle string) (*wff.FulfillmentFile, error) {
	companyID = strings.TrimSpace(companyID)
	fileID = strings.TrimSpace(fileID)
	if companyID == "" || fileID == "" {
		return nil, nil
	}
	q := `
		SELECT id, company_id, disclosure_record_id, workflow_instance_id, step_code,
		       requirement_snapshot_id, storage_key, original_file_name, mime_type, file_size,
		       uploaded_by, uploaded_at, lifecycle_status,
		       supersedes_file_id, superseded_by_file_id, deleted_at, deleted_by
		FROM workflow_step_document_fulfillment_files
		WHERE company_id = ? AND id = ?`
	args := []any{companyID, fileID}
	if lifecycle != "" {
		q += ` AND lifecycle_status = ?`
		args = append(args, lifecycle)
	}
	row := r.db.QueryRowContext(ctx, q, args...)
	f, err := scanFile(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return f, err
}

func (r *Repository) CountActiveByRequirement(ctx context.Context, requirementSnapshotID string) (int, error) {
	requirementSnapshotID = strings.TrimSpace(requirementSnapshotID)
	if requirementSnapshotID == "" {
		return 0, nil
	}
	var n int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM workflow_step_document_fulfillment_files
		WHERE requirement_snapshot_id = ? AND lifecycle_status = ?
	`, requirementSnapshotID, wff.LifecycleActive).Scan(&n)
	return n, err
}

func (r *Repository) UploadInTx(ctx context.Context, in wff.UploadTxInput) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := lockRequirementSnapshot(ctx, tx, in.File.CompanyID, in.RequirementSnapshotID); err != nil {
		return err
	}
	if in.RequireNotCompleted {
		completed, err := lockStepCompleted(ctx, tx, in.File.CompanyID, in.WorkflowInstanceID, in.StepCode)
		if err != nil {
			return err
		}
		if completed {
			return wff.ErrStepCompleted{}
		}
	}
	var count int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM workflow_step_document_fulfillment_files
		WHERE requirement_snapshot_id = ? AND lifecycle_status = ?
	`, in.RequirementSnapshotID, wff.LifecycleActive).Scan(&count); err != nil {
		return err
	}
	maxActive := in.MaxActive
	if maxActive <= 0 {
		maxActive = wff.MaxActiveFilesPerRequirement
	}
	if count >= maxActive {
		return wff.ErrActiveLimitReached{}
	}
	if err := wff.ValidateInitialLifecycle(in.File.LifecycleStatus); err != nil {
		return err
	}
	if err := insertFile(ctx, tx, in.File); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) ReplaceInTx(ctx context.Context, in wff.ReplaceTxInput) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := lockRequirementSnapshot(ctx, tx, in.CompanyID, in.RequirementSnapshotID); err != nil {
		return err
	}
	if in.RequireNotCompleted {
		completed, err := lockStepCompleted(ctx, tx, in.CompanyID, in.WorkflowInstanceID, in.StepCode)
		if err != nil {
			return err
		}
		if completed {
			return wff.ErrStepCompleted{}
		}
	}

	old, err := lockActiveFile(ctx, tx, in.CompanyID, in.OldFileID)
	if err != nil {
		return err
	}
	if old == nil {
		return wff.ErrNotActive{Reason: "file not active"}
	}
	if old.RequirementSnapshotID != in.RequirementSnapshotID ||
		old.WorkflowInstanceID != in.WorkflowInstanceID ||
		old.StepCode != in.StepCode {
		return wff.ErrNotActive{Reason: "file context mismatch"}
	}
	if err := wff.ValidateTransition(old.LifecycleStatus, wff.LifecycleSuperseded); err != nil {
		return err
	}
	if err := wff.ValidateInitialLifecycle(in.NewFile.LifecycleStatus); err != nil {
		return err
	}

	in.NewFile.SupersedesFileID = old.ID
	if err := insertFile(ctx, tx, in.NewFile); err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, `
		UPDATE workflow_step_document_fulfillment_files
		SET lifecycle_status = ?, superseded_by_file_id = ?, updated_at = CURRENT_TIMESTAMP(3)
		WHERE id = ? AND company_id = ? AND lifecycle_status = ?
	`, wff.LifecycleSuperseded, in.NewFile.ID, old.ID, in.CompanyID, wff.LifecycleActive)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return wff.ErrNotActive{Reason: "replace lost race"}
	}
	return tx.Commit()
}

func (r *Repository) DeleteInTx(ctx context.Context, in wff.DeleteTxInput) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if in.RequireNotCompleted {
		completed, err := lockStepCompleted(ctx, tx, in.CompanyID, in.WorkflowInstanceID, in.StepCode)
		if err != nil {
			return err
		}
		if completed {
			return wff.ErrStepCompleted{}
		}
	}
	old, err := lockActiveFile(ctx, tx, in.CompanyID, in.FileID)
	if err != nil {
		return err
	}
	if old == nil {
		return wff.ErrNotActive{Reason: "file not active"}
	}
	if old.WorkflowInstanceID != in.WorkflowInstanceID || old.StepCode != in.StepCode {
		return wff.ErrNotActive{Reason: "file context mismatch"}
	}
	if err := wff.ValidateTransition(old.LifecycleStatus, wff.LifecycleDeleted); err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, `
		UPDATE workflow_step_document_fulfillment_files
		SET lifecycle_status = ?, deleted_at = ?, deleted_by = ?, updated_at = CURRENT_TIMESTAMP(3)
		WHERE id = ? AND company_id = ? AND lifecycle_status = ?
	`, wff.LifecycleDeleted, in.DeletedAt, in.DeletedBy, in.FileID, in.CompanyID, wff.LifecycleActive)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return wff.ErrNotActive{Reason: "delete lost race"}
	}
	return tx.Commit()
}

func lockRequirementSnapshot(ctx context.Context, tx *sql.Tx, companyID, snapshotID string) error {
	var id string
	err := tx.QueryRowContext(ctx, `
		SELECT id FROM workflow_step_document_requirement_snapshots
		WHERE id = ? AND company_id = ?
		FOR UPDATE
	`, snapshotID, companyID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("requirement snapshot not found")
	}
	return err
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
			// Concurrent insert — re-select FOR UPDATE.
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

func lockActiveFile(ctx context.Context, tx *sql.Tx, companyID, fileID string) (*wff.FulfillmentFile, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT id, company_id, disclosure_record_id, workflow_instance_id, step_code,
		       requirement_snapshot_id, storage_key, original_file_name, mime_type, file_size,
		       uploaded_by, uploaded_at, lifecycle_status,
		       supersedes_file_id, superseded_by_file_id, deleted_at, deleted_by
		FROM workflow_step_document_fulfillment_files
		WHERE company_id = ? AND id = ? AND lifecycle_status = ?
		FOR UPDATE
	`, companyID, fileID, wff.LifecycleActive)
	f, err := scanFile(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return f, err
}

func insertFile(ctx context.Context, tx *sql.Tx, f wff.FulfillmentFile) error {
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
		INSERT INTO workflow_step_document_fulfillment_files (
			id, company_id, disclosure_record_id, workflow_instance_id, step_code,
			requirement_snapshot_id, storage_key, original_file_name, mime_type, file_size,
			uploaded_by, uploaded_at, lifecycle_status,
			supersedes_file_id, superseded_by_file_id, deleted_at, deleted_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, f.ID, f.CompanyID, f.DisclosureRecordID, f.WorkflowInstanceID, f.StepCode,
		f.RequirementSnapshotID, f.StorageKey, f.OriginalFileName, f.MimeType, f.FileSize,
		f.UploadedBy, f.UploadedAt, f.LifecycleStatus,
		supersedes, supersededBy, deletedAt, deletedBy)
	if err != nil {
		return fmt.Errorf("insert fulfillment file: %w", err)
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanFile(row scanner) (*wff.FulfillmentFile, error) {
	var f wff.FulfillmentFile
	var supersedes, supersededBy, deletedBy sql.NullString
	var deletedAt sql.NullTime
	err := row.Scan(
		&f.ID, &f.CompanyID, &f.DisclosureRecordID, &f.WorkflowInstanceID, &f.StepCode,
		&f.RequirementSnapshotID, &f.StorageKey, &f.OriginalFileName, &f.MimeType, &f.FileSize,
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
	if deletedBy.Valid {
		f.DeletedBy = deletedBy.String
	}
	if deletedAt.Valid {
		t := deletedAt.Time.UTC()
		f.DeletedAt = &t
	}
	f.UploadedAt = f.UploadedAt.UTC()
	return &f, nil
}

func scanFiles(rows *sql.Rows) ([]wff.FulfillmentFile, error) {
	out := make([]wff.FulfillmentFile, 0)
	for rows.Next() {
		f, err := scanFile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *f)
	}
	return out, rows.Err()
}
