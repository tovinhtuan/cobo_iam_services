package app

import (
	"context"
	"net/http"
	"testing"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

// archiveRepo tracks ArchiveGlobalTemplate calls and can inject errors.
type archiveRepo struct {
	Repository
	archivedTypeID string
	archiveErr     error
}

func (r *archiveRepo) ArchiveGlobalTemplate(_ context.Context, typeID, _ string) error {
	if r.archiveErr != nil {
		return r.archiveErr
	}
	r.archivedTypeID = typeID
	return nil
}

func newCMSArchiveService(perms []string, repo Repository) Service {
	return NewService(repo, &fakeAuthService{permissions: perms}, idgen.UUIDv7Generator{})
}

// TestCmsArchiveTemplate_Success verifies the happy path: archive returns {TypeID, status=archived}.
func TestCmsArchiveTemplate_Success(t *testing.T) {
	repo := &archiveRepo{}
	svc := newCMSArchiveService(
		[]string{permissionPlatformCMSView, permissionCMSTemplateArchive},
		repo,
	)
	resp, err := svc.CmsArchiveTemplate(context.Background(), CmsArchiveTemplateRequest{
		Subject: Subject{UserID: "user-001", MembershipID: "m-001", CompanyID: "c-001"},
		TypeID:  "dt-global-001",
		Reason:  "QA cleanup",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.TypeID != "dt-global-001" {
		t.Errorf("type_id=%s want dt-global-001", resp.TypeID)
	}
	if resp.Status != "archived" {
		t.Errorf("status=%s want archived", resp.Status)
	}
	if repo.archivedTypeID != "dt-global-001" {
		t.Errorf("repo.archivedTypeID=%s want dt-global-001", repo.archivedTypeID)
	}
}

// TestCmsArchiveTemplate_NotFound verifies 404 is returned when template doesn't exist.
func TestCmsArchiveTemplate_NotFound(t *testing.T) {
	notFoundErr := perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "global template not found", nil)
	repo := &archiveRepo{archiveErr: notFoundErr}
	svc := newCMSArchiveService(
		[]string{permissionPlatformCMSView, permissionCMSTemplateArchive},
		repo,
	)
	_, err := svc.CmsArchiveTemplate(context.Background(), CmsArchiveTemplateRequest{
		Subject: Subject{UserID: "user-001", MembershipID: "m-001", CompanyID: "c-001"},
		TypeID:  "dt-nonexistent",
	})
	if err == nil {
		t.Fatal("expected error for non-existent template")
	}
	herr, ok := err.(*perr.HTTPError)
	if !ok {
		t.Fatalf("expected *perr.HTTPError, got %T", err)
	}
	if herr.HTTPStatus != http.StatusNotFound {
		t.Errorf("status=%d want 404", herr.HTTPStatus)
	}
}

// TestCmsArchiveTemplate_RequiresArchivePermission verifies 403 without archive permission.
func TestCmsArchiveTemplate_RequiresArchivePermission(t *testing.T) {
	repo := &archiveRepo{}
	svc := newCMSArchiveService(
		[]string{permissionPlatformCMSView, permissionCMSTemplateWrite}, // write but not archive
		repo,
	)
	_, err := svc.CmsArchiveTemplate(context.Background(), CmsArchiveTemplateRequest{
		Subject: Subject{UserID: "user-001", MembershipID: "m-001", CompanyID: "c-001"},
		TypeID:  "dt-global-001",
	})
	if err == nil {
		t.Fatal("expected permission error")
	}
	herr, ok := err.(*perr.HTTPError)
	if !ok {
		t.Fatalf("expected *perr.HTTPError, got %T", err)
	}
	if herr.HTTPStatus != http.StatusForbidden {
		t.Errorf("status=%d want 403", herr.HTTPStatus)
	}
}

// TestCmsArchiveTemplate_LegacyPublishPermissionAllowed verifies legacy disclosure_type.publish works.
func TestCmsArchiveTemplate_LegacyPublishPermissionAllowed(t *testing.T) {
	repo := &archiveRepo{}
	svc := newCMSArchiveService(
		[]string{permissionPlatformCMSView, permissionLegacyPublish},
		repo,
	)
	resp, err := svc.CmsArchiveTemplate(context.Background(), CmsArchiveTemplateRequest{
		Subject: Subject{UserID: "user-001", MembershipID: "m-001", CompanyID: "c-001"},
		TypeID:  "dt-global-001",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != "archived" {
		t.Errorf("status=%s want archived", resp.Status)
	}
}

// TestCmsArchiveTemplate_EmptyTypeID verifies 400 for empty type_id.
func TestCmsArchiveTemplate_EmptyTypeID(t *testing.T) {
	repo := &archiveRepo{}
	svc := newCMSArchiveService(
		[]string{permissionPlatformCMSView, permissionCMSTemplateArchive},
		repo,
	)
	_, err := svc.CmsArchiveTemplate(context.Background(), CmsArchiveTemplateRequest{
		Subject: Subject{UserID: "user-001", MembershipID: "m-001", CompanyID: "c-001"},
		TypeID:  "",
	})
	if err == nil {
		t.Fatal("expected error for empty type_id")
	}
	herr, ok := err.(*perr.HTTPError)
	if !ok {
		t.Fatalf("expected *perr.HTTPError, got %T", err)
	}
	if herr.HTTPStatus != http.StatusBadRequest {
		t.Errorf("status=%d want 400", herr.HTTPStatus)
	}
}
