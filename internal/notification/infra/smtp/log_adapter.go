package smtp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"

	notificationapp "github.com/cobo/cobo_iam_services/internal/notification/app"
)

// LogOnlyAdapter is the no-SMTP variant used when SMTP_HOST is empty. It logs
// a fixed-shape record per send and returns a synthetic Message-ID so the
// worker pipeline behaves identically to the production path. Email content
// (subject, body) is NEVER logged — only metadata sufficient to confirm the
// dispatch happened and route a support inquiry.
type LogOnlyAdapter struct {
	logger *slog.Logger
}

func NewLogOnlyAdapter() *LogOnlyAdapter {
	return &LogOnlyAdapter{logger: slog.Default()}
}

// NewLogOnlyAdapterWith lets tests inject a logger that captures records.
func NewLogOnlyAdapterWith(l *slog.Logger) *LogOnlyAdapter {
	if l == nil {
		l = slog.Default()
	}
	return &LogOnlyAdapter{logger: l}
}

func (a *LogOnlyAdapter) Send(_ context.Context, msg notificationapp.DeliveryMessage) (notificationapp.DeliveryResult, error) {
	if strings.TrimSpace(msg.To) == "" {
		return notificationapp.DeliveryResult{}, fmt.Errorf("550 invalid recipient: empty To")
	}
	var buf [8]byte
	_, _ = rand.Read(buf[:])
	id := "mock-" + hex.EncodeToString(buf[:])
	a.logger.Info("email.log_only.sent",
		"notification_id", msg.NotificationID,
		"to", maskEmail(msg.To),
		"subject_len", len(msg.Subject),
		"text_body_len", len(msg.TextBody),
		"html_body_len", len(msg.HTMLBody),
		"provider_message_id", id,
	)
	return notificationapp.DeliveryResult{ProviderMessageID: id, Provider: "log_only"}, nil
}

// maskEmail returns a redacted form of an address suitable for structured
// logs: "n****@example.com". Used by the log-only adapter and may be reused
// by the worker when emitting events.
func maskEmail(addr string) string {
	addr = strings.TrimSpace(addr)
	at := strings.LastIndex(addr, "@")
	if at <= 0 {
		return "<invalid>"
	}
	local := addr[:at]
	domain := addr[at:]
	if len(local) <= 1 {
		return local + "****" + domain
	}
	return string(local[0]) + strings.Repeat("*", min(len(local)-1, 4)) + domain
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
