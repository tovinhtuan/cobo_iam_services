package app

import (
	"context"
	"sort"
	"strings"
)

// Tracking step UI states derived from runtime authority (never persisted as proposal.status).
const (
	TrackingStepCompleted = "COMPLETED"
	TrackingStepActive    = "ACTIVE"
	TrackingStepFuture    = "FUTURE"
	TrackingStepRejected  = "REJECTED"
)

// TrackingAssigneeSource distinguishes runtime handlers vs frozen plan display.
const (
	TrackingAssigneeSourceRuntime    = "runtime"
	TrackingAssigneeSourceFrozenPlan = "frozen_plan"
)

// WorkflowRuntimeReader loads instance + tasks for proposal detail tracking projection.
// Optional — nil means tracking is omitted (fail-soft).
type WorkflowRuntimeReader interface {
	FindInstanceRuntime(ctx context.Context, companyID, workflowInstanceID string) (currentStepCode, status string, err error)
	ListInstanceTasks(ctx context.Context, companyID, workflowInstanceID string) ([]RuntimeTaskView, error)
}

// RuntimeTaskView is a minimal task projection for tracking (no write side effects).
type RuntimeTaskView struct {
	TaskID                string
	StepCode              string
	Status                string
	AssigneeMembershipID  string
	AssigneeMembershipIDs []string
}

// TrackingAssigneeDTO is one handler shown on detail tracking.
type TrackingAssigneeDTO struct {
	MembershipID string `json:"membership_id"`
	Source       string `json:"source,omitempty"`
}

// TrackingStepDTO is one frozen-plan step with derived runtime state.
type TrackingStepDTO struct {
	StepID         string                `json:"step_id"`
	Order          int                   `json:"order"`
	Name           string                `json:"name"`
	DepartmentID   string                `json:"department_id,omitempty"`
	ProcessingDays int                   `json:"processing_days"`
	Status         string                `json:"status"`
	Assignees      []TrackingAssigneeDTO `json:"assignees,omitempty"`
	TaskID         string                `json:"task_id,omitempty"`
	TaskStatus     string                `json:"task_status,omitempty"`
}

// ProposalTrackingDTO is additive detail enrichment for creator/runtime tracking.
// Does not change proposal.status lifecycle.
type ProposalTrackingDTO struct {
	TotalSteps       int               `json:"total_steps"`
	CompletedSteps   int               `json:"completed_steps"`
	CurrentStepOrder *int              `json:"current_step_order,omitempty"`
	InstanceStatus   string            `json:"instance_status,omitempty"`
	HasRuntime       bool              `json:"has_runtime"`
	CurrentStep      *TrackingStepDTO  `json:"current_step,omitempty"`
	Steps            []TrackingStepDTO `json:"steps"`
}

// AttachRuntimeTracking injects the optional workflow runtime reader without changing NewService.
func AttachRuntimeTracking(svc Service, reader WorkflowRuntimeReader) Service {
	if s, ok := svc.(*service); ok {
		s.runtime = reader
	}
	return svc
}

// BuildProposalTracking projects frozen workflow + optional runtime into detail tracking.
// Authority:
//   - current step = instance.current_step_code (when hasRuntime)
//   - active assignees = workflow task relation (v3) or singular (v2)
//   - future assignees = frozen plan only (never presented as active by callers)
func BuildProposalTracking(
	proposalStatus string,
	snap *ProposalWorkflowSnapshot,
	hasRuntime bool,
	currentStepCode string,
	instanceStatus string,
	tasks []RuntimeTaskView,
) *ProposalTrackingDTO {
	if snap == nil || len(snap.Steps) == 0 {
		return nil
	}
	steps := append([]ProposalWorkflowStep(nil), snap.Steps...)
	sort.SliceStable(steps, func(i, j int) bool {
		if steps[i].Order == steps[j].Order {
			return strings.TrimSpace(steps[i].ID) < strings.TrimSpace(steps[j].ID)
		}
		return steps[i].Order < steps[j].Order
	})

	taskByStep := map[string]RuntimeTaskView{}
	for _, t := range tasks {
		code := strings.TrimSpace(t.StepCode)
		if code == "" {
			continue
		}
		// Prefer pending task when duplicates exist; otherwise keep first.
		if prev, ok := taskByStep[code]; ok {
			if prev.Status == "pending" {
				continue
			}
			if t.Status != "pending" {
				continue
			}
		}
		taskByStep[code] = t
	}

	currentCode := strings.TrimSpace(currentStepCode)
	instanceStatus = strings.TrimSpace(instanceStatus)
	schema := snap.SchemaVersion

	// Without runtime (draft / pending review / approved without instance yet): plan-only FUTURE.
	if !hasRuntime {
		outSteps := make([]TrackingStepDTO, 0, len(steps))
		for _, step := range steps {
			outSteps = append(outSteps, trackingStepFromFrozen(step, schema, TrackingStepFuture, nil))
		}
		return &ProposalTrackingDTO{
			TotalSteps:     len(outSteps),
			CompletedSteps: 0,
			HasRuntime:     false,
			Steps:          outSteps,
		}
	}

	currentIdx := -1
	for i, step := range steps {
		if strings.TrimSpace(step.ID) == currentCode {
			currentIdx = i
			break
		}
	}

	workflowDone := instanceStatus == "approved"
	workflowRejected := instanceStatus == "rejected"

	outSteps := make([]TrackingStepDTO, 0, len(steps))
	completed := 0
	var current *TrackingStepDTO
	var currentOrder *int

	for i, step := range steps {
		stepID := strings.TrimSpace(step.ID)
		task, hasTask := taskByStep[stepID]
		status := TrackingStepFuture

		switch {
		case workflowDone:
			status = TrackingStepCompleted
		case workflowRejected:
			if currentIdx >= 0 {
				if i < currentIdx {
					status = TrackingStepCompleted
				} else if i == currentIdx {
					status = TrackingStepRejected
				} else {
					status = TrackingStepFuture
				}
			} else if hasTask && task.Status == "approved" {
				status = TrackingStepCompleted
			} else if hasTask && task.Status == "rejected" {
				status = TrackingStepRejected
			} else {
				status = TrackingStepFuture
			}
		default:
			if hasTask && task.Status == "approved" {
				status = TrackingStepCompleted
			} else if currentIdx >= 0 {
				if i < currentIdx {
					status = TrackingStepCompleted
				} else if i == currentIdx {
					status = TrackingStepActive
				} else {
					status = TrackingStepFuture
				}
			} else if hasTask && task.Status == "pending" {
				status = TrackingStepActive
			} else if proposalStatus == StatusApproved && i == 0 && currentCode == "" {
				// Defensive: approved with empty current — do not invent active.
				status = TrackingStepFuture
			} else {
				status = TrackingStepFuture
			}
		}

		var dto TrackingStepDTO
		if status == TrackingStepActive && hasTask {
			dto = trackingStepFromRuntime(step, schema, status, task)
		} else if status == TrackingStepCompleted && hasTask {
			dto = trackingStepFromRuntime(step, schema, status, task)
			if len(dto.Assignees) == 0 {
				dto = trackingStepFromFrozen(step, schema, status, &task)
			}
		} else if status == TrackingStepRejected && hasTask {
			dto = trackingStepFromRuntime(step, schema, status, task)
		} else {
			var tPtr *RuntimeTaskView
			if hasTask {
				tPtr = &task
			}
			dto = trackingStepFromFrozen(step, schema, status, tPtr)
		}

		if status == TrackingStepCompleted {
			completed++
		}
		if status == TrackingStepActive {
			cp := dto
			current = &cp
			order := dto.Order
			currentOrder = &order
		}
		outSteps = append(outSteps, dto)
	}

	return &ProposalTrackingDTO{
		TotalSteps:       len(outSteps),
		CompletedSteps:   completed,
		CurrentStepOrder: currentOrder,
		InstanceStatus:   instanceStatus,
		HasRuntime:       true,
		CurrentStep:      current,
		Steps:            outSteps,
	}
}

func trackingStepFromFrozen(step ProposalWorkflowStep, schema int, status string, task *RuntimeTaskView) TrackingStepDTO {
	ids := EffectiveAssigneeMembershipIDs(step, schema)
	assignees := make([]TrackingAssigneeDTO, 0, len(ids))
	for _, id := range ids {
		assignees = append(assignees, TrackingAssigneeDTO{
			MembershipID: id,
			Source:       TrackingAssigneeSourceFrozenPlan,
		})
	}
	dto := TrackingStepDTO{
		StepID:         strings.TrimSpace(step.ID),
		Order:          step.Order,
		Name:           strings.TrimSpace(step.Name),
		DepartmentID:   strings.TrimSpace(step.DepartmentID),
		ProcessingDays: step.ProcessingDays,
		Status:         status,
		Assignees:      assignees,
	}
	if task != nil {
		dto.TaskID = task.TaskID
		dto.TaskStatus = task.Status
	}
	return dto
}

func trackingStepFromRuntime(step ProposalWorkflowStep, schema int, status string, task RuntimeTaskView) TrackingStepDTO {
	ids := runtimeAssigneeIDs(task)
	if len(ids) == 0 {
		// Fallback to frozen only when runtime has no assignees (should be rare).
		return trackingStepFromFrozen(step, schema, status, &task)
	}
	assignees := make([]TrackingAssigneeDTO, 0, len(ids))
	for _, id := range ids {
		assignees = append(assignees, TrackingAssigneeDTO{
			MembershipID: id,
			Source:       TrackingAssigneeSourceRuntime,
		})
	}
	return TrackingStepDTO{
		StepID:         strings.TrimSpace(step.ID),
		Order:          step.Order,
		Name:           strings.TrimSpace(step.Name),
		DepartmentID:   strings.TrimSpace(step.DepartmentID),
		ProcessingDays: step.ProcessingDays,
		Status:         status,
		Assignees:      assignees,
		TaskID:         task.TaskID,
		TaskStatus:     task.Status,
	}
}

func runtimeAssigneeIDs(task RuntimeTaskView) []string {
	ids := normalizeAssigneeIDList(task.AssigneeMembershipIDs)
	if len(ids) > 0 {
		return ids
	}
	if singular := strings.TrimSpace(task.AssigneeMembershipID); singular != "" {
		return []string{singular}
	}
	return nil
}
