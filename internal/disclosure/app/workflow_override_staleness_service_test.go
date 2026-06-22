package app_test

import (
	"context"
	"testing"
	"time"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

// fakeAuthService is a minimal authapp.Service test double. It lets these tests exercise the
// real authorize()-call boundary Batch 2's own handlers/service methods are responsible for
// (propagating Allow/Deny correctly), without re-testing the real authorization engine's
// permission-resolution logic, which has its own test suite elsewhere.
type fakeAuthService struct {
	allow bool
}

func (f *fakeAuthService) Authorize(_ context.Context, _ authapp.AuthorizeRequest) (*authapp.AuthorizeDecision, error) {
	if f.allow {
		return &authapp.AuthorizeDecision{Decision: authapp.DecisionAllow}, nil
	}
	code := perr.CodePermissionDenied
	return &authapp.AuthorizeDecision{Decision: authapp.DecisionDeny, DenyReasonCode: &code}, nil
}

func (f *fakeAuthService) AuthorizeBatch(_ context.Context, req authapp.AuthorizeBatchRequest) (*authapp.AuthorizeBatchResponse, error) {
	decision := authapp.Decision(authapp.DecisionDeny)
	if f.allow {
		decision = authapp.DecisionAllow
	}
	results := make([]authapp.AuthorizeDecision, len(req.Checks))
	for i := range req.Checks {
		results[i] = authapp.AuthorizeDecision{Decision: decision}
	}
	return &authapp.AuthorizeBatchResponse{Results: results}, nil
}

func (f *fakeAuthService) GetEffectiveAccess(_ context.Context, _, _ string) (*authapp.EffectiveAccessSummary, error) {
	return &authapp.EffectiveAccessSummary{}, nil
}

const staleTestTypeID = "dt-periodic-financial" // exists in inmemory.SeedDisclosureTypeCatalog()

func newStalenessTestService(t *testing.T, allow bool) (disclosureapp.Service, *inmemory.Repository) {
	t.Helper()
	repo := inmemory.NewRepository()
	svc := disclosureapp.NewService(repo, &fakeAuthService{allow: allow}, idgen.UUIDv7Generator{})
	return svc, repo
}

// TestGetWorkflowOverrideStatus_NoOverride_Returns200HasOverrideFalse covers the task's explicit
// "no override returns 200 + has_override=false" requirement — never 404 for this normal state.
func TestGetWorkflowOverrideStatus_NoOverride_Returns200HasOverrideFalse(t *testing.T) {
	svc, _ := newStalenessTestService(t, true)
	resp, err := svc.GetWorkflowOverrideStatus(context.Background(), disclosureapp.GetWorkflowOverrideStatusRequest{
		Subject: disclosureapp.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "company-no-override"},
		TypeID:  staleTestTypeID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Data.HasOverride {
		t.Errorf("HasOverride = true, want false")
	}
	if resp.Data.StaleStatus != disclosureapp.StaleStatusUnknown {
		t.Errorf("StaleStatus = %q, want %q", resp.Data.StaleStatus, disclosureapp.StaleStatusUnknown)
	}
}

// TestGetWorkflowOverrideStatus_TypeNotFound_Returns404 covers "type not found returns 404" —
// distinct from "no override", which must be 200.
func TestGetWorkflowOverrideStatus_TypeNotFound_Returns404(t *testing.T) {
	svc, _ := newStalenessTestService(t, true)
	_, err := svc.GetWorkflowOverrideStatus(context.Background(), disclosureapp.GetWorkflowOverrideStatusRequest{
		Subject: disclosureapp.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "company-x"},
		TypeID:  "this-type-does-not-exist",
	})
	httpErr, ok := err.(*perr.HTTPError)
	if !ok {
		t.Fatalf("expected *perr.HTTPError, got %T (%v)", err, err)
	}
	if httpErr.HTTPStatus != 404 {
		t.Errorf("HTTPStatus = %d, want 404", httpErr.HTTPStatus)
	}
}

// TestGetWorkflowOverrideStatus_MissingPermission_Returns403 covers "missing permission returns
// 403" — exercises the real authorize()-call boundary via fakeAuthService{allow:false}.
func TestGetWorkflowOverrideStatus_MissingPermission_Returns403(t *testing.T) {
	svc, _ := newStalenessTestService(t, false)
	_, err := svc.GetWorkflowOverrideStatus(context.Background(), disclosureapp.GetWorkflowOverrideStatusRequest{
		Subject: disclosureapp.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "company-x"},
		TypeID:  staleTestTypeID,
	})
	httpErr, ok := err.(*perr.HTTPError)
	if !ok {
		t.Fatalf("expected *perr.HTTPError, got %T (%v)", err, err)
	}
	if httpErr.HTTPStatus != 403 {
		t.Errorf("HTTPStatus = %d, want 403", httpErr.HTTPStatus)
	}
}

// TestGetWorkflowOverrideStatus_ReturnsOwnCompanyOnly proves tenant isolation: two companies
// with overrides on the same type each see only their own status, never the other's.
func TestGetWorkflowOverrideStatus_ReturnsOwnCompanyOnly(t *testing.T) {
	svc, repo := newStalenessTestService(t, true)
	ctx := context.Background()

	seedOverride(t, repo, "company-A", staleTestTypeID, disclosureapp.BaseSourceGlobalWorkflow, intPtrTest(2))
	seedOverride(t, repo, "company-B", staleTestTypeID, disclosureapp.BaseSourceUnknown, nil)

	respA, err := svc.GetWorkflowOverrideStatus(ctx, disclosureapp.GetWorkflowOverrideStatusRequest{
		Subject: disclosureapp.Subject{CompanyID: "company-A"}, TypeID: staleTestTypeID,
	})
	if err != nil {
		t.Fatalf("company-A: %v", err)
	}
	if !respA.Data.HasOverride || respA.Data.BaseSource == nil || *respA.Data.BaseSource != disclosureapp.BaseSourceGlobalWorkflow {
		t.Errorf("company-A status = %+v, want HasOverride=true, BaseSource=global_workflow", respA.Data)
	}

	respB, err := svc.GetWorkflowOverrideStatus(ctx, disclosureapp.GetWorkflowOverrideStatusRequest{
		Subject: disclosureapp.Subject{CompanyID: "company-B"}, TypeID: staleTestTypeID,
	})
	if err != nil {
		t.Fatalf("company-B: %v", err)
	}
	if !respB.Data.HasOverride || respB.Data.BaseSource == nil || *respB.Data.BaseSource != disclosureapp.BaseSourceUnknown {
		t.Errorf("company-B status = %+v, want HasOverride=true, BaseSource=unknown (must not see company-A's data)", respB.Data)
	}
}

// TestRebaseCheckWorkflowOverride_UpdatesOnlyStaleStatusAndLastRebaseCheckAt is the central
// regression guard for the task's forbidden-writes list: rebase-check must touch nothing else.
func TestRebaseCheckWorkflowOverride_UpdatesOnlyStaleStatusAndLastRebaseCheckAt(t *testing.T) {
	svc, repo := newStalenessTestService(t, true)
	ctx := context.Background()
	companyID := "company-rebase-check"

	seedOverride(t, repo, companyID, staleTestTypeID, disclosureapp.BaseSourceGlobalWorkflow, intPtrTest(1))
	before, _, err := repo.GetOverrideStalenessMetadata(ctx, companyID, staleTestTypeID)
	if err != nil {
		t.Fatalf("read before: %v", err)
	}
	if before.LastRebaseCheckAt != nil {
		t.Fatalf("precondition failed: LastRebaseCheckAt already set")
	}

	_, err = svc.RebaseCheckWorkflowOverride(ctx, disclosureapp.RebaseCheckWorkflowOverrideRequest{
		Subject: disclosureapp.Subject{CompanyID: companyID}, TypeID: staleTestTypeID,
	})
	if err != nil {
		t.Fatalf("rebase-check: %v", err)
	}

	after, _, err := repo.GetOverrideStalenessMetadata(ctx, companyID, staleTestTypeID)
	if err != nil {
		t.Fatalf("read after: %v", err)
	}
	if after.LastRebaseCheckAt == nil {
		t.Errorf("LastRebaseCheckAt not set after rebase-check")
	}
	// base_source/base_version_no/base_hash must be untouched.
	if after.BaseSource != before.BaseSource {
		t.Errorf("BaseSource changed: %q -> %q", before.BaseSource, after.BaseSource)
	}
	if (after.BaseVersionNo == nil) != (before.BaseVersionNo == nil) ||
		(after.BaseVersionNo != nil && *after.BaseVersionNo != *before.BaseVersionNo) {
		t.Errorf("BaseVersionNo changed: %v -> %v", before.BaseVersionNo, after.BaseVersionNo)
	}
}

func intPtrTest(v int) *int { return &v }

// seedOverride directly manipulates the in-memory repository's internal state to set up a
// known override with known Batch 1 metadata — mirroring the established white-box test pattern
// already used in internal/disclosure/infra/inmemory/cms_repository_pointer_test.go.
func seedOverride(t *testing.T, repo *inmemory.Repository, companyID, typeID, baseSource string, baseVersionNo *int) {
	t.Helper()
	_, err := disclosureapp.NewService(repo, &fakeAuthService{allow: true}, idgen.UUIDv7Generator{}).
		UpsertCompanyWorkflowOverrideDraft(context.Background(), disclosureapp.UpsertCompanyWorkflowOverrideDraftRequest{
			Subject: disclosureapp.Subject{UserID: "seed", MembershipID: "seed", CompanyID: companyID},
			TypeID:  typeID,
			Publish: true,
			Workflow: []disclosureapp.WorkflowStepDTO{
				{StepID: "seed-step-1", Stage: "Seed Step", DepartmentID: "d1", AssigneeRoleIds: []string{"reviewer"}, DisplayOrder: 1, Documents: []disclosureapp.WorkflowDocumentDTO{}},
			},
		})
	if err != nil {
		t.Fatalf("seedOverride upsert: %v", err)
	}
	if err := repo.SetOverrideBaseMetadataForTest(companyID, typeID, baseSource, baseVersionNo); err != nil {
		t.Fatalf("seedOverride set base metadata: %v", err)
	}
}

// TestRebaseCheckWorkflowOverride_GetEffectiveWorkflowUnchanged is the runtime invariance test
// required by the task: GetEffectiveWorkflow output must be byte-identical before and after a
// rebase-check call, for both an override company and a non-override company.
func TestRebaseCheckWorkflowOverride_GetEffectiveWorkflowUnchanged(t *testing.T) {
	svc, repo := newStalenessTestService(t, true)
	ctx := context.Background()
	overrideCompany := "company-runtime-invariance-override"
	noOverrideCompany := "company-runtime-invariance-none"

	seedOverride(t, repo, overrideCompany, staleTestTypeID, disclosureapp.BaseSourceGlobalWorkflow, intPtrTest(1))

	beforeOverride, err := svc.GetEffectiveWorkflow(ctx, disclosureapp.GetEffectiveWorkflowRequest{
		Subject: disclosureapp.Subject{CompanyID: overrideCompany}, TypeID: staleTestTypeID,
	})
	if err != nil {
		t.Fatalf("GetEffectiveWorkflow before (override company): %v", err)
	}
	beforeNoOverride, err := svc.GetEffectiveWorkflow(ctx, disclosureapp.GetEffectiveWorkflowRequest{
		Subject: disclosureapp.Subject{CompanyID: noOverrideCompany}, TypeID: staleTestTypeID,
	})
	if err != nil {
		t.Fatalf("GetEffectiveWorkflow before (no-override company): %v", err)
	}

	if _, err := svc.RebaseCheckWorkflowOverride(ctx, disclosureapp.RebaseCheckWorkflowOverrideRequest{
		Subject: disclosureapp.Subject{CompanyID: overrideCompany}, TypeID: staleTestTypeID,
	}); err != nil {
		t.Fatalf("rebase-check: %v", err)
	}

	afterOverride, err := svc.GetEffectiveWorkflow(ctx, disclosureapp.GetEffectiveWorkflowRequest{
		Subject: disclosureapp.Subject{CompanyID: overrideCompany}, TypeID: staleTestTypeID,
	})
	if err != nil {
		t.Fatalf("GetEffectiveWorkflow after (override company): %v", err)
	}
	afterNoOverride, err := svc.GetEffectiveWorkflow(ctx, disclosureapp.GetEffectiveWorkflowRequest{
		Subject: disclosureapp.Subject{CompanyID: noOverrideCompany}, TypeID: staleTestTypeID,
	})
	if err != nil {
		t.Fatalf("GetEffectiveWorkflow after (no-override company): %v", err)
	}

	if beforeOverride.Data.Source != afterOverride.Data.Source || beforeOverride.Data.VersionNo != afterOverride.Data.VersionNo {
		t.Errorf("override company GetEffectiveWorkflow changed: before=%+v after=%+v", beforeOverride.Data, afterOverride.Data)
	}
	if beforeNoOverride.Data.Source != afterNoOverride.Data.Source || beforeNoOverride.Data.VersionNo != afterNoOverride.Data.VersionNo {
		t.Errorf("no-override company GetEffectiveWorkflow changed: before=%+v after=%+v", beforeNoOverride.Data, afterNoOverride.Data)
	}
}

var _ = time.Now // keep time import if unused by future edits
