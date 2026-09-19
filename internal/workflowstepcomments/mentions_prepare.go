package workflowstepcomments

import (
	"fmt"
	"sort"
	"strings"
)

// ErrInvalidMentions is returned when persist rows fail structural contract checks.
// Phase 3 maps this to HTTP 400 INVALID_REQUEST; Phase 2 keeps it as a plain error.
type ErrInvalidMentions struct {
	Reason string
}

func (e ErrInvalidMentions) Error() string {
	if e.Reason == "" {
		return "invalid mentions"
	}
	return "invalid mentions: " + e.Reason
}

// PrepareMentionsForPersist validates structural rules, rejects duplicates, and sorts
// by (start_offset, end_offset, mentioned_membership_id) for deterministic storage.
// Does not authorize memberships or check body rune bounds (Phase 3 service).
// Does not store display names.
func PrepareMentionsForPersist(companyID, commentID string, in []Mention) ([]Mention, error) {
	companyID = strings.TrimSpace(companyID)
	commentID = strings.TrimSpace(commentID)
	if companyID == "" || commentID == "" {
		return nil, ErrInvalidMentions{Reason: "company_id and comment_id required"}
	}
	if len(in) > MaxMentionsPerComment {
		return nil, ErrInvalidMentions{Reason: fmt.Sprintf("max %d mentions per comment", MaxMentionsPerComment)}
	}
	out := make([]Mention, 0, len(in))
	seen := map[string]struct{}{}
	for i, m := range in {
		m.CompanyID = companyID
		m.CommentID = commentID
		m.MentionedMembershipID = strings.TrimSpace(m.MentionedMembershipID)
		m.ID = strings.TrimSpace(m.ID)
		if m.MentionedMembershipID == "" {
			return nil, ErrInvalidMentions{Reason: fmt.Sprintf("mention[%d] membership_id required", i)}
		}
		if len(m.MentionedMembershipID) > MentionIDLength {
			return nil, ErrInvalidMentions{Reason: fmt.Sprintf("mention[%d] membership_id too long", i)}
		}
		if m.StartOffset < 0 {
			return nil, ErrInvalidMentions{Reason: fmt.Sprintf("mention[%d] start_offset must be >= 0", i)}
		}
		if m.EndOffset <= m.StartOffset {
			return nil, ErrInvalidMentions{Reason: fmt.Sprintf("mention[%d] end_offset must be > start_offset", i)}
		}
		key := fmt.Sprintf("%d:%d:%s", m.StartOffset, m.EndOffset, m.MentionedMembershipID)
		if _, ok := seen[key]; ok {
			return nil, ErrInvalidMentions{Reason: "duplicate mention offsets/membership"}
		}
		seen[key] = struct{}{}
		if m.ID == "" {
			return nil, ErrInvalidMentions{Reason: fmt.Sprintf("mention[%d] id required", i)}
		}
		if len(m.ID) > MentionIDLength {
			return nil, ErrInvalidMentions{Reason: fmt.Sprintf("mention[%d] id too long", i)}
		}
		out = append(out, m)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].StartOffset != out[j].StartOffset {
			return out[i].StartOffset < out[j].StartOffset
		}
		if out[i].EndOffset != out[j].EndOffset {
			return out[i].EndOffset < out[j].EndOffset
		}
		return out[i].MentionedMembershipID < out[j].MentionedMembershipID
	})
	return out, nil
}
