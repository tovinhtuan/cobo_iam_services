package observe

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type fakePeriodicSource struct {
	states   map[string]int
	stale    int
	dbYears  map[int]bool
	err      error
	gotLease time.Duration
}

func (f *fakePeriodicSource) CountCyclesByState(context.Context) (map[string]int, error) {
	return f.states, f.err
}

func (f *fakePeriodicSource) CountStaleClaims(_ context.Context, olderThan time.Duration) (int, error) {
	f.gotLease = olderThan
	return f.stale, f.err
}

func (f *fakePeriodicSource) HasDBCalendar(_ context.Context, year int) (bool, error) {
	return f.dbYears[year], f.err
}

func gatherPeriodic(t *testing.T, c *PeriodicObservabilityCollector) map[string]float64 {
	t.Helper()
	reg := prometheus.NewRegistry()
	if err := reg.Register(c); err != nil {
		t.Fatal(err)
	}
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]float64{}
	for _, mf := range mfs {
		for _, m := range mf.GetMetric() {
			key := mf.GetName()
			for _, l := range m.GetLabel() {
				key += "|" + l.GetName() + "=" + l.GetValue()
			}
			if m.GetGauge() != nil {
				out[key] = m.GetGauge().GetValue()
			} else if m.GetCounter() != nil {
				out[key] = m.GetCounter().GetValue()
			}
		}
	}
	return out
}

// PER-06: alerting signals for the periodic worker and next year's holiday calendar.
func TestPeriodicCollector_Values(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "2026.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	src := &fakePeriodicSource{states: map[string]int{"PENDING": 3, "CLAIMED": 1}, stale: 1, dbYears: map[int]bool{2026: true}}
	c := NewPeriodicObservabilityCollector(src, dir, 10*time.Minute)
	// 2026-12-31 18:00 UTC is already 2027-01-01 in Asia/Ho_Chi_Minh.
	c.now = func() time.Time { return time.Date(2026, 12, 31, 18, 0, 0, 0, time.UTC) }
	got := gatherPeriodic(t, c)
	want := map[string]float64{
		"cobo_periodic_cycles|state=PENDING":                            3,
		"cobo_periodic_cycles|state=CLAIMED":                            1,
		"cobo_periodic_cycles_claimed_stale":                            1,
		"cobo_holiday_calendar_year_available|slot=current|source=db":   0,
		"cobo_holiday_calendar_year_available|slot=current|source=file": 0,
		"cobo_holiday_calendar_year_available|slot=next|source=db":      0,
		"cobo_periodic_metrics_scrape_errors_total":                     0,
	}
	for k, w := range want {
		if v, ok := got[k]; !ok || v != w {
			t.Fatalf("%s = %v (present=%v), want %v; all=%v", k, v, ok, w, got)
		}
	}
	if src.gotLease != 10*time.Minute {
		t.Fatalf("stale query lease = %s", src.gotLease)
	}

	c.now = func() time.Time { return time.Date(2026, 10, 11, 3, 0, 0, 0, time.UTC) }
	got = gatherPeriodic(t, c)
	if got["cobo_holiday_calendar_year_available|slot=current|source=db"] != 1 ||
		got["cobo_holiday_calendar_year_available|slot=current|source=file"] != 1 {
		t.Fatalf("2026 calendar must be available: %v", got)
	}
}

func TestPeriodicCollector_ErrorOmitsGaugesAndCounts(t *testing.T) {
	src := &fakePeriodicSource{err: errors.New("db down")}
	c := NewPeriodicObservabilityCollector(src, t.TempDir(), 10*time.Minute)
	got := gatherPeriodic(t, c)
	if _, ok := got["cobo_periodic_cycles_claimed_stale"]; ok {
		t.Fatal("gauge must be omitted on query error")
	}
	if got["cobo_periodic_metrics_scrape_errors_total"] != 4 {
		t.Fatalf("scrape errors = %v, want 4", got["cobo_periodic_metrics_scrape_errors_total"])
	}
}
