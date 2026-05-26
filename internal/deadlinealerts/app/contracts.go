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
	Status    string // UPCOMING|DUE_SOON|OVERDUE|PENDING_CONFIRM|DONE or FE labels
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
	Status             string   `json:"status"` // UPCOMING|DUE_SOON|OVERDUE|PENDING_CONFIRM|DONE
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
	PlannedDate        string
	WorkflowInstanceID string
	CurrentStepCode    string
	SnapshotJSON       []byte
	AdHocDeadlineDate  string
	TemplateCategory   string
	DeadlineConfigJSON []byte
	ConfirmedBy        string
	ConfirmedAt        *time.Time
}

type Repository interface {
	ListRows(ctx context.Context, companyID string) ([]AlertRow, error)
	GetCompanyDeadlineContext(ctx context.Context, companyID string) (disclosureapp.CompanyDeadlineContext, error)
	GetTypeDeadlineConfig(ctx context.Context, companyID, typeID string) (*disclosureapp.TemplateDeadlineConfig, error)
	HasDisclosureRecord(ctx context.Context, companyID, recordID string) (bool, error)
	ConfirmDeadlineAlert(ctx context.Context, companyID, recordID, confirmedBy, note, idempotencyKey string, at time.Time) error
}

type Service interface {
	ListDeadlineAlerts(ctx context.Context, req ListDeadlineAlertsRequest) (*ListDeadlineAlertsResponse, error)
	ConfirmDeadlineAlert(ctx context.Context, req ConfirmDeadlineAlertRequest) (*ConfirmDeadlineAlertResponse, error)
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
