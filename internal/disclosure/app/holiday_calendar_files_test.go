package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const nonTradingDaysDir = "../../../configs/non_trading_days"

func readNonTradingYear(t *testing.T, year int) HolidayCalendarYearFile {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(nonTradingDaysDir, fmt.Sprintf("%d.json", year)))
	if err != nil {
		t.Fatalf("holiday file %d: %v", year, err)
	}
	var f HolidayCalendarYearFile
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatalf("holiday file %d: %v", year, err)
	}
	return f
}

// DUE-03 / Q-06: the files hold the certain non-trading days of 2026 and 2027 (days that depend
// on yearly announcements are entered through the CMS). Each entry parses, belongs to its year
// and is unique.
func TestNonTradingDayFiles_RequiredYears(t *testing.T) {
	for _, year := range []int{2026, 2027} {
		f := readNonTradingYear(t, year)
		if f.Year != year {
			t.Fatalf("%d.json declares year %d", year, f.Year)
		}
		seen := map[string]bool{}
		for _, h := range f.Holidays {
			d, err := time.Parse("2006-01-02", h.Date)
			if err != nil || d.Year() != year {
				t.Fatalf("%d.json: bad date %q", year, h.Date)
			}
			if seen[h.Date] {
				t.Fatalf("%d.json: duplicate %s", year, h.Date)
			}
			seen[h.Date] = true
			if h.Type != "PUBLIC_HOLIDAY" && h.Type != "COMPENSATORY_DAY" {
				t.Fatalf("%d.json: %s has type %q", year, h.Date, h.Type)
			}
		}
	}
}

// Q-06: the certain days approved by the PO (2026-10-11).
func TestNonTradingDayFiles_CertainDays(t *testing.T) {
	want := map[int][]string{
		2026: {"2026-01-01", "2026-02-16", "2026-02-17", "2026-02-18", "2026-02-19", "2026-02-20", "2026-04-27", "2026-04-30", "2026-05-01", "2026-09-02"},
		2027: {"2027-01-01", "2027-04-30", "2027-05-03", "2027-09-02"},
	}
	for year, days := range want {
		f := readNonTradingYear(t, year)
		have := map[string]bool{}
		for _, h := range f.Holidays {
			have[h.Date] = true
		}
		for _, d := range days {
			if !have[d] {
				t.Fatalf("%d.json is missing %s", year, d)
			}
			if day, _ := time.Parse("2006-01-02", d); day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
				t.Fatalf("%s falls on a weekend; list the compensatory weekday instead", d)
			}
		}
	}
}

// From October the next year's file must exist (yearly BA update, Q-06). Time-dependent on purpose.
func TestNonTradingDayFiles_NextYearFromOctober(t *testing.T) {
	now := time.Now()
	if now.Month() < time.October {
		t.Skip("checked from October")
	}
	if _, err := os.Stat(filepath.Join(nonTradingDaysDir, fmt.Sprintf("%d.json", now.Year()+1))); err != nil {
		t.Fatalf("holiday file for %d is required from October: %v", now.Year()+1, err)
	}
}

// make deploy-be ships deploy-artifacts/backend/configs, not configs/: both copies must match.
func TestNonTradingDayFiles_DeployArtifactsInSync(t *testing.T) {
	entries, err := os.ReadDir(nonTradingDaysDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		src, err := os.ReadFile(filepath.Join(nonTradingDaysDir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		dst, err := os.ReadFile(filepath.Join("../../../deploy-artifacts/backend/configs/non_trading_days", e.Name()))
		if err != nil {
			t.Fatalf("deploy-artifacts copy of %s: %v", e.Name(), err)
		}
		if !bytes.Equal(bytes.ReplaceAll(src, []byte("\r\n"), []byte("\n")), bytes.ReplaceAll(dst, []byte("\r\n"), []byte("\n"))) {
			t.Fatalf("%s differs between configs/ and deploy-artifacts/", e.Name())
		}
	}
}
