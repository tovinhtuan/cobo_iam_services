package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cobo/cobo_iam_services/internal/disclosure/app/deadlineengine"
)

// ErrInvalidProposalDeadlineDayType is returned when a persisted non-empty day type
// is not WORKING_DAYS or CALENDAR_DAYS (corrupted row). Nil/empty still means calendar.
var ErrInvalidProposalDeadlineDayType = errors.New("invalid proposed_deadline_day_type")

// ResolveProposalDeadlineDayTypeForDue maps nil/empty → CALENDAR_DAYS and rejects
// unknown non-empty values. Use for runtime due calculation only.
func ResolveProposalDeadlineDayTypeForDue(v *ProposalDeadlineDayType) (ProposalDeadlineDayType, error) {
	if v == nil || strings.TrimSpace(string(*v)) == "" {
		return ProposalDeadlineDayTypeCalendarDays, nil
	}
	switch *v {
	case ProposalDeadlineDayTypeWorkingDays, ProposalDeadlineDayTypeCalendarDays:
		return *v, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrInvalidProposalDeadlineDayType, string(*v))
	}
}

// ProposalDueInput is the proposal-level deadline source for runtime/read models.
// Absolute dates short-circuit calculation (final then proposed absolute).
type ProposalDueInput struct {
	FinalDeadlineDate    sql.NullTime
	ProposedT0Date       sql.NullTime
	ProposedDeadlineDays sql.NullInt64
	ProposedDeadlineDate sql.NullTime
	// DayType is the persisted proposed_deadline_day_type (may be nil/empty).
	DayType *ProposalDeadlineDayType
}

// FormatProposalDueDate is the single authoritative Ad-hoc proposal due formatter
// (YYYY-MM-DD). Marker: SINGLE_AUTHORITATIVE_PROPOSAL_DUE_CALCULATION.
//
// Precedence (preserved from legacy formatAdHocDueDate):
//  1. final_deadline_date
//  2. T0 + proposed_deadline_days with effective day type (days > 0)
//  3. proposed_deadline_date (absolute, when valid >= 2000-01-01)
//
// CALENDAR_DAYS uses AddDate(0,0,N) via deadlineengine.AddDaysAfter.
// WORKING_DAYS uses the same AddDaysAfter working path (weekends + holidays).
func FormatProposalDueDate(ctx context.Context, in ProposalDueInput, holidays deadlineengine.NonTradingDayChecker) (string, error) {
	if in.FinalDeadlineDate.Valid {
		return in.FinalDeadlineDate.Time.UTC().Format("2006-01-02"), nil
	}

	if in.ProposedT0Date.Valid && in.ProposedDeadlineDays.Valid && in.ProposedDeadlineDays.Int64 > 0 {
		dayType, err := ResolveProposalDeadlineDayTypeForDue(in.DayType)
		if err != nil {
			return "", err
		}
		start := dateOnlyUTC(in.ProposedT0Date.Time)
		n := int(in.ProposedDeadlineDays.Int64)
		var engineType deadlineengine.DayType
		switch dayType {
		case ProposalDeadlineDayTypeWorkingDays:
			engineType = deadlineengine.DayTypeWorking
		default:
			engineType = deadlineengine.DayTypeCalendar
		}
		due, err := deadlineengine.AddDaysAfter(ctx, start, n, engineType, holidays)
		if err != nil {
			return "", err
		}
		return due.UTC().Format("2006-01-02"), nil
	}

	if in.ProposedDeadlineDate.Valid && !in.ProposedDeadlineDate.Time.Before(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)) {
		return in.ProposedDeadlineDate.Time.UTC().Format("2006-01-02"), nil
	}
	return "", nil
}

// ParsePersistedDeadlineDayType converts a DB NULL/string into a typed pointer.
// Empty/NULL → nil (legacy). Invalid non-empty is kept as typed value so due
// calculation can reject it via ResolveProposalDeadlineDayTypeForDue.
func ParsePersistedDeadlineDayType(raw sql.NullString) *ProposalDeadlineDayType {
	if !raw.Valid {
		return nil
	}
	s := strings.TrimSpace(raw.String)
	if s == "" {
		return nil
	}
	v := ProposalDeadlineDayType(s)
	return &v
}

func dateOnlyUTC(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}
