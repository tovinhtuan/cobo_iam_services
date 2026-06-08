package observability

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	promInitOnce    sync.Once
	transitionTotal *prometheus.CounterVec
)

// PromMetrics implements adhocapp.Metrics, emitting cobo_adhoc_proposal_transition_total
// (Batch 5(a) / AK.3 — the program's minimum-viable instrumentation metric, proven
// against real Batch-1 traffic before Batch 2A/Batch 2's higher-risk work begins).
type PromMetrics struct{}

func NewPromMetrics() *PromMetrics {
	promInitOnce.Do(func() {
		transitionTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "cobo",
			Subsystem: "adhoc",
			Name:      "proposal_transition_total",
			Help:      "Total ad-hoc proposal status transitions actually applied (excludes idempotent replays)",
		}, []string{"company_id", "from_status", "to_status"})
		prometheus.MustRegister(transitionTotal)
	})
	return &PromMetrics{}
}

func (p *PromMetrics) RecordTransition(companyID, fromStatus, toStatus string) {
	transitionTotal.WithLabelValues(companyID, fromStatus, toStatus).Inc()
}
