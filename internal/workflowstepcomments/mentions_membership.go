package workflowstepcomments

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// MySQLMembershipUserResolver resolves active membership → user_id (same company).
type MySQLMembershipUserResolver struct {
	db *sql.DB
}

func NewMySQLMembershipUserResolver(db *sql.DB) *MySQLMembershipUserResolver {
	return &MySQLMembershipUserResolver{db: db}
}

func (r *MySQLMembershipUserResolver) ResolveActiveUser(ctx context.Context, companyID, membershipID string) (userID string, ok bool, err error) {
	companyID = strings.TrimSpace(companyID)
	membershipID = strings.TrimSpace(membershipID)
	if companyID == "" || membershipID == "" {
		return "", false, nil
	}
	var uid, status, rowCompany string
	err = r.db.QueryRowContext(ctx, `
		SELECT user_id, company_id, membership_status
		FROM memberships
		WHERE membership_id = ?
		LIMIT 1
	`, membershipID).Scan(&uid, &rowCompany, &status)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("resolve membership user for mention notify: %w", err)
	}
	if strings.TrimSpace(rowCompany) != companyID {
		return "", false, nil
	}
	if !strings.EqualFold(strings.TrimSpace(status), "active") {
		return "", false, nil
	}
	uid = strings.TrimSpace(uid)
	if uid == "" {
		return "", false, nil
	}
	return uid, true, nil
}

// MemoryMembershipUserResolver is a test double.
type MemoryMembershipUserResolver struct {
	// UserByMembership[companyID][membershipID] = userID (active only).
	UserByMembership map[string]map[string]string
}

func (m *MemoryMembershipUserResolver) ResolveActiveUser(_ context.Context, companyID, membershipID string) (string, bool, error) {
	if m.UserByMembership == nil {
		return "", false, nil
	}
	byCo, ok := m.UserByMembership[companyID]
	if !ok {
		return "", false, nil
	}
	uid, found := byCo[membershipID]
	uid = strings.TrimSpace(uid)
	if !found || uid == "" {
		return "", false, nil
	}
	return uid, true, nil
}

// MySQLMembershipActiveLookup checks memberships.company_id + active status.
type MySQLMembershipActiveLookup struct {
	db *sql.DB
}

func NewMySQLMembershipActiveLookup(db *sql.DB) *MySQLMembershipActiveLookup {
	return &MySQLMembershipActiveLookup{db: db}
}

func (l *MySQLMembershipActiveLookup) LookupActiveInCompany(ctx context.Context, companyID, membershipID string) (active bool, found bool, err error) {
	companyID = strings.TrimSpace(companyID)
	membershipID = strings.TrimSpace(membershipID)
	if companyID == "" || membershipID == "" {
		return false, false, nil
	}
	var status string
	var rowCompany string
	err = l.db.QueryRowContext(ctx, `
		SELECT company_id, membership_status
		FROM memberships
		WHERE membership_id = ?
		LIMIT 1
	`, membershipID).Scan(&rowCompany, &status)
	if err == sql.ErrNoRows {
		return false, false, nil
	}
	if err != nil {
		return false, false, fmt.Errorf("lookup membership for mention: %w", err)
	}
	if strings.TrimSpace(rowCompany) != companyID {
		// Cross-tenant: treat as not found for fail-closed validation (400, no leak).
		return false, false, nil
	}
	return strings.EqualFold(strings.TrimSpace(status), "active"), true, nil
}

// MySQLMentionDisplayResolver resolves membership display names for response enrichment.
type MySQLMentionDisplayResolver struct {
	db *sql.DB
}

func NewMySQLMentionDisplayResolver(db *sql.DB) *MySQLMentionDisplayResolver {
	return &MySQLMentionDisplayResolver{db: db}
}

func (r *MySQLMentionDisplayResolver) ResolveMentionDisplays(ctx context.Context, companyID string, membershipIDs []string) (map[string]MentionDisplayInfo, error) {
	out := make(map[string]MentionDisplayInfo, len(membershipIDs))
	uniq := make([]string, 0, len(membershipIDs))
	seen := map[string]struct{}{}
	for _, id := range membershipIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		uniq = append(uniq, id)
	}
	if len(uniq) == 0 {
		return out, nil
	}
	placeholders := make([]string, len(uniq))
	args := make([]any, 0, len(uniq)+1)
	args = append(args, strings.TrimSpace(companyID))
	for i, id := range uniq {
		placeholders[i] = "?"
		args = append(args, id)
	}
	q := fmt.Sprintf(`
		SELECT m.membership_id,
		       m.membership_status,
		       COALESCE(NULLIF(TRIM(u.full_name), ''), NULLIF(TRIM(u.login_id), ''), m.membership_id)
		FROM memberships m
		LEFT JOIN users u ON u.user_id = m.user_id
		WHERE m.company_id = ?
		  AND m.membership_id IN (%s)
	`, strings.Join(placeholders, ","))
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return out, fmt.Errorf("resolve mention displays: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var mid, status, name string
		if err := rows.Scan(&mid, &status, &name); err != nil {
			return out, err
		}
		active := strings.EqualFold(strings.TrimSpace(status), "active")
		if !active {
			name = InactiveMentionDisplayName
		}
		out[mid] = MentionDisplayInfo{DisplayName: name, Active: active}
	}
	return out, rows.Err()
}

// MemoryMembershipActiveLookup is a test double.
type MemoryMembershipActiveLookup struct {
	// Active[companyID][membershipID] = true when active in that company.
	Active map[string]map[string]bool
	// OtherCompany holds membership IDs that exist but belong elsewhere (still found=false to caller).
	OtherCompany map[string]bool
}

func (m *MemoryMembershipActiveLookup) LookupActiveInCompany(_ context.Context, companyID, membershipID string) (bool, bool, error) {
	if m.OtherCompany[membershipID] {
		return false, false, nil
	}
	if m.Active == nil {
		return false, false, nil
	}
	byCo, ok := m.Active[companyID]
	if !ok {
		return false, false, nil
	}
	active, found := byCo[membershipID]
	return active && found, found, nil
}

// MemoryMentionDisplayResolver is a test double.
type MemoryMentionDisplayResolver struct {
	ByMembership map[string]MentionDisplayInfo
}

func (m *MemoryMentionDisplayResolver) ResolveMentionDisplays(_ context.Context, _ string, membershipIDs []string) (map[string]MentionDisplayInfo, error) {
	out := make(map[string]MentionDisplayInfo, len(membershipIDs))
	for _, id := range membershipIDs {
		if info, ok := m.ByMembership[id]; ok {
			if !info.Active {
				info.DisplayName = InactiveMentionDisplayName
			}
			out[id] = info
		} else {
			out[id] = MentionDisplayInfo{DisplayName: InactiveMentionDisplayName, Active: false}
		}
	}
	return out, nil
}
