package observability

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	initOnce sync.Once

	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	partialTotal    *prometheus.CounterVec
	errorsTotal     *prometheus.CounterVec
)

func ensureMetrics() {
	initOnce.Do(func() {
		requestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "cobo",
			Subsystem: "dashboard_overview",
			Name:      "requests_total",
			Help:      "Total GET /api/v1/company/dashboard/overview requests",
		}, []string{"status", "range"})
		requestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "cobo",
			Subsystem: "dashboard_overview",
			Name:      "request_duration_seconds",
			Help:      "Overview endpoint request duration in seconds",
			Buckets:   []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10},
		}, []string{"range"})
		partialTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "cobo",
			Subsystem: "dashboard_overview",
			Name:      "partial_responses_total",
			Help:      "Overview responses with meta.partial=true",
		}, []string{"range"})
		errorsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "cobo",
			Subsystem: "dashboard_overview",
			Name:      "errors_total",
			Help:      "Overview endpoint errors by HTTP status code",
		}, []string{"code"})
		prometheus.MustRegister(requestsTotal, requestDuration, partialTotal, errorsTotal)
	})
}

// RecordRequest records request outcome metrics. status is HTTP status as string (e.g. "200", "401").
func RecordRequest(status, rangePreset string, duration time.Duration, partial bool) {
	ensureMetrics()
	if rangePreset == "" {
		rangePreset = "30d"
	}
	requestsTotal.WithLabelValues(status, rangePreset).Inc()
	requestDuration.WithLabelValues(rangePreset).Observe(duration.Seconds())
	if partial {
		partialTotal.WithLabelValues(rangePreset).Inc()
	}
	if status != "200" {
		errorsTotal.WithLabelValues(status).Inc()
	}
}
