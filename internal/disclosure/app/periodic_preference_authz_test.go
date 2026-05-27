package app

import (
	"context"
	"net/http"
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

type preferenceAuthzRepo struct {
	Repository
	pref *CompanyTypePreference
}

func (r *preferenceAuthzRepo) GetCompanyTypePreference(_ context.Context, companyID, typeID string) (*CompanyTypePreference, error) {
	if r.pref != nil {
		return r.pref, nil
	}
	return nil, nil
}

func (r *preferenceAuthzRepo) UpsertCompanyTypePreference(_ context.Context, in CompanyTypePreference) error {
	r.pref = &in
	return nil
}

type permissionAuthService struct {
	permissions map[string]struct{}
	lastAction  string
}

func (p *permissionAuthService) GetEffectiveAccess(_ context.Context, _, _ string) (*authapp.EffectiveAccessSummary, error) {
	return &authapp.EffectiveAccessSummary{}, nil
}

func (p *permissionAuthService) Authorize(_ context.Context, req authapp.AuthorizeRequest) (*authapp.AuthorizeDecision, error) {
	p.lastAction = req.Action
	if _, ok := p.permissions[req.Action]; ok {
		return &authapp.AuthorizeDecision{Decision: authapp.DecisionAllow}, nil
	}
	return &authapp.AuthorizeDecision{Decision: authapp.DecisionDeny}, nil
}

func (p *permissionAuthService) AuthorizeBatch(_ context.Context, _ authapp.AuthorizeBatchRequest) (*authapp.AuthorizeBatchResponse, error) {
	return &authapp.AuthorizeBatchResponse{}, nil
}

func newPreferenceAuthzService(perms ...string) Service {
	set := make(map[string]struct{}, len(perms))
	for _, p := range perms {
		set[p] = struct{}{}
	}
	return NewService(&preferenceAuthzRepo{}, &permissionAuthService{permissions: set}, idgen.UUIDv7Generator{})
}

func TestGetCompanyTypePreference_RequiresDisclosureView(t *testing.T) {
	sub := Subject{UserID: "u1", MembershipID: "m1", CompanyID: "co-1"}
	_, err := newPreferenceAuthzService().GetCompanyTypePreference(context.Background(), GetCompanyTypePreferenceRequest{
		Subject: sub,
		TypeID:  "type-1",
	})
	if err == nil {
		t.Fatal("expected forbidden")
	}
	herr, ok := err.(*perr.HTTPError)
	if !ok || herr.HTTPStatus != http.StatusForbidden {
		t.Fatalf("err=%v", err)
	}

	svc := newPreferenceAuthzService("disclosure.view")
	dto, err := svc.GetCompanyTypePreference(context.Background(), GetCompanyTypePreferenceRequest{
		Subject: sub,
		TypeID:  "type-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !dto.AutoCreateEnabled {
		t.Fatal("expected default enabled")
	}
}

func TestUpsertCompanyTypePreference_RequiresAutoCreateManage(t *testing.T) {
	sub := Subject{UserID: "u1", MembershipID: "m1", CompanyID: "co-1"}
	_, err := newPreferenceAuthzService("disclosure.view").UpsertCompanyTypePreference(context.Background(), UpsertCompanyTypePreferenceRequest{
		Subject:           sub,
		TypeID:            "type-1",
		AutoCreateEnabled: false,
	})
	if err == nil {
		t.Fatal("expected forbidden")
	}
	herr, ok := err.(*perr.HTTPError)
	if !ok || herr.HTTPStatus != http.StatusForbidden {
		t.Fatalf("err=%v", err)
	}

	svc := newPreferenceAuthzService("disclosure.auto_create.manage")
	dto, err := svc.UpsertCompanyTypePreference(context.Background(), UpsertCompanyTypePreferenceRequest{
		Subject:           sub,
		TypeID:            "type-1",
		AutoCreateEnabled: false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.AutoCreateEnabled {
		t.Fatal("expected disabled")
	}
}

func TestUpsertCompanyTypePreference_ManageOnlyDoesNotRequireView(t *testing.T) {
	sub := Subject{UserID: "u1", MembershipID: "m1", CompanyID: "co-1"}
	auth := &permissionAuthService{permissions: map[string]struct{}{
		"disclosure.auto_create.manage": {},
	}}
	svc := NewService(&preferenceAuthzRepo{}, auth, idgen.UUIDv7Generator{})

	dto, err := svc.UpsertCompanyTypePreference(context.Background(), UpsertCompanyTypePreferenceRequest{
		Subject:           sub,
		TypeID:            "type-1",
		AutoCreateEnabled: false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.AutoCreateEnabled {
		t.Fatal("expected disabled")
	}
	if dto.CompanyID != "co-1" || dto.TypeID != "type-1" {
		t.Fatalf("dto=%+v", dto)
	}
	if auth.lastAction != "disclosure.auto_create.manage" {
		t.Fatalf("authorize action=%q want disclosure.auto_create.manage only", auth.lastAction)
	}
}
