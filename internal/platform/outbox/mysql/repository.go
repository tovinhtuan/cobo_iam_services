package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/cobo/cobo_iam_services/internal/platform/outbox"
)

// Repository persists outbox rows in MySQL 8+ (FOR UPDATE SKIP LOCKED).
type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Insert(ctx context.Context, p outbox.InsertParams) error {
	if r.db == nil {
		return fmt.Errorf("outbox mysql: nil db")
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO outbox_events (
			event_id, aggregate_type, aggregate_id, event_type, payload_json,
			status, available_at
		) VALUES (?, ?, ?, ?, ?, 'pending', ?)
	`, p.EventID, p.AggregateType, p.AggregateID, p.EventType, p.PayloadJSON, p.AvailableAt)
	if err != nil {
		return fmt.Errorf("outbox insert: %w", err)
	}
	return nil
}

func (r *Repository) LockPendingBatch(ctx context.Context, batchSize int, now time.Time) ([]outbox.QueuedEvent, error) {
	if r.db == nil {
		return nil, fmt.Errorf("outbox mysql: nil db")
	}
	if batchSize <= 0 {
		batchSize = 50
	}

	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("outbox lock batch begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, `
		SELECT event_id FROM outbox_events
		WHERE status = 'pending' AND available_at <= ?
		ORDER BY available_at ASC, event_id ASC
		LIMIT ?
		FOR UPDATE SKIP LOCKED
	`, now, batchSize)
	if err != nil {
		return nil, fmt.Errorf("outbox lock batch select: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("outbox lock batch scan id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("outbox lock batch rows: %w", err)
	}
	_ = rows.Close()

	if len(ids) == 0 {
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("outbox lock batch commit: %w", err)
		}
		return nil, nil
	}

	ph := placeholders(len(ids))
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE outbox_events SET status = 'processing'
		WHERE event_id IN (`+ph+`) AND status = 'pending'
	`, args...); err != nil {
		return nil, fmt.Errorf("outbox lock batch update: %w", err)
	}

	q := `
		SELECT event_id, aggregate_type, aggregate_id, event_type, payload_json, retry_count, available_at
		FROM outbox_events
		WHERE event_id IN (` + ph + `)
	`
	sel, err := tx.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("outbox lock batch load: %w", err)
	}
	defer sel.Close()

	var out []outbox.QueuedEvent
	for sel.Next() {
		var e outbox.QueuedEvent
		var payload []byte
		if err := sel.Scan(&e.EventID, &e.AggregateType, &e.AggregateID, &e.EventType, &payload, &e.RetryCount, &e.AvailableAt); err != nil {
			return nil, fmt.Errorf("outbox lock batch scan row: %w", err)
		}
		e.PayloadJSON = append([]byte(nil), payload...)
		out = append(out, e)
	}
	if err := sel.Err(); err != nil {
		return nil, fmt.Errorf("outbox lock batch scan rows: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("outbox lock batch commit: %w", err)
	}
	return out, nil
}

func (r *Repository) MarkProcessed(ctx context.Context, eventID string, processedAt time.Time) error {
	if r.db == nil {
		return fmt.Errorf("outbox mysql: nil db")
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE outbox_events
		SET status = 'processed', processed_at = ?, last_error = NULL
		WHERE event_id = ?
	`, processedAt, eventID)
	if err != nil {
		return fmt.Errorf("outbox mark processed: %w", err)
	}
	return nil
}

func (r *Repository) MarkFailedPermanent(ctx context.Context, eventID string, at time.Time, lastErr string) error {
	if r.db == nil {
		return fmt.Errorf("outbox mysql: nil db")
	}
	if len(lastErr) > 1024 {
		lastErr = lastErr[:1024]
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE outbox_events
		SET status = 'failed_permanent', processed_at = ?, last_error = ?
		WHERE event_id = ?
	`, at, lastErr, eventID)
	if err != nil {
		return fmt.Errorf("outbox mark failed_permanent: %w", err)
	}
	return nil
}

func (r *Repository) MarkRetry(ctx context.Context, eventID string, retryCount int, nextAt time.Time, lastErr string) error {
	if r.db == nil {
		return fmt.Errorf("outbox mysql: nil db")
	}
	if len(lastErr) > 1024 {
		lastErr = lastErr[:1024]
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE outbox_events
		SET status = 'pending', retry_count = ?, available_at = ?, last_error = ?
		WHERE event_id = ?
	`, retryCount, nextAt, lastErr, eventID)
	if err != nil {
		return fmt.Errorf("outbox mark retry: %w", err)
	}
	return nil
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}
