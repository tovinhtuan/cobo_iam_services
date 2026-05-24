package app

import (
	"context"
	"net/http"
	"testing"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

// fakeCMSRepo stubs only GetTypeVersionDetail so that ActivateTypeVersion can
// proceed past the permission check and fail at the repo level (not panic).
type fakeCMSRepo struct {
	Repository
}

func (f *fakeCMSRepo) GetTypeVersionDetail(_ context.Context, _, _ string, _ int) (*DisclosureTypeDTO, error) {
	return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "type version not found (stub)", nil)
}

func newCMSPermTestService(permissions []string) Service {
	return NewService(&fakeCMSRepo{}, &fakeAuthService{permissions: permissions}, idgen.UUIDv7Generator{})
}

// TestCMSTemplateActivate_CheckerOnly verifies that disclosure_type.manage (maker) alone
// cannot activate a CMS system template version — only disclosure_type.publish (checker) can.
// This locks the DOC-001 Q2 maker-checker boundary for CMS template activation.
func TestCMSTemplateActivate_CheckerOnly(t *testing.T) {
	t.Run("manage_only_is_forbidden", func(t *testing.T) {
		svc := newCMSPermTestService([]string{permissionPlatformCMSView, permissionLegacyTemplateManage})
		_, err := svc.ActivateTypeVersion(context.Background(), ActivateTypeVersionRequest{
			Subject:   subject(),
			TypeID:    "type-001",
			VersionNo: 1,
		})
		if err == nil {
			t.Fatal("expected 403, got nil error")
		}
		herr, ok := err.(*perr.HTTPError)
		if !ok || herr.HTTPStatus != http.StatusForbidden {
			t.Fatalf("expected 403 forbidden, got %v", err)
		}
	})

	t.Run("publish_passes_permission_check", func(t *testing.T) {
		svc := newCMSPermTestService([]string{permissionPlatformCMSView, permissionLegacyPublish})
		_, err := svc.ActivateTypeVersion(context.Background(), ActivateTypeVersionRequest{
			Subject:   subject(),
			TypeID:    "type-001",
			VersionNo: 1,
		})
		// Permission passes; stub repo returns 404. A 403 here would mean the fix is wrong.
		if err != nil {
			if herr, ok := err.(*perr.HTTPError); ok && herr.HTTPStatus == http.StatusForbidden {
				t.Fatalf("disclosure_type.publish should pass the activate permission check, got 403: %v", err)
			}
		}
	})

	t.Run("canonical_cms_template_activate_passes", func(t *testing.T) {
		svc := newCMSPermTestService([]string{permissionPlatformCMSView, permissionCMSTemplateActivate})
		_, err := svc.ActivateTypeVersion(context.Background(), ActivateTypeVersionRequest{
			Subject:   subject(),
			TypeID:    "type-001",
			VersionNo: 1,
		})
		if err != nil {
			if herr, ok := err.(*perr.HTTPError); ok && herr.HTTPStatus == http.StatusForbidden {
				t.Fatalf("cms.template.activate should pass the activate permission check, got 403: %v", err)
			}
		}
	})
}
