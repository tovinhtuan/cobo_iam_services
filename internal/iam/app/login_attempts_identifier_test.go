package app_test

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
)

type attemptLog struct{ recs []iamapp.LoginAttemptRecord }

func (a *attemptLog) Record(_ context.Context, rec iamapp.LoginAttemptRecord) error {
	a.recs = append(a.recs, rec)
	return nil
}

// bug-login-old-accounts-2026-10-16 (BUG-1): the web form sends `email`, not `login_id`, and
// recordLoginAttempt only read LoginID, so no failed sign-in was recorded after 2026-09-20.
func TestLogin_RecordsFailedAttemptsSentWithTheEmailAlias(t *testing.T) {
	ctx := context.Background()
	attempts := &attemptLog{}
	svc := newTestIAMService(t, testIAMDeps{
		cred:    testCred(),
		members: cainmem.NewMembershipQueryService(),
		opts:    []iamapp.ServiceOption{iamapp.WithLoginAttemptRecorder(attempts)},
	})

	if _, err := svc.Login(ctx, iamapp.LoginRequest{Email: " User@Example.com ", Password: "wrong"}); err == nil {
		t.Fatal("wrong password: want error")
	}
	if _, err := svc.Login(ctx, iamapp.LoginRequest{Email: "nobody@example.com", Password: "secret"}); err == nil {
		t.Fatal("unknown account: want error")
	}

	want := []iamapp.LoginAttemptRecord{
		{LoginID: "user@example.com", FailureCode: "INVALID_CREDENTIALS"},
		{LoginID: "nobody@example.com", FailureCode: "INVALID_CREDENTIALS"},
	}
	if !reflect.DeepEqual(attempts.recs, want) {
		t.Fatalf("recorded attempts\n got: %+v\nwant: %+v", attempts.recs, want)
	}
}

func TestLoginRequest_IdentifierPrefersLoginID(t *testing.T) {
	cases := []struct {
		req  iamapp.LoginRequest
		want string
	}{
		{iamapp.LoginRequest{LoginID: " admin_dn ", Email: "x@example.com"}, "admin_dn"},
		{iamapp.LoginRequest{Email: " x@example.com "}, "x@example.com"},
		{iamapp.LoginRequest{LoginID: "  ", Email: ""}, ""},
	}
	for _, c := range cases {
		if got := c.req.Identifier(); got != c.want {
			t.Errorf("Identifier(%+v) = %q, want %q", c.req, got, c.want)
		}
	}
}

// Review BES-03: failed attempts store what was typed (also in audit metadata), so cap it.
func TestLoginRequest_RecordedIdentifierIsLowercasedAndBounded(t *testing.T) {
	if got := (iamapp.LoginRequest{Email: " Chau@CongTy.VN "}).RecordedIdentifier(); got != "chau@congty.vn" {
		t.Fatalf("RecordedIdentifier = %q", got)
	}
	long := iamapp.LoginRequest{Email: strings.Repeat("Ệ", 5000)}.RecordedIdentifier()
	if long != strings.Repeat("ệ", 191) {
		t.Fatalf("RecordedIdentifier kept %d runes, want 191 lowercased", utf8.RuneCountInString(long))
	}
}
