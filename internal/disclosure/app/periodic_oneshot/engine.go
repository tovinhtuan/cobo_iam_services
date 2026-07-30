package periodic_oneshot

import (
	"context"
	"fmt"
	"strings"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
)

const (
	ActionCreateCycleAndRecord = "CREATE_CYCLE_AND_DISCLOSURE_RECORD"
	ActionNoOp                 = "NO_OP_ALREADY_MATERIALIZED"
	StatusMaterialized         = "MATERIALIZED"
	StatusNoOp                 = "NO_OP_ALREADY_MATERIALIZED"
	StatusPreview              = "PREVIEW_OK"
)

func (e *Engine) Preview(ctx context.Context, scope Scope) (*Report, error) {
	if err := e.Env.Validate(); err != nil {
		return nil, err
	}
	if err := ValidateAllowlist(scope); err != nil {
		return nil, err
	}
	plan, err := e.buildPlan(ctx, scope.Normalize())
	if err != nil {
		return nil, err
	}
	rep := plan.toReport("PREVIEW")
	rep.Mutations = 0
	rep.Status = StatusPreview
	if plan.plannedAction == ActionNoOp {
		rep.Status = StatusNoOp
	}
	return rep, nil
}

func (e *Engine) Apply(ctx context.Context, scope Scope, confirmToken string) (*Report, error) {
	if err := e.Env.Validate(); err != nil {
		return nil, err
	}
	if err := ValidateAllowlist(scope); err != nil {
		return nil, err
	}
	scope = scope.Normalize()
	plan, err := e.buildPlan(ctx, scope)
	if err != nil {
		return nil, err
	}
	if err := ConfirmTokenOK(confirmToken, plan.confirmToken); err != nil {
		return nil, fmt.Errorf("confirm token: %w", err)
	}

	if plan.plannedAction == ActionNoOp {
		rep := plan.toReport("APPLY")
		rep.Status = StatusNoOp
		rep.Mutations = 0
		rep.CycleCreated = false
		rep.RecordCreated = false
		rep.TransactionCommitted = true
		return rep, nil
	}

	// Freshness re-read
	plan2, err := e.buildPlan(ctx, scope)
	if err != nil {
		return nil, err
	}
	if plan2.snapshotChecksum != plan.snapshotChecksum {
		return nil, fmt.Errorf("MATERIALIZATION_STATE_CHANGED: snapshot drifted between preview and apply")
	}
	if plan2.confirmToken != plan.confirmToken {
		return nil, fmt.Errorf("MATERIALIZATION_STATE_CHANGED: confirm token stale")
	}

	cycleID := plan.cycleID
	cycleCreated := false
	if !plan.cycleExists {
		cycleID = e.Domain.NewCycleID()
		row := disclosureapp.PeriodicCycleRow{
			CycleID:    cycleID,
			TypeID:     scope.TypeID,
			TypeName:   plan.typeName,
			CompanyID:  scope.CompanyID,
			CycleLabel: plan.cycleLabel,
			CycleStart: plan.cycleStart,
			DueDate:    plan.due,
		}
		if err := e.Domain.InsertCycle(ctx, row); err != nil {
			return nil, fmt.Errorf("insert cycle: %w", err)
		}
		cycleCreated = true
	}

	claimed, err := e.Domain.ClaimCycle(ctx, cycleID)
	if err != nil {
		if cycleCreated {
			_ = e.Domain.DeleteUnmaterializedCycle(ctx, cycleID)
		}
		return nil, fmt.Errorf("claim cycle: %w", err)
	}
	if !claimed {
		// Another worker claimed — re-read
		cyc, _ := e.Domain.LoadCycle(ctx, scope.TypeID, scope.CompanyID, plan.cycleLabel)
		if cyc.Exists && strings.TrimSpace(cyc.RecordID) != "" {
			rep := plan.toReport("APPLY")
			rep.Status = StatusNoOp
			rep.CycleID = cyc.CycleID
			rep.RecordID = cyc.RecordID
			rep.Mutations = 0
			return rep, nil
		}
		if cycleCreated {
			_ = e.Domain.DeleteUnmaterializedCycle(ctx, cycleID)
		}
		return nil, fmt.Errorf("MATERIALIZATION_STATE_CHANGED: cycle claim lost")
	}

	t0 := plan.cycleStart
	plannedDate := plan.due.Format("2006-01-02")
	title := fmt.Sprintf("%s — %s", plan.typeName, plan.cycleLabel)
	recordID, wfID, err := e.Domain.CreateAndSubmitRecordWithPlannedDate(
		ctx, scope.CompanyID, scope.TypeID, "m_system_oneshot", title, &t0, plannedDate,
	)
	if err != nil {
		_ = e.Domain.ReleaseCycle(ctx, cycleID)
		if cycleCreated {
			_ = e.Domain.DeleteUnmaterializedCycle(ctx, cycleID)
		}
		return nil, fmt.Errorf("create disclosure record: %w", err)
	}
	if recordID == "" || wfID == "" {
		_ = e.Domain.ReleaseCycle(ctx, cycleID)
		if cycleCreated {
			_ = e.Domain.DeleteUnmaterializedCycle(ctx, cycleID)
		}
		return nil, fmt.Errorf("create disclosure record: missing record or workflow instance")
	}
	if err := e.Domain.UpdateCycleRecord(ctx, cycleID, recordID); err != nil {
		_ = e.Domain.ReleaseCycle(ctx, cycleID)
		if cycleCreated {
			_ = e.Domain.DeleteUnmaterializedCycle(ctx, cycleID)
		}
		return nil, fmt.Errorf("link cycle record: %w", err)
	}

	rep := plan.toReport("APPLY")
	rep.Status = StatusMaterialized
	rep.CycleCreated = cycleCreated
	rep.RecordCreated = true
	rep.TransactionCommitted = true
	rep.Mutations = 0
	if cycleCreated {
		rep.Mutations++
	}
	rep.Mutations++
	rep.CycleID = cycleID
	rep.RecordID = recordID
	rep.ConfirmToken = "" // do not echo on apply success noise
	return rep, nil
}

type planState struct {
	scope            Scope
	typeName         string
	activeVersion    int
	cycleLabel       string
	cycleStart       time.Time
	due              time.Time
	durationType     string
	deadlineDays     int
	cycleExists      bool
	cycleID          string
	recordExists     bool
	recordID         string
	plannedAction    string
	plannedActions   []string
	snapshotChecksum string
	confirmToken     string
	initialStatus    string
}

func (e *Engine) buildPlan(ctx context.Context, scope Scope) (*planState, error) {
	ts, err := e.Domain.LoadType(ctx, scope.TypeID, scope.CompanyID)
	if err != nil {
		return nil, fmt.Errorf("load type: %w", err)
	}
	if err := validateTypePreconditions(ts); err != nil {
		return nil, err
	}
	profile, err := e.Domain.LoadCompanyProfile(ctx, scope.CompanyID)
	if err != nil {
		return nil, fmt.Errorf("load company profile: %w", err)
	}
	if ts.IsGlobal && !applicability.IsApplicable(ts.ApplicabilityRules, profile, true) {
		return nil, fmt.Errorf("REFUSE: company %s not applicable to type", scope.CompanyID)
	}

	loc := e.Domain.Location()
	cycleStart, cycleLabel, err := parseMonthlyPeriod(scope.Period, loc)
	if err != nil {
		return nil, err
	}
	if cycleLabel != AllowedPeriod || cycleStart.Format("2006-01-02") != ExpectedPeriodStart {
		return nil, fmt.Errorf("REFUSE: period resolution mismatch")
	}

	deadlineDays := ts.DeadlineDays
	durationType := disclosureapp.DurationTypeCalendarDays
	if ts.ApplicabilityRules != nil {
		if days, ok := applicability.ResolveDeadlineDays(ts.ApplicabilityRules, profile); ok {
			deadlineDays = days
		}
		durationType = applicability.ResolveDeadlineDurationType(ts.ApplicabilityRules)
	} else if deadlineDays > 0 {
		durationType = disclosureapp.DurationTypeWorkingDays
	}
	if deadlineDays != ExpectedDeadlineDays {
		return nil, fmt.Errorf("REFUSE: deadline_days drift want %d got %d", ExpectedDeadlineDays, deadlineDays)
	}
	if durationType != ExpectedDurationUnit {
		return nil, fmt.Errorf("REFUSE: duration unit drift want %s got %s", ExpectedDurationUnit, durationType)
	}

	due, err := e.Domain.ComputeDue(ctx, cycleStart, deadlineDays, durationType)
	if err != nil {
		return nil, fmt.Errorf("calculator: %w", err)
	}
	if due.Format("2006-01-02") != ExpectedDueDate {
		return nil, fmt.Errorf("REFUSE_CALCULATOR_MISMATCH: want %s got %s", ExpectedDueDate, due.Format("2006-01-02"))
	}

	cyc, err := e.Domain.LoadCycle(ctx, scope.TypeID, scope.CompanyID, cycleLabel)
	if err != nil {
		return nil, fmt.Errorf("load cycle: %w", err)
	}

	plan := &planState{
		scope:         scope,
		typeName:      ts.TypeName,
		activeVersion: ts.ActiveVersionNo,
		cycleLabel:    cycleLabel,
		cycleStart:    cycleStart,
		due:           due,
		durationType:  durationType,
		deadlineDays:  deadlineDays,
		cycleExists:   cyc.Exists,
		cycleID:       cyc.CycleID,
		initialStatus: "submitted_via_materialize",
	}

	if cyc.Exists {
		if cyc.DueDate != "" && cyc.DueDate != ExpectedDueDate {
			return nil, fmt.Errorf("REFUSE: existing cycle due_date incompatible %s", cyc.DueDate)
		}
		if cyc.CycleStart != "" && cyc.CycleStart != ExpectedPeriodStart {
			return nil, fmt.Errorf("REFUSE: existing cycle_start incompatible %s", cyc.CycleStart)
		}
		if strings.TrimSpace(cyc.RecordID) != "" {
			plan.recordExists = true
			plan.recordID = cyc.RecordID
			plan.plannedAction = ActionNoOp
			plan.plannedActions = []string{ActionNoOp}
		} else {
			// cycle without record — continue to create record only
			plan.plannedAction = ActionCreateCycleAndRecord
			plan.plannedActions = []string{"CREATE_DISCLOSURE_RECORD"}
		}
	} else {
		plan.plannedAction = ActionCreateCycleAndRecord
		plan.plannedActions = []string{"CREATE_PERIODIC_CYCLE", "CREATE_DISCLOSURE_RECORD"}
	}

	plan.snapshotChecksum = SnapshotChecksum(
		scope.TypeID, scope.CompanyID, scope.Period,
		fmt.Sprintf("%d", ts.ActiveVersionNo),
		ts.DeadlineMode, fmt.Sprintf("%d", deadlineDays), durationType,
		due.Format("2006-01-02"),
		fmt.Sprintf("%v", cyc.Exists), cyc.CycleID, cyc.RecordID,
		plan.plannedAction,
	)
	plan.confirmToken = BuildConfirmToken(
		e.Env.Environment, e.Env.Database,
		scope.TypeID, scope.CompanyID, scope.Period,
		ts.ActiveVersionNo, due.Format("2006-01-02"),
		plan.snapshotChecksum, plan.plannedAction,
	)
	return plan, nil
}

func (p *planState) toReport(mode string) *Report {
	return &Report{
		Mode:        mode,
		Environment: "DEV",
		Scope:       p.scope,
		Resolved: map[string]any{
			"period_start":   ExpectedPeriodStart,
			"period_end":     ExpectedPeriodEnd,
			"due_date":       p.due.Format("2006-01-02"),
			"deadline_days":  p.deadlineDays,
			"deadline_unit":  p.durationType,
			"cycle_label":    p.cycleLabel,
			"initial_status": p.initialStatus,
			"active_version": p.activeVersion,
			"type_name":      p.typeName,
			"planned_action": p.plannedAction,
		},
		Existing: map[string]any{
			"cycle":     p.cycleExists,
			"record":    p.recordExists,
			"cycle_id":  p.cycleID,
			"record_id": p.recordID,
		},
		PlannedActions:   p.plannedActions,
		SnapshotChecksum: p.snapshotChecksum,
		ConfirmToken:     p.confirmToken,
		CycleID:          p.cycleID,
		RecordID:         p.recordID,
	}
}

func validateTypePreconditions(ts TypeSnapshot) error {
	if !strings.EqualFold(strings.TrimSpace(ts.Status), "active") {
		return fmt.Errorf("REFUSE: type status not active (%s)", ts.Status)
	}
	if ts.ActiveVersionNo <= 0 {
		return fmt.Errorf("REFUSE: active_version_no invalid")
	}
	if !strings.EqualFold(ts.DeadlineMode, ExpectedDeadlineMode) {
		return fmt.Errorf("REFUSE: deadline_mode want %s got %s", ExpectedDeadlineMode, ts.DeadlineMode)
	}
	if !strings.EqualFold(ts.FrequencyUnit, ExpectedFreqUnit) {
		return fmt.Errorf("REFUSE: frequency_unit want %s got %s", ExpectedFreqUnit, ts.FrequencyUnit)
	}
	if ts.DeadlineDays != ExpectedDeadlineDays {
		return fmt.Errorf("REFUSE: deadline_days want %d got %d", ExpectedDeadlineDays, ts.DeadlineDays)
	}
	if !ts.HasWorkflow {
		return fmt.Errorf("REFUSE: workflow prerequisite not satisfied")
	}
	return nil
}

func parseMonthlyPeriod(period string, loc *time.Location) (start time.Time, label string, err error) {
	period = strings.TrimSpace(period)
	t, err := time.ParseInLocation("2006-01", period, loc)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("invalid period %q (want YYYY-MM)", period)
	}
	start = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, loc)
	label = start.Format("2006-01")
	return start, label, nil
}
