package workflowdept

import (
	"bytes"
	"context"
	"errors"
	"testing"

	notificationsmtp "github.com/cobo/cobo_iam_services/internal/notification/infra/smtp"
)

func TestClassifiedSendMapping(t *testing.T) {
	id, uncertain, permanent := ClassifiedSend("smtp-1", nil)
	if id != "smtp-1" || uncertain || permanent {
		t.Fatalf("accepted %s %v %v", id, uncertain, permanent)
	}
	if _, uncertain, permanent = ClassifiedSend("", errors.New("dial timeout")); !uncertain || permanent {
		t.Fatal("timeout must be uncertain")
	}
	if _, uncertain, permanent = ClassifiedSend("", errors.New("421 busy")); uncertain || permanent {
		t.Fatal("421 must stay retryable")
	}
	if _, uncertain, permanent = ClassifiedSend("", errors.New("550 mailbox")); uncertain || !permanent {
		t.Fatal("550 must be permanent")
	}
	if _, uncertain, permanent = ClassifiedSend("", notificationsmtp.ErrBindingConfigMissing); uncertain || !permanent {
		t.Fatal("missing config must be permanent")
	}
}

func TestInvalidRecipientDoesNotSend(t *testing.T) {
	key := bytes.Repeat([]byte{6}, 32)
	blob, err := EncryptRecipientEmails(key, 1, "c1", "occ", "res", []byte("not-an-email"))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvBindingEnabled, "true")
	t.Setenv(EnvEmailBindingEnabled, "true")
	sender := &countingSender{id: "smtp-1"}
	store := newMemStore(&DeliveryResolution{
		ResolutionID: "res", CompanyID: "c1", OccurrenceID: "occ", Ciphertext: blob, KeyVersion: 1, SendStatus: SendPending,
	})
	loaded := RecipientKey{Key: key, Version: 1}
	if err := RunBindingTick(context.Background(), BindingDeps{
		Store: store, Sender: sender, Key: &loaded, SMTPHost: "smtp.example",
	}); err != nil {
		t.Fatal(err)
	}
	if sender.calls != 0 || store.must("res").SendStatus != SendPermanentFailed || store.must("res").LastErrorCode != ErrCodeInvalidRecipient {
		t.Fatalf("status=%s code=%s calls=%d", store.must("res").SendStatus, store.must("res").LastErrorCode, sender.calls)
	}
}
