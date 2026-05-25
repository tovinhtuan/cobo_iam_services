package app_test

import (
	"encoding/json"
	"strings"
	"testing"

	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
)

func TestLoginResponse_emailVerifiedFalseIsSerialized(t *testing.T) {
	resp := iamapp.LoginResponse{
		User:        iamapp.LoginUser{UserID: "u1", FullName: "U"},
		Session:     iamapp.LoginSession{RefreshToken: "r"},
		NextAction:  "no_company_onboarding",
		EmailVerified: false,
	}
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"email_verified":false`) {
		t.Fatalf("expected email_verified:false in JSON, got %s", string(b))
	}
}
