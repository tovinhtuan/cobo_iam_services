package http

import (
	"sync"
	"time"
)

type cmsRouteMetric struct {
	Requests      int64  `json:"requests"`
	Errors        int64  `json:"errors"`
	AvgLatencyMs  int64  `json:"avg_latency_ms"`
	LastStatus    int    `json:"last_status"`
	LastLatencyMs int64  `json:"last_latency_ms"`
	LastRequestID string `json:"last_request_id,omitempty"`
}

type cmsMetrics struct {
	mu     sync.Mutex
	routes map[string]cmsRouteMetric
}

func newCMSMetrics() *cmsMetrics {
	return &cmsMetrics{routes: map[string]cmsRouteMetric{}}
}

func (m *cmsMetrics) record(route, requestID string, status int, latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	metric := m.routes[route]
	metric.Requests++
	if status >= 400 {
		metric.Errors++
	}
	latencyMs := latency.Milliseconds()
	metric.LastStatus = status
	metric.LastLatencyMs = latencyMs
	metric.LastRequestID = requestID
	if metric.Requests == 1 {
		metric.AvgLatencyMs = latencyMs
	} else {
		metric.AvgLatencyMs = ((metric.AvgLatencyMs * (metric.Requests - 1)) + latencyMs) / metric.Requests
	}
	m.routes[route] = metric
}

func (m *cmsMetrics) snapshot() map[string]cmsRouteMetric {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]cmsRouteMetric, len(m.routes))
	for key, value := range m.routes {
		out[key] = value
	}
	return out
}
