package workflowstepcomments

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// MySQLMentionCandidateStore searches active memberships by full_name/login_id.
//
// Performance note (Phase 1 — no new migration):
// - Filter leads with memberships.company_id + membership_status='active' (typical membership indexes).
// - LIKE '%q%' on full_name/login_id cannot use a normal B-tree prefix index; acceptable for
//   company-scoped active sets with page_size ≤ 50. Follow-up index/FTS is out of Phase 1 scope.
type MySQLMentionCandidateStore struct {
	db *sql.DB
}

func NewMySQLMentionCandidateStore(db *sql.DB) *MySQLMentionCandidateStore {
	return &MySQLMentionCandidateStore{db: db}
}

func (s *MySQLMentionCandidateStore) SearchActiveMembers(ctx context.Context, companyID, q string, page, pageSize int) ([]MentionCandidate, int, error) {
	companyID = strings.TrimSpace(companyID)
	q = strings.TrimSpace(q)
	if companyID == "" || q == "" {
		return nil, 0, fmt.Errorf("company_id and q required")
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = CandidatePageDefault
	}
	if pageSize > CandidatePageMax {
		pageSize = CandidatePageMax
	}
	pattern := "%" + EscapeLikePattern(q) + "%"
	var total int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM memberships m
		INNER JOIN users u ON u.user_id = m.user_id
		WHERE m.company_id = ?
		  AND m.membership_status = 'active'
		  AND (
		    LOWER(COALESCE(u.full_name, '')) LIKE LOWER(?) ESCAPE '\\'
		    OR LOWER(COALESCE(u.login_id, '')) LIKE LOWER(?) ESCAPE '\\'
		  )
	`, companyID, pattern, pattern).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count mention candidates: %w", err)
	}
	offset := (page - 1) * pageSize
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.membership_id,
		       COALESCE(NULLIF(TRIM(u.full_name), ''), NULLIF(TRIM(u.login_id), ''), m.user_id) AS display_name
		FROM memberships m
		INNER JOIN users u ON u.user_id = m.user_id
		WHERE m.company_id = ?
		  AND m.membership_status = 'active'
		  AND (
		    LOWER(COALESCE(u.full_name, '')) LIKE LOWER(?) ESCAPE '\\'
		    OR LOWER(COALESCE(u.login_id, '')) LIKE LOWER(?) ESCAPE '\\'
		  )
		ORDER BY display_name ASC, m.membership_id ASC
		LIMIT ? OFFSET ?
	`, companyID, pattern, pattern, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list mention candidates: %w", err)
	}
	defer rows.Close()
	items := make([]MentionCandidate, 0, pageSize)
	for rows.Next() {
		var c MentionCandidate
		if err := rows.Scan(&c.MembershipID, &c.DisplayName); err != nil {
			return nil, 0, fmt.Errorf("scan mention candidate: %w", err)
		}
		items = append(items, c)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
