package observability

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	promInitOnce     sync.Once
	transitionTotal  *prometheus.CounterVec
	emailShadowTotal *prometheus.CounterVec
)

// PromMetrics implements adhocapp.Metrics, emitting:
//   - cobo_adhoc_proposal_transition_total (Batch 5(a) / AK.3 — the program's
//     minimum-viable instrumentation metric, proven against real Batch-1 traffic
//     before Batch 2A/Batch 2's higher-risk work begins)
//   - cobo_adhoc_email_shadow_total (Batch 2 / §AK.5 L657 — the Shadow Mode
//     comparison gate metric; "mismatch" readings drive the 24h/48h rollout
//     window's zero-tolerance reset rule)
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

		emailShadowTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "cobo",
			Subsystem: "adhoc",
			Name:      "email_shadow_total",
			Help:      "Shadow Mode email pipeline comparison outcomes (match|mismatch) per §AK.5 L657's idempotency-key collision check",
		}, []string{"company_id", "outcome"})
		prometheus.MustRegister(emailShadowTotal)
	})
	return &PromMetrics{}
}

func (p *PromMetrics) RecordTransition(companyID, fromStatus, toStatus string) {
	transitionTotal.WithLabelValues(companyID, fromStatus, toStatus).Inc()
}

func (p *PromMetrics) RecordEmailShadowOutcome(companyID, outcome string) {
	emailShadowTotal.WithLabelValues(companyID, outcome).Inc()
}
