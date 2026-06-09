package email

import (
	"context"
	"errors"
	"net"
	"net/smtp"
	"testing"
	"time"
)

func timeoutTestPayload() map[string]any {
	return map[string]any{
		"title":         "Q1 Report",
		"deadline_date": "2026-06-30",
		"disclosure_id": "DISC-1",
	}
}

func newTimeoutTestSender() *Sender {
	return NewSMTPSender(SMTPConfig{Host: "smtp.example.com", Port: 587, From: "no-reply@cobo.local"})
}

// netTimeoutErr implements net.Error with Timeout()=true.
type netTimeoutErr struct{}

func (netTimeoutErr) Error() string   { return "dial tcp: i/o timeout" }
func (netTimeoutErr) Timeout() bool   { return true }
func (netTimeoutErr) Temporary() bool { return true }

var _ net.Error = netTimeoutErr{}

func TestSendReminderEmail_TimeoutClassifiedTemporary(t *testing.T) {
	s := newTimeoutTestSender()
	s.sendMail = func(string, smtp.Auth, string, []string, []byte) error { return netTimeoutErr{} }

	_, err := s.SendReminderEmail(context.Background(), "REMINDER_DISCLOSURE_DUE", timeoutTestPayload(), []string{"a@b.com"}, "k")
	var te TemporaryError
	if !errors.As(err, &te) {
		t.Fatalf("timeout err = %T (%v), want TemporaryError", err, err)
	}
}

func TestSendReminderEmail_TransientClassifiedTemporary(t *testing.T) {
	s := newTimeoutTestSender()
	s.sendMail = func(string, smtp.Auth, string, []string, []byte) error {
		return errors.New("421 4.7.0 Service temporarily unavailable")
	}

	_, err := s.SendReminderEmail(context.Background(), "REMINDER_DISCLOSURE_DUE", timeoutTestPayload(), []string{"a@b.com"}, "k")
	var te TemporaryError
	if !errors.As(err, &te) {
		t.Fatalf("transient err = %T (%v), want TemporaryError", err, err)
	}
}

func TestSendReminderEmail_PermanentClassifiedPermanent(t *testing.T) {
	s := newTimeoutTestSender()
	s.sendMail = func(string, smtp.Auth, string, []string, []byte) error {
		return errors.New("550 5.1.1 mailbox not found")
	}

	_, err := s.SendReminderEmail(context.Background(), "REMINDER_DISCLOSURE_DUE", timeoutTestPayload(), []string{"a@b.com"}, "k")
	var pe PermanentError
	if !errors.As(err, &pe) {
		t.Fatalf("permanent err = %T (%v), want PermanentError", err, err)
	}
}

// TestBoundedSendMail_DoesNotBlockForever proves the connection deadline trips against a
// server that accepts the TCP connection but never speaks SMTP, so the worker goroutine
// is freed instead of hanging indefinitely.
func TestBoundedSendMail_DoesNotBlockForever(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			// Hold the connection open and silent: never send the SMTP greeting.
			go func(c net.Conn) { time.Sleep(5 * time.Second); c.Close() }(conn)
		}
	}()

	s := NewSMTPSender(SMTPConfig{Host: "127.0.0.1", From: "no-reply@cobo.local"})
	s.dialTimeout = 500 * time.Millisecond
	s.opTimeout = 300 * time.Millisecond

	done := make(chan error, 1)
	start := time.Now()
	go func() { done <- s.boundedSendMail(ln.Addr().String(), nil, "from@x", []string{"to@y"}, []byte("msg")) }()

	select {
	case err := <-done:
		if err == nil {
			t.Fatalf("expected a deadline error, got nil")
		}
		if elapsed := time.Since(start); elapsed > 2*time.Second {
			t.Fatalf("boundedSendMail took %v, expected to be bounded near opTimeout", elapsed)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("boundedSendMail did not return within bound — it blocked")
	}
}
