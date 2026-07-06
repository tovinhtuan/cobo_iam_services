package app

import (
	"strings"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
)

// StepRuntimeState is persisted per workflow instance step.
type StepRuntimeState struct {
	StepCode                    string
	CompletedAt                 *time.Time
	CompletedByMembershipID     string
	MarkedIncompleteAt          *time.Time
	MarkedIncompleteByMembershipID string
	IncompleteReason            string
	DelayDaysApplied            int
}

// DeadlineStepDTO is the API view for one workflow step on deadline detail.
type DeadlineStepDTO struct {
	StepCode           string   `json:"step_code"`
	StepName           string   `json:"step_name"`
	Order              int      `json:"order"`
	DepartmentName     string   `json:"department_name,omitempty"`
	PlannedStartDate   string   `json:"planned_start_date"`
	PlannedEndDate     string   `json:"planned_end_date"`
	DurationDays       int      `json:"duration_days"`
	Status             string   `json:"status"`
	IsCurrentByTime    bool     `json:"is_current_by_time"`
	IsCompleted        bool     `json:"is_completed"`
	IsLocked           bool     `json:"is_locked"`
	IsFuture           bool     `json:"is_future"`
	IsDelayed          bool     `json:"is_delayed"`
	CompletedAt        string   `json:"completed_at,omitempty"`
	CompletedByName    string   `json:"completed_by_name,omitempty"`
	DelayDays          int      `json:"delay_days"`
	AvailableActions   []string `json:"available_actions"`
	LockReason         string   `json:"lock_reason,omitempty"`
}

// ListDeadlineStepsResponse is returned by GET .../deadlines/{record_id}/steps.
type ListDeadlineStepsResponse struct {
	RecordID        string            `json:"record_id"`
	CurrentStepCode string            `json:"current_step_code"`
	Steps           []DeadlineStepDTO `json:"steps"`
}

type MarkIncompleteStepRequest struct {
	Subject    Subject
	RecordID   string
	StepCode   string
	Reason     string
	DelayDays  int
}

type CompleteStepRequest struct {
	Subject  Subject
	RecordID string
	StepCode string
}

type WorkflowInstanceContext struct {
	WorkflowInstanceID string
	CompanyID          string
	RecordID           string
	T0Date             time.Time
	Snapshot           []workflowapp.StepSnapshot
	Timezone           string
}

// ComputeDeadlineSteps builds step DTOs with time-based current detection.
func ComputeDeadlineSteps(
	ctx WorkflowInstanceContext,
	states map[string]StepRuntimeState,
	today time.Time,
	tz string,
	canManage bool,
) (ListDeadlineStepsResponse, error) {
	snapshot := sortedSnapshot(ctx.Snapshot)
	if len(snapshot) == 0 {
		return ListDeadlineStepsResponse{RecordID: ctx.RecordID, Steps: []DeadlineStepDTO{}}, nil
	}
	loc := disclosureapp.CompanyLocation(tz)
	todayLocal := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, loc)
	t0 := ctx.T0Date.In(loc)
	t0 = time.Date(t0.Year(), t0.Month(), t0.Day(), 0, 0, 0, 0, loc)

	wfSteps := snapshotToWorkflowSteps(snapshot)
	timelines, err := disclosureapp.ComputeStepTimelines(t0, tz, wfSteps, 1)
	if err != nil {
		return ListDeadlineStepsResponse{}, err
	}
	shifted := applyDelayShifts(timelines, snapshot, states)

	currentIdx := resolveCurrentStepIndex(shifted, states, todayLocal)
	steps := make([]DeadlineStepDTO, 0, len(snapshot))
	var currentCode string

	for i, snap := range snapshot {
		code := stepCodeFromSnapshot(snap)
		st := states[code]
		tl := shifted[i]
		isCompleted := st.CompletedAt != nil
		isDelayed := st.DelayDaysApplied > 0 || st.MarkedIncompleteAt != nil

		cls := classifyWorkflowStep(i, currentIdx, tl, st, todayLocal)
		isCurrent := cls.isCurrent
		isCurrentByTime := isCurrent && st.MarkedIncompleteAt == nil
		isFuture := cls.isFuture
		status := cls.status
		isLocked := cls.isLocked

		var actions []string
		if canManage && isCurrent && !isCompleted && !isFuture {
			actions = append(actions, "complete")
			if st.MarkedIncompleteAt == nil {
				actions = append(actions, "mark_incomplete")
			}
		}

		lockReason := ""
		switch {
		case isCompleted:
			lockReason = "completed"
		case isFuture:
			lockReason = "not_started"
		case status == "overdue" || status == "past_incomplete":
			lockReason = "past_incomplete"
		case isLocked:
			lockReason = "not_current"
		}

		dto := DeadlineStepDTO{
			StepCode:         code,
			StepName:         displayStepName(snap),
			Order:            i + 1,
			DepartmentName:   snap.Department,
			PlannedStartDate: tl.StartDate.Format("2006-01-02"),
			PlannedEndDate:   tl.EndDate.Format("2006-01-02"),
			DurationDays:     tl.ProcessingDays,
			Status:           status,
			IsCurrentByTime:  isCurrentByTime,
			IsCompleted:      isCompleted,
			IsLocked:         isLocked,
			IsFuture:         isFuture,
			IsDelayed:        isDelayed,
			DelayDays:        st.DelayDaysApplied,
			AvailableActions: actions,
			LockReason:       lockReason,
		}
		if st.CompletedAt != nil {
			dto.CompletedAt = st.CompletedAt.UTC().Format(time.RFC3339)
		}
		if isCurrent {
			currentCode = code
		}
		steps = append(steps, dto)
	}

	return ListDeadlineStepsResponse{
		RecordID:        ctx.RecordID,
		CurrentStepCode: currentCode,
		Steps:           steps,
	}, nil
}

func snapshotToWorkflowSteps(snapshot []workflowapp.StepSnapshot) []disclosureapp.WorkflowStepDTO {
	out := make([]disclosureapp.WorkflowStepDTO, 0, len(snapshot))
	for _, s := range snapshot {
		stepID := stepCodeFromSnapshot(s)
		if stepID == "" {
			continue
		}
		out = append(out, disclosureapp.WorkflowStepDTO{
			StepID:         stepID,
			Stage:          s.Stage,
			DepartmentID:   s.Department,
			DueRule:        s.DueRule,
			ProcessingDays: s.ProcessingDays,
			DisplayOrder:   s.DisplayOrder,
		})
	}
	return out
}

func sortedSnapshot(snapshot []workflowapp.StepSnapshot) []workflowapp.StepSnapshot {
	if len(snapshot) == 0 {
		return nil
	}
	out := append([]workflowapp.StepSnapshot(nil), snapshot...)
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].DisplayOrder < out[i].DisplayOrder {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

func stepCodeFromSnapshot(s workflowapp.StepSnapshot) string {
	if c := strings.TrimSpace(s.StepCode); c != "" {
		return c
	}
	return strings.TrimSpace(s.StepID)
}

func displayStepName(s workflowapp.StepSnapshot) string {
	if n := strings.TrimSpace(s.Stage); n != "" {
		return n
	}
	return strings.TrimSpace(s.StepID)
}

func applyDelayShifts(
	base []disclosureapp.StepTimeline,
	snapshot []workflowapp.StepSnapshot,
	states map[string]StepRuntimeState,
) []disclosureapp.StepTimeline {
	if len(base) == 0 {
		return base
	}
	out := make([]disclosureapp.StepTimeline, len(base))
	copy(out, base)
	cumulative := 0
	for i, snap := range snapshot {
		code := stepCodeFromSnapshot(snap)
		st := states[code]
		if cumulative > 0 {
			out[i].StartDate = out[i].StartDate.AddDate(0, 0, cumulative)
			out[i].EndDate = out[i].EndDate.AddDate(0, 0, cumulative)
		}
		if st.DelayDaysApplied > 0 {
			cumulative += st.DelayDaysApplied
		}
	}
	return out
}

func resolveCurrentStepIndex(
	timelines []disclosureapp.StepTimeline,
	states map[string]StepRuntimeState,
	today time.Time,
) int {
	// Blocking incomplete step first.
	for i, tl := range timelines {
		code := tl.StepID
		st := states[code]
		if st.CompletedAt != nil {
			continue
		}
		if st.MarkedIncompleteAt != nil {
			return i
		}
		_ = tl
	}
	// Time window.
	for i, tl := range timelines {
		st := states[tl.StepID]
		if st.CompletedAt != nil {
			continue
		}
		if !today.Before(tl.StartDate) && !today.After(tl.EndDate) {
			return i
		}
	}
	// Overdue: first non-completed step whose end is before today.
	for i, tl := range timelines {
		st := states[tl.StepID]
		if st.CompletedAt != nil {
			continue
		}
		if today.After(tl.EndDate) {
			return i
		}
	}
	return -1
}

type stepClassification struct {
	status    string
	isFuture  bool
	isCurrent bool
	isLocked  bool
}

// classifyWorkflowStep assigns status using order/time relative to the current step.
// Past steps (before current or past planned_end) are never "not_started"/future.
func classifyWorkflowStep(
	index int,
	currentIdx int,
	tl disclosureapp.StepTimeline,
	st StepRuntimeState,
	today time.Time,
) stepClassification {
	isCompleted := st.CompletedAt != nil
	isBlockingIncomplete := st.MarkedIncompleteAt != nil && !isCompleted
	isCurrent := index == currentIdx && !isCompleted

	if isCompleted {
		return stepClassification{status: "completed", isFuture: false, isCurrent: false, isLocked: true}
	}
	if isBlockingIncomplete && isCurrent {
		return stepClassification{status: "incomplete", isFuture: false, isCurrent: true, isLocked: false}
	}
	if isCurrent {
		return stepClassification{status: "current", isFuture: false, isCurrent: true, isLocked: false}
	}

	// Steps before the current step are always in the past — never future.
	if currentIdx >= 0 && index < currentIdx {
		if today.After(tl.EndDate) {
			return stepClassification{status: "overdue", isFuture: false, isCurrent: false, isLocked: true}
		}
		return stepClassification{status: "past_incomplete", isFuture: false, isCurrent: false, isLocked: true}
	}

	if today.After(tl.EndDate) {
		return stepClassification{status: "overdue", isFuture: false, isCurrent: false, isLocked: true}
	}

	if today.Before(tl.StartDate) || (currentIdx >= 0 && index > currentIdx) {
		return stepClassification{status: "not_started", isFuture: true, isCurrent: false, isLocked: true}
	}

	return stepClassification{status: "past_incomplete", isFuture: false, isCurrent: false, isLocked: true}
}

// CurrentStepNameFromTimelines resolves display label for list cards (time-based).
func CurrentStepNameFromTimelines(
	snapshot []workflowapp.StepSnapshot,
	states map[string]StepRuntimeState,
	t0 time.Time,
	today time.Time,
	tz string,
) string {
	ctx := WorkflowInstanceContext{Snapshot: snapshot, T0Date: t0}
	resp, err := ComputeDeadlineSteps(ctx, states, today, tz, false)
	if err != nil || resp.CurrentStepCode == "" {
		return ""
	}
	for _, step := range resp.Steps {
		if step.StepCode == resp.CurrentStepCode {
			return step.StepName
		}
	}
	return ""
}
