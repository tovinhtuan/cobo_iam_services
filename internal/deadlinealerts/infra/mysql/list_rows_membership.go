package mysql

import (
	"strings"
	"time"

	deadlinealertsapp "github.com/cobo/cobo_iam_services/internal/deadlinealerts/app"
)

// adHocDeadlineKnownSQL keeps, among the open ad-hoc rows, those whose approved proposal carries a
// deadline input (the sources of adhoc FormatProposalDueDate). The alerts list only shows
// obligations with a due date (contract v1.2 §5.2) and its count must match it; rows past Draft
// that are ObligationOpen are only the ad-hoc ones.
const adHocDeadlineKnownSQL = `
		  AND (
		    LOWER(TRIM(dr.status)) = 'draft'
		    OR EXISTS (
		      SELECT 1 FROM ad_hoc_proposals ahp_due
		      WHERE ahp_due.company_id = dr.company_id
		        AND ahp_due.record_id = dr.record_id
		        AND ahp_due.status = 'approved'
		        AND (
		          ahp_due.final_deadline_date IS NOT NULL
		          OR (ahp_due.proposed_t0_date IS NOT NULL AND ahp_due.proposed_deadline_days > 0)
		          OR ahp_due.proposed_deadline_date >= '2000-01-01'
		        )
		    )
		  )`

// listRowsV1ObligationMembershipSQL is the shared ObligationOpen predicate (deadlinealerts/app
// ObligationOpenSQL) plus adHocDeadlineKnownSQL, after `dr.company_id = ?`. One bind argument:
// todayHCM.
const listRowsV1ObligationMembershipSQL = deadlinealertsapp.ObligationOpenSQL + adHocDeadlineKnownSQL

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
// approvedAdHoc: the record is linked to an approved ad-hoc proposal; adHocDeadlineKnown: that
// proposal carries a deadline input.
func listRowsV1MembershipEligible(
	status string,
	submittedAtNull bool,
	approvedAdHoc bool,
	adHocDeadlineKnown bool,
	cycles []periodicCycleDates,
	todayHCM time.Time,
) bool {
	if approvedAdHoc && !strings.EqualFold(strings.TrimSpace(status), "draft") {
		switch strings.ToLower(strings.TrimSpace(status)) {
		case "approved", "published", "completed", "done":
			return false
		default:
			// ad-hoc open from approval until its workflow completes (AC-11), listed with a due date
			return adHocDeadlineKnown
		}
	}
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
