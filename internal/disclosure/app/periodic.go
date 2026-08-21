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
// Effective T = Company override ?? CMS active anchors (canonical ResolveEffectiveAnchor + ResolveOccurrenceT).
func seedPeriodicCycles(ctx context.Context, now time.Time, repo Repository, idg idgen.Generator, calc *DeadlineCalculator, strictApplicabilityFilter bool, shadow *deadlineEngineShadowRunner) (int, error) {
	types, err := repo.ListActivePeriodicTypes(ctx)
	if err != nil {
		return 0, fmt.Errorf("list periodic types: %w", err)
	}
	companyIDs, err := repo.ListAllActiveCompanyIDs(ctx)
	if err != nil {
		return 0, fmt.Errorf("list active companies: %w", err)
	}

	typeIDs := make([]string, 0, len(types))
	for _, t := range types {
		typeIDs = append(typeIDs, t.TypeID)
	}
	prefs, err := repo.ListCompanyTypePreferencesByTypeIDs(ctx, typeIDs)
	if err != nil {
		return 0, fmt.Errorf("list company preferences: %w", err)
	}
	prefByKey := make(map[string]CompanyTypePreference, len(prefs))
	for _, p := range prefs {
		prefByKey[p.CompanyID+"|"+p.TypeID] = p
	}

	loc := asiaHoChiMinh()
	seeded := 0
	for _, t := range types {
		label := ResolveLogicalSlot(t.FrequencyUnit, now, loc)
		cmsAnchor := AnchorConfig{Month: t.CycleAnchorMonth, Day: t.CycleAnchorDay}

		for _, companyID := range companyIDs {
			pref, hasPref := prefByKey[companyID+"|"+t.TypeID]
			if hasPref && !pref.AutoCreateEnabled {
				continue
			}
			profile, err := repo.GetCompanyApplicabilityProfile(ctx, companyID)
			if err != nil {
				continue
			}
			if t.IsGlobal && !applicability.IsApplicable(t.ApplicabilityRules, profile, strictApplicabilityFilter) {
				continue
			}

			companyAnchor := AnchorConfig{}
			if hasPref {
				companyAnchor = AnchorConfig{Month: pref.CycleAnchorMonth, Day: pref.CycleAnchorDay}
			}
			effAnchor, tSource := ResolveEffectiveAnchor(cmsAnchor, companyAnchor)
			cycleStart, err := ResolveOccurrenceT(t.FrequencyUnit, label, effAnchor, loc)
			if err != nil {
				slog.WarnContext(ctx, "periodic seed skip: resolve T",
					slog.String("type_id", t.TypeID),
					slog.String("company_id", companyID),
					slog.String("err", err.Error()))
				continue
			}
			_ = tSource

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
				continue
			}
			openAt := ResolveOpenAt(cycleStart, t.OpenDaysBeforeT)

			shadow.periodicWorker(ctx, companyID, t, profile, cycleStart, dueDate, now)
			if err := repo.UpsertPeriodicCycle(ctx, PeriodicCycleRow{
				CycleID:    idg.NewUUID(),
				TypeID:     t.TypeID,
				CompanyID:  companyID,
				CycleLabel: label,
				CycleStart: cycleStart,
				OpenAt:     openAt,
				DueDate:    dueDate,
			}); err != nil {
				continue
			}
			seeded++
		}
	}
	return seeded, nil
}

// materializePeriodicDisclosures picks pending cycles whose OpenAt <= now+buffer
// and creates disclosure records with workflow (no company submitted_at).
func materializePeriodicDisclosures(ctx context.Context, now time.Time, repo PeriodicMaterializeRepository, creator PeriodicRecordCreator) (int, error) {
	const bufferDays = 7 // MATERIALIZATION_LOOKAHEAD (technical)
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

// computeCycleLabelAndStart is the legacy helper used by unit tests; delegates to canonical resolver.
func computeCycleLabelAndStart(t PeriodicTypeRow, now time.Time) (label string, start time.Time) {
	loc := asiaHoChiMinh()
	label = ResolveLogicalSlot(t.FrequencyUnit, now, loc)
	anchor := AnchorConfig{Month: t.CycleAnchorMonth, Day: t.CycleAnchorDay}
	start, err := ResolveOccurrenceT(t.FrequencyUnit, label, anchor, loc)
	if err != nil {
		start = stripTime(now.In(loc))
	}
	return
}
