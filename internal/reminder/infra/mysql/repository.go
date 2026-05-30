package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	reminderapp "github.com/cobo/cobo_iam_services/internal/reminder/app"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) UpsertByScope(ctx context.Context, in reminderapp.ReminderConfigDTO) (*reminderapp.ReminderConfigDTO, error) {
	daysBeforeJSON, err := json.Marshal(in.Config.DaysBefore)
	if err != nil {
		return nil, fmt.Errorf("marshal daysBefore: %w", err)
	}
	specificDatesJSON, err := json.Marshal(in.Config.SpecificDates)
	if err != nil {
		return nil, fmt.Errorf("marshal specificDates: %w", err)
	}
	departmentsJSON, err := json.Marshal(in.Config.Departments)
	if err != nil {
		return nil, fmt.Errorf("marshal departments: %w", err)
	}
	recipientsJSON, err := json.Marshal(in.Config.Recipients)
	if err != nil {
		return nil, fmt.Errorf("marshal recipients: %w", err)
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO reminder_configs (
			disclosure_id, scope_type, scope_id, enabled, mode, days_before_json, specific_dates_json,
			recipient_type, departments_json, recipients_json, version, updated_by
		) VALUES (SUBSTRING_INDEX(?, ':', 1), ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?)
		ON DUPLICATE KEY UPDATE
			disclosure_id = VALUES(disclosure_id),
			enabled = VALUES(enabled),
			mode = VALUES(mode),
			days_before_json = VALUES(days_before_json),
			specific_dates_json = VALUES(specific_dates_json),
			recipient_type = VALUES(recipient_type),
			departments_json = VALUES(departments_json),
			recipients_json = VALUES(recipients_json),
			version = version + 1,
			updated_by = VALUES(updated_by)
	`, in.ScopeID, in.ScopeType, in.ScopeID, in.Config.Enabled, in.Config.Mode, daysBeforeJSON, specificDatesJSON,
		in.Config.RecipientType, departmentsJSON, recipientsJSON, in.UpdatedBy)
	if err != nil {
		return nil, fmt.Errorf("upsert reminder config: %w", err)
	}
	return r.GetByScope(ctx, in.ScopeType, in.ScopeID)
}

func (r *Repository) GetByScope(ctx context.Context, scopeType reminderapp.ScopeType, scopeID string) (*reminderapp.ReminderConfigDTO, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT scope_type, scope_id, enabled, mode, days_before_json, specific_dates_json,
		       recipient_type, departments_json, recipients_json, version, updated_at, updated_by
		FROM reminder_configs
		WHERE scope_type = ? AND scope_id = ?
	`, scopeType, scopeID)

	var out reminderapp.ReminderConfigDTO
	var enabled bool
	var mode string
	var recipientType string
	var daysBeforeJSON, specificDatesJSON, departmentsJSON, recipientsJSON []byte
	if err := row.Scan(
		&out.ScopeType, &out.ScopeID, &enabled, &mode, &daysBeforeJSON, &specificDatesJSON,
		&recipientType, &departmentsJSON, &recipientsJSON, &out.Version, &out.UpdatedAt, &out.UpdatedBy,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "reminder config not found", nil)
		}
		return nil, fmt.Errorf("get reminder config: %w", err)
	}
	out.Config.Enabled = enabled
	out.Config.Mode = reminderapp.ReminderMode(mode)
	out.Config.RecipientType = reminderapp.ReminderRecipientType(recipientType)
	_ = json.Unmarshal(daysBeforeJSON, &out.Config.DaysBefore)
	_ = json.Unmarshal(specificDatesJSON, &out.Config.SpecificDates)
	_ = json.Unmarshal(departmentsJSON, &out.Config.Departments)
	_ = json.Unmarshal(recipientsJSON, &out.Config.Recipients)
	return &out, nil
}

func (r *Repository) ListHistoryByDisclosure(ctx context.Context, disclosureID string, q reminderapp.HistoryQuery) (*reminderapp.ReminderHistoryPage, error) {
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 20
	}
	offset := (q.Page - 1) * q.PageSize

	whereParts := []string{"disclosure_id = ?"}
	args := []any{disclosureID}
	if strings.TrimSpace(q.Scope) == "disclosure" {
		whereParts = append(whereParts, "scope_type = 'DISCLOSURE'")
	} else if strings.TrimSpace(q.Scope) == "step" {
		whereParts = append(whereParts, "scope_type = 'WORKFLOW_STEP'")
	}
	if q.Status != "" {
		whereParts = append(whereParts, "status = ?")
		args = append(args, q.Status)
	}
	if q.From != nil {
		whereParts = append(whereParts, "scheduled_at >= ?")
		args = append(args, q.From.UTC())
	}
	if q.To != nil {
		whereParts = append(whereParts, "scheduled_at <= ?")
		args = append(args, q.To.UTC())
	}
	whereClause := strings.Join(whereParts, " AND ")

	var total int
	countQuery := "SELECT COUNT(1) FROM reminder_occurrences WHERE " + whereClause
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("count reminder history: %w", err)
	}

	listArgs := append(append([]any{}, args...), q.PageSize, offset)
	rows, err := r.db.QueryContext(ctx, `
		SELECT occurrence_id, disclosure_id, scope_type, scope_id, scheduled_at, status,
		       attempt_count, idempotency_key, last_attempt_at, last_error_code, provider_message_id
		FROM reminder_occurrences
		WHERE `+whereClause+`
		ORDER BY scheduled_at DESC
		LIMIT ? OFFSET ?
	`, listArgs...)
	if err != nil {
		return nil, fmt.Errorf("list reminder history: %w", err)
	}
	defer rows.Close()

	items := make([]reminderapp.ReminderOccurrenceDTO, 0)
	for rows.Next() {
		var it reminderapp.ReminderOccurrenceDTO
		var scopeType, status string
		var lastAttemptAt sql.NullTime
		var lastErrorCode, providerMessageID sql.NullString
		if err := rows.Scan(
			&it.OccurrenceID, &it.DisclosureID, &scopeType, &it.ScopeID, &it.ScheduledAt, &status,
			&it.AttemptCount, &it.IdempotencyKey, &lastAttemptAt, &lastErrorCode, &providerMessageID,
		); err != nil {
			return nil, fmt.Errorf("scan reminder history item: %w", err)
		}
		it.ScopeType = reminderapp.ScopeType(scopeType)
		it.Status = reminderapp.ReminderStatus(status)
		if lastAttemptAt.Valid {
			t := lastAttemptAt.Time.UTC()
			it.LastAttemptAt = &t
		}
		if lastErrorCode.Valid {
			it.LastErrorCode = lastErrorCode.String
		}
		if providerMessageID.Valid {
			it.ProviderMessageID = providerMessageID.String
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate reminder history rows: %w", err)
	}
	return &reminderapp.ReminderHistoryPage{
		Items:    items,
		Page:     q.Page,
		PageSize: q.PageSize,
		Total:    total,
	}, nil
}

func (r *Repository) ClaimForDispatch(ctx context.Context, occurrenceID string) (*reminderapp.ReminderOccurrenceDTO, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx claim occurrence: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx, `
		SELECT occurrence_id, disclosure_id, scope_type, scope_id, scheduled_at, status,
		       attempt_count, idempotency_key, last_attempt_at, last_error_code, provider_message_id
		FROM reminder_occurrences
		WHERE occurrence_id = ?
		FOR UPDATE
	`, occurrenceID)
	occ, err := scanOccurrenceRow(row)
	if err != nil {
		return nil, err
	}
	if occ.Status != reminderapp.ReminderStatusPending && occ.Status != reminderapp.ReminderStatusRetryScheduled {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "occurrence is not dispatchable", nil)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE reminder_occurrences
		SET status = ?, updated_at = CURRENT_TIMESTAMP(3)
		WHERE occurrence_id = ?
	`, reminderapp.ReminderStatusDispatching, occurrenceID); err != nil {
		return nil, fmt.Errorf("set occurrence dispatching: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit claim occurrence: %w", err)
	}
	occ.Status = reminderapp.ReminderStatusDispatching
	return occ, nil
}

func (r *Repository) UpdateDispatchResult(ctx context.Context, in reminderapp.DispatchResultInput) error {
	inc := 0
	if in.IncrementAttempt {
		inc = 1
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE reminder_occurrences
		SET status = ?,
		    attempt_count = attempt_count + ?,
		    next_retry_at = ?,
		    last_attempt_at = CASE WHEN ? = 1 THEN CURRENT_TIMESTAMP(3) ELSE last_attempt_at END,
		    last_error_code = ?,
		    last_error_message = ?,
		    provider_message_id = ?,
		    updated_at = CURRENT_TIMESTAMP(3)
		WHERE occurrence_id = ?
	`, in.Status, inc, nullableTime(in.NextRetryAt), inc, nullableString(in.LastErrorCode), nullableString(in.LastErrorMessage), nullableString(in.ProviderMessageID), in.OccurrenceID)
	if err != nil {
		return fmt.Errorf("update occurrence dispatch result: %w", err)
	}
	return nil
}

func (r *Repository) InsertAttempt(ctx context.Context, in reminderapp.ReminderDeliveryAttemptDTO) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO reminder_delivery_attempts (
			occurrence_id, attempt_no, status, provider_message_id, error_code, error_message, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`, in.OccurrenceID, in.AttemptNo, in.Status, nullableString(in.ProviderMessageID), nullableString(in.ErrorCode), nullableString(in.ErrorMessage), in.CreatedAt.UTC())
	if err != nil {
		return fmt.Errorf("insert reminder delivery attempt: %w", err)
	}
	return nil
}

func (r *Repository) SeedOccurrence(ctx context.Context, in reminderapp.ReminderOccurrenceDTO) (*reminderapp.ReminderOccurrenceDTO, error) {
	if in.OccurrenceID == "" {
		in.OccurrenceID = in.IdempotencyKey
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO reminder_occurrences (
			occurrence_id, disclosure_id, scope_type, scope_id, scheduled_at, status, attempt_count,
			idempotency_key, next_retry_at, last_attempt_at, last_error_code, last_error_message, provider_message_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			disclosure_id = VALUES(disclosure_id),
			scope_type = VALUES(scope_type),
			scope_id = VALUES(scope_id),
			scheduled_at = VALUES(scheduled_at),
			status = VALUES(status),
			idempotency_key = VALUES(idempotency_key),
			updated_at = CURRENT_TIMESTAMP(3)
	`, in.OccurrenceID, in.DisclosureID, in.ScopeType, in.ScopeID, in.ScheduledAt.UTC(), in.Status, in.AttemptCount, in.IdempotencyKey,
		nil, nullableTime(in.LastAttemptAt), nullableString(in.LastErrorCode), nil, nullableString(in.ProviderMessageID))
	if err != nil {
		return nil, fmt.Errorf("seed reminder occurrence: %w", err)
	}

	row := r.db.QueryRowContext(ctx, `
		SELECT occurrence_id, disclosure_id, scope_type, scope_id, scheduled_at, status,
		       attempt_count, idempotency_key, last_attempt_at, last_error_code, provider_message_id
		FROM reminder_occurrences
		WHERE occurrence_id = ?
	`, in.OccurrenceID)
	return scanOccurrenceRow(row)
}

func (r *Repository) MaterializeDueOccurrences(ctx context.Context, now time.Time) (int, error) {
	now = now.UTC()
	// Global timezone + deadline cutoff:
	// - DaysBefore uses disclosure planned_date minus N days.
	// - SpecificDate must be <= disclosure planned_date.
	// Merge by deterministic idempotency key and INSERT IGNORE.
	result, err := r.db.ExecContext(ctx, `
		INSERT IGNORE INTO reminder_occurrences (
			occurrence_id, disclosure_id, scope_type, scope_id, scheduled_at, status, attempt_count, idempotency_key
		)
		SELECT
			SHA2(CONCAT_WS('|',
				'REM_OCC',
				src.disclosure_id,
				src.scope_type,
				src.scope_id,
				DATE_FORMAT(src.due_utc, '%Y-%m-%dT%H:%i:%sZ'),
				src.recipient_hash
			), 256) AS occurrence_id,
			src.disclosure_id,
			src.scope_type,
			src.scope_id,
			src.due_utc,
			'PENDING',
			0,
			SHA2(CONCAT_WS('|',
				'REM_OCC',
				src.disclosure_id,
				src.scope_type,
				src.scope_id,
				DATE_FORMAT(src.due_utc, '%Y-%m-%dT%H:%i:%sZ'),
				src.recipient_hash
			), 256) AS idempotency_key
		FROM (
			SELECT
				c.disclosure_id,
				c.scope_type,
				c.scope_id,
				SHA2(CONCAT_WS('|',
					c.recipient_type,
					COALESCE(CAST(c.departments_json AS CHAR), ''),
					COALESCE(CAST(c.recipients_json AS CHAR), '')
				), 256) AS recipient_hash,
				TIMESTAMP(DATE_SUB(dr.planned_date, INTERVAL CAST(jd.day_before AS SIGNED) DAY)) AS due_utc
			FROM reminder_configs c
			INNER JOIN disclosure_records dr ON dr.record_id = c.disclosure_id
			INNER JOIN JSON_TABLE(c.days_before_json, '$[*]' COLUMNS (day_before INT PATH '$')) jd
			WHERE c.enabled = 1
			  AND c.mode = 'DaysBefore'
			  AND dr.planned_date IS NOT NULL

			UNION ALL

			SELECT
				c.disclosure_id,
				c.scope_type,
				c.scope_id,
				SHA2(CONCAT_WS('|',
					c.recipient_type,
					COALESCE(CAST(c.departments_json AS CHAR), ''),
					COALESCE(CAST(c.recipients_json AS CHAR), '')
				), 256) AS recipient_hash,
				TIMESTAMP(CAST(js.specific_date AS DATE)) AS due_utc
			FROM reminder_configs c
			INNER JOIN disclosure_records dr ON dr.record_id = c.disclosure_id
			INNER JOIN JSON_TABLE(c.specific_dates_json, '$[*]' COLUMNS (specific_date VARCHAR(32) PATH '$')) js
			WHERE c.enabled = 1
			  AND c.mode = 'SpecificDate'
			  AND dr.planned_date IS NOT NULL
			  AND CAST(js.specific_date AS DATE) <= dr.planned_date
		) src
		WHERE src.due_utc <= ?
	`, now)
	if err != nil {
		return 0, fmt.Errorf("materialize due reminder occurrences: %w", err)
	}
	n, _ := result.RowsAffected()
	return int(n), nil
}

func (r *Repository) ListDispatchCandidates(ctx context.Context, now time.Time, limit int) ([]reminderapp.DispatchCandidate, error) {
	if limit <= 0 {
		limit = 50
	}
	// JOIN strategy for cross-scope type_id and company resolution:
	//  - DISCLOSURE scope:   o.disclosure_id = disclosure_records.record_id  → dr
	//  - WORKFLOW_STEP scope: o.disclosure_id = workflow_instances.workflow_instance_id → wi → dr2
	// reminder_configs is LEFT JOIN so workflow_step occurrences without a config still appear.
	rows, err := r.db.QueryContext(ctx, `
		SELECT o.occurrence_id, o.idempotency_key, o.attempt_count, o.scheduled_at,
		       c.recipients_json,
		       o.disclosure_id,
		       o.scope_type,
		       o.scope_id,
		       COALESCE(dr.title, ''),
		       COALESCE(dr.planned_date, DATE(o.scheduled_at)),
		       COALESCE(dr.status, ''),
		       COALESCE(dr.company_id, wi.company_id, ''),
		       COALESCE(dr.type_id, dr2.type_id, ''),
		       COALESCE(comp.company_name, '')
		FROM reminder_occurrences o
		LEFT JOIN reminder_configs c
		  ON c.scope_type = o.scope_type AND c.scope_id = o.scope_id
		LEFT JOIN disclosure_records dr
		  ON dr.record_id = o.disclosure_id
		LEFT JOIN workflow_instances wi
		  ON wi.workflow_instance_id = o.disclosure_id
		LEFT JOIN disclosure_records dr2
		  ON dr2.record_id = wi.record_id
		LEFT JOIN companies comp
		  ON comp.company_id = COALESCE(dr.company_id, wi.company_id)
		WHERE (
		    o.status = 'PENDING' AND o.scheduled_at <= ?
		) OR (
		    o.status = 'RETRY_SCHEDULED' AND o.next_retry_at IS NOT NULL AND o.next_retry_at <= ?
		)
		ORDER BY o.scheduled_at ASC
		LIMIT ?
	`, now.UTC(), now.UTC(), limit)
	if err != nil {
		return nil, fmt.Errorf("list dispatch candidates: %w", err)
	}
	defer rows.Close()

	out := make([]reminderapp.DispatchCandidate, 0)
	for rows.Next() {
		var c reminderapp.DispatchCandidate
		var recipientsJSON []byte
		var disclosureID, scopeType, scopeID, title, status, companyID string
		var deadlineDate time.Time
		var disclosureTypeID, companyName string
		if err := rows.Scan(
			&c.OccurrenceID,
			&c.IdempotencyKey,
			&c.CurrentAttempt,
			&c.ScheduledAt,
			&recipientsJSON,
			&disclosureID,
			&scopeType,
			&scopeID,
			&title,
			&deadlineDate,
			&status,
			&companyID,
			&disclosureTypeID,
			&companyName,
		); err != nil {
			return nil, fmt.Errorf("scan dispatch candidate: %w", err)
		}
		_ = json.Unmarshal(recipientsJSON, &c.RecipientEmails)
		c.TemplateCode = "REMINDER_DISCLOSURE_DUE"
		c.TemplatePayload = buildReminderTemplatePayload(disclosureID, scopeType, scopeID, title, deadlineDate, c.ScheduledAt, status, companyID)
		// Populate new Phase 4 fields.
		c.ScopeType = reminderapp.ScopeType(scopeType)
		c.ScopeID = scopeID
		c.CompanyID = companyID
		c.CompanyName = companyName
		c.DisclosureTypeID = disclosureTypeID
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dispatch candidates: %w", err)
	}
	return out, nil
}

func buildReminderTemplatePayload(disclosureID, scopeType, scopeID, title string, deadlineDate, scheduledAt time.Time, status, companyID string) map[string]any {
	displayTitle := strings.TrimSpace(title)
	if displayTitle == "" {
		displayTitle = disclosureID
	}
	return map[string]any{
		"disclosure_id": disclosureID,
		"scope_type":    scopeType,
		"scope_id":      scopeID,
		"title":         displayTitle,
		"deadline_date": deadlineDate.UTC().Format("2006-01-02"),
		"scheduled_at":  scheduledAt.UTC().Format(time.RFC3339),
		"status":        strings.TrimSpace(status),
		"company_id":    strings.TrimSpace(companyID),
		"action_url":    "/app/disclosures/" + disclosureID,
	}
}

type occurrenceScanner interface {
	Scan(dest ...any) error
}

func scanOccurrenceRow(row occurrenceScanner) (*reminderapp.ReminderOccurrenceDTO, error) {
	var out reminderapp.ReminderOccurrenceDTO
	var scopeType, status string
	var lastAttemptAt sql.NullTime
	var lastErrorCode, providerMessageID sql.NullString
	if err := row.Scan(
		&out.OccurrenceID, &out.DisclosureID, &scopeType, &out.ScopeID, &out.ScheduledAt, &status,
		&out.AttemptCount, &out.IdempotencyKey, &lastAttemptAt, &lastErrorCode, &providerMessageID,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "occurrence not found", nil)
		}
		return nil, err
	}
	out.ScopeType = reminderapp.ScopeType(scopeType)
	out.Status = reminderapp.ReminderStatus(status)
	if lastAttemptAt.Valid {
		t := lastAttemptAt.Time.UTC()
		out.LastAttemptAt = &t
	}
	if lastErrorCode.Valid {
		out.LastErrorCode = lastErrorCode.String
	}
	if providerMessageID.Valid {
		out.ProviderMessageID = providerMessageID.String
	}
	return &out, nil
}

func nullableString(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func nullableTime(t *time.Time) any {
	if t == nil || t.IsZero() {
		return nil
	}
	return t.UTC()
}
