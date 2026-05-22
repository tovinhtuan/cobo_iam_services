package app_test

import (
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	notificationapp "github.com/cobo/cobo_iam_services/internal/notification/app"
)

func TestNextRetryDelay(t *testing.T) {
	tests := []struct {
		attemptNo int
		wantOK    bool
		want      time.Duration
	}{
		{1, true, 1 * time.Minute},
		{2, true, 5 * time.Minute},
		{3, true, 15 * time.Minute},
		{4, true, 1 * time.Hour},
		{5, false, 0}, // hit the cap, no more retries
		{0, false, 0},
		{-1, false, 0},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("attempt=%d", tt.attemptNo), func(t *testing.T) {
			d, ok := notificationapp.NextRetryDelay(tt.attemptNo)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && d != tt.want {
				t.Fatalf("delay = %v, want %v", d, tt.want)
			}
		})
	}
}

type fakeNetErr struct{}

func (fakeNetErr) Error() string   { return "fake network timeout" }
func (fakeNetErr) Timeout() bool   { return true }
func (fakeNetErr) Temporary() bool { return true }

// Force the type assertion path so net.Error wrapping is exercised.
var _ net.Error = fakeNetErr{}

func TestClassifySMTPError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want notificationapp.ErrorClass
	}{
		{"nil", nil, notificationapp.ErrorClassTransient},
		{"net error", fakeNetErr{}, notificationapp.ErrorClassTransient},
		{"transient 421", errors.New("421 service not available"), notificationapp.ErrorClassTransient},
		{"transient 450", errors.New("450 mailbox temporarily unavailable"), notificationapp.ErrorClassTransient},
		{"permanent 550", errors.New("550 user unknown"), notificationapp.ErrorClassPermanent},
		{"permanent 554", errors.New("554 transaction failed"), notificationapp.ErrorClassPermanent},
		{"auth 535", errors.New("535 authentication credentials invalid"), notificationapp.ErrorClassPermanentAuthOps},
		{"auth 530", errors.New("530 authentication required"), notificationapp.ErrorClassPermanentAuthOps},
		{"unknown defaults to transient", errors.New("something weird happened"), notificationapp.ErrorClassTransient},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := notificationapp.ClassifySMTPError(tt.err)
			if got != tt.want {
				t.Fatalf("class = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestRedactErrorMessage_StripsEmailsAndCaps(t *testing.T) {
	long := "550 user not found - recipient was unknown@example.com please contact support"
	got := notificationapp.RedactErrorMessage(errors.New(long))
	if got == "" {
		t.Fatal("expected non-empty redacted message")
	}
	if containsEmail(got) {
		t.Fatalf("email leaked: %q", got)
	}
}

func TestRedactErrorMessage_KeepsFirstLineOnly(t *testing.T) {
	got := notificationapp.RedactErrorMessage(errors.New("550 hard fail\nrecipient: alice@example.com"))
	if containsEmail(got) {
		t.Fatalf("multi-line email leaked: %q", got)
	}
}

func TestFormatAttemptError(t *testing.T) {
	if notificationapp.FormatAttemptError(notificationapp.ErrorClassTransient, errors.New("x")) != "transient_smtp" {
		t.Fatal("transient code mismatch")
	}
	if notificationapp.FormatAttemptError(notificationapp.ErrorClassPermanent, errors.New("x")) != "permanent_smtp" {
		t.Fatal("permanent code mismatch")
	}
	if notificationapp.FormatAttemptError(notificationapp.ErrorClassPermanentAuthOps, errors.New("x")) != "permanent_smtp_auth" {
		t.Fatal("permanent_auth code mismatch")
	}
	if notificationapp.FormatAttemptError(notificationapp.ErrorClassTransient, nil) != "" {
		t.Fatal("nil error must yield empty code")
	}
}

func containsEmail(s string) bool {
	// crude but enough for the test: "@" with characters on both sides.
	for i := 1; i < len(s)-1; i++ {
		if s[i] == '@' && s[i-1] != ' ' && s[i+1] != ' ' && s[i+1] != '<' && s[i+1] != '>' {
			return true
		}
	}
	return false
}
