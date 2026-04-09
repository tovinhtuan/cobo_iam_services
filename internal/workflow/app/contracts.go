package app

import "context"

type Service interface {
	CreateWorkflowInstance(ctx context.Context, req CreateWorkflowInstanceRequest) (*WorkflowInstanceDTO, error)
	ApproveTask(ctx context.Context, req TaskActionRequest) (*TaskDTO, error)
	ReviewTask(ctx context.Context, req TaskActionRequest) (*TaskDTO, error)
	ConfirmTask(ctx context.Context, req TaskActionRequest) (*TaskDTO, error)
	ResolveAssignees(ctx context.Context, req ResolveAssigneesRequest) (*ResolveAssigneesResponse, error)
}

type Repository interface {
	CreateInstance(ctx context.Context, in WorkflowInstanceDTO) (*WorkflowInstanceDTO, error)
	FindInstance(ctx context.Context, companyID, workflowInstanceID string) (*WorkflowInstanceDTO, error)
	UpdateInstance(ctx context.Context, in WorkflowInstanceDTO) (*WorkflowInstanceDTO, error)
	CreateTask(ctx context.Context, task TaskDTO) (*TaskDTO, error)
	FindTask(ctx context.Context, companyID, taskID string) (*TaskDTO, error)
	UpdateTask(ctx context.Context, task TaskDTO) (*TaskDTO, error)
	ListTasksByInstance(ctx context.Context, companyID, workflowInstanceID string) ([]TaskDTO, error)
}

type Subject struct {
	UserID       string
	MembershipID string
	CompanyID    string
}

type CreateWorkflowInstanceRequest struct {
	Subject  Subject
	RecordID string `json:"record_id"`
}

type TaskActionRequest struct {
	Subject Subject
	TaskID  string
	Comment string `json:"comment,omitempty"`
}

type ResolveAssigneesRequest struct {
	Subject            Subject
	WorkflowInstanceID string `json:"workflow_instance_id"`
	StepCode           string `json:"step_code"`
}

type ResolveAssigneesResponse struct {
	MembershipIDs []string `json:"membership_ids"`
}

type WorkflowInstanceDTO struct {
	WorkflowInstanceID string `json:"workflow_instance_id"`
	CompanyID          string `json:"company_id"`
	RecordID           string `json:"record_id"`
	Status             string `json:"status"`
	CurrentStepCode    string `json:"current_step_code"`
	CreatedBy          string `json:"created_by"`
}

type TaskDTO struct {
	TaskID               string `json:"task_id"`
	CompanyID            string `json:"company_id"`
	WorkflowInstanceID   string `json:"workflow_instance_id"`
	StepCode             string `json:"step_code"`
	AssigneeMembershipID string `json:"assignee_membership_id"`
	Status               string `json:"status"`
}
