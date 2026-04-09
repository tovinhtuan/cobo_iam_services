package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/cobo/cobo_iam_services/internal/platform/events"
	"github.com/cobo/cobo_iam_services/internal/platform/outbox"
)

// InsertTx inserts one outbox row inside an existing transaction (transactional outbox).
func (r *Repository) InsertTx(ctx context.Context, tx *sql.Tx, p outbox.InsertParams) error {
	if tx == nil {
		return fmt.Errorf("outbox mysql InsertTx: nil tx")
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO outbox_events (
			event_id, aggregate_type, aggregate_id, event_type, payload_json,
			status, available_at
		) VALUES (?, ?, ?, ?, ?, 'pending', ?)
	`, p.EventID, p.AggregateType, p.AggregateID, p.EventType, p.PayloadJSON, p.AvailableAt)
	if err != nil {
		return fmt.Errorf("outbox insert tx: %w", err)
	}
	return nil
}

// PublishEventTx marshals an envelope and inserts using InsertTx.
func (r *Repository) PublishEventTx(ctx context.Context, tx *sql.Tx, event events.Event) error {
	payload, err := json.Marshal(event.Payload)
	if err != nil {
		return fmt.Errorf("marshal event payload: %w", err)
	}
	return r.InsertTx(ctx, tx, outbox.InsertParams{
		EventID:       event.EventID,
		AggregateType: event.AggregateType,
		AggregateID:   event.AggregateID,
		EventType:     event.EventType,
		PayloadJSON:   payload,
		AvailableAt:   event.OccurredAt,
	})
}
