package app

import (
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func TestParseDateCellLayouts(t *testing.T) {
	loc := time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)
	cases := []struct {
		in   string
		want string
	}{
		{"2026-01-02", "2026-01-02"},
		{"02/01/2026", "2026-01-02"},
		{"2/1/2026", "2026-01-02"},
	}
	for _, tc := range cases {
		d, err := parseDateCell(tc.in, loc)
		if err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
		if got := d.Format("2006-01-02"); got != tc.want {
			t.Fatalf("%q: got %s want %s", tc.in, got, tc.want)
		}
	}
}

func TestParseHolidayXLSXDuplicate(t *testing.T) {
	loc := time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	sheet := f.GetSheetName(0)
	_ = f.SetCellValue(sheet, "A1", "2026-01-01")
	_ = f.SetCellValue(sheet, "A2", "2026-01-01")
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	res := ParseHolidayXLSX(buf.Bytes(), 2026, loc)
	if len(res.Errors) == 0 {
		t.Fatal("expected duplicate error")
	}
	if res.Errors[0].Code != issueCodeDuplicate {
		t.Fatalf("code=%s", res.Errors[0].Code)
	}
}

func TestParseHolidayXLSXSkipsTextHeaderRow(t *testing.T) {
	loc := time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	sheet := f.GetSheetName(0)
	_ = f.SetCellValue(sheet, "A1", "Ngày nghỉ")
	_ = f.SetCellValue(sheet, "A2", "2026-01-02")
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	res := ParseHolidayXLSX(buf.Bytes(), 2026, loc)
	if len(res.Errors) != 0 {
		t.Fatalf("expected no errors, got %v", res.Errors)
	}
	if res.TotalAccepted != 1 || len(res.Rows) != 1 {
		t.Fatalf("accepted=%d rows=%d", res.TotalAccepted, len(res.Rows))
	}
}

func TestParseHolidayXLSXWeekendWarning(t *testing.T) {
	loc := time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	sheet := f.GetSheetName(0)
	// 2026-01-03 is Saturday
	_ = f.SetCellValue(sheet, "A1", "2026-01-03")
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	res := ParseHolidayXLSX(buf.Bytes(), 2026, loc)
	if len(res.Warnings) != 1 || res.Warnings[0].Code != issueCodeWeekendOverlap {
		t.Fatalf("warnings=%v", res.Warnings)
	}
	if res.TotalAccepted != 1 {
		t.Fatalf("accepted=%d", res.TotalAccepted)
	}
}
