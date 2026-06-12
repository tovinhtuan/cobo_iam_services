package app

import (
	"testing"
	"time"
)

func TestClassifyDivergence_None(t *testing.T) {
	d := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	if got := ClassifyDivergence(d, d); got != DivergenceNone {
		t.Fatalf("expected %s, got %s", DivergenceNone, got)
	}
}

func TestClassifyDivergence_DateShift(t *testing.T) {
	old := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	newT := time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC)
	if got := ClassifyDivergence(old, newT); got != DivergenceDateShift {
		t.Fatalf("expected %s, got %s", DivergenceDateShift, got)
	}
}

func TestClassifyDivergence_MonthShift(t *testing.T) {
	old := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	newT := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	if got := ClassifyDivergence(old, newT); got != DivergenceMonthShift {
		t.Fatalf("expected %s, got %s", DivergenceMonthShift, got)
	}
}

func TestClassifyDivergence_YearlyRollback(t *testing.T) {
	// RK-E09: Source B seeds the future current-year anchor date, Source C
	// (past-occurrence rollback) identifies the already-started previous-year cycle.
	old := time.Date(2026, 4, 4, 0, 0, 0, 0, time.UTC)  // Source B
	newT := time.Date(2025, 4, 4, 0, 0, 0, 0, time.UTC) // Source C
	if got := ClassifyDivergence(old, newT); got != DivergenceYearlyRollback {
		t.Fatalf("expected %s, got %s", DivergenceYearlyRollback, got)
	}
}

func TestNewDeadlineComparison(t *testing.T) {
	oldStart := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	newStart := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	oldDue := time.Date(2026, 5, 12, 0, 0, 0, 0, time.UTC)
	newDue := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)

	cmp := NewDeadlineComparison(oldStart, newStart, oldDue, newDue)

	if !cmp.OldCycleStart.Equal(oldStart) || !cmp.NewCycleStart.Equal(newStart) {
		t.Fatalf("cycle_start fields not preserved: %#v", cmp)
	}
	if !cmp.OldDueDate.Equal(oldDue) || !cmp.NewDueDate.Equal(newDue) {
		t.Fatalf("due_date fields not preserved: %#v", cmp)
	}
	if cmp.DivergenceClass != DivergenceMonthShift {
		t.Fatalf("expected %s, got %s", DivergenceMonthShift, cmp.DivergenceClass)
	}
}

func TestNewDeadlineAudit(t *testing.T) {
	oldStart := time.Date(2026, 4, 4, 0, 0, 0, 0, time.UTC)
	newStart := time.Date(2025, 4, 4, 0, 0, 0, 0, time.UTC)
	cmp := NewDeadlineComparison(oldStart, newStart, oldStart, newStart)

	audit := NewDeadlineAudit("c_001", "bao-cao-tai-chinh-nam", cmp)

	if audit.CompanyID != "c_001" || audit.TypeID != "bao-cao-tai-chinh-nam" {
		t.Fatalf("identity fields not set: %#v", audit)
	}
	if !audit.OldCycleStart.Equal(oldStart) || !audit.NewCycleStart.Equal(newStart) {
		t.Fatalf("cycle_start fields not propagated: %#v", audit)
	}
	if audit.DivergenceClass != DivergenceYearlyRollback {
		t.Fatalf("expected %s, got %s", DivergenceYearlyRollback, audit.DivergenceClass)
	}
}
