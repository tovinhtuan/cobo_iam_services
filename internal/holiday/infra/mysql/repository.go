package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

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

func stringsTrim(s string) string {
	return s
}

// ReplaceCalendar deletes any existing calendar for the year and inserts a new one with dates (single transaction).
func (r *Repository) ReplaceCalendar(ctx context.Context, in ReplaceCalendarInput) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM holiday_calendars WHERE calendar_year = ?`, in.CalendarYear); err != nil {
		return fmt.Errorf("delete holiday_calendars: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO holiday_calendars (
			calendar_id, calendar_year, source_file_name, content_sha256, total_days,
			description, uploaded_by
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`, in.CalendarID, in.CalendarYear, nullStr(in.SourceFileName), nullStr(in.ContentSHA256), len(in.Dates),
		nullStr(in.Description), in.UploadedBy); err != nil {
		return fmt.Errorf("insert holiday_calendars: %w", err)
	}

	for _, d := range in.Dates {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO holiday_dates (calendar_id, calendar_year, holiday_date, day_type, name)
			VALUES (?, ?, ?, ?, ?)
		`, in.CalendarID, in.CalendarYear, d.Date, d.DayType, nullStr(d.Name)); err != nil {
			return fmt.Errorf("insert holiday_dates: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
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
