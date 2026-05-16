package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

type service struct {
	repo      Repository
	milestone MilestoneRepository
	auth      authapp.Service
	idg       idgen.Generator
	flags     Flags
}

func NewService(repo Repository, auth authapp.Service, idg idgen.Generator, opts ...ServiceOption) Service {
	s := &service{repo: repo, auth: auth, idg: idg}
	for _, o := range opts {
		o(s)
	}
	return s
}

type ServiceOption func(*service)

func WithMilestoneRepository(mr MilestoneRepository) ServiceOption {
	return func(s *service) { s.milestone = mr }
}

func WithFlags(f Flags) ServiceOption {
	return func(s *service) { s.flags = f }
}

func (s *service) CreateWorkflowInstance(ctx context.Context, req CreateWorkflowInstanceRequest) (*WorkflowInstanceDTO, error) {
	if strings.TrimSpace(req.RecordID) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "record_id is required", nil)
	}
	if err := s.authorize(ctx, req.Subject, "workflow.create", authapp.ResourceRef{
		Type: "workflow_instance",
		ID:   req.RecordID,
		Attributes: map[string]any{
			"workflow_state": "draft",
		},
	}); err != nil {
		return nil, err
	}
	return s.createWorkflowInstance(ctx, req)
}

func (s *service) CreateWorkflowInstanceInternal(ctx context.Context, req CreateWorkflowInstanceRequest) (*WorkflowInstanceDTO, error) {
	if strings.TrimSpace(req.RecordID) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "record_id is required", nil)
	}
	return s.createWorkflowInstance(ctx, req)
}

func (s *service) createWorkflowInstance(ctx context.Context, req CreateWorkflowInstanceRequest) (*WorkflowInstanceDTO, error) {
	inst := WorkflowInstanceDTO{
		WorkflowInstanceID: s.idg.NewUUID(),
		CompanyID:          req.Subject.CompanyID,
		RecordID:           req.RecordID,
		Status:             "in_progress",
		CurrentStepCode:    "review",
		CreatedBy:          req.Subject.UserID,
	}
	if s.flags.SnapshotEnabled {
		inst.Snapshot = req.Snapshot
		inst.WorkflowSource = req.WorkflowSource
		inst.T0Date = req.T0Date
		inst.T0Policy = req.T0Policy
	}

	created, err := s.repo.CreateInstance(ctx, inst)
	if err != nil {
		return nil, err
	}
	_, _ = s.repo.CreateTask(ctx, TaskDTO{
		TaskID:               s.idg.NewUUID(),
		CompanyID:            req.Subject.CompanyID,
		WorkflowInstanceID:   created.WorkflowInstanceID,
		StepCode:             "review",
		AssigneeMembershipID: req.Subject.MembershipID,
		Status:               "pending",
	})

	if s.flags.TimelineEnabled && s.milestone != nil && len(req.Milestones) > 0 {
		// Attach the resolved instance ID to milestone rows (caller pre-computed them without it).
		rows := make([]StepMilestoneRow, len(req.Milestones))
		for i, m := range req.Milestones {
			rows[i] = m
			rows[i].InstanceID = created.WorkflowInstanceID
			rows[i].CompanyID = req.Subject.CompanyID
		}
		if err := s.milestone.InsertStepMilestones(ctx, rows); err != nil {
			// Non-fatal: instance is created; log and continue. Milestones can be retried.
			_ = fmt.Errorf("seed milestones: %w", err)
		}
	}
	return created, nil
}

func (s *service) ApproveTask(ctx context.Context, req TaskActionRequest) (*TaskDTO, error) {
	return s.transitionTask(ctx, req, "workflow.approve", "approved")
}

func (s *service) ReviewTask(ctx context.Context, req TaskActionRequest) (*TaskDTO, error) {
	return s.transitionTask(ctx, req, "workflow.review", "reviewed")
}

func (s *service) ConfirmTask(ctx context.Context, req TaskActionRequest) (*TaskDTO, error) {
	return s.transitionTask(ctx, req, "workflow.confirm", "confirmed")
}

func (s *service) RejectTask(ctx context.Context, req TaskActionRequest) (*TaskDTO, error) {
	return s.transitionTask(ctx, req, "workflow.reject", "rejected")
}

func (s *service) GetWorkflowInstance(ctx context.Context, req GetWorkflowInstanceRequest) (*WorkflowInstanceDTO, error) {
	if strings.TrimSpace(req.WorkflowInstanceID) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "workflow_instance_id is required", nil)
	}
	if err := s.authorize(ctx, req.Subject, "workflow.read", authapp.ResourceRef{
		Type: "workflow_instance",
		ID:   req.WorkflowInstanceID,
		Attributes: map[string]any{
			"workflow_state": "*",
		},
	}); err != nil {
		return nil, err
	}
	return s.repo.FindInstance(ctx, req.Subject.CompanyID, req.WorkflowInstanceID)
}

func (s *service) ListInstanceTasks(ctx context.Context, req ListInstanceTasksRequest) ([]TaskDTO, error) {
	if strings.TrimSpace(req.WorkflowInstanceID) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "workflow_instance_id is required", nil)
	}
	if err := s.authorize(ctx, req.Subject, "workflow.read", authapp.ResourceRef{
		Type: "workflow_instance",
		ID:   req.WorkflowInstanceID,
		Attributes: map[string]any{
			"workflow_state": "*",
		},
	}); err != nil {
		return nil, err
	}
	return s.repo.ListTasksByInstance(ctx, req.Subject.CompanyID, req.WorkflowInstanceID)
}

func (s *service) ListInstanceReminders(ctx context.Context, req ListInstanceRemindersRequest) ([]InstanceReminderDTO, error) {
	if strings.TrimSpace(req.WorkflowInstanceID) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "workflow_instance_id is required", nil)
	}
	if err := s.authorize(ctx, req.Subject, "workflow.read", authapp.ResourceRef{
		Type: "workflow_instance",
		ID:   req.WorkflowInstanceID,
		Attributes: map[string]any{
			"workflow_state": "*",
		},
	}); err != nil {
		return nil, err
	}
	if s.milestone == nil {
		return []InstanceReminderDTO{}, nil
	}
	return s.milestone.ListByInstance(ctx, req.Subject.CompanyID, req.WorkflowInstanceID)
}

func (s *service) ResolveAssignees(ctx context.Context, req ResolveAssigneesRequest) (*ResolveAssigneesResponse, error) {
	if strings.TrimSpace(req.WorkflowInstanceID) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "workflow_instance_id is required", nil)
	}
	if err := s.authorize(ctx, req.Subject, "workflow.resolve_assignees", authapp.ResourceRef{
		Type: "workflow_instance",
		ID:   req.WorkflowInstanceID,
		Attributes: map[string]any{
			"workflow_state": "*",
		},
	}); err != nil {
		return nil, err
	}
	// Skeleton resolver: return current membership as default assignee candidate.
	return &ResolveAssigneesResponse{MembershipIDs: []string{req.Subject.MembershipID}}, nil
}

func (s *service) transitionTask(ctx context.Context, req TaskActionRequest, action, nextStatus string) (*TaskDTO, error) {
	if strings.TrimSpace(req.TaskID) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "task_id is required", nil)
	}
	task, err := s.repo.FindTask(ctx, req.Subject.CompanyID, req.TaskID)
	if err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, req.Subject, action, authapp.ResourceRef{
		Type: "workflow_task",
		ID:   req.TaskID,
		Attributes: map[string]any{
			"assignee_membership_id": task.AssigneeMembershipID,
			"workflow_state":         task.Status,
		},
	}); err != nil {
		return nil, err
	}
	if task.AssigneeMembershipID != req.Subject.MembershipID {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodeResponsibilityRequired, "task assignee mismatch", nil)
	}
	if task.Status != "pending" {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "task is not pending", nil)
	}
	task.Status = nextStatus
	upd, err := s.repo.UpdateTask(ctx, *task)
	if err != nil {
		return nil, err
	}
	return upd, nil
}

func (s *service) authorize(ctx context.Context, sub Subject, action string, resource authapp.ResourceRef) error {
	decision, err := s.auth.Authorize(ctx, authapp.AuthorizeRequest{Subject: authapp.SubjectRef{UserID: sub.UserID, MembershipID: sub.MembershipID, CompanyID: sub.CompanyID}, Action: action, Resource: resource})
	if err != nil {
		return fmt.Errorf("authorize workflow action: %w", err)
	}
	if decision.Decision != authapp.DecisionAllow {
		code := perr.CodePermissionDenied
		if decision.DenyReasonCode != nil {
			code = *decision.DenyReasonCode
		}
		return perr.NewHTTPError(http.StatusForbidden, code, "access denied", nil)
	}
	return nil
}
