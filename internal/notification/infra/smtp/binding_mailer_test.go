package smtp

import (
	"context"
	"net/smtp"
	"testing"
)

func TestBindingMailerEmptyHostDoesNotDial(t *testing.T) {
	calls := 0
	m := NewBindingMailer(Config{Host: " ", Port: 587}, func(string, smtp.Auth, string, []string, []byte) error {
		calls++
		return nil
	})
	_, err := m.Send(context.Background(), BindingMessage{Recipients: []string{"a@example.com"}, Subject: "s", TextBody: "t"})
	if err != ErrBindingConfigMissing {
		t.Fatalf("err=%v", err)
	}
	if calls != 0 {
		t.Fatalf("dialed %d", calls)
	}
}

func TestBindingMailerAcceptedUsesTransport(t *testing.T) {
	calls := 0
	m := NewBindingMailer(Config{Host: "smtp.example", Port: 587, From: "from@example.com"}, func(addr string, _ smtp.Auth, from string, to []string, msg []byte) error {
		calls++
		if addr != "smtp.example:587" || from != "from@example.com" || len(to) != 1 || len(msg) == 0 {
			t.Fatalf("addr=%s from=%s to=%v", addr, from, to)
		}
		return nil
	})
	id, err := m.Send(context.Background(), BindingMessage{Recipients: []string{"a@example.com"}, Subject: "s", TextBody: "t"})
	if err != nil || id == "" || calls != 1 {
		t.Fatalf("id=%q err=%v calls=%d", id, err, calls)
	}
}
