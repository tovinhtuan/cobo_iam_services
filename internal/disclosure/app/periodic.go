package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
	workflowerrs "github.com/cobo/cobo_iam_services/internal/workflow/errs"
)

// seedPeriodicCycles computes expected cycles for the current tick and upserts them.
// Effective T = Company override ?? CMS active anchors (canonical ResolveEffectiveAnchor + ResolveOccurrenceT).
//
// CURRENT_SLOT path is unchanged (AF gate on current). When PeriodicCycleGenerationLeadDays > 0,
// also seeds the next applicable logical slot iff TodayHCM >= GenerateAt(company Effective T, lead).
// Does not create disclosure_record / workflow at seed time.
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
	todayHCM := stripTime(now.In(loc))
	seeded := 0
	for _, t := range types {
		label := ResolveLogicalSlot(t.FrequencyUnit, now, loc)
		eligible, decision, afErr := EvaluateApplicableFromEligibility(
			t.FrequencyUnit, label, t.ApplicableFromMode, t.ApplicableFromSlot,
		)
		if afErr != nil {
			slog.WarnContext(ctx, "periodic seed skip: applicable_from invalid",
				slog.String("type_id", t.TypeID),
				slog.String("frequency_unit", t.FrequencyUnit),
				slog.String("candidate_slot", label),
				slog.String("applicable_from_slot", t.ApplicableFromSlot),
				slog.String("decision", decision),
				slog.String("err", afErr.Error()))
			continue
		}
		cmsAnchor := AnchorConfig{
			Month:          t.CycleAnchorMonth,
			Day:            t.CycleAnchorDay,
			Weekday:        t.CycleAnchorWeekday,
			MonthInQuarter: t.MonthInQuarter,
		}

		// CURRENT_SLOT seeding: AF gate unchanged — skip current when ineligible,
		// but do not abort the type (future pregen may still apply).
		if !eligible {
			slog.DebugContext(ctx, "periodic seed skip: before applicable_from",
				slog.String("type_id", t.TypeID),
				slog.String("frequency_unit", t.FrequencyUnit),
				slog.String("candidate_slot", label),
				slog.String("applicable_from_slot", t.ApplicableFromSlot),
				slog.String("decision", decision))
		} else {
			for _, companyID := range companyIDs {
				if ok := seedOneCompanySlot(ctx, now, repo, idg, calc, strictApplicabilityFilter, shadow, t, companyID, label, cmsAnchor, prefByKey, loc); ok {
					seeded++
				}
			}
		}

		lead := t.PeriodicCycleGenerationLeadDays
		if lead <= 0 {
			continue
		}
		nextLabel, nextErr := ResolveNextApplicableLogicalSlot(
			t.FrequencyUnit, label,
			t.ApplicableFromMode, t.ApplicableFromSlot, t.ApplicableTo,
			cmsAnchor, loc,
		)
		if nextErr != nil {
			slog.WarnContext(ctx, "periodic future pregen skip: next applicable resolve",
				slog.String("type_id", t.TypeID),
				slog.String("err", nextErr.Error()))
			continue
		}
		if strings.TrimSpace(nextLabel) == "" {
			continue
		}
		for _, companyID := range companyIDs {
			if !periodicTypeAppliesToCompany(t, companyID) {
				continue
			}
			pref, hasPref := prefByKey[companyID+"|"+t.TypeID]
			if hasPref && !pref.AutoCreateEnabled {
				continue
			}
			profile, err := repo.GetCompanyApplicabilityProfile(ctx, companyID)
			if err != nil {
				warnPeriodicSeedSkip(ctx, "company_profile_error", t.TypeID, companyID, 0, err)
				continue
			}
			if t.IsGlobal && !applicability.IsApplicable(t.ApplicabilityRules, profile, strictApplicabilityFilter) {
				continue
			}
			companyAuth := CompanyOverrideAuthority{}
			if hasPref {
				companyAuth = PreferenceToOverrideAuthority(&pref, t.FrequencyUnit)
			}
			effAnchor, _ := ResolveEffectiveAnchor(t.FrequencyUnit, cmsAnchor, companyAuth)
			futureT, err := ResolveOccurrenceT(t.FrequencyUnit, nextLabel, effAnchor, loc)
			if err != nil {
				slog.WarnContext(ctx, "periodic future pregen skip: resolve T",
					slog.String("type_id", t.TypeID),
					slog.String("company_id", companyID),
					slog.String("cycle_label", nextLabel),
					slog.String("err", err.Error()))
				continue
			}
			generateAt := GenerateAtDate(futureT, lead)
			if todayHCM.Before(generateAt) {
				continue
			}
			if ok := seedOneCompanySlot(ctx, now, repo, idg, calc, strictApplicabilityFilter, shadow, t, companyID, nextLabel, cmsAnchor, prefByKey, loc); ok {
				seeded++
			}
		}
	}
	return seeded, nil
}

// periodicTypeAppliesToCompany reports whether t may be seeded for companyID: global types for
// every company, company-scoped types only for their owner (BES-38/PER-02). The SQL sets
// OwnerCompanyID for every company-scoped row; an empty owner only occurs in test fakes.
func periodicTypeAppliesToCompany(t PeriodicTypeRow, companyID string) bool {
	if t.IsGlobal {
		return true
	}
	owner := strings.TrimSpace(t.OwnerCompanyID)
	return owner == "" || owner == strings.TrimSpace(companyID)
}

// seedOneCompanySlot resolves override/T/AT/due/open and UpsertPeriodicCycle for one (company, label).
// Returns true when an upsert was attempted successfully.
func seedOneCompanySlot(
	ctx context.Context,
	now time.Time,
	repo Repository,
	idg idgen.Generator,
	calc *DeadlineCalculator,
	strictApplicabilityFilter bool,
	shadow *deadlineEngineShadowRunner,
	t PeriodicTypeRow,
	companyID, label string,
	cmsAnchor AnchorConfig,
	prefByKey map[string]CompanyTypePreference,
	loc *time.Location,
) bool {
	if !periodicTypeAppliesToCompany(t, companyID) {
		return false
	}
	pref, hasPref := prefByKey[companyID+"|"+t.TypeID]
	if hasPref && !pref.AutoCreateEnabled {
		return false
	}
	profile, err := repo.GetCompanyApplicabilityProfile(ctx, companyID)
	if err != nil {
		warnPeriodicSeedSkip(ctx, "company_profile_error", t.TypeID, companyID, 0, err)
		return false
	}
	if t.IsGlobal && !applicability.IsApplicable(t.ApplicabilityRules, profile, strictApplicabilityFilter) {
		return false
	}

	companyAuth := CompanyOverrideAuthority{}
	if hasPref {
		companyAuth = PreferenceToOverrideAuthority(&pref, t.FrequencyUnit)
	}
	effAnchor, tSource := ResolveEffectiveAnchor(t.FrequencyUnit, cmsAnchor, companyAuth)
	cycleStart, err := ResolveOccurrenceT(t.FrequencyUnit, label, effAnchor, loc)
	if err != nil {
		slog.WarnContext(ctx, "periodic seed skip: resolve T",
			slog.String("type_id", t.TypeID),
			slog.String("company_id", companyID),
			slog.String("cycle_label", label),
			slog.String("err", err.Error()))
		return false
	}
	_ = tSource

	toEligible, toDecision, toErr := EvaluateApplicableToEligibility(cycleStart, t.ApplicableTo, loc)
	if toErr != nil {
		slog.WarnContext(ctx, "periodic seed skip: applicable_to invalid",
			slog.String("type_id", t.TypeID),
			slog.String("company_id", companyID),
			slog.String("applicable_to", t.ApplicableTo),
			slog.String("decision", toDecision),
			slog.String("err", toErr.Error()))
		return false
	}
	if !toEligible {
		slog.DebugContext(ctx, "periodic seed skip: after applicable_to",
			slog.String("type_id", t.TypeID),
			slog.String("company_id", companyID),
			slog.String("candidate_slot", label),
			slog.String("applicable_to", t.ApplicableTo),
			slog.String("decision", toDecision))
		return false
	}

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
		reason := "due_date_error"
		if errors.Is(err, ErrHolidayYearMissing) {
			reason = "holiday_year_missing"
		}
		warnPeriodicSeedSkip(ctx, reason, t.TypeID, companyID, cycleStart.Year(), err)
		return false
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
		warnPeriodicSeedSkip(ctx, "upsert_error", t.TypeID, companyID, cycleStart.Year(), err)
		return false
	}
	return true
}

// periodicSeedWarnEvery limits repeated seed-skip warnings per (type, reason, year); the seeder
// runs every tick, so an unthrottled warning would flood the log.
const periodicSeedWarnEvery = time.Hour

var periodicSeedWarned = struct {
	sync.Mutex
	at map[string]time.Time
}{at: map[string]time.Time{}}

// warnPeriodicSeedSkip logs why a cycle was not seeded (PER-06): these were dropped silently,
// so a missing holiday year simply stopped cycles from appearing.
func warnPeriodicSeedSkip(ctx context.Context, reason, typeID, companyID string, year int, err error) {
	key := fmt.Sprintf("%s|%s|%d", typeID, reason, year)
	now := time.Now()
	periodicSeedWarned.Lock()
	last, seen := periodicSeedWarned.at[key]
	if seen && now.Sub(last) < periodicSeedWarnEvery {
		periodicSeedWarned.Unlock()
		return
	}
	periodicSeedWarned.at[key] = now
	periodicSeedWarned.Unlock()
	slog.WarnContext(ctx, "periodic seed skipped",
		slog.String("reason", reason),
		slog.String("type_id", typeID),
		slog.String("company_id", companyID),
		slog.Int("year", year),
		slog.String("err", err.Error()))
}

const (
	// PeriodicClaimLease: a CLAIMED cycle older than this is requeued; it must exceed
	// periodicItemTimeout so a live claim is never reaped (PER-04).
	PeriodicClaimLease = 10 * time.Minute
	// periodicFinalizeTimeout bounds the state transition written after the create call, on
	// its own context so an expired item context cannot leave the cycle CLAIMED (PERF-01).
	periodicFinalizeTimeout = 10 * time.Second
)

// periodicItemTimeout bounds the create call of one claimed cycle, detached from the tick
// context (a variable only so tests can shorten it).
var periodicItemTimeout = 2 * time.Minute

// PeriodicStaleClaimReaper requeues claims abandoned by a stopped worker (PER-04).
type PeriodicStaleClaimReaper interface {
	RequeueStalePeriodicClaims(ctx context.Context, olderThanSec int) (int64, error)
}

// materializePeriodicDisclosures picks pending cycles whose OpenAt <= TodayHCM
// (bufferDays=0; asOf = HCM date-only) and creates disclosure records with workflow.
func materializePeriodicDisclosures(ctx context.Context, now time.Time, repo PeriodicMaterializeRepository, creator PeriodicRecordCreator) (int, error) {
	if creator == nil {
		return 0, fmt.Errorf("periodic record creator is unavailable")
	}
	if reaper, ok := repo.(PeriodicStaleClaimReaper); ok {
		n, err := reaper.RequeueStalePeriodicClaims(ctx, int(PeriodicClaimLease/time.Second))
		if err != nil {
			slog.WarnContext(ctx, "periodic stale claim reaper failed", slog.String("err", err.Error()))
		} else if n > 0 {
			slog.WarnContext(ctx, "periodic stale claims requeued", slog.Int64("count", n))
		}
	}
	const bufferDays = 0 // no lookahead — require TodayHCM >= COALESCE(open_at, cycle_start)
	asOf := stripTime(now.In(asiaHoChiMinh()))
	cycles, err := repo.ListPendingCycles(ctx, asOf, bufferDays)
	if err != nil {
		return 0, fmt.Errorf("list pending cycles: %w", err)
	}
	materialized := 0
	for _, c := range cycles {
		if ctx.Err() != nil {
			return materialized, nil // shutting down: claim nothing new
		}
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
		// A claimed cycle is finished even if the tick is cancelled (SIGTERM), so it never
		// stays CLAIMED; the timeout keeps shutdown bounded (PER-04).
		itemCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), periodicItemTimeout)
		ok, err := materializeClaimedCycle(itemCtx, repo, creator, c)
		cancel()
		if err != nil {
			return materialized, err
		}
		if ok {
			materialized++
		}
	}
	return materialized, nil
}

// permanentGateStatus reports the HTTP status of a creator error that retrying cannot fix
// (400/403/404/422); 402/409/429 and non-HTTP errors stay retryable (PER-03).
func permanentGateStatus(err error) (int, bool) {
	httpErr, ok := perr.AsHTTPError(err)
	if !ok || httpErr == nil {
		return 0, false
	}
	switch httpErr.HTTPStatus {
	case http.StatusBadRequest, http.StatusForbidden, http.StatusNotFound, http.StatusUnprocessableEntity:
		return httpErr.HTTPStatus, true
	}
	return 0, false
}

// materializeClaimedCycle creates the record of one claimed cycle and moves the cycle to
// COMPLETED, RETRY or FAILED. It returns true when the cycle completed.
func materializeClaimedCycle(ctx context.Context, repo PeriodicMaterializeRepository, creator PeriodicRecordCreator, c PeriodicCycleRow) (bool, error) {
	t0 := c.CycleStart
	plannedDate := ""
	if !c.DueDate.IsZero() {
		plannedDate = c.DueDate.Format("2006-01-02")
	}
	recordID, workflowInstanceID, err := creator.CreateAndSubmitPeriodicRecord(ctx, c.CycleID, c.CompanyID, c.TypeID, "m_system_worker", autoRecordTitle(c), &t0, plannedDate)
	// The transition is written on its own short context: if the create call used up the item
	// timeout, the cycle must still leave CLAIMED (PERF-01).
	fin, cancelFin := context.WithTimeout(context.WithoutCancel(ctx), periodicFinalizeTimeout)
	defer cancelFin()
	if err != nil {
		if workflowerrs.IsDuplicateWorkflowStep(err) {
			// Configuration error: retrying cannot succeed until the template/override is fixed.
			if markErr := repo.MarkPeriodicCycleFailed(fin, c.CycleID, recordID, "DUPLICATE_WORKFLOW_STEP", "effective workflow has duplicate step ids"); markErr != nil {
				return false, fmt.Errorf("mark periodic cycle %s failed: %w", c.CycleID, markErr)
			}
			slog.ErrorContext(ctx, "periodic materialization failed permanently: duplicate workflow step ids",
				slog.String("cycle_id", c.CycleID),
				slog.String("type_id", c.TypeID),
				slog.String("company_id", c.CompanyID))
			return false, nil
		}
		if workflowerrs.IsEmptyEffectiveWorkflow(err) {
			if markErr := repo.MarkPeriodicCycleFailed(fin, c.CycleID, recordID, "EMPTY_EFFECTIVE_WORKFLOW", "effective workflow has no materializable steps"); markErr != nil {
				return false, fmt.Errorf("mark periodic cycle %s failed: %w", c.CycleID, markErr)
			}
			slog.ErrorContext(ctx, "periodic materialization failed permanently: empty effective workflow",
				slog.String("cycle_id", c.CycleID),
				slog.String("type_id", c.TypeID),
				slog.String("company_id", c.CompanyID))
			return false, nil
		}
		if status, permanent := permanentGateStatus(err); permanent {
			code := fmt.Sprintf("GATE_%d", status)
			if markErr := repo.MarkPeriodicCycleFailed(fin, c.CycleID, recordID, code, "periodic materialization rejected; see structured worker log"); markErr != nil {
				return false, fmt.Errorf("mark periodic cycle %s failed: %w", c.CycleID, markErr)
			}
			slog.ErrorContext(ctx, "periodic materialization failed permanently: rejected by gate",
				slog.String("cycle_id", c.CycleID),
				slog.String("type_id", c.TypeID),
				slog.String("company_id", c.CompanyID),
				slog.String("code", code),
				slog.String("err", err.Error()))
			return false, nil
		}
		if markErr := repo.MarkPeriodicCycleRetry(fin, c.CycleID, recordID, "MATERIALIZATION_ERROR", "periodic materialization failed; see structured worker log"); markErr != nil {
			return false, fmt.Errorf("mark periodic cycle %s retry: %w", c.CycleID, markErr)
		}
		slog.WarnContext(ctx, "periodic materialization will retry",
			slog.String("cycle_id", c.CycleID),
			slog.String("type_id", c.TypeID),
			slog.String("company_id", c.CompanyID),
			slog.String("record_id", recordID),
			slog.String("err", err.Error()))
		return false, nil
	}
	if recordID == "" || workflowInstanceID == "" {
		if markErr := repo.MarkPeriodicCycleFailed(fin, c.CycleID, recordID, "INCOMPLETE_MATERIALIZATION", "record and workflow instance are both required"); markErr != nil {
			return false, fmt.Errorf("mark periodic cycle %s failed: %w", c.CycleID, markErr)
		}
		slog.ErrorContext(ctx, "periodic materialization failed permanently: missing record or workflow instance",
			slog.String("cycle_id", c.CycleID),
			slog.String("record_id", recordID),
			slog.String("workflow_instance_id", workflowInstanceID))
		return false, nil
	}
	if err := repo.UpdateCycleRecord(fin, c.CycleID, recordID); err != nil {
		if markErr := repo.MarkPeriodicCycleRetry(fin, c.CycleID, recordID, "CYCLE_COMPLETION_ERROR", "record and workflow are ready but cycle completion failed"); markErr != nil {
			return false, fmt.Errorf("complete periodic cycle %s: %w; mark retry: %v", c.CycleID, err, markErr)
		}
		return false, fmt.Errorf("complete periodic cycle %s: %w", c.CycleID, err)
	}
	return true, nil
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
	anchor := AnchorConfig{
		Month:          t.CycleAnchorMonth,
		Day:            t.CycleAnchorDay,
		Weekday:        t.CycleAnchorWeekday,
		MonthInQuarter: t.MonthInQuarter,
	}
	start, err := ResolveOccurrenceT(t.FrequencyUnit, label, anchor, loc)
	if err != nil {
		start = stripTime(now.In(loc))
	}
	return
}
