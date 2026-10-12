package mysql

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	mysqldrv "github.com/go-sql-driver/mysql"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// noCalendarQueries answers the in-transaction reads as "the year has no calendar yet".
func noCalendarQueries(q string, _ []driver.Value) (driver.Rows, error) {
	switch {
	case strings.Contains(q, "FROM holiday_calendars"):
		return &captureRows{cols: []string{"rev", "total_days"}}, nil
	case strings.Contains(q, "FROM holiday_dates"):
		return &captureRows{cols: []string{"holiday_date"}}, nil
	}
	return nil, errors.New("unexpected query")
}

// existingCalendar answers as "the year has a calendar with revision rev and these dates".
func existingCalendar(rev string, dates ...string) func(string, []driver.Value) (driver.Rows, error) {
	return func(q string, _ []driver.Value) (driver.Rows, error) {
		switch {
		case strings.Contains(q, "FROM holiday_calendars"):
			return &captureRows{cols: []string{"rev", "total_days"}, vals: [][]driver.Value{{rev, int64(len(dates))}}}, nil
		case strings.Contains(q, "FROM holiday_dates"):
			rows := &captureRows{cols: []string{"holiday_date"}}
			for _, d := range dates {
				day, _ := time.Parse("2006-01-02", d)
				rows.vals = append(rows.vals, []driver.Value{day})
			}
			return rows, nil
		}
		return nil, errors.New("unexpected query")
	}
}

func execIndex(d *captureDriver, fragment string) int {
	for i, q := range d.query {
		if strings.Contains(q, fragment) {
			return i
		}
	}
	return -1
}

func day(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}

// The DEV schema lacks the holiday_dates -> holiday_calendars cascade, so the dates must be
// deleted explicitly before re-inserting (otherwise uk_holiday_year_date fails).
func TestReplaceCalendar_DeletesDatesExplicitlyBeforeInsert(t *testing.T) {
	db, d := openCaptureDB(t)
	d.queryFn = existingCalendar("rev-old", "2026-09-02")
	_, err := NewRepository(db).ReplaceCalendar(context.Background(), ReplaceCalendarInput{
		CalendarID: "cal-2", CalendarYear: 2026, UploadedBy: "u",
		Dates: []HolidayDateRow{{Date: day("2026-09-02"), DayType: "PUBLIC_HOLIDAY"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	delDates := execIndex(d, "DELETE FROM holiday_dates")
	delCal := execIndex(d, "DELETE FROM holiday_calendars")
	insDates := execIndex(d, "INSERT INTO holiday_dates")
	if delDates < 0 || delCal < 0 || insDates < 0 || !(delDates < insDates && delCal < insDates) {
		t.Fatalf("expected explicit holiday_dates delete before insert, got %v", d.query)
	}
}

func TestReplaceCalendar_StaleRevisionConflicts(t *testing.T) {
	db, d := openCaptureDB(t)
	d.queryFn = existingCalendar("rev-current", "2026-01-01")
	stale := "rev-old"
	_, err := NewRepository(db).ReplaceCalendar(context.Background(), ReplaceCalendarInput{
		CalendarID: "cal-3", CalendarYear: 2026, UploadedBy: "u", ExpectedRevision: &stale,
		Dates: []HolidayDateRow{{Date: day("2026-01-01"), DayType: "PUBLIC_HOLIDAY"}},
	})
	var he *perr.HTTPError
	if !errors.As(err, &he) || he.HTTPStatus != 409 || he.Code != perr.CodeStaleEtag {
		t.Fatalf("want 409 STALE_ETAG, got %v", err)
	}
	if he.Details["current_revision"] != "rev-current" {
		t.Fatalf("current_revision = %v", he.Details["current_revision"])
	}
	if len(d.execs) != 0 || d.commits != 0 {
		t.Fatalf("stale replace must not write: execs=%d commits=%d", len(d.execs), d.commits)
	}
}

func TestReplaceCalendar_EmptyExpectedRevisionRequiresNoCalendar(t *testing.T) {
	db, d := openCaptureDB(t)
	d.queryFn = existingCalendar("rev-current")
	none := ""
	_, err := NewRepository(db).ReplaceCalendar(context.Background(), ReplaceCalendarInput{
		CalendarID: "cal-4", CalendarYear: 2026, UploadedBy: "u", ExpectedRevision: &none,
	})
	var he *perr.HTTPError
	if !errors.As(err, &he) || he.HTTPStatus != 409 {
		t.Fatalf("want 409, got %v", err)
	}
}

func TestReplaceCalendar_AuditInSameTransactionWithDiff(t *testing.T) {
	db, d := openCaptureDB(t)
	d.queryFn = existingCalendar("rev-1", "2026-01-01", "2026-09-02")
	match := "rev-1"
	res, err := NewRepository(db).ReplaceCalendar(context.Background(), ReplaceCalendarInput{
		CalendarID: "cal-5", CalendarYear: 2026, UploadedBy: "u", ExpectedRevision: &match,
		Dates: []HolidayDateRow{
			{Date: day("2026-01-01"), DayType: "PUBLIC_HOLIDAY"},
			{Date: day("2026-02-16"), DayType: "PUBLIC_HOLIDAY"},
			{Date: day("2026-02-17"), DayType: "PUBLIC_HOLIDAY"},
		},
		Audit: &AuditEntry{EventID: "ev-1", ActorUserID: "u", CompanyID: "c", Action: "cms.holiday_calendar.replace",
			Metadata: map[string]any{"source": "editor"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Added != 2 || res.Removed != 1 {
		t.Fatalf("added/removed = %d/%d, want 2/1", res.Added, res.Removed)
	}
	i := execIndex(d, "INSERT INTO audit_logs")
	if i < 0 || d.commits != 1 {
		t.Fatalf("audit insert missing or not committed once: idx=%d commits=%d", i, d.commits)
	}
	var meta map[string]any
	if err := json.Unmarshal([]byte(d.execs[i][len(d.execs[i])-1].(string)), &meta); err != nil {
		t.Fatal(err)
	}
	if meta["added"] != float64(2) || meta["removed"] != float64(1) || meta["source"] != "editor" {
		t.Fatalf("audit metadata = %v", meta)
	}
	if d.execs[i][6] != "2026" {
		t.Fatalf("resource_id = %v, want year", d.execs[i][6])
	}
}

func TestDeleteCalendar_NotFoundWhenNoCalendar(t *testing.T) {
	db, d := openCaptureDB(t)
	d.queryFn = noCalendarQueries
	_, err := NewRepository(db).DeleteCalendar(context.Background(), DeleteCalendarInput{CalendarYear: 2026})
	var he *perr.HTTPError
	if !errors.As(err, &he) || he.HTTPStatus != 404 {
		t.Fatalf("want 404, got %v", err)
	}
	if len(d.execs) != 0 {
		t.Fatalf("no writes expected, got %v", d.query)
	}
}

func TestDeleteCalendar_DeletesAndAudits(t *testing.T) {
	db, d := openCaptureDB(t)
	d.queryFn = existingCalendar("rev-1", "2026-01-01", "2026-09-02")
	total, err := NewRepository(db).DeleteCalendar(context.Background(), DeleteCalendarInput{
		CalendarYear: 2026,
		Audit:        &AuditEntry{EventID: "ev-2", Action: "cms.holiday_calendar.delete", Metadata: map[string]any{"reason": "x"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("total = %d", total)
	}
	if execIndex(d, "DELETE FROM holiday_dates") < 0 || execIndex(d, "DELETE FROM holiday_calendars") < 0 || execIndex(d, "INSERT INTO audit_logs") < 0 || d.commits != 1 {
		t.Fatalf("unexpected statements %v commits=%d", d.query, d.commits)
	}
}

func TestConcurrentWrite_MapsDeadlockAndDuplicateTo409(t *testing.T) {
	for _, n := range []uint16{1213, 1062} {
		if !concurrentWrite(fmt.Errorf("insert holiday_calendars: %w", &mysqldrv.MySQLError{Number: n})) {
			t.Fatalf("%d not treated as a concurrent write", n)
		}
	}
	if concurrentWrite(&mysqldrv.MySQLError{Number: 1406}) || concurrentWrite(errors.New("x")) {
		t.Fatal("unrelated errors must stay 500")
	}
	var he *perr.HTTPError
	if !errors.As(staleConflict(""), &he) || he.HTTPStatus != 409 || he.Code != perr.CodeStaleEtag {
		t.Fatalf("staleConflict = %v", he)
	}
}

func TestInsertAudit_TruncatesRequestID(t *testing.T) {
	db, d := openCaptureDB(t)
	d.queryFn = noCalendarQueries
	_, err := NewRepository(db).ReplaceCalendar(context.Background(), ReplaceCalendarInput{
		CalendarID: "cal-6", CalendarYear: 2026, UploadedBy: "u",
		Dates: []HolidayDateRow{{Date: day("2026-01-01"), DayType: "PUBLIC_HOLIDAY"}},
		Audit: &AuditEntry{EventID: "ev", Action: "a", RequestID: strings.Repeat("r", 200)},
	})
	if err != nil {
		t.Fatal(err)
	}
	i := execIndex(d, "INSERT INTO audit_logs")
	if got := d.execs[i][7].(string); len(got) != 64 {
		t.Fatalf("request_id length = %d", len(got))
	}
}
