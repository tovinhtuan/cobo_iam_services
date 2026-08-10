package app

import (
	"net/http"
	"strings"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// ProposalDeadlineDayType is the proposal-owned day-kind for proposed_deadline_days.
// Wire values match disclosure duration labels but this type is intentionally
// owned by adhoc (PROPOSAL_DAY_TYPE_OWNED) — do not inherit from type/CMS.
type ProposalDeadlineDayType string

const (
	ProposalDeadlineDayTypeWorkingDays  ProposalDeadlineDayType = "WORKING_DAYS"
	ProposalDeadlineDayTypeCalendarDays ProposalDeadlineDayType = "CALENDAR_DAYS"
)

// EffectiveProposalDeadlineDayType maps persisted NULL/empty to CALENDAR_DAYS.
// Marker: LEGACY_NULL_MEANS_CALENDAR_DAYS / EXISTING_PROPOSAL_DEADLINE_SEMANTICS_PRESERVED.
// Phase A introduces the helper; runtime due wiring waits until Phase C.
func EffectiveProposalDeadlineDayType(v *ProposalDeadlineDayType) ProposalDeadlineDayType {
	if v == nil || strings.TrimSpace(string(*v)) == "" {
		return ProposalDeadlineDayTypeCalendarDays
	}
	return *v
}

// parseOptionalProposalDeadlineDayType validates create/PATCH wire input.
// empty → (nil, nil); valid enum → pointer; anything else → field error.
func parseOptionalProposalDeadlineDayType(raw string) (*ProposalDeadlineDayType, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	switch ProposalDeadlineDayType(raw) {
	case ProposalDeadlineDayTypeWorkingDays, ProposalDeadlineDayTypeCalendarDays:
		v := ProposalDeadlineDayType(raw)
		return &v, nil
	default:
		return nil, newAdHocFieldError(
			http.StatusBadRequest,
			perr.CodeInvalidRequest,
			"proposed_deadline_day_type",
			"proposed_deadline_day_type must be WORKING_DAYS or CALENDAR_DAYS",
		)
	}
}
