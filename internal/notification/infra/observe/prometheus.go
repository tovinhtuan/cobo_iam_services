// Package observe holds the Prometheus-backed implementations of the
// notification app's observability seams. Keeping them here (infra) preserves
// the transport -> app -> infra layering: internal/notification/app never
// imports the Prometheus client directly, mirroring internal/reminder/infra/observe.
package observe

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	deliveryInitOnce   sync.Once
	emailDeliveryTotal *prometheus.CounterVec
)

// PromDeliveryMetrics implements notificationapp.DeliveryMetrics, emitting
// cobo_email_delivery_total{outcome,template_key}.
//
// outcome ∈ {sent, retry_scheduled, failed_permanent, render_error}. This is the
// Batch 2B delivery-observability metric: it turns the previously silent
// failed_permanent drop into an alertable signal. NOTE: the worker process must
// expose a /metrics endpoint for this counter to be scraped (see Batch 2B
// follow-up); until then the handler's structured failed_permanent log is the
// live alert source.
type PromDeliveryMetrics struct{}

func NewPromDeliveryMetrics() *PromDeliveryMetrics {
	deliveryInitOnce.Do(func() {
		emailDeliveryTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "cobo",
			Subsystem: "email",
			Name:      "delivery_total",
			Help:      "Email delivery outcomes by terminal-for-attempt result and template (Batch 2B)",
		}, []string{"outcome", "template_key"})
		prometheus.MustRegister(emailDeliveryTotal)
	})
	return &PromDeliveryMetrics{}
}

func (p *PromDeliveryMetrics) RecordDelivery(outcome, templateKey string) {
	if emailDeliveryTotal == nil {
		return
	}
	emailDeliveryTotal.WithLabelValues(outcome, templateKey).Inc()
}
