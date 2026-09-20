package app

import (
	"context"
	"net/http"
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func TestRequireDisclosureCatalogRead_allowsAdHocPropose(t *testing.T) {
	svc := &service{auth: &catalogAuthStub{perms: []string{"ad_hoc_alert.propose"}}}
	if err := svc.requireDisclosureCatalogRead(context.Background(), Subject{
		UserID: "u1", MembershipID: "m1", CompanyID: "c_001",
	}); err != nil {
		t.Fatalf("propose should read catalog: %v", err)
	}
}

func TestRequireDisclosureCatalogRead_deniesWithoutProposeOrCreate(t *testing.T) {
	svc := &service{auth: &catalogAuthStub{perms: []string{"deadline.view"}, denyAuthorize: true}}
	err := svc.requireDisclosureCatalogRead(context.Background(), Subject{
		UserID: "u1", MembershipID: "m1", CompanyID: "c_001",
	})
	if err == nil {
		t.Fatal("expected deny")
	}
	httpErr, ok := err.(*perr.HTTPError)
	if !ok || httpErr.HTTPStatus != http.StatusForbidden {
		t.Fatalf("want 403, got %v", err)
	}
}

type catalogAuthStub struct {
	perms        []string
	denyAuthorize bool
}

func (a *catalogAuthStub) GetEffectiveAccess(_ context.Context, membershipID, companyID string) (*authapp.EffectiveAccessSummary, error) {
	return &authapp.EffectiveAccessSummary{
		CompanyID:    companyID,
		MembershipID: membershipID,
		Permissions:  a.perms,
	}, nil
}

func (a *catalogAuthStub) Authorize(_ context.Context, _ authapp.AuthorizeRequest) (*authapp.AuthorizeDecision, error) {
	if a.denyAuthorize {
		return &authapp.AuthorizeDecision{Decision: authapp.DecisionDeny}, nil
	}
	return &authapp.AuthorizeDecision{Decision: authapp.DecisionAllow}, nil
}

func (a *catalogAuthStub) AuthorizeBatch(_ context.Context, _ authapp.AuthorizeBatchRequest) (*authapp.AuthorizeBatchResponse, error) {
	return &authapp.AuthorizeBatchResponse{}, nil
}
