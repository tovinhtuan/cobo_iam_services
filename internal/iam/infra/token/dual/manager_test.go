package dual_test

import (
	"context"
	"testing"
	"time"

	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	iamtokendual "github.com/cobo/cobo_iam_services/internal/iam/infra/token/dual"
	iamtokenjwt "github.com/cobo/cobo_iam_services/internal/iam/infra/token/jwt"
	iamtokenopaque "github.com/cobo/cobo_iam_services/internal/iam/infra/token/opaque"
	"github.com/cobo/cobo_iam_services/internal/platform/config"
)

type seqID struct{ n int }

func (s *seqID) NewUUID() string {
	s.n++
	return "dual-id-" + time.Now().UTC().Format("150405") + "-" + string(rune('a'+s.n))
}

func TestDualManager_FallbackOpaqueInspect(t *testing.T) {
	id := &seqID{}
	opaque := iamtokenopaque.NewManager(id)
	cfg := config.Config{
		JWTAlg:               "HS256",
		JWTSigningPrivateKey: "dual-secret",
		JWTIssuer:            "issuer-a",
		JWTAudience:          "aud-a",
		AccessTokenTTL:       time.Minute,
	}
	j := iamtokenjwt.NewManager(cfg, id, opaque)
	m := iamtokendual.NewManager(j, opaque, j)

	opaqueTok, _, err := opaque.IssueAccessToken(context.Background(), iamapp.AccessTokenClaims{
		Sub: "u-opaque", SessionID: "s-opaque", MembershipID: "m-opaque", CompanyID: "c-opaque",
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := m.InspectAccessToken(context.Background(), opaqueTok)
	if err != nil {
		t.Fatal(err)
	}
	if got.Sub != "u-opaque" {
		t.Fatalf("unexpected subject: %+v", got)
	}
}
