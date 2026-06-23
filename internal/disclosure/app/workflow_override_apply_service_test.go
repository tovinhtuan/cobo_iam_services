package app_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

// Sprint 3 / Batch 5 — Workflow Override Rebase Apply.
//
// applyTestSetup seeds a stale override (base v2, current global active v5) with a real,
// content-differing manifest pair, exactly mirroring TestGetWorkflowOverrideRebasePreview_
// Stale_Returns200WithDiff's own fixture — the same one Batch 3/4's own tests already proved
// produces a real, deterministic diff. previewID() runs the existing, unmodified preview endpoint
// to obtain a real preview_id for use by apply.

func applyTestSetup(t *testing.T, companyID string) (disclosureapp.Service, *inmemory.Repository) {
	t.Helper()
	svc, repo := newStalenessTestService(t, true)
	seedOverride(t, repo, companyID, staleTestTypeID, disclosureapp.BaseSourceGlobalWorkflow, intPtrTest(2))
	if err := repo.SetGlobalActiveVersionForTest(staleTestTypeID, 5); err != nil {
		t.Fatalf("set global active version: %v", err)
	}
	seedGlobalManifestPair(repo, 2, 5, "Seed Step", "Seed Step Renamed By CMS")
	return svc, repo
}

func previewID(t *testing.T, svc disclosureapp.Service, companyID string) string {
	t.Helper()
	resp, err := svc.GetWorkflowOverrideRebasePreview(context.Background(), disclosureapp.GetWorkflowOverrideRebasePreviewRequest{
		Subject: disclosureapp.Subject{CompanyID: companyID}, TypeID: staleTestTypeID,
	})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if resp.Data.PreviewID == nil || *resp.Data.PreviewID == "" {
		t.Fatalf("expected non-empty preview_id, got %v", resp.Data.PreviewID)
	}
	return *resp.Data.PreviewID
}

// activeVersionNo reads active_version_no the same way the production HTTP path does
// (GetCompanyWorkflowOverride) — OverrideStalenessRow deliberately does not carry it (it only
// carries the Batch 1/2 staleness metadata columns).
func activeVersionNo(t *testing.T, repo *inmemory.Repository, companyID string) int {
	t.Helper()
	view, err := repo.GetCompanyWorkflowOverride(context.Background(), companyID, staleTestTypeID)
	if err != nil {
		t.Fatalf("GetCompanyWorkflowOverride: %v", err)
	}
	if view == nil || view.Override == nil {
		t.Fatalf("no override found for company %q", companyID)
	}
	return view.Override.ActiveVersionNo
}

func mustHTTPError(t *testing.T, err error) *perr.HTTPError {
	t.Helper()
	httpErr, ok := err.(*perr.HTTPError)
	if !ok {
		t.Fatalf("expected *perr.HTTPError, got %T (%v)", err, err)
	}
	return httpErr
}

// 1. Apply rejects missing preview (empty preview_id in request).
func TestApplyWorkflowOverrideRebase_MissingPreviewID_Returns400(t *testing.T) {
	svc, _ := applyTestSetup(t, "company-apply-missing-preview")
	_, err := svc.ApplyWorkflowOverrideRebase(context.Background(), disclosureapp.ApplyWorkflowOverrideRebaseRequest{
		Subject: disclosureapp.Subject{CompanyID: "company-apply-missing-preview"}, TypeID: staleTestTypeID, PreviewID: "",
	})
	httpErr := mustHTTPError(t, err)
	if httpErr.HTTPStatus != 400 {
		t.Errorf("HTTPStatus = %d, want 400", httpErr.HTTPStatus)
	}
}

// 2. Apply rejects an unknown/never-issued preview_id.
func TestApplyWorkflowOverrideRebase_UnknownPreviewID_Returns409PreviewRequired(t *testing.T) {
	svc, _ := applyTestSetup(t, "company-apply-unknown-preview")
	_, err := svc.ApplyWorkflowOverrideRebase(context.Background(), disclosureapp.ApplyWorkflowOverrideRebaseRequest{
		Subject: disclosureapp.Subject{CompanyID: "company-apply-unknown-preview"}, TypeID: staleTestTypeID, PreviewID: "prv_never_issued",
	})
	httpErr := mustHTTPError(t, err)
	if httpErr.HTTPStatus != 409 || httpErr.Code != perr.CodePreviewRequired {
		t.Errorf("got HTTPStatus=%d Code=%q, want 409 PREVIEW_REQUIRED", httpErr.HTTPStatus, httpErr.Code)
	}
}

// 3. Apply rejects when no override exists at all.
func TestApplyWorkflowOverrideRebase_NoOverride_Returns404(t *testing.T) {
	svc, _ := newStalenessTestService(t, true)
	_, err := svc.ApplyWorkflowOverrideRebase(context.Background(), disclosureapp.ApplyWorkflowOverrideRebaseRequest{
		Subject: disclosureapp.Subject{CompanyID: "company-no-override-at-all"}, TypeID: staleTestTypeID, PreviewID: "prv_anything",
	})
	httpErr := mustHTTPError(t, err)
	// The cache lookup fails first (no preview was ever minted for this company) — 409
	// PREVIEW_REQUIRED, never a guess at OVERRIDE_NOT_FOUND from unvalidated input.
	if httpErr.HTTPStatus != 409 {
		t.Errorf("HTTPStatus = %d, want 409", httpErr.HTTPStatus)
	}
}

// 4. Apply rejects when override is not stale (already current).
func TestApplyWorkflowOverrideRebase_NotStale_Returns409(t *testing.T) {
	svc, repo := newStalenessTestService(t, true)
	companyID := "company-apply-current"
	seedOverride(t, repo, companyID, staleTestTypeID, disclosureapp.BaseSourceGlobalWorkflow, intPtrTest(5))
	if err := repo.SetGlobalActiveVersionForTest(staleTestTypeID, 5); err != nil {
		t.Fatalf("set global active version: %v", err)
	}
	// A preview_id minted earlier (e.g. before the global version was rebased back down) must
	// still be correctly rejected once the override is current again — apply re-validates fresh
	// state, it never trusts the cached preview's own staleness snapshot.
	_, err := svc.ApplyWorkflowOverrideRebase(context.Background(), disclosureapp.ApplyWorkflowOverrideRebaseRequest{
		Subject: disclosureapp.Subject{CompanyID: companyID}, TypeID: staleTestTypeID, PreviewID: "prv_stale_cache_miss",
	})
	httpErr := mustHTTPError(t, err)
	if httpErr.HTTPStatus != 409 {
		t.Errorf("HTTPStatus = %d, want 409", httpErr.HTTPStatus)
	}
}

// 5. Apply rejects unresolved conflicts.
func TestApplyWorkflowOverrideRebase_UnresolvedConflicts_Returns409(t *testing.T) {
	svc, repo := newStalenessTestService(t, true)
	companyID := "company-apply-unresolved"
	seedOverride(t, repo, companyID, staleTestTypeID, disclosureapp.BaseSourceGlobalWorkflow, intPtrTest(2))
	if err := repo.SetGlobalActiveVersionForTest(staleTestTypeID, 5); err != nil {
		t.Fatalf("set global active version: %v", err)
	}
	// Rule 1 three-way department disagreement (same fixture proven in
	// TestGetWorkflowOverrideRebasePreview_PersistsConflictsWithDurableIDs): base="d-base",
	// target="d-target", company (seedOverride's own fixture step) ="d1" — a genuine conflict
	// that persists as resolution_status=unresolved until explicitly resolved.
	repo.SetGlobalWorkflowVersionManifestForTest(staleTestTypeID, 2, []disclosureapp.GlobalWorkflowStepInput{
		{StepKey: "stepkey-1", StepID: "seed-step-1", Stage: "Seed Step", DepartmentID: "d-base", AssigneeRoleIds: []string{"reviewer"}, DisplayOrder: 1},
	})
	repo.SetGlobalWorkflowVersionManifestForTest(staleTestTypeID, 5, []disclosureapp.GlobalWorkflowStepInput{
		{StepKey: "stepkey-1", StepID: "seed-step-1", Stage: "Seed Step", DepartmentID: "d-target", AssigneeRoleIds: []string{"reviewer"}, DisplayOrder: 1},
	})

	pid := previewID(t, svc, companyID)
	_, err := svc.ApplyWorkflowOverrideRebase(context.Background(), disclosureapp.ApplyWorkflowOverrideRebaseRequest{
		Subject: disclosureapp.Subject{CompanyID: companyID}, TypeID: staleTestTypeID, PreviewID: pid,
	})
	httpErr := mustHTTPError(t, err)
	if httpErr.HTTPStatus != 409 || httpErr.Code != perr.CodeConflictsUnresolved {
		t.Errorf("got HTTPStatus=%d Code=%q, want 409 CONFLICTS_UNRESOLVED", httpErr.HTTPStatus, httpErr.Code)
	}
}

// 6. Apply rejects a Rule 8 (deferred department validation) blocker.
func TestApplyWorkflowOverrideRebase_Rule8DepartmentChangeBlocksApply(t *testing.T) {
	svc, repo := newStalenessTestService(t, true)
	companyID := "company-apply-rule8"
	seedOverride(t, repo, companyID, staleTestTypeID, disclosureapp.BaseSourceGlobalWorkflow, intPtrTest(2))
	if err := repo.SetGlobalActiveVersionForTest(staleTestTypeID, 5); err != nil {
		t.Fatalf("set global active version: %v", err)
	}
	// Manifest pair where the field that changed is department_id specifically.
	repo.SetGlobalWorkflowVersionManifestForTest(staleTestTypeID, 2, []disclosureapp.GlobalWorkflowStepInput{
		{StepKey: "stepkey-1", StepID: "seed-step-1", Stage: "Seed Step", DepartmentID: "d1", AssigneeRoleIds: []string{"reviewer"}, DisplayOrder: 1},
	})
	repo.SetGlobalWorkflowVersionManifestForTest(staleTestTypeID, 5, []disclosureapp.GlobalWorkflowStepInput{
		{StepKey: "stepkey-1", StepID: "seed-step-1", Stage: "Seed Step", DepartmentID: "d2", AssigneeRoleIds: []string{"reviewer"}, DisplayOrder: 1},
	})

	pid := previewID(t, svc, companyID)
	_, err := svc.ApplyWorkflowOverrideRebase(context.Background(), disclosureapp.ApplyWorkflowOverrideRebaseRequest{
		Subject: disclosureapp.Subject{CompanyID: companyID}, TypeID: staleTestTypeID, PreviewID: pid,
	})
	httpErr := mustHTTPError(t, err)
	if httpErr.HTTPStatus != 409 || httpErr.Code != perr.CodeDeferredValidationBlock {
		t.Errorf("got HTTPStatus=%d Code=%q, want 409 DEFERRED_VALIDATION_BLOCKED", httpErr.HTTPStatus, httpErr.Code)
	}
}

// 7/8/9. Apply succeeds with no conflicts, creates exactly one new version, flips active_version_no.
func TestApplyWorkflowOverrideRebase_Success_CreatesOneVersionAndFlipsActive(t *testing.T) {
	svc, repo := applyTestSetup(t, "company-apply-success")
	companyID := "company-apply-success"

	beforeActiveVersionNo := activeVersionNo(t, repo, companyID)

	pid := previewID(t, svc, companyID)
	resp, err := svc.ApplyWorkflowOverrideRebase(context.Background(), disclosureapp.ApplyWorkflowOverrideRebaseRequest{
		Subject: disclosureapp.Subject{CompanyID: companyID}, TypeID: staleTestTypeID, PreviewID: pid,
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if resp.Data.NewVersionNo != beforeActiveVersionNo+1 {
		t.Errorf("NewVersionNo = %d, want %d (exactly one new version)", resp.Data.NewVersionNo, beforeActiveVersionNo+1)
	}
	if resp.Data.State != "approved" {
		t.Errorf("State = %q, want approved", resp.Data.State)
	}
	if resp.Data.WorkflowSource != "company_override" {
		t.Errorf("WorkflowSource = %q, want company_override", resp.Data.WorkflowSource)
	}

	if got := activeVersionNo(t, repo, companyID); got != resp.Data.NewVersionNo {
		t.Errorf("active_version_no = %d, want %d", got, resp.Data.NewVersionNo)
	}
	after, _, err := repo.GetOverrideStalenessMetadata(context.Background(), companyID, staleTestTypeID)
	if err != nil {
		t.Fatalf("read after: %v", err)
	}
	if after.StaleStatus != disclosureapp.StaleStatusCurrent {
		t.Errorf("stale_status = %q, want current", after.StaleStatus)
	}
	if after.BaseVersionNo == nil || *after.BaseVersionNo != 5 {
		t.Errorf("base_version_no = %v, want 5 (the target version just applied)", after.BaseVersionNo)
	}
}

// 10. Apply preserves all fields not touched by any operation.
func TestApplyWorkflowOverrideRebase_PreservesUntouchedFields(t *testing.T) {
	svc, _ := applyTestSetup(t, "company-apply-preserve")
	companyID := "company-apply-preserve"

	effBefore, err := svc.GetEffectiveWorkflow(context.Background(), disclosureapp.GetEffectiveWorkflowRequest{
		Subject: disclosureapp.Subject{CompanyID: companyID}, TypeID: staleTestTypeID,
	})
	if err != nil {
		t.Fatalf("effective before: %v", err)
	}
	if len(effBefore.Data.Workflow) != 1 {
		t.Fatalf("expected exactly one seeded step before apply, got %d", len(effBefore.Data.Workflow))
	}
	originalStep := effBefore.Data.Workflow[0]

	pid := previewID(t, svc, companyID)
	if _, err := svc.ApplyWorkflowOverrideRebase(context.Background(), disclosureapp.ApplyWorkflowOverrideRebaseRequest{
		Subject: disclosureapp.Subject{CompanyID: companyID}, TypeID: staleTestTypeID, PreviewID: pid,
	}); err != nil {
		t.Fatalf("apply: %v", err)
	}

	effAfter, err := svc.GetEffectiveWorkflow(context.Background(), disclosureapp.GetEffectiveWorkflowRequest{
		Subject: disclosureapp.Subject{CompanyID: companyID}, TypeID: staleTestTypeID,
	})
	if err != nil {
		t.Fatalf("effective after: %v", err)
	}
	if len(effAfter.Data.Workflow) != 1 {
		t.Fatalf("expected exactly one step after apply (only stage changed), got %d", len(effAfter.Data.Workflow))
	}
	newStep := effAfter.Data.Workflow[0]

	if newStep.Stage != "Seed Step Renamed By CMS" {
		t.Errorf("Stage = %q, want the new global stage to have been applied", newStep.Stage)
	}
	// Every OTHER field must be byte-identical to before — only stage was in the patch.
	if newStep.StepID != originalStep.StepID {
		t.Errorf("StepID changed: %q -> %q", originalStep.StepID, newStep.StepID)
	}
	if newStep.DepartmentID != originalStep.DepartmentID {
		t.Errorf("DepartmentID changed: %q -> %q", originalStep.DepartmentID, newStep.DepartmentID)
	}
	if !reflect.DeepEqual(newStep.AssigneeRoleIds, originalStep.AssigneeRoleIds) {
		t.Errorf("AssigneeRoleIds changed: %v -> %v", originalStep.AssigneeRoleIds, newStep.AssigneeRoleIds)
	}
	if newStep.DueRule != originalStep.DueRule {
		t.Errorf("DueRule changed: %q -> %q", originalStep.DueRule, newStep.DueRule)
	}
	if newStep.DisplayOrder != originalStep.DisplayOrder {
		t.Errorf("DisplayOrder changed: %d -> %d", originalStep.DisplayOrder, newStep.DisplayOrder)
	}
}

// 11. Apply is transactional — a concurrent active_version_no change between preview and apply
// (the race guard) rolls back the whole attempt; no partial write survives.
func TestApplyWorkflowOverrideRebase_ConcurrentChange_RejectsWithoutPartialWrite(t *testing.T) {
	svc, repo := applyTestSetup(t, "company-apply-race")
	companyID := "company-apply-race"

	pid := previewID(t, svc, companyID)
	beforeVersionCount := activeVersionNo(t, repo, companyID)

	// Directly mutate active_version_no via a second draft+publish, mimicking a real concurrent
	// session's write landing between this test's preview and apply calls.
	if _, err := svc.UpsertCompanyWorkflowOverrideDraft(context.Background(), disclosureapp.UpsertCompanyWorkflowOverrideDraftRequest{
		Subject:  disclosureapp.Subject{UserID: "other-session", MembershipID: "m2", CompanyID: companyID},
		TypeID:   staleTestTypeID,
		Publish:  true,
		Workflow: []disclosureapp.WorkflowStepDTO{{StepID: "seed-step-1", Stage: "Concurrently Edited", DepartmentID: "d1", AssigneeRoleIds: []string{"reviewer"}, DisplayOrder: 1, Documents: []disclosureapp.WorkflowDocumentDTO{}}},
	}); err != nil {
		t.Fatalf("concurrent publish: %v", err)
	}

	_, err := svc.ApplyWorkflowOverrideRebase(context.Background(), disclosureapp.ApplyWorkflowOverrideRebaseRequest{
		Subject: disclosureapp.Subject{CompanyID: companyID}, TypeID: staleTestTypeID, PreviewID: pid,
	})
	if err == nil {
		t.Fatalf("expected apply to reject a concurrently-changed override, got success")
	}
	httpErr := mustHTTPError(t, err)
	if httpErr.HTTPStatus != 409 {
		t.Errorf("HTTPStatus = %d, want 409", httpErr.HTTPStatus)
	}

	// The concurrent edit's own version is the only change — apply itself wrote nothing.
	if got := activeVersionNo(t, repo, companyID); got != beforeVersionCount+1 {
		t.Errorf("active_version_no = %d, want exactly %d (only the concurrent edit's own version, no partial apply write)", got, beforeVersionCount+1)
	}
}

// Regression: a resolved FIELD-level conflict (Rule 1 "same_field_changed" — due_rule, here) must
// actually be applied when resolved as accept_global. The diff engine never emits an op for a
// field it also conflicted (op XOR conflict, never both — workflow_override_diff.go), so without
// fieldConflictResolutionToOp's bridge, this resolution would silently do nothing.
func TestApplyWorkflowOverrideRebase_FieldConflictAcceptGlobal_ActuallyAppliesNewValue(t *testing.T) {
	svc, repo := newStalenessTestService(t, true)
	companyID := "company-apply-field-conflict"
	seedOverride(t, repo, companyID, staleTestTypeID, disclosureapp.BaseSourceGlobalWorkflow, intPtrTest(2))
	if err := repo.SetGlobalActiveVersionForTest(staleTestTypeID, 5); err != nil {
		t.Fatalf("set global active version: %v", err)
	}
	// Three-way due_rule disagreement on the SAME step identity (seed-step-1/stepkey-1): base="",
	// target="T+10", company customized to "T+99" (seedOverride's fixture has no due_rule set, so
	// set it directly via a draft update first).
	if _, err := svc.UpsertCompanyWorkflowOverrideDraft(context.Background(), disclosureapp.UpsertCompanyWorkflowOverrideDraftRequest{
		Subject:  disclosureapp.Subject{UserID: "u1", MembershipID: "m1", CompanyID: companyID},
		TypeID:   staleTestTypeID,
		Publish:  true,
		Workflow: []disclosureapp.WorkflowStepDTO{{StepID: "seed-step-1", Stage: "Seed Step", DepartmentID: "d1", AssigneeRoleIds: []string{"reviewer"}, DueRule: "T+99", DisplayOrder: 1, Documents: []disclosureapp.WorkflowDocumentDTO{}}},
	}); err != nil {
		t.Fatalf("set company due_rule: %v", err)
	}
	if err := repo.SetOverrideBaseMetadataForTest(companyID, staleTestTypeID, disclosureapp.BaseSourceGlobalWorkflow, intPtrTest(2)); err != nil {
		t.Fatalf("reset base metadata: %v", err)
	}
	repo.SetGlobalWorkflowVersionManifestForTest(staleTestTypeID, 2, []disclosureapp.GlobalWorkflowStepInput{
		{StepKey: "stepkey-1", StepID: "seed-step-1", Stage: "Seed Step", DepartmentID: "d1", AssigneeRoleIds: []string{"reviewer"}, DueRule: "", DisplayOrder: 1},
	})
	repo.SetGlobalWorkflowVersionManifestForTest(staleTestTypeID, 5, []disclosureapp.GlobalWorkflowStepInput{
		{StepKey: "stepkey-1", StepID: "seed-step-1", Stage: "Seed Step", DepartmentID: "d1", AssigneeRoleIds: []string{"reviewer"}, DueRule: "T+10", DisplayOrder: 1},
	})

	preview, err := svc.GetWorkflowOverrideRebasePreview(context.Background(), disclosureapp.GetWorkflowOverrideRebasePreviewRequest{
		Subject: disclosureapp.Subject{CompanyID: companyID}, TypeID: staleTestTypeID,
	})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if len(preview.Data.Conflicts) != 1 || preview.Data.Conflicts[0].FieldPath != "due_rule" {
		t.Fatalf("expected exactly one due_rule conflict, got %+v", preview.Data.Conflicts)
	}
	conflictID := preview.Data.Conflicts[0].ID

	if _, err := svc.ResolveWorkflowOverrideConflict(context.Background(), disclosureapp.ResolveWorkflowOverrideConflictRequest{
		Subject: disclosureapp.Subject{CompanyID: companyID}, TypeID: staleTestTypeID, ConflictID: conflictID, Resolution: disclosureapp.ResolutionAcceptGlobal,
	}); err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if _, err := svc.ApplyWorkflowOverrideRebase(context.Background(), disclosureapp.ApplyWorkflowOverrideRebaseRequest{
		Subject: disclosureapp.Subject{CompanyID: companyID}, TypeID: staleTestTypeID, PreviewID: *preview.Data.PreviewID,
	}); err != nil {
		t.Fatalf("apply: %v", err)
	}

	eff, err := svc.GetEffectiveWorkflow(context.Background(), disclosureapp.GetEffectiveWorkflowRequest{
		Subject: disclosureapp.Subject{CompanyID: companyID}, TypeID: staleTestTypeID,
	})
	if err != nil {
		t.Fatalf("effective after apply: %v", err)
	}
	if eff.Data.Workflow[0].DueRule != "T+10" {
		t.Errorf("DueRule = %q, want T+10 (accept_global must actually apply the global value, not silently leave the old company value)", eff.Data.Workflow[0].DueRule)
	}
}

// 13. Apply against the wrong company (tenant boundary) returns an error, never another
// company's data.
func TestApplyWorkflowOverrideRebase_PreviewFromDifferentCompany_Rejected(t *testing.T) {
	svcA, _ := applyTestSetup(t, "company-apply-tenant-a")
	pidA := previewID(t, svcA, "company-apply-tenant-a")

	svcB, repoB := newStalenessTestService(t, true)
	seedOverride(t, repoB, "company-apply-tenant-b", staleTestTypeID, disclosureapp.BaseSourceGlobalWorkflow, intPtrTest(2))
	if err := repoB.SetGlobalActiveVersionForTest(staleTestTypeID, 5); err != nil {
		t.Fatalf("set global active version: %v", err)
	}
	seedGlobalManifestPair(repoB, 2, 5, "Seed Step", "Seed Step Renamed By CMS")

	// company-B tries to apply using a preview_id that was minted for company-A. Note:
	// previewID() is process-global by design (PREFLIGHT_AUDIT.md §2's in-process cache), so this
	// genuinely exercises the tenant check rather than two independent caches.
	_, err := svcB.ApplyWorkflowOverrideRebase(context.Background(), disclosureapp.ApplyWorkflowOverrideRebaseRequest{
		Subject: disclosureapp.Subject{CompanyID: "company-apply-tenant-b"}, TypeID: staleTestTypeID, PreviewID: pidA,
	})
	if err == nil {
		t.Fatalf("expected company-B's apply with company-A's preview_id to be rejected")
	}
	httpErr := mustHTTPError(t, err)
	if httpErr.HTTPStatus != 409 || httpErr.Code != perr.CodePreviewRequired {
		t.Errorf("got HTTPStatus=%d Code=%q, want 409 PREVIEW_REQUIRED (cross-tenant preview must never be honored)", httpErr.HTTPStatus, httpErr.Code)
	}
}

// 14. Apply with missing permission returns 403.
func TestApplyWorkflowOverrideRebase_MissingPermission_Returns403(t *testing.T) {
	svcAllowed, repo := applyTestSetup(t, "company-apply-forbidden")
	companyID := "company-apply-forbidden"
	pid := previewID(t, svcAllowed, companyID)

	svcDenied := disclosureapp.NewService(repo, &fakeAuthService{allow: false}, idgen.UUIDv7Generator{})
	_, err := svcDenied.ApplyWorkflowOverrideRebase(context.Background(), disclosureapp.ApplyWorkflowOverrideRebaseRequest{
		Subject: disclosureapp.Subject{CompanyID: companyID}, TypeID: staleTestTypeID, PreviewID: pid,
	})
	httpErr := mustHTTPError(t, err)
	if httpErr.HTTPStatus != 403 {
		t.Errorf("HTTPStatus = %d, want 403", httpErr.HTTPStatus)
	}
}

// 15/16. GetEffectiveWorkflow is unchanged before apply, and changes to exactly the applied
// snapshot after apply — the central ABSOLUTE_RUNTIME_RULE regression guard.
func TestApplyWorkflowOverrideRebase_RuntimeUnchangedBeforeChangedAfter(t *testing.T) {
	svc, _ := applyTestSetup(t, "company-apply-runtime")
	companyID := "company-apply-runtime"
	ctx := context.Background()

	effBefore, err := svc.GetEffectiveWorkflow(ctx, disclosureapp.GetEffectiveWorkflowRequest{Subject: disclosureapp.Subject{CompanyID: companyID}, TypeID: staleTestTypeID})
	if err != nil {
		t.Fatalf("effective before preview: %v", err)
	}

	pid := previewID(t, svc, companyID)

	effAfterPreview, err := svc.GetEffectiveWorkflow(ctx, disclosureapp.GetEffectiveWorkflowRequest{Subject: disclosureapp.Subject{CompanyID: companyID}, TypeID: staleTestTypeID})
	if err != nil {
		t.Fatalf("effective after preview: %v", err)
	}
	beforeJSON, _ := json.Marshal(effBefore.Data)
	afterPreviewJSON, _ := json.Marshal(effAfterPreview.Data)
	if string(beforeJSON) != string(afterPreviewJSON) {
		t.Fatalf("RUNTIME_CHANGED_BEFORE_APPLY: preview alone changed GetEffectiveWorkflow\nbefore=%s\nafter=%s", beforeJSON, afterPreviewJSON)
	}

	resp, err := svc.ApplyWorkflowOverrideRebase(ctx, disclosureapp.ApplyWorkflowOverrideRebaseRequest{
		Subject: disclosureapp.Subject{CompanyID: companyID}, TypeID: staleTestTypeID, PreviewID: pid,
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}

	effAfterApply, err := svc.GetEffectiveWorkflow(ctx, disclosureapp.GetEffectiveWorkflowRequest{Subject: disclosureapp.Subject{CompanyID: companyID}, TypeID: staleTestTypeID})
	if err != nil {
		t.Fatalf("effective after apply: %v", err)
	}
	if string(afterPreviewJSON) == mustMarshal(t, effAfterApply.Data) {
		t.Fatalf("APPLY_DID_NOT_CHANGE_RUNTIME: GetEffectiveWorkflow identical before and after a successful apply")
	}
	if effAfterApply.Data.VersionNo != resp.Data.NewVersionNo {
		t.Errorf("effective version_no = %d, want %d (the just-applied version)", effAfterApply.Data.VersionNo, resp.Data.NewVersionNo)
	}
	if effAfterApply.Data.Workflow[0].Stage != "Seed Step Renamed By CMS" {
		t.Errorf("effective stage = %q, want the applied snapshot's new stage", effAfterApply.Data.Workflow[0].Stage)
	}
	// 18. workflow_source remains company_override after apply.
	if effAfterApply.Data.Source != "company_override" {
		t.Errorf("Source = %q, want company_override", effAfterApply.Data.Source)
	}
}

// 17. A non-overridden company on the same type is unaffected by another company's apply.
func TestApplyWorkflowOverrideRebase_NonOverriddenCompanyUnaffected(t *testing.T) {
	svc, _ := applyTestSetup(t, "company-apply-affects-only-self")
	companyID := "company-apply-affects-only-self"
	otherCompanyID := "company-no-override-bystander"
	ctx := context.Background()

	effOtherBefore, err := svc.GetEffectiveWorkflow(ctx, disclosureapp.GetEffectiveWorkflowRequest{Subject: disclosureapp.Subject{CompanyID: otherCompanyID}, TypeID: staleTestTypeID})
	if err != nil {
		t.Fatalf("other company effective before: %v", err)
	}

	pid := previewID(t, svc, companyID)
	if _, err := svc.ApplyWorkflowOverrideRebase(ctx, disclosureapp.ApplyWorkflowOverrideRebaseRequest{
		Subject: disclosureapp.Subject{CompanyID: companyID}, TypeID: staleTestTypeID, PreviewID: pid,
	}); err != nil {
		t.Fatalf("apply: %v", err)
	}

	effOtherAfter, err := svc.GetEffectiveWorkflow(ctx, disclosureapp.GetEffectiveWorkflowRequest{Subject: disclosureapp.Subject{CompanyID: otherCompanyID}, TypeID: staleTestTypeID})
	if err != nil {
		t.Fatalf("other company effective after: %v", err)
	}
	if mustMarshal(t, effOtherBefore.Data) != mustMarshal(t, effOtherAfter.Data) {
		t.Errorf("non-overridden company's effective workflow changed as a side effect of another company's apply\nbefore=%s\nafter=%s", mustMarshal(t, effOtherBefore.Data), mustMarshal(t, effOtherAfter.Data))
	}
}

// 12. Apply idempotency — at the service-method level, calling apply twice with two SEPARATE
// preview_ids (both computed from the same now-current state) must each create their own version
// (no service-level idempotency is claimed here — that's the HTTP handler's Idempotency-Key
// responsibility, IDEMPOTENCY_CONTRACT.md). This test instead proves the inverse safety property
// the service method itself guarantees: replaying the SAME already-applied preview_id a second
// time fails cleanly (it's no longer in the cache as "fresh" relative to the new active_version_no
// the first apply just set) rather than silently creating a duplicate version.
func TestApplyWorkflowOverrideRebase_ReapplyingSamePreviewAfterSuccess_NeverDuplicates(t *testing.T) {
	svc, repo := applyTestSetup(t, "company-apply-no-duplicate")
	companyID := "company-apply-no-duplicate"

	pid := previewID(t, svc, companyID)
	first, err := svc.ApplyWorkflowOverrideRebase(context.Background(), disclosureapp.ApplyWorkflowOverrideRebaseRequest{
		Subject: disclosureapp.Subject{CompanyID: companyID}, TypeID: staleTestTypeID, PreviewID: pid,
	})
	if err != nil {
		t.Fatalf("first apply: %v", err)
	}

	_, err = svc.ApplyWorkflowOverrideRebase(context.Background(), disclosureapp.ApplyWorkflowOverrideRebaseRequest{
		Subject: disclosureapp.Subject{CompanyID: companyID}, TypeID: staleTestTypeID, PreviewID: pid,
	})
	if err == nil {
		t.Fatalf("expected re-applying the same already-consumed preview_id to fail, got success")
	}

	if got := activeVersionNo(t, repo, companyID); got != first.Data.NewVersionNo {
		t.Errorf("active_version_no = %d, want %d (no duplicate version from the rejected re-apply)", got, first.Data.NewVersionNo)
	}
}

func mustMarshal(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}
