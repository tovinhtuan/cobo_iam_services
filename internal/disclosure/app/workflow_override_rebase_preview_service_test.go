package app_test

import (
	"context"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

// seedGlobalManifestPair seeds base and target global manifest versions for staleTestTypeID,
// using "seed-step-1" as the shared step_id so seedOverride's own fixture step (same step_id)
// bridges correctly per PREFLIGHT_AUDIT.md §5. baseStage/targetStage let a test control whether
// the two versions actually differ (so the diff produces a real operation) or are identical.
func seedGlobalManifestPair(repo *inmemory.Repository, baseVersionNo, targetVersionNo int, baseStage, targetStage string) {
	repo.SetGlobalWorkflowVersionManifestForTest(staleTestTypeID, baseVersionNo, []disclosureapp.GlobalWorkflowStepInput{
		{StepKey: "stepkey-1", StepID: "seed-step-1", Stage: baseStage, DepartmentID: "d1", AssigneeRoleIds: []string{"reviewer"}, DueRule: "", ProcessingDays: 0, DisplayOrder: 1},
	})
	repo.SetGlobalWorkflowVersionManifestForTest(staleTestTypeID, targetVersionNo, []disclosureapp.GlobalWorkflowStepInput{
		{StepKey: "stepkey-1", StepID: "seed-step-1", Stage: targetStage, DepartmentID: "d1", AssigneeRoleIds: []string{"reviewer"}, DueRule: "", ProcessingDays: 0, DisplayOrder: 1},
	})
}

func TestGetWorkflowOverrideRebasePreview_NoOverride_Returns404OverrideNotFound(t *testing.T) {
	svc, _ := newStalenessTestService(t, true)
	_, err := svc.GetWorkflowOverrideRebasePreview(context.Background(), disclosureapp.GetWorkflowOverrideRebasePreviewRequest{
		Subject: disclosureapp.Subject{CompanyID: "company-no-override"}, TypeID: staleTestTypeID,
	})
	httpErr, ok := err.(*perr.HTTPError)
	if !ok {
		t.Fatalf("expected *perr.HTTPError, got %T (%v)", err, err)
	}
	if httpErr.HTTPStatus != 404 {
		t.Errorf("HTTPStatus = %d, want 404", httpErr.HTTPStatus)
	}
	if httpErr.Code != perr.CodeOverrideNotFound {
		t.Errorf("Code = %q, want %q", httpErr.Code, perr.CodeOverrideNotFound)
	}
}

func TestGetWorkflowOverrideRebasePreview_TypeNotFound_Returns404(t *testing.T) {
	svc, _ := newStalenessTestService(t, true)
	_, err := svc.GetWorkflowOverrideRebasePreview(context.Background(), disclosureapp.GetWorkflowOverrideRebasePreviewRequest{
		Subject: disclosureapp.Subject{CompanyID: "company-x"}, TypeID: "this-type-does-not-exist",
	})
	httpErr, ok := err.(*perr.HTTPError)
	if !ok {
		t.Fatalf("expected *perr.HTTPError, got %T (%v)", err, err)
	}
	if httpErr.HTTPStatus != 404 {
		t.Errorf("HTTPStatus = %d, want 404", httpErr.HTTPStatus)
	}
}

func TestGetWorkflowOverrideRebasePreview_CurrentOverride_Returns409NotStale(t *testing.T) {
	svc, repo := newStalenessTestService(t, true)
	companyID := "company-current"
	seedOverride(t, repo, companyID, staleTestTypeID, disclosureapp.BaseSourceGlobalWorkflow, intPtrTest(5))
	if err := repo.SetGlobalActiveVersionForTest(staleTestTypeID, 5); err != nil {
		t.Fatalf("set global active version: %v", err)
	}

	_, err := svc.GetWorkflowOverrideRebasePreview(context.Background(), disclosureapp.GetWorkflowOverrideRebasePreviewRequest{
		Subject: disclosureapp.Subject{CompanyID: companyID}, TypeID: staleTestTypeID,
	})
	httpErr, ok := err.(*perr.HTTPError)
	if !ok {
		t.Fatalf("expected *perr.HTTPError, got %T (%v)", err, err)
	}
	if httpErr.HTTPStatus != 409 {
		t.Errorf("HTTPStatus = %d, want 409", httpErr.HTTPStatus)
	}
	if httpErr.Code != perr.CodeNotStale {
		t.Errorf("Code = %q, want %q", httpErr.Code, perr.CodeNotStale)
	}
}

func TestGetWorkflowOverrideRebasePreview_MissingPermission_Returns403(t *testing.T) {
	svc, repo := newStalenessTestService(t, false)
	companyID := "company-forbidden"
	seedOverride(t, repo, companyID, staleTestTypeID, disclosureapp.BaseSourceGlobalWorkflow, intPtrTest(2))

	_, err := svc.GetWorkflowOverrideRebasePreview(context.Background(), disclosureapp.GetWorkflowOverrideRebasePreviewRequest{
		Subject: disclosureapp.Subject{CompanyID: companyID}, TypeID: staleTestTypeID,
	})
	httpErr, ok := err.(*perr.HTTPError)
	if !ok {
		t.Fatalf("expected *perr.HTTPError, got %T (%v)", err, err)
	}
	if httpErr.HTTPStatus != 403 {
		t.Errorf("HTTPStatus = %d, want 403", httpErr.HTTPStatus)
	}
}

func TestGetWorkflowOverrideRebasePreview_DecodeFailure_Returns410PreviewUnavailable(t *testing.T) {
	svc, repo := newStalenessTestService(t, true)
	companyID := "company-undecodable"
	// base_version_no=2 set, but NO manifest seeded for version 2 or the current active version
	// -> GetGlobalWorkflowVersionManifest returns ok=false for both.
	seedOverride(t, repo, companyID, staleTestTypeID, disclosureapp.BaseSourceGlobalWorkflow, intPtrTest(2))
	if err := repo.SetGlobalActiveVersionForTest(staleTestTypeID, 9); err != nil {
		t.Fatalf("set global active version: %v", err)
	}

	_, err := svc.GetWorkflowOverrideRebasePreview(context.Background(), disclosureapp.GetWorkflowOverrideRebasePreviewRequest{
		Subject: disclosureapp.Subject{CompanyID: companyID}, TypeID: staleTestTypeID,
	})
	httpErr, ok := err.(*perr.HTTPError)
	if !ok {
		t.Fatalf("expected *perr.HTTPError, got %T (%v)", err, err)
	}
	if httpErr.HTTPStatus != 410 {
		t.Errorf("HTTPStatus = %d, want 410", httpErr.HTTPStatus)
	}
	if httpErr.Code != perr.CodePreviewUnavailable {
		t.Errorf("Code = %q, want %q", httpErr.Code, perr.CodePreviewUnavailable)
	}
}

func TestGetWorkflowOverrideRebasePreview_UnknownBase_Returns410PreviewUnavailable(t *testing.T) {
	svc, repo := newStalenessTestService(t, true)
	companyID := "company-unknown-base"
	seedOverride(t, repo, companyID, staleTestTypeID, disclosureapp.BaseSourceUnknown, nil)
	if err := repo.SetGlobalActiveVersionForTest(staleTestTypeID, 9); err != nil {
		t.Fatalf("set global active version: %v", err)
	}

	_, err := svc.GetWorkflowOverrideRebasePreview(context.Background(), disclosureapp.GetWorkflowOverrideRebasePreviewRequest{
		Subject: disclosureapp.Subject{CompanyID: companyID}, TypeID: staleTestTypeID,
	})
	httpErr, ok := err.(*perr.HTTPError)
	if !ok {
		t.Fatalf("expected *perr.HTTPError for unknown base, got %T (%v)", err, err)
	}
	if httpErr.HTTPStatus != 410 {
		t.Errorf("HTTPStatus = %d, want 410 (unknown base must never be guessed into a preview)", httpErr.HTTPStatus)
	}
}

func TestGetWorkflowOverrideRebasePreview_Stale_Returns200WithDiff(t *testing.T) {
	svc, repo := newStalenessTestService(t, true)
	companyID := "company-stale-preview"
	seedOverride(t, repo, companyID, staleTestTypeID, disclosureapp.BaseSourceGlobalWorkflow, intPtrTest(2))
	if err := repo.SetGlobalActiveVersionForTest(staleTestTypeID, 5); err != nil {
		t.Fatalf("set global active version: %v", err)
	}
	seedGlobalManifestPair(repo, 2, 5, "Seed Step", "Seed Step Renamed By CMS")

	resp, err := svc.GetWorkflowOverrideRebasePreview(context.Background(), disclosureapp.GetWorkflowOverrideRebasePreviewRequest{
		Subject: disclosureapp.Subject{CompanyID: companyID}, TypeID: staleTestTypeID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Data.BaseVersionNo != 2 || resp.Data.TargetVersionNo != 5 {
		t.Errorf("version numbers wrong: base=%d target=%d", resp.Data.BaseVersionNo, resp.Data.TargetVersionNo)
	}
	if resp.Data.StaleStatus != disclosureapp.StaleStatusStale {
		t.Errorf("stale_status = %q, want %q", resp.Data.StaleStatus, disclosureapp.StaleStatusStale)
	}
	found := false
	for _, op := range resp.Data.PatchOperations {
		if op.StepKey == "stepkey-1" && op.Op == disclosureapp.OpUpdateStepField && op.FieldPath == "stage" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected an update_step_field/stage operation for stepkey-1, got %+v", resp.Data.PatchOperations)
	}
	if resp.Data.PreviewID != nil {
		t.Errorf("preview_id must be nil/ephemeral in Batch 3, got %v", *resp.Data.PreviewID)
	}
}

func TestGetWorkflowOverrideRebasePreview_ReturnsOwnCompanyOnly(t *testing.T) {
	svc, repo := newStalenessTestService(t, true)
	seedOverride(t, repo, "company-A", staleTestTypeID, disclosureapp.BaseSourceGlobalWorkflow, intPtrTest(2))
	seedOverride(t, repo, "company-B", staleTestTypeID, disclosureapp.BaseSourceGlobalWorkflow, intPtrTest(2))
	if err := repo.SetGlobalActiveVersionForTest(staleTestTypeID, 5); err != nil {
		t.Fatalf("set global active version: %v", err)
	}
	seedGlobalManifestPair(repo, 2, 5, "Seed Step", "Seed Step Renamed By CMS")

	respA, errA := svc.GetWorkflowOverrideRebasePreview(context.Background(), disclosureapp.GetWorkflowOverrideRebasePreviewRequest{
		Subject: disclosureapp.Subject{CompanyID: "company-A"}, TypeID: staleTestTypeID,
	})
	respB, errB := svc.GetWorkflowOverrideRebasePreview(context.Background(), disclosureapp.GetWorkflowOverrideRebasePreviewRequest{
		Subject: disclosureapp.Subject{CompanyID: "company-B"}, TypeID: staleTestTypeID,
	})
	if errA != nil || errB != nil {
		t.Fatalf("unexpected errors: A=%v B=%v", errA, errB)
	}
	if respA.Data.CompanyID != "company-A" || respB.Data.CompanyID != "company-B" {
		t.Errorf("cross-company leak: respA.CompanyID=%q respB.CompanyID=%q", respA.Data.CompanyID, respB.Data.CompanyID)
	}
}

// TestGetWorkflowOverrideRebasePreview_RuntimeInvariance is the central Batch 3 proof: calling
// the preview (any number of times) must never change GetEffectiveWorkflow's output.
func TestGetWorkflowOverrideRebasePreview_RuntimeInvariance(t *testing.T) {
	svc, repo := newStalenessTestService(t, true)
	companyID := "company-runtime-invariance-preview"
	seedOverride(t, repo, companyID, staleTestTypeID, disclosureapp.BaseSourceGlobalWorkflow, intPtrTest(2))
	if err := repo.SetGlobalActiveVersionForTest(staleTestTypeID, 5); err != nil {
		t.Fatalf("set global active version: %v", err)
	}
	seedGlobalManifestPair(repo, 2, 5, "Seed Step", "Seed Step Renamed By CMS")

	ctx := context.Background()
	before, err := svc.GetEffectiveWorkflow(ctx, disclosureapp.GetEffectiveWorkflowRequest{
		Subject: disclosureapp.Subject{CompanyID: companyID}, TypeID: staleTestTypeID,
	})
	if err != nil {
		t.Fatalf("GetEffectiveWorkflow before: %v", err)
	}

	for i := 0; i < 3; i++ {
		if _, err := svc.GetWorkflowOverrideRebasePreview(ctx, disclosureapp.GetWorkflowOverrideRebasePreviewRequest{
			Subject: disclosureapp.Subject{CompanyID: companyID}, TypeID: staleTestTypeID,
		}); err != nil {
			t.Fatalf("preview call %d: %v", i, err)
		}
	}

	after, err := svc.GetEffectiveWorkflow(ctx, disclosureapp.GetEffectiveWorkflowRequest{
		Subject: disclosureapp.Subject{CompanyID: companyID}, TypeID: staleTestTypeID,
	})
	if err != nil {
		t.Fatalf("GetEffectiveWorkflow after: %v", err)
	}

	if before.Data.Source != after.Data.Source || before.Data.VersionNo != after.Data.VersionNo {
		t.Errorf("GetEffectiveWorkflow changed: before=%+v after=%+v", before.Data, after.Data)
	}
	if len(before.Data.Workflow) != len(after.Data.Workflow) {
		t.Fatalf("workflow length changed: before=%d after=%d", len(before.Data.Workflow), len(after.Data.Workflow))
	}
	for i := range before.Data.Workflow {
		if before.Data.Workflow[i].Stage != after.Data.Workflow[i].Stage {
			t.Errorf("step %d Stage changed: before=%q after=%q — calling rebase-preview must never mutate the override's own snapshot", i, before.Data.Workflow[i].Stage, after.Data.Workflow[i].Stage)
		}
	}
}

var _ = idgen.UUIDv7Generator{}
