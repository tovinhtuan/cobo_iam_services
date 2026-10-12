package app

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

const (
	issueCodeBadDate        = "BAD_DATE"
	issueCodeWrongYear      = "WRONG_YEAR"
	issueCodeDuplicate      = "DUPLICATE_DATE"
	issueCodeWeekendOverlap = "DATE_ON_WEEKEND"
)

var dateFormats = []string{
	"2006-01-02",
	"02/01/2006",
	"2/1/2006",
	"02-01-2006",
}

const (
	issueCodeTruncated = "VALUE_TRUNCATED"

	maxNameRunes    = 512
	maxDayTypeRunes = 32
	defaultDayType  = "PUBLIC_HOLIDAY"
)

// candidateRow is one input row before validation; xlsx and JSON both feed ValidateHolidayRows.
type candidateRow struct {
	RowIndex int
	Date     time.Time
	DateErr  error
	Name     string
	DayType  string
}

// validateHolidayRows applies the rules shared by .xlsx upload and the JSON editor: the date must
// parse and belong to expectedYear, no duplicates, weekend dates warn (weekends are already
// non-trading), name/day_type are trimmed and cut to the column size with a warning.
func validateHolidayRows(cands []candidateRow, expectedYear int, loc *time.Location) PreviewResult {
	out := PreviewResult{CalendarYear: expectedYear}
	seen := make(map[string]int)
	for _, c := range cands {
		rn := c.RowIndex
		if c.DateErr != nil {
			out.Errors = append(out.Errors, ParseIssue{Row: rn, Code: issueCodeBadDate, Message: c.DateErr.Error()})
			continue
		}
		d := time.Date(c.Date.Year(), c.Date.Month(), c.Date.Day(), 0, 0, 0, 0, loc)
		if y := d.Year(); y != expectedYear {
			out.Errors = append(out.Errors, ParseIssue{
				Row: rn, Code: issueCodeWrongYear,
				Message: fmt.Sprintf("date must belong to calendar year %d (got %d)", expectedYear, y),
			})
			continue
		}

		key := d.Format("2006-01-02")
		if prev, dup := seen[key]; dup {
			out.Errors = append(out.Errors, ParseIssue{
				Row: rn, Code: issueCodeDuplicate,
				Message: fmt.Sprintf("duplicate date %s (also row %d)", key, prev),
			})
			continue
		}
		seen[key] = rn

		dayType := strings.TrimSpace(c.DayType)
		if dayType == "" {
			dayType = defaultDayType
		}
		name := strings.TrimSpace(c.Name)
		if runeLen(name) > maxNameRunes || runeLen(dayType) > maxDayTypeRunes {
			out.Warnings = append(out.Warnings, ParseIssue{
				Row: rn, Code: issueCodeTruncated,
				Message: fmt.Sprintf("name is cut to %d characters and type to %d", maxNameRunes, maxDayTypeRunes),
			})
		}

		if wd := d.Weekday(); wd == time.Saturday || wd == time.Sunday {
			out.Warnings = append(out.Warnings, ParseIssue{
				Row: rn, Code: issueCodeWeekendOverlap,
				Message: "date falls on a weekend; weekends are already counted by the system, no need to list it",
			})
		}

		out.Rows = append(out.Rows, ParsedHolidayRow{
			RowIndex: rn,
			Date:     d,
			DayType:  truncateRunes(dayType, maxDayTypeRunes),
			Name:     truncateRunes(name, maxNameRunes),
		})
		out.TotalAccepted++
	}
	return out
}

func runeLen(s string) int { return len([]rune(s)) }

// ParseHolidayXLSX parses the first worksheet: column A = date, B = optional name, C = optional day type.
// If column A cannot be parsed as a date and contains no ASCII digits (e.g. a header like "Ngày nghỉ"),
// the row is skipped without error so spreadsheets may include a title row.
func ParseHolidayXLSX(raw []byte, expectedYear int, loc *time.Location) PreviewResult {
	out := PreviewResult{CalendarYear: expectedYear}
	if loc == nil {
		loc = time.UTC
	}

	f, err := excelize.OpenReader(bytes.NewReader(raw))
	if err != nil {
		out.Errors = append(out.Errors, ParseIssue{Row: 0, Code: "INVALID_XLSX", Message: fmt.Sprintf("cannot open workbook: %v", err)})
		return out
	}
	defer func() { _ = f.Close() }()

	sheet := f.GetSheetName(0)
	if sheet == "" {
		out.Errors = append(out.Errors, ParseIssue{Row: 0, Code: "EMPTY_WORKBOOK", Message: "workbook has no sheets"})
		return out
	}

	rows, err := f.GetRows(sheet)
	if err != nil {
		out.Errors = append(out.Errors, ParseIssue{Row: 0, Code: "READ_SHEET", Message: err.Error()})
		return out
	}

	var cands []candidateRow
	for i, row := range rows {
		if len(row) == 0 {
			continue
		}
		cell := strings.TrimSpace(row[0])
		if cell == "" {
			continue
		}
		c := candidateRow{RowIndex: i + 1}
		c.Date, c.DateErr = parseDateCell(cell, loc)
		if c.DateErr != nil && !hasASCIIDigit(cell) {
			continue
		}
		if len(row) > 1 {
			c.Name = row[1]
		}
		if len(row) > 2 {
			c.DayType = row[2]
		}
		cands = append(cands, c)
	}
	return validateHolidayRows(cands, expectedYear, loc)
}

// ValidateHolidayInput validates editor rows (JSON). Dates must be YYYY-MM-DD; RowIndex is the
// 1-based position in the request. ContentSHA256 is the hash of the normalized accepted rows,
// so the same list always yields the same revision.
func ValidateHolidayInput(in []HolidayInputRow, expectedYear int, loc *time.Location) PreviewResult {
	if loc == nil {
		loc = time.UTC
	}
	cands := make([]candidateRow, 0, len(in))
	for i, r := range in {
		c := candidateRow{RowIndex: i + 1, Name: r.Name, DayType: r.DayType}
		ds := strings.TrimSpace(r.Date)
		if ds == "" {
			c.DateErr = fmt.Errorf("date is required")
		} else if d, err := time.ParseInLocation("2006-01-02", ds, loc); err != nil {
			c.DateErr = fmt.Errorf("unrecognized date value %q (expected YYYY-MM-DD)", ds)
		} else {
			c.Date = d
		}
		cands = append(cands, c)
	}
	out := validateHolidayRows(cands, expectedYear, loc)
	out.ContentSHA256 = rowsSHA256(out.Rows)
	return out
}

func rowsSHA256(rows []ParsedHolidayRow) string {
	sorted := append([]ParsedHolidayRow(nil), rows...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Date.Before(sorted[j].Date) })
	// JSON keeps field boundaries unambiguous even when a name contains "|" or a newline.
	canon := make([][3]string, 0, len(sorted))
	for _, r := range sorted {
		canon = append(canon, [3]string{r.Date.Format("2006-01-02"), r.DayType, r.Name})
	}
	raw, _ := json.Marshal(canon)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func hasASCIIDigit(s string) bool {
	for _, r := range s {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

func truncateRunes(s string, max int) string {
	s = strings.TrimSpace(s)
	if s == "" || max <= 0 {
		return s
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

func parseDateCell(s string, loc *time.Location) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty cell")
	}
	for _, layout := range dateFormats {
		if t, err := time.ParseInLocation(layout, s, loc); err == nil {
			return t, nil
		}
	}
	// Excel serial (possibly "45678.0")
	if f, err := strconv.ParseFloat(strings.Replace(s, ",", ".", 1), 64); err == nil {
		t, err2 := excelSerialToTime(f, loc)
		if err2 == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized date value %q", s)
}

// excelSerialToTime converts an Excel 1900 date serial to a civil date in loc (date-only).
// Uses the 1900 date system epoch 1899-12-30.
func excelSerialToTime(serial float64, loc *time.Location) (time.Time, error) {
	if serial <= 0 {
		return time.Time{}, fmt.Errorf("invalid serial %v", serial)
	}
	whole := int(serial + 0.0000001) // stabilize tiny float error
	epoch := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
	t := epoch.AddDate(0, 0, whole)
	t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
	return t, nil
}
