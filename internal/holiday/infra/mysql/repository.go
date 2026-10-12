package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	mysqldrv "github.com/go-sql-driver/mysql"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// Repository persists holiday calendars and dates.
type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// HasCalendarForYear returns true if a calendar row exists for the civil year.
func (r *Repository) HasCalendarForYear(ctx context.Context, year int) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(1) FROM holiday_calendars WHERE calendar_year = ?
	`, year).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("holiday_calendars exists: %w", err)
	}
	return n > 0, nil
}

// GetHolidayMapForYear returns date -> reason strings for IsNonTradingDay (reason prefers name, else day_type).
func (r *Repository) GetHolidayMapForYear(ctx context.Context, year int) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT holiday_date, day_type, name
		FROM holiday_dates
		WHERE calendar_year = ?
		ORDER BY holiday_date
	`, year)
	if err != nil {
		return nil, fmt.Errorf("holiday_dates query: %w", err)
	}
	defer rows.Close()

	out := make(map[string]string)
	for rows.Next() {
		var d time.Time
		var dayType, name sql.NullString
		if err := rows.Scan(&d, &dayType, &name); err != nil {
			return nil, err
		}
		key := d.Format("2006-01-02")
		reason := "PUBLIC_HOLIDAY"
		if dayType.Valid && strings.TrimSpace(dayType.String) != "" {
			reason = strings.TrimSpace(dayType.String)
		}
		if name.Valid && strings.TrimSpace(name.String) != "" {
			reason = strings.TrimSpace(name.String)
		}
		out[key] = reason
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// AuditEntry is an audit_logs row written in the same transaction as a calendar change, so a
// committed change always has its event and a rolled-back one never does.
type AuditEntry struct {
	EventID           string
	OccurredAt        time.Time
	ActorUserID       string
	ActorMembershipID string
	CompanyID         string
	Action            string
	RequestID         string
	IP                string
	UserAgent         string
	Metadata          map[string]any
}

// ReplaceResult reports the change against the calendar that was replaced.
type ReplaceResult struct {
	Added   int
	Removed int
}

// lockRevision locks the year's calendar row (if any) and checks the caller's expected revision.
// expected == nil skips the check; "" expects no calendar. Returns whether a calendar exists,
// its revision and total_days.
func lockRevision(ctx context.Context, tx *sql.Tx, year int, expected *string) (bool, string, int, error) {
	var rev string
	var total int
	err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(NULLIF(content_sha256, ''), calendar_id), total_days
		FROM holiday_calendars WHERE calendar_year = ? FOR UPDATE
	`, year).Scan(&rev, &total)
	exists := true
	if errors.Is(err, sql.ErrNoRows) {
		exists, rev, total, err = false, "", 0, nil
	}
	if err != nil {
		return false, "", 0, fmt.Errorf("lock holiday_calendars: %w", err)
	}
	if expected != nil && strings.TrimSpace(*expected) != rev {
		return false, "", 0, staleConflict(rev)
	}
	return exists, rev, total, nil
}

func staleConflict(current string) error {
	he := perr.NewHTTPError(http.StatusConflict, perr.CodeStaleEtag, "holiday calendar was changed by someone else; reload and try again", nil)
	he.Details = map[string]any{"current_revision": current}
	return he
}

// concurrentWrite reports the errors two first-time saves of the same year race into: under
// REPEATABLE READ both take gap locks and one INSERT deadlocks (1213), or loses on the unique
// year key (1062). The winner's calendar stands, so the loser gets 409 instead of 500.
func concurrentWrite(err error) bool {
	var me *mysqldrv.MySQLError
	return errors.As(err, &me) && (me.Number == 1213 || me.Number == 1062)
}

func storedDateSet(ctx context.Context, tx *sql.Tx, year int) (map[string]struct{}, error) {
	rows, err := tx.QueryContext(ctx, `SELECT holiday_date FROM holiday_dates WHERE calendar_year = ?`, year)
	if err != nil {
		return nil, fmt.Errorf("holiday_dates query: %w", err)
	}
	defer rows.Close()
	out := map[string]struct{}{}
	for rows.Next() {
		var d time.Time
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		out[d.Format("2006-01-02")] = struct{}{}
	}
	return out, rows.Err()
}

// deleteYear removes the year's dates and calendar. holiday_dates is deleted explicitly:
// some environments lack the ON DELETE CASCADE foreign key (schema drift found on DEV
// 2026-10-12), and the stale dates then collide with uk_holiday_year_date on re-insert.
func deleteYear(ctx context.Context, tx *sql.Tx, year int) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM holiday_dates WHERE calendar_year = ?`, year); err != nil {
		return fmt.Errorf("delete holiday_dates: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM holiday_calendars WHERE calendar_year = ?`, year); err != nil {
		return fmt.Errorf("delete holiday_calendars: %w", err)
	}
	return nil
}

func insertAuditTx(ctx context.Context, tx *sql.Tx, year int, a *AuditEntry) error {
	if a == nil {
		return nil
	}
	meta, err := json.Marshal(a.Metadata)
	if err != nil {
		return fmt.Errorf("holiday audit metadata: %w", err)
	}
	occurred := a.OccurredAt
	if occurred.IsZero() {
		occurred = time.Now()
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO audit_logs (
			event_id, occurred_at, actor_user_id, actor_membership_id, company_id,
			action, resource_type, resource_id, decision, request_id, ip, user_agent, metadata_json
		) VALUES (?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?, 'holiday_calendar', ?, 'allow', NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?)
	`, a.EventID, occurred.UTC(), a.ActorUserID, a.ActorMembershipID, a.CompanyID, a.Action,
		strconv.Itoa(year), truncate(a.RequestID, 64), truncate(a.IP, 64), truncate(a.UserAgent, 512), string(meta)); err != nil {
		return fmt.Errorf("holiday audit insert: %w", err)
	}
	return nil
}

func truncate(s string, max int) string {
	if r := []rune(s); len(r) > max {
		return string(r[:max])
	}
	return s
}

// ReplaceCalendar deletes any existing calendar for the year and inserts a new one with dates
// (single transaction, with the optional revision check and audit event).
func (r *Repository) ReplaceCalendar(ctx context.Context, in ReplaceCalendarInput) (ReplaceResult, error) {
	res, err := r.replaceCalendar(ctx, in)
	if err != nil && concurrentWrite(err) {
		return ReplaceResult{}, staleConflict("")
	}
	return res, err
}

func (r *Repository) replaceCalendar(ctx context.Context, in ReplaceCalendarInput) (ReplaceResult, error) {
	var res ReplaceResult
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return res, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, _, _, err := lockRevision(ctx, tx, in.CalendarYear, in.ExpectedRevision); err != nil {
		return res, err
	}
	before, err := storedDateSet(ctx, tx, in.CalendarYear)
	if err != nil {
		return res, err
	}
	if err := deleteYear(ctx, tx, in.CalendarYear); err != nil {
		return res, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO holiday_calendars (
			calendar_id, calendar_year, source_file_name, content_sha256, total_days,
			description, uploaded_by
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`, in.CalendarID, in.CalendarYear, nullStr(in.SourceFileName), nullStr(in.ContentSHA256), len(in.Dates),
		nullStr(in.Description), in.UploadedBy); err != nil {
		return res, fmt.Errorf("insert holiday_calendars: %w", err)
	}

	for _, d := range in.Dates {
		key := d.Date.Format("2006-01-02")
		if _, ok := before[key]; ok {
			delete(before, key)
		} else {
			res.Added++
		}
		// Bind the civil date as text: a time.Time at 00:00 Asia/Ho_Chi_Minh would be converted
		// to UTC by the driver (loc=UTC) and stored as the previous day (DUE-01).
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO holiday_dates (calendar_id, calendar_year, holiday_date, day_type, name)
			VALUES (?, ?, ?, ?, ?)
		`, in.CalendarID, in.CalendarYear, key, d.DayType, nullStr(d.Name)); err != nil {
			return res, fmt.Errorf("insert holiday_dates: %w", err)
		}
	}
	res.Removed = len(before)

	if in.Audit != nil {
		if in.Audit.Metadata == nil {
			in.Audit.Metadata = map[string]any{}
		}
		in.Audit.Metadata["added"] = res.Added
		in.Audit.Metadata["removed"] = res.Removed
	}
	if err := insertAuditTx(ctx, tx, in.CalendarYear, in.Audit); err != nil {
		return res, err
	}
	if err := tx.Commit(); err != nil {
		return res, err
	}
	return res, nil
}

// DeleteCalendarInput removes a year's calendar so the year falls back to the default file.
type DeleteCalendarInput struct {
	CalendarYear     int
	ExpectedRevision *string
	Audit            *AuditEntry
}

// DeleteCalendar removes the year's calendar and dates; 404 when the year has no calendar.
// Returns the number of days the removed calendar had.
func (r *Repository) DeleteCalendar(ctx context.Context, in DeleteCalendarInput) (int, error) {
	total, err := r.deleteCalendar(ctx, in)
	if err != nil && concurrentWrite(err) {
		return 0, staleConflict("")
	}
	return total, err
}

func (r *Repository) deleteCalendar(ctx context.Context, in DeleteCalendarInput) (int, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	exists, _, total, err := lockRevision(ctx, tx, in.CalendarYear, in.ExpectedRevision)
	if err != nil {
		return 0, err
	}
	if !exists {
		return 0, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "holiday calendar not found for year", nil)
	}
	if err := deleteYear(ctx, tx, in.CalendarYear); err != nil {
		return 0, err
	}
	if in.Audit != nil {
		if in.Audit.Metadata == nil {
			in.Audit.Metadata = map[string]any{}
		}
		in.Audit.Metadata["total_days"] = total
	}
	if err := insertAuditTx(ctx, tx, in.CalendarYear, in.Audit); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return total, nil
}

func nullStr(s string) interface{} {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

// ReplaceCalendarInput is the transactional replace payload.
type ReplaceCalendarInput struct {
	CalendarID     string
	CalendarYear   int
	SourceFileName string
	ContentSHA256  string
	Description    string
	UploadedBy     string
	Dates          []HolidayDateRow
	// ExpectedRevision: nil = no check; "" = the year must have no calendar; else must match.
	ExpectedRevision *string
	Audit            *AuditEntry
}

// HolidayDateRow is a normalized DB row.
type HolidayDateRow struct {
	Date    time.Time
	DayType string
	Name    string
}

// StoredCalendar is a calendar row with dates loaded from MySQL.
type StoredCalendar struct {
	CalendarID     string
	CalendarYear   int
	SourceFileName string
	ContentSHA256  string
	TotalDays      int
	Description    string
	UploadedBy     string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Dates          []StoredHolidayDate
}

// StoredHolidayDate is one row from holiday_dates.
type StoredHolidayDate struct {
	Date    string
	DayType string
	Name    string
}

// GetCalendar returns metadata + dates for a year, or ErrNotFound.
func (r *Repository) GetCalendar(ctx context.Context, year int) (*StoredCalendar, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT calendar_id, calendar_year, source_file_name, content_sha256, total_days,
		       description, uploaded_by, created_at, updated_at
		FROM holiday_calendars
		WHERE calendar_year = ?
	`, year)

	var (
		id, uploaded     string
		cy               int
		src, sha, desc   sql.NullString
		total            int
		created, updated time.Time
	)
	if err := row.Scan(&id, &cy, &src, &sha, &total, &desc, &uploaded, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "holiday calendar not found for year", nil)
		}
		return nil, err
	}

	drows, err := r.db.QueryContext(ctx, `
		SELECT holiday_date, day_type, name
		FROM holiday_dates
		WHERE calendar_year = ?
		ORDER BY holiday_date
	`, year)
	if err != nil {
		return nil, err
	}
	defer drows.Close()

	var dates []StoredHolidayDate
	for drows.Next() {
		var hd time.Time
		var dt, nm sql.NullString
		if err := drows.Scan(&hd, &dt, &nm); err != nil {
			return nil, err
		}
		entry := StoredHolidayDate{
			Date:    hd.Format("2006-01-02"),
			DayType: dt.String,
		}
		if nm.Valid {
			entry.Name = nm.String
		}
		dates = append(dates, entry)
	}
	if err := drows.Err(); err != nil {
		return nil, err
	}

	cv := &StoredCalendar{
		CalendarID:   id,
		CalendarYear: cy,
		TotalDays:    total,
		UploadedBy:   uploaded,
		CreatedAt:    created.UTC(),
		UpdatedAt:    updated.UTC(),
		Dates:        dates,
	}
	if src.Valid {
		cv.SourceFileName = src.String
	}
	if sha.Valid {
		cv.ContentSHA256 = sha.String
	}
	if desc.Valid {
		cv.Description = desc.String
	}
	return cv, nil
}
