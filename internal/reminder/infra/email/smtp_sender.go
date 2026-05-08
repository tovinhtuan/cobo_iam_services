package email

import (
	"context"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"
)

type SMTPConfig struct {
	Host string
	Port int
	User string
	Pass string
	From string
}

type Sender struct {
	cfg SMTPConfig
}

func NewSMTPSender(cfg SMTPConfig) *Sender {
	return &Sender{cfg: cfg}
}

func (s *Sender) SendReminderEmail(_ context.Context, templateCode string, payload map[string]any, recipients []string, _ string) (string, error) {
	if strings.TrimSpace(templateCode) == "" {
		return "", PermanentError{Err: fmt.Errorf("template_code is required")}
	}
	if len(recipients) == 0 {
		return "", PermanentError{Err: fmt.Errorf("recipient list is empty")}
	}
	subject, body, err := renderReminderEmail(templateCode, payload)
	if err != nil {
		return "", PermanentError{Err: err}
	}
	if strings.TrimSpace(s.cfg.Host) == "" {
		// In local/no-smtp mode, treat as accepted to keep delivery pipeline testable.
		return "mock-no-smtp", nil
	}

	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	from := strings.TrimSpace(s.cfg.From)
	if from == "" {
		from = "no-reply@cobo.local"
	}
	msg := "From: " + from + "\r\n" +
		"To: " + strings.Join(recipients, ",") + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n\r\n" +
		body + "\r\n"

	var auth smtp.Auth
	if strings.TrimSpace(s.cfg.User) != "" {
		auth = smtp.PlainAuth("", s.cfg.User, s.cfg.Pass, s.cfg.Host)
	}
	if err := smtp.SendMail(addr, auth, from, recipients, []byte(msg)); err != nil {
		if isTemporarySMTPError(err) {
			return "", TemporaryError{Err: err}
		}
		return "", PermanentError{Err: err}
	}
	return fmt.Sprintf("smtp-%d", time.Now().UnixNano()), nil
}

func renderReminderEmail(templateCode string, payload map[string]any) (string, string, error) {
	switch strings.TrimSpace(templateCode) {
	case "REMINDER_DISCLOSURE_DUE":
		title := requiredString(payload, "title")
		deadline := requiredString(payload, "deadline_date")
		disclosureID := requiredString(payload, "disclosure_id")
		actionURL := optionalString(payload, "action_url")
		if title == "" || deadline == "" || disclosureID == "" {
			return "", "", fmt.Errorf("missing required reminder template fields")
		}
		subject := fmt.Sprintf("[COBO] Reminder: %s is due on %s", title, deadline)
		lines := []string{
			"Hello,",
			"",
			"This is an automated reminder for a disclosure task.",
			"",
			"Disclosure: " + title,
			"Disclosure ID: " + disclosureID,
			"Deadline: " + deadline,
		}
		if status := optionalString(payload, "status"); status != "" {
			lines = append(lines, "Current status: "+status)
		}
		if actionURL != "" {
			lines = append(lines, "Action link: "+actionURL)
		}
		lines = append(lines,
			"",
			"Please review and complete the required action before the deadline.",
			"",
			"COBO Notification System",
		)
		return subject, strings.Join(lines, "\r\n"), nil
	default:
		return "", "", fmt.Errorf("unsupported reminder template_code: %s", templateCode)
	}
}

func requiredString(payload map[string]any, key string) string {
	value, ok := payload[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func optionalString(payload map[string]any, key string) string {
	value, ok := payload[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

type TemporaryError struct {
	Err error
}

func (e TemporaryError) Error() string { return e.Err.Error() }
func (e TemporaryError) Unwrap() error { return e.Err }
func (TemporaryError) Temporary() bool { return true }

type PermanentError struct {
	Err error
}

func (e PermanentError) Error() string { return e.Err.Error() }
func (e PermanentError) Unwrap() error { return e.Err }

func isTemporarySMTPError(err error) bool {
	if ne, ok := err.(net.Error); ok {
		return ne.Timeout() || ne.Temporary()
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") || strings.Contains(msg, "temporar") || strings.Contains(msg, "421") || strings.Contains(msg, "450") || strings.Contains(msg, "451") || strings.Contains(msg, "452")
}
