package app_test

import (
	"context"
	"testing"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

// ---------------------------------------------------------------------------
// Helpers — sticky upsert store mirrors ON DUPLICATE KEY UPDATE cycle_id=cycle_id
// ---------------------------------------------------------------------------

type futurePregenRepo struct {
	*inmemory.Repository
	types     []disclosureapp.PeriodicTypeRow
	companies []string
	prefs     []disclosureapp.CompanyTypePreference
	captured  []disclosureapp.PeriodicCycleRow
	store     map[string]disclosureapp.PeriodicCycleRow
}

func (r *futurePregenRepo) ListActivePeriodicTypes(_ context.Context) ([]disclosureapp.PeriodicTypeRow, error) {
	return r.types, nil
}

func (r *futurePregenRepo) ListAllActiveCompanyIDs(_ context.Context) ([]string, error) {
	return r.companies, nil
}

func (r *futurePregenRepo) ListCompanyTypePreferencesByTypeIDs(_ context.Context, _ []string) ([]disclosureapp.CompanyTypePreference, error) {
	return r.prefs, nil
}

func (r *futurePregenRepo) UpsertPeriodicCycle(_ context.Context, row disclosureapp.PeriodicCycleRow) error {
	r.captured = append(r.captured, row)
	if r.store == nil {
		r.store = map[string]disclosureapp.PeriodicCycleRow{}
	}
	key := row.CompanyID + "|" + row.TypeID + "|" + row.CycleLabel
	if _, ok := r.store[key]; ok {
		return nil // sticky
	}
	r.store[key] = row
	return nil
}

func newFuturePregenSvc(repo *futurePregenRepo) disclosureapp.Service {
	return disclosureapp.NewService(repo, nil, idgen.UUIDv7Generator{}, disclosureapp.WithHolidayCalendarProvider(noHolidaysProvider{}))
}

func storeLabels(repo *futurePregenRepo) []string {
	out := make([]string, 0, len(repo.store))
	for _, row := range repo.store {
		out = append(out, row.CycleLabel)
	}
	return out
}

func TestGenerateAtDate(t *testing.T) {
	loc := time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)
	tt := time.Date(2026, 10, 5, 15, 30, 0, 0, loc)
	got := disclosureapp.GenerateAtDate(tt, 10)
	want := time.Date(2026, 9, 25, 0, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("GenerateAtDate=%v want %v", got, want)
	}
	if !disclosureapp.GenerateAtDate(tt, 0).Equal(time.Date(2026, 10, 5, 0, 0, 0, 0, loc)) {
		t.Fatal("lead 0 should yield T date-only")
	}
}

func TestValidatePeriodicCycleGenerationLeadDays(t *testing.T) {
	if err := disclosureapp.ValidatePeriodicCycleGenerationLeadDays(nil); err != nil {
		t.Fatalf("nil: %v", err)
	}
	zero := 0
	if err := disclosureapp.ValidatePeriodicCycleGenerationLeadDays(&zero); err != nil {
		t.Fatalf("0: %v", err)
	}
	ok := 90
	if err := disclosureapp.ValidatePeriodicCycleGenerationLeadDays(&ok); err != nil {
		t.Fatalf("90: %v", err)
	}
	neg := -1
	if err := disclosureapp.ValidatePeriodicCycleGenerationLeadDays(&neg); err == nil {
		t.Fatal("want reject negative")
	}
	over := 91
	err := disclosureapp.ValidatePeriodicCycleGenerationLeadDays(&over)
	if err == nil {
		t.Fatal("want reject >90")
	}
	he, okHE := err.(*perr.HTTPError)
	if !okHE || he.HTTPStatus != 400 {
		t.Fatalf("want 400 HTTPError, got %#v", err)
	}
}

func TestResolveNextApplicableLogicalSlot_AFJumpAndAT(t *testing.T) {
	loc := time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)
	anchor := disclosureapp.AnchorConfig{Day: 5}

	got, err := disclosureapp.ResolveNextApplicableLogicalSlot(
		"monthly", "2026-10",
		disclosureapp.ApplicableFromModeSpecific, "2026-11", "",
		anchor, loc,
	)
	if err != nil || got != "2026-11" {
		t.Fatalf("AF jump want 2026-11 got %q err=%v", got, err)
	}

	got, err = disclosureapp.ResolveNextApplicableLogicalSlot(
		"monthly", "2026-09",
		"", "", "2026-09-30",
		anchor, loc,
	)
	if err != nil || got != "" {
		t.Fatalf("AT none want empty got %q err=%v", got, err)
	}
}

func TestFuturePregen_NullOrZero_NoFutureSeed(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	repo := &futurePregenRepo{
		Repository: inmemory.NewRepository(),
		types: []disclosureapp.PeriodicTypeRow{{
			TypeID: "t-zero", FrequencyUnit: "monthly", CycleAnchorDay: 5, DeadlineDays: 10,
			PeriodicCycleGenerationLeadDays: 0,
		}},
		companies: []string{"co-1"},
		store:     map[string]disclosureapp.PeriodicCycleRow{},
	}
	n, err := newFuturePregenSvc(repo).SeedPeriodicCycles(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || len(repo.store) != 1 {
		t.Fatalf("want only current; seeded=%d store=%v", n, storeLabels(repo))
	}
	if _, ok := repo.store["co-1|t-zero|2026-09"]; !ok {
		t.Fatal("missing current slot")
	}
}

func TestFuturePregen_Monthly_GenerateAtBeforeAndOn(t *testing.T) {
	ctx := context.Background()
	base := disclosureapp.PeriodicTypeRow{
		TypeID: "t-m", FrequencyUnit: "monthly", CycleAnchorDay: 5, DeadlineDays: 10,
		PeriodicCycleGenerationLeadDays: 10,
	}

	t.Run("before", func(t *testing.T) {
		now := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
		repo := &futurePregenRepo{
			Repository: inmemory.NewRepository(),
			types:      []disclosureapp.PeriodicTypeRow{base},
			companies:  []string{"co-1"},
			store:      map[string]disclosureapp.PeriodicCycleRow{},
		}
		n, err := newFuturePregenSvc(repo).SeedPeriodicCycles(ctx, now)
		if err != nil {
			t.Fatal(err)
		}
		if n != 1 || len(repo.store) != 1 {
			t.Fatalf("before GenerateAt want current only; seeded=%d store=%v", n, storeLabels(repo))
		}
	})

	t.Run("on", func(t *testing.T) {
		now := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
		repo := &futurePregenRepo{
			Repository: inmemory.NewRepository(),
			types:      []disclosureapp.PeriodicTypeRow{base},
			companies:  []string{"co-1"},
			store:      map[string]disclosureapp.PeriodicCycleRow{},
		}
		n, err := newFuturePregenSvc(repo).SeedPeriodicCycles(ctx, now)
		if err != nil {
			t.Fatal(err)
		}
		if n != 2 || len(repo.store) != 2 {
			t.Fatalf("on GenerateAt want current+next; seeded=%d store=%v", n, storeLabels(repo))
		}
		if _, ok := repo.store["co-1|t-m|2026-10"]; !ok {
			t.Fatal("missing future Oct slot")
		}
	})
}

func TestFuturePregen_Quarterly_GenerateAt(t *testing.T) {
	ctx := context.Background()
	miq := 1
	now := time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)
	repo := &futurePregenRepo{
		Repository: inmemory.NewRepository(),
		types: []disclosureapp.PeriodicTypeRow{{
			TypeID: "t-q", FrequencyUnit: "quarterly", CycleAnchorDay: 1, MonthInQuarter: &miq, DeadlineDays: 5,
			PeriodicCycleGenerationLeadDays: 20,
		}},
		companies: []string{"co-1"},
		store:     map[string]disclosureapp.PeriodicCycleRow{},
	}
	n, err := newFuturePregenSvc(repo).SeedPeriodicCycles(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("want current Q3 + future Q4; seeded=%d store=%v", n, storeLabels(repo))
	}
	if _, ok := repo.store["co-1|t-q|2026-Q4"]; !ok {
		t.Fatalf("missing Q4; store=%v", storeLabels(repo))
	}
}

func TestFuturePregen_Yearly_NextOnly(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	repo := &futurePregenRepo{
		Repository: inmemory.NewRepository(),
		types: []disclosureapp.PeriodicTypeRow{{
			TypeID: "t-y", FrequencyUnit: "yearly", CycleAnchorMonth: 1, CycleAnchorDay: 15, DeadlineDays: 5,
			PeriodicCycleGenerationLeadDays: 90,
		}},
		companies: []string{"co-1"},
		store:     map[string]disclosureapp.PeriodicCycleRow{},
	}
	n, err := newFuturePregenSvc(repo).SeedPeriodicCycles(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 || len(repo.store) != 2 {
		t.Fatalf("want 2026+2027 only; seeded=%d store=%v", n, storeLabels(repo))
	}
	if _, ok := repo.store["co-1|t-y|2027"]; !ok {
		t.Fatal("missing 2027")
	}
	if _, ok := repo.store["co-1|t-y|2028"]; ok {
		t.Fatal("must not seed more than one future")
	}
}

func TestFuturePregen_AFSkipCurrent_MaySeedFuture(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 10, 10, 0, 0, 0, time.UTC)
	repo := &futurePregenRepo{
		Repository: inmemory.NewRepository(),
		types: []disclosureapp.PeriodicTypeRow{{
			TypeID: "t-af", FrequencyUnit: "monthly", CycleAnchorDay: 5, DeadlineDays: 10,
			ApplicableFromMode: disclosureapp.ApplicableFromModeSpecific, ApplicableFromSlot: "2026-11",
			PeriodicCycleGenerationLeadDays: 40,
		}},
		companies: []string{"co-1"},
		store:     map[string]disclosureapp.PeriodicCycleRow{},
	}
	n, err := newFuturePregenSvc(repo).SeedPeriodicCycles(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := repo.store["co-1|t-af|2026-10"]; ok {
		t.Fatal("current Oct must remain skipped by AF")
	}
	if n != 1 || len(repo.store) != 1 {
		t.Fatalf("want only Nov; seeded=%d store=%v", n, storeLabels(repo))
	}
	if _, ok := repo.store["co-1|t-af|2026-11"]; !ok {
		t.Fatal("want Nov future seed")
	}

	nowEarly := time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC)
	repo2 := &futurePregenRepo{
		Repository: inmemory.NewRepository(),
		types:      repo.types,
		companies:  []string{"co-1"},
		store:      map[string]disclosureapp.PeriodicCycleRow{},
	}
	n, err = newFuturePregenSvc(repo2).SeedPeriodicCycles(ctx, nowEarly)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 || len(repo2.store) != 0 {
		t.Fatalf("AF skip + GenerateAt unmet want 0; seeded=%d", n)
	}
}

func TestFuturePregen_ApplicableToNone(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	repo := &futurePregenRepo{
		Repository: inmemory.NewRepository(),
		types: []disclosureapp.PeriodicTypeRow{{
			TypeID: "t-at", FrequencyUnit: "monthly", CycleAnchorDay: 10, DeadlineDays: 5,
			ApplicableTo: "2026-09-30", PeriodicCycleGenerationLeadDays: 90,
		}},
		companies: []string{"co-1"},
		store:     map[string]disclosureapp.PeriodicCycleRow{},
	}
	n, err := newFuturePregenSvc(repo).SeedPeriodicCycles(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || len(repo.store) != 1 {
		t.Fatalf("want current only; seeded=%d store=%v", n, storeLabels(repo))
	}
}

func TestFuturePregen_CompanyEffectiveT_GenerateAt(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	active := true
	repo := &futurePregenRepo{
		Repository: inmemory.NewRepository(),
		types: []disclosureapp.PeriodicTypeRow{{
			TypeID: "t-co", FrequencyUnit: "monthly", CycleAnchorDay: 30, DeadlineDays: 10,
			PeriodicCycleGenerationLeadDays: 10,
		}},
		companies: []string{"co-early", "co-late"},
		prefs: []disclosureapp.CompanyTypePreference{{
			CompanyID: "co-early", TypeID: "t-co", AutoCreateEnabled: true,
			CycleAnchorDay: 5, OverrideFrequency: "monthly", OverrideActive: &active,
		}},
		store: map[string]disclosureapp.PeriodicCycleRow{},
	}
	_, err := newFuturePregenSvc(repo).SeedPeriodicCycles(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := repo.store["co-early|t-co|2026-10"]; !ok {
		t.Fatalf("early company must seed future; store=%v", storeLabels(repo))
	}
	if _, ok := repo.store["co-late|t-co|2026-10"]; ok {
		t.Fatal("late company GenerateAt unmet must not seed future")
	}
	if _, ok := repo.store["co-late|t-co|2026-09"]; !ok {
		t.Fatal("late company must still seed current")
	}
}

func TestFuturePregen_LargeLead_NextOnly(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	repo := &futurePregenRepo{
		Repository: inmemory.NewRepository(),
		types: []disclosureapp.PeriodicTypeRow{{
			TypeID: "t-one", FrequencyUnit: "monthly", CycleAnchorDay: 5, DeadlineDays: 10,
			PeriodicCycleGenerationLeadDays: 90,
		}},
		companies: []string{"co-1"},
		store:     map[string]disclosureapp.PeriodicCycleRow{},
	}
	n, err := newFuturePregenSvc(repo).SeedPeriodicCycles(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("want exactly current+one future; seeded=%d store=%v", n, storeLabels(repo))
	}
	if _, ok := repo.store["co-1|t-one|2026-11"]; ok {
		t.Fatal("must not seed second future")
	}
}

func TestFuturePregen_IdempotentStickyUpsert(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
	repo := &futurePregenRepo{
		Repository: inmemory.NewRepository(),
		types: []disclosureapp.PeriodicTypeRow{{
			TypeID: "t-id", FrequencyUnit: "monthly", CycleAnchorDay: 5, DeadlineDays: 10,
			PeriodicCycleGenerationLeadDays: 10,
		}},
		companies: []string{"co-1"},
		store:     map[string]disclosureapp.PeriodicCycleRow{},
	}
	svc := newFuturePregenSvc(repo)
	if _, err := svc.SeedPeriodicCycles(ctx, now); err != nil {
		t.Fatal(err)
	}
	first := repo.store["co-1|t-id|2026-10"]
	if _, err := svc.SeedPeriodicCycles(ctx, now); err != nil {
		t.Fatal(err)
	}
	second := repo.store["co-1|t-id|2026-10"]
	if first.CycleID != second.CycleID || !first.CycleStart.Equal(second.CycleStart) {
		t.Fatalf("sticky immutability broken: %#v vs %#v", first, second)
	}
	if len(repo.store) != 2 {
		t.Fatalf("store size want 2 got %d", len(repo.store))
	}
}

func TestFuturePregen_NoHistoricalBackfill(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 10, 10, 0, 0, 0, time.UTC)
	repo := &futurePregenRepo{
		Repository: inmemory.NewRepository(),
		types: []disclosureapp.PeriodicTypeRow{{
			TypeID: "t-nobf", FrequencyUnit: "monthly", CycleAnchorDay: 5, DeadlineDays: 10,
			PeriodicCycleGenerationLeadDays: 90,
		}},
		companies: []string{"co-1"},
		store:     map[string]disclosureapp.PeriodicCycleRow{},
	}
	_, err := newFuturePregenSvc(repo).SeedPeriodicCycles(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := repo.store["co-1|t-nobf|2026-09"]; ok {
		t.Fatal("must not backfill historical Sept")
	}
	if _, ok := repo.store["co-1|t-nobf|2026-10"]; !ok {
		t.Fatal("want current Oct")
	}
	if _, ok := repo.store["co-1|t-nobf|2026-11"]; !ok {
		t.Fatal("want next Nov only")
	}
}

func TestFuturePregen_PregeneratedBecomesCurrentReuse(t *testing.T) {
	ctx := context.Background()
	repo := &futurePregenRepo{
		Repository: inmemory.NewRepository(),
		types: []disclosureapp.PeriodicTypeRow{{
			TypeID: "t-reuse", FrequencyUnit: "monthly", CycleAnchorDay: 5, DeadlineDays: 10,
			PeriodicCycleGenerationLeadDays: 10,
		}},
		companies: []string{"co-1"},
		store:     map[string]disclosureapp.PeriodicCycleRow{},
	}
	svc := newFuturePregenSvc(repo)
	if _, err := svc.SeedPeriodicCycles(ctx, time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	pre := repo.store["co-1|t-reuse|2026-10"]
	if pre.CycleID == "" {
		t.Fatal("expected pregenerated Oct")
	}
	if _, err := svc.SeedPeriodicCycles(ctx, time.Date(2026, 10, 10, 10, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	cur := repo.store["co-1|t-reuse|2026-10"]
	if cur.CycleID != pre.CycleID || !cur.CycleStart.Equal(pre.CycleStart) || !cur.OpenAt.Equal(pre.OpenAt) {
		t.Fatalf("pregenerated must reuse sticky snapshot: pre=%#v cur=%#v", pre, cur)
	}
}
