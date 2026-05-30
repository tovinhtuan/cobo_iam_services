package mysql

import (
	"context"
	"database/sql"
	"errors"
	"time"

	reminderapp "github.com/cobo/cobo_iam_services/internal/reminder/app"
)

// Compile-time assertion: *AlertConfigRepository implements the interface.
var _ reminderapp.AlertConfigRepository = (*AlertConfigRepository)(nil)

// AlertConfigRepository implements reminderapp.AlertConfigRepository backed by MySQL.
type AlertConfigRepository struct {
	db *sql.DB
}

func NewAlertConfigRepository(db *sql.DB) *AlertConfigRepository {
	return &AlertConfigRepository{db: db}
}

func (r *AlertConfigRepository) GetByTypeID(ctx context.Context, typeID string) ([]reminderapp.AlertTemplateConfig, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, type_id, alert_kind, template_key, enabled, created_by, created_at, updated_at
		FROM alert_template_configs
		WHERE type_id = ?
	`, typeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []reminderapp.AlertTemplateConfig
	for rows.Next() {
		var c reminderapp.AlertTemplateConfig
		var enabled int8
		if err := rows.Scan(
			&c.ID, &c.TypeID, &c.AlertKind, &c.TemplateKey,
			&enabled, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		c.Enabled = enabled == 1
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *AlertConfigRepository) GetByTypeAndKind(ctx context.Context, typeID, alertKind string) (*reminderapp.AlertTemplateConfig, error) {
	var c reminderapp.AlertTemplateConfig
	var enabled int8
	err := r.db.QueryRowContext(ctx, `
		SELECT id, type_id, alert_kind, template_key, enabled, created_by, created_at, updated_at
		FROM alert_template_configs
		WHERE type_id = ? AND alert_kind = ?
	`, typeID, alertKind).Scan(
		&c.ID, &c.TypeID, &c.AlertKind, &c.TemplateKey,
		&enabled, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c.Enabled = enabled == 1
	return &c, nil
}

func (r *AlertConfigRepository) Upsert(ctx context.Context, in reminderapp.AlertTemplateConfig) error {
	enabledVal := 0
	if in.Enabled {
		enabledVal = 1
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO alert_template_configs (type_id, alert_kind, template_key, enabled, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			template_key = VALUES(template_key),
			enabled      = VALUES(enabled),
			updated_at   = VALUES(updated_at)
	`,
		in.TypeID, in.AlertKind, in.TemplateKey, enabledVal, in.CreatedBy,
		time.Now().UTC(), time.Now().UTC(),
	)
	return err
}
