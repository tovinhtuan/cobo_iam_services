package app

import (
	"fmt"
	"sort"
	"strings"

	adhocapp "github.com/cobo/cobo_iam_services/internal/adhoc/app"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	workflowerrs "github.com/cobo/cobo_iam_services/internal/workflow/errs"
)

// ErrEmptyWorkflowSnapshot is re-exported for callers that already depend on workflow/app.
var ErrEmptyWorkflowSnapshot = workflowerrs.ErrEmptyWorkflowSnapshot

// MapEffectiveWorkflowToSnapshot converts effective workflow steps into frozen snapshot rows.
func MapEffectiveWorkflowToSnapshot(steps []disclosureapp.WorkflowStepDTO, workflowSource string) []StepSnapshot {
	_ = strings.TrimSpace(workflowSource)
	out := make([]StepSnapshot, 0, len(steps))
	for _, step := range steps {
		stepID := strings.TrimSpace(step.StepID)
		if stepID == "" {
			continue
		}
		stepCode := stepID
		assigneeRole := ""
		if len(step.AssigneeRoleIds) > 0 {
			assigneeRole = strings.Join(step.AssigneeRoleIds, ",")
		}
		department := strings.TrimSpace(step.DepartmentID)
		if department == "" {
			department = strings.TrimSpace(step.Stage)
		}
		dueRule := strings.TrimSpace(step.DueRule)
		if dueRule == "" {
			dueRule = "T+N"
		}
		out = append(out, StepSnapshot{
			StepID:         stepID,
			StepCode:       stepCode,
			Stage:          strings.TrimSpace(step.Stage),
			Department:     department,
			AssigneeRole:   assigneeRole,
			DueRule:        dueRule,
			DisplayOrder:   step.DisplayOrder,
			ProcessingDays: step.ProcessingDays,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].DisplayOrder == out[j].DisplayOrder {
			return out[i].StepID < out[j].StepID
		}
		return out[i].DisplayOrder < out[j].DisplayOrder
	})
	return out
}

// ApplyAdHocStepOverrides returns a copy of snapshot with processing_days patched by step_id.
func ApplyAdHocStepOverrides(snapshot []StepSnapshot, overrides []adhocapp.WorkflowStepOverride) []StepSnapshot {
	if len(snapshot) == 0 || len(overrides) == 0 {
		return append([]StepSnapshot(nil), snapshot...)
	}
	byStep := make(map[string]int, len(overrides))
	for _, o := range overrides {
		id := strings.TrimSpace(o.StepID)
		if id == "" {
			continue
		}
		byStep[id] = o.ProcessingDays
	}
	out := make([]StepSnapshot, len(snapshot))
	for i, step := range snapshot {
		out[i] = step
		if days, ok := byStep[step.StepID]; ok {
			out[i].ProcessingDays = days
		}
	}
	return out
}

// FirstStepCode returns the step_code of the snapshot step with the smallest display_order.
func FirstStepCode(snapshot []StepSnapshot) string {
	if len(snapshot) == 0 {
		return ""
	}
	best := snapshot[0]
	for _, step := range snapshot[1:] {
		if step.DisplayOrder < best.DisplayOrder {
			best = step
		} else if step.DisplayOrder == best.DisplayOrder && step.StepID < best.StepID {
			best = step
		}
	}
	code := strings.TrimSpace(best.StepCode)
	if code == "" {
		code = strings.TrimSpace(best.StepID)
	}
	return code
}

// ValidateSnapshot ensures at least one step exists when a snapshot is required.
func ValidateSnapshot(snapshot []StepSnapshot) error {
	if len(snapshot) == 0 {
		return fmt.Errorf("%w", workflowerrs.ErrEmptyWorkflowSnapshot)
	}
	return nil
}

// IsEmptyEffectiveWorkflow reports whether err is an empty effective workflow (ad-hoc 4xx or worker skip).
func IsEmptyEffectiveWorkflow(err error) bool {
	return workflowerrs.IsEmptyEffectiveWorkflow(err)
}
