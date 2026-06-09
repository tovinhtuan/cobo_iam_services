package observe_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cobo/cobo_iam_services/internal/reminder/infra/observe"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type stubSource struct{}

func (stubSource) CountBacklog(context.Context, time.Time) (int, error)        { return 4, nil }
func (stubSource) CountFailedSince(context.Context, time.Time) (int, error)    { return 1, nil }
func (stubSource) CountFailedTotal(context.Context) (int, error)              { return 5, nil }
func (stubSource) CountStuckDispatching(context.Context, time.Time) (int, error) { return 0, nil }

// TestReminderMetricsEndpoint_ExposesAllNames is the deterministic equivalent of
// `curl /metrics | grep cobo_reminder_*`: it serves the collector through the same
// promhttp pipeline the API uses and asserts every required series is present.
func TestReminderMetricsEndpoint_ExposesAllNames(t *testing.T) {
	reg := prometheus.NewRegistry()
	if err := reg.Register(observe.NewReminderObservabilityCollector(stubSource{}, 5*time.Minute)); err != nil {
		t.Fatalf("register: %v", err)
	}

	srv := httptest.NewServer(promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	text := string(body)

	for _, name := range []string{
		"cobo_reminder_backlog",
		"cobo_reminder_failed_recent",
		"cobo_reminder_failed_total",
		"cobo_reminder_stuck_dispatching",
		"cobo_reminder_metrics_scrape_errors_total",
	} {
		if !strings.Contains(text, name) {
			t.Fatalf("/metrics body missing %q\n---\n%s", name, text)
		}
	}
}
