package sessionbound_test

import (
	"context"
	"testing"

	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	iaminmem "github.com/cobo/cobo_iam_services/internal/iam/infra/inmemory"
	iamtokenopaque "github.com/cobo/cobo_iam_services/internal/iam/infra/token/opaque"
	iamtokensessionbound "github.com/cobo/cobo_iam_services/internal/iam/infra/token/sessionbound"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

type companyStatuses map[string]string

func (c companyStatuses) CompanyStatus(_ context.Context, companyID string) (string, error) {
	return c[companyID], nil
}

// A deactivated company ("Ngừng hoạt động") blocks every request of a session bound to it, at
// once, and reactivating it restores access without signing in again.
func TestInspectAccessToken_CompanyInactiveBlocksAtOnce(t *testing.T) {
	id := idgen.UUIDv7Generator{}
	sessions := iaminmem.NewSessionRepository()
	statuses := companyStatuses{"c_1": "active"}
	mgr := iamtokensessionbound.New(iamtokenopaque.NewManager(id), sessions, iamtokensessionbound.WithCompanyStatus(statuses))
	ctx := context.Background()
	sid := id.NewUUID()
	if err := sessions.Create(ctx, iamapp.CreateSessionParams{SessionID: sid, UserID: "u_1", RefreshToken: "rtk", MembershipID: "m_1", CompanyID: "c_1"}); err != nil {
		t.Fatal(err)
	}
	tok, _, err := mgr.IssueAccessToken(ctx, iamapp.AccessTokenClaims{Sub: "u_1", SessionID: sid, MembershipID: "m_1", CompanyID: "c_1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.InspectAccessToken(ctx, tok); err != nil {
		t.Fatalf("active company: %v", err)
	}

	statuses["c_1"] = "inactive"
	_, err = mgr.InspectAccessToken(ctx, tok)
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != 403 || he.Code != perr.CodeCompanyInactive {
		t.Fatalf("inactive company: got %v, want 403 COMPANY_INACTIVE", err)
	}

	statuses["c_1"] = "active"
	if _, err := mgr.InspectAccessToken(ctx, tok); err != nil {
		t.Fatalf("reactivated company: %v", err)
	}
}

func TestInspectAccessToken_CompanyStatusOnlyForCompanyContext(t *testing.T) {
	id := idgen.UUIDv7Generator{}
	sessions := iaminmem.NewSessionRepository()
	mgr := iamtokensessionbound.New(iamtokenopaque.NewManager(id), sessions,
		iamtokensessionbound.WithCompanyStatus(companyStatuses{"c_gone": "", "c_odd": "Legacy-Value"}))
	ctx := context.Background()
	issue := func(company string) string {
		t.Helper()
		sid := id.NewUUID()
		if err := sessions.Create(ctx, iamapp.CreateSessionParams{SessionID: sid, UserID: "u_1", RefreshToken: "rtk_" + sid, CompanyID: company}); err != nil {
			t.Fatal(err)
		}
		tok, _, err := mgr.IssueAccessToken(ctx, iamapp.AccessTokenClaims{Sub: "u_1", SessionID: sid, CompanyID: company})
		if err != nil {
			t.Fatal(err)
		}
		return tok
	}
	if _, err := mgr.InspectAccessToken(ctx, issue("")); err != nil {
		t.Fatalf("token without company context: %v", err)
	}
	if _, err := mgr.InspectAccessToken(ctx, issue("c_odd")); err != nil {
		t.Fatalf("legacy status value must stay readable (only inactive blocks): %v", err)
	}
	_, err := mgr.InspectAccessToken(ctx, issue("c_gone"))
	if he, ok := perr.AsHTTPError(err); !ok || he.Code != perr.CodeCompanyInactive {
		t.Fatalf("company that no longer exists: got %v, want COMPANY_INACTIVE", err)
	}
}
