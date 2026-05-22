package smtp_test

import (
	"context"
	"errors"
	"net/smtp"
	"strings"
	"testing"

	notificationapp "github.com/cobo/cobo_iam_services/internal/notification/app"
	smtpinfra "github.com/cobo/cobo_iam_services/internal/notification/infra/smtp"
)

func TestNewAdapter_EmptyHostReturnsLogOnly(t *testing.T) {
	adapter := smtpinfra.NewAdapter(smtpinfra.Config{}, nil)
	res, err := adapter.Send(context.Background(), notificationapp.DeliveryMessage{
		NotificationID: "n-1", To: "a@example.com", Subject: "s", TextBody: "b",
	})
	if err != nil {
		t.Fatalf("log-only Send error = %v", err)
	}
	if res.Provider != "log_only" {
		t.Fatalf("provider = %q, want log_only", res.Provider)
	}
	if !strings.HasPrefix(res.ProviderMessageID, "mock-") {
		t.Fatalf("message id = %q, want mock-...", res.ProviderMessageID)
	}
}

func TestAdapter_Send_HappyPath(t *testing.T) {
	var (
		gotAddr string
		gotFrom string
		gotTo   []string
		gotMsg  []byte
	)
	adapter := smtpinfra.NewAdapter(smtpinfra.Config{
		Host: "mail.example.com", Port: 587, User: "u", Pass: "p", From: "no-reply@cobo.local",
	}, func(addr string, _ smtp.Auth, from string, to []string, msg []byte) error {
		gotAddr = addr
		gotFrom = from
		gotTo = to
		gotMsg = msg
		return nil
	})

	res, err := adapter.Send(context.Background(), notificationapp.DeliveryMessage{
		NotificationID: "n-1",
		To:             "nguyen@example.com",
		Subject:        "Verify your email",
		TextBody:       "Body line one\nBody line two",
	})
	if err != nil {
		t.Fatalf("Send error = %v", err)
	}
	if res.Provider != "smtp" {
		t.Fatalf("provider = %q", res.Provider)
	}
	if gotAddr != "mail.example.com:587" {
		t.Fatalf("addr = %q", gotAddr)
	}
	if gotFrom != "no-reply@cobo.local" {
		t.Fatalf("from = %q", gotFrom)
	}
	if len(gotTo) != 1 || gotTo[0] != "nguyen@example.com" {
		t.Fatalf("to = %v", gotTo)
	}
	out := string(gotMsg)
	if !strings.Contains(out, "Subject: ") {
		t.Fatalf("missing subject header:\n%s", out)
	}
}

func TestAdapter_Send_PropagatesErrorForClassification(t *testing.T) {
	adapter := smtpinfra.NewAdapter(smtpinfra.Config{Host: "mail.example.com", Port: 25, From: "no-reply@cobo.local"},
		func(string, smtp.Auth, string, []string, []byte) error {
			return errors.New("550 user not found")
		},
	)
	_, err := adapter.Send(context.Background(), notificationapp.DeliveryMessage{
		NotificationID: "n-1", To: "ghost@example.com", Subject: "s", TextBody: "b",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if notificationapp.ClassifySMTPError(err) != notificationapp.ErrorClassPermanent {
		t.Fatalf("error should classify as permanent: %v", err)
	}
}

func TestAdapter_Send_EmptyRecipientRejectedBeforeWire(t *testing.T) {
	called := false
	adapter := smtpinfra.NewAdapter(smtpinfra.Config{Host: "mail.example.com", Port: 25, From: "no-reply@cobo.local"},
		func(string, smtp.Auth, string, []string, []byte) error {
			called = true
			return nil
		},
	)
	_, err := adapter.Send(context.Background(), notificationapp.DeliveryMessage{NotificationID: "n-1", Subject: "s", TextBody: "b"})
	if err == nil {
		t.Fatal("expected error for empty recipient")
	}
	if called {
		t.Fatal("Send must reject empty recipient before reaching SMTP")
	}
}
