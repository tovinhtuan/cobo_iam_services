package smtp

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"

	notificationapp "github.com/cobo/cobo_iam_services/internal/notification/app"
)

// Config carries the SMTP credentials the adapter dials. Sender (From) is the
// envelope-from used at MAIL FROM; it MUST be set to a deliverable address or
// most relays will refuse.
type Config struct {
	Host string
	Port int
	User string
	Pass string
	From string
}

// Adapter is the production net/smtp implementation of
// notificationapp.DeliveryAdapter. When Host is empty the constructor
// returns a LogOnlyAdapter instead so dev/test/CI never reach out to a real
// MTA by accident.
type Adapter struct {
	cfg Config
	// sendFn is overridable in tests so we can simulate transient/permanent
	// failures without standing up a real SMTP server.
	sendFn func(addr string, auth smtp.Auth, from string, to []string, msg []byte) error
}

// NewAdapter returns the SMTP-or-log-only adapter depending on cfg.Host.
// Pass a non-nil sendOverride only in tests.
func NewAdapter(cfg Config, sendOverride func(addr string, auth smtp.Auth, from string, to []string, msg []byte) error) notificationapp.DeliveryAdapter {
	if strings.TrimSpace(cfg.Host) == "" {
		return NewLogOnlyAdapter()
	}
	a := &Adapter{cfg: cfg, sendFn: sendOverride}
	if a.sendFn == nil {
		a.sendFn = smtp.SendMail
	}
	return a
}

func (a *Adapter) Send(_ context.Context, msg notificationapp.DeliveryMessage) (notificationapp.DeliveryResult, error) {
	if strings.TrimSpace(msg.To) == "" {
		return notificationapp.DeliveryResult{}, fmt.Errorf("550 invalid recipient: empty To")
	}
	from := strings.TrimSpace(a.cfg.From)
	if from == "" {
		from = "no-reply@cobo.local"
	}
	payload, messageID := BuildMessage(from, msg.To, msg.Subject, msg.TextBody, msg.HTMLBody)
	addr := fmt.Sprintf("%s:%d", a.cfg.Host, a.cfg.Port)
	var auth smtp.Auth
	if strings.TrimSpace(a.cfg.User) != "" {
		auth = smtp.PlainAuth("", a.cfg.User, a.cfg.Pass, a.cfg.Host)
	}
	if err := a.sendFn(addr, auth, from, []string{msg.To}, payload); err != nil {
		return notificationapp.DeliveryResult{}, err
	}
	return notificationapp.DeliveryResult{ProviderMessageID: messageID, Provider: "smtp"}, nil
}
