package app

import (
	"context"
	"net/http"
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

// fakeAuthService returns a fixed permission set for every call.
type fakeAuthService struct {
	permissions []string
}

func (f *fakeAuthService) GetEffectiveAccess(_ context.Context, _, _ string) (*authapp.EffectiveAccessSummary, error) {
	return &authapp.EffectiveAccessSummary{
		MembershipID: "member-001",
		CompanyID:    "company-001",
		Permissions:  f.permissions,
	}, nil
}

func (f *fakeAuthService) Authorize(_ context.Context, _ authapp.AuthorizeRequest) (*authapp.AuthorizeDecision, error) {
	return nil, nil
}

func (f *fakeAuthService) AuthorizeBatch(_ context.Context, _ authapp.AuthorizeBatchRequest) (*authapp.AuthorizeBatchResponse, error) {
	return nil, nil
}

// fakeLifecycleRepo only stubs the company-template lifecycle methods.
type fakeLifecycleRepo struct {
	Repository
	templateResp *CompanyTemplateWriteResponse
	templateErr  error
}

func (f *fakeLifecycleRepo) GetCompanyTemplateForLifecycle(_ context.Context, _, _ string) (*CompanyTemplateWriteResponse, error) {
	return f.templateResp, f.templateErr
}

func (f *fakeLifecycleRepo) TransitionCompanyTemplateReviewStatus(_ context.Context, _, _, _, _ string) error {
	return nil
}

func newLifecycleTestService(permissions []string, template *CompanyTemplateWriteResponse, templateErr error) Service {
	repo := &fakeLifecycleRepo{
		templateResp: template,
		templateErr:  templateErr,
	}
	return NewService(repo, &fakeAuthService{permissions: permissions}, idgen.UUIDv7Generator{})
}

func subject() Subject {
	return Subject{MembershipID: "member-001", CompanyID: "company-001"}
}

// TestCompanyTemplateLifecycleCapability_CheckerActions verifies publish/reject/archive
// require disclosure_type.publish (not disclosure_type.manage).
func TestCompanyTemplateLifecycleCapability_CheckerActions(t *testing.T) {
	checkerActions := []string{"publish", "reject", "archive"}
	for _, action := range checkerActions {
		t.Run(action+"_denied_with_manage_only", func(t *testing.T) {
			svc := newLifecycleTestService([]string{"disclosure_type.manage"}, nil, nil)
			_, err := svc.TransitionCompanyTemplateLifecycle(context.Background(), TransitionCompanyTemplateLifecycleRequest{
				Subject: subject(),
				TypeID:  "type-001",
				Action:  action,
			})
			if err == nil {
				t.Fatalf("action %q: expected 403, got nil error", action)
			}
			herr, ok := err.(*perr.HTTPError)
			if !ok {
				t.Fatalf("action %q: expected *perr.HTTPError, got %T", action, err)
			}
			if herr.HTTPStatus != http.StatusForbidden {
				t.Fatalf("action %q: status=%d want 403", action, herr.HTTPStatus)
			}
		})

		t.Run(action+"_allowed_with_publish_capability", func(t *testing.T) {
			// Repo returns a published template so the transition succeeds.
			tmpl := &CompanyTemplateWriteResponse{
				TypeID: "type-001", CompanyID: "company-001", ReviewStatus: "in_review",
			}
			svc := newLifecycleTestService([]string{"disclosure_type.publish"}, tmpl, nil)
			_, err := svc.TransitionCompanyTemplateLifecycle(context.Background(), TransitionCompanyTemplateLifecycleRequest{
				Subject: subject(),
				TypeID:  "type-001",
				Action:  action,
			})
			// A state-conflict 409 is acceptable here because the stub always returns in_review
			// regardless of the action; what matters is that the permission check passed (no 403).
			if err != nil {
				herr, ok := err.(*perr.HTTPError)
				if !ok || herr.HTTPStatus == http.StatusForbidden {
					t.Fatalf("action %q: got unexpected error %v", action, err)
				}
			}
		})
	}
}

// TestCompanyTemplateLifecycleCapability_MakerAction verifies submit-review passes
// the permission check with disclosure_type.manage (maker capability).
func TestCompanyTemplateLifecycleCapability_MakerAction(t *testing.T) {
	tmpl := &CompanyTemplateWriteResponse{
		TypeID: "type-002", CompanyID: "company-001", ReviewStatus: "draft",
	}
	svc := newLifecycleTestService([]string{"disclosure_type.manage"}, tmpl, nil)
	_, err := svc.TransitionCompanyTemplateLifecycle(context.Background(), TransitionCompanyTemplateLifecycleRequest{
		Subject: subject(),
		TypeID:  "type-002",
		Action:  "submit-review",
	})
	// Permission check must pass; any error at transition level is not a 403.
	if err != nil {
		herr, ok := err.(*perr.HTTPError)
		if ok && herr.HTTPStatus == http.StatusForbidden {
			t.Fatalf("submit-review with disclosure_type.manage should not be 403, got: %v", err)
		}
	}
}
