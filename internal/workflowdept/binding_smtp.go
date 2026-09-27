package workflowdept

import (
	"context"
	"errors"
	"net/mail"
	"strings"

	notificationsmtp "github.com/cobo/cobo_iam_services/internal/notification/infra/smtp"
)

const (
	ErrCodeInvalidRecipient  = "invalid_recipient"
	ErrCodeSMTPConfigMissing = "smtp_config_missing"
)

// ClassifiedSend maps one SMTP result onto the binding state machine.
func ClassifiedSend(providerID string, err error) (id string, uncertain, permanent bool) {
	if errors.Is(err, notificationsmtp.ErrBindingConfigMissing) {
		return "", false, true
	}
	if err == nil {
		if strings.TrimSpace(providerID) == "" {
			return "", true, false
		}
		return strings.TrimSpace(providerID), false, false
	}
	if isUncertain(err) {
		return "", true, false
	}
	msg := strings.ToLower(err.Error())
	for _, code := range []string{"550", "551", "553", "554"} {
		if strings.Contains(msg, code) {
			return "", false, true
		}
	}
	return "", false, false
}

// SMTPBindingSender adapts the existing notification SMTP mailer. It does not
// call the legacy reminder sender and does not dial when the mailer refuses config.
type SMTPBindingSender struct {
	mailer interface {
		Send(ctx context.Context, msg notificationsmtp.BindingMessage) (string, error)
	}
}

func NewSMTPBindingSender(mailer interface {
	Send(ctx context.Context, msg notificationsmtp.BindingMessage) (string, error)
}) SMTPBindingSender {
	return SMTPBindingSender{mailer: mailer}
}

func (s SMTPBindingSender) Send(ctx context.Context, recipients []string) (string, bool, bool, error) {
	if s.mailer == nil {
		return "", false, true, notificationsmtp.ErrBindingConfigMissing
	}
	id, err := s.mailer.Send(ctx, notificationsmtp.BindingMessage{
		Recipients: recipients,
		Subject:    "[CoBo] Thong bao phong ban",
		TextBody:   "Thong bao tu dong tu CoBo. Vui long dang nhap he thong de xem chi tiet.",
	})
	if errors.Is(err, notificationsmtp.ErrBindingConfigMissing) {
		return "", false, true, err
	}
	got, uncertain, permanent := ClassifiedSend(id, err)
	return got, uncertain, permanent, err
}

func validRecipients(in []string) ([]string, bool) {
	out := make([]string, 0, len(in))
	for _, raw := range in {
		addr, err := mail.ParseAddress(strings.TrimSpace(raw))
		if err != nil || strings.TrimSpace(addr.Address) == "" || !strings.Contains(addr.Address, "@") {
			return nil, false
		}
		out = append(out, addr.Address)
	}
	return out, len(out) > 0
}
