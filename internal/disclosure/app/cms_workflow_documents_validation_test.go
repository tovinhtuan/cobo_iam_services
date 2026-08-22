package app_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/infra/inmemory"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

type recordingDocBinder struct {
	calls   [][3]string // fileID, bindScope, companyID
	allowed map[string]bool
}

func (b *recordingDocBinder) AssertCanBind(_ context.Context, fileID, bindScope, bindCompanyID string) error {
	b.calls = append(b.calls, [3]string{fileID, bindScope, bindCompanyID})
	if b.allowed != nil && b.allowed[fileID] {
		return nil
	}
	return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "cannot reference this file", nil)
}

func newSeededWFServiceWithBinder(t *testing.T, typeID string, binder disclosureapp.WorkflowDocTemplateBinder) (disclosureapp.Service, *inmemory.Repository) {
	t.Helper()
	repo := inmemory.NewRepository()
	seedTemplateDraft(t, repo, typeID)
	opts := []disclosureapp.ServiceOption{}
	if binder != nil {
		opts = append(opts, disclosureapp.WithWorkflowDocTemplateBinder(binder))
	}
	return disclosureapp.NewService(repo, nil, idgen.UUIDv7Generator{}, opts...), repo
}

func cmsOneStep(docs []disclosureapp.WorkflowDocumentDTO) []disclosureapp.GlobalWorkflowStepInput {
	return []disclosureapp.GlobalWorkflowStepInput{{
		Stage: "Review", DepartmentID: "d1", AssigneeRoleIds: []string{"r1"},
		ProcessingDays: 3, DueRule: "T+3", DisplayOrder: 1, Documents: docs,
	}}
}

func TestCmsUpsert_NameOnlyDocumentAccepted(t *testing.T) {
	const typeID = "dt-cms-doc-name-only"
	binder := &recordingDocBinder{allowed: map[string]bool{}}
	svc, _ := newSeededWFServiceWithBinder(t, typeID, binder)
	wf, err := svc.CmsUpsertGlobalWorkflow(context.Background(), disclosureapp.CmsUpsertGlobalWorkflowRequest{
		Subject: testSubjectWF, TypeID: typeID,
		Steps: cmsOneStep([]disclosureapp.WorkflowDocumentDTO{
			{DocID: "doc-a", Name: "Biên bản xác nhận", Required: true},
		}),
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if len(binder.calls) != 0 {
		t.Fatalf("AssertCanBind must not run when file absent, calls=%v", binder.calls)
	}
	if len(wf.Steps[0].Documents) != 1 || wf.Steps[0].Documents[0].Name != "Biên bản xác nhận" {
		t.Fatalf("documents=%#v", wf.Steps[0].Documents)
	}
}

func TestCmsUpsert_BlankNameRejected(t *testing.T) {
	const typeID = "dt-cms-doc-blank"
	svc, _ := newSeededWFServiceWithBinder(t, typeID, &recordingDocBinder{allowed: map[string]bool{}})
	_, err := svc.CmsUpsertGlobalWorkflow(context.Background(), disclosureapp.CmsUpsertGlobalWorkflowRequest{
		Subject: testSubjectWF, TypeID: typeID,
		Steps: cmsOneStep([]disclosureapp.WorkflowDocumentDTO{
			{DocID: "doc-a", Name: "", Required: true},
		}),
	})
	if err == nil {
		t.Fatal("expected blank name reject")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("want 400, got %#v", err)
	}
}

func TestCmsUpsert_WhitespaceNameRejected(t *testing.T) {
	const typeID = "dt-cms-doc-ws"
	svc, _ := newSeededWFServiceWithBinder(t, typeID, &recordingDocBinder{allowed: map[string]bool{}})
	_, err := svc.CmsUpsertGlobalWorkflow(context.Background(), disclosureapp.CmsUpsertGlobalWorkflowRequest{
		Subject: testSubjectWF, TypeID: typeID,
		Steps: cmsOneStep([]disclosureapp.WorkflowDocumentDTO{
			{DocID: "doc-a", Name: "   ", Required: true},
		}),
	})
	if err == nil {
		t.Fatal("expected whitespace name reject")
	}
}

func TestCmsUpsert_FileWithBlankNameRejected(t *testing.T) {
	const typeID = "dt-cms-doc-file-blank"
	binder := &recordingDocBinder{allowed: map[string]bool{"wdt_cms_ok": true}}
	svc, _ := newSeededWFServiceWithBinder(t, typeID, binder)
	_, err := svc.CmsUpsertGlobalWorkflow(context.Background(), disclosureapp.CmsUpsertGlobalWorkflowRequest{
		Subject: testSubjectWF, TypeID: typeID,
		Steps: cmsOneStep([]disclosureapp.WorkflowDocumentDTO{
			{DocID: "doc-a", Name: "", Required: true, TemplateFileID: "wdt_cms_ok", TemplateFileName: "a.xlsx"},
		}),
	})
	if err == nil {
		t.Fatal("file must not bypass blank name")
	}
	if len(binder.calls) != 0 {
		t.Fatalf("name fail should short-circuit before bind, calls=%v", binder.calls)
	}
}

func TestCmsUpsert_ValidCMSFileAccepted(t *testing.T) {
	const typeID = "dt-cms-doc-file-ok"
	binder := &recordingDocBinder{allowed: map[string]bool{"wdt_cms_f1": true}}
	svc, _ := newSeededWFServiceWithBinder(t, typeID, binder)
	wf, err := svc.CmsUpsertGlobalWorkflow(context.Background(), disclosureapp.CmsUpsertGlobalWorkflowRequest{
		Subject: testSubjectWF, TypeID: typeID,
		Steps: cmsOneStep([]disclosureapp.WorkflowDocumentDTO{
			{DocID: "doc-a", Name: "BCTC Q3", Required: true, TemplateFileID: "wdt_cms_f1", TemplateFileName: "form.xlsx"},
		}),
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if len(binder.calls) != 1 || binder.calls[0][0] != "wdt_cms_f1" || binder.calls[0][1] != "cms" {
		t.Fatalf("AssertCanBind calls=%v want cms scope for wdt_cms_f1", binder.calls)
	}
	if wf.Steps[0].Documents[0].TemplateFileID != "wdt_cms_f1" {
		t.Fatalf("file dropped: %#v", wf.Steps[0].Documents)
	}
}

func TestCmsUpsert_NonexistentOrDeniedFileRejected(t *testing.T) {
	const typeID = "dt-cms-doc-file-deny"
	binder := &recordingDocBinder{allowed: map[string]bool{}} // deny all
	svc, _ := newSeededWFServiceWithBinder(t, typeID, binder)
	_, err := svc.CmsUpsertGlobalWorkflow(context.Background(), disclosureapp.CmsUpsertGlobalWorkflowRequest{
		Subject: testSubjectWF, TypeID: typeID,
		Steps: cmsOneStep([]disclosureapp.WorkflowDocumentDTO{
			{DocID: "doc-a", Name: "BCTC", Required: true, TemplateFileID: "wdt_missing"},
		}),
	})
	if err == nil {
		t.Fatal("expected bind reject")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusForbidden {
		t.Fatalf("want 403 cannot reference, got %#v", err)
	}
	if len(binder.calls) != 1 || binder.calls[0][1] != "cms" {
		t.Fatalf("calls=%v", binder.calls)
	}
}

func TestCmsUpsert_OneInvalidAmongManyRejectsEntireWrite(t *testing.T) {
	const typeID = "dt-cms-doc-partial"
	binder := &recordingDocBinder{allowed: map[string]bool{"wdt_ok": true}}
	svc, _ := newSeededWFServiceWithBinder(t, typeID, binder)
	_, err := svc.CmsUpsertGlobalWorkflow(context.Background(), disclosureapp.CmsUpsertGlobalWorkflowRequest{
		Subject: testSubjectWF, TypeID: typeID,
		Steps: cmsOneStep([]disclosureapp.WorkflowDocumentDTO{
			{DocID: "doc-a", Name: "A", Required: true},
			{DocID: "doc-b", Name: "   ", Required: true, TemplateFileID: "wdt_ok"},
			{DocID: "doc-c", Name: "C", Required: true},
		}),
	})
	if err == nil {
		t.Fatal("expected reject")
	}
	got, gerr := svc.CmsGetGlobalWorkflow(context.Background(), disclosureapp.CmsGetGlobalWorkflowRequest{
		Subject: testSubjectWF, TypeID: typeID,
	})
	if gerr != nil {
		t.Fatalf("get: %v", gerr)
	}
	// Seed had empty workflow steps in block; failed upsert must not persist invalid docs.
	for _, st := range got.Data.Steps {
		for _, d := range st.Documents {
			if strings.TrimSpace(d.Name) == "" {
				t.Fatalf("blank name leaked into stored workflow: %#v", st.Documents)
			}
		}
	}
}

func TestCmsUpsert_EmptyDocumentsListValid(t *testing.T) {
	const typeID = "dt-cms-doc-empty"
	svc, _ := newSeededWFServiceWithBinder(t, typeID, &recordingDocBinder{allowed: map[string]bool{}})
	wf, err := svc.CmsUpsertGlobalWorkflow(context.Background(), disclosureapp.CmsUpsertGlobalWorkflowRequest{
		Subject: testSubjectWF, TypeID: typeID,
		Steps: cmsOneStep(nil),
	})
	if err != nil {
		t.Fatalf("empty documents must be valid: %v", err)
	}
	if len(wf.Steps[0].Documents) != 0 {
		t.Fatalf("documents=%#v", wf.Steps[0].Documents)
	}
}

func TestCmsUpsert_FileRemovalClearsTemplateRefs(t *testing.T) {
	const typeID = "dt-cms-doc-remove-file"
	binder := &recordingDocBinder{allowed: map[string]bool{"wdt_f1": true}}
	svc, _ := newSeededWFServiceWithBinder(t, typeID, binder)
	_, err := svc.CmsUpsertGlobalWorkflow(context.Background(), disclosureapp.CmsUpsertGlobalWorkflowRequest{
		Subject: testSubjectWF, TypeID: typeID,
		Steps: cmsOneStep([]disclosureapp.WorkflowDocumentDTO{
			{DocID: "doc-a", Name: "A", Required: true, TemplateFileID: "wdt_f1", TemplateFileName: "a.xlsx"},
		}),
	})
	if err != nil {
		t.Fatalf("seed file: %v", err)
	}
	wf, err := svc.CmsUpsertGlobalWorkflow(context.Background(), disclosureapp.CmsUpsertGlobalWorkflowRequest{
		Subject: testSubjectWF, TypeID: typeID,
		Steps: cmsOneStep([]disclosureapp.WorkflowDocumentDTO{
			{DocID: "doc-a", Name: "A", Required: true},
		}),
	})
	if err != nil {
		t.Fatalf("remove file: %v", err)
	}
	d := wf.Steps[0].Documents[0]
	if d.DocID != "doc-a" || d.Name != "A" || d.TemplateFileID != "" {
		t.Fatalf("want name-only preserved doc_id, got %#v", d)
	}
}

func TestCmsUpsert_DocIDStableAcrossFileReplace(t *testing.T) {
	const typeID = "dt-cms-doc-stable-id"
	binder := &recordingDocBinder{allowed: map[string]bool{"wdt_f1": true, "wdt_f2": true}}
	svc, _ := newSeededWFServiceWithBinder(t, typeID, binder)
	_, err := svc.CmsUpsertGlobalWorkflow(context.Background(), disclosureapp.CmsUpsertGlobalWorkflowRequest{
		Subject: testSubjectWF, TypeID: typeID,
		Steps: cmsOneStep([]disclosureapp.WorkflowDocumentDTO{
			{DocID: "doc-stable", Name: "A", Required: true, TemplateFileID: "wdt_f1"},
		}),
	})
	if err != nil {
		t.Fatalf("f1: %v", err)
	}
	wf, err := svc.CmsUpsertGlobalWorkflow(context.Background(), disclosureapp.CmsUpsertGlobalWorkflowRequest{
		Subject: testSubjectWF, TypeID: typeID,
		Steps: cmsOneStep([]disclosureapp.WorkflowDocumentDTO{
			{DocID: "doc-stable", Name: "A", Required: true, TemplateFileID: "wdt_f2", TemplateFileName: "b.xlsx"},
		}),
	})
	if err != nil {
		t.Fatalf("f2: %v", err)
	}
	if wf.Steps[0].Documents[0].DocID != "doc-stable" || wf.Steps[0].Documents[0].TemplateFileID != "wdt_f2" {
		t.Fatalf("%#v", wf.Steps[0].Documents[0])
	}
}
