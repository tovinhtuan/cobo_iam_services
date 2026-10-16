package app_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	"github.com/cobo/cobo_iam_services/internal/platform/events"
)

// mailboxRecovery returns fixed recovery users and records the reset tokens stored for them.
type mailboxRecovery struct {
	stubRecoveryForBaseline
	users     []iamapp.RecoveryUser
	err       error
	gotLimit  int
	tokensFor []string
}

func (r *mailboxRecovery) FindRecoveryUsers(_ context.Context, _ string, limit int) ([]iamapp.RecoveryUser, error) {
	r.gotLimit = limit
	return r.users, r.err
}

func (r *mailboxRecovery) StorePasswordResetToken(_ context.Context, tok iamapp.RecoveryTokenRecord) error {
	r.tokensFor = append(r.tokensFor, tok.UserID)
	return nil
}

type allEventsPublisher struct{ events []events.Event }

func (p *allEventsPublisher) Publish(_ context.Context, e events.Event) error {
	p.events = append(p.events, e)
	return nil
}

func forgotPassword(t *testing.T, rec *mailboxRecovery) (*allEventsPublisher, *iamapp.ForgotPasswordResponse) {
	t.Helper()
	publisher := &allEventsPublisher{}
	svc := newTestIAMService(t, testIAMDeps{
		cred:    testCred(),
		members: cainmem.NewMembershipQueryService(),
		opts: []iamapp.ServiceOption{
			iamapp.WithAuthRecoveryRepository(rec),
			iamapp.WithOutboxPublisher(publisher),
			iamapp.WithAuthFlowConfig(iamapp.AuthFlowConfig{WebBaseURL: "https://app.example.com"}),
		},
	})
	resp, err := svc.ForgotPassword(context.Background(), iamapp.ForgotPasswordRequest{Email: "Shared@Example.com"})
	if err != nil {
		t.Fatalf("ForgotPassword error = %v", err)
	}
	if resp == nil || !resp.Success {
		t.Fatalf("ForgotPassword resp = %+v, want the generic success", resp)
	}
	return publisher, resp
}

// bug-login-old-accounts-2026-10-16 (BUG-2): every account whose mailbox is the typed address gets
// its own link at that address, instead of one arbitrary account.
func TestForgotPassword_MailsEveryAccountOfTheAddress(t *testing.T) {
	rec := &mailboxRecovery{users: []iamapp.RecoveryUser{
		{UserID: "u_personal", Email: "shared@example.com", LoginID: "shared@example.com", FullName: "Personal"},
		{UserID: "u_seed", Email: "Shared@example.com", LoginID: "admin_dn", FullName: "Seed admin"},
	}}
	publisher, _ := forgotPassword(t, rec)

	if rec.gotLimit != 6 {
		t.Fatalf("lookup limit = %d, want 6 (5 accounts + 1 to detect more)", rec.gotLimit)
	}
	if !reflect.DeepEqual(rec.tokensFor, []string{"u_personal", "u_seed"}) {
		t.Fatalf("tokens for %v, want both accounts", rec.tokensFor)
	}
	var got []string
	for _, e := range publisher.events {
		got = append(got, fmt.Sprintf("%s:%s->%v", e.EventType, e.AggregateID, e.Payload["to"]))
	}
	want := []string{
		"auth.password_reset_requested:u_personal->shared@example.com",
		"auth.password_reset_requested:u_seed->Shared@example.com",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mails\n got: %v\nwant: %v", got, want)
	}
}

func TestForgotPassword_CapsTheAccountsOfOneAddress(t *testing.T) {
	rec := &mailboxRecovery{}
	for i := 0; i < 6; i++ {
		rec.users = append(rec.users, iamapp.RecoveryUser{UserID: fmt.Sprintf("u_%d", i), Email: "shared@example.com"})
	}
	publisher, _ := forgotPassword(t, rec)
	if len(rec.tokensFor) != 5 || len(publisher.events) != 5 || rec.tokensFor[4] != "u_4" {
		t.Fatalf("tokens %v, mails %d; want the 5 oldest accounts", rec.tokensFor, len(publisher.events))
	}
}

func TestForgotPassword_SendsNothingWhenNoAccountOrLookupFails(t *testing.T) {
	for name, rec := range map[string]*mailboxRecovery{
		"no account":    {},
		"lookup failed": {err: errors.New("db down")},
	} {
		publisher, _ := forgotPassword(t, rec)
		if len(rec.tokensFor) != 0 || len(publisher.events) != 0 {
			t.Fatalf("%s: tokens %v, mails %d; want none", name, rec.tokensFor, len(publisher.events))
		}
	}
}
