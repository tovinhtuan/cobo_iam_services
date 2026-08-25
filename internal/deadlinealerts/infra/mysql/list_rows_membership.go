package mysql

import (
	"strings"
	"time"
)

// listRowsV1ObligationMembershipSQL is the Phase-1 V1 membership AND-clauses
// after `dr.company_id = ?`.
//
// NeedsCompanyAction: Draft + submitted_at IS NULL.
// Periodic AlertWindow: EXISTS cycle with COALESCE(open_at, cycle_start) <= :todayHCM
//   (open_at NULL → cycle_start = LEGACY_COMPATIBILITY_FALLBACK only).
// Irregular: NOT EXISTS cycle → no OpenAt gate.
// Malformed cycle (both dates NULL): EXISTS branch fails → excluded.
//
// Requires one bind argument: todayHCM as YYYY-MM-DD (Asia/Ho_Chi_Minh business date).
// EXISTS (not JOIN) avoids row duplication when multiple cycles share a record_id.
const listRowsV1ObligationMembershipSQL = `
		  AND LOWER(TRIM(dr.status)) = 'draft'
		  AND dr.submitted_at IS NULL
		  AND (
		    NOT EXISTS (
		      SELECT 1 FROM periodic_cycles pc_ir
		      WHERE pc_ir.record_id = dr.record_id
		    )
		    OR EXISTS (
		      SELECT 1 FROM periodic_cycles pc
		      WHERE pc.record_id = dr.record_id
		        AND COALESCE(pc.open_at, pc.cycle_start) IS NOT NULL
		        AND COALESCE(pc.open_at, pc.cycle_start) <= ?
		    )
		  )`

// businessDateHCM returns the Asia/Ho_Chi_Minh calendar date for now (YYYY-MM-DD).
// Matches deadlinealerts service calculatorLocation convention; no DB session TZ.
func businessDateHCM(now time.Time) string {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		loc = time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)
	}
	n := now.In(loc)
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, loc).Format("2006-01-02")
}

// periodicCycleDates models one periodic_cycles row linked by record_id.
// openAt / cycleStart are calendar dates (DATE columns); nil means SQL NULL.
type periodicCycleDates struct {
	openAt     *time.Time
	cycleStart *time.Time
}

// listRowsV1MembershipEligible mirrors listRowsV1ObligationMembershipSQL for
// deterministic repository-boundary tests (same package). Keep in sync with SQL.
// todayHCM must be a calendar date in Asia/Ho_Chi_Minh (time-of-day ignored).
func listRowsV1MembershipEligible(
	status string,
	submittedAtNull bool,
	cycles []periodicCycleDates,
	todayHCM time.Time,
) bool {
	if strings.ToLower(strings.TrimSpace(status)) != "draft" {
		return false
	}
	if !submittedAtNull {
		return false
	}
	if len(cycles) == 0 {
		return true // irregular: no OpenAt gate
	}
	today := truncateDate(todayHCM)
	for _, c := range cycles {
		alertFrom := coalesceDate(c.openAt, c.cycleStart)
		if alertFrom == nil {
			continue // malformed row: both NULL → this cycle does not pass EXISTS
		}
		if !truncateDate(*alertFrom).After(today) {
			return true // COALESCE(...) <= todayHCM
		}
	}
	return false
}

func coalesceDate(a, b *time.Time) *time.Time {
	if a != nil {
		return a
	}
	return b
}

func truncateDate(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
