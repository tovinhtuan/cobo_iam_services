package app

import (
	"context"
	"time"
)

type Service interface {
	UpsertDisclosureReminderConfig(ctx context.Context, req UpsertReminderConfigRequest) (*ReminderConfigDTO, error)
	UpsertWorkflowStepReminderConfig(ctx context.Context, req UpsertReminderConfigRequest) (*ReminderConfigDTO, error)
	GetDisclosureReminderConfig(ctx context.Context, req GetReminderConfigRequest) (*ReminderConfigDTO, error)
	GetWorkflowStepReminderConfig(ctx context.Context, req GetReminderConfigRequest) (*ReminderConfigDTO, error)
	GetReminderHistory(ctx context.Context, req GetReminderHistoryRequest) (*ReminderHistoryPage, error)
	DispatchOccurrence(ctx context.Context, req DispatchOccurrenceRequest) (*DispatchOccurrenceResponse, error)
	SeedOccurrence(ctx context.Context, req SeedOccurrenceRequest) (*ReminderOccurrenceDTO, error)
	MaterializeDueOccurrences(ctx context.Context, now time.Time) (int, error)
	DispatchDueOccurrences(ctx context.Context, now time.Time, limit int) (*DispatchDueResult, error)
	// SeedOccurrencesFromDueMilestones scans workflow_step_milestones for rows whose
	// scheduled_date <= DATE(now) and reminder_sent=0, seeds reminder_occurrences, and
	// marks each milestone sent. Returns count of occurrences seeded.
	SeedOccurrencesFromDueMilestones(ctx context.Context, now time.Time) (int, error)
}

// DueMilestone is a row returned by MilestoneScanner.
type DueMilestone struct {
	MilestoneID         string
	CompanyID           string
	WorkflowInstanceID  string
	StepID              string
	MilestoneType       string
	ScheduledDate       time.Time
}

// MilestoneScanner reads due milestone rows from workflow_step_milestones.
type MilestoneScanner interface {
	ListDueMilestones(ctx context.Context, asOf time.Time, limit int) ([]DueMilestone, error)
	MarkMilestoneSent(ctx context.Context, milestoneID, occurrenceID string) error
}

type ConfigRepository interface {
	UpsertByScope(ctx context.Context, in ReminderConfigDTO) (*ReminderConfigDTO, error)
	GetByScope(ctx context.Context, scopeType ScopeType, scopeID string) (*ReminderConfigDTO, error)
}

type OccurrenceRepository interface {
	ListHistoryByDisclosure(ctx context.Context, disclosureID string, q HistoryQuery) (*ReminderHistoryPage, error)
	ClaimForDispatch(ctx context.Context, occurrenceID string) (*ReminderOccurrenceDTO, error)
	UpdateDispatchResult(ctx context.Context, in DispatchResultInput) error
	SeedOccurrence(ctx context.Context, in ReminderOccurrenceDTO) (*ReminderOccurrenceDTO, error)
	MaterializeDueOccurrences(ctx context.Context, now time.Time) (int, error)
	ListDispatchCandidates(ctx context.Context, now time.Time, limit int) ([]DispatchCandidate, error)
}

type AttemptRepository interface {
	InsertAttempt(ctx context.Context, in ReminderDeliveryAttemptDTO) error
}

type Subject struct {
	UserID       string
	MembershipID string
	CompanyID    string
}

type ScopeType string

const (
	ScopeTypeDisclosure   ScopeType = "DISCLOSURE"
	ScopeTypeWorkflowStep ScopeType = "WORKFLOW_STEP"
)

type ReminderMode string

const (
	ReminderModeDaysBefore   ReminderMode = "DaysBefore"
	ReminderModeSpecificDate ReminderMode = "SpecificDate"
)

type ReminderRecipientType string

const (
	ReminderRecipientTypeDepartments ReminderRecipientType = "Departments"
	ReminderRecipientTypeIndividuals ReminderRecipientType = "Individuals"
	ReminderRecipientTypeBoth        ReminderRecipientType = "Both"
)

type ReminderStatus string

const (
	ReminderStatusPending        ReminderStatus = "PENDING"
	ReminderStatusDispatching    ReminderStatus = "DISPATCHING"
	ReminderStatusRetryScheduled ReminderStatus = "RETRY_SCHEDULED"
	ReminderStatusSent           ReminderStatus = "SENT"
	ReminderStatusFailed         ReminderStatus = "FAILED"
)

type UpsertReminderConfigRequest struct {
	Subject       Subject
	DisclosureID  string
	WorkflowStepID string
	Config        ReminderConfigInput `json:"config"`
}

type GetReminderConfigRequest struct {
	Subject        Subject
	DisclosureID   string
	WorkflowStepID string
}

type GetReminderHistoryRequest struct {
	Subject      Subject
	DisclosureID string
	Scope        string
	Status       ReminderStatus
	From         time.Time
	To           time.Time
	Page         int
	PageSize     int
}

type DispatchOccurrenceRequest struct {
	OccurrenceID  string         `json:"occurrence_id"`
	IdempotencyKey string        `json:"idempotency_key"`
	TemplateCode  string         `json:"template_code"`
	TemplatePayload map[string]any `json:"template_payload"`
	RecipientEmails []string     `json:"recipient_emails"`
}

type DispatchCandidate struct {
	OccurrenceID    string
	IdempotencyKey  string
	TemplateCode    string
	TemplatePayload map[string]any
	RecipientEmails []string
	CurrentAttempt  int
	ScheduledAt     time.Time
	// Fields added for alert dispatch (Phase 4):
	ScopeType        ScopeType // "DISCLOSURE" | "WORKFLOW_STEP"
	ScopeID          string    // disclosure_id for DISCLOSURE; step_id for WORKFLOW_STEP
	CompanyID        string    // company_id from disclosure_records or workflow_instances
	CompanyName      string    // company_name from companies
	DisclosureTypeID string    // type_id from disclosure_types (via disclosure_records)
}

// RecipientResolver resolves the email recipient list for a dispatch candidate.
// Implementations must enforce tenant isolation: only return emails belonging to companyID.
type RecipientResolver interface {
	ResolveForDeadline(ctx context.Context, companyID, scopeID string) ([]string, error)
	ResolveForWorkflowStep(ctx context.Context, companyID, stepID string) ([]string, error)
}

// MembershipEmailQuerier performs low-level membership → email lookups.
// All methods must filter by companyID to enforce tenant isolation.
type MembershipEmailQuerier interface {
	EmailsByDepartments(ctx context.Context, companyID string, departmentIDs []string) ([]string, error)
	EmailsByRoles(ctx context.Context, companyID string, roleIDs []string, departmentID string) ([]string, error)
}

// WorkflowStepReader looks up global workflow step config by step_id.
type WorkflowStepReader interface {
	GetStepByID(ctx context.Context, stepID string) (*WorkflowStepConfig, error)
}

type DispatchOccurrenceResponse struct {
	Accepted bool           `json:"accepted"`
	Status   ReminderStatus `json:"status"`
	Message  string         `json:"message,omitempty"`
}

type DispatchDueResult struct {
	Processed int `json:"processed"`
	Sent      int `json:"sent"`
	Retried   int `json:"retried"`
	Failed    int `json:"failed"`
}

type SeedOccurrenceRequest struct {
	DisclosureID   string         `json:"disclosure_id"`
	ScopeType      ScopeType      `json:"scope_type"`
	ScopeID        string         `json:"scope_id"`
	ScheduledAt    time.Time      `json:"scheduled_at"`
	Status         ReminderStatus `json:"status"`
	IdempotencyKey string         `json:"idempotency_key"`
}

type ReminderConfigInput struct {
	Enabled       bool                  `json:"enabled"`
	Mode          ReminderMode          `json:"mode"`
	DaysBefore    []int                 `json:"daysBefore,omitempty"`
	SpecificDates []string              `json:"specificDates,omitempty"`
	RecipientType ReminderRecipientType `json:"recipientType"`
	Departments   []string              `json:"departments,omitempty"`
	Recipients    []string              `json:"recipients,omitempty"`
}

type ReminderConfigDTO struct {
	ScopeType ScopeType           `json:"scope_type"`
	ScopeID   string              `json:"scope_id"`
	Config    ReminderConfigInput `json:"config"`
	Version   int                 `json:"version"`
	UpdatedAt time.Time           `json:"updated_at"`
	UpdatedBy string              `json:"updated_by"`
}

type ReminderOccurrenceDTO struct {
	OccurrenceID      string         `json:"occurrence_id"`
	DisclosureID      string         `json:"disclosure_id"`
	ScopeType         ScopeType      `json:"scope_type"`
	ScopeID           string         `json:"scope_id"`
	ScheduledAt       time.Time      `json:"scheduled_at"`
	Status            ReminderStatus `json:"status"`
	AttemptCount      int            `json:"attempt_count"`
	IdempotencyKey    string         `json:"idempotency_key"`
	LastAttemptAt     *time.Time     `json:"last_attempt_at,omitempty"`
	LastErrorCode     string         `json:"last_error_code,omitempty"`
	ProviderMessageID string         `json:"provider_message_id,omitempty"`
}

type ReminderDeliveryAttemptDTO struct {
	OccurrenceID      string
	AttemptNo         int
	Status            ReminderStatus
	ProviderMessageID string
	ErrorCode         string
	ErrorMessage      string
	CreatedAt         time.Time
}

type ReminderHistoryPage struct {
	Items    []ReminderOccurrenceDTO `json:"items"`
	Page     int                     `json:"page"`
	PageSize int                     `json:"page_size"`
	Total    int                     `json:"total"`
}

type HistoryQuery struct {
	Scope    string
	Status   ReminderStatus
	From     *time.Time
	To       *time.Time
	Page     int
	PageSize int
}

type DispatchResultInput struct {
	OccurrenceID      string
	Status            ReminderStatus
	NextRetryAt       *time.Time
	LastErrorCode     string
	LastErrorMessage  string
	ProviderMessageID string
	IncrementAttempt  bool
}

// AlertTemplateConfig maps a (disclosure type, alert kind) pair to an email template key.
// alert_kind values: "deadline" | "workflow_step"
type AlertTemplateConfig struct {
	ID          int64
	TypeID      string
	AlertKind   string
	TemplateKey string
	Enabled     bool
	CreatedBy   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// AlertKind constants for AlertTemplateConfig.AlertKind.
const (
	AlertKindDeadline     = "deadline"
	AlertKindWorkflowStep = "workflow_step"
)

// AlertConfigRepository persists alert_template_configs rows.
type AlertConfigRepository interface {
	GetByTypeID(ctx context.Context, typeID string) ([]AlertTemplateConfig, error)
	GetByTypeAndKind(ctx context.Context, typeID, alertKind string) (*AlertTemplateConfig, error)
	Upsert(ctx context.Context, in AlertTemplateConfig) error
}
