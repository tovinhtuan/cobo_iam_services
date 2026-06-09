package observe_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cobo/cobo_iam_services/internal/notification/infra/observe"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type stubSource struct{}

func (stubSource) CountBacklog(context.Context) (int, error)                         { return 4, nil }
func (stubSource) CountFailedPermanentSince(context.Context, time.Time) (int, error) { return 1, nil }
func (stubSource) CountFailedPermanentTotal(context.Context) (int, error)            { return 5, nil }
func (stubSource) CountStaleProcessing(context.Context, time.Time) (int, error)      { return 0, nil }

// TestMetricsEndpoint_ExposesAllNames is the deterministic equivalent of
// `curl /metrics | grep cobo_email_*`: it serves the collector through the same
// promhttp pipeline the API uses and asserts every required series is present.
func TestMetricsEndpoint_ExposesAllNames(t *testing.T) {
	reg := prometheus.NewRegistry()
	if err := reg.Register(observe.NewEmailDeliveryCollector(stubSource{}, 5*time.Minute)); err != nil {
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
		"cobo_email_backlog",
		"cobo_email_failed_permanent_recent",
		"cobo_email_failed_permanent_total",
		"cobo_outbox_stale_processing",
		"cobo_email_metrics_scrape_errors_total",
	} {
		if !strings.Contains(text, name) {
			t.Fatalf("/metrics body missing %q\n---\n%s", name, text)
		}
	}
}
