package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// MaterializerEffectiveOpenAtSQL is the business OpenAt authority Next Alert must match.
//
// Proven sources (Phase B.1 reconciliation):
//   - materializePeriodicDisclosures comment + bufferDays=0 intent
//   - fakePeriodicRepo.ListPendingCycles gate: OpenAt else CycleStart (no due_date)
//   - materializePeriodicDisclosures skips cycles with zero CycleStart (due_date alone cannot materialize)
//   - deadlinealerts ListRows membership: COALESCE(open_at, cycle_start)
//
// Note: disclosure mysql ListPendingCycles still lists with COALESCE(open_at, cycle_start, due_date),
// but app-layer CycleStart requirement makes due_date an unreachable successful-materialize fallback.
const MaterializerEffectiveOpenAtSQL = "COALESCE(pc.open_at, pc.cycle_start)"

// EffectiveOpenAtYMD returns materializer-aligned OpenAt as YYYY-MM-DD:
// COALESCE(open_at, cycle_start). due_date must NOT extend the boundary.
func EffectiveOpenAtYMD(openAt, cycleStart string) string {
	for _, v := range []string{openAt, cycleStart} {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

// IsNextAlertOpenAtEligible mirrors materializer boundary: NEXT only while TodayHCM < EffectiveOpenAt.
// At/after OpenAt (including materializer lag without record) → NOT_NEXT.
func IsNextAlertOpenAtEligible(todayHCM, effectiveOpenAtYMD string) bool {
	today := strings.TrimSpace(todayHCM)
	open := strings.TrimSpace(effectiveOpenAtYMD)
	if today == "" || open == "" {
		return false
	}
	return today < open
}

// SelectNearestNextAlertPerType keeps at most one cycle per type_id.
// Input must already be sorted by OpenAt ASC, CycleStart ASC, TypeID ASC, CycleID ASC.
func SelectNearestNextAlertPerType(rows []NextAlertCycleRow) []NextAlertCycleRow {
	if len(rows) == 0 {
		return nil
	}
	out := make([]NextAlertCycleRow, 0, len(rows))
	seen := map[string]struct{}{}
	for _, row := range rows {
		tid := strings.TrimSpace(row.TypeID)
		if tid == "" {
			continue
		}
		if _, ok := seen[tid]; ok {
			continue
		}
		seen[tid] = struct{}{}
		out = append(out, row)
	}
	return out
}

func (s *service) ListNextDeadlineAlerts(ctx context.Context, sub Subject) (*ListNextDeadlineAlertsResponse, error) {
	if strings.TrimSpace(sub.CompanyID) == "" {
		return nil, perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeCompanyContextRequired, "company_id is required", nil)
	}
	if err := s.authorizeView(ctx, sub); err != nil {
		return nil, err
	}

	todayHCM := businessDateHCMService(s.now())
	rows, err := s.repo.ListNextAlertCycles(ctx, sub.CompanyID, todayHCM)
	if err != nil {
		return nil, fmt.Errorf("list next alert cycles: %w", err)
	}

	eligible := make([]NextAlertCycleRow, 0, len(rows))
	for _, row := range rows {
		if !IsNextAlertOpenAtEligible(todayHCM, row.OpenAt) {
			continue
		}
		eligible = append(eligible, row)
	}
	nearest := SelectNearestNextAlertPerType(eligible)
	items := make([]NextDeadlineAlertDTO, 0, len(nearest))
	for _, row := range nearest {
		items = append(items, NextDeadlineAlertDTO{
			CycleID:       strings.TrimSpace(row.CycleID),
			TypeID:        strings.TrimSpace(row.TypeID),
			TypeName:      strings.TrimSpace(row.TypeName),
			FrequencyUnit: strings.TrimSpace(row.FrequencyUnit),
			CycleLabel:    strings.TrimSpace(row.CycleLabel),
			CycleStart:    strings.TrimSpace(row.CycleStart),
			OpenAt:        strings.TrimSpace(row.OpenAt),
			DueAt:         strings.TrimSpace(row.DueAt),
			State:         NextDeadlineAlertStateUpcoming,
		})
	}
	if items == nil {
		items = []NextDeadlineAlertDTO{}
	}
	return &ListNextDeadlineAlertsResponse{Items: items}, nil
}

func businessDateHCMService(now time.Time) string {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		loc = time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)
	}
	n := now.In(loc)
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, loc).Format("2006-01-02")
}
