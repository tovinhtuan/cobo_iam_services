package app

import (
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// Membership list search + status filter (CMS users screen, contract:
// cobo_web_design/docs/ai-cache/cms-users-redesign-2026-10-12/02-contract.md).

// MembershipSearchMaxLen bounds ListCompanyMembershipsRequest.Q (runes).
const MembershipSearchMaxLen = 100

// Account-status groups accepted by ListCompanyMembershipsRequest.Status.
const (
	MembershipStatusActive   = "active"
	MembershipStatusInvited  = "invited"
	MembershipStatusPending  = "pending"
	MembershipStatusInactive = "inactive"
)

// MembershipStatusCounts counts rows per account-status group after the search filter
// (before the status filter), so every chip keeps its number while one is selected.
type MembershipStatusCounts struct {
	All      int `json:"all"`
	Active   int `json:"active"`
	Invited  int `json:"invited"`
	Pending  int `json:"pending"`
	Inactive int `json:"inactive"`
}

var membershipStatusGroups = map[string]string{
	"active":                     MembershipStatusActive,
	"invited":                    MembershipStatusInvited,
	"pending_email_verification": MembershipStatusPending,
	"pending_verification":       MembershipStatusPending,
	"inactive":                   MembershipStatusInactive,
	"suspended":                  MembershipStatusInactive,
	"locked":                     MembershipStatusInactive,
	"disabled":                   MembershipStatusInactive,
	"revoked":                    MembershipStatusInactive,
}

// membershipStatusGroup maps users.account_status to its filter group ("" = only counted in All).
func membershipStatusGroup(accountStatus string) string {
	return membershipStatusGroups[strings.ToLower(strings.TrimSpace(accountStatus))]
}

// normalizeMembershipFilter trims the inputs and rejects an over-long query or an unknown status.
func normalizeMembershipFilter(q, status string) (string, string, error) {
	q = strings.TrimSpace(q)
	status = strings.ToLower(strings.TrimSpace(status))
	fieldErrors := map[string]any{}
	if utf8.RuneCountInString(q) > MembershipSearchMaxLen {
		fieldErrors["q"] = "Từ khoá tìm kiếm tối đa 100 ký tự."
	}
	switch status {
	case "", MembershipStatusActive, MembershipStatusInvited, MembershipStatusPending, MembershipStatusInactive:
	default:
		fieldErrors["status"] = "Trạng thái không hợp lệ (active, invited, pending, inactive)."
	}
	if len(fieldErrors) > 0 {
		return "", "", &perr.HTTPError{
			Code: perr.CodeInvalidRequest, Message: "invalid membership list filter", HTTPStatus: http.StatusBadRequest,
			Details: map[string]any{"field_errors": fieldErrors},
		}
	}
	return q, status, nil
}

// foldSearch lower-cases and strips Vietnamese diacritics so "nguyen" matches "Nguyễn".
func foldSearch(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(strings.ToLower(s)) {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		if r == 'đ' {
			r = 'd'
		}
		b.WriteRune(r)
	}
	return b.String()
}

// filterMemberships applies the search, counts the groups, then applies the status filter.
// The input slice is not modified.
func filterMemberships(all []MembershipView, q, status string) ([]MembershipView, MembershipStatusCounts) {
	needle := foldSearch(q)
	var counts MembershipStatusCounts
	out := make([]MembershipView, 0, len(all))
	for _, m := range all {
		if needle != "" && !strings.Contains(foldSearch(m.FullName), needle) && !strings.Contains(foldSearch(m.LoginID), needle) {
			continue
		}
		group := membershipStatusGroup(m.AccountStatus)
		counts.All++
		switch group {
		case MembershipStatusActive:
			counts.Active++
		case MembershipStatusInvited:
			counts.Invited++
		case MembershipStatusPending:
			counts.Pending++
		case MembershipStatusInactive:
			counts.Inactive++
		}
		if status != "" && group != status {
			continue
		}
		out = append(out, m)
	}
	return out, counts
}
