package observe

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// fakeSource is a programmable CountSource recording the time bounds it was called with.
type fakeSource struct {
	backlog      int
	failedRecent int
	failedTotal  int
	stuck        int
	err          error
	block        bool
	gotNow       time.Time
	gotSince     time.Time
	gotOlderThan time.Time
}

func (f *fakeSource) maybe(ctx context.Context) error {
	if f.block {
		<-ctx.Done()
		return ctx.Err()
	}
	return f.err
}

func (f *fakeSource) CountBacklog(ctx context.Context, now time.Time) (int, error) {
	f.gotNow = now
	if err := f.maybe(ctx); err != nil {
		return 0, err
	}
	return f.backlog, nil
}

func (f *fakeSource) CountFailedSince(ctx context.Context, since time.Time) (int, error) {
	f.gotSince = since
	if err := f.maybe(ctx); err != nil {
		return 0, err
	}
	return f.failedRecent, nil
}

func (f *fakeSource) CountFailedTotal(ctx context.Context) (int, error) {
	if err := f.maybe(ctx); err != nil {
		return 0, err
	}
	return f.failedTotal, nil
}

func (f *fakeSource) CountStuckDispatching(ctx context.Context, olderThan time.Time) (int, error) {
	f.gotOlderThan = olderThan
	if err := f.maybe(ctx); err != nil {
		return 0, err
	}
	return f.stuck, nil
}

func gatherValues(t *testing.T, c *ReminderObservabilityCollector) map[string]float64 {
	t.Helper()
	reg := prometheus.NewRegistry()
	if err := reg.Register(c); err != nil {
		t.Fatalf("register: %v", err)
	}
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	out := map[string]float64{}
	for _, mf := range mfs {
		for _, m := range mf.GetMetric() {
			switch {
			case m.GetGauge() != nil:
				out[mf.GetName()] = m.GetGauge().GetValue()
			case m.GetCounter() != nil:
				out[mf.GetName()] = m.GetCounter().GetValue()
			}
		}
	}
	return out
}

func TestReminderCollector_QueryMappingAndValues(t *testing.T) {
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	src := &fakeSource{backlog: 7, failedRecent: 2, failedTotal: 9, stuck: 1}
	c := NewReminderObservabilityCollector(src, 5*time.Minute)
	c.now = func() time.Time { return now }

	vals := gatherValues(t, c)

	want := map[string]float64{
		"cobo_reminder_backlog":                     7,
		"cobo_reminder_failed_recent":               2,
		"cobo_reminder_failed_total":                9,
		"cobo_reminder_stuck_dispatching":           1,
		"cobo_reminder_metrics_scrape_errors_total": 0,
	}
	for name, w := range want {
		got, ok := vals[name]
		if !ok {
			t.Fatalf("metric %q missing from /metrics", name)
		}
		if got != w {
			t.Fatalf("metric %q = %v, want %v", name, got, w)
		}
	}

	if !src.gotNow.Equal(now) {
		t.Fatalf("backlog now = %s, want %s", src.gotNow, now)
	}
	if !src.gotSince.Equal(now.Add(-15 * time.Minute)) {
		t.Fatalf("failed_recent since = %s, want %s", src.gotSince, now.Add(-15*time.Minute))
	}
	if !src.gotOlderThan.Equal(now.Add(-5 * time.Minute)) {
		t.Fatalf("stuck olderThan = %s, want %s", src.gotOlderThan, now.Add(-5*time.Minute))
	}
}

func TestReminderCollector_ErrorPathIncrementsScrapeErrors(t *testing.T) {
	src := &fakeSource{err: errors.New("db down")}
	c := NewReminderObservabilityCollector(src, 5*time.Minute)

	vals := gatherValues(t, c)

	for _, gauge := range []string{
		"cobo_reminder_backlog", "cobo_reminder_failed_recent",
		"cobo_reminder_failed_total", "cobo_reminder_stuck_dispatching",
	} {
		if _, ok := vals[gauge]; ok {
			t.Fatalf("gauge %q should be omitted on query error", gauge)
		}
	}
	if got := vals["cobo_reminder_metrics_scrape_errors_total"]; got != 4 {
		t.Fatalf("scrape_errors = %v, want 4", got)
	}
}

func TestReminderCollector_TimeoutPathIncrementsScrapeErrors(t *testing.T) {
	src := &fakeSource{block: true}
	c := NewReminderObservabilityCollector(src, 5*time.Minute)
	c.queryTimeout = time.Millisecond

	vals := gatherValues(t, c)

	if _, ok := vals["cobo_reminder_backlog"]; ok {
		t.Fatalf("backlog gauge should be omitted on timeout")
	}
	if got := vals["cobo_reminder_metrics_scrape_errors_total"]; got != 4 {
		t.Fatalf("scrape_errors on timeout = %v, want 4", got)
	}
}
