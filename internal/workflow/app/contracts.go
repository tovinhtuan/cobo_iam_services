package app

import (
	"context"
	"time"
)

type Service interface {
	CreateWorkflowInstance(ctx context.Context, req CreateWorkflowInstanceRequest) (*WorkflowInstanceDTO, error)
	// CreateWorkflowInstanceInternal skips HTTP-layer authorization for trusted in-process callers.
	CreateWorkflowInstanceInternal(ctx context.Context, req CreateWorkflowInstanceRequest) (*WorkflowInstanceDTO, error)
	GetWorkflowInstance(ctx context.Context, req GetWorkflowInstanceRequest) (*WorkflowInstanceDTO, error)
	ListInstanceTasks(ctx context.Context, req ListInstanceTasksRequest) ([]TaskDTO, error)
	ListInstanceReminders(ctx context.Context, req ListInstanceRemindersRequest) ([]InstanceReminderDTO, error)
	ApproveTask(ctx context.Context, req TaskActionRequest) (*TaskDTO, error)
	ReviewTask(ctx context.Context, req TaskActionRequest) (*TaskDTO, error)
	ConfirmTask(ctx context.Context, req TaskActionRequest) (*TaskDTO, error)
	RejectTask(ctx context.Context, req TaskActionRequest) (*TaskDTO, error)
	ResolveAssignees(ctx context.Context, req ResolveAssigneesRequest) (*ResolveAssigneesResponse, error)
}

// RecordStatusUpdater advances disclosure record status when workflow completes.
type RecordStatusUpdater interface {
	MarkRecordApproved(ctx context.Context, companyID, recordID, actorUserID string) error
}

// WorkflowNotifier emits workflow lifecycle emails.
type WorkflowNotifier interface {
	NotifyWorkflowApproved(ctx context.Context, companyID, recordID, workflowInstanceID, actorMembershipID string) error
}

type Repository interface {
	CreateInstance(ctx context.Context, in WorkflowInstanceDTO) (*WorkflowInstanceDTO, error)
	FindInstance(ctx context.Context, companyID, workflowInstanceID string) (*WorkflowInstanceDTO, error)
	UpdateInstance(ctx context.Context, in WorkflowInstanceDTO) (*WorkflowInstanceDTO, error)
	CreateTask(ctx context.Context, task TaskDTO) (*TaskDTO, error)
	FindTask(ctx context.Context, companyID, taskID string) (*TaskDTO, error)
	UpdateTask(ctx context.Context, task TaskDTO) (*TaskDTO, error)
	ListTasksByInstance(ctx context.Context, companyID, workflowInstanceID string) ([]TaskDTO, error)
	// ApplyTaskTransition atomically updates a pending task and optionally creates the next
	// task + updates the instance (v2 multi-step) or marks the instance terminal.
	ApplyTaskTransition(ctx context.Context, in TaskTransitionApply) (*TaskDTO, error)
}

// TaskTransitionApply is one atomic pending→terminal task transition.
type TaskTransitionApply struct {
	CompanyID  string
	TaskID     string
	FromStatus string
	ToStatus   string
	// NextTask, when set, is inserted in the same transaction; instance stays non-terminal.
	NextTask *TaskDTO
	// Instance, when set, updates status + current_step_code in the same transaction.
	Instance *WorkflowInstanceDTO
}

// MilestoneRepository persists workflow step milestone rows seeded at instance creation.
type MilestoneRepository interface {
	InsertStepMilestones(ctx context.Context, rows []StepMilestoneRow) error
	ListByInstance(ctx context.Context, companyID, workflowInstanceID string) ([]InstanceReminderDTO, error)
}

type GetWorkflowInstanceRequest struct {
	Subject            Subject
	WorkflowInstanceID string
}

type ListInstanceTasksRequest struct {
	Subject            Subject
	WorkflowInstanceID string
}

type ListInstanceRemindersRequest struct {
	Subject            Subject
	WorkflowInstanceID string
}

type InstanceReminderDTO struct {
	ReminderID         string `json:"reminder_id"`
	WorkflowInstanceID string `json:"workflow_instance_id"`
	StepID             string `json:"step_id"`
	StepIndex          int    `json:"step_index"`
	MilestoneType      string `json:"milestone_type"`
	ReminderAt         string `json:"reminder_at"`
	Status             string `json:"status"`
}

// Flags carries feature-flag state for the workflow service.
type Flags struct {
	SnapshotEnabled bool
	TimelineEnabled bool
	// AssigneeResolutionEnabled gates strict assignee resolution (Batch 4). Default OFF.
	AssigneeResolutionEnabled bool
}

// AssigneeResolver resolves a workflow step's assignees to membership ids. It MUST NOT fall back to
// the current user. Returns (membershipIDs, resolved); resolved=false means controlled-unresolved.
// Satisfied by internal/workflowconfig/app.WorkflowAssigneeResolverAdapter (structural).
type AssigneeResolver interface {
	ResolveStepAssignees(ctx context.Context, companyID, stepCode, roleID string, explicitMembershipIDs []string) (membershipIDs []string, resolved bool)
}

type Subject struct {
	UserID       string
	MembershipID string
	CompanyID    string
}

// StepSnapshot is the workflow step data frozen into workflow_instances.snapshot_json.
type StepSnapshot struct {
	StepID       string `json:"step_id"`
	StepCode     string `json:"step_code,omitempty"`
	Stage        string `json:"stage,omitempty"`
	Department   string `json:"department,omitempty"`
	AssigneeRole string `json:"assignee_role,omitempty"`
	// AssigneeMembershipID is the direct handler for proposal_snapshot_v2 steps (model A).
	// Empty for legacy template-derived snapshots.
	AssigneeMembershipID string `json:"assignee_membership_id,omitempty"`
	DueRule              string `json:"due_rule,omitempty"`
	DisplayOrder         int    `json:"display_order"`
	ProcessingDays       int    `json:"processing_days,omitempty"`
}

// StepMilestoneRow is a single milestone row ready for persistence.
type StepMilestoneRow struct {
	MilestoneID   string
	CompanyID     string
	InstanceID    string
	StepID        string
	StepOrder     int
	MilestoneType string
	ScheduledDate time.Time
}

type CreateWorkflowInstanceRequest struct {
	Subject  Subject
	RecordID string `json:"record_id"`
	// Optional: filled by caller when WORKFLOW_SNAPSHOT_ENABLED=true.
	Snapshot       []StepSnapshot `json:"snapshot,omitempty"`
	WorkflowSource string         `json:"workflow_source,omitempty"` // system_template | company_override | proposal_snapshot_v2
	T0Date         *time.Time     `json:"t0_date,omitempty"`
	T0Policy       string         `json:"t0_policy,omitempty"`
	// Optional: pre-computed milestones from timeline computation (when WORKFLOW_TIMELINE_ENABLED=true).
	Milestones []StepMilestoneRow `json:"milestones,omitempty"`
	// FirstTaskAssigneeMembershipID, when non-empty, assigns the first pending task to this
	// membership (schema v2 direct assignee). When empty, legacy behavior uses Subject.MembershipID
	// (proposal creator for ad-hoc finalize).
	FirstTaskAssigneeMembershipID string `json:"first_task_assignee_membership_id,omitempty"`
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
	WorkflowInstanceID string         `json:"workflow_instance_id"`
	CompanyID          string         `json:"company_id"`
	RecordID           string         `json:"record_id"`
	Status             string         `json:"status"`
	CurrentStepCode    string         `json:"current_step_code"`
	CreatedBy          string         `json:"created_by"`
	Snapshot           []StepSnapshot `json:"snapshot,omitempty"`
	WorkflowSource     string         `json:"workflow_source,omitempty"`
	T0Date             *time.Time     `json:"t0_date,omitempty"`
	T0Policy           string         `json:"t0_policy,omitempty"`
}

// TaskAssigneeDTO is additive display metadata for portal responsibility views.
// Does not expose password, tokens, or private profile fields.
type TaskAssigneeDTO struct {
	MembershipID string `json:"membership_id,omitempty"`
	// ActorType is empty for normal members; "SYSTEM" for synthetic/system assignees
	// (e.g. m_system_oneshot) that may not resolve to a users row.
	ActorType      string `json:"actor_type,omitempty"`
	DisplayName    string `json:"display_name,omitempty"`
	Email          string `json:"email,omitempty"`
	DepartmentName string `json:"department_name,omitempty"`
}

type TaskDTO struct {
	TaskID               string           `json:"task_id"`
	CompanyID            string           `json:"company_id"`
	WorkflowInstanceID   string           `json:"workflow_instance_id"`
	StepCode             string           `json:"step_code"`
	AssigneeMembershipID string           `json:"assignee_membership_id"`
	Status               string           `json:"status"`
	Assignee             *TaskAssigneeDTO `json:"assignee,omitempty"`
}
