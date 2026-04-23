package http

import (
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
)

func TestApplyAccessTokenToSubject(t *testing.T) {
	t.Run("sets from claims", func(t *testing.T) {
		var s authapp.SubjectRef
		err := applyAccessTokenToSubject(&iamapp.AccessTokenClaims{
			Sub: "u1", SessionID: "s1", MembershipID: "m1", CompanyID: "c1",
		}, &s)
		if err != nil {
			t.Fatal(err)
		}
		if s.UserID != "u1" || s.MembershipID != "m1" || s.CompanyID != "c1" {
			t.Fatalf("got %#v", s)
		}
	})
	t.Run("rejects without company", func(t *testing.T) {
		var s authapp.SubjectRef
		err := applyAccessTokenToSubject(&iamapp.AccessTokenClaims{Sub: "u1", MembershipID: "m0"}, &s)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}
