package app

import (
	"strings"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

const defaultCompanyTimezone = "Asia/Ho_Chi_Minh"

// buildMilestonesFromSnapshot derives workflow_step_milestones rows from a frozen snapshot and T0.
func buildMilestonesFromSnapshot(instanceID, companyID string, snapshot []StepSnapshot, t0 time.Time, idFn func() string) ([]StepMilestoneRow, error) {
	if instanceID == "" || companyID == "" || len(snapshot) == 0 || t0.IsZero() {
		return nil, nil
	}
	loc, err := time.LoadLocation(defaultCompanyTimezone)
	if err != nil {
		return nil, err
	}
	t0Local := t0.In(loc)
	t0Local = time.Date(t0Local.Year(), t0Local.Month(), t0Local.Day(), 0, 0, 0, 0, loc)

	timelines, err := disclosureapp.ComputeStepTimelines(t0Local, defaultCompanyTimezone, snapshotToWorkflowSteps(snapshot), 1)
	if err != nil {
		return nil, err
	}
	out := make([]StepMilestoneRow, 0, len(timelines)*4)
	for i, tl := range timelines {
		for _, row := range disclosureapp.GenerateTimelineMilestoneCandidates(tl, t0Local, companyID, instanceID, idFn) {
			out = append(out, StepMilestoneRow{
				MilestoneID:   row.MilestoneID,
				CompanyID:     companyID,
				InstanceID:    instanceID,
				StepID:        row.StepID,
				StepOrder:     row.StepOrder,
				MilestoneType: string(row.MilestoneType),
				ScheduledDate: row.ScheduledDate,
			})
		}
		effective := snapshotEffectiveReminderDays(snapshot, i, tl.StepID)
		for _, row := range disclosureapp.GenerateDueMinusReminderCandidates(tl, t0Local, companyID, instanceID, effective, idFn) {
			out = append(out, StepMilestoneRow{
				MilestoneID:   row.MilestoneID,
				CompanyID:     companyID,
				InstanceID:    instanceID,
				StepID:        row.StepID,
				StepOrder:     row.StepOrder,
				MilestoneType: string(row.MilestoneType),
				ScheduledDate: row.ScheduledDate,
			})
		}
	}
	return out, nil
}

func snapshotEffectiveReminderDays(snapshot []StepSnapshot, index int, stepID string) []int {
	if index >= 0 && index < len(snapshot) && snapshot[index].StepID == stepID {
		if snapshot[index].EffectiveReminderDays != nil {
			return snapshot[index].EffectiveReminderDays
		}
		return disclosureapp.ResolveWorkflowStepReminderRule(snapshot[index].ReminderConfig).EffectiveDays
	}
	for _, s := range snapshot {
		if s.StepID == stepID {
			if s.EffectiveReminderDays != nil {
				return s.EffectiveReminderDays
			}
			return disclosureapp.ResolveWorkflowStepReminderRule(s.ReminderConfig).EffectiveDays
		}
	}
	return append([]int(nil), disclosureapp.DefaultWorkflowStepReminderDays...)
}

func snapshotToWorkflowSteps(snapshot []StepSnapshot) []disclosureapp.WorkflowStepDTO {
	steps := make([]disclosureapp.WorkflowStepDTO, 0, len(snapshot))
	for _, s := range snapshot {
		stepID := strings.TrimSpace(s.StepID)
		if stepID == "" {
			continue
		}
		steps = append(steps, disclosureapp.WorkflowStepDTO{
			StepID:         stepID,
			Stage:          strings.TrimSpace(s.Stage),
			DepartmentID:   strings.TrimSpace(s.Department),
			DueRule:        strings.TrimSpace(s.DueRule),
			ProcessingDays: s.ProcessingDays,
			DisplayOrder:   s.DisplayOrder,
		})
	}
	return steps
}
