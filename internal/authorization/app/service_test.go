package app_test

import (
	"context"
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	authinmem "github.com/cobo/cobo_iam_services/internal/authorization/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func TestAuthorize_missingCompanyContext(t *testing.T) {
	ctx := context.Background()
	svc := newAuthService(t)

	_, err := svc.Authorize(ctx, authapp.AuthorizeRequest{
		Subject:  authapp.SubjectRef{UserID: "u", MembershipID: "", CompanyID: "c_001"},
		Action:   "disclosure.view",
		Resource: authapp.ResourceRef{Type: "disclosure_record", ID: "r1", Attributes: map[string]any{"department_id": "d_legal"}},
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.Code != perr.CodeCompanyContextRequired {
		t.Fatalf("got %v", err)
	}
}

func TestAuthorize_allow_withDepartmentScope(t *testing.T) {
	ctx := context.Background()
	svc := newAuthService(t)

	dec, err := svc.Authorize(ctx, authapp.AuthorizeRequest{
		Subject:  authapp.SubjectRef{UserID: "u_123", MembershipID: "m_001", CompanyID: "c_001"},
		Action:   "disclosure.view",
		Resource: authapp.ResourceRef{Type: "disclosure_record", ID: "r_1001", Attributes: map[string]any{"department_id": "d_legal"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if dec.Decision != authapp.DecisionAllow {
		t.Fatalf("decision=%s reasons=%v", dec.Decision, dec.ScopeReasons)
	}
}

func TestAuthorize_deny_permissionMissing(t *testing.T) {
	ctx := context.Background()
	svc := newAuthService(t)

	dec, err := svc.Authorize(ctx, authapp.AuthorizeRequest{
		Subject:  authapp.SubjectRef{UserID: "u", MembershipID: "m_002", CompanyID: "c_002"},
		Action:   "disclosure.approve",
		Resource: authapp.ResourceRef{Type: "disclosure_record", ID: "r1", Attributes: map[string]any{"department_id": "d_legal"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if dec.Decision != authapp.DecisionDeny || dec.DenyReasonCode == nil || *dec.DenyReasonCode != perr.CodePermissionDenied {
		t.Fatalf("got %+v", dec)
	}
}

func TestAuthorizeBatch_twoChecks(t *testing.T) {
	ctx := context.Background()
	svc := newAuthService(t)

	out, err := svc.AuthorizeBatch(ctx, authapp.AuthorizeBatchRequest{
		Subject: authapp.SubjectRef{UserID: "u_123", MembershipID: "m_001", CompanyID: "c_001"},
		Checks: []authapp.AuthorizeBatchCheck{
			{Action: "disclosure.view", Resource: authapp.ResourceRef{Type: "disclosure_record", ID: "r_1001", Attributes: map[string]any{"department_id": "d_legal"}}},
			{Action: "disclosure.view", Resource: authapp.ResourceRef{Type: "disclosure_record", ID: "r_other", Attributes: map[string]any{"department_id": "d_unknown"}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Results) != 2 || out.Results[0].Decision != authapp.DecisionAllow || out.Results[1].Decision != authapp.DecisionDeny {
		t.Fatalf("results=%+v", out.Results)
	}
}

func newAuthService(t *testing.T) authapp.Service {
	t.Helper()
	repo := authinmem.NewRepository()
	resolver := authinmem.NewResolver(repo)
	checker := authinmem.NewChecker()
	return authapp.NewService(resolver, checker, repo)
}
