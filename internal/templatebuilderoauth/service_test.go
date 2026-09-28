package templatebuilderoauth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestAuthorizationCodePKCEIsOneTimeAndScopeBound(t *testing.T) {
	svc, err := NewService(NewMemoryStore(), ClientConfig{
		ClientID: "chatgpt-template-builder", RedirectURIs: []string{"https://chatgpt.com/aip/g-123/oauth/callback"},
		SigningKey: []byte("01234567890123456789012345678901"),
	})
	if err != nil {
		t.Fatal(err)
	}
	svc.now = func() time.Time { return time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC) }
	verifier := strings.Repeat("a", 43)
	hash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(hash[:])
	redirect, err := svc.Approve(context.Background(), ApproveRequest{
		ClientID: "chatgpt-template-builder", RedirectURI: "https://chatgpt.com/aip/g-123/oauth/callback", Scope: ScopeTemplateImportValidate,
		State: "opaque-state", CodeChallenge: challenge, CodeChallengeMethod: "S256",
		Subject: Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(strings.Split(redirect, "code=")[1], "&")
	code := parts[0]
	token, err := svc.Exchange(context.Background(), ExchangeRequest{ClientID: "chatgpt-template-builder", RedirectURI: "https://chatgpt.com/aip/g-123/oauth/callback", Code: code, CodeVerifier: verifier})
	if err != nil {
		t.Fatal(err)
	}
	if token.Scope != ScopeTemplateImportValidate || token.AccessToken == "" {
		t.Fatalf("unexpected token response: %#v", token)
	}
	if _, err := svc.Exchange(context.Background(), ExchangeRequest{ClientID: "chatgpt-template-builder", RedirectURI: "https://chatgpt.com/aip/g-123/oauth/callback", Code: code, CodeVerifier: verifier}); err == nil {
		t.Fatal("authorization code replay was accepted")
	}
	subject, err := svc.ValidateAccessToken(token.AccessToken)
	if err != nil || subject.UserID != "u1" || subject.CompanyID != "c1" {
		t.Fatalf("token subject: %#v, %v", subject, err)
	}
}
