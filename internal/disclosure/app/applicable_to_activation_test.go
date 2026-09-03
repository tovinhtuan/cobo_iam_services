package app

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

func pinnedWorkflowManifest() *WorkflowPublicationManifest {
	return &WorkflowPublicationManifest{
		SchemaVersion: WorkflowManifestSchemaVersion,
		Steps: []WorkflowPublicationStep{
			{WorkflowStepDTO: WorkflowStepDTO{
				StepID: "s1", Stage: "A", DepartmentID: "d1",
				AssigneeRoleIds: []string{"r1"}, ProcessingDays: 1, DueRule: "T+1",
			}},
		},
	}
}

func readyPeriodicItem(cfg *TemplateDeadlineConfig) *DisclosureTypeDTO {
	return &DisclosureTypeDTO{
		TypeID:                "dt-at-ready",
		VersionNo:             1,
		WorkflowAuthorityMode: WorkflowAuthorityTemplatePinned,
		WorkflowManifest:      pinnedWorkflowManifest(),
		DeadlineConfig:        cfg,
	}
}

func hasBlockerCode(blockers []ActivationBlockerDTO, code string) bool {
	for _, b := range blockers {
		if b.Code == code {
			return true
		}
	}
	return false
}

func TestCollectApplicableToActivationBlockers_OpenEnded(t *testing.T) {
	eval := time.Date(2026, 9, 30, 10, 0, 0, 0, asiaHoChiMinh())
	if got := CollectApplicableToActivationBlockers(nil, eval); len(got) != 0 {
		t.Fatalf("nil cfg: %v", got)
	}
	cfg := &TemplateDeadlineConfig{FrequencyUnit: "monthly", CycleAnchorDay: 10, ApplicableTo: ""}
	if got := CollectApplicableToActivationBlockers(cfg, eval); len(got) != 0 {
		t.Fatalf("open-ended: %v", got)
	}
}

func TestCollectApplicableToActivationBlockers_InvalidFormat(t *testing.T) {
	eval := time.Date(2026, 9, 1, 10, 0, 0, 0, asiaHoChiMinh())
	cfg := &TemplateDeadlineConfig{FrequencyUnit: "daily", ApplicableTo: "2026-02-30"}
	got := CollectApplicableToActivationBlockers(cfg, eval)
	if len(got) != 1 || got[0].Code != ActivationBlockerApplicableToInvalid {
		t.Fatalf("got=%v", got)
	}
}

func TestCollectApplicableToActivationBlockers_Past(t *testing.T) {
	eval := time.Date(2026, 9, 30, 10, 0, 0, 0, asiaHoChiMinh())
	cfg := &TemplateDeadlineConfig{FrequencyUnit: "daily", ApplicableTo: "2026-09-29"}
	got := CollectApplicableToActivationBlockers(cfg, eval)
	if len(got) != 1 || got[0].Code != ActivationBlockerApplicableToPast {
		t.Fatalf("got=%v", got)
	}
}

func TestCollectApplicableToActivationBlockers_EqualTodayAllowed(t *testing.T) {
	loc := asiaHoChiMinh()
	eval := time.Date(2026, 9, 30, 10, 0, 0, 0, loc)
	cfg := &TemplateDeadlineConfig{
		FrequencyUnit: "daily", ApplicableTo: "2026-09-30",
		ApplicableFromMode: ApplicableFromModeSpecific, ApplicableFromSlot: "2026-09-30",
	}
	got := CollectApplicableToActivationBlockers(cfg, eval)
	if len(got) != 0 {
		t.Fatalf("equal today + T equal must allow: %v", got)
	}
}

func TestCollectApplicableToActivationBlockers_MonthlyPartialPeriodP0(t *testing.T) {
	loc := asiaHoChiMinh()
	eval := time.Date(2026, 9, 1, 10, 0, 0, 0, loc)
	cfg := &TemplateDeadlineConfig{
		FrequencyUnit: "monthly", CycleAnchorDay: 30, ApplicableTo: "2026-09-15",
		ApplicableFromMode: ApplicableFromModeCurrent,
	}
	got := CollectApplicableToActivationBlockers(cfg, eval)
	if len(got) != 1 || got[0].Code != ActivationBlockerApplicabilityRangeInvalid {
		t.Fatalf("monthly T=30 To=15 → RANGE: got=%v", got)
	}
}

func TestCollectApplicableToActivationBlockers_MonthlyValid(t *testing.T) {
	loc := asiaHoChiMinh()
	eval := time.Date(2026, 9, 1, 10, 0, 0, 0, loc)
	cfg := &TemplateDeadlineConfig{
		FrequencyUnit: "monthly", CycleAnchorDay: 10, ApplicableTo: "2026-09-15",
		ApplicableFromMode: ApplicableFromModeCurrent,
	}
	got := CollectApplicableToActivationBlockers(cfg, eval)
	if len(got) != 0 {
		t.Fatalf("monthly T=10 To=15: %v", got)
	}
}

func TestCollectApplicableToActivationBlockers_WeeklyOverlap(t *testing.T) {
	loc := asiaHoChiMinh()
	// Wed 2026-09-16 week starts Sun 13; To=15 → range invalid.
	eval := time.Date(2026, 9, 14, 10, 0, 0, 0, loc)
	wed := int(time.Wednesday)
	cfg := &TemplateDeadlineConfig{
		FrequencyUnit: "weekly", CycleAnchorWeekday: &wed, ApplicableTo: "2026-09-15",
		ApplicableFromMode: ApplicableFromModeCurrent,
	}
	got := CollectApplicableToActivationBlockers(cfg, eval)
	if len(got) != 1 || got[0].Code != ActivationBlockerApplicabilityRangeInvalid {
		t.Fatalf("weekly overlap: %v", got)
	}
}

func TestCollectApplicableToActivationBlockers_WeeklyInclusive(t *testing.T) {
	loc := asiaHoChiMinh()
	eval := time.Date(2026, 9, 14, 10, 0, 0, 0, loc)
	tue := int(time.Tuesday) // 2026-09-15
	cfg := &TemplateDeadlineConfig{
		FrequencyUnit: "weekly", CycleAnchorWeekday: &tue, ApplicableTo: "2026-09-15",
		ApplicableFromMode: ApplicableFromModeCurrent,
	}
	got := CollectApplicableToActivationBlockers(cfg, eval)
	if len(got) != 0 {
		t.Fatalf("weekly inclusive: %v", got)
	}
}

func TestCollectApplicableToActivationBlockers_DailyRange(t *testing.T) {
	loc := asiaHoChiMinh()
	eval := time.Date(2026, 9, 10, 10, 0, 0, 0, loc)
	cfg := &TemplateDeadlineConfig{
		FrequencyUnit: "daily", ApplicableTo: "2026-09-15",
		ApplicableFromMode: ApplicableFromModeSpecific, ApplicableFromSlot: "2026-09-16",
	}
	got := CollectApplicableToActivationBlockers(cfg, eval)
	if len(got) != 1 || got[0].Code != ActivationBlockerApplicabilityRangeInvalid {
		t.Fatalf("daily T after To: %v", got)
	}
}

func TestCollectApplicableToActivationBlockers_NextSlot(t *testing.T) {
	loc := asiaHoChiMinh()
	eval := time.Date(2026, 9, 10, 10, 0, 0, 0, loc)
	cfg := &TemplateDeadlineConfig{
		FrequencyUnit: "monthly", CycleAnchorDay: 10, ApplicableTo: "2026-09-30",
		ApplicableFromMode: ApplicableFromModeNext, // next = 2026-10 → T=10/10 > 30/09
	}
	got := CollectApplicableToActivationBlockers(cfg, eval)
	if len(got) != 1 || got[0].Code != ActivationBlockerApplicabilityRangeInvalid {
		t.Fatalf("NEXT_SLOT after To: %v", got)
	}
}

func TestCollectApplicableToActivationBlockers_SpecificSlot(t *testing.T) {
	loc := asiaHoChiMinh()
	eval := time.Date(2026, 8, 1, 10, 0, 0, 0, loc)
	cfg := &TemplateDeadlineConfig{
		FrequencyUnit: "monthly", CycleAnchorDay: 30, ApplicableTo: "2026-09-15",
		ApplicableFromMode: ApplicableFromModeSpecific, ApplicableFromSlot: "2026-09",
	}
	got := CollectApplicableToActivationBlockers(cfg, eval)
	if len(got) != 1 || got[0].Code != ActivationBlockerApplicabilityRangeInvalid {
		t.Fatalf("SPECIFIC monthly P0: %v", got)
	}
}

func TestCollectApplicableToActivationBlockers_MonthlyClamp(t *testing.T) {
	loc := asiaHoChiMinh()
	eval := time.Date(2026, 4, 1, 10, 0, 0, 0, loc)
	cfg := &TemplateDeadlineConfig{
		FrequencyUnit: "monthly", CycleAnchorDay: 31, ApplicableTo: "2026-04-30",
		ApplicableFromMode: ApplicableFromModeCurrent,
	}
	got := CollectApplicableToActivationBlockers(cfg, eval)
	if len(got) != 0 {
		t.Fatalf("April clamp T=30 To=30: %v", got)
	}
}

func TestCollectApplicableToActivationBlockers_QuarterlyYearlyLeap(t *testing.T) {
	loc := asiaHoChiMinh()
	miq3 := 3
	eval := time.Date(2026, 7, 1, 10, 0, 0, 0, loc)
	qCfg := &TemplateDeadlineConfig{
		FrequencyUnit: "quarterly", CycleAnchorDay: 30, MonthInQuarter: &miq3,
		ApplicableTo: "2026-08-31", ApplicableFromMode: ApplicableFromModeCurrent,
	}
	if got := CollectApplicableToActivationBlockers(qCfg, eval); len(got) != 1 || got[0].Code != ActivationBlockerApplicabilityRangeInvalid {
		t.Fatalf("quarterly: %v", got)
	}

	yEval := time.Date(2026, 1, 15, 10, 0, 0, 0, loc)
	yCfg := &TemplateDeadlineConfig{
		FrequencyUnit: "yearly", CycleAnchorMonth: 12, CycleAnchorDay: 31,
		ApplicableTo: "2026-06-30", ApplicableFromMode: ApplicableFromModeCurrent,
	}
	if got := CollectApplicableToActivationBlockers(yCfg, yEval); len(got) != 1 || got[0].Code != ActivationBlockerApplicabilityRangeInvalid {
		t.Fatalf("yearly: %v", got)
	}

	leapEval := time.Date(2026, 1, 1, 10, 0, 0, 0, loc)
	leapCfg := &TemplateDeadlineConfig{
		FrequencyUnit: "yearly", CycleAnchorMonth: 2, CycleAnchorDay: 29,
		ApplicableTo: "2026-02-28", ApplicableFromMode: ApplicableFromModeCurrent,
	}
	if got := CollectApplicableToActivationBlockers(leapCfg, leapEval); len(got) != 0 {
		t.Fatalf("leap clamp: %v", got)
	}
}

func TestCollectApplicableToActivationBlockers_HCMPastAcrossUTC(t *testing.T) {
	// UTC 2026-09-30 18:30 → HCM 2026-10-01; ApplicableTo=2026-09-30 → PAST
	eval := time.Date(2026, 9, 30, 18, 30, 0, 0, time.UTC)
	cfg := &TemplateDeadlineConfig{FrequencyUnit: "daily", ApplicableTo: "2026-09-30"}
	got := CollectApplicableToActivationBlockers(cfg, eval)
	if len(got) != 1 || got[0].Code != ActivationBlockerApplicableToPast {
		t.Fatalf("HCM past: %v", got)
	}
}

func TestApplyActivationReadiness_ApplicableToPast(t *testing.T) {
	loc := asiaHoChiMinh()
	item := readyPeriodicItem(&TemplateDeadlineConfig{
		FrequencyUnit: "daily", ApplicableTo: "2026-09-29", DeadlineDays: 5,
	})
	applyActivationReadiness(item, time.Date(2026, 9, 30, 10, 0, 0, 0, loc), nil)
	if item.ActivationReady || !hasBlockerCode(item.ActivationBlockers, ActivationBlockerApplicableToPast) {
		t.Fatalf("ready=%v blockers=%v", item.ActivationReady, item.ActivationBlockers)
	}
}

func TestApplyActivationReadiness_ApplicableToRange(t *testing.T) {
	loc := asiaHoChiMinh()
	item := readyPeriodicItem(&TemplateDeadlineConfig{
		FrequencyUnit: "monthly", CycleAnchorDay: 30, DeadlineDays: 10,
		ApplicableTo: "2026-09-15", ApplicableFromMode: ApplicableFromModeCurrent,
	})
	applyActivationReadiness(item, time.Date(2026, 9, 1, 10, 0, 0, 0, loc), nil)
	if item.ActivationReady || !hasBlockerCode(item.ActivationBlockers, ActivationBlockerApplicabilityRangeInvalid) {
		t.Fatalf("ready=%v blockers=%v", item.ActivationReady, item.ActivationBlockers)
	}
}

func TestApplyActivationReadiness_ApplicableToValid(t *testing.T) {
	loc := asiaHoChiMinh()
	item := readyPeriodicItem(&TemplateDeadlineConfig{
		FrequencyUnit: "monthly", CycleAnchorDay: 10, DeadlineDays: 10,
		ApplicableTo: "2026-09-15", ApplicableFromMode: ApplicableFromModeCurrent,
	})
	applyActivationReadiness(item, time.Date(2026, 9, 1, 10, 0, 0, 0, loc), nil)
	if !item.ActivationReady {
		t.Fatalf("want ready, blockers=%v", item.ActivationBlockers)
	}
}

func TestApplyActivationReadiness_ApplicableToOpenEndedUnchanged(t *testing.T) {
	item := readyPeriodicItem(&TemplateDeadlineConfig{
		FrequencyUnit: "monthly", CycleAnchorDay: 5, DeadlineDays: 10,
		ApplicableFromMode: ApplicableFromModeCurrent,
	})
	applyActivationReadiness(item, time.Date(2026, 9, 26, 10, 0, 0, 0, asiaHoChiMinh()), nil)
	if !item.ActivationReady {
		t.Fatalf("open-ended must remain ready, blockers=%v", item.ActivationBlockers)
	}
}

type activateApplicableToRepo struct {
	Repository
	detail     *DisclosureTypeDTO
	activated  bool
	activateN  int
}

func (r *activateApplicableToRepo) GetTypeVersionDetail(_ context.Context, _, _ string, _ int) (*DisclosureTypeDTO, error) {
	return pinDisclosureWorkflow(r.detail), nil
}

func (r *activateApplicableToRepo) ActivateTypeVersion(_ context.Context, req ActivateTypeVersionRequest) (*ActivateTypeVersionResponse, error) {
	r.activated = true
	r.activateN++
	return &ActivateTypeVersionResponse{TypeID: req.TypeID, VersionNo: req.VersionNo, IsActive: true}, nil
}

func (r *activateApplicableToRepo) GetActiveGlobalWorkflow(_ context.Context, _ string) ([]WorkflowStepDTO, int, bool, error) {
	return nil, 0, false, nil
}

func (r *activateApplicableToRepo) ListActiveDeadlineRuleCatalog(_ context.Context) ([]DeadlineRuleCatalogDTO, error) {
	return defaultDeadlineRuleCatalog(), nil
}

func newActivateApplicableToService(detail *DisclosureTypeDTO) (Service, *activateApplicableToRepo) {
	repo := &activateApplicableToRepo{detail: detail}
	svc := NewService(repo, &fakeAuthService{
		permissions: []string{permissionPlatformCMSView, permissionCMSTemplateActivate},
	}, idgen.UUIDv7Generator{})
	return svc, repo
}

func baseActivateDetail(cfg *TemplateDeadlineConfig) *DisclosureTypeDTO {
	return &DisclosureTypeDTO{
		TypeID:             "dt-at-act",
		VersionNo:          2,
		Scope:              "global",
		TemplateCategory:   TemplateCategoryPeriodic,
		DeadlineRule:       "T+5",
		ApplicabilityRules: applicability.DefaultGlobalRules(true),
		DeadlineConfig:     cfg,
		Blocks: []TemplateBlockDTO{{
			BlockID: "block-workflow", BlockKey: "enterprise_workflow", BlockType: "rich_text",
			Config: map[string]any{
				"steps": []any{
					map[string]any{
						"step_id": "review", "stage": "Review", "department_id": "dept-finance",
						"assignee_role_ids": []any{"role-reviewer"}, "processing_days": float64(2),
						"display_order": float64(1), "documents": []any{},
					},
				},
			},
		}},
	}
}

func TestActivateTypeVersion_ApplicableToPastRejected(t *testing.T) {
	// Note: Activate uses time.Now().UTC() — use ApplicableTo far in the past.
	cfg := &TemplateDeadlineConfig{
		FrequencyUnit: "daily", ApplicableTo: "2020-01-01",
		ApplicableFromMode: ApplicableFromModeSpecific, ApplicableFromSlot: "2020-01-01",
		DeadlineDays: 5,
	}
	svc, repo := newActivateApplicableToService(baseActivateDetail(cfg))
	_, err := svc.ActivateTypeVersion(context.Background(), ActivateTypeVersionRequest{
		Subject: Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"},
		TypeID: "dt-at-act", VersionNo: 2,
	})
	if err == nil {
		t.Fatal("expected reject")
	}
	he, ok := err.(*perr.HTTPError)
	if !ok || he.HTTPStatus != http.StatusUnprocessableEntity || he.Code != perr.Code(ActivationBlockerApplicableToPast) {
		t.Fatalf("err=%v", err)
	}
	if repo.activated {
		t.Fatal("must not mutate active version on ApplicableTo failure")
	}
}

func TestActivateTypeVersion_ApplicableToRangeRejected(t *testing.T) {
	// Fixed SPECIFIC far future relative to To — independent of wall clock past-check if To is future.
	// Use To far future but first T after To via SPECIFIC slot.
	cfg := &TemplateDeadlineConfig{
		FrequencyUnit: "monthly", CycleAnchorDay: 30, DeadlineDays: 10,
		ApplicableTo: "2099-01-15",
		ApplicableFromMode: ApplicableFromModeSpecific, ApplicableFromSlot: "2099-01",
	}
	svc, repo := newActivateApplicableToService(baseActivateDetail(cfg))
	_, err := svc.ActivateTypeVersion(context.Background(), ActivateTypeVersionRequest{
		Subject: Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"},
		TypeID: "dt-at-act", VersionNo: 2,
	})
	if err == nil {
		t.Fatal("expected range reject")
	}
	he, ok := err.(*perr.HTTPError)
	if !ok || he.Code != perr.Code(ActivationBlockerApplicabilityRangeInvalid) {
		t.Fatalf("err=%v", err)
	}
	if repo.activated {
		t.Fatal("no activation mutation")
	}
}

func TestActivateTypeVersion_ApplicableToOpenEndedOK(t *testing.T) {
	cfg := &TemplateDeadlineConfig{
		FrequencyUnit: "monthly", CycleAnchorDay: 5, DeadlineDays: 10,
		ApplicableFromMode: ApplicableFromModeNext,
	}
	svc, repo := newActivateApplicableToService(baseActivateDetail(cfg))
	_, err := svc.ActivateTypeVersion(context.Background(), ActivateTypeVersionRequest{
		Subject: Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"},
		TypeID: "dt-at-act", VersionNo: 2,
	})
	if err != nil {
		t.Fatalf("open-ended activate: %v", err)
	}
	if !repo.activated {
		t.Fatal("want activate")
	}
}

func TestActivateTypeVersion_ApplicableToValidOK(t *testing.T) {
	cfg := &TemplateDeadlineConfig{
		FrequencyUnit: "monthly", CycleAnchorDay: 10, DeadlineDays: 10,
		ApplicableTo: "2099-12-31",
		ApplicableFromMode: ApplicableFromModeSpecific, ApplicableFromSlot: "2099-01",
	}
	svc, repo := newActivateApplicableToService(baseActivateDetail(cfg))
	_, err := svc.ActivateTypeVersion(context.Background(), ActivateTypeVersionRequest{
		Subject: Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"},
		TypeID: "dt-at-act", VersionNo: 2,
	})
	if err != nil {
		t.Fatalf("valid To activate: %v", err)
	}
	if !repo.activated {
		t.Fatal("want activate")
	}
}

func TestReadinessActivateParity_ApplicableToPast(t *testing.T) {
	loc := asiaHoChiMinh()
	eval := time.Date(2026, 9, 30, 10, 0, 0, 0, loc)
	cfg := &TemplateDeadlineConfig{FrequencyUnit: "daily", ApplicableTo: "2026-09-29", DeadlineDays: 5}
	item := readyPeriodicItem(cfg)
	applyActivationReadiness(item, eval, nil)
	if item.ActivationReady || !hasBlockerCode(item.ActivationBlockers, ActivationBlockerApplicableToPast) {
		t.Fatalf("readiness=%v", item.ActivationBlockers)
	}
	blockers := CollectApplicableToActivationBlockers(cfg, eval)
	if len(blockers) != 1 || blockers[0].Code != ActivationBlockerApplicableToPast {
		t.Fatalf("validator=%v", blockers)
	}
	// Activate path uses same collector → same code
	err := applicableToActivationHTTPError(blockers[0])
	he := err.(*perr.HTTPError)
	if string(he.Code) != ActivationBlockerApplicableToPast {
		t.Fatalf("activate code=%s", he.Code)
	}
}

func TestReadinessActivateParity_ApplicableToRange(t *testing.T) {
	loc := asiaHoChiMinh()
	eval := time.Date(2026, 9, 1, 10, 0, 0, 0, loc)
	cfg := &TemplateDeadlineConfig{
		FrequencyUnit: "monthly", CycleAnchorDay: 30, DeadlineDays: 10,
		ApplicableTo: "2026-09-15", ApplicableFromMode: ApplicableFromModeCurrent,
	}
	item := readyPeriodicItem(cfg)
	applyActivationReadiness(item, eval, nil)
	blockers := CollectApplicableToActivationBlockers(cfg, eval)
	if !hasBlockerCode(item.ActivationBlockers, ActivationBlockerApplicabilityRangeInvalid) {
		t.Fatalf("readiness=%v", item.ActivationBlockers)
	}
	if len(blockers) != 1 || blockers[0].Code != item.ActivationBlockers[0].Code {
		t.Fatalf("parity readiness=%v validator=%v", item.ActivationBlockers, blockers)
	}
}
