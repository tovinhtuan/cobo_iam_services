package http_test

import (
	"context"
	"net/http"
	"testing"

	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
	wfcapp "github.com/cobo/cobo_iam_services/internal/workflowconfig/app"
	wfchttp "github.com/cobo/cobo_iam_services/internal/workflowconfig/transport/http"
)

type recordingAudit struct {
	entries []auditapp.AppendAuditLogRequest
}

func (r *recordingAudit) AppendAuditLog(_ context.Context, req auditapp.AppendAuditLogRequest) error {
	r.entries = append(r.entries, req)
	return nil
}

func newAuditedServer(audit *recordingAudit) *http.ServeMux {
	versionSvc := wfcapp.NewVersionService(newFakeVersionRepo(), nil)
	readinessSvc := wfcapp.NewReadinessService(versionSvc, wfcapp.DefaultRoleRegistry())
	configSvc := wfcapp.NewConfigService(versionSvc, readinessSvc)
	mux := http.NewServeMux()
	wfchttp.NewHandler(versionSvc, configSvc, nil, fakeInspector{}, fakeAuthorizer{}).WithAudit(audit).Register(mux)
	return mux
}

func TestWorkflowVersionActivate_WritesAudit(t *testing.T) {
	audit := &recordingAudit{}
	mux := newAuditedServer(audit)
	rec := do(t, mux, http.MethodPost, "/api/v1/platform/cms/templates/t1/workflow/versions/1/activate", tokCMSChecker, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(audit.entries) != 1 {
		t.Fatalf("want 1 audit entry, got %d", len(audit.entries))
	}
	e := audit.entries[0]
	if e.Action != "cms_workflow.version.activate" || e.ResourceType != "disclosure_type" || e.ResourceID != "t1" {
		t.Fatalf("unexpected audit entry: %+v", e)
	}
	if e.Metadata["version_no"] != 1 {
		t.Fatalf("metadata version_no=%v", e.Metadata["version_no"])
	}
	if _, ok := e.Metadata["prev_active_version_no"]; !ok {
		t.Fatal("metadata must include prev_active_version_no")
	}
	if e.ActorUserID != "u-"+tokCMSChecker {
		t.Fatalf("actor=%q", e.ActorUserID)
	}
}

func TestWorkflowVersionPublish_FailureWritesNoAudit(t *testing.T) {
	audit := &recordingAudit{}
	mux := newAuditedServer(audit)
	// fake manifest has no steps → validation fails → no audit
	rec := do(t, mux, http.MethodPost, "/api/v1/platform/cms/templates/t1/workflow/publish", tokCMSMaker, `{"change_note":"x"}`)
	if rec.Code < 400 {
		t.Fatalf("expected failure for empty manifest, got %d", rec.Code)
	}
	if len(audit.entries) != 0 {
		t.Fatalf("failed publish must not be audited, got %+v", audit.entries)
	}
}

func TestWorkflowVersionPublish_MalformedBodyRejected(t *testing.T) {
	audit := &recordingAudit{}
	mux := newAuditedServer(audit)
	rec := do(t, mux, http.MethodPost, "/api/v1/platform/cms/templates/t1/workflow/publish", tokCMSMaker, `{"change_note":`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400; body=%s", rec.Code, rec.Body.String())
	}
}
