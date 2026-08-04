package companyplan

import (
	"context"
	"testing"
	"time"
)

func TestDisplayNameAndWireSource(t *testing.T) {
	if PlanCodePremium.DisplayName() != "Premium" {
		t.Fatalf("display=%q", PlanCodePremium.DisplayName())
	}
	p := CompanyPlan{Code: PlanCodePremium, Status: PlanStatusActive}
	if p.WireSource() != PlanSourceCompanySubscription {
		t.Fatalf("source=%q", p.WireSource())
	}
}

func TestValidEnums(t *testing.T) {
	if !ValidPlanCode(PlanCodePremium) || ValidPlanCode("GOLD") {
		t.Fatal("plan code validation")
	}
	if !ValidPlanStatus(PlanStatusTrial) || ValidPlanStatus("PENDING") {
		t.Fatal("plan status validation")
	}
}

func TestWindowsOverlap(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mid := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	// Half-open [Jan,Jun) and [Jun,Dec) touch at Jun → no overlap.
	if WindowsOverlap(from, &mid, mid, &end) {
		t.Fatal("touching half-open windows must not overlap")
	}
	later := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	if !WindowsOverlap(from, &end, mid, &later) {
		t.Fatal("expected overlap")
	}
	if !WindowsOverlap(from, nil, mid, &end) {
		t.Fatal("open-ended should overlap finite")
	}
}

func TestOccupyingOverlapIgnoresExpired(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	existing := []CompanyPlan{{
		ID: "old", CompanyID: "c1", Status: PlanStatusExpired,
		EffectiveFrom: from, ExpiresAt: &end,
	}}
	cand := CompanyPlan{
		ID: "new", CompanyID: "c1", Status: PlanStatusActive,
		EffectiveFrom: from, ExpiresAt: nil,
	}
	if OccupyingOverlap(existing, cand, "") {
		t.Fatal("expired must not occupy")
	}
}

func TestSelectEffectivePlan(t *testing.T) {
	at := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	exp := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	pastExp := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	rows := []CompanyPlan{
		{ID: "a", CompanyID: "c1", Code: PlanCodePremium, Status: PlanStatusExpired, EffectiveFrom: from, ExpiresAt: &pastExp},
		{ID: "b", CompanyID: "c1", Code: PlanCodePremium, Status: PlanStatusActive, EffectiveFrom: from, ExpiresAt: &exp},
		{ID: "c", CompanyID: "c1", Code: PlanCodePremium, Status: PlanStatusTrial, EffectiveFrom: from, ExpiresAt: &exp},
	}
	got := SelectEffectivePlan(rows, at)
	if got == nil || got.ID != "b" {
		t.Fatalf("want ACTIVE id=b, got %+v", got)
	}
	if SelectEffectivePlan(rows, time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)) != nil {
		t.Fatal("expected nil outside window")
	}
}

func TestMemoryRepository_CreateOverlapAndRead(t *testing.T) {
	repo := NewMemoryRepository()
	ctx := context.Background()
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	premium := CompanyPlan{
		ID: "p1", CompanyID: "c_001", Code: PlanCodePremium, Status: PlanStatusActive,
		EffectiveFrom: from, Origin: RecordOriginDevFixture,
	}
	if err := repo.Create(ctx, premium); err != nil {
		t.Fatal(err)
	}
	dup := premium
	dup.ID = "p2"
	if err := repo.Create(ctx, dup); err != ErrOverlap {
		t.Fatalf("want ErrOverlap, got %v", err)
	}
	at := time.Date(2026, 8, 4, 0, 0, 0, 0, time.UTC)
	got, err := repo.GetEffectivePlan(ctx, "c_001", at)
	if err != nil || got == nil || got.Code != PlanCodePremium || got.Status != PlanStatusActive {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	if got.WireSource() != PlanSourceCompanySubscription {
		t.Fatal("wire source")
	}
	// No record → nil
	none, err := repo.GetEffectivePlan(ctx, "c_002", at)
	if err != nil || none != nil {
		t.Fatalf("want nil plan for c_002, got %+v err=%v", none, err)
	}
	// TRIAL returned as-is (reader does not badge-filter)
	_ = repo.DeleteByIDs(ctx, []string{"p1"})
	trial := CompanyPlan{
		ID: "t1", CompanyID: "c_001", Code: PlanCodePremium, Status: PlanStatusTrial,
		EffectiveFrom: from, Origin: RecordOriginDevFixture,
	}
	if err := repo.Create(ctx, trial); err != nil {
		t.Fatal(err)
	}
	got, err = repo.GetEffectivePlan(ctx, "c_001", at)
	if err != nil || got == nil || got.Status != PlanStatusTrial {
		t.Fatalf("want TRIAL returned, got %+v", got)
	}
}

func TestMemoryRepository_BatchAndCleanup(t *testing.T) {
	repo := NewMemoryRepository()
	ctx := context.Background()
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	_ = repo.Create(ctx, CompanyPlan{
		ID: "p1", CompanyID: "c_001", Code: PlanCodePremium, Status: PlanStatusActive,
		EffectiveFrom: from, Origin: RecordOriginDevFixture,
	})
	_ = repo.Create(ctx, CompanyPlan{
		ID: "p2", CompanyID: "c_003", Code: PlanCodeEnterprise, Status: PlanStatusSuspended,
		EffectiveFrom: from, Origin: RecordOriginDevFixture,
	})
	at := time.Date(2026, 8, 4, 0, 0, 0, 0, time.UTC)
	m, err := repo.GetEffectivePlans(ctx, []string{"c_001", "c_002", "c_003"}, at)
	if err != nil {
		t.Fatal(err)
	}
	if m["c_001"] == nil || m["c_002"] != nil || m["c_003"] == nil {
		t.Fatalf("batch map=%v", m)
	}
	if m["c_003"].Status != PlanStatusSuspended {
		t.Fatal("suspended must be returned")
	}
	if err := repo.DeleteByOrigin(ctx, RecordOriginDevFixture); err != nil {
		t.Fatal(err)
	}
	m, _ = repo.GetEffectivePlans(ctx, []string{"c_001", "c_003"}, at)
	if len(m) != 0 {
		t.Fatalf("cleanup failed: %v", m)
	}
}

func TestValidateCreate(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	badEnd := from
	if err := ValidateCreate(CompanyPlan{
		ID: "x", CompanyID: "c", Code: PlanCodePremium, Status: PlanStatusActive,
		EffectiveFrom: from, ExpiresAt: &badEnd, Origin: RecordOriginManual,
	}); err != ErrInvalidPlan {
		t.Fatalf("want invalid for non-after expires, got %v", err)
	}
}
