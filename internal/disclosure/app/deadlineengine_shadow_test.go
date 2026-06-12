package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
	"github.com/cobo/cobo_iam_services/internal/disclosure/app/deadlineengine"
)

// fakeShadowAdapter is a minimal DeadlineEngineAdapter test double — it never
// calls the real deadlineengine package, only records the input and returns a
// pre-configured Resolution/error.
type fakeShadowAdapter struct {
	res    deadlineengine.Resolution
	err    error
	called int
	last   DeadlineEngineAdapterInput
}

func (f *fakeShadowAdapter) ResolveDeadline(_ context.Context, in DeadlineEngineAdapterInput) (deadlineengine.Resolution, error) {
	f.called++
	f.last = in
	return f.res, f.err
}

func newBufferRunner(adapter DeadlineEngineAdapter, enabled bool) (*deadlineEngineShadowRunner, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(buf, nil))
	return &deadlineEngineShadowRunner{
		adapter: adapter,
		enabled: enabled,
		sampler: &shadowSampler{},
		logger:  logger,
	}, buf
}

func decodeShadowLog(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("decode shadow log: %v (raw=%q)", err, buf.String())
	}
	return m
}

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// ---------------------------------------------------------------------------
// shadowSampler / Phase F sampling
// ---------------------------------------------------------------------------

func TestShadowSampler_NoneSampledOneInN(t *testing.T) {
	s := &shadowSampler{}
	logged := 0
	for i := 0; i < 250; i++ {
		if s.shouldSample() {
			logged++
		}
	}
	// n%100==1 for n=1,101,201 -> 3 samples in 250 calls.
	if logged != 3 {
		t.Fatalf("expected 3 sampled events in 250 calls, got %d", logged)
	}
}

func TestShadowSampler_NilSamplerNeverSamples(t *testing.T) {
	var s *shadowSampler
	if s.shouldSample() {
		t.Fatal("nil sampler must never sample")
	}
}

// ---------------------------------------------------------------------------
// formatShadowDate / parseShadowDate
// ---------------------------------------------------------------------------

func TestFormatShadowDate(t *testing.T) {
	if got := formatShadowDate(time.Time{}); got != "" {
		t.Fatalf("zero time should format to empty string, got %q", got)
	}
	if got := formatShadowDate(date(2026, 6, 12)); got != "2026-06-12" {
		t.Fatalf("got %q", got)
	}
}

func TestParseShadowDate(t *testing.T) {
	if _, ok := parseShadowDate(nil); ok {
		t.Fatal("nil pointer should not parse")
	}
	empty := ""
	if _, ok := parseShadowDate(&empty); ok {
		t.Fatal("empty string should not parse")
	}
	invalid := "not-a-date"
	if _, ok := parseShadowDate(&invalid); ok {
		t.Fatal("invalid date should not parse")
	}
	valid := "2026-06-12"
	got, ok := parseShadowDate(&valid)
	if !ok || !got.Equal(date(2026, 6, 12)) {
		t.Fatalf("got %v, ok=%v", got, ok)
	}
}

// ---------------------------------------------------------------------------
// logDeadlineEngineShadow / Phase E format + Phase F sampling
// ---------------------------------------------------------------------------

func TestLogDeadlineEngineShadow_AlwaysLogsNonNone(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(buf, nil))
	sampler := &shadowSampler{}

	audit := DeadlineAudit{
		CompanyID:       "co-1",
		TypeID:          "type-1",
		OldCycleStart:   date(2026, 6, 1),
		NewCycleStart:   date(2026, 6, 5),
		OldDueDate:      date(2026, 6, 10),
		NewDueDate:      date(2026, 6, 14),
		DivergenceClass: DivergenceDateShift,
	}
	logDeadlineEngineShadow(context.Background(), logger, sampler, audit)

	m := decodeShadowLog(t, buf)
	if m["event"] != "deadline_engine_shadow" {
		t.Fatalf("event=%v", m["event"])
	}
	if m["company_id"] != "co-1" || m["type_id"] != "type-1" {
		t.Fatalf("identity fields: %#v", m)
	}
	if m["divergence_class"] != DivergenceDateShift {
		t.Fatalf("divergence_class=%v", m["divergence_class"])
	}
	if m["old_cycle_start"] != "2026-06-01" || m["new_cycle_start"] != "2026-06-05" {
		t.Fatalf("cycle_start fields: %#v", m)
	}
	if m["old_due_date"] != "2026-06-10" || m["new_due_date"] != "2026-06-14" {
		t.Fatalf("due_date fields: %#v", m)
	}
}

func TestLogDeadlineEngineShadow_NoneSampledOnlyFirst(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(buf, nil))
	sampler := &shadowSampler{}
	audit := DeadlineAudit{CompanyID: "co-1", TypeID: "type-1", DivergenceClass: DivergenceNone}

	logDeadlineEngineShadow(context.Background(), logger, sampler, audit) // n=1 -> sampled
	if buf.Len() == 0 {
		t.Fatal("expected first NONE event to be sampled/logged")
	}
	buf.Reset()
	logDeadlineEngineShadow(context.Background(), logger, sampler, audit) // n=2 -> not sampled
	if buf.Len() != 0 {
		t.Fatalf("expected second NONE event to be skipped, got %q", buf.String())
	}
}

func TestLogDeadlineEngineShadow_NilLoggerDoesNotPanic(t *testing.T) {
	sampler := &shadowSampler{}
	audit := DeadlineAudit{DivergenceClass: DivergenceYearlyRollback}
	logDeadlineEngineShadow(context.Background(), nil, sampler, audit)
}

// ---------------------------------------------------------------------------
// Phase A — Portal Preview shadow
// ---------------------------------------------------------------------------

func periodicConfig() *TemplateDeadlineConfig {
	return &TemplateDeadlineConfig{
		DeadlineMode:    DeadlineModePeriodic,
		FrequencyUnit:   "monthly",
		CycleAnchorDay:  5,
		AllowT0Override: boolPtr(false),
	}
}

func periodicRules() *applicability.TemplateApplicabilityRules {
	return &applicability.TemplateApplicabilityRules{DeadlineDays: 10, DeadlineDayType: "calendar"}
}

func TestPortalPreview_Success_LogsComparison(t *testing.T) {
	adapter := &fakeShadowAdapter{res: deadlineengine.Resolution{
		ResolvedT0:  date(2026, 6, 5),
		PlannedDate: date(2026, 6, 14),
	}}
	runner, buf := newBufferRunner(adapter, true)

	oldStart := "2026-06-01"
	oldDue := "2026-06-10"
	runner.portalPreview(context.Background(), "co-1", "type-1",
		periodicConfig(), periodicRules(), TemplateCategoryPeriodic,
		CompanyDeadlineContext{}, applicability.CompanyApplicabilityProfile{},
		&oldStart, &oldDue, time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC))

	if adapter.called != 1 {
		t.Fatalf("adapter.called=%d, want 1", adapter.called)
	}
	m := decodeShadowLog(t, buf)
	if m["divergence_class"] != DivergenceDateShift {
		t.Fatalf("divergence_class=%v, want %s", m["divergence_class"], DivergenceDateShift)
	}
	if m["old_cycle_start"] != "2026-06-01" || m["new_cycle_start"] != "2026-06-05" {
		t.Fatalf("cycle_start: %#v", m)
	}
}

func TestPortalPreview_Disabled_NoAdapterCallNoLog(t *testing.T) {
	adapter := &fakeShadowAdapter{}
	runner, buf := newBufferRunner(adapter, false)

	oldStart := "2026-06-01"
	oldDue := "2026-06-10"
	runner.portalPreview(context.Background(), "co-1", "type-1",
		periodicConfig(), periodicRules(), TemplateCategoryPeriodic,
		CompanyDeadlineContext{}, applicability.CompanyApplicabilityProfile{},
		&oldStart, &oldDue, time.Now())

	if adapter.called != 0 {
		t.Fatalf("adapter.called=%d, want 0", adapter.called)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected no log, got %q", buf.String())
	}
}

func TestPortalPreview_NilRunner_NoPanic(t *testing.T) {
	var runner *deadlineEngineShadowRunner
	oldStart := "2026-06-01"
	oldDue := "2026-06-10"
	runner.portalPreview(context.Background(), "co-1", "type-1",
		periodicConfig(), periodicRules(), TemplateCategoryPeriodic,
		CompanyDeadlineContext{}, applicability.CompanyApplicabilityProfile{},
		&oldStart, &oldDue, time.Now())
}

func TestPortalPreview_NonPeriodicSkipped(t *testing.T) {
	adapter := &fakeShadowAdapter{}
	runner, buf := newBufferRunner(adapter, true)

	cfg := &TemplateDeadlineConfig{DeadlineMode: DeadlineModeFixedDate}
	oldStart := "2026-06-01"
	oldDue := "2026-06-10"
	runner.portalPreview(context.Background(), "co-1", "type-1",
		cfg, periodicRules(), TemplateCategoryIrregular,
		CompanyDeadlineContext{}, applicability.CompanyApplicabilityProfile{},
		&oldStart, &oldDue, time.Now())

	if adapter.called != 0 || buf.Len() != 0 {
		t.Fatalf("expected skip for non-periodic config, called=%d log=%q", adapter.called, buf.String())
	}
}

func TestPortalPreview_NilRulesSkipped(t *testing.T) {
	adapter := &fakeShadowAdapter{}
	runner, buf := newBufferRunner(adapter, true)

	oldStart := "2026-06-01"
	oldDue := "2026-06-10"
	runner.portalPreview(context.Background(), "co-1", "type-1",
		periodicConfig(), nil, TemplateCategoryPeriodic,
		CompanyDeadlineContext{}, applicability.CompanyApplicabilityProfile{},
		&oldStart, &oldDue, time.Now())

	if adapter.called != 0 || buf.Len() != 0 {
		t.Fatalf("expected skip for nil rules, called=%d log=%q", adapter.called, buf.String())
	}
}

func TestPortalPreview_InvalidOldDatesSkipped(t *testing.T) {
	adapter := &fakeShadowAdapter{}
	runner, buf := newBufferRunner(adapter, true)

	runner.portalPreview(context.Background(), "co-1", "type-1",
		periodicConfig(), periodicRules(), TemplateCategoryPeriodic,
		CompanyDeadlineContext{}, applicability.CompanyApplicabilityProfile{},
		nil, nil, time.Now())

	if adapter.called != 0 || buf.Len() != 0 {
		t.Fatalf("expected skip for nil old dates, called=%d log=%q", adapter.called, buf.String())
	}
}

func TestPortalPreview_AdapterErrorSwallowed(t *testing.T) {
	adapter := &fakeShadowAdapter{err: errors.New("boom")}
	runner, buf := newBufferRunner(adapter, true)

	oldStart := "2026-06-01"
	oldDue := "2026-06-10"
	runner.portalPreview(context.Background(), "co-1", "type-1",
		periodicConfig(), periodicRules(), TemplateCategoryPeriodic,
		CompanyDeadlineContext{}, applicability.CompanyApplicabilityProfile{},
		&oldStart, &oldDue, time.Now())

	if adapter.called != 1 {
		t.Fatalf("adapter.called=%d, want 1", adapter.called)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected no deadline_engine_shadow log on adapter error, got %q", buf.String())
	}
}

// ---------------------------------------------------------------------------
// Phase B — Periodic Worker shadow
// ---------------------------------------------------------------------------

func TestPeriodicWorker_Success_LogsComparison(t *testing.T) {
	adapter := &fakeShadowAdapter{res: deadlineengine.Resolution{
		ResolvedT0:  date(2026, 6, 5),
		PlannedDate: date(2026, 6, 15),
	}}
	runner, buf := newBufferRunner(adapter, true)

	row := PeriodicTypeRow{
		TypeID:         "type-1",
		FrequencyUnit:  "monthly",
		CycleAnchorDay: 5,
		DeadlineDays:   10,
	}
	oldCycleStart := date(2026, 6, 1) // Source B: always day 1
	oldDueDate := date(2026, 6, 10)

	runner.periodicWorker(context.Background(), "co-1", row,
		applicability.CompanyApplicabilityProfile{}, oldCycleStart, oldDueDate, time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC))

	if adapter.called != 1 {
		t.Fatalf("adapter.called=%d, want 1", adapter.called)
	}
	if adapter.last.Rules == nil || adapter.last.Rules.DeadlineDays != 10 || adapter.last.Rules.DeadlineDayType != "working" {
		t.Fatalf("expected default rules (deadline_days=10, working), got %#v", adapter.last.Rules)
	}
	if adapter.last.Config.FrequencyUnit != "monthly" || adapter.last.Config.CycleAnchorDay != 5 {
		t.Fatalf("expected config mapped from PeriodicTypeRow, got %#v", adapter.last.Config)
	}

	m := decodeShadowLog(t, buf)
	if m["divergence_class"] != DivergenceDateShift {
		t.Fatalf("divergence_class=%v, want %s", m["divergence_class"], DivergenceDateShift)
	}
	if m["old_cycle_start"] != "2026-06-01" || m["new_cycle_start"] != "2026-06-05" {
		t.Fatalf("cycle_start: %#v", m)
	}
	if m["old_due_date"] != "2026-06-10" || m["new_due_date"] != "2026-06-15" {
		t.Fatalf("due_date: %#v", m)
	}
}

func TestPeriodicWorker_UsesProvidedApplicabilityRules(t *testing.T) {
	adapter := &fakeShadowAdapter{res: deadlineengine.Resolution{ResolvedT0: date(2026, 6, 1), PlannedDate: date(2026, 6, 1)}}
	runner, _ := newBufferRunner(adapter, true)

	rules := &applicability.TemplateApplicabilityRules{DeadlineDays: 30, DeadlineDayType: "calendar"}
	row := PeriodicTypeRow{TypeID: "type-1", FrequencyUnit: "monthly", CycleAnchorDay: 1, DeadlineDays: 10, ApplicabilityRules: rules}

	runner.periodicWorker(context.Background(), "co-1", row,
		applicability.CompanyApplicabilityProfile{}, date(2026, 6, 1), date(2026, 6, 1), time.Now())

	if adapter.last.Rules != rules {
		t.Fatalf("expected PeriodicTypeRow.ApplicabilityRules passed through, got %#v", adapter.last.Rules)
	}
}

func TestPeriodicWorker_Disabled_NoAdapterCallNoLog(t *testing.T) {
	adapter := &fakeShadowAdapter{}
	runner, buf := newBufferRunner(adapter, false)

	row := PeriodicTypeRow{TypeID: "type-1", FrequencyUnit: "monthly", CycleAnchorDay: 5, DeadlineDays: 10}
	runner.periodicWorker(context.Background(), "co-1", row, applicability.CompanyApplicabilityProfile{}, date(2026, 6, 1), date(2026, 6, 10), time.Now())

	if adapter.called != 0 || buf.Len() != 0 {
		t.Fatalf("expected skip when disabled, called=%d log=%q", adapter.called, buf.String())
	}
}

func TestPeriodicWorker_NilRunner_NoPanic(t *testing.T) {
	var runner *deadlineEngineShadowRunner
	row := PeriodicTypeRow{TypeID: "type-1", FrequencyUnit: "monthly", CycleAnchorDay: 5, DeadlineDays: 10}
	runner.periodicWorker(context.Background(), "co-1", row, applicability.CompanyApplicabilityProfile{}, date(2026, 6, 1), date(2026, 6, 10), time.Now())
}

func TestPeriodicWorker_AdapterErrorSwallowed(t *testing.T) {
	adapter := &fakeShadowAdapter{err: errors.New("boom")}
	runner, buf := newBufferRunner(adapter, true)

	row := PeriodicTypeRow{TypeID: "type-1", FrequencyUnit: "monthly", CycleAnchorDay: 5, DeadlineDays: 10}
	runner.periodicWorker(context.Background(), "co-1", row, applicability.CompanyApplicabilityProfile{}, date(2026, 6, 1), date(2026, 6, 10), time.Now())

	if adapter.called != 1 {
		t.Fatalf("adapter.called=%d, want 1", adapter.called)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected no deadline_engine_shadow log on adapter error, got %q", buf.String())
	}
}

// ---------------------------------------------------------------------------
// Phase C — Manual Create shadow
// ---------------------------------------------------------------------------

func TestManualCreate_Success_LogsDueDateDivergence(t *testing.T) {
	adapter := &fakeShadowAdapter{res: deadlineengine.Resolution{
		ResolvedT0:  date(2026, 6, 5),
		PlannedDate: date(2026, 6, 15),
	}}
	runner, buf := newBufferRunner(adapter, true)

	runner.manualCreate(context.Background(), "co-1", "type-1",
		periodicConfig(), periodicRules(), TemplateCategoryPeriodic,
		CompanyDeadlineContext{}, applicability.CompanyApplicabilityProfile{},
		"2026-06-10", time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC))

	if adapter.called != 1 {
		t.Fatalf("adapter.called=%d, want 1", adapter.called)
	}
	m := decodeShadowLog(t, buf)
	if m["divergence_class"] != DivergenceDateShift {
		t.Fatalf("divergence_class=%v, want %s", m["divergence_class"], DivergenceDateShift)
	}
	if m["old_due_date"] != "2026-06-10" || m["new_due_date"] != "2026-06-15" {
		t.Fatalf("due_date: %#v", m)
	}
	if m["old_cycle_start"] != "2026-06-05" || m["new_cycle_start"] != "2026-06-05" {
		t.Fatalf("cycle_start (engine T0, both sides) = %#v", m)
	}
}

func TestManualCreate_EmptyPlannedDateSkipped(t *testing.T) {
	adapter := &fakeShadowAdapter{}
	runner, buf := newBufferRunner(adapter, true)

	runner.manualCreate(context.Background(), "co-1", "type-1",
		periodicConfig(), periodicRules(), TemplateCategoryPeriodic,
		CompanyDeadlineContext{}, applicability.CompanyApplicabilityProfile{},
		"", time.Now())

	if adapter.called != 0 || buf.Len() != 0 {
		t.Fatalf("expected skip for empty planned_date, called=%d log=%q", adapter.called, buf.String())
	}
}

func TestManualCreate_NonPeriodicSkipped(t *testing.T) {
	adapter := &fakeShadowAdapter{}
	runner, buf := newBufferRunner(adapter, true)

	cfg := &TemplateDeadlineConfig{DeadlineMode: DeadlineModeFixedDate}
	runner.manualCreate(context.Background(), "co-1", "type-1",
		cfg, periodicRules(), TemplateCategoryIrregular,
		CompanyDeadlineContext{}, applicability.CompanyApplicabilityProfile{},
		"2026-06-10", time.Now())

	if adapter.called != 0 || buf.Len() != 0 {
		t.Fatalf("expected skip for non-periodic config, called=%d log=%q", adapter.called, buf.String())
	}
}

func TestManualCreate_Disabled_NoAdapterCallNoLog(t *testing.T) {
	adapter := &fakeShadowAdapter{}
	runner, buf := newBufferRunner(adapter, false)

	runner.manualCreate(context.Background(), "co-1", "type-1",
		periodicConfig(), periodicRules(), TemplateCategoryPeriodic,
		CompanyDeadlineContext{}, applicability.CompanyApplicabilityProfile{},
		"2026-06-10", time.Now())

	if adapter.called != 0 || buf.Len() != 0 {
		t.Fatalf("expected skip when disabled, called=%d log=%q", adapter.called, buf.String())
	}
}

func TestManualCreate_NilRunner_NoPanic(t *testing.T) {
	var runner *deadlineEngineShadowRunner
	runner.manualCreate(context.Background(), "co-1", "type-1",
		periodicConfig(), periodicRules(), TemplateCategoryPeriodic,
		CompanyDeadlineContext{}, applicability.CompanyApplicabilityProfile{},
		"2026-06-10", time.Now())
}

func TestManualCreate_AdapterErrorSwallowed(t *testing.T) {
	adapter := &fakeShadowAdapter{err: errors.New("boom")}
	runner, buf := newBufferRunner(adapter, true)

	runner.manualCreate(context.Background(), "co-1", "type-1",
		periodicConfig(), periodicRules(), TemplateCategoryPeriodic,
		CompanyDeadlineContext{}, applicability.CompanyApplicabilityProfile{},
		"2026-06-10", time.Now())

	if adapter.called != 1 {
		t.Fatalf("adapter.called=%d, want 1", adapter.called)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected no deadline_engine_shadow log on adapter error, got %q", buf.String())
	}
}

func TestManualCreate_InvalidPlannedDateSkipped(t *testing.T) {
	adapter := &fakeShadowAdapter{}
	runner, buf := newBufferRunner(adapter, true)

	runner.manualCreate(context.Background(), "co-1", "type-1",
		periodicConfig(), periodicRules(), TemplateCategoryPeriodic,
		CompanyDeadlineContext{}, applicability.CompanyApplicabilityProfile{},
		"not-a-date", time.Now())

	if adapter.called != 0 || buf.Len() != 0 {
		t.Fatalf("expected skip for invalid planned_date, called=%d log=%q", adapter.called, buf.String())
	}
}

// ---------------------------------------------------------------------------
// newDeadlineEngineShadowRunner
// ---------------------------------------------------------------------------

func TestNewDeadlineEngineShadowRunner_NilAdapterForcesDisabled(t *testing.T) {
	r := newDeadlineEngineShadowRunner(nil, true)
	if r.enabled {
		t.Fatal("expected nil adapter to force enabled=false")
	}
}

func TestNewDeadlineEngineShadowRunner_EnabledWithAdapter(t *testing.T) {
	adapter := NewDeadlineEngineAdapter(nil)
	r := newDeadlineEngineShadowRunner(adapter, true)
	if !r.enabled {
		t.Fatal("expected enabled=true with non-nil adapter and enabled=true")
	}
}
