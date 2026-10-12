package mysql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// captureDriver is a minimal database/sql driver that records Exec arguments.
type captureDriver struct {
	mu      sync.Mutex
	execs   [][]driver.Value
	query   []string
	queries int
	// queryOK makes holiday_calendars / holiday_dates queries succeed with no calendar rows.
	queryOK         bool
	calendarQueries int
	dateQueries     int
	// queryFn, when set, answers every query (used by the transactional replace/delete tests).
	queryFn func(q string, args []driver.Value) (driver.Rows, error)
	commits int
}

func (d *captureDriver) Open(string) (driver.Conn, error) { return &captureConn{d: d}, nil }

type captureConn struct{ d *captureDriver }

func (c *captureConn) Prepare(q string) (driver.Stmt, error) { return &captureStmt{c: c, q: q}, nil }
func (c *captureConn) Close() error                          { return nil }
func (c *captureConn) Begin() (driver.Tx, error)             { return captureTx{d: c.d}, nil }

type captureTx struct{ d *captureDriver }

func (t captureTx) Commit() error {
	t.d.mu.Lock()
	t.d.commits++
	t.d.mu.Unlock()
	return nil
}
func (captureTx) Rollback() error { return nil }

type captureStmt struct {
	c *captureConn
	q string
}

func (s *captureStmt) Close() error  { return nil }
func (s *captureStmt) NumInput() int { return -1 }
func (s *captureStmt) Exec(args []driver.Value) (driver.Result, error) {
	s.c.d.mu.Lock()
	defer s.c.d.mu.Unlock()
	s.c.d.query = append(s.c.d.query, s.q)
	s.c.d.execs = append(s.c.d.execs, args)
	return driver.RowsAffected(1), nil
}
func (s *captureStmt) Query(args []driver.Value) (driver.Rows, error) {
	s.c.d.mu.Lock()
	defer s.c.d.mu.Unlock()
	s.c.d.queries++
	if s.c.d.queryFn != nil {
		return s.c.d.queryFn(s.q, args)
	}
	if s.c.d.queryOK {
		switch {
		case strings.Contains(s.q, "FROM holiday_calendars"):
			s.c.d.calendarQueries++
			return &captureRows{cols: []string{"n"}, vals: [][]driver.Value{{int64(0)}}}, nil
		case strings.Contains(s.q, "FROM holiday_dates"):
			s.c.d.dateQueries++
			return &captureRows{cols: []string{"holiday_date", "day_type", "name"}}, nil
		}
	}
	return nil, errors.New("db unavailable")
}

type captureRows struct {
	cols []string
	vals [][]driver.Value
	i    int
}

func (r *captureRows) Columns() []string { return r.cols }
func (r *captureRows) Close() error      { return nil }
func (r *captureRows) Next(dest []driver.Value) error {
	if r.i >= len(r.vals) {
		return io.EOF
	}
	copy(dest, r.vals[r.i])
	r.i++
	return nil
}

var captureDrivers sync.Map

// openCaptureDB registers a fresh capture driver per test and opens it.
func openCaptureDB(t *testing.T) (*sql.DB, *captureDriver) {
	t.Helper()
	d := &captureDriver{}
	name := "holiday-capture-" + t.Name()
	if _, loaded := captureDrivers.LoadOrStore(name, d); loaded {
		t.Fatalf("driver %s registered twice", name)
	}
	sql.Register(name, d)
	db, err := sql.Open(name, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, d
}

// DUE-01: holiday_date is a DATE. Binding a time.Time at 00:00 Asia/Ho_Chi_Minh is converted by
// the driver to UTC (17:00 the previous day) and MySQL truncates it to the previous day, so the
// civil date must be bound as YYYY-MM-DD.
func TestReplaceCalendar_BindsCivilDate(t *testing.T) {
	hcm, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		t.Skip("tzdata unavailable")
	}
	db, d := openCaptureDB(t)
	d.queryFn = noCalendarQueries

	want := []string{"2027-01-01", "2026-02-16", "2026-12-31"}
	var rows []HolidayDateRow
	for _, s := range want {
		day, _ := time.ParseInLocation("2006-01-02", s, hcm)
		rows = append(rows, HolidayDateRow{Date: day, DayType: "PUBLIC_HOLIDAY", Name: "x"})
	}
	if _, err := NewRepository(db).ReplaceCalendar(context.Background(), ReplaceCalendarInput{
		CalendarID: "cal-1", CalendarYear: 2027, UploadedBy: "u", Dates: rows,
	}); err != nil {
		t.Fatal(err)
	}
	var got []string
	for i, q := range d.query {
		if !strings.Contains(q, "INSERT INTO holiday_dates") {
			continue
		}
		switch v := d.execs[i][2].(type) {
		case string:
			got = append(got, v)
		case time.Time:
			got = append(got, v.UTC().Format("2006-01-02"))
		default:
			t.Fatalf("unexpected holiday_date arg type %T", v)
		}
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("holiday_date bound as %v, want %v (civil dates)", got, want)
	}
}

// DUE-01/DUE-12: a transient load error must not be cached, otherwise the year stays broken
// until the process restarts.
func TestDBProvider_DoesNotCacheLoadErrors(t *testing.T) {
	db, d := openCaptureDB(t)
	p := NewDBProvider(NewRepository(db))
	day := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 2; i++ {
		if _, _, err := p.IsNonTradingDay(context.Background(), day); err == nil {
			t.Fatal("expected load error")
		}
	}
	if d.queries != 2 {
		t.Fatalf("load error was cached: %d queries for 2 lookups", d.queries)
	}
}
