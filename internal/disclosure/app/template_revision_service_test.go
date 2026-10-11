package app_test

import (
	"context"
	"net/http"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

func revisionStep(stage string) []disclosureapp.GlobalWorkflowStepInput {
	return []disclosureapp.GlobalWorkflowStepInput{{
		StepID: "review", Stage: stage, DepartmentID: "dept-compliance",
		AssigneeRoleIds: []string{"role-reviewer"}, DueRule: "T+5", ProcessingDays: 5, DisplayOrder: 1,
	}}
}

func headRevisionOf(t *testing.T, svc disclosureapp.Service, typeID string) string {
	t.Helper()
	out, err := svc.ListTypeVersions(context.Background(), disclosureapp.ListTypeVersionsRequest{Subject: testSubjectWF, TypeID: typeID})
	if err != nil {
		t.Fatalf("ListTypeVersions: %v", err)
	}
	return out.HeadRevision
}

func TestTemplateRevision_ExposedAndEnforcedOnWorkflowFacade(t *testing.T) {
	ctx := context.Background()
	repo := inmemory.NewRepository()
	const typeID = "dt-revision-facade"
	seedTemplateDraft(t, repo, typeID)
	svc := disclosureapp.NewService(repo, nil, idgen.UUIDv7Generator{})

	// First save (legacy, no token) materializes a candidate hash.
	saved, err := svc.CmsUpsertGlobalWorkflow(ctx, disclosureapp.CmsUpsertGlobalWorkflowRequest{
		Subject: testSubjectWF, TypeID: typeID, ChangeNote: "init", Steps: revisionStep("Review"),
	})
	if err != nil {
		t.Fatalf("initial save: %v", err)
	}
	head := headRevisionOf(t, svc, typeID)
	if head == "" || saved.Revision != head {
		t.Fatalf("save response revision %q must equal head_revision %q", saved.Revision, head)
	}
	got, err := svc.CmsGetGlobalWorkflow(ctx, disclosureapp.CmsGetGlobalWorkflowRequest{Subject: testSubjectWF, TypeID: typeID})
	if err != nil || got.Data.Revision != head {
		t.Fatalf("GET workflow revision=%q err=%v want %q", got.Data.Revision, err, head)
	}

	// Fresh token → OK and a new revision.
	next, err := svc.CmsUpsertGlobalWorkflow(ctx, disclosureapp.CmsUpsertGlobalWorkflowRequest{
		Subject: testSubjectWF, TypeID: typeID, Steps: revisionStep("Review 2"), ExpectedRevision: head,
	})
	if err != nil {
		t.Fatalf("save with fresh revision: %v", err)
	}
	if next.Revision == "" || next.Revision == head {
		t.Fatalf("content changed, revision must change: %q", next.Revision)
	}

	// Stale token → 409 STALE_ETAG.
	_, err = svc.CmsUpsertGlobalWorkflow(ctx, disclosureapp.CmsUpsertGlobalWorkflowRequest{
		Subject: testSubjectWF, TypeID: typeID, Steps: revisionStep("Review 3"), ExpectedRevision: head,
	})
	if he, ok := perr.AsHTTPError(err); !ok || he.HTTPStatus != http.StatusConflict || he.Code != perr.CodeStaleEtag {
		t.Fatalf("want 409 STALE_ETAG, got %v", err)
	}
	err = svc.CmsDeleteGlobalWorkflow(ctx, disclosureapp.CmsDeleteGlobalWorkflowRequest{Subject: testSubjectWF, TypeID: typeID, ExpectedRevision: head})
	if he, ok := perr.AsHTTPError(err); !ok || he.Code != perr.CodeStaleEtag {
		t.Fatalf("delete with stale revision: want STALE_ETAG, got %v", err)
	}

	// Version detail carries the same revision as the head.
	items, _ := svc.ListTypeVersions(ctx, disclosureapp.ListTypeVersionsRequest{Subject: testSubjectWF, TypeID: typeID})
	detail, err := svc.GetTypeVersionDetail(ctx, disclosureapp.GetTypeVersionDetailRequest{Subject: testSubjectWF, TypeID: typeID, VersionNo: items.Items[0].VersionNo})
	if err != nil || detail.Revision == "" || detail.Revision != items.Items[0].Revision {
		t.Fatalf("detail revision=%q err=%v want %q", detail.Revision, err, items.Items[0].Revision)
	}
}

func TestTemplateRevision_ActivateRejectsStaleClientRevision(t *testing.T) {
	ctx := context.Background()
	repo := inmemory.NewRepository()
	const typeID = "dt-revision-activate"
	seedTemplateDraft(t, repo, typeID)
	svc := disclosureapp.NewService(repo, nil, idgen.UUIDv7Generator{})
	if _, err := svc.CmsUpsertGlobalWorkflow(ctx, disclosureapp.CmsUpsertGlobalWorkflowRequest{
		Subject: testSubjectWF, TypeID: typeID, Steps: revisionStep("Review"),
	}); err != nil {
		t.Fatalf("seed save: %v", err)
	}
	items, _ := svc.ListTypeVersions(ctx, disclosureapp.ListTypeVersionsRequest{Subject: testSubjectWF, TypeID: typeID})
	_, err := svc.ActivateTypeVersion(ctx, disclosureapp.ActivateTypeVersionRequest{
		Subject: testSubjectWF, TypeID: typeID, VersionNo: items.Items[0].VersionNo,
		ExpectedCandidateHash: "not-the-reviewed-content", ExpectedFromClient: true,
	})
	if he, ok := perr.AsHTTPError(err); !ok || he.HTTPStatus != http.StatusConflict || he.Code != perr.CodeStaleEtag {
		t.Fatalf("want 409 STALE_ETAG, got %v", err)
	}
}
