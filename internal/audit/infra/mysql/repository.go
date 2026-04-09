package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
)

// Repository appends rows to audit_logs (migration 0001).
type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Append(ctx context.Context, e auditapp.Entry) error {
	if e.EventID == "" {
		return fmt.Errorf("audit append: event_id required")
	}
	occurred, err := time.Parse(time.RFC3339, e.OccurredAt)
	if err != nil {
		return fmt.Errorf("audit append: parse occurred_at: %w", err)
	}

	meta, err := jsonColumn(e.Metadata)
	if err != nil {
		return fmt.Errorf("audit append: metadata_json: %w", err)
	}
	permSnap, err := jsonColumn(e.EffectivePermissionsSnapshot)
	if err != nil {
		return fmt.Errorf("audit append: effective_permissions_snapshot: %w", err)
	}
	scopeSnap, err := jsonColumn(e.EffectiveScopeSnapshot)
	if err != nil {
		return fmt.Errorf("audit append: effective_scope_snapshot: %w", err)
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO audit_logs (
			event_id, occurred_at, actor_user_id, actor_membership_id, company_id,
			action, resource_type, resource_id, decision, request_id, ip, user_agent,
			effective_permissions_snapshot, effective_scope_snapshot, metadata_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		e.EventID,
		occurred,
		nullString(e.ActorUserID),
		nullString(e.ActorMembershipID),
		nullString(e.CompanyID),
		e.Action,
		nullString(e.ResourceType),
		nullString(e.ResourceID),
		nullString(e.Decision),
		nullString(e.RequestID),
		nullString(trunc(e.IP, 64)),
		nullString(trunc(e.UserAgent, 512)),
		permSnap,
		scopeSnap,
		meta,
	)
	if err != nil {
		return fmt.Errorf("audit insert: %w", err)
	}
	return nil
}

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func trunc(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

// jsonColumn returns nil for NULL column, or JSON bytes for MySQL JSON type.
func jsonColumn(m map[string]any) (interface{}, error) {
	if m == nil {
		return nil, nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return b, nil
}
