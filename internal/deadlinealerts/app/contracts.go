package app

import (
	"context"
	"time"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

type Subject struct {
	UserID       string
	MembershipID string
	CompanyID    string
}

type ListDeadlineAlertsRequest struct {
	Subject   Subject
	Status    string // UPCOMING|DUE_SOON|OVERDUE|DONE or FE labels
	Query     string
	StartDate string // YYYY-MM-DD
	EndDate   string // YYYY-MM-DD
	Page      int
	PageSize  int
}

type DeadlineAlertDTO struct {
	AlertID            string   `json:"alert_id"`
	RecordID           string   `json:"record_id"`
	WorkflowInstanceID string   `json:"workflow_instance_id,omitempty"`
	TypeID             string   `json:"type_id"`
	Title              string   `json:"title"`
	DueDate            string   `json:"due_date"`
	Status             string   `json:"status"` // UPCOMING|DUE_SOON|OVERDUE|DONE
	ActiveDepartments  []string `json:"active_departments"`
	Source             string   `json:"source"`
	TemplateCategory   string   `json:"template_category,omitempty"`
}

type ListDeadlineAlertsResponse struct {
	Items    []DeadlineAlertDTO `json:"items"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
	Total    int                `json:"total"`
}

// AlertRow is a DB read model before due-date enrichment.
type AlertRow struct {
	CompanyID          string
	RecordID           string
	TypeID             string
	Title              string
	RecordStatus       string
	PlannedDate        string
	WorkflowInstanceID string
	CurrentStepCode    string
	SnapshotJSON       []byte
	AdHocDeadlineDate  string
	TemplateCategory   string
	DeadlineConfigJSON []byte
}

type Repository interface {
	ListRows(ctx context.Context, companyID string) ([]AlertRow, error)
	GetCompanyDeadlineContext(ctx context.Context, companyID string) (disclosureapp.CompanyDeadlineContext, error)
	GetTypeDeadlineConfig(ctx context.Context, companyID, typeID string) (*disclosureapp.TemplateDeadlineConfig, error)
}

type Service interface {
	ListDeadlineAlerts(ctx context.Context, req ListDeadlineAlertsRequest) (*ListDeadlineAlertsResponse, error)
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
