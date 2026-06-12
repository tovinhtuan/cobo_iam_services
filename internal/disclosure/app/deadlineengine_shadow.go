package app

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
)

// deadlineEngineShadowNoneSampleRate: when DivergenceClass==NONE, only 1 in N
// shadow events is logged (Phase F sampling). DATE_SHIFT/MONTH_SHIFT/
// YEARLY_ROLLBACK are always logged.
const deadlineEngineShadowNoneSampleRate = 100

// shadowSampler implements the Phase F sampling rule for NONE-divergence
// shadow events. Not safe to copy after first use (embeds atomic.Int64) —
// always reference via pointer.
type shadowSampler struct {
	counter atomic.Int64
}

// shouldSample reports whether this NONE event should be logged (1-in-N,
// deterministic).
func (s *shadowSampler) shouldSample() bool {
	if s == nil {
		return false
	}
	n := s.counter.Add(1)
	return n%deadlineEngineShadowNoneSampleRate == 1
}

// formatShadowDate renders a time.Time as YYYY-MM-DD for shadow logs, or ""
// for the zero value (e.g. Manual Create has no "old cycle_start").
func formatShadowDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

// parseShadowDate parses a YYYY-MM-DD date string. Empty or nil input returns
// the zero time and ok=false.
func parseShadowDate(s *string) (time.Time, bool) {
	if s == nil || *s == "" {
		return time.Time{}, false
	}
	t, err := time.Parse("2006-01-02", *s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// logDeadlineEngineShadow emits a single deadline_engine_shadow log line per
// Phase E (format) and Phase F (sampling). Logging only — no DB write, no
// event publish, no metric emission.
func logDeadlineEngineShadow(ctx context.Context, logger *slog.Logger, sampler *shadowSampler, audit DeadlineAudit) {
	if audit.DivergenceClass == DivergenceNone && !sampler.shouldSample() {
		return
	}
	if logger == nil {
		logger = slog.Default()
	}
	logger.InfoContext(ctx, "deadline_engine_shadow",
		slog.String("event", "deadline_engine_shadow"),
		slog.String("company_id", audit.CompanyID),
		slog.String("type_id", audit.TypeID),
		slog.String("divergence_class", audit.DivergenceClass),
		slog.String("old_cycle_start", formatShadowDate(audit.OldCycleStart)),
		slog.String("new_cycle_start", formatShadowDate(audit.NewCycleStart)),
		slog.String("old_due_date", formatShadowDate(audit.OldDueDate)),
		slog.String("new_due_date", formatShadowDate(audit.NewDueDate)),
	)
}

// deadlineEngineShadowRunner is the Batch 5B shadow-compute integration
// point: Old Runtime continues to write DB as today; this runner additionally
// calls DeadlineEngineAdapter.ResolveDeadline (Source C) for comparison only
// and logs a DeadlineAudit. It never returns a value used by the caller and
// never errors out to the caller — any adapter error is logged and swallowed
// (shadow compute must not affect the old runtime path).
//
// enabled is controlled by DEADLINE_ENGINE_V2_SHADOW (default false). When
// disabled, all methods are no-ops with zero extra adapter/repo calls.
type deadlineEngineShadowRunner struct {
	adapter DeadlineEngineAdapter
	enabled bool
	sampler *shadowSampler
	logger  *slog.Logger
}

// newDeadlineEngineShadowRunner builds a shadow runner. adapter may be nil
// (treated as disabled regardless of enabled).
func newDeadlineEngineShadowRunner(adapter DeadlineEngineAdapter, enabled bool) *deadlineEngineShadowRunner {
	return &deadlineEngineShadowRunner{
		adapter: adapter,
		enabled: enabled && adapter != nil,
		sampler: &shadowSampler{},
	}
}

// portalPreview (Phase A): shadow-compute a DeadlineEngineV2 resolution for a
// DeadlineModePeriodic template and compare it against the
// DeadlineCalculator (Source A) result already computed for the API
// response. The API response (oldStartDate/oldDueDate) is NOT modified.
func (r *deadlineEngineShadowRunner) portalPreview(
	ctx context.Context,
	companyID, typeID string,
	config *TemplateDeadlineConfig,
	rules *applicability.TemplateApplicabilityRules,
	templateCategory string,
	company CompanyDeadlineContext,
	profile applicability.CompanyApplicabilityProfile,
	oldStartDate, oldDueDate *string,
	refTime time.Time,
) {
	if r == nil || !r.enabled {
		return
	}
	if config == nil || config.DeadlineMode != DeadlineModePeriodic || rules == nil {
		return
	}
	oldCycleStart, ok := parseShadowDate(oldStartDate)
	if !ok {
		return
	}
	oldDue, ok := parseShadowDate(oldDueDate)
	if !ok {
		return
	}

	res, err := r.adapter.ResolveDeadline(ctx, DeadlineEngineAdapterInput{
		TemplateCategory: templateCategory,
		Config:           config,
		Rules:            rules,
		Company:          company,
		CompanyProfile:   profile,
		RefTime:          refTime,
	})
	if err != nil {
		slog.WarnContext(ctx, "deadline_engine_shadow_error",
			slog.String("phase", "portal_preview"),
			slog.String("company_id", companyID),
			slog.String("type_id", typeID),
			slog.String("error", err.Error()))
		return
	}

	cmp := NewDeadlineComparison(oldCycleStart, res.ResolvedT0, oldDue, res.PlannedDate)
	logDeadlineEngineShadow(ctx, r.logger, r.sampler, NewDeadlineAudit(companyID, typeID, cmp))
}

// periodicWorker (Phase B): shadow-compute a DeadlineEngineV2 resolution for
// a periodic type's seed tick and compare it against Source B
// (computeCycleLabelAndStart, already computed by the caller). DB write
// continues to use oldCycleStart/oldDueDate (Source B) — newResolution is
// never persisted.
//
// CompanyDeadlineContext is intentionally zero-value here: Source B
// (computeCycleLabelAndStart) is anchor-blind and never applies a company
// cycle_anchor_* override, so the comparison isolates the Source B vs
// Source C algorithm divergence (RK-E09) without confounding it with R-C.
func (r *deadlineEngineShadowRunner) periodicWorker(
	ctx context.Context,
	companyID string,
	t PeriodicTypeRow,
	profile applicability.CompanyApplicabilityProfile,
	oldCycleStart, oldDueDate time.Time,
	refTime time.Time,
) {
	if r == nil || !r.enabled {
		return
	}

	config := &TemplateDeadlineConfig{
		DeadlineMode:     DeadlineModePeriodic,
		FrequencyUnit:    t.FrequencyUnit,
		CycleAnchorMonth: t.CycleAnchorMonth,
		CycleAnchorDay:   t.CycleAnchorDay,
	}
	rules := t.ApplicabilityRules
	if rules == nil {
		// Mirrors seedPeriodicCycles' default when no applicability rules are
		// configured: DeadlineDays from PeriodicTypeRow, working days (Source
		// B's default durationType).
		rules = &applicability.TemplateApplicabilityRules{
			DeadlineDays:    t.DeadlineDays,
			DeadlineDayType: "working",
		}
	}

	res, err := r.adapter.ResolveDeadline(ctx, DeadlineEngineAdapterInput{
		TemplateCategory: TemplateCategoryPeriodic,
		Config:           config,
		Rules:            rules,
		CompanyProfile:   profile,
		RefTime:          refTime,
	})
	if err != nil {
		slog.WarnContext(ctx, "deadline_engine_shadow_error",
			slog.String("phase", "periodic_worker"),
			slog.String("company_id", companyID),
			slog.String("type_id", t.TypeID),
			slog.String("error", err.Error()))
		return
	}

	cmp := NewDeadlineComparison(oldCycleStart, res.ResolvedT0, oldDueDate, res.PlannedDate)
	logDeadlineEngineShadow(ctx, r.logger, r.sampler, NewDeadlineAudit(companyID, t.TypeID, cmp))
}

// manualCreate (Phase C): shadow-compute a DeadlineEngineV2 resolution for a
// just-created record's type and compare its PlannedDate against the
// client-supplied planned_date. The request/record are not mutated — this
// runs after the record has already been persisted with the client value.
//
// There is no "old cycle_start" on this path (the client never supplies one),
// so DivergenceClass is classified on due-date divergence (client-supplied
// planned_date vs the engine's resolved PlannedDate) rather than cycle_start
// divergence — OldCycleStart/NewCycleStart are both set to the engine's
// ResolvedT0 for log context only (always equal, never drive the class).
func (r *deadlineEngineShadowRunner) manualCreate(
	ctx context.Context,
	companyID, typeID string,
	config *TemplateDeadlineConfig,
	rules *applicability.TemplateApplicabilityRules,
	templateCategory string,
	company CompanyDeadlineContext,
	profile applicability.CompanyApplicabilityProfile,
	clientPlannedDate string,
	refTime time.Time,
) {
	if r == nil || !r.enabled {
		return
	}
	if config == nil || config.DeadlineMode != DeadlineModePeriodic || rules == nil {
		return
	}
	if clientPlannedDate == "" {
		return
	}
	oldDue, ok := parseShadowDate(&clientPlannedDate)
	if !ok {
		return
	}

	res, err := r.adapter.ResolveDeadline(ctx, DeadlineEngineAdapterInput{
		TemplateCategory: templateCategory,
		Config:           config,
		Rules:            rules,
		Company:          company,
		CompanyProfile:   profile,
		RefTime:          refTime,
	})
	if err != nil {
		slog.WarnContext(ctx, "deadline_engine_shadow_error",
			slog.String("phase", "manual_create"),
			slog.String("company_id", companyID),
			slog.String("type_id", typeID),
			slog.String("error", err.Error()))
		return
	}

	audit := DeadlineAudit{
		CompanyID:       companyID,
		TypeID:          typeID,
		OldCycleStart:   res.ResolvedT0,
		NewCycleStart:   res.ResolvedT0,
		OldDueDate:      oldDue,
		NewDueDate:      res.PlannedDate,
		DivergenceClass: ClassifyDivergence(oldDue, res.PlannedDate),
	}
	logDeadlineEngineShadow(ctx, r.logger, r.sampler, audit)
}
