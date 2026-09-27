package smtp

import (
	"context"
	"errors"
	"fmt"
	"net/smtp"
	"strings"
)

// ErrBindingConfigMissing is fail-closed. Binding delivery must not treat an
// empty host as success and must not use the log-only or mock-no-smtp paths.
var ErrBindingConfigMissing = errors.New("smtp_config_missing")

// BindingMessage is one binding email. The caller has already claimed the row.
type BindingMessage struct {
	Recipients []string
	Subject    string
	TextBody   string
	HTMLBody   string
}

// BindingMailer sends with the existing SMTP adapter transport. It never logs
// the password, the recipient, or the body.
type BindingMailer struct {
	cfg    Config
	sendFn func(addr string, auth smtp.Auth, from string, to []string, msg []byte) error
}

// NewBindingMailer returns a mailer that dials only when host and port are set.
// An empty host returns ErrBindingConfigMissing from Send and does not call sendFn.
func NewBindingMailer(cfg Config, sendOverride func(addr string, auth smtp.Auth, from string, to []string, msg []byte) error) *BindingMailer {
	m := &BindingMailer{cfg: cfg, sendFn: sendOverride}
	if m.sendFn == nil {
		m.sendFn = smtp.SendMail
	}
	return m
}

func (m *BindingMailer) Send(_ context.Context, msg BindingMessage) (string, error) {
	if strings.TrimSpace(m.cfg.Host) == "" || m.cfg.Port <= 0 {
		return "", ErrBindingConfigMissing
	}
	if len(msg.Recipients) == 0 {
		return "", fmt.Errorf("550 invalid recipient")
	}
	from := strings.TrimSpace(m.cfg.From)
	if from == "" {
		from = "no-reply@cobo.local"
	}
	payload, messageID := BuildMessage(from, strings.Join(msg.Recipients, ","), msg.Subject, msg.TextBody, msg.HTMLBody)
	addr := fmt.Sprintf("%s:%d", m.cfg.Host, m.cfg.Port)
	var auth smtp.Auth
	if strings.TrimSpace(m.cfg.User) != "" {
		auth = smtp.PlainAuth("", m.cfg.User, m.cfg.Pass, m.cfg.Host)
	}
	if err := m.sendFn(addr, auth, from, msg.Recipients, payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(messageID) == "" {
		return "", errors.New("uncertain empty provider id")
	}
	return messageID, nil
}
