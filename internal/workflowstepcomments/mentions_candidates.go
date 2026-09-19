package workflowstepcomments

import (
	"context"
	"strings"
	"unicode/utf8"
)

// MentionCandidate is an active company membership eligible for @mention.
type MentionCandidate struct {
	MembershipID string
	DisplayName  string
}

// MentionCandidateStore searches active members within a company (tenant-scoped).
type MentionCandidateStore interface {
	SearchActiveMembers(ctx context.Context, companyID, q string, page, pageSize int) (items []MentionCandidate, total int, err error)
}

// EscapeLikePattern escapes %, _, and \ for SQL LIKE with ESCAPE '\'.
func EscapeLikePattern(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

// RuneLen returns UTF-8 rune count.
func RuneLen(s string) int {
	return utf8.RuneCountInString(s)
}

// MemoryMentionCandidateStore is an in-memory candidate store for unit tests.
type MemoryMentionCandidateStore struct {
	Items []MentionCandidate
	// CompanyID filters; empty Items.CompanyID means match any when CompanyID set on search.
	ByCompany map[string][]MentionCandidate
}

func (m *MemoryMentionCandidateStore) SearchActiveMembers(_ context.Context, companyID, q string, page, pageSize int) ([]MentionCandidate, int, error) {
	src := m.Items
	if m.ByCompany != nil {
		src = m.ByCompany[companyID]
	}
	qLower := strings.ToLower(strings.TrimSpace(q))
	matched := make([]MentionCandidate, 0)
	for _, it := range src {
		name := strings.ToLower(it.DisplayName)
		id := strings.ToLower(it.MembershipID)
		if strings.Contains(name, qLower) || strings.Contains(id, qLower) {
			matched = append(matched, it)
		}
	}
	total := len(matched)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = CandidatePageDefault
	}
	start := (page - 1) * pageSize
	if start >= total {
		return []MentionCandidate{}, total, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	out := make([]MentionCandidate, end-start)
	copy(out, matched[start:end])
	return out, total, nil
}
