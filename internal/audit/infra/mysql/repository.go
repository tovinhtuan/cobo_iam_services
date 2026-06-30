package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
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

func (r *Repository) ListByCompany(ctx context.Context, companyID, action, resourceType, resourceID, fromOccurredAt, toOccurredAt, cursor string, limit int) ([]auditapp.Entry, error) {
	return r.ListFiltered(ctx, auditapp.ListFilter{
		CompanyID:      companyID,
		Action:         action,
		ResourceType:   resourceType,
		ResourceID:     resourceID,
		FromOccurredAt: fromOccurredAt,
		ToOccurredAt: toOccurredAt,
		Cursor:         cursor,
		Limit:          limit,
	})
}

func (r *Repository) ListFiltered(ctx context.Context, filter auditapp.ListFilter) ([]auditapp.Entry, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	companyID := strings.TrimSpace(filter.CompanyID)
	action := strings.TrimSpace(filter.Action)
	resourceType := strings.TrimSpace(filter.ResourceType)
	resourceID := strings.TrimSpace(filter.ResourceID)
	fromOccurredAt := strings.TrimSpace(filter.FromOccurredAt)
	toOccurredAt := strings.TrimSpace(filter.ToOccurredAt)
	cursor := strings.TrimSpace(filter.Cursor)

	actionSQL := ""
	actionArgs := []any{}
	switch {
	case filter.ActionPrefix && action != "":
		actionSQL = "AND action LIKE CONCAT(?, '%')"
		actionArgs = append(actionArgs, action)
	case action != "":
		actionSQL = "AND action = ?"
		actionArgs = append(actionArgs, action)
	case filter.RequireAdminPrefix:
		actionSQL = "AND action LIKE 'admin.%'"
	}

	args := []any{
		companyID, companyID,
		resourceType, resourceType,
		resourceID, resourceID,
		fromOccurredAt, fromOccurredAt,
		toOccurredAt, toOccurredAt,
		cursor, cursor,
	}
	args = append(args, actionArgs...)
	args = append(args, limit)

	query := `
		SELECT event_id, occurred_at, IFNULL(actor_user_id, ''), IFNULL(actor_membership_id, ''), IFNULL(company_id, ''),
		       IFNULL(action, ''), IFNULL(resource_type, ''), IFNULL(resource_id, ''), IFNULL(decision, ''),
		       IFNULL(request_id, ''), IFNULL(ip, ''), IFNULL(user_agent, ''),
		       effective_permissions_snapshot, effective_scope_snapshot, metadata_json
		FROM audit_logs
		WHERE (? = '' OR company_id = ?)
		  AND (? = '' OR resource_type = ?)
		  AND (? = '' OR resource_id = ?)
		  AND (? = '' OR occurred_at >= ?)
		  AND (? = '' OR occurred_at <= ?)
		  AND (? = '' OR occurred_at < ?)
		  ` + actionSQL + `
		ORDER BY occurred_at DESC
		LIMIT ?`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("audit list by company: %w", err)
	}
	defer rows.Close()

	items := make([]auditapp.Entry, 0, limit)
	for rows.Next() {
		var (
			entry                     auditapp.Entry
			occurredAt                time.Time
			permSnap, scopeSnap, meta []byte
		)
		if err := rows.Scan(
			&entry.EventID, &occurredAt, &entry.ActorUserID, &entry.ActorMembershipID, &entry.CompanyID,
			&entry.Action, &entry.ResourceType, &entry.ResourceID, &entry.Decision,
			&entry.RequestID, &entry.IP, &entry.UserAgent,
			&permSnap, &scopeSnap, &meta,
		); err != nil {
			return nil, fmt.Errorf("audit list scan: %w", err)
		}
		entry.OccurredAt = occurredAt.UTC().Format(time.RFC3339)
		entry.EffectivePermissionsSnapshot = decodeJSONMap(permSnap)
		entry.EffectiveScopeSnapshot = decodeJSONMap(scopeSnap)
		entry.Metadata = decodeJSONMap(meta)
		items = append(items, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("audit list rows: %w", err)
	}
	return items, nil
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

func decodeJSONMap(raw []byte) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}
