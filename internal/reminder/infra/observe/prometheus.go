package observe

import (
	"sort"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	promInitOnce sync.Once
	promSet      *promMetricsSet
)

type promMetricsSet struct {
	configUpsertTotal        *prometheus.CounterVec
	occurrenceCreatedTotal   *prometheus.CounterVec
	dispatchAttemptTotal     *prometheus.CounterVec
	retryAlertThresholdTotal *prometheus.CounterVec
	slaBreachTotal           *prometheus.CounterVec
	dispatchDueLatencyMs     *prometheus.HistogramVec
	// Workflow milestone bridge counters (PR-24).
	milestoneSeedTotal      *prometheus.CounterVec
	milestoneSeedErrTotal   *prometheus.CounterVec
	milestoneMarkErrTotal   *prometheus.CounterVec
}

func newPromMetricsSet() *promMetricsSet {
	return &promMetricsSet{
		configUpsertTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "cobo",
			Subsystem: "reminder",
			Name:      "config_upsert_total",
			Help:      "Total reminder config upserts",
		}, []string{"scope_type"}),
		occurrenceCreatedTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "cobo",
			Subsystem: "reminder",
			Name:      "occurrence_created_total",
			Help:      "Total reminder occurrences created",
		}, []string{"source"}),
		dispatchAttemptTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "cobo",
			Subsystem: "reminder",
			Name:      "dispatch_attempt_total",
			Help:      "Total reminder dispatch attempts by status",
		}, []string{"status"}),
		retryAlertThresholdTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "cobo",
			Subsystem: "reminder",
			Name:      "retry_alert_threshold_total",
			Help:      "Total reminder retries that reached alert threshold",
		}, []string{"threshold"}),
		slaBreachTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "cobo",
			Subsystem: "reminder",
			Name:      "sla_breach_total",
			Help:      "Total reminder SLA breaches",
		}, []string{"window"}),
		dispatchDueLatencyMs: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "cobo",
			Subsystem: "reminder",
			Name:      "dispatch_due_duration_ms",
			Help:      "Duration of dispatch due batch processing in milliseconds",
			Buckets:   []float64{10, 50, 100, 250, 500, 1000, 2000, 5000, 10000},
		}, []string{"status"}),
		milestoneSeedTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "cobo",
			Subsystem: "reminder",
			Name:      "milestone_seeded_total",
			Help:      "Total workflow step milestones successfully seeded as reminder occurrences",
		}, []string{"milestone_type"}),
		milestoneSeedErrTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "cobo",
			Subsystem: "reminder",
			Name:      "milestone_seed_error_total",
			Help:      "Total errors while seeding milestone occurrences",
		}, []string{"milestone_type"}),
		milestoneMarkErrTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "cobo",
			Subsystem: "reminder",
			Name:      "milestone_mark_error_total",
			Help:      "Total errors while marking milestones as sent",
		}, []string{"milestone_type"}),
	}
}

type PromMetrics struct{}

func NewPromMetrics() *PromMetrics {
	promInitOnce.Do(func() {
		promSet = newPromMetricsSet()
		prometheus.MustRegister(
			promSet.configUpsertTotal,
			promSet.occurrenceCreatedTotal,
			promSet.dispatchAttemptTotal,
			promSet.retryAlertThresholdTotal,
			promSet.slaBreachTotal,
			promSet.dispatchDueLatencyMs,
			promSet.milestoneSeedTotal,
			promSet.milestoneSeedErrTotal,
			promSet.milestoneMarkErrTotal,
		)
	})
	return &PromMetrics{}
}

func (p *PromMetrics) IncCounter(name string, tags map[string]string) {
	switch name {
	case "reminder_config_upsert_total":
		promSet.configUpsertTotal.WithLabelValues(tags["scope_type"]).Inc()
	case "reminder_occurrence_created_total":
		promSet.occurrenceCreatedTotal.WithLabelValues(tags["source"]).Inc()
	case "reminder_dispatch_attempt_total":
		promSet.dispatchAttemptTotal.WithLabelValues(tags["status"]).Inc()
	case "reminder_retry_alert_threshold_total":
		promSet.retryAlertThresholdTotal.WithLabelValues(tags["threshold"]).Inc()
	case "reminder_sla_breach_total":
		promSet.slaBreachTotal.WithLabelValues(tags["window"]).Inc()
	case "reminder_milestone_seeded_total":
		promSet.milestoneSeedTotal.WithLabelValues(tags["milestone_type"]).Inc()
	case "reminder_milestone_seed_error_total":
		promSet.milestoneSeedErrTotal.WithLabelValues(tags["milestone_type"]).Inc()
	case "reminder_milestone_mark_error_total":
		promSet.milestoneMarkErrTotal.WithLabelValues(tags["milestone_type"]).Inc()
	default:
		_ = stableTagString(tags)
	}
}

func (p *PromMetrics) ObserveLatency(name string, ms int64, tags map[string]string) {
	switch name {
	case "reminder_dispatch_due_ms":
		promSet.dispatchDueLatencyMs.WithLabelValues(tags["status"]).Observe(float64(ms))
	default:
		_ = stableTagString(tags)
	}
}

func stableTagString(tags map[string]string) string {
	keys := make([]string, 0, len(tags))
	for k := range tags {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := ""
	for _, k := range keys {
		out += k + "=" + tags[k] + ";"
	}
	return out
}
