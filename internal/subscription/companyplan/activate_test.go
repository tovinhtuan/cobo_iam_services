package companyplan

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestValidPaidManualPlanCode(t *testing.T) {
	if !ValidPaidManualPlanCode(PlanCodePremium) || !ValidPaidManualPlanCode(PlanCodeEnterprise) {
		t.Fatal("paid codes")
	}
	if ValidPaidManualPlanCode(PlanCodeFree) {
		t.Fatal("FREE is not a paid activation target")
	}
}

func TestActivateImmediate_IdempotentSamePlan(t *testing.T) {
	repo := NewMemoryRepository()
	ctx := context.Background()
	now := time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC)
	prev := NowUTC
	NowUTC = func() time.Time { return now }
	defer func() { NowUTC = prev }()

	first, err := repo.ActivateImmediate(ctx, "c1", PlanCodePremium, RecordOriginPlatformAdminManual, "n1")
	if err != nil || first == nil || first.AlreadyActive || first.Plan.Code != PlanCodePremium {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	if first.Plan.ExpiresAt != nil {
		t.Fatal("manual activation must be open-ended")
	}
	second, err := repo.ActivateImmediate(ctx, "c1", PlanCodePremium, RecordOriginPlatformAdminManual, "n2")
	if err != nil || second == nil || !second.AlreadyActive {
		t.Fatalf("second must be no-op, got %+v err=%v", second, err)
	}
	if second.Plan.ID != first.Plan.ID {
		t.Fatalf("must not insert duplicate, ids %s vs %s", first.Plan.ID, second.Plan.ID)
	}
	occ, _ := repo.ListOccupyingByCompany(ctx, "c1")
	active := 0
	for _, p := range occ {
		if p.Status == PlanStatusActive && p.Covers(now) {
			active++
		}
	}
	if active != 1 {
		t.Fatalf("want 1 covering ACTIVE, occupying=%d %+v", active, occ)
	}
}

func TestActivateImmediate_ReplacesOccupyingWithoutOverlap(t *testing.T) {
	repo := NewMemoryRepository()
	ctx := context.Background()
	now := time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC)
	prev := NowUTC
	NowUTC = func() time.Time { return now }
	defer func() { NowUTC = prev }()

	if _, err := repo.ActivateImmediate(ctx, "c1", PlanCodePremium, RecordOriginPlatformAdminManual, "n1"); err != nil {
		t.Fatal(err)
	}
	out, err := repo.ActivateImmediate(ctx, "c1", PlanCodeEnterprise, RecordOriginPlatformAdminManual, "n2")
	if err != nil {
		t.Fatal(err)
	}
	if out.AlreadyActive || out.Plan.Code != PlanCodeEnterprise || out.PreviousCode != PlanCodePremium {
		t.Fatalf("got %+v", out)
	}
	got, err := repo.GetEffectivePlan(ctx, "c1", now)
	if err != nil || got == nil || got.Code != PlanCodeEnterprise || got.Status != PlanStatusActive {
		t.Fatalf("effective=%+v err=%v", got, err)
	}
	if OccupyingOverlap(mustListAll(repo, "c1"), *got, got.ID) {
		t.Fatal("new plan must not overlap remaining occupying windows")
	}
}

func TestActivateImmediate_RejectsFreeAndConcurrentSamePlan(t *testing.T) {
	repo := NewMemoryRepository()
	ctx := context.Background()
	if _, err := repo.ActivateImmediate(ctx, "c1", PlanCodeFree, RecordOriginPlatformAdminManual, "n1"); err != ErrUnsupportedManualPlan {
		t.Fatalf("want unsupported, got %v", err)
	}

	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	prev := NowUTC
	NowUTC = func() time.Time { return now }
	defer func() { NowUTC = prev }()

	var wg sync.WaitGroup
	errCh := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := repo.ActivateImmediate(ctx, "c-race", PlanCodePremium, RecordOriginPlatformAdminManual, fmt.Sprintf("id-%d", i))
			errCh <- err
		}(i)
	}
	wg.Wait()
	close(errCh)
	var fail int
	for err := range errCh {
		if err != nil {
			fail++
		}
	}
	if fail != 0 {
		t.Fatalf("memory mutex should serialize; failures=%d", fail)
	}
	occ, _ := repo.ListOccupyingByCompany(ctx, "c-race")
	covering := 0
	for _, p := range occ {
		if p.Status == PlanStatusActive && p.Covers(now) {
			covering++
		}
	}
	if covering != 1 {
		t.Fatalf("want exactly 1 covering ACTIVE, got %d %+v", covering, occ)
	}
}

func TestActivateImmediate_DoesNotTouchOtherCompany(t *testing.T) {
	repo := NewMemoryRepository()
	ctx := context.Background()
	now := time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC)
	prev := NowUTC
	NowUTC = func() time.Time { return now }
	defer func() { NowUTC = prev }()
	if _, err := repo.ActivateImmediate(ctx, "a", PlanCodePremium, RecordOriginPlatformAdminManual, "n1"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ActivateImmediate(ctx, "b", PlanCodeEnterprise, RecordOriginPlatformAdminManual, "n2"); err != nil {
		t.Fatal(err)
	}
	pa, _ := repo.GetEffectivePlan(ctx, "a", now)
	pb, _ := repo.GetEffectivePlan(ctx, "b", now)
	if pa == nil || pa.Code != PlanCodePremium || pb == nil || pb.Code != PlanCodeEnterprise {
		t.Fatalf("isolation a=%+v b=%+v", pa, pb)
	}
}

func mustListAll(repo *MemoryRepository, companyID string) []CompanyPlan {
	var out []CompanyPlan
	for _, p := range repo.rows {
		if p.CompanyID == companyID {
			out = append(out, p)
		}
	}
	return out
}
