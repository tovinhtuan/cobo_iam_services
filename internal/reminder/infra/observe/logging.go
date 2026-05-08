package observe

import (
	"context"
	"log/slog"
	"time"

	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
	reminderapp "github.com/cobo/cobo_iam_services/internal/reminder/app"
)

type LogMetrics struct {
	Log *slog.Logger
}

func (m LogMetrics) IncCounter(name string, tags map[string]string) {
	if m.Log == nil {
		return
	}
	m.Log.Info("metric_counter", slog.String("name", name), slog.Any("tags", tags), slog.Int("value", 1))
}

func (m LogMetrics) ObserveLatency(name string, ms int64, tags map[string]string) {
	if m.Log == nil {
		return
	}
	m.Log.Info("metric_latency", slog.String("name", name), slog.Int64("ms", ms), slog.Any("tags", tags))
}

type AuditRecorder struct {
	Svc auditapp.Service
	IDG idgen.Generator
}

func (a AuditRecorder) Record(ctx context.Context, action string, resourceType string, resourceID string, metadata map[string]any) {
	if a.Svc == nil || a.IDG == nil {
		return
	}
	_ = a.Svc.AppendAuditLog(ctx, auditapp.AppendAuditLogRequest{
		EventID:      a.IDG.NewUUID(),
		OccurredAt:   time.Now().UTC().Format(time.RFC3339),
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Decision:     "allow",
		Metadata:     metadata,
	})
}

type AlertLogger struct {
	Log *slog.Logger
}

func (a AlertLogger) Notify(_ context.Context, alertCode string, labels map[string]string, metadata map[string]any) {
	if a.Log == nil {
		return
	}
	a.Log.Warn("reminder_alert", slog.String("alert_code", alertCode), slog.Any("labels", labels), slog.Any("metadata", metadata))
}

var _ reminderapp.Metrics = (*LogMetrics)(nil)
var _ reminderapp.Auditor = (*AuditRecorder)(nil)
var _ reminderapp.AlertHook = (*AlertLogger)(nil)
