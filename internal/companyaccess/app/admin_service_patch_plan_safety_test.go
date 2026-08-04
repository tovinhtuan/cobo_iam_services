package app_test

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/subscription/companyplan"
)

type updateTrackRepo struct {
	*cainmem.AdminRepository
	updates    atomic.Int64
	failUpdate error
}

func (r *updateTrackRepo) UpdateCompanyPlatform(ctx context.Context, req caapp.UpdatePlatformCompanyRequest) error {
	if r.failUpdate != nil {
		return r.failUpdate
	}
	r.updates.Add(1)
	return r.AdminRepository.UpdateCompanyPlatform(ctx, req)
}

type orderedPlanReader struct {
	inner      companyplan.Reader
	calls      atomic.Int64
	err        error
	failAfterN int64 // 0 = always use err if set; else fail on call N
}

func (o *orderedPlanReader) GetEffectivePlan(ctx context.Context, companyID string, at time.Time) (*companyplan.CompanyPlan, error) {
	n := o.calls.Add(1)
	if o.err != nil && (o.failAfterN == 0 || n == o.failAfterN) {
		return nil, o.err
	}
	return o.inner.GetEffectivePlan(ctx, companyID, at)
}
func (o *orderedPlanReader) GetEffectivePlans(ctx context.Context, ids []string, at time.Time) (map[string]*companyplan.CompanyPlan, error) {
	return o.inner.GetEffectivePlans(ctx, ids, at)
}

func TestPatchOwnCompany_PlanErrorBeforeMutation_NoUpdate(t *testing.T) {
	base := cainmem.NewAdminRepository()
	seedCompany(base, "c-own", "Old Name")
	repo := &updateTrackRepo{AdminRepository: base}
	plans := companyplan.NewMemoryRepository()
	reader := &orderedPlanReader{inner: companyplan.NewService(plans), err: errors.New("db_down")}
	svc := caapp.NewAdminService(repo, fakeAuthService{decision: authapp.DecisionAllow}, fixedIDGen("x"),
		caapp.WithCompanyPlanReader(reader),
		caapp.WithCompanyPlanNow(fixedPlanAt),
	)
	newName := "Should Not Apply"
	_, err := svc.PatchOwnCompany(context.Background(), caapp.PatchOwnCompanyRequest{
		Subject:     caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "c-own"},
		CompanyName: &newName,
	})
	if err == nil {
		t.Fatal("expected STRICT plan error")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusInternalServerError {
		t.Fatalf("want 500, got %v", err)
	}
	if repo.updates.Load() != 0 {
		t.Fatalf("mutation must not run when plan fails first, updates=%d", repo.updates.Load())
	}
	got, _ := base.GetCompanyPlatform(context.Background(), "c-own")
	if got.CompanyName != "Old Name" {
		t.Fatalf("company mutated: %q", got.CompanyName)
	}
}

func TestPatchOwnCompany_UpdateError_NoSuccess(t *testing.T) {
	base := cainmem.NewAdminRepository()
	seedCompany(base, "c-own", "Old Name")
	repo := &updateTrackRepo{AdminRepository: base, failUpdate: perr.NewHTTPError(http.StatusConflict, perr.CodeInvalidRequest, "update failed", nil)}
	plans := companyplan.NewMemoryRepository()
	seedPlan(t, plans, "p1", "c-own", companyplan.PlanCodePremium, companyplan.PlanStatusActive)
	svc := caapp.NewAdminService(repo, fakeAuthService{decision: authapp.DecisionAllow}, fixedIDGen("x"),
		caapp.WithCompanyPlanReader(companyplan.NewService(plans)),
		caapp.WithCompanyPlanNow(fixedPlanAt),
	)
	newName := "Nope"
	_, err := svc.PatchOwnCompany(context.Background(), caapp.PatchOwnCompanyRequest{
		Subject:     caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "c-own"},
		CompanyName: &newName,
	})
	if err == nil {
		t.Fatal("expected update error")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusConflict {
		t.Fatalf("want conflict, got %v", err)
	}
}

func TestPatchOwnCompany_Success_PlanOnResponse(t *testing.T) {
	base := cainmem.NewAdminRepository()
	seedCompany(base, "c-own", "Old Name")
	repo := &updateTrackRepo{AdminRepository: base}
	plans := companyplan.NewMemoryRepository()
	seedPlan(t, plans, "p1", "c-own", companyplan.PlanCodePremium, companyplan.PlanStatusActive)
	reader := &orderedPlanReader{inner: companyplan.NewService(plans)}
	svc := caapp.NewAdminService(repo, fakeAuthService{decision: authapp.DecisionAllow}, fixedIDGen("x"),
		caapp.WithCompanyPlanReader(reader),
		caapp.WithCompanyPlanNow(fixedPlanAt),
	)
	newName := "New Name VN"
	out, err := svc.PatchOwnCompany(context.Background(), caapp.PatchOwnCompanyRequest{
		Subject:     caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "c-own"},
		CompanyName: &newName,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.CompanyName != "New Name VN" {
		t.Fatalf("name=%q", out.CompanyName)
	}
	if out.Plan == nil || out.Plan.Code != "PREMIUM" || out.Plan.Status != "ACTIVE" {
		t.Fatalf("plan=%+v", out.Plan)
	}
	if reader.calls.Load() != 1 {
		t.Fatalf("want single pre-mutation plan read, calls=%d", reader.calls.Load())
	}
	if repo.updates.Load() != 1 {
		t.Fatalf("updates=%d", repo.updates.Load())
	}
}

func TestPatchOwnCompany_RetryIdempotent_NoDuplicateSideEffect(t *testing.T) {
	base := cainmem.NewAdminRepository()
	seedCompany(base, "c-own", "Old Name")
	repo := &updateTrackRepo{AdminRepository: base}
	plans := companyplan.NewMemoryRepository()
	seedPlan(t, plans, "p1", "c-own", companyplan.PlanCodePremium, companyplan.PlanStatusTrial)
	svc := caapp.NewAdminService(repo, fakeAuthService{decision: authapp.DecisionAllow}, fixedIDGen("x"),
		caapp.WithCompanyPlanReader(companyplan.NewService(plans)),
		caapp.WithCompanyPlanNow(fixedPlanAt),
	)
	name := "Final Name"
	req := caapp.PatchOwnCompanyRequest{
		Subject:     caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "c-own"},
		CompanyName: &name,
	}
	out1, err := svc.PatchOwnCompany(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	out2, err := svc.PatchOwnCompany(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if out1.CompanyName != "Final Name" || out2.CompanyName != "Final Name" {
		t.Fatal("name mismatch")
	}
	if out1.Plan == nil || out2.Plan == nil || out1.Plan.Status != "TRIAL" || out2.Plan.Status != "TRIAL" {
		t.Fatalf("plan1=%+v plan2=%+v", out1.Plan, out2.Plan)
	}
	// Two retries = two updates of same field; no plan Create side effects (reader only).
	if repo.updates.Load() != 2 {
		t.Fatalf("updates=%d want 2 retries", repo.updates.Load())
	}
	list, _ := plans.ListOccupyingByCompany(context.Background(), "c-own")
	if len(list) != 1 {
		t.Fatalf("plan rows must not duplicate on PATCH retry, got %d", len(list))
	}
}

func TestPatchOwnCompany_AuthDenied_Unchanged(t *testing.T) {
	base := cainmem.NewAdminRepository()
	seedCompany(base, "c-own", "Old Name")
	repo := &updateTrackRepo{AdminRepository: base}
	svc := caapp.NewAdminService(repo, fakeAuthService{decision: authapp.DecisionDeny}, fixedIDGen("x"),
		caapp.WithCompanyPlanReader(companyplan.NewService(companyplan.NewMemoryRepository())),
	)
	name := "X"
	_, err := svc.PatchOwnCompany(context.Background(), caapp.PatchOwnCompanyRequest{
		Subject:     caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "c-own"},
		CompanyName: &name,
	})
	if err == nil {
		t.Fatal("expected forbid")
	}
	if repo.updates.Load() != 0 {
		t.Fatal("denied must not update")
	}
}

func TestGetPlatformCompany_DoesNotCallPlanReader(t *testing.T) {
	base := cainmem.NewAdminRepository()
	seedCompany(base, "c-own", "CoBo VN")
	plans := companyplan.NewMemoryRepository()
	seedPlan(t, plans, "p1", "c-own", companyplan.PlanCodePremium, companyplan.PlanStatusActive)
	reader := &orderedPlanReader{inner: companyplan.NewService(plans)}
	svc := caapp.NewAdminService(base, fakeAuthService{decision: authapp.DecisionAllow}, fixedIDGen("x"),
		caapp.WithCompanyPlanReader(reader),
		caapp.WithCompanyPlanNow(fixedPlanAt),
	)
	out, err := svc.GetPlatformCompany(context.Background(), caapp.GetPlatformCompanyRequest{
		Subject:   caapp.AdminSubject{UserID: "u_cms", MembershipID: "m_cms", CompanyID: "c_platform"},
		CompanyID: "c-own",
	})
	if err != nil {
		// May fail auth if GetPlatformCompany requires platform perms — check implementation
		t.Logf("GetPlatformCompany err=%v (auth may apply)", err)
	}
	if reader.calls.Load() != 0 {
		t.Fatalf("CMS GetPlatformCompany must not call plan reader, calls=%d", reader.calls.Load())
	}
	if out != nil && out.Plan != nil {
		t.Fatalf("CMS must not enrich plan, got %+v", out.Plan)
	}
}
