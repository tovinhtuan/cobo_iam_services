package app

import "time"

// Divergence classes for a Source B (legacy, periodic.go) vs Source C
// (deadlineengine, locked Cycle Start SoT) cycle_start comparison — see
// deadline-engine-cycle-start-sot-review-2026-06-12.md Section 3 and
// deadline-engine-rk-e09-dev-closeout-2026-06-12.md Section 7.
const (
	// DivergenceNone: Source B and Source C produce the same cycle_start.
	DivergenceNone = "NONE"
	// DivergenceDateShift: same month/year, different day (e.g. monthly
	// templates with cycle_anchor_day != 1).
	DivergenceDateShift = "DATE_SHIFT"
	// DivergenceMonthShift: same year, different month (e.g. quarterly
	// templates with cycle_anchor_month != 1 — fiscal vs calendar quarter).
	DivergenceMonthShift = "MONTH_SHIFT"
	// DivergenceYearlyRollback: different year — Source C's past-occurrence
	// rollback identifies a different cycle altogether (RK-E09).
	DivergenceYearlyRollback = "YEARLY_ROLLBACK"
)

// DeadlineComparison is the Batch 5D dual-compute result for a single
// (company, type) periodic pair: Source B's persisted/legacy cycle_start and
// due_date vs Source C's (deadlineengine, locked SoT) recomputation.
//
// Batch 5A note: this type is a contract only. Nothing in this batch computes,
// stores, publishes, or emits a DeadlineComparison/DeadlineAudit — that is
// Batch 5D's scope, gated behind DEADLINE_ENGINE_V2.
type DeadlineComparison struct {
	OldCycleStart time.Time
	NewCycleStart time.Time

	OldDueDate time.Time
	NewDueDate time.Time

	DivergenceClass string
}

// NewDeadlineComparison builds a DeadlineComparison and classifies the
// cycle_start divergence via ClassifyDivergence.
func NewDeadlineComparison(oldCycleStart, newCycleStart, oldDueDate, newDueDate time.Time) DeadlineComparison {
	return DeadlineComparison{
		OldCycleStart:   oldCycleStart,
		NewCycleStart:   newCycleStart,
		OldDueDate:      oldDueDate,
		NewDueDate:      newDueDate,
		DivergenceClass: ClassifyDivergence(oldCycleStart, newCycleStart),
	}
}

// DeadlineAudit is the Batch 5D audit record shape for a single
// (company_id, type_id) pair — a DeadlineComparison plus its identity. Not
// persisted by Batch 5A (no DB write, no event, no metric — structure only).
type DeadlineAudit struct {
	CompanyID string
	TypeID    string

	OldCycleStart time.Time
	NewCycleStart time.Time

	OldDueDate time.Time
	NewDueDate time.Time

	DivergenceClass string
}

// NewDeadlineAudit builds a DeadlineAudit for (companyID, typeID) from a
// DeadlineComparison.
func NewDeadlineAudit(companyID, typeID string, comparison DeadlineComparison) DeadlineAudit {
	return DeadlineAudit{
		CompanyID:       companyID,
		TypeID:          typeID,
		OldCycleStart:   comparison.OldCycleStart,
		NewCycleStart:   comparison.NewCycleStart,
		OldDueDate:      comparison.OldDueDate,
		NewDueDate:      comparison.NewDueDate,
		DivergenceClass: comparison.DivergenceClass,
	}
}

// ClassifyDivergence classifies the divergence between a Source B
// (oldCycleStart) and Source C (newCycleStart) cycle_start.
//
//   - Equal dates                -> DivergenceNone
//   - Different year             -> DivergenceYearlyRollback (RK-E09: a
//     different cycle altogether, not a date shift — see SoT review §3)
//   - Same year, different month -> DivergenceMonthShift (quarterly fiscal
//     vs calendar quarter)
//   - Same year/month, different day -> DivergenceDateShift (monthly
//     anchor_day != 1)
func ClassifyDivergence(oldCycleStart, newCycleStart time.Time) string {
	if oldCycleStart.Equal(newCycleStart) {
		return DivergenceNone
	}
	if oldCycleStart.Year() != newCycleStart.Year() {
		return DivergenceYearlyRollback
	}
	if oldCycleStart.Month() != newCycleStart.Month() {
		return DivergenceMonthShift
	}
	return DivergenceDateShift
}
