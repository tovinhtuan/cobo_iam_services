package app

import (
	"context"
	"net/http"
	"slices"
	"testing"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// MR-3 (PO decision Q1-B): archiving suspends the template's cycles without a record; a restore
// backfills only the cycles Admin Web picks; the archive impact needs the archive permission.

type impactRepo struct {
	archiveRepo
	impactTypeID string
	impactToday  string
}

func (r *impactRepo) GetTemplateArchiveImpact(_ context.Context, typeID, todayHCM string) (*CmsTemplateArchiveImpactResponse, error) {
	r.impactTypeID, r.impactToday = typeID, todayHCM
	return &CmsTemplateArchiveImpactResponse{TypeID: typeID, Status: "active"}, nil
}

var cmsArchiveSubject = Subject{UserID: "user-001", MembershipID: "m-001", CompanyID: "c-001"}

func TestCmsArchiveTemplate_ReportsSuspendedCycles(t *testing.T) {
	repo := &archiveRepo{archiveResult: &ArchiveGlobalTemplateResult{TypeID: "dt-1", AfterStatus: "archived", SuspendedCycles: 3}}
	svc := newCMSArchiveService([]string{permissionPlatformCMSView, permissionCMSTemplateArchive}, repo)
	resp, err := svc.CmsArchiveTemplate(context.Background(), CmsArchiveTemplateRequest{Subject: cmsArchiveSubject, TypeID: "dt-1"})
	if err != nil || resp.SuspendedCycles != 3 {
		t.Fatalf("resp=%+v err=%v, want suspended_cycles=3", resp, err)
	}
}

func TestCmsRestoreTemplate_PassesTheBackfillChoice(t *testing.T) {
	repo := &archiveRepo{
		lifecycle:     &TypeLifecycleDTO{TypeID: "dt-1", Status: "archived"},
		restoreResult: &RestoreGlobalTemplateResult{TypeID: "dt-1", AfterStatus: "active", RestoredMode: "draft", BackfilledCycles: 2, SkippedCycles: 1},
	}
	svc := newCMSArchiveService([]string{permissionPlatformCMSView, permissionCMSTemplateArchive}, repo)
	resp, err := svc.CmsRestoreTemplate(context.Background(), CmsRestoreTemplateRequest{
		Subject: cmsArchiveSubject, TypeID: "dt-1", BackfillCycleIDs: []string{" cy-1 ", "cy-2", "cy-1", ""},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(repo.restoreParams.BackfillCycleIDs, []string{"cy-1", "cy-2"}) {
		t.Fatalf("backfill ids=%v, want trimmed and deduplicated [cy-1 cy-2]", repo.restoreParams.BackfillCycleIDs)
	}
	if resp.BackfilledCycles != 2 || resp.SkippedCycles != 1 {
		t.Fatalf("resp=%+v", resp)
	}
}

func TestCmsRestoreTemplate_DefaultsToSkippingEverySuspendedCycle(t *testing.T) {
	repo := &archiveRepo{lifecycle: &TypeLifecycleDTO{TypeID: "dt-1", Status: "archived"}}
	svc := newCMSArchiveService([]string{permissionPlatformCMSView, permissionCMSTemplateArchive}, repo)
	if _, err := svc.CmsRestoreTemplate(context.Background(), CmsRestoreTemplateRequest{Subject: cmsArchiveSubject, TypeID: "dt-1"}); err != nil {
		t.Fatal(err)
	}
	if len(repo.restoreParams.BackfillCycleIDs) != 0 {
		t.Fatalf("backfill ids=%v, want none: nothing is created in bulk without a choice", repo.restoreParams.BackfillCycleIDs)
	}
}

func TestCmsRestoreTemplate_RejectsTooManyBackfillIDs(t *testing.T) {
	repo := &archiveRepo{lifecycle: &TypeLifecycleDTO{TypeID: "dt-1", Status: "archived"}}
	svc := newCMSArchiveService([]string{permissionPlatformCMSView, permissionCMSTemplateArchive}, repo)
	ids := make([]string, MaxBackfillCycleIDs+1)
	for i := range ids {
		ids[i] = "cy"
	}
	_, err := svc.CmsRestoreTemplate(context.Background(), CmsRestoreTemplateRequest{Subject: cmsArchiveSubject, TypeID: "dt-1", BackfillCycleIDs: ids})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusUnprocessableEntity || he.Details["field"] != "backfill_cycle_ids" {
		t.Fatalf("err=%v, want 422 on backfill_cycle_ids", err)
	}
}

func TestCmsTemplateArchiveImpact_RequiresTheArchivePermission(t *testing.T) {
	repo := &impactRepo{}
	svc := newCMSArchiveService([]string{permissionPlatformCMSView}, repo)
	_, err := svc.CmsTemplateArchiveImpact(context.Background(), CmsTemplateArchiveImpactRequest{Subject: cmsArchiveSubject, TypeID: "dt-1"})
	if he, ok := perr.AsHTTPError(err); !ok || he.HTTPStatus != http.StatusForbidden {
		t.Fatalf("err=%v, want 403 without cms.template.archive", err)
	}
	if repo.impactTypeID != "" {
		t.Fatal("impact read without permission")
	}
}

func TestCmsTemplateArchiveImpact_ReadsWithTheBusinessDate(t *testing.T) {
	repo := &impactRepo{}
	svc := newCMSArchiveService([]string{permissionPlatformCMSView, permissionCMSTemplateArchive}, repo)
	resp, err := svc.CmsTemplateArchiveImpact(context.Background(), CmsTemplateArchiveImpactRequest{Subject: cmsArchiveSubject, TypeID: " dt-1 "})
	if err != nil || resp.TypeID != "dt-1" || repo.impactTypeID != "dt-1" || len(repo.impactToday) != len("2006-01-02") {
		t.Fatalf("resp=%+v err=%v type=%q today=%q", resp, err, repo.impactTypeID, repo.impactToday)
	}
}

// A tenant role carrying the archive or legacy publish permission is not Admin Web without
// platform.cms.view: neither the impact nor the restore (with a backfill choice) reaches the repo.
func TestCmsArchiveImpactAndRestore_RequirePlatformCMSView(t *testing.T) {
	for _, perms := range [][]string{{permissionCMSTemplateArchive}, {permissionLegacyPublish}} {
		repo := &impactRepo{archiveRepo: archiveRepo{lifecycle: &TypeLifecycleDTO{TypeID: "dt-1", Status: "archived"}}}
		svc := newCMSArchiveService(perms, repo)
		_, err := svc.CmsTemplateArchiveImpact(context.Background(), CmsTemplateArchiveImpactRequest{Subject: cmsArchiveSubject, TypeID: "dt-1"})
		if he, ok := perr.AsHTTPError(err); !ok || he.HTTPStatus != http.StatusForbidden {
			t.Fatalf("perms=%v impact err=%v, want 403", perms, err)
		}
		_, err = svc.CmsRestoreTemplate(context.Background(), CmsRestoreTemplateRequest{Subject: cmsArchiveSubject, TypeID: "dt-1", BackfillCycleIDs: []string{"cy-1"}})
		if he, ok := perr.AsHTTPError(err); !ok || he.HTTPStatus != http.StatusForbidden {
			t.Fatalf("perms=%v restore err=%v, want 403", perms, err)
		}
		if repo.impactTypeID != "" || repo.restoreParams.TypeID != "" {
			t.Fatalf("perms=%v reached the repo (impact=%q restore=%q)", perms, repo.impactTypeID, repo.restoreParams.TypeID)
		}
	}
}
