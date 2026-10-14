package app_test

import (
	"context"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/infra/inmemory"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

type spyWorkflowBootstrap struct{ calls int }

func (b *spyWorkflowBootstrap) EnsureOnSubmit(context.Context, disclosureapp.Subject, disclosureapp.RecordDTO) (string, error) {
	b.calls++
	return "wf-template", nil
}

func submitWithBootstrap(t *testing.T, skip bool) (*disclosureapp.RecordDTO, *spyWorkflowBootstrap) {
	t.Helper()
	repo := inmemory.NewRepository()
	svc := disclosureapp.NewService(repo, submitAllowAuth{}, idgen.UUIDv7Generator{})
	spy := &spyWorkflowBootstrap{}
	svc.(interface {
		SetWorkflowBootstrap(disclosureapp.WorkflowBootstrapper)
	}).SetWorkflowBootstrap(spy)
	created, err := repo.Create(context.Background(), disclosureapp.RecordDTO{
		RecordID: "rec_bs_1", CompanyID: "c1", TypeID: "t1", Title: "BS", Content: "c", Status: "Draft", CreatedBy: "u1", UpdatedBy: "u1",
	})
	if err != nil {
		t.Fatal(err)
	}
	out, err := svc.SubmitRecord(context.Background(), disclosureapp.SubmitRecordRequest{
		Subject:               disclosureapp.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"},
		RecordID:              created.RecordID,
		SkipWorkflowBootstrap: skip,
	})
	if err != nil {
		t.Fatal(err)
	}
	return out, spy
}

// Portal submit of a record without a workflow still bootstraps one from the effective template.
func TestSubmitRecord_BootstrapsWorkflowByDefault(t *testing.T) {
	out, spy := submitWithBootstrap(t, false)
	if spy.calls != 1 || out.WorkflowInstanceID != "wf-template" {
		t.Fatalf("calls=%d instance=%q", spy.calls, out.WorkflowInstanceID)
	}
}

// Callers that create their own instance (ad-hoc proposal materialization) must not get a second,
// template-based instance for the same record.
func TestSubmitRecord_SkipWorkflowBootstrap(t *testing.T) {
	out, spy := submitWithBootstrap(t, true)
	if spy.calls != 0 || out.WorkflowInstanceID != "" {
		t.Fatalf("bootstrap must be skipped: calls=%d instance=%q", spy.calls, out.WorkflowInstanceID)
	}
	if out.SubmittedAt == nil {
		t.Fatal("submit itself must still happen")
	}
}
