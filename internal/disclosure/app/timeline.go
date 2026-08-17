package app

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var dueRuleProcessingDaysPattern = regexp.MustCompile(`(?i)t\s*\+\s*(\d+)`)

// StepTimeline holds the computed date window for one workflow step.
type StepTimeline struct {
	StepID         string
	StepOrder      int
	StartDate      time.Time // inclusive, midnight in company TZ
	EndDate        time.Time // inclusive, midnight in company TZ
	ProcessingDays int
}

// MilestoneType enumerates the 5 reminder milestones per step (Q6: previous_step_end removed).
type MilestoneType string

const (
	MilestoneBefore5d  MilestoneType = "before_start_5d"
	MilestoneBefore3d  MilestoneType = "before_start_3d"
	MilestoneBefore1d  MilestoneType = "before_start_1d"
	MilestoneStepStart MilestoneType = "step_start"
	MilestoneStepEnd   MilestoneType = "step_end"
)

// StepMilestoneRow is a single reminder milestone candidate ready for persistence.
type StepMilestoneRow struct {
	MilestoneID   string
	CompanyID     string
	InstanceID    string
	StepID        string
	StepOrder     int
	MilestoneType MilestoneType
	ScheduledDate time.Time // date only, normalized to midnight UTC for storage
}

const defaultCompanyTimezone = "Asia/Ho_Chi_Minh"

// CompanyLocation returns the time.Location for the given IANA tz name.
// Falls back to Asia/Ho_Chi_Minh if tz is empty or unknown.
func CompanyLocation(tz string) *time.Location {
	if tz == "" {
		tz = defaultCompanyTimezone
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc, _ = time.LoadLocation(defaultCompanyTimezone)
	}
	return loc
}

// ComputeStepTimelines derives start/end dates for each step from T0.
// Steps are assumed ordered by DisplayOrder ascending.
// Each step's start = previous step's end + 1 day; first step starts on T0.
// processingDaysOverride: per-step processing_days from step config (0 = use typeDefault).
// typeDefaultProcessingDays: fallback from TemplateDeadlineConfig.ProcessingDays.
func ComputeStepTimelines(t0 time.Time, tz string, steps []WorkflowStepDTO, typeDefaultProcessingDays int) ([]StepTimeline, error) {
	if len(steps) == 0 {
		return nil, nil
	}
	loc := CompanyLocation(tz)
	// normalise T0 to midnight in company timezone
	t0Local := time.Date(t0.Year(), t0.Month(), t0.Day(), 0, 0, 0, 0, loc)

	timelines := make([]StepTimeline, 0, len(steps))
	cursor := t0Local
	for i, step := range steps {
		pd := processingDaysForStep(step, typeDefaultProcessingDays)
		tl := StepTimeline{
			StepID:         step.StepID,
			StepOrder:      i + 1,
			StartDate:      cursor,
			ProcessingDays: pd,
		}
		// end = start + processing_days - 1  (inclusive range)
		tl.EndDate = cursor.AddDate(0, 0, pd-1)
		timelines = append(timelines, tl)
		cursor = tl.EndDate.AddDate(0, 0, 1) // next step starts day after current ends
	}
	return timelines, nil
}

// GenerateTimelineMilestoneCandidates produces step_start / step_end rows for timeline/UI/reporting.
// These are not reminder-dispatch occurrences.
func GenerateTimelineMilestoneCandidates(tl StepTimeline, t0 time.Time, companyID, instanceID string, idFn func() string) []StepMilestoneRow {
	t0Day := time.Date(t0.Year(), t0.Month(), t0.Day(), 0, 0, 0, 0, tl.StartDate.Location())
	candidates := []struct {
		mtype MilestoneType
		date  time.Time
	}{
		{MilestoneStepStart, tl.StartDate},
		{MilestoneStepEnd, tl.EndDate},
	}
	rows := make([]StepMilestoneRow, 0, len(candidates))
	for _, c := range candidates {
		if c.date.Before(t0Day) {
			continue
		}
		rows = append(rows, StepMilestoneRow{
			MilestoneID:   buildMilestoneID(instanceID, tl.StepID, string(c.mtype), idFn),
			CompanyID:     companyID,
			InstanceID:    instanceID,
			StepID:        tl.StepID,
			StepOrder:     tl.StepOrder,
			MilestoneType: c.mtype,
			ScheduledDate: c.date,
		})
	}
	return rows
}

// GenerateDueMinusReminderCandidates produces EndDate-minus-N reminder rows from frozen effective days.
// remind_at = StepTimeline.EndDate - offset. Does not use StartDate.
func GenerateDueMinusReminderCandidates(tl StepTimeline, t0 time.Time, companyID, instanceID string, effectiveDays []int, idFn func() string) []StepMilestoneRow {
	t0Day := time.Date(t0.Year(), t0.Month(), t0.Day(), 0, 0, 0, 0, tl.EndDate.Location())
	rows := make([]StepMilestoneRow, 0, len(effectiveDays))
	for _, offset := range effectiveDays {
		if offset <= 0 {
			continue
		}
		mtype := DueMinusMilestoneType(offset)
		date := tl.EndDate.AddDate(0, 0, -offset)
		if date.Before(t0Day) {
			continue
		}
		rows = append(rows, StepMilestoneRow{
			MilestoneID:   buildMilestoneID(instanceID, tl.StepID, string(mtype), idFn),
			CompanyID:     companyID,
			InstanceID:    instanceID,
			StepID:        tl.StepID,
			StepOrder:     tl.StepOrder,
			MilestoneType: mtype,
			ScheduledDate: date,
		})
	}
	return rows
}

// GenerateConfiguredReminderPreviewCandidates matches runtime instance seeding:
// step_start/step_end timeline rows plus due-minus reminders from the step's reminder_config.
func GenerateConfiguredReminderPreviewCandidates(tl StepTimeline, t0 time.Time, companyID, instanceID string, reminderCfg *WorkflowStepReminderConfig, idFn func() string) []StepMilestoneRow {
	rows := GenerateTimelineMilestoneCandidates(tl, t0, companyID, instanceID, idFn)
	days := ResolveWorkflowStepReminderRule(reminderCfg).EffectiveDays
	return append(rows, GenerateDueMinusReminderCandidates(tl, t0, companyID, instanceID, days, idFn)...)
}

// GenerateMilestoneCandidates produces the legacy 5 milestone rows for a step timeline
// (before_start_* from StartDate plus step_start/step_end). Not used by runtime seeding
// or the Portal reminder-preview API after Phase 2D.
func GenerateMilestoneCandidates(tl StepTimeline, t0 time.Time, companyID, instanceID string, idFn func() string) []StepMilestoneRow {
	t0Day := time.Date(t0.Year(), t0.Month(), t0.Day(), 0, 0, 0, 0, tl.StartDate.Location())
	candidates := []struct {
		mtype MilestoneType
		date  time.Time
	}{
		{MilestoneBefore5d, tl.StartDate.AddDate(0, 0, -5)},
		{MilestoneBefore3d, tl.StartDate.AddDate(0, 0, -3)},
		{MilestoneBefore1d, tl.StartDate.AddDate(0, 0, -1)},
		{MilestoneStepStart, tl.StartDate},
		{MilestoneStepEnd, tl.EndDate},
	}
	rows := make([]StepMilestoneRow, 0, len(candidates))
	for _, c := range candidates {
		if c.date.Before(t0Day) {
			continue // skip milestones before T0 (Q6 decision)
		}
		rows = append(rows, StepMilestoneRow{
			MilestoneID:   buildMilestoneID(instanceID, tl.StepID, string(c.mtype), idFn),
			CompanyID:     companyID,
			InstanceID:    instanceID,
			StepID:        tl.StepID,
			StepOrder:     tl.StepOrder,
			MilestoneType: c.mtype,
			ScheduledDate: c.date,
		})
	}
	return rows
}

func buildMilestoneID(instanceID, stepID, milestoneType string, idFn func() string) string {
	// deterministic prefix + random suffix to guarantee uniqueness
	return fmt.Sprintf("%s_%s_%s_%s", instanceID[:8], stepID[:min(8, len(stepID))], milestoneType, idFn()[:8])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// processingDaysForStep resolves per-step duration from due_rule (T+N) or defaults.
func processingDaysForStep(step WorkflowStepDTO, typeDefaultProcessingDays int) int {
	if step.ProcessingDays > 0 {
		return step.ProcessingDays
	}
	if days := parseDueRuleProcessingDays(step.DueRule); days > 0 {
		return days
	}
	if typeDefaultProcessingDays > 0 {
		return typeDefaultProcessingDays
	}
	return 1
}

func parseDueRuleProcessingDays(dueRule string) int {
	matches := dueRuleProcessingDaysPattern.FindStringSubmatch(strings.TrimSpace(dueRule))
	if len(matches) < 2 {
		return 0
	}
	n, err := strconv.Atoi(matches[1])
	if err != nil || n <= 0 {
		return 0
	}
	return n
}
