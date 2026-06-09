package observe

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// fakeSource is a programmable CountSource that records the time bounds it was
// called with so tests can assert the collector's now-window math.
type fakeSource struct {
	backlog      int
	failedRecent int
	failedTotal  int
	stale        int
	err          error
	block        bool // when true, respect ctx and return ctx.Err() (timeout path)
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

func (f *fakeSource) CountBacklog(ctx context.Context) (int, error) {
	if err := f.maybe(ctx); err != nil {
		return 0, err
	}
	return f.backlog, nil
}

func (f *fakeSource) CountFailedPermanentSince(ctx context.Context, since time.Time) (int, error) {
	f.gotSince = since
	if err := f.maybe(ctx); err != nil {
		return 0, err
	}
	return f.failedRecent, nil
}

func (f *fakeSource) CountFailedPermanentTotal(ctx context.Context) (int, error) {
	if err := f.maybe(ctx); err != nil {
		return 0, err
	}
	return f.failedTotal, nil
}

func (f *fakeSource) CountStaleProcessing(ctx context.Context, olderThan time.Time) (int, error) {
	f.gotOlderThan = olderThan
	if err := f.maybe(ctx); err != nil {
		return 0, err
	}
	return f.stale, nil
}

// gatherValues registers the collector to a fresh registry (the same path
// promhttp.Handler uses) and returns name -> value for single-sample families.
func gatherValues(t *testing.T, c *EmailDeliveryCollector) map[string]float64 {
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

func TestCollector_QueryMappingAndValues(t *testing.T) {
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	src := &fakeSource{backlog: 7, failedRecent: 2, failedTotal: 9, stale: 1}
	c := NewEmailDeliveryCollector(src, 5*time.Minute)
	c.now = func() time.Time { return now }

	vals := gatherValues(t, c)

	want := map[string]float64{
		"cobo_email_backlog":                     7,
		"cobo_email_failed_permanent_recent":     2,
		"cobo_email_failed_permanent_total":      9,
		"cobo_outbox_stale_processing":           1,
		"cobo_email_metrics_scrape_errors_total": 0,
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

	// now-window math: recent uses now-15m, stale uses now-visibility(5m).
	if !src.gotSince.Equal(now.Add(-15 * time.Minute)) {
		t.Fatalf("failed_permanent_recent since = %s, want %s", src.gotSince, now.Add(-15*time.Minute))
	}
	if !src.gotOlderThan.Equal(now.Add(-5 * time.Minute)) {
		t.Fatalf("stale olderThan = %s, want %s", src.gotOlderThan, now.Add(-5*time.Minute))
	}
}

func TestCollector_ErrorPathIncrementsScrapeErrorsAndOmitsGauges(t *testing.T) {
	src := &fakeSource{err: errors.New("db down")}
	c := NewEmailDeliveryCollector(src, 5*time.Minute)

	vals := gatherValues(t, c)

	// All four queries failed -> their gauges must be absent (Prometheus keeps
	// staleness rather than a misleading zero), scrape_errors == 4.
	for _, gauge := range []string{
		"cobo_email_backlog", "cobo_email_failed_permanent_recent",
		"cobo_email_failed_permanent_total", "cobo_outbox_stale_processing",
	} {
		if _, ok := vals[gauge]; ok {
			t.Fatalf("gauge %q should be omitted on query error", gauge)
		}
	}
	if got := vals["cobo_email_metrics_scrape_errors_total"]; got != 4 {
		t.Fatalf("scrape_errors = %v, want 4", got)
	}
}

func TestCollector_TimeoutPathIncrementsScrapeErrors(t *testing.T) {
	src := &fakeSource{block: true}
	c := NewEmailDeliveryCollector(src, 5*time.Minute)
	c.queryTimeout = time.Millisecond // force deadline without slow test

	vals := gatherValues(t, c)

	if _, ok := vals["cobo_email_backlog"]; ok {
		t.Fatalf("backlog gauge should be omitted on timeout")
	}
	if got := vals["cobo_email_metrics_scrape_errors_total"]; got != 4 {
		t.Fatalf("scrape_errors on timeout = %v, want 4", got)
	}
}

func TestCollector_PartialFailureIsolatesGauges(t *testing.T) {
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	// Backlog succeeds; the rest are governed by err. Use a source where only
	// total errors by overriding via a wrapper.
	base := &fakeSource{backlog: 3, failedRecent: 0, failedTotal: 0, stale: 0}
	c := NewEmailDeliveryCollector(partialErrSource{base}, 5*time.Minute)
	c.now = func() time.Time { return now }

	vals := gatherValues(t, c)
	if vals["cobo_email_backlog"] != 3 {
		t.Fatalf("backlog = %v, want 3", vals["cobo_email_backlog"])
	}
	if _, ok := vals["cobo_email_failed_permanent_total"]; ok {
		t.Fatalf("failed_permanent_total should be omitted")
	}
	if vals["cobo_email_metrics_scrape_errors_total"] != 1 {
		t.Fatalf("scrape_errors = %v, want 1", vals["cobo_email_metrics_scrape_errors_total"])
	}
}

// partialErrSource fails ONLY CountFailedPermanentTotal to prove gauge isolation.
type partialErrSource struct{ *fakeSource }

func (p partialErrSource) CountFailedPermanentTotal(context.Context) (int, error) {
	return 0, errors.New("boom")
}
