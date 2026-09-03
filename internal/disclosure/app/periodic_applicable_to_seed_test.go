package app_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/infra/inmemory"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

// seedApplicableToRepo extends seedShadow harness with prefs + cycle store for Phase C.
type seedApplicableToRepo struct {
	*inmemory.Repository
	types     []disclosureapp.PeriodicTypeRow
	companies []string
	prefs     []disclosureapp.CompanyTypePreference
	captured  []disclosureapp.PeriodicCycleRow
	store     map[string]disclosureapp.PeriodicCycleRow
	deletes   int
}

func (r *seedApplicableToRepo) ListActivePeriodicTypes(_ context.Context) ([]disclosureapp.PeriodicTypeRow, error) {
	return r.types, nil
}

func (r *seedApplicableToRepo) ListAllActiveCompanyIDs(_ context.Context) ([]string, error) {
	return r.companies, nil
}

func (r *seedApplicableToRepo) ListCompanyTypePreferencesByTypeIDs(_ context.Context, _ []string) ([]disclosureapp.CompanyTypePreference, error) {
	return r.prefs, nil
}

func (r *seedApplicableToRepo) UpsertPeriodicCycle(_ context.Context, row disclosureapp.PeriodicCycleRow) error {
	r.captured = append(r.captured, row)
	if r.store == nil {
		r.store = map[string]disclosureapp.PeriodicCycleRow{}
	}
	key := row.CompanyID + "|" + row.TypeID + "|" + row.CycleLabel
	r.store[key] = row
	return nil
}

func (r *seedApplicableToRepo) DeleteUnmaterializedPeriodicCycle(_ context.Context, _ string) error {
	r.deletes++
	return nil
}

func newSeedApplicableToSvc(repo *seedApplicableToRepo) disclosureapp.Service {
	return disclosureapp.NewService(repo, nil, idgen.UUIDv7Generator{}, disclosureapp.WithHolidayCalendarProvider(noHolidaysProvider{}))
}

func upsertCountFor(repo *seedApplicableToRepo, companyID, typeID string) int {
	n := 0
	for _, row := range repo.captured {
		if row.CompanyID == companyID && row.TypeID == typeID {
			n++
		}
	}
	return n
}

func TestSeedPeriodicCycles_ApplicableTo_NullOpenEnded(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	repo := &seedApplicableToRepo{
		Repository: inmemory.NewRepository(),
		types: []disclosureapp.PeriodicTypeRow{{
			TypeID: "t-open", FrequencyUnit: "monthly", CycleAnchorDay: 5, DeadlineDays: 10,
			ApplicableTo: "",
		}},
		companies: []string{"co-1"},
	}
	svc := newSeedApplicableToSvc(repo)
	n, err := svc.SeedPeriodicCycles(ctx, now)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if n != 1 || len(repo.captured) != 1 {
		t.Fatalf("open-ended want 1 upsert, seeded=%d captured=%d", n, len(repo.captured))
	}
}

func TestSeedPeriodicCycles_ApplicableTo_DailyBeforeEqualAfter(t *testing.T) {
	ctx := context.Background()
	base := disclosureapp.PeriodicTypeRow{
		TypeID: "t-daily", FrequencyUnit: "daily", DeadlineDays: 1, ApplicableTo: "2026-09-05",
	}

	cases := []struct {
		name    string
		now     time.Time
		wantUps int
	}{
		{"before", time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC), 1},
		{"equal", time.Date(2026, 9, 5, 10, 0, 0, 0, time.UTC), 1},
		{"after", time.Date(2026, 9, 6, 10, 0, 0, 0, time.UTC), 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &seedApplicableToRepo{
				Repository: inmemory.NewRepository(),
				types:      []disclosureapp.PeriodicTypeRow{base},
				companies:  []string{"co-1"},
			}
			n, err := newSeedApplicableToSvc(repo).SeedPeriodicCycles(ctx, tc.now)
			if err != nil {
				t.Fatalf("seed: %v", err)
			}
			if n != tc.wantUps || len(repo.captured) != tc.wantUps {
				t.Fatalf("seeded=%d captured=%d want %d", n, len(repo.captured), tc.wantUps)
			}
		})
	}
}

func TestSeedPeriodicCycles_ApplicableTo_WeeklyOverlapAndEqual(t *testing.T) {
	ctx := context.Background()
	// 2026-09-14 (Mon) → week slot Sunday 2026-09-13.
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	wed := int(time.Wednesday) // T=2026-09-16
	tue := int(time.Tuesday)   // T=2026-09-15

	repoSkip := &seedApplicableToRepo{
		Repository: inmemory.NewRepository(),
		types: []disclosureapp.PeriodicTypeRow{{
			TypeID: "t-week-skip", FrequencyUnit: "weekly", DeadlineDays: 1,
			CycleAnchorWeekday: &wed, ApplicableTo: "2026-09-15",
		}},
		companies: []string{"co-1"},
	}
	n, err := newSeedApplicableToSvc(repoSkip).SeedPeriodicCycles(ctx, now)
	if err != nil {
		t.Fatalf("skip seed: %v", err)
	}
	if n != 0 || len(repoSkip.captured) != 0 {
		t.Fatalf("weekly overlap T>To must skip upsert; seeded=%d", n)
	}

	repoEq := &seedApplicableToRepo{
		Repository: inmemory.NewRepository(),
		types: []disclosureapp.PeriodicTypeRow{{
			TypeID: "t-week-eq", FrequencyUnit: "weekly", DeadlineDays: 1,
			CycleAnchorWeekday: &tue, ApplicableTo: "2026-09-15",
		}},
		companies: []string{"co-1"},
	}
	n, err = newSeedApplicableToSvc(repoEq).SeedPeriodicCycles(ctx, now)
	if err != nil {
		t.Fatalf("equal seed: %v", err)
	}
	if n != 1 || len(repoEq.captured) != 1 {
		t.Fatalf("weekly T==To must upsert; seeded=%d", n)
	}
}

func TestSeedPeriodicCycles_ApplicableTo_MonthlyPartialP0AndValid(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)

	repoSkip := &seedApplicableToRepo{
		Repository: inmemory.NewRepository(),
		types: []disclosureapp.PeriodicTypeRow{{
			TypeID: "t-m-p0", FrequencyUnit: "monthly", CycleAnchorDay: 30, DeadlineDays: 10,
			ApplicableTo: "2026-09-15",
		}},
		companies: []string{"co-1"},
	}
	n, err := newSeedApplicableToSvc(repoSkip).SeedPeriodicCycles(ctx, now)
	if err != nil {
		t.Fatalf("p0 seed: %v", err)
	}
	if n != 0 || len(repoSkip.captured) != 0 {
		t.Fatalf("monthly T=30/09 To=15 must not upsert; seeded=%d", n)
	}

	repoOK := &seedApplicableToRepo{
		Repository: inmemory.NewRepository(),
		types: []disclosureapp.PeriodicTypeRow{{
			TypeID: "t-m-ok", FrequencyUnit: "monthly", CycleAnchorDay: 10, DeadlineDays: 10,
			ApplicableTo: "2026-09-15",
		}},
		companies: []string{"co-1"},
	}
	n, err = newSeedApplicableToSvc(repoOK).SeedPeriodicCycles(ctx, now)
	if err != nil {
		t.Fatalf("valid seed: %v", err)
	}
	if n != 1 || len(repoOK.captured) != 1 {
		t.Fatalf("monthly T=10/09 To=15 must upsert; seeded=%d", n)
	}
	if repoOK.captured[0].CycleStart.Format("2006-01-02") != "2026-09-10" {
		t.Fatalf("cycle_start want 2026-09-10 got %s", repoOK.captured[0].CycleStart.Format("2006-01-02"))
	}
}

func TestSeedPeriodicCycles_ApplicableTo_MonthlyClamp(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC)
	repo := &seedApplicableToRepo{
		Repository: inmemory.NewRepository(),
		types: []disclosureapp.PeriodicTypeRow{{
			TypeID: "t-clamp", FrequencyUnit: "monthly", CycleAnchorDay: 31, DeadlineDays: 5,
			ApplicableTo: "2026-04-30",
		}},
		companies: []string{"co-1"},
	}
	n, err := newSeedApplicableToSvc(repo).SeedPeriodicCycles(ctx, now)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if n != 1 || len(repo.captured) != 1 {
		t.Fatalf("clamped T=30/04 == To must upsert; seeded=%d", n)
	}
	if repo.captured[0].CycleStart.Format("2006-01-02") != "2026-04-30" {
		t.Fatalf("want clamped T 2026-04-30 got %s", repo.captured[0].CycleStart.Format("2006-01-02"))
	}
}

func TestSeedPeriodicCycles_ApplicableTo_QuarterlyAfter(t *testing.T) {
	ctx := context.Background()
	// Sept 2026 → 2026-Q3; MiQ=3 day=30 → T=2026-09-30 > To=2026-09-15
	now := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	miq := 3
	repo := &seedApplicableToRepo{
		Repository: inmemory.NewRepository(),
		types: []disclosureapp.PeriodicTypeRow{{
			TypeID: "t-q", FrequencyUnit: "quarterly", CycleAnchorDay: 30, MonthInQuarter: &miq, DeadlineDays: 5,
			ApplicableTo: "2026-09-15",
		}},
		companies: []string{"co-1"},
	}
	n, err := newSeedApplicableToSvc(repo).SeedPeriodicCycles(ctx, now)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if n != 0 || len(repo.captured) != 0 {
		t.Fatalf("quarterly T after To must skip; seeded=%d", n)
	}
}

func TestSeedPeriodicCycles_ApplicableTo_YearlyAfter(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	repo := &seedApplicableToRepo{
		Repository: inmemory.NewRepository(),
		types: []disclosureapp.PeriodicTypeRow{{
			TypeID: "t-y", FrequencyUnit: "yearly", CycleAnchorMonth: 12, CycleAnchorDay: 31, DeadlineDays: 5,
			ApplicableTo: "2026-06-30",
		}},
		companies: []string{"co-1"},
	}
	n, err := newSeedApplicableToSvc(repo).SeedPeriodicCycles(ctx, now)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if n != 0 || len(repo.captured) != 0 {
		t.Fatalf("yearly T after To must skip; seeded=%d", n)
	}
}

func TestSeedPeriodicCycles_ApplicableTo_CompanyEffectiveTSplit(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	active := true
	repo := &seedApplicableToRepo{
		Repository: inmemory.NewRepository(),
		types: []disclosureapp.PeriodicTypeRow{{
			TypeID: "t-co", FrequencyUnit: "monthly", CycleAnchorDay: 30, DeadlineDays: 10,
			ApplicableTo: "2026-09-15",
		}},
		companies: []string{"co-a", "co-b"},
		prefs: []disclosureapp.CompanyTypePreference{{
			CompanyID: "co-a", TypeID: "t-co", AutoCreateEnabled: true,
			CycleAnchorDay: 10, OverrideFrequency: "monthly", OverrideActive: &active,
		}},
	}
	n, err := newSeedApplicableToSvc(repo).SeedPeriodicCycles(ctx, now)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if n != 1 {
		t.Fatalf("want exactly 1 cycle (co-a only); seeded=%d captured=%#v", n, repo.captured)
	}
	if upsertCountFor(repo, "co-a", "t-co") != 1 {
		t.Fatal("Company A override T=10/09 must allow cycle")
	}
	if upsertCountFor(repo, "co-b", "t-co") != 0 {
		t.Fatal("Company B inherit CMS T=30/09 must block cycle")
	}
	if repo.captured[0].CycleStart.Format("2006-01-02") != "2026-09-10" {
		t.Fatalf("co-a T want 2026-09-10 got %s", repo.captured[0].CycleStart.Format("2006-01-02"))
	}
}

func TestSeedPeriodicCycles_ApplicableTo_InvalidFailClosedAndOtherContinues(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	repo := &seedApplicableToRepo{
		Repository: inmemory.NewRepository(),
		types: []disclosureapp.PeriodicTypeRow{
			{TypeID: "t-bad", FrequencyUnit: "monthly", CycleAnchorDay: 5, DeadlineDays: 10, ApplicableTo: "2026-02-30"},
			{TypeID: "t-ok", FrequencyUnit: "monthly", CycleAnchorDay: 5, DeadlineDays: 10, ApplicableTo: "2026-09-30"},
		},
		companies: []string{"co-1"},
	}
	n, err := newSeedApplicableToSvc(repo).SeedPeriodicCycles(ctx, now)
	if err != nil {
		t.Fatalf("batch must not abort: %v", err)
	}
	if n != 1 || upsertCountFor(repo, "co-1", "t-ok") != 1 {
		t.Fatalf("valid template must still seed; seeded=%d", n)
	}
	if upsertCountFor(repo, "co-1", "t-bad") != 0 {
		t.Fatal("invalid ApplicableTo must not create cycle")
	}
}

func TestSeedPeriodicCycles_ApplicableTo_ExistingCyclePreservedAfterShorten(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	repo := &seedApplicableToRepo{
		Repository: inmemory.NewRepository(),
		types: []disclosureapp.PeriodicTypeRow{{
			TypeID: "t-exist", FrequencyUnit: "monthly", CycleAnchorDay: 30, DeadlineDays: 10,
			ApplicableTo: "",
		}},
		companies: []string{"co-1"},
		store:     map[string]disclosureapp.PeriodicCycleRow{},
	}
	svc := newSeedApplicableToSvc(repo)
	if _, err := svc.SeedPeriodicCycles(ctx, now); err != nil {
		t.Fatalf("initial seed: %v", err)
	}
	key := "co-1|t-exist|2026-09"
	before, ok := repo.store[key]
	if !ok {
		t.Fatal("expected September cycle after open-ended seed")
	}
	repo.types[0].ApplicableTo = "2026-09-15" // shorten; T=30/09 now ineligible
	capturedBefore := len(repo.captured)
	n, err := svc.SeedPeriodicCycles(ctx, now)
	if err != nil {
		t.Fatalf("shortened seed: %v", err)
	}
	if n != 0 {
		t.Fatalf("shortened To must not upsert new candidate; seeded=%d", n)
	}
	if len(repo.captured) != capturedBefore {
		t.Fatalf("no additional upserts expected; before=%d after=%d", capturedBefore, len(repo.captured))
	}
	after, ok := repo.store[key]
	if !ok {
		t.Fatal("existing cycle must remain in store")
	}
	if after.CycleID != before.CycleID || after.CycleStart != before.CycleStart {
		t.Fatalf("existing cycle mutated: before=%#v after=%#v", before, after)
	}
	if repo.deletes != 0 {
		t.Fatalf("must not delete cycles; deletes=%d", repo.deletes)
	}
}

func TestSeedPeriodicCycles_ApplicableTo_ExtensionSameCurrentSlot(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	repo := &seedApplicableToRepo{
		Repository: inmemory.NewRepository(),
		types: []disclosureapp.PeriodicTypeRow{{
			TypeID: "t-ext", FrequencyUnit: "monthly", CycleAnchorDay: 30, DeadlineDays: 10,
			ApplicableTo: "2026-09-15",
		}},
		companies: []string{"co-1"},
	}
	svc := newSeedApplicableToSvc(repo)
	n, err := svc.SeedPeriodicCycles(ctx, now)
	if err != nil {
		t.Fatalf("blocked seed: %v", err)
	}
	if n != 0 {
		t.Fatalf("initially blocked want 0; got %d", n)
	}
	repo.types[0].ApplicableTo = "2026-09-30"
	n, err = svc.SeedPeriodicCycles(ctx, now)
	if err != nil {
		t.Fatalf("extended seed: %v", err)
	}
	if n != 1 || len(repo.captured) != 1 {
		t.Fatalf("same current slot after extend must allow; seeded=%d", n)
	}
}

func TestSeedPeriodicCycles_ApplicableTo_NoHistoricalBackfill(t *testing.T) {
	ctx := context.Background()
	repo := &seedApplicableToRepo{
		Repository: inmemory.NewRepository(),
		types: []disclosureapp.PeriodicTypeRow{{
			TypeID: "t-nobf", FrequencyUnit: "monthly", CycleAnchorDay: 30, DeadlineDays: 10,
			ApplicableTo: "2026-09-15",
		}},
		companies: []string{"co-1"},
	}
	svc := newSeedApplicableToSvc(repo)
	sept := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	if n, err := svc.SeedPeriodicCycles(ctx, sept); err != nil || n != 0 {
		t.Fatalf("sept skip: n=%d err=%v", n, err)
	}
	repo.types[0].ApplicableTo = "2026-12-31"
	oct := time.Date(2026, 10, 10, 10, 0, 0, 0, time.UTC)
	n, err := svc.SeedPeriodicCycles(ctx, oct)
	if err != nil {
		t.Fatalf("oct seed: %v", err)
	}
	if n != 1 || len(repo.captured) != 1 {
		t.Fatalf("want only current Oct slot; seeded=%d captured=%d", n, len(repo.captured))
	}
	if repo.captured[0].CycleLabel != "2026-10" {
		t.Fatalf("must not backfill Sept; got label %s", repo.captured[0].CycleLabel)
	}
}

func TestSeedPeriodicCycles_ApplicableTo_ApplicableFromStillGates(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	repo := &seedApplicableToRepo{
		Repository: inmemory.NewRepository(),
		types: []disclosureapp.PeriodicTypeRow{{
			TypeID: "t-af", FrequencyUnit: "monthly", CycleAnchorDay: 5, DeadlineDays: 10,
			ApplicableFromMode: disclosureapp.ApplicableFromModeNext, ApplicableFromSlot: "2026-09",
			ApplicableTo: "2026-12-31",
		}},
		companies: []string{"co-1"},
	}
	n, err := newSeedApplicableToSvc(repo).SeedPeriodicCycles(ctx, now)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if n != 0 || len(repo.captured) != 0 {
		t.Fatalf("ApplicableFrom before must still skip despite open ApplicableTo; seeded=%d", n)
	}
}

func TestSeedPeriodicCycles_ApplicableTo_IdempotentRepeatedTick(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	repo := &seedApplicableToRepo{
		Repository: inmemory.NewRepository(),
		types: []disclosureapp.PeriodicTypeRow{{
			TypeID: "t-id", FrequencyUnit: "monthly", CycleAnchorDay: 10, DeadlineDays: 10,
			ApplicableTo: "2026-09-15",
		}},
		companies: []string{"co-1"},
		store:     map[string]disclosureapp.PeriodicCycleRow{},
	}
	svc := newSeedApplicableToSvc(repo)
	if _, err := svc.SeedPeriodicCycles(ctx, now); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SeedPeriodicCycles(ctx, now); err != nil {
		t.Fatal(err)
	}
	if len(repo.store) != 1 {
		t.Fatalf("unique key store size want 1 got %d", len(repo.store))
	}
}

func TestApplicableTo_MaterializerSourceHasNoGate(t *testing.T) {
	data, err := os.ReadFile("periodic.go")
	if err != nil {
		t.Fatalf("read periodic.go: %v", err)
	}
	src := string(data)
	matIdx := strings.Index(src, "func materializePeriodicDisclosures")
	if matIdx < 0 {
		t.Fatal("materializePeriodicDisclosures not found")
	}
	matBody := src[matIdx:]
	if next := strings.Index(matBody[1:], "\nfunc "); next > 0 {
		matBody = matBody[:next+1]
	}
	if strings.Contains(matBody, "ApplicableTo") || strings.Contains(matBody, "EvaluateApplicableToEligibility") {
		t.Fatal("materializer must not recheck ApplicableTo")
	}
}
