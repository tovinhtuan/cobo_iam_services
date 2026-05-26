package sessionbound_test

import (
	"context"
	"testing"

	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	iaminmem "github.com/cobo/cobo_iam_services/internal/iam/infra/inmemory"
	iamtokenopaque "github.com/cobo/cobo_iam_services/internal/iam/infra/token/opaque"
	iamtokensessionbound "github.com/cobo/cobo_iam_services/internal/iam/infra/token/sessionbound"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func TestInspectAccessToken_rejectsRevokedSession(t *testing.T) {
	id := idgen.UUIDv7Generator{}
	opaque := iamtokenopaque.NewManager(id)
	sessions := iaminmem.NewSessionRepository()
	mgr := iamtokensessionbound.New(opaque, sessions)

	ctx := context.Background()
	sid := id.NewUUID()
	refresh := "rtk_test"
	if err := sessions.Create(ctx, iamapp.CreateSessionParams{
		SessionID: sid, UserID: "u_1", RefreshToken: refresh,
		MembershipID: "m_1", CompanyID: "c_1",
	}); err != nil {
		t.Fatal(err)
	}
	tok, _, err := mgr.IssueAccessToken(ctx, iamapp.AccessTokenClaims{
		Sub: "u_1", SessionID: sid, MembershipID: "m_1", CompanyID: "c_1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.InspectAccessToken(ctx, tok); err != nil {
		t.Fatalf("expected active session: %v", err)
	}
	if err := sessions.RevokeAllByUser(ctx, "u_1", "test"); err != nil {
		t.Fatal(err)
	}
	_, err = mgr.InspectAccessToken(ctx, tok)
	if err == nil {
		t.Fatal("expected error for revoked session")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.Code != perr.CodeSessionExpired {
		t.Fatalf("want SESSION_EXPIRED, got %v", err)
	}
}
