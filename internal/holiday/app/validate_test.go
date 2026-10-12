package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

var testLoc = time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)

func xlsxOf(t *testing.T, rows [][]string) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	sheet := f.GetSheetName(0)
	for i, r := range rows {
		for j, v := range r {
			cell, _ := excelize.CoordinatesToCellName(j+1, i+1)
			_ = f.SetCellStr(sheet, cell, v)
		}
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func codes(issues []ParseIssue) string {
	var out []string
	for _, i := range issues {
		out = append(out, i.Code)
	}
	return strings.Join(out, ",")
}

// The editor (JSON) and the .xlsx upload must apply the same rules to the same data.
func TestValidate_JSONMatchesXLSX(t *testing.T) {
	longName := strings.Repeat("Đ", 600)
	in := []HolidayInputRow{
		{Date: "2026-02-16", Name: "Tết", DayType: ""},
		{Date: "2026-02-16", Name: "dup"},
		{Date: "2025-12-31"},
		{Date: "2026-02-21", Name: "Thứ 7"},
		{Date: "2026-09-02", Name: longName, DayType: "COMPENSATORY_DAY_OFF"},
	}
	var sheet [][]string
	for _, r := range in {
		sheet = append(sheet, []string{r.Date, r.Name, r.DayType})
	}
	j := ValidateHolidayInput(in, 2026, testLoc)
	x := ParseHolidayXLSX(xlsxOf(t, sheet), 2026, testLoc)

	if codes(j.Errors) != codes(x.Errors) || codes(j.Warnings) != codes(x.Warnings) || j.TotalAccepted != x.TotalAccepted {
		t.Fatalf("json errors=%s warnings=%s accepted=%d; xlsx errors=%s warnings=%s accepted=%d",
			codes(j.Errors), codes(j.Warnings), j.TotalAccepted, codes(x.Errors), codes(x.Warnings), x.TotalAccepted)
	}
	if codes(j.Errors) != "DUPLICATE_DATE,WRONG_YEAR" || codes(j.Warnings) != "DATE_ON_WEEKEND,VALUE_TRUNCATED" {
		t.Fatalf("errors=%s warnings=%s", codes(j.Errors), codes(j.Warnings))
	}
	if j.Rows[0].DayType != "PUBLIC_HOLIDAY" || len([]rune(j.Rows[2].Name)) != 512 {
		t.Fatalf("rows = %+v", j.Rows[0])
	}
}

func TestValidateHolidayInput_StrictDateFormat(t *testing.T) {
	res := ValidateHolidayInput([]HolidayInputRow{{Date: "16/02/2026"}, {Date: ""}, {Date: "2026-02-30"}}, 2026, testLoc)
	if codes(res.Errors) != "BAD_DATE,BAD_DATE,BAD_DATE" || res.TotalAccepted != 0 {
		t.Fatalf("errors=%s", codes(res.Errors))
	}
	if res.Errors[0].Row != 1 || res.Errors[2].Row != 3 {
		t.Fatalf("row index must be 1-based position: %+v", res.Errors)
	}
}

func TestValidateHolidayInput_RevisionIgnoresOrder(t *testing.T) {
	a := ValidateHolidayInput([]HolidayInputRow{{Date: "2026-01-01", Name: "A"}, {Date: "2026-09-02", Name: "B"}}, 2026, testLoc)
	b := ValidateHolidayInput([]HolidayInputRow{{Date: "2026-09-02", Name: "B"}, {Date: "2026-01-01", Name: "A"}}, 2026, testLoc)
	c := ValidateHolidayInput([]HolidayInputRow{{Date: "2026-09-02", Name: "B2"}, {Date: "2026-01-01", Name: "A"}}, 2026, testLoc)
	if a.ContentSHA256 == "" || a.ContentSHA256 != b.ContentSHA256 || a.ContentSHA256 == c.ContentSHA256 {
		t.Fatalf("sha a=%s b=%s c=%s", a.ContentSHA256, b.ContentSHA256, c.ContentSHA256)
	}
}

func TestPreviewRows_RowLimits(t *testing.T) {
	s := NewService(nil, nil, nil)
	if _, err := s.PreviewRows(context.Background(), 2026, nil); err == nil || !strings.Contains(err.Error(), "at least one") {
		t.Fatalf("empty: %v", err)
	}
	rows := make([]HolidayInputRow, MaxEditorRows+1)
	if _, err := s.PreviewRows(context.Background(), 2026, rows); err == nil || !strings.Contains(err.Error(), "at most") {
		t.Fatalf("too many: %v", err)
	}
}

func writeDefault(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "2026.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestDefault_ReadsConfigAndNotFound(t *testing.T) {
	dir := writeDefault(t, `{"year":2026,"holidays":[{"date":"2026-09-02","name":"Quoc khanh","type":""},{"date":"2026-01-01","name":"Tet Duong Lich","type":"PUBLIC_HOLIDAY"}]}`)
	s := NewService(nil, nil, nil, WithDefaultCalendarDir(dir))
	dv, err := s.Default(context.Background(), 2026)
	if err != nil {
		t.Fatal(err)
	}
	if len(dv.Dates) != 2 || dv.Dates[0].Date != "2026-01-01" || dv.Dates[1].DayType != "PUBLIC_HOLIDAY" || dv.Source != "default" {
		t.Fatalf("default = %+v", dv)
	}
	if _, err := s.Default(context.Background(), 2030); err == nil || !strings.Contains(err.Error(), "no default") {
		t.Fatalf("missing year: %v", err)
	}
}

// Downloading a calendar and uploading it again must give the same data.
func TestExportXLSX_RoundTripsThroughParser(t *testing.T) {
	dir := writeDefault(t, `{"year":2026,"holidays":[{"date":"2026-01-01","name":"Tết Dương lịch","type":"PUBLIC_HOLIDAY"},{"date":"2026-04-27","name":"Giỗ Tổ (nghỉ bù)","type":"COMPENSATORY_DAY_OFF"}]}`)
	s := NewService(nil, nil, nil, WithDefaultCalendarDir(dir))
	raw, err := s.ExportXLSX(context.Background(), 2026, "default")
	if err != nil {
		t.Fatal(err)
	}
	res := ParseHolidayXLSX(raw, 2026, testLoc)
	if len(res.Errors) != 0 || res.TotalAccepted != 2 {
		t.Fatalf("round trip: errors=%v accepted=%d", res.Errors, res.TotalAccepted)
	}
	got := res.Rows[1]
	if got.Date.Format("2006-01-02") != "2026-04-27" || got.Name != "Giỗ Tổ (nghỉ bù)" || got.DayType != "COMPENSATORY_DAY_OFF" {
		t.Fatalf("row = %+v", got)
	}
	if _, err := s.ExportXLSX(context.Background(), 2026, "bogus"); err == nil {
		t.Fatal("expected error for unknown source")
	}
}

// The shipped default calendars must stay readable by the editor rules.
func TestShippedDefaultCalendarsValidate(t *testing.T) {
	s := NewService(nil, nil, nil, WithDefaultCalendarDir(filepath.Join("..", "..", "..", "configs", "non_trading_days")))
	for _, year := range []int{2026, 2027} {
		dv, err := s.Default(nil, year)
		if err != nil {
			t.Fatalf("%d: %v", year, err)
		}
		var rows []HolidayInputRow
		for _, d := range dv.Dates {
			rows = append(rows, HolidayInputRow{Date: d.Date, Name: d.Name, DayType: d.DayType})
		}
		if res := ValidateHolidayInput(rows, year, testLoc); len(res.Errors) != 0 {
			t.Fatalf("%d: %v", year, res.Errors)
		}
	}
}

// A name containing the old "|"/newline separators must not make two lists hash alike.
func TestRevision_UnambiguousWithSeparatorsInName(t *testing.T) {
	a := ValidateHolidayInput([]HolidayInputRow{{Date: "2026-01-01", Name: "x\n2026-01-02|PUBLIC_HOLIDAY|y"}}, 2026, testLoc)
	b := ValidateHolidayInput([]HolidayInputRow{{Date: "2026-01-01", Name: "x"}, {Date: "2026-01-02", Name: "y"}}, 2026, testLoc)
	if a.ContentSHA256 == b.ContentSHA256 {
		t.Fatal("revision collision")
	}
}

func TestReplace_DescriptionTooLongIs400(t *testing.T) {
	s := NewService(nil, nil, nil)
	_, err := s.Replace(context.Background(), ReplaceRequest{
		Year: 2026, FromEditor: true, Rows: []HolidayInputRow{{Date: "2026-01-01"}},
		Description: strings.Repeat("d", 2001), Actor: Actor{UserID: "u"},
	})
	if err == nil || !strings.Contains(err.Error(), "description") {
		t.Fatalf("err = %v", err)
	}
}
