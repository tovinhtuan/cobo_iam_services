package app_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
	"github.com/cobo/cobo_iam_services/internal/disclosure/infra/inmemory"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

// noHolidaysProvider always reports trading days, so addDurationInclusive
// never errors on missing configs/non_trading_days/*.json fixtures in the
// test working directory — letting the shadow code paths actually execute
// instead of short-circuiting on a calculator error.
type noHolidaysProvider struct{}

func (noHolidaysProvider) IsNonTradingDay(_ context.Context, _ time.Time) (bool, string, error) {
	return false, "", nil
}

// ---------------------------------------------------------------------------
// Batch 5B — Behavior Preservation: SeedPeriodicCycles (Phase B shadow)
// ---------------------------------------------------------------------------

type seedShadowTestRepo struct {
	*inmemory.Repository
	types     []disclosureapp.PeriodicTypeRow
	companies []string
	captured  []disclosureapp.PeriodicCycleRow
}

func (r *seedShadowTestRepo) ListActivePeriodicTypes(_ context.Context) ([]disclosureapp.PeriodicTypeRow, error) {
	return r.types, nil
}

func (r *seedShadowTestRepo) ListAllActiveCompanyIDs(_ context.Context) ([]string, error) {
	return r.companies, nil
}

func (r *seedShadowTestRepo) UpsertPeriodicCycle(_ context.Context, row disclosureapp.PeriodicCycleRow) error {
	r.captured = append(r.captured, row)
	return nil
}

func newSeedShadowRepo() *seedShadowTestRepo {
	return &seedShadowTestRepo{
		Repository: inmemory.NewRepository(),
		types: []disclosureapp.PeriodicTypeRow{
			{
				TypeID:         "pt-monthly",
				FrequencyUnit:  "monthly",
				CycleAnchorDay: 5,
				DeadlineDays:   10,
				IsGlobal:       false,
			},
		},
		companies: []string{"co-1"},
	}
}

// stripCycleID zeroes the generated CycleID so two independent UUIDv7 runs
// can be compared by value.
func stripCycleID(rows []disclosureapp.PeriodicCycleRow) []disclosureapp.PeriodicCycleRow {
	out := make([]disclosureapp.PeriodicCycleRow, len(rows))
	for i, r := range rows {
		r.CycleID = ""
		out[i] = r
	}
	return out
}

func TestSeedPeriodicCycles_ShadowEnabledVsDisabled_SameDBWrites(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)

	repoOff := newSeedShadowRepo()
	svcOff := disclosureapp.NewService(repoOff, nil, idgen.UUIDv7Generator{}, disclosureapp.WithHolidayCalendarProvider(noHolidaysProvider{}))
	nOff, err := svcOff.SeedPeriodicCycles(ctx, now)
	if err != nil {
		t.Fatalf("seed (shadow off) error: %v", err)
	}

	repoOn := newSeedShadowRepo()
	svcOn := disclosureapp.NewService(repoOn, nil, idgen.UUIDv7Generator{}, disclosureapp.WithHolidayCalendarProvider(noHolidaysProvider{}), disclosureapp.WithDeadlineEngineV2Shadow(true))
	nOn, err := svcOn.SeedPeriodicCycles(ctx, now)
	if err != nil {
		t.Fatalf("seed (shadow on) error: %v", err)
	}

	if nOff != nOn {
		t.Fatalf("seeded count differs: off=%d on=%d", nOff, nOn)
	}
	if len(repoOff.captured) == 0 {
		t.Fatal("expected at least one UpsertPeriodicCycle call")
	}
	if !reflect.DeepEqual(stripCycleID(repoOff.captured), stripCycleID(repoOn.captured)) {
		t.Fatalf("DB writes differ:\noff=%#v\non=%#v", repoOff.captured, repoOn.captured)
	}
}

// ---------------------------------------------------------------------------
// Batch 5B — Behavior Preservation: GetTypeDetail (Phase A shadow)
// ---------------------------------------------------------------------------

type typeDetailShadowTestRepo struct {
	*inmemory.Repository
	detail *disclosureapp.DisclosureTypeDTO
}

func (r *typeDetailShadowTestRepo) GetTypeDetail(_ context.Context, _, _ string) (*disclosureapp.DisclosureTypeDTO, error) {
	cp := *r.detail
	return &cp, nil
}

func periodicTypeDetail() *disclosureapp.DisclosureTypeDTO {
	return &disclosureapp.DisclosureTypeDTO{
		TypeID: "pt-monthly",
		Scope:  "company",
		DeadlineConfig: &disclosureapp.TemplateDeadlineConfig{
			DeadlineMode:   disclosureapp.DeadlineModePeriodic,
			DeadlineDays:   10,
			FrequencyUnit:  "monthly",
			CycleAnchorDay: 5,
		},
		ApplicabilityRules: &applicability.TemplateApplicabilityRules{
			DeadlineDays:    10,
			DeadlineDayType: "working",
		},
	}
}

func TestGetTypeDetail_ShadowEnabledVsDisabled_SameSummary(t *testing.T) {
	ctx := context.Background()
	sub := disclosureapp.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "co-1"}
	req := disclosureapp.GetTypeDetailRequest{Subject: sub, TypeID: "pt-monthly"}

	repoOff := &typeDetailShadowTestRepo{Repository: inmemory.NewRepository(), detail: periodicTypeDetail()}
	svcOff := disclosureapp.NewService(repoOff, nil, idgen.UUIDv7Generator{}, disclosureapp.WithHolidayCalendarProvider(noHolidaysProvider{}))
	gotOff, err := svcOff.GetTypeDetail(ctx, req)
	if err != nil {
		t.Fatalf("GetTypeDetail (shadow off) error: %v", err)
	}

	repoOn := &typeDetailShadowTestRepo{Repository: inmemory.NewRepository(), detail: periodicTypeDetail()}
	svcOn := disclosureapp.NewService(repoOn, nil, idgen.UUIDv7Generator{}, disclosureapp.WithHolidayCalendarProvider(noHolidaysProvider{}), disclosureapp.WithDeadlineEngineV2Shadow(true))
	gotOn, err := svcOn.GetTypeDetail(ctx, req)
	if err != nil {
		t.Fatalf("GetTypeDetail (shadow on) error: %v", err)
	}

	if gotOff.DeadlineSummary == nil || gotOn.DeadlineSummary == nil {
		t.Fatalf("expected non-nil DeadlineSummary, off=%#v on=%#v", gotOff.DeadlineSummary, gotOn.DeadlineSummary)
	}
	if !reflect.DeepEqual(gotOff.DeadlineSummary, gotOn.DeadlineSummary) {
		t.Fatalf("DeadlineSummary differs:\noff=%#v\non=%#v", gotOff.DeadlineSummary, gotOn.DeadlineSummary)
	}
	if !reflect.DeepEqual(gotOff, gotOn) {
		t.Fatalf("DisclosureTypeDTO differs:\noff=%#v\non=%#v", gotOff, gotOn)
	}
}

// ---------------------------------------------------------------------------
// Batch 5B — Behavior Preservation: CreateRecord (Phase C shadow)
// ---------------------------------------------------------------------------

type createRecordShadowTestRepo struct {
	*inmemory.Repository
	detail *disclosureapp.DisclosureTypeDTO
}

func (r *createRecordShadowTestRepo) GetTypeDetail(_ context.Context, _, _ string) (*disclosureapp.DisclosureTypeDTO, error) {
	cp := *r.detail
	return &cp, nil
}

func (r *createRecordShadowTestRepo) HasActiveEnterpriseWorkflow(_ context.Context, _, _ string) (bool, error) {
	return disclosureapp.TemplateHasWorkflow(r.detail.Blocks), nil
}

func periodicWorkflowTypeDetail() *disclosureapp.DisclosureTypeDTO {
	detail := periodicTypeDetail()
	detail.Blocks = []disclosureapp.TemplateBlockDTO{
		{
			BlockKey:  "enterprise_workflow",
			BlockType: "rich_text",
			Enabled:   true,
			Config: map[string]any{
				"steps": []any{
					map[string]any{
						"step_id":           "s1",
						"stage":             "Review",
						"department_id":     "dept-1",
						"assignee_role_ids": []any{"role-1"},
						"processing_days":   2,
						"display_order":     1,
					},
				},
			},
		},
	}
	return detail
}

func createRecordReq() disclosureapp.CreateRecordRequest {
	return disclosureapp.CreateRecordRequest{
		RecordID: "rec-fixed-1",
		Subject:  disclosureapp.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "co-1"},
		Payload: disclosureapp.RecordPayload{
			TypeID:      "pt-monthly",
			Title:       "Báo cáo quý",
			Content:     "Nội dung",
			PlannedDate: "2026-06-30",
		},
	}
}

func TestCreateRecord_ShadowEnabledVsDisabled_SameRecord(t *testing.T) {
	ctx := context.Background()

	repoOff := &createRecordShadowTestRepo{Repository: inmemory.NewRepository(), detail: periodicWorkflowTypeDetail()}
	svcOff := disclosureapp.NewService(repoOff, nil, idgen.UUIDv7Generator{}, disclosureapp.WithHolidayCalendarProvider(noHolidaysProvider{}))
	gotOff, err := svcOff.CreateRecord(ctx, createRecordReq())
	if err != nil {
		t.Fatalf("CreateRecord (shadow off) error: %v", err)
	}

	repoOn := &createRecordShadowTestRepo{Repository: inmemory.NewRepository(), detail: periodicWorkflowTypeDetail()}
	svcOn := disclosureapp.NewService(repoOn, nil, idgen.UUIDv7Generator{}, disclosureapp.WithHolidayCalendarProvider(noHolidaysProvider{}), disclosureapp.WithDeadlineEngineV2Shadow(true))
	gotOn, err := svcOn.CreateRecord(ctx, createRecordReq())
	if err != nil {
		t.Fatalf("CreateRecord (shadow on) error: %v", err)
	}

	if !reflect.DeepEqual(gotOff, gotOn) {
		t.Fatalf("CreateRecord result differs:\noff=%#v\non=%#v", gotOff, gotOn)
	}
}
