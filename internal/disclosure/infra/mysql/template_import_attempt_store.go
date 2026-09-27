package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"net/http"
)

func (r *Repository) CreateImportAttempt(ctx context.Context, row *disclosureapp.TemplateImportAttempt) error {
	errorsJSON, err := json.Marshal(row.ErrorCodes)
	if err != nil {
		return err
	}
	mappingsJSON, err := json.Marshal(row.MappingSummary)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO cms_template_import_attempts (
			id, company_id, actor_user_id, actor_membership_id, filename, file_size_bytes,
			file_sha256, canonical_payload_sha256, validation_token_sha256, schema_version, status,
			parse_valid, domain_valid, activation_ready, can_confirm, mapping_required,
			required_mapping_count, resolved_mapping_count, unresolved_mapping_count,
			error_codes, mapping_summary, target_type_id, created_type_id, confirm_error_code,
			row_version, created_at, validated_at, confirmed_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?, ?, ?)`,
		row.ID, row.CompanyID, row.ActorUserID, importAttemptNullString(row.ActorMembershipID), row.Filename, row.FileSizeBytes,
		row.FileSHA256, row.CanonicalPayloadSHA256, row.ValidationTokenSHA256, row.SchemaVersion, row.Status,
		boolTiny(row.ParseValid), boolTiny(row.DomainValid), boolTiny(row.ActivationReady), boolTiny(row.CanConfirm), boolTiny(row.MappingRequired),
		row.RequiredMappingCount, row.ResolvedMappingCount, row.UnresolvedMappingCount,
		errorsJSON, mappingsJSON, row.TargetTypeID, row.CreatedTypeID, row.ConfirmErrorCode,
		row.RowVersion, row.CreatedAt, row.ValidatedAt, nullTime(row.ConfirmedAt), row.UpdatedAt,
	)
	return err
}

func (r *Repository) GetImportAttemptByID(ctx context.Context, id string) (*disclosureapp.TemplateImportAttempt, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, company_id, actor_user_id, COALESCE(actor_membership_id, ''), filename, file_size_bytes,
			file_sha256, COALESCE(canonical_payload_sha256, ''), COALESCE(validation_token_sha256, ''), COALESCE(schema_version, ''), status,
			parse_valid, domain_valid, activation_ready, can_confirm, mapping_required,
			required_mapping_count, resolved_mapping_count, unresolved_mapping_count,
			error_codes, mapping_summary, COALESCE(target_type_id, ''), COALESCE(created_type_id, ''), COALESCE(confirm_error_code, ''),
			row_version, created_at, validated_at, confirmed_at, updated_at
		FROM cms_template_import_attempts WHERE id = ?`, id)
	return scanImportAttempt(row)
}

func (r *Repository) ClaimImportAttempt(ctx context.Context, id, companyID, actorUserID string, rowVersion int64, cutoff time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE cms_template_import_attempts
		SET row_version = row_version + 1, status = 'CONFIRMING', updated_at = CURRENT_TIMESTAMP(3)
		WHERE id = ? AND company_id = ? AND actor_user_id = ? AND row_version = ?
		  AND status IN ('VALIDATED', 'MAPPING_REQUIRED') AND created_at >= ?`,
		id, companyID, actorUserID, rowVersion, cutoff)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, nil
	}
	return rowVersion + 1, nil
}

func (r *Repository) MarkImportAttemptConfirmed(ctx context.Context, id string, rowVersion int64, createdTypeID, targetTypeID string, at time.Time) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE cms_template_import_attempts
		SET status = 'CONFIRMED', created_type_id = ?, target_type_id = ?, confirmed_at = ?, updated_at = ?
		WHERE id = ? AND row_version = ? AND status = 'CONFIRMING'`,
		createdTypeID, targetTypeID, at, at, id, rowVersion)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return perr.NewHTTPError(http.StatusConflict, perr.CodeImportAttemptAlreadyConfirmed, "import attempt version mismatch", nil)
	}
	return nil
}

func (r *Repository) MarkImportAttemptFailed(ctx context.Context, id string, rowVersion int64, errorCode, targetTypeID string, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE cms_template_import_attempts
		SET status = 'CONFIRM_FAILED', confirm_error_code = ?, target_type_id = NULLIF(?, ''), confirmed_at = ?, updated_at = ?
		WHERE id = ? AND row_version = ? AND status = 'CONFIRMING'`,
		errorCode, targetTypeID, at, at, id, rowVersion)
	return err
}

func (r *Repository) ListImportAttempts(ctx context.Context, q disclosureapp.ListTemplateImportHistoryRequest) ([]disclosureapp.TemplateImportAttempt, error) {
	cutoff := time.Now().UTC().Add(-time.Duration(disclosureapp.TemplateImportRetentionDays) * 24 * time.Hour)
	cursorTime, cursorID, err := disclosureapp.DecodeImportHistoryCursor(q.Cursor)
	if err != nil {
		return nil, err
	}
	args := []any{q.Subject.CompanyID, cutoff}
	where := `WHERE company_id = ? AND created_at >= ?`
	if q.Status != "" {
		where += ` AND status = ?`
		args = append(args, q.Status)
	}
	if q.Filename != "" {
		where += ` AND filename LIKE ?`
		args = append(args, "%"+strings.ReplaceAll(q.Filename, "%", "")+"%")
	}
	if q.FileSHA256 != "" {
		where += ` AND file_sha256 = ?`
		args = append(args, q.FileSHA256)
	}
	if !q.From.IsZero() {
		where += ` AND created_at >= ?`
		args = append(args, q.From.UTC())
	}
	if !q.To.IsZero() {
		where += ` AND created_at <= ?`
		args = append(args, q.To.UTC())
	}
	if !cursorTime.IsZero() {
		where += ` AND (created_at < ? OR (created_at = ? AND id < ?))`
		args = append(args, cursorTime.UTC(), cursorTime.UTC(), cursorID)
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 20
	}
	args = append(args, limit+1)
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, company_id, actor_user_id, COALESCE(actor_membership_id, ''), filename, file_size_bytes,
			file_sha256, COALESCE(canonical_payload_sha256, ''), COALESCE(validation_token_sha256, ''), COALESCE(schema_version, ''), status,
			parse_valid, domain_valid, activation_ready, can_confirm, mapping_required,
			required_mapping_count, resolved_mapping_count, unresolved_mapping_count,
			error_codes, mapping_summary, COALESCE(target_type_id, ''), COALESCE(created_type_id, ''), COALESCE(confirm_error_code, ''),
			row_version, created_at, validated_at, confirmed_at, updated_at
		FROM cms_template_import_attempts `+where+` ORDER BY created_at DESC, id DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]disclosureapp.TemplateImportAttempt, 0)
	for rows.Next() {
		item, err := scanImportAttempt(rows)
		if err != nil {
			return nil, err
		}
		if item != nil {
			out = append(out, *item)
		}
	}
	return out, rows.Err()
}

func scanImportAttempt(row rowScanner) (*disclosureapp.TemplateImportAttempt, error) {
	var item disclosureapp.TemplateImportAttempt
	var parseValid, domainValid, activationReady, canConfirm, mappingRequired int
	var errorsJSON, mappingsJSON []byte
	var confirmed sql.NullTime
	err := row.Scan(
		&item.ID, &item.CompanyID, &item.ActorUserID, &item.ActorMembershipID, &item.Filename, &item.FileSizeBytes,
		&item.FileSHA256, &item.CanonicalPayloadSHA256, &item.ValidationTokenSHA256, &item.SchemaVersion, &item.Status,
		&parseValid, &domainValid, &activationReady, &canConfirm, &mappingRequired,
		&item.RequiredMappingCount, &item.ResolvedMappingCount, &item.UnresolvedMappingCount,
		&errorsJSON, &mappingsJSON, &item.TargetTypeID, &item.CreatedTypeID, &item.ConfirmErrorCode,
		&item.RowVersion, &item.CreatedAt, &item.ValidatedAt, &confirmed, &item.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	item.ParseValid = parseValid == 1
	item.DomainValid = domainValid == 1
	item.ActivationReady = activationReady == 1
	item.CanConfirm = canConfirm == 1
	item.MappingRequired = mappingRequired == 1
	if confirmed.Valid {
		item.ConfirmedAt = confirmed.Time
	}
	_ = json.Unmarshal(errorsJSON, &item.ErrorCodes)
	_ = json.Unmarshal(mappingsJSON, &item.MappingSummary)
	return &item, nil
}

func boolTiny(v bool) int {
	if v {
		return 1
	}
	return 0
}

func importAttemptNullString(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}

func nullTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}
