package app

import (
	"bytes"
	"fmt"
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

// ParseHolidayXLSX parses the first worksheet: column A = date, B = optional name, C = optional day type.
// If column A cannot be parsed as a date and contains no ASCII digits (e.g. a header like "Ngày nghỉ"),
// the row is skipped without error so spreadsheets may include a title row.
func ParseHolidayXLSX(raw []byte, expectedYear int, loc *time.Location) PreviewResult {
	out := PreviewResult{
		CalendarYear: expectedYear,
		Rows:         nil,
	}
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

	seen := make(map[string]int)
	for i, row := range rows {
		rn := i + 1
		if len(row) == 0 {
			continue
		}
		cell := strings.TrimSpace(row[0])
		if cell == "" {
			continue
		}

		d, err := parseDateCell(cell, loc)
		if err != nil {
			if !hasASCIIDigit(cell) {
				continue
			}
			out.Errors = append(out.Errors, ParseIssue{Row: rn, Code: issueCodeBadDate, Message: err.Error()})
			continue
		}
		d = time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc)
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

		dayType := "PUBLIC_HOLIDAY"
		name := ""
		if len(row) > 1 {
			name = strings.TrimSpace(row[1])
		}
		if len(row) > 2 {
			if v := strings.TrimSpace(row[2]); v != "" {
				dayType = v
			}
		}

		wd := d.Weekday()
		if wd == time.Saturday || wd == time.Sunday {
			out.Warnings = append(out.Warnings, ParseIssue{
				Row: rn, Code: issueCodeWeekendOverlap,
				Message: "date falls on a weekend; it still counts as one non-trading day when listed",
			})
		}

		out.Rows = append(out.Rows, ParsedHolidayRow{
			RowIndex: rn,
			Date:     d,
			DayType:  truncateRunes(dayType, 32),
			Name:     truncateRunes(name, 512),
		})
		out.TotalAccepted++
	}

	return out
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
