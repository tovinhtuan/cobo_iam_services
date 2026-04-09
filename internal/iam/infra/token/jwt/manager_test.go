package jwt_test

import (
	"context"
	"testing"
	"time"

	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	iamtokenjwt "github.com/cobo/cobo_iam_services/internal/iam/infra/token/jwt"
	iamtokenopaque "github.com/cobo/cobo_iam_services/internal/iam/infra/token/opaque"
	"github.com/cobo/cobo_iam_services/internal/platform/config"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

type seqID struct{ n int }

func (s *seqID) NewUUID() string {
	s.n++
	return "id-" + time.Now().UTC().Format("150405") + "-" + string(rune('a'+s.n))
}

func TestJWTManager_IssueAndInspectAccessToken_HS256(t *testing.T) {
	id := &seqID{}
	opaque := iamtokenopaque.NewManager(id)
	cfg := config.Config{
		JWTAlg:               "HS256",
		JWTSigningPrivateKey: "test-secret",
		JWTIssuer:            "issuer-a",
		JWTAudience:          "aud-a",
		AccessTokenTTL:       2 * time.Minute,
		JWTClockSkewSec:      1,
	}
	m := iamtokenjwt.NewManager(cfg, id, opaque)
	tok, _, err := m.IssueAccessToken(context.Background(), iamapp.AccessTokenClaims{
		Sub: "u1", SessionID: "s1", MembershipID: "m1", CompanyID: "c1",
	})
	if err != nil {
		t.Fatal(err)
	}
	claims, err := m.InspectAccessToken(context.Background(), tok)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Sub != "u1" || claims.SessionID != "s1" || claims.MembershipID != "m1" || claims.CompanyID != "c1" {
		t.Fatalf("claims mismatch: %+v", claims)
	}
}

func TestJWTManager_ExpiredToken(t *testing.T) {
	id := &seqID{}
	opaque := iamtokenopaque.NewManager(id)
	cfg := config.Config{
		JWTAlg:               "HS256",
		JWTSigningPrivateKey: "test-secret",
		JWTIssuer:            "issuer-a",
		JWTAudience:          "aud-a",
		AccessTokenTTL:       time.Second,
		JWTClockSkewSec:      0,
	}
	m := iamtokenjwt.NewManager(cfg, id, opaque)
	tok, _, err := m.IssueAccessToken(context.Background(), iamapp.AccessTokenClaims{
		Sub: "u1", SessionID: "s1", MembershipID: "m1", CompanyID: "c1",
	})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Second)
	_, err = m.InspectAccessToken(context.Background(), tok)
	if err == nil {
		t.Fatal("expected expired token error")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.Code != perr.CodeSessionExpired {
		t.Fatalf("unexpected error: %#v", err)
	}
}

func TestJWTManager_InvalidAudience(t *testing.T) {
	id := &seqID{}
	opaque := iamtokenopaque.NewManager(id)
	issueCfg := config.Config{
		JWTAlg:               "HS256",
		JWTSigningPrivateKey: "test-secret",
		JWTIssuer:            "issuer-a",
		JWTAudience:          "aud-a",
		AccessTokenTTL:       time.Minute,
		JWTClockSkewSec:      1,
	}
	verifyCfg := issueCfg
	verifyCfg.JWTAudience = "aud-b"
	issuer := iamtokenjwt.NewManager(issueCfg, id, opaque)
	verifier := iamtokenjwt.NewManager(verifyCfg, id, opaque)
	tok, _, err := issuer.IssueAccessToken(context.Background(), iamapp.AccessTokenClaims{
		Sub: "u1", SessionID: "s1", MembershipID: "m1", CompanyID: "c1",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = verifier.InspectAccessToken(context.Background(), tok)
	if err == nil {
		t.Fatal("expected invalid audience error")
	}
}
