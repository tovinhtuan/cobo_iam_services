package email

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"

	notificationapp "github.com/cobo/cobo_iam_services/internal/notification/app"
	notificationsmtp "github.com/cobo/cobo_iam_services/internal/notification/infra/smtp"
)

// SMTP timeout hardening (Reminder Reliability Hardening, P0). The previous
// net/smtp.SendMail dials and reads/writes with NO deadline, so a wedged or slow MX can
// block a worker goroutine indefinitely and silently stall the whole reminder pipeline.
// boundedSendMail (below) replaces it with: net.DialTimeout (connection timeout) +
// conn.SetDeadline (bounds every subsequent read and write). All three — connect, write,
// read — are therefore bounded. Chosen approach: Option A (custom dialer timeout) plus
// Option B-style deadline on the live connection.
const (
	smtpDialTimeout = 10 * time.Second
	smtpOpTimeout   = 30 * time.Second
)

type SMTPConfig struct {
	Host string
	Port int
	User string
	Pass string
	From string
}

type Sender struct {
	cfg            SMTPConfig
	templateSource string
	registry       notificationapp.TemplateRegistry
	renderer       notificationapp.EmailRenderer
	dialTimeout    time.Duration
	opTimeout      time.Duration
	// sendMail is the transport seam: defaults to boundedSendMail (real, timeout-bounded
	// SMTP) and is overridden in tests to exercise timeout / transient / permanent paths.
	sendMail func(addr string, auth smtp.Auth, from string, to []string, msg []byte) error
}

type Option func(*Sender)

func WithTemplateRendering(source string, registry notificationapp.TemplateRegistry, renderer notificationapp.EmailRenderer) Option {
	return func(s *Sender) {
		s.templateSource = source
		s.registry = registry
		s.renderer = renderer
	}
}

func NewSMTPSender(cfg SMTPConfig, opts ...Option) *Sender {
	s := &Sender{
		cfg:            cfg,
		templateSource: "legacy",
		dialTimeout:    smtpDialTimeout,
		opTimeout:      smtpOpTimeout,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}
	if s.sendMail == nil {
		s.sendMail = s.boundedSendMail
	}
	return s
}

func (s *Sender) SendReminderEmail(_ context.Context, templateCode string, payload map[string]any, recipients []string, _ string) (string, error) {
	if strings.TrimSpace(templateCode) == "" {
		return "", PermanentError{Err: fmt.Errorf("template_code is required")}
	}
	if len(recipients) == 0 {
		return "", PermanentError{Err: fmt.Errorf("recipient list is empty")}
	}
	subject, textBody, htmlBody, err := s.renderReminderEmailContent(templateCode, payload)
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
	msg, _ := notificationsmtp.BuildMessage(from, strings.Join(recipients, ","), subject, textBody, htmlBody)

	var auth smtp.Auth
	if strings.TrimSpace(s.cfg.User) != "" {
		auth = smtp.PlainAuth("", s.cfg.User, s.cfg.Pass, s.cfg.Host)
	}
	if err := s.sendMail(addr, auth, from, recipients, msg); err != nil {
		if isTemporarySMTPError(err) {
			return "", TemporaryError{Err: err}
		}
		return "", PermanentError{Err: err}
	}
	return fmt.Sprintf("smtp-%d", time.Now().UnixNano()), nil
}

// boundedSendMail performs an SMTP delivery with bounded connect/read/write timeouts so a
// slow or wedged server can never block the worker goroutine indefinitely. It mirrors the
// handshake net/smtp.SendMail performs (EHLO → STARTTLS if offered → AUTH → MAIL/RCPT/DATA)
// but on a connection dialed with a timeout and capped by an absolute deadline.
func (s *Sender) boundedSendMail(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
	dialTimeout := s.dialTimeout
	if dialTimeout <= 0 {
		dialTimeout = smtpDialTimeout
	}
	opTimeout := s.opTimeout
	if opTimeout <= 0 {
		opTimeout = smtpOpTimeout
	}

	conn, err := net.DialTimeout("tcp", addr, dialTimeout)
	if err != nil {
		return err
	}
	// Bound every subsequent read and write; a wedged peer trips this deadline instead
	// of hanging forever.
	_ = conn.SetDeadline(time.Now().Add(opTimeout))

	c, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		_ = conn.Close()
		return err
	}
	defer func() { _ = c.Close() }()

	if ok, _ := c.Extension("STARTTLS"); ok {
		if err := c.StartTLS(&tls.Config{ServerName: s.cfg.Host}); err != nil {
			return err
		}
	}
	if auth != nil {
		if ok, _ := c.Extension("AUTH"); ok {
			if err := c.Auth(auth); err != nil {
				return err
			}
		}
	}
	if err := c.Mail(from); err != nil {
		return err
	}
	for _, rcpt := range to {
		if err := c.Rcpt(rcpt); err != nil {
			return err
		}
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

func (s *Sender) renderReminderEmailContent(templateCode string, payload map[string]any) (subject, textBody, htmlBody string, err error) {
	if s.templateSource == "embed" && s.registry != nil && s.renderer != nil {
		if key, ok := reminderTemplateKey(templateCode); ok {
			resolved, err := s.registry.Resolve(context.Background(), key, "vi")
			if err == nil {
				rendered, renderErr := s.renderer.Render(resolved, payload)
				if renderErr == nil {
					return rendered.Subject, rendered.TextBody, rendered.HTMLBody, nil
				}
			}
		}
	}
	return renderReminderEmail(templateCode, payload)
}

func reminderTemplateKey(templateCode string) (string, bool) {
	switch strings.TrimSpace(templateCode) {
	case "REMINDER_DISCLOSURE_DUE":
		return "reminder.disclosure_deadline", true
	default:
		// Pass-through: treat non-legacy codes as direct template keys (e.g. reminder.deadline_approaching).
		if key := strings.TrimSpace(templateCode); key != "" {
			return key, true
		}
		return "", false
	}
}

func renderReminderEmail(templateCode string, payload map[string]any) (subject, textBody, htmlBody string, err error) {
	switch strings.TrimSpace(templateCode) {
	case "REMINDER_DISCLOSURE_DUE":
		// Title/deadline prefer the augmented human-readable keys (disclosure_title,
		// due_date in dd/mm/yyyy); fall back to the raw payload keys for safety. The
		// disclosure_id (UUID) and status enum are intentionally NOT surfaced to the
		// recipient — this is a business communication, not a debug record.
		title := requiredString(payload, "disclosure_title")
		if title == "" {
			title = requiredString(payload, "title")
		}
		deadline := requiredString(payload, "due_date")
		if deadline == "" {
			deadline = requiredString(payload, "deadline_date")
		}
		companyName := optionalString(payload, "company_name")
		actionURL := optionalString(payload, "portal_url")
		if actionURL == "" {
			actionURL = optionalString(payload, "action_url")
		}
		if title == "" || deadline == "" {
			return "", "", "", fmt.Errorf("missing required reminder template fields")
		}
		subject := fmt.Sprintf("[CoBo] Nhắc nhở: %s sắp đến hạn vào %s", title, deadline)
		lines := []string{
			"Kính gửi,",
			"",
			"Đây là thông báo nhắc nhở tự động từ hệ thống CoBo Portal.",
			"",
		}
		if companyName != "" {
			lines = append(lines, "Công ty: "+companyName)
		}
		lines = append(lines,
			"Nghĩa vụ công bố: "+title,
			"Hạn nộp: "+deadline,
			"",
			"Vui lòng truy cập hệ thống để hoàn thành nghĩa vụ công bố thông tin trước thời hạn:",
		)
		if actionURL != "" {
			lines = append(lines, "", actionURL)
		}
		lines = append(lines,
			"",
			"Lưu ý: Email này được gửi tự động, vui lòng không trả lời trực tiếp.",
			"",
			"Trân trọng,",
			"Hệ thống CoBo Portal",
		)
		return subject, strings.Join(lines, "\n"), "", nil
	default:
		return "", "", "", fmt.Errorf("unsupported reminder template_code: %s", templateCode)
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
