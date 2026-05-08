package app

import "context"

type Metrics interface {
	IncCounter(name string, tags map[string]string)
	ObserveLatency(name string, ms int64, tags map[string]string)
}

type Auditor interface {
	Record(ctx context.Context, action string, resourceType string, resourceID string, metadata map[string]any)
}

type AlertHook interface {
	Notify(ctx context.Context, alertCode string, labels map[string]string, metadata map[string]any)
}

type noopMetrics struct{}

func (noopMetrics) IncCounter(string, map[string]string)            {}
func (noopMetrics) ObserveLatency(string, int64, map[string]string) {}

type noopAuditor struct{}

func (noopAuditor) Record(context.Context, string, string, string, map[string]any) {}

type noopAlertHook struct{}

func (noopAlertHook) Notify(context.Context, string, map[string]string, map[string]any) {}
