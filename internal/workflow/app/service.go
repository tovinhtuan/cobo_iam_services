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
	repo Repository
	auth authapp.Service
	idg  idgen.Generator
}

func NewService(repo Repository, auth authapp.Service, idg idgen.Generator) Service {
	return &service{repo: repo, auth: auth, idg: idg}
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
	inst := WorkflowInstanceDTO{WorkflowInstanceID: s.idg.NewUUID(), CompanyID: req.Subject.CompanyID, RecordID: req.RecordID, Status: "in_progress", CurrentStepCode: "review", CreatedBy: req.Subject.UserID}
	created, err := s.repo.CreateInstance(ctx, inst)
	if err != nil {
		return nil, err
	}
	_, _ = s.repo.CreateTask(ctx, TaskDTO{TaskID: s.idg.NewUUID(), CompanyID: req.Subject.CompanyID, WorkflowInstanceID: created.WorkflowInstanceID, StepCode: "review", AssigneeMembershipID: req.Subject.MembershipID, Status: "pending"})
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
