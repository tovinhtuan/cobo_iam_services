package workflowstepcomments

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"unicode/utf8"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/google/uuid"
)

// InactiveMentionDisplayName is the locked inactive-member label (no email/phone/UUID).
const InactiveMentionDisplayName = "(thành viên không còn hoạt động)"

// MentionInput is the write-side mention payload (no display_name).
type MentionInput struct {
	MembershipID string `json:"membership_id"`
	Start        int    `json:"start"`
	End          int    `json:"end"`
}

// CanonicalMention is a validated, sorted mention used for hash + persist (no display name).
type CanonicalMention struct {
	MembershipID string `json:"membership_id"`
	Start        int    `json:"start"`
	End          int    `json:"end"`
}

// MembershipActiveLookup verifies mention targets at write time.
type MembershipActiveLookup interface {
	// LookupActiveInCompany returns true only when membership exists in company and is active.
	// found=false covers unknown id; wrongCompany=true when id exists in another tenant (treat as invalid).
	LookupActiveInCompany(ctx context.Context, companyID, membershipID string) (active bool, found bool, err error)
}

// MentionDisplayResolver resolves display names for response enrichment (read path).
type MentionDisplayResolver interface {
	ResolveMentionDisplays(ctx context.Context, companyID string, membershipIDs []string) (map[string]MentionDisplayInfo, error)
}

// MentionDisplayInfo is read-path only.
type MentionDisplayInfo struct {
	DisplayName string
	Active      bool
}

// ValidateAndCanonicalizeMentions validates write mentions against trimmed body + memberships.
// Returns canonical slice (sorted) and compact JSON for RequestHash (empty JSON array "[]" when none).
// Does not resolve display names.
func ValidateAndCanonicalizeMentions(
	ctx context.Context,
	companyID string,
	trimmedBody string,
	inputs []MentionInput,
	lookup MembershipActiveLookup,
) (canonical []CanonicalMention, canonicalJSON string, err error) {
	companyID = strings.TrimSpace(companyID)
	if companyID == "" {
		return nil, "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "company_id required", nil)
	}
	if inputs == nil {
		inputs = []MentionInput{}
	}
	if len(inputs) > MaxMentionsPerComment {
		return nil, "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest,
			fmt.Sprintf("mentions exceeds max %d", MaxMentionsPerComment), nil)
	}

	bodyRunes := []rune(trimmedBody)
	bodyLen := len(bodyRunes)
	seen := map[string]struct{}{}
	out := make([]CanonicalMention, 0, len(inputs))

	for i, in := range inputs {
		mid := strings.TrimSpace(in.MembershipID)
		if mid == "" {
			return nil, "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest,
				fmt.Sprintf("mentions[%d].membership_id is required", i), nil)
		}
		if _, err := uuid.Parse(mid); err != nil {
			return nil, "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest,
				fmt.Sprintf("mentions[%d].membership_id must be a UUID", i), nil)
		}
		if in.Start < 0 {
			return nil, "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest,
				fmt.Sprintf("mentions[%d].start must be >= 0", i), nil)
		}
		if in.End <= in.Start {
			return nil, "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest,
				fmt.Sprintf("mentions[%d].end must be > start", i), nil)
		}
		if in.End > bodyLen {
			return nil, "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest,
				fmt.Sprintf("mentions[%d].end exceeds body length", i), nil)
		}
		key := fmt.Sprintf("%d:%d:%s", in.Start, in.End, mid)
		if _, ok := seen[key]; ok {
			return nil, "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest,
				"duplicate mention", nil)
		}
		seen[key] = struct{}{}

		if lookup == nil {
			return nil, "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest,
				"mention membership validation unavailable", nil)
		}
		active, found, lerr := lookup.LookupActiveInCompany(ctx, companyID, mid)
		if lerr != nil {
			return nil, "", lerr
		}
		if !found || !active {
			return nil, "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest,
				fmt.Sprintf("mentions[%d].membership_id is invalid or inactive", i), nil)
		}

		out = append(out, CanonicalMention{
			MembershipID: mid,
			Start:        in.Start,
			End:          in.End,
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Start != out[j].Start {
			return out[i].Start < out[j].Start
		}
		if out[i].End != out[j].End {
			return out[i].End < out[j].End
		}
		return out[i].MembershipID < out[j].MembershipID
	})

	canonicalJSON, err = MarshalCanonicalMentionsJSON(out)
	if err != nil {
		return nil, "", err
	}
	_ = utf8.ValidString(trimmedBody) // body already validated as UTF-8 string
	return out, canonicalJSON, nil
}

// MarshalCanonicalMentionsJSON returns compact JSON with no insignificant whitespace.
func MarshalCanonicalMentionsJSON(canonical []CanonicalMention) (string, error) {
	if canonical == nil {
		canonical = []CanonicalMention{}
	}
	b, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// MentionsNonEmpty reports whether the client sent at least one mention.
func MentionsNonEmpty(inputs []MentionInput) bool {
	return len(inputs) > 0
}
