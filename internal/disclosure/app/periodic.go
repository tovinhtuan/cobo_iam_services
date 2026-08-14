package app

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
	workflowerrs "github.com/cobo/cobo_iam_services/internal/workflow/errs"
)

// seedPeriodicCycles computes expected cycles for the current tick and upserts them.
// Returns the number of new cycle slots inserted (duplicates are silently ignored).
//
// READY_FOR_5B (Periodic Worker): computeCycleLabelAndStart below is Source B
// (legacy, anchor-blind cycle_start) — the locked Cycle Start SoT is Source C
// (deadlineengine.ResolveT0, see deadline-engine-cycle-start-sot-review-2026-06-12.md).
// Batch 5B added a shadow=Source C compute (see deadlineengine_shadow.go,
// shadow.periodicWorker below) for comparison/audit only — the seeded
// cycle_start/due_date below remain Source B, unchanged. 5B/5E cutover
// replaces the computeCycleLabelAndStart call with
// DeadlineEngineAdapter.ResolveDeadline (cycleCtx=nil for new seeds, per I-21);
// cycle_label may remain derived from this function's calendar-period logic.
func seedPeriodicCycles(ctx context.Context, now time.Time, repo Repository, idg idgen.Generator, calc *DeadlineCalculator, strictApplicabilityFilter bool, shadow *deadlineEngineShadowRunner) (int, error) {
	types, err := repo.ListActivePeriodicTypes(ctx)
	if err != nil {
		return 0, fmt.Errorf("list periodic types: %w", err)
	}
	companyIDs, err := repo.ListAllActiveCompanyIDs(ctx)
	if err != nil {
		return 0, fmt.Errorf("list active companies: %w", err)
	}

	seeded := 0
	for _, t := range types {
		for _, companyID := range companyIDs {
			pref, _ := repo.GetCompanyTypePreference(ctx, companyID, t.TypeID)
			if pref != nil && !pref.AutoCreateEnabled {
				continue
			}
			profile, err := repo.GetCompanyApplicabilityProfile(ctx, companyID)
			if err != nil {
				continue
			}
			if t.IsGlobal && !applicability.IsApplicable(t.ApplicabilityRules, profile, strictApplicabilityFilter) {
				continue
			}
			label, cycleStart := computeCycleLabelAndStart(t, now)
			deadlineDays := t.DeadlineDays
			durationType := DurationTypeCalendarDays
			if t.ApplicabilityRules != nil {
				if days, ok := applicability.ResolveDeadlineDays(t.ApplicabilityRules, profile); ok {
					deadlineDays = days
				}
				durationType = applicability.ResolveDeadlineDurationType(t.ApplicabilityRules)
			} else if deadlineDays > 0 {
				durationType = DurationTypeWorkingDays
			}
			dueDate, err := calc.addDurationInclusive(ctx, cycleStart, deadlineDays, durationType)
			if err != nil {
				// log-only: don't fail entire batch for one bad config
				continue
			}
			// Batch 5B Phase B (shadow only, see deadlineengine_shadow.go):
			// compares Source B (cycleStart/dueDate above, persisted below)
			// against Source C for audit. Never written to periodic_cycles.
			shadow.periodicWorker(ctx, companyID, t, profile, cycleStart, dueDate, now)
			if err := repo.UpsertPeriodicCycle(ctx, PeriodicCycleRow{
				CycleID:    idg.NewUUID(),
				TypeID:     t.TypeID,
				CompanyID:  companyID,
				CycleLabel: label,
				CycleStart: cycleStart,
				DueDate:    dueDate,
			}); err != nil {
				continue
			}
			seeded++
		}
	}
	return seeded, nil
}

// materializePeriodicDisclosures picks up pending cycles whose due_date <= now+buffer
// and creates the actual disclosure records with workflow (WF-A).
func materializePeriodicDisclosures(ctx context.Context, now time.Time, repo PeriodicMaterializeRepository, creator PeriodicRecordCreator) (int, error) {
	const bufferDays = 7
	cycles, err := repo.ListPendingCycles(ctx, now, bufferDays)
	if err != nil {
		return 0, fmt.Errorf("list pending cycles: %w", err)
	}
	materialized := 0
	for _, c := range cycles {
		if c.CycleStart.IsZero() {
			slog.WarnContext(ctx, "periodic cycle missing cycle_start; skip materialize",
				slog.String("cycle_id", c.CycleID))
			continue
		}
		claimed, err := repo.TryClaimPeriodicCycle(ctx, c.CycleID)
		if err != nil {
			return materialized, fmt.Errorf("claim periodic cycle %s: %w", c.CycleID, err)
		}
		if !claimed {
			continue
		}
		t0 := c.CycleStart
		plannedDate := ""
		if !c.DueDate.IsZero() {
			plannedDate = c.DueDate.Format("2006-01-02")
		}
		recordID, workflowInstanceID, err := creator.CreateAndSubmitRecordWithPlannedDate(ctx, c.CompanyID, c.TypeID, "m_system_worker", autoRecordTitle(c), &t0, plannedDate)
		if err != nil {
			_ = repo.ReleasePeriodicCycleClaim(ctx, c.CycleID)
			if workflowerrs.IsEmptyEffectiveWorkflow(err) {
				slog.WarnContext(ctx, "periodic materialize skipped: empty effective workflow",
					slog.String("cycle_id", c.CycleID),
					slog.String("type_id", c.TypeID),
					slog.String("company_id", c.CompanyID))
			}
			continue
		}
		if recordID == "" || workflowInstanceID == "" {
			_ = repo.ReleasePeriodicCycleClaim(ctx, c.CycleID)
			slog.WarnContext(ctx, "periodic materialize failed: missing record or workflow instance",
				slog.String("cycle_id", c.CycleID),
				slog.String("record_id", recordID),
				slog.String("workflow_instance_id", workflowInstanceID))
			continue
		}
		if err := repo.UpdateCycleRecord(ctx, c.CycleID, recordID); err != nil {
			_ = repo.ReleasePeriodicCycleClaim(ctx, c.CycleID)
			return materialized, fmt.Errorf("complete periodic cycle %s: %w", c.CycleID, err)
		}
		materialized++
	}
	return materialized, nil
}

// computeCycleLabelAndStart returns the unique cycle_label and the cycle start date
// for a given template type and reference time.
func computeCycleLabelAndStart(t PeriodicTypeRow, now time.Time) (label string, start time.Time) {
	switch NormalizeFrequencyUnit(t.FrequencyUnit) {
	case PeriodicityDaily:
		ict := now.In(asiaHoChiMinh())
		start = time.Date(ict.Year(), ict.Month(), ict.Day(), 0, 0, 0, 0, ict.Location())
		label = start.Format("2006-01-02")
	case PeriodicityWeekly:
		ict := now.In(asiaHoChiMinh())
		start = weekStartSunday(ict)
		label = start.Format("2006-01-02")
	case PeriodicityMonthly:
		label = now.Format("2006-01")
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	case PeriodicityQuarterly:
		q := (int(now.Month())-1)/3 + 1
		startMonth := time.Month((q-1)*3 + 1)
		label = fmt.Sprintf("%d-Q%d", now.Year(), q)
		start = time.Date(now.Year(), startMonth, 1, 0, 0, 0, 0, now.Location())
	case PeriodicityYearly:
		anchorMonth := t.CycleAnchorMonth
		if anchorMonth <= 0 || anchorMonth > 12 {
			anchorMonth = 1
		}
		anchorDay := t.CycleAnchorDay
		if anchorDay <= 0 || anchorDay > 31 {
			anchorDay = 1
		}
		label = fmt.Sprintf("%d", now.Year())
		start = time.Date(now.Year(), time.Month(anchorMonth), anchorDay, 0, 0, 0, 0, now.Location())
	default:
		label = now.Format("2006-01-02")
		start = now
	}
	return
}

func autoRecordTitle(c PeriodicCycleRow) string {
	name := strings.TrimSpace(c.TypeName)
	if name == "" {
		name = strings.TrimSpace(c.TypeID)
	}
	cycle := strings.TrimSpace(c.CycleLabel)
	if cycle == "" {
		return name
	}
	return fmt.Sprintf("%s — %s", name, cycle)
}
