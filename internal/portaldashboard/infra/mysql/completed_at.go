package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// CompletedAtRepository reads disclosure_records.completed_at only (no writes).
type CompletedAtRepository struct {
	db *sql.DB
}

func NewCompletedAtRepository(db *sql.DB) *CompletedAtRepository {
	return &CompletedAtRepository{db: db}
}

func (r *CompletedAtRepository) MapCompletedAt(ctx context.Context, companyID string, recordIDs []string) (map[string]time.Time, error) {
	out := map[string]time.Time{}
	if r == nil || r.db == nil || strings.TrimSpace(companyID) == "" || len(recordIDs) == 0 {
		return out, nil
	}
	ids := uniqueNonEmpty(recordIDs)
	if len(ids) == 0 {
		return out, nil
	}
	// Chunk to keep query size bounded.
	const chunk = 200
	for i := 0; i < len(ids); i += chunk {
		end := i + chunk
		if end > len(ids) {
			end = len(ids)
		}
		part := ids[i:end]
		placeholders := make([]string, len(part))
		args := make([]any, 0, 1+len(part))
		args = append(args, companyID)
		for j, id := range part {
			placeholders[j] = "?"
			args = append(args, id)
		}
		q := fmt.Sprintf(`
			SELECT record_id, completed_at
			FROM disclosure_records
			WHERE company_id = ?
			  AND record_id IN (%s)
			  AND completed_at IS NOT NULL
		`, strings.Join(placeholders, ","))
		rows, err := r.db.QueryContext(ctx, q, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id string
			var completedAt time.Time
			if err := rows.Scan(&id, &completedAt); err != nil {
				rows.Close()
				return nil, err
			}
			out[id] = completedAt.UTC()
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func uniqueNonEmpty(ids []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
