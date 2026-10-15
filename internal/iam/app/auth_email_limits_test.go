package app_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/events"
)

// AUTH-03 (risk-review-auth-flows-2026-10-15): unauthenticated endpoints must not send
// unlimited auth emails to one recipient.

// limitsRecovery reports how many OTPs / reset tokens were issued for the user in the last
// hour and the last day, and records new ones.
type limitsRecovery struct {
	stubRecoveryForBaseline
	otpLastHour, otpLastDay     int
	resetLastHour, resetLastDay int
	otpStored, resetStored      int
}

func countForWindow(since time.Time, lastHour, lastDay int) int {
	if time.Since(since) > 2*time.Hour {
		return lastDay
	}
	return lastHour
}

func (r *limitsRecovery) CountEmailVerificationOTPsSince(_ context.Context, _ string, since time.Time) (int, error) {
	return countForWindow(since, r.otpLastHour, r.otpLastDay), nil
}

func (r *limitsRecovery) StoreEmailVerificationOTP(context.Context, iamapp.EmailOTPRecord) error {
	r.otpStored++
	return nil
}

func withinWindows(windows []iamapp.SendWindow, lastHour, lastDay int) bool {
	for _, w := range windows {
		if countForWindow(w.Since, lastHour, lastDay) >= w.Max {
			return false
		}
	}
	return true
}

// The fake implements LimitedRecoveryIssuer, like the MySQL repository, so the tests run the production path.
func (r *limitsRecovery) IssueEmailVerificationOTPWithinLimits(_ context.Context, _ iamapp.EmailOTPRecord, windows []iamapp.SendWindow) (bool, error) {
	if !withinWindows(windows, r.otpLastHour, r.otpLastDay) {
		return false, nil
	}
	r.otpStored++
	return true, nil
}

func (r *limitsRecovery) StorePasswordResetTokenWithinLimits(_ context.Context, _ iamapp.RecoveryTokenRecord, windows []iamapp.SendWindow) (bool, error) {
	if !withinWindows(windows, r.resetLastHour, r.resetLastDay) {
		return false, nil
	}
	r.resetStored++
	return true, nil
}

func (r *limitsRecovery) StorePasswordResetToken(context.Context, iamapp.RecoveryTokenRecord) error {
	r.resetStored++
	return nil
}

type countingOutboxPublisher struct{ n int }

func (c *countingOutboxPublisher) Publish(context.Context, events.Event) error {
	c.n++
	return nil
}

func newLimitsService(t *testing.T, rec *limitsRecovery, pub *countingOutboxPublisher) iamapp.Service {
	t.Helper()
	rec.user = &iamapp.RecoveryUser{UserID: "u_limits", Email: "limits@example.com", FullName: "Limits", LoginID: "limits@example.com"}
	return newTestIAMService(t, testIAMDeps{
		cred:    testCred(),
		members: cainmem.NewMembershipQueryService(),
		opts: []iamapp.ServiceOption{
			iamapp.WithOutboxPublisher(pub),
			iamapp.WithAuthRecoveryRepository(rec),
			iamapp.WithAuthFlowConfig(iamapp.AuthFlowConfig{
				WebBaseURL:              "https://app.example.com",
				PasswordResetTokenTTL:   30 * time.Minute,
				EmailVerificationOTPTTL: 15 * time.Minute,
			}),
		},
	})
}

func TestAuthEmailLimits_ForgotPassword(t *testing.T) {
	cases := []struct {
		name              string
		lastHour, lastDay int
		wantSent          bool
	}{
		{name: "AEL-01 below limits sends", lastHour: 2, lastDay: 9, wantSent: true},
		{name: "AEL-02 hourly limit reached", lastHour: 3, lastDay: 3, wantSent: false},
		{name: "AEL-03 daily limit reached", lastHour: 0, lastDay: 10, wantSent: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &limitsRecovery{resetLastHour: tc.lastHour, resetLastDay: tc.lastDay}
			pub := &countingOutboxPublisher{}
			svc := newLimitsService(t, rec, pub)

			resp, err := svc.ForgotPassword(context.Background(), iamapp.ForgotPasswordRequest{Email: "limits@example.com"})
			if err != nil {
				t.Fatalf("ForgotPassword error = %v (limited requests must keep the generic success)", err)
			}
			if resp == nil || !resp.Success {
				t.Fatalf("ForgotPassword resp = %+v, want generic success", resp)
			}
			sent := rec.resetStored == 1 && pub.n == 1
			if sent != tc.wantSent {
				t.Fatalf("token stored=%d emails published=%d, want sent=%v", rec.resetStored, pub.n, tc.wantSent)
			}
		})
	}
}

func TestAuthEmailLimits_AdminPasswordResetIsNotLimited(t *testing.T) {
	rec := &limitsRecovery{resetLastHour: 50, resetLastDay: 50}
	pub := &countingOutboxPublisher{}
	svc := newLimitsService(t, rec, pub)

	if err := svc.AdminRequestPasswordReset(context.Background(), "u_limits"); err != nil {
		t.Fatalf("AdminRequestPasswordReset error = %v", err)
	}
	if rec.resetStored != 1 || pub.n != 1 {
		t.Fatalf("admin reset stored=%d published=%d, want 1/1", rec.resetStored, pub.n)
	}
}

func TestAuthEmailLimits_ResendVerificationOTP(t *testing.T) {
	cases := []struct {
		name              string
		lastHour, lastDay int
		wantSent          bool
	}{
		{name: "AEL-04 below limits sends", lastHour: 4, lastDay: 9, wantSent: true},
		{name: "AEL-05 hourly limit reached", lastHour: 5, lastDay: 5, wantSent: false},
		{name: "AEL-06 daily limit reached", lastHour: 1, lastDay: 10, wantSent: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &limitsRecovery{otpLastHour: tc.lastHour, otpLastDay: tc.lastDay}
			pub := &countingOutboxPublisher{}
			svc := newLimitsService(t, rec, pub)

			_, err := svc.ResendVerificationEmail(context.Background(), iamapp.ResendVerificationEmailRequest{UserID: "u_limits"})
			if tc.wantSent {
				if err != nil {
					t.Fatalf("ResendVerificationEmail error = %v", err)
				}
				if rec.otpStored != 1 || pub.n != 1 {
					t.Fatalf("otp stored=%d published=%d, want 1/1", rec.otpStored, pub.n)
				}
				return
			}
			var he *perr.HTTPError
			if !errors.As(err, &he) || he.HTTPStatus != http.StatusTooManyRequests {
				t.Fatalf("ResendVerificationEmail error = %v, want 429", err)
			}
			if rec.otpStored != 0 || pub.n != 0 {
				t.Fatalf("otp stored=%d published=%d, want nothing sent", rec.otpStored, pub.n)
			}
		})
	}
}

// An email-only (signed-out) resend cannot be used to verify, so it must not send mail or reveal
// whether the account exists (429 vs 200).
func TestAuthEmailLimits_AnonymousResendSendsNothing(t *testing.T) {
	for _, tc := range []struct {
		name     string
		lastHour int
	}{
		{name: "AEL-07 below limits", lastHour: 0},
		{name: "AEL-08 limit reached", lastHour: 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := &limitsRecovery{otpLastHour: tc.lastHour, otpLastDay: tc.lastHour}
			pub := &countingOutboxPublisher{}
			svc := newLimitsService(t, rec, pub)

			resp, err := svc.ResendVerificationEmail(context.Background(), iamapp.ResendVerificationEmailRequest{Email: "limits@example.com"})
			if err != nil || resp == nil || !resp.Success {
				t.Fatalf("anonymous resend = %+v, %v; want generic success", resp, err)
			}
			if rec.otpStored != 0 || pub.n != 0 {
				t.Fatalf("otp stored=%d published=%d, want nothing sent", rec.otpStored, pub.n)
			}
		})
	}
}
