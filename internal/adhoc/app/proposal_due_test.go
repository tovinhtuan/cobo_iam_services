package app

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/cobo/cobo_iam_services/internal/disclosure/app/deadlineengine"
)

func TestFormatProposalDueDate_AbsolutePrecedence(t *testing.T) {
	t0 := time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC) // Monday
	final := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	proposedAbs := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)
	work := ProposalDeadlineDayTypeWorkingDays

	got, err := FormatProposalDueDate(context.Background(), ProposalDueInput{
		FinalDeadlineDate:    sql.NullTime{Valid: true, Time: final},
		ProposedT0Date:       sql.NullTime{Valid: true, Time: t0},
		ProposedDeadlineDays: sql.NullInt64{Valid: true, Int64: 5},
		DayType:              &work,
	}, nil)
	if err != nil || got != "2026-05-01" {
		t.Fatalf("final wins: got %q err=%v", got, err)
	}

	got, err = FormatProposalDueDate(context.Background(), ProposalDueInput{
		ProposedT0Date:       sql.NullTime{Valid: true, Time: t0},
		ProposedDeadlineDays: sql.NullInt64{Valid: true, Int64: 5},
		ProposedDeadlineDate: sql.NullTime{Valid: true, Time: proposedAbs},
		DayType:              &work,
	}, nil)
	// T0+days takes precedence over proposed absolute when days>0 (legacy order).
	if err != nil || got != "2026-04-13" { // Mon+5 calendar... wait WORKING: Mon+5 working = next Mon 04-13
		t.Fatalf("T0+days before proposed abs: got %q err=%v want working Mon+5=2026-04-13", got, err)
	}

	got, err = FormatProposalDueDate(context.Background(), ProposalDueInput{
		ProposedDeadlineDate: sql.NullTime{Valid: true, Time: proposedAbs},
		DayType:              &work,
	}, nil)
	if err != nil || got != "2026-04-20" {
		t.Fatalf("absolute when no days: got %q err=%v", got, err)
	}
}

func TestFormatProposalDueDate_CalendarMatchesLegacyAddDate(t *testing.T) {
	// Monday 2026-04-06 + 5 calendar days = Saturday 2026-04-11 (legacy AddDate).
	t0 := time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC)
	cal := ProposalDeadlineDayTypeCalendarDays
	got, err := FormatProposalDueDate(context.Background(), ProposalDueInput{
		ProposedT0Date:       sql.NullTime{Valid: true, Time: t0},
		ProposedDeadlineDays: sql.NullInt64{Valid: true, Int64: 5},
		DayType:              &cal,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := t0.AddDate(0, 0, 5).Format("2006-01-02")
	if got != want || got != "2026-04-11" {
		t.Fatalf("got %q want %q", got, want)
	}

	// nil day type == calendar
	got, err = FormatProposalDueDate(context.Background(), ProposalDueInput{
		ProposedT0Date:       sql.NullTime{Valid: true, Time: t0},
		ProposedDeadlineDays: sql.NullInt64{Valid: true, Int64: 5},
	}, nil)
	if err != nil || got != "2026-04-11" {
		t.Fatalf("legacy nil: got %q err=%v", got, err)
	}
}

func TestFormatProposalDueDate_WorkingSkipsWeekend(t *testing.T) {
	// Friday 2026-04-03 + 1 working day after = Monday 2026-04-06
	fri := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	work := ProposalDeadlineDayTypeWorkingDays
	got, err := FormatProposalDueDate(context.Background(), ProposalDueInput{
		ProposedT0Date:       sql.NullTime{Valid: true, Time: fri},
		ProposedDeadlineDays: sql.NullInt64{Valid: true, Int64: 1},
		DayType:              &work,
	}, nil)
	if err != nil || got != "2026-04-06" {
		t.Fatalf("Fri+1 working: got %q err=%v", got, err)
	}

	// Friday + 2 working = Tuesday 2026-04-07
	got, err = FormatProposalDueDate(context.Background(), ProposalDueInput{
		ProposedT0Date:       sql.NullTime{Valid: true, Time: fri},
		ProposedDeadlineDays: sql.NullInt64{Valid: true, Int64: 2},
		DayType:              &work,
	}, nil)
	if err != nil || got != "2026-04-07" {
		t.Fatalf("Fri+2 working: got %q err=%v", got, err)
	}

	// Monday + 5 working = next Monday 2026-04-13
	mon := time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC)
	got, err = FormatProposalDueDate(context.Background(), ProposalDueInput{
		ProposedT0Date:       sql.NullTime{Valid: true, Time: mon},
		ProposedDeadlineDays: sql.NullInt64{Valid: true, Int64: 5},
		DayType:              &work,
	}, nil)
	if err != nil || got != "2026-04-13" {
		t.Fatalf("Mon+5 working: got %q err=%v", got, err)
	}
}

func TestFormatProposalDueDate_WorkingUsesHolidayChecker(t *testing.T) {
	// Monday 2026-04-06; treat Tue 2026-04-07 as holiday; +1 working → Wed 2026-04-08
	mon := time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC)
	work := ProposalDeadlineDayTypeWorkingDays
	holidays := deadlineengine.IsHolidayFunc(func(_ context.Context, date time.Time) (bool, error) {
		return date.Format("2006-01-02") == "2026-04-07", nil
	})
	got, err := FormatProposalDueDate(context.Background(), ProposalDueInput{
		ProposedT0Date:       sql.NullTime{Valid: true, Time: mon},
		ProposedDeadlineDays: sql.NullInt64{Valid: true, Int64: 1},
		DayType:              &work,
	}, holidays)
	if err != nil || got != "2026-04-08" {
		t.Fatalf("holiday skip: got %q err=%v", got, err)
	}
}

func TestResolveProposalDeadlineDayTypeForDue_Corrupted(t *testing.T) {
	bad := ProposalDeadlineDayType("FOO")
	_, err := ResolveProposalDeadlineDayTypeForDue(&bad)
	if !errors.Is(err, ErrInvalidProposalDeadlineDayType) {
		t.Fatalf("expected ErrInvalidProposalDeadlineDayType, got %v", err)
	}
}

func TestFormatProposalDueDate_CorruptedDayType(t *testing.T) {
	bad := ProposalDeadlineDayType("FOO")
	_, err := FormatProposalDueDate(context.Background(), ProposalDueInput{
		ProposedT0Date:       sql.NullTime{Valid: true, Time: time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC)},
		ProposedDeadlineDays: sql.NullInt64{Valid: true, Int64: 1},
		DayType:              &bad,
	}, nil)
	if !errors.Is(err, ErrInvalidProposalDeadlineDayType) {
		t.Fatalf("expected invalid day type, got %v", err)
	}
}
