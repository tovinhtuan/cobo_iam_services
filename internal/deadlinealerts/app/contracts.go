package app

import (
	"context"
	"time"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
)

type Subject struct {
	UserID       string
	MembershipID string
	CompanyID    string
}

type ListDeadlineAlertsRequest struct {
	Subject          Subject
	Status           string // UPCOMING|DUE_SOON|OVERDUE|PENDING_CONFIRM|DONE or FE labels
	Query            string
	StartDate        string // YYYY-MM-DD
	EndDate          string // YYYY-MM-DD
	DepartmentID     string // optional: current-step / record department filter
	DisplayGroupCode string // optional: template_display_groups filter
	Page             int
	PageSize         int
}

type DeadlineAlertFilterOptionDTO struct {
	ID   string `json:"id"`
	Code string `json:"code,omitempty"`
	Name string `json:"name"`
}

type DeadlineAlertFilterOptionsResponse struct {
	Departments  []DeadlineAlertFilterOptionDTO `json:"departments"`
	ReportGroups []DeadlineAlertFilterOptionDTO `json:"report_groups"`
}

type DeadlineAlertDTO struct {
	AlertID            string   `json:"alert_id"`
	RecordID           string   `json:"record_id"`
	WorkflowInstanceID string   `json:"workflow_instance_id,omitempty"`
	TypeID             string   `json:"type_id"`
	Title              string   `json:"title"`
	DueDate            string   `json:"due_date"`
	Status             string   `json:"status"` // UPCOMING|DUE_SOON|OVERDUE|PENDING_CONFIRM|DONE
	ActiveDepartments  []string `json:"active_departments"`
	CurrentStepName    string   `json:"current_step_name,omitempty"`
	Source             string   `json:"source"`
	TemplateCategory   string   `json:"template_category,omitempty"`
}

type ListDeadlineAlertsResponse struct {
	Items    []DeadlineAlertDTO `json:"items"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
	Total    int                `json:"total"`
}

type ConfirmDeadlineAlertRequest struct {
	Subject        Subject
	RecordID       string
	Note           string
	IdempotencyKey string
}

type ConfirmDeadlineAlertResponse struct {
	RecordID    string `json:"record_id"`
	CompanyID   string `json:"company_id"`
	ConfirmedBy string `json:"confirmed_by"`
	ConfirmedAt string `json:"confirmed_at"`
}

// AlertRow is a DB read model before due-date enrichment.
type AlertRow struct {
	CompanyID          string
	RecordID           string
	TypeID             string
	Title              string
	TypeName           string
	AdHocTitleLine     string
	RecordStatus       string
	RecordDepartmentID string
	PlannedDate        string
	HasTaskAssignee    bool
	WorkflowInstanceID string
	CurrentStepCode        string
	CurrentStepDepartment  string
	CurrentStepName        string
	SnapshotJSON           []byte // populated only when full snapshot is required (not list query)
	AdHocDeadlineDate  string
	TemplateCategory   string
	DeadlineConfigJSON []byte
	ConfirmedBy        string
	ConfirmedAt        *time.Time
}

type WorkflowInstanceRow struct {
	WorkflowInstanceID string
	T0Date             time.Time
	SnapshotJSON       []byte
	Timezone           string
}

type Repository interface {
	ListRows(ctx context.Context, companyID string, scope DeadlineAlertAccessScope) ([]AlertRow, error)
	ListDisplayGroupCodesByTypeIDs(ctx context.Context, typeIDs []string) (map[string][]string, error)
	ListCompanyDepartments(ctx context.Context, companyID string) ([]DeadlineAlertFilterOptionDTO, error)
	ListTemplateDepartments(ctx context.Context) ([]DeadlineAlertFilterOptionDTO, error)
	ListReportGroupOptions(ctx context.Context) ([]DeadlineAlertFilterOptionDTO, error)
	GetCompanyDeadlineContext(ctx context.Context, companyID string) (disclosureapp.CompanyDeadlineContext, error)
	// GetCompanyTypeDeadlineContext returns company context enriched with
	// per-company cycle anchor override for the given type.
	GetCompanyTypeDeadlineContext(ctx context.Context, companyID, typeID string) (disclosureapp.CompanyDeadlineContext, error)
	GetTypeDeadlineConfig(ctx context.Context, companyID, typeID string) (*disclosureapp.TemplateDeadlineConfig, error)
	HasDisclosureRecord(ctx context.Context, companyID, recordID string) (bool, error)
	ConfirmDeadlineAlert(ctx context.Context, companyID, recordID, confirmedBy, note, idempotencyKey string, at time.Time) error
	GetWorkflowInstanceByRecord(ctx context.Context, companyID, recordID string) (*WorkflowInstanceRow, error)
	GetEffectiveWorkflowSnapshot(ctx context.Context, companyID, typeID string) ([]workflowapp.StepSnapshot, error)
	ListStepStates(ctx context.Context, workflowInstanceID string) (map[string]StepRuntimeState, error)
	UpsertStepCompleted(ctx context.Context, companyID, workflowInstanceID, stepCode, membershipID string, at time.Time) error
	UpsertStepIncomplete(ctx context.Context, companyID, workflowInstanceID, stepCode, membershipID, reason string, delayDays int, at time.Time) error
}

type Service interface {
	ListDeadlineAlerts(ctx context.Context, req ListDeadlineAlertsRequest) (*ListDeadlineAlertsResponse, error)
	ListDeadlineAlertFilterOptions(ctx context.Context, sub Subject) (*DeadlineAlertFilterOptionsResponse, error)
	ConfirmDeadlineAlert(ctx context.Context, req ConfirmDeadlineAlertRequest) (*ConfirmDeadlineAlertResponse, error)
	ListDeadlineSteps(ctx context.Context, sub Subject, recordID string) (*ListDeadlineStepsResponse, error)
	CompleteDeadlineStep(ctx context.Context, req CompleteStepRequest) (*ListDeadlineStepsResponse, error)
	MarkDeadlineStepIncomplete(ctx context.Context, req MarkIncompleteStepRequest) (*ListDeadlineStepsResponse, error)
}

type service struct {
	repo       Repository
	auth       authapp.Service
	calculator *disclosureapp.DeadlineCalculator
	now        func() time.Time
}

func NewService(repo Repository, auth authapp.Service, calculator *disclosureapp.DeadlineCalculator) Service {
	return &service{
		repo:       repo,
		auth:       auth,
		calculator: calculator,
		now:        time.Now,
	}
}
