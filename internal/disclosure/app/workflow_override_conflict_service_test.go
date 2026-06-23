package app_test

import (
	"context"
	"testing"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// ── Repository-level tests (against the in-memory repository directly) ─────────────────────────

func TestUpsertWorkflowOverrideConflicts_InsertThenDedupe(t *testing.T) {
	repo := inmemory.NewRepository()
	ctx := context.Background()
	input := disclosureapp.PersistedConflictInput{
		PreviewConflict: disclosureapp.PreviewConflict{
			StepKey: "review", FieldPath: "due_rule", Severity: disclosureapp.ConflictSeverityAdvisory,
			ConflictType: disclosureapp.ConflictTypeSameFieldChanged, GlobalOld: "T+3", GlobalNew: "T+5", CompanyValue: "T+9",
			ResolutionOptions: []string{disclosureapp.ResolutionKeepCompany},
		},
		CompanyID: "company-A", TypeID: "type-1", BaseVersionNo: 2, TargetVersionNo: 3, CreatedBy: "tester",
	}

	first, err := repo.UpsertWorkflowOverrideConflicts(ctx, []disclosureapp.PersistedConflictInput{input})
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("expected 1 row, got %d", len(first))
	}
	if first[0].ResolutionStatus != disclosureapp.ResolutionStatusUnresolved {
		t.Errorf("ResolutionStatus = %q, want %q", first[0].ResolutionStatus, disclosureapp.ResolutionStatusUnresolved)
	}
	firstID := first[0].ID

	// Re-detecting the SAME conflict (identical key fields) must upsert the SAME row, not create
	// a second one — Option B idempotency (PREFLIGHT_AUDIT.md §8).
	second, err := repo.UpsertWorkflowOverrideConflicts(ctx, []disclosureapp.PersistedConflictInput{input})
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if len(second) != 1 || second[0].ID != firstID {
		t.Fatalf("expected the SAME conflict id on re-detection, got first=%q second=%+v", firstID, second)
	}
}

func TestUpsertWorkflowOverrideConflicts_PreservesResolutionAcrossRedetection(t *testing.T) {
	repo := inmemory.NewRepository()
	ctx := context.Background()
	input := disclosureapp.PersistedConflictInput{
		PreviewConflict: disclosureapp.PreviewConflict{
			StepKey: "review", FieldPath: "due_rule", Severity: disclosureapp.ConflictSeverityAdvisory,
			ConflictType: disclosureapp.ConflictTypeSameFieldChanged, ResolutionOptions: []string{disclosureapp.ResolutionKeepCompany},
		},
		CompanyID: "company-A", TypeID: "type-1", BaseVersionNo: 2, TargetVersionNo: 3, CreatedBy: "tester",
	}
	rows, err := repo.UpsertWorkflowOverrideConflicts(ctx, []disclosureapp.PersistedConflictInput{input})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	conflictID := rows[0].ID

	resolved, err := repo.ResolveWorkflowOverrideConflict(ctx, "company-A", "type-1", conflictID, disclosureapp.ResolutionKeepCompany, nil, "admin-1", time.Now())
	if err != nil || resolved == nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved.ResolutionStatus != disclosureapp.ResolutionStatusResolved {
		t.Fatalf("expected resolved status, got %+v", resolved)
	}

	// Re-detect the SAME conflict (e.g. a second preview call) — must NOT reset resolution_status.
	again, err := repo.UpsertWorkflowOverrideConflicts(ctx, []disclosureapp.PersistedConflictInput{input})
	if err != nil {
		t.Fatalf("re-upsert: %v", err)
	}
	if again[0].ResolutionStatus != disclosureapp.ResolutionStatusResolved {
		t.Errorf("re-detection must preserve resolution_status=resolved, got %q", again[0].ResolutionStatus)
	}
}

func TestGetWorkflowOverrideConflict_CrossCompanyAccessReturnsNil(t *testing.T) {
	repo := inmemory.NewRepository()
	ctx := context.Background()
	input := disclosureapp.PersistedConflictInput{
		PreviewConflict: disclosureapp.PreviewConflict{StepKey: "s", FieldPath: "f", Severity: disclosureapp.ConflictSeverityAdvisory, ConflictType: disclosureapp.ConflictTypeSameFieldChanged},
		CompanyID:       "company-A", TypeID: "type-1", BaseVersionNo: 1, TargetVersionNo: 2, CreatedBy: "tester",
	}
	rows, err := repo.UpsertWorkflowOverrideConflicts(ctx, []disclosureapp.PersistedConflictInput{input})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	conflictID := rows[0].ID

	got, err := repo.GetWorkflowOverrideConflict(ctx, "company-B", "type-1", conflictID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("cross-company access must return nil, got %+v", got)
	}
}

// ── Service/handler-level tests ─────────────────────────────────────────────────────────────────

func TestResolveWorkflowOverrideConflict_InvalidResolution_Returns400(t *testing.T) {
	svc, repo := newStalenessTestService(t, true)
	companyID := "company-invalid-resolution"
	seedOverride(t, repo, companyID, staleTestTypeID, disclosureapp.BaseSourceGlobalWorkflow, intPtrTest(2))
	rows, err := repo.UpsertWorkflowOverrideConflicts(context.Background(), []disclosureapp.PersistedConflictInput{{
		PreviewConflict: disclosureapp.PreviewConflict{StepKey: "s", FieldPath: "f", Severity: disclosureapp.ConflictSeverityAdvisory, ConflictType: disclosureapp.ConflictTypeSameFieldChanged},
		CompanyID:       companyID, TypeID: staleTestTypeID, BaseVersionNo: 1, TargetVersionNo: 2, CreatedBy: "tester",
	}})
	if err != nil {
		t.Fatalf("seed conflict: %v", err)
	}

	_, err = svc.ResolveWorkflowOverrideConflict(context.Background(), disclosureapp.ResolveWorkflowOverrideConflictRequest{
		Subject: disclosureapp.Subject{CompanyID: companyID}, TypeID: staleTestTypeID, ConflictID: rows[0].ID, Resolution: "not_a_real_resolution",
	})
	httpErr, ok := err.(*perr.HTTPError)
	if !ok {
		t.Fatalf("expected *perr.HTTPError, got %T (%v)", err, err)
	}
	if httpErr.HTTPStatus != 400 {
		t.Errorf("HTTPStatus = %d, want 400", httpErr.HTTPStatus)
	}
	if httpErr.Code != perr.CodeInvalidResolution {
		t.Errorf("Code = %q, want %q", httpErr.Code, perr.CodeInvalidResolution)
	}
}

func TestResolveWorkflowOverrideConflict_MissingPermission_Returns403(t *testing.T) {
	svc, repo := newStalenessTestService(t, false)
	companyID := "company-forbidden-resolve"
	rows, err := repo.UpsertWorkflowOverrideConflicts(context.Background(), []disclosureapp.PersistedConflictInput{{
		PreviewConflict: disclosureapp.PreviewConflict{StepKey: "s", FieldPath: "f", Severity: disclosureapp.ConflictSeverityAdvisory, ConflictType: disclosureapp.ConflictTypeSameFieldChanged},
		CompanyID:       companyID, TypeID: staleTestTypeID, BaseVersionNo: 1, TargetVersionNo: 2, CreatedBy: "tester",
	}})
	if err != nil {
		t.Fatalf("seed conflict: %v", err)
	}

	_, err = svc.ResolveWorkflowOverrideConflict(context.Background(), disclosureapp.ResolveWorkflowOverrideConflictRequest{
		Subject: disclosureapp.Subject{CompanyID: companyID}, TypeID: staleTestTypeID, ConflictID: rows[0].ID, Resolution: disclosureapp.ResolutionKeepCompany,
	})
	httpErr, ok := err.(*perr.HTTPError)
	if !ok {
		t.Fatalf("expected *perr.HTTPError, got %T (%v)", err, err)
	}
	if httpErr.HTTPStatus != 403 {
		t.Errorf("HTTPStatus = %d, want 403", httpErr.HTTPStatus)
	}
}

func TestResolveWorkflowOverrideConflict_WrongCompany_Returns404(t *testing.T) {
	svc, repo := newStalenessTestService(t, true)
	rows, err := repo.UpsertWorkflowOverrideConflicts(context.Background(), []disclosureapp.PersistedConflictInput{{
		PreviewConflict: disclosureapp.PreviewConflict{StepKey: "s", FieldPath: "f", Severity: disclosureapp.ConflictSeverityAdvisory, ConflictType: disclosureapp.ConflictTypeSameFieldChanged},
		CompanyID:       "company-owner", TypeID: staleTestTypeID, BaseVersionNo: 1, TargetVersionNo: 2, CreatedBy: "tester",
	}})
	if err != nil {
		t.Fatalf("seed conflict: %v", err)
	}

	_, err = svc.ResolveWorkflowOverrideConflict(context.Background(), disclosureapp.ResolveWorkflowOverrideConflictRequest{
		Subject: disclosureapp.Subject{CompanyID: "company-attacker"}, TypeID: staleTestTypeID, ConflictID: rows[0].ID, Resolution: disclosureapp.ResolutionKeepCompany,
	})
	httpErr, ok := err.(*perr.HTTPError)
	if !ok {
		t.Fatalf("expected *perr.HTTPError, got %T (%v)", err, err)
	}
	if httpErr.HTTPStatus != 404 {
		t.Errorf("HTTPStatus = %d, want 404 (cross-company access must look identical to not-found)", httpErr.HTTPStatus)
	}
}

func TestResolveWorkflowOverrideConflict_SameResolutionTwice_IsIdempotentNoOp(t *testing.T) {
	svc, repo := newStalenessTestService(t, true)
	companyID := "company-idempotent-resolve"
	rows, err := repo.UpsertWorkflowOverrideConflicts(context.Background(), []disclosureapp.PersistedConflictInput{{
		PreviewConflict: disclosureapp.PreviewConflict{StepKey: "s", FieldPath: "f", Severity: disclosureapp.ConflictSeverityAdvisory, ConflictType: disclosureapp.ConflictTypeSameFieldChanged},
		CompanyID:       companyID, TypeID: staleTestTypeID, BaseVersionNo: 1, TargetVersionNo: 2, CreatedBy: "tester",
	}})
	if err != nil {
		t.Fatalf("seed conflict: %v", err)
	}
	ctx := context.Background()
	req := disclosureapp.ResolveWorkflowOverrideConflictRequest{
		Subject: disclosureapp.Subject{CompanyID: companyID}, TypeID: staleTestTypeID, ConflictID: rows[0].ID, Resolution: disclosureapp.ResolutionKeepCompany,
	}
	if _, err := svc.ResolveWorkflowOverrideConflict(ctx, req); err != nil {
		t.Fatalf("first resolve: %v", err)
	}
	resp2, err := svc.ResolveWorkflowOverrideConflict(ctx, req)
	if err != nil {
		t.Fatalf("second resolve (same resolution) must be a no-op success, got error: %v", err)
	}
	if resp2.Data.ResolutionStatus != disclosureapp.ResolutionStatusResolved {
		t.Errorf("expected resolved status on idempotent re-resolve, got %+v", resp2.Data)
	}
}

func TestResolveWorkflowOverrideConflict_DifferentResolutionAfterResolved_Returns409(t *testing.T) {
	svc, repo := newStalenessTestService(t, true)
	companyID := "company-conflicting-resolve"
	rows, err := repo.UpsertWorkflowOverrideConflicts(context.Background(), []disclosureapp.PersistedConflictInput{{
		PreviewConflict: disclosureapp.PreviewConflict{StepKey: "s", FieldPath: "f", Severity: disclosureapp.ConflictSeverityAdvisory, ConflictType: disclosureapp.ConflictTypeSameFieldChanged},
		CompanyID:       companyID, TypeID: staleTestTypeID, BaseVersionNo: 1, TargetVersionNo: 2, CreatedBy: "tester",
	}})
	if err != nil {
		t.Fatalf("seed conflict: %v", err)
	}
	ctx := context.Background()
	if _, err := svc.ResolveWorkflowOverrideConflict(ctx, disclosureapp.ResolveWorkflowOverrideConflictRequest{
		Subject: disclosureapp.Subject{CompanyID: companyID}, TypeID: staleTestTypeID, ConflictID: rows[0].ID, Resolution: disclosureapp.ResolutionKeepCompany,
	}); err != nil {
		t.Fatalf("first resolve: %v", err)
	}

	_, err = svc.ResolveWorkflowOverrideConflict(ctx, disclosureapp.ResolveWorkflowOverrideConflictRequest{
		Subject: disclosureapp.Subject{CompanyID: companyID}, TypeID: staleTestTypeID, ConflictID: rows[0].ID, Resolution: disclosureapp.ResolutionAcceptGlobal,
	})
	httpErr, ok := err.(*perr.HTTPError)
	if !ok {
		t.Fatalf("expected *perr.HTTPError, got %T (%v)", err, err)
	}
	if httpErr.HTTPStatus != 409 {
		t.Errorf("HTTPStatus = %d, want 409", httpErr.HTTPStatus)
	}
	if httpErr.Code != perr.CodeConflictAlreadyResolved {
		t.Errorf("Code = %q, want %q", httpErr.Code, perr.CodeConflictAlreadyResolved)
	}
}

func TestResolveWorkflowOverrideConflict_NotFound_Returns404(t *testing.T) {
	svc, _ := newStalenessTestService(t, true)
	_, err := svc.ResolveWorkflowOverrideConflict(context.Background(), disclosureapp.ResolveWorkflowOverrideConflictRequest{
		Subject: disclosureapp.Subject{CompanyID: "company-x"}, TypeID: staleTestTypeID, ConflictID: "does-not-exist", Resolution: disclosureapp.ResolutionKeepCompany,
	})
	httpErr, ok := err.(*perr.HTTPError)
	if !ok {
		t.Fatalf("expected *perr.HTTPError, got %T (%v)", err, err)
	}
	if httpErr.HTTPStatus != 404 {
		t.Errorf("HTTPStatus = %d, want 404", httpErr.HTTPStatus)
	}
}

// TestGetWorkflowOverrideRebasePreview_PersistsConflictsWithDurableIDs proves the preview
// endpoint, when it detects a conflict, persists it and returns a durable id usable by resolve.
func TestGetWorkflowOverrideRebasePreview_PersistsConflictsWithDurableIDs(t *testing.T) {
	svc, repo := newStalenessTestService(t, true)
	companyID := "company-conflict-preview"
	seedOverride(t, repo, companyID, staleTestTypeID, disclosureapp.BaseSourceGlobalWorkflow, intPtrTest(2))
	if err := repo.SetGlobalActiveVersionForTest(staleTestTypeID, 5); err != nil {
		t.Fatalf("set global active version: %v", err)
	}
	// Base step's department differs from target's -> rule 1 conflict if company also customized
	// department differently. seedOverride's fixture step has DepartmentID="d1"; make base/target
	// disagree with the company's value.
	repo.SetGlobalWorkflowVersionManifestForTest(staleTestTypeID, 2, []disclosureapp.GlobalWorkflowStepInput{
		{StepKey: "stepkey-1", StepID: "seed-step-1", Stage: "Seed Step", DepartmentID: "d-base", AssigneeRoleIds: []string{"reviewer"}, DisplayOrder: 1},
	})
	repo.SetGlobalWorkflowVersionManifestForTest(staleTestTypeID, 5, []disclosureapp.GlobalWorkflowStepInput{
		{StepKey: "stepkey-1", StepID: "seed-step-1", Stage: "Seed Step", DepartmentID: "d-target", AssigneeRoleIds: []string{"reviewer"}, DisplayOrder: 1},
	})
	// seedOverride's own fixture step has DepartmentID="d1" — different from both base AND
	// target, so this IS a genuine three-way disagreement (rule 1).

	resp, err := svc.GetWorkflowOverrideRebasePreview(context.Background(), disclosureapp.GetWorkflowOverrideRebasePreviewRequest{
		Subject: disclosureapp.Subject{CompanyID: companyID}, TypeID: staleTestTypeID,
	})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if len(resp.Data.Conflicts) == 0 {
		t.Fatalf("expected at least 1 persisted conflict, got none: %+v", resp.Data)
	}
	conflictID := resp.Data.Conflicts[0].ID
	if conflictID == "" {
		t.Fatalf("expected a non-empty durable conflict id")
	}

	// The id must actually be resolvable via the resolve endpoint — proving it's a REAL durable
	// id, not just a label.
	_, err = svc.ResolveWorkflowOverrideConflict(context.Background(), disclosureapp.ResolveWorkflowOverrideConflictRequest{
		Subject: disclosureapp.Subject{CompanyID: companyID}, TypeID: staleTestTypeID, ConflictID: conflictID, Resolution: disclosureapp.ResolutionKeepCompany,
	})
	if err != nil {
		t.Fatalf("resolving the conflict returned by preview failed: %v", err)
	}
}

// TestGetWorkflowOverrideRebasePreview_RuntimeInvarianceWithConflictPersistence re-proves Batch
// 3's runtime invariance test still holds now that preview ALSO persists conflicts.
func TestGetWorkflowOverrideRebasePreview_RuntimeInvarianceWithConflictPersistence(t *testing.T) {
	svc, repo := newStalenessTestService(t, true)
	companyID := "company-runtime-invariance-conflicts"
	seedOverride(t, repo, companyID, staleTestTypeID, disclosureapp.BaseSourceGlobalWorkflow, intPtrTest(2))
	if err := repo.SetGlobalActiveVersionForTest(staleTestTypeID, 5); err != nil {
		t.Fatalf("set global active version: %v", err)
	}
	repo.SetGlobalWorkflowVersionManifestForTest(staleTestTypeID, 2, []disclosureapp.GlobalWorkflowStepInput{
		{StepKey: "stepkey-1", StepID: "seed-step-1", Stage: "Seed Step", DepartmentID: "d-base", AssigneeRoleIds: []string{"reviewer"}, DisplayOrder: 1},
	})
	repo.SetGlobalWorkflowVersionManifestForTest(staleTestTypeID, 5, []disclosureapp.GlobalWorkflowStepInput{
		{StepKey: "stepkey-1", StepID: "seed-step-1", Stage: "Seed Step", DepartmentID: "d-target", AssigneeRoleIds: []string{"reviewer"}, DisplayOrder: 1},
	})

	ctx := context.Background()
	before, err := svc.GetEffectiveWorkflow(ctx, disclosureapp.GetEffectiveWorkflowRequest{
		Subject: disclosureapp.Subject{CompanyID: companyID}, TypeID: staleTestTypeID,
	})
	if err != nil {
		t.Fatalf("GetEffectiveWorkflow before: %v", err)
	}

	resp, err := svc.GetWorkflowOverrideRebasePreview(ctx, disclosureapp.GetWorkflowOverrideRebasePreviewRequest{
		Subject: disclosureapp.Subject{CompanyID: companyID}, TypeID: staleTestTypeID,
	})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if len(resp.Data.Conflicts) == 0 {
		t.Fatalf("expected at least 1 persisted conflict for this invariance test to be meaningful")
	}

	if _, err := svc.ResolveWorkflowOverrideConflict(ctx, disclosureapp.ResolveWorkflowOverrideConflictRequest{
		Subject: disclosureapp.Subject{CompanyID: companyID}, TypeID: staleTestTypeID, ConflictID: resp.Data.Conflicts[0].ID, Resolution: disclosureapp.ResolutionKeepCompany,
	}); err != nil {
		t.Fatalf("resolve: %v", err)
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
	for i := range before.Data.Workflow {
		if before.Data.Workflow[i].Stage != after.Data.Workflow[i].Stage || before.Data.Workflow[i].DepartmentID != after.Data.Workflow[i].DepartmentID {
			t.Errorf("step %d changed: before=%+v after=%+v", i, before.Data.Workflow[i], after.Data.Workflow[i])
		}
	}
}
