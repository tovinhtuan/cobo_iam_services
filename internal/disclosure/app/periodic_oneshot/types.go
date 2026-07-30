package periodic_oneshot

import (
	"context"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
)

// TypeSnapshot is the active template state needed for guards + calculator.
type TypeSnapshot struct {
	TypeID             string
	TypeName           string
	Status             string
	ActiveVersionNo    int
	DeadlineMode       string
	DeadlineDays       int
	FrequencyUnit      string
	DeadlineDayType    string // working|calendar from applicability
	IsGlobal           bool
	ApplicabilityRules *applicability.TemplateApplicabilityRules
	HasWorkflow        bool
}

// CycleSnapshot is existing periodic_cycles row (if any).
type CycleSnapshot struct {
	Exists     bool
	CycleID    string
	CycleLabel string
	CycleStart string // YYYY-MM-DD
	DueDate    string
	RecordID   string
}

// RecordSnapshot is existing disclosure record linked or found for scope.
type RecordSnapshot struct {
	Exists   bool
	RecordID string
	Status   string
}

// Domain is the production-path surface used by the one-shot engine.
type Domain interface {
	LoadType(ctx context.Context, typeID, companyID string) (TypeSnapshot, error)
	LoadCompanyProfile(ctx context.Context, companyID string) (applicability.CompanyApplicabilityProfile, error)
	LoadCycle(ctx context.Context, typeID, companyID, cycleLabel string) (CycleSnapshot, error)
	InsertCycle(ctx context.Context, row disclosureapp.PeriodicCycleRow) error
	DeleteUnmaterializedCycle(ctx context.Context, cycleID string) error
	ClaimCycle(ctx context.Context, cycleID string) (bool, error)
	ReleaseCycle(ctx context.Context, cycleID string) error
	UpdateCycleRecord(ctx context.Context, cycleID, recordID string) error
	CreateAndSubmitRecordWithPlannedDate(ctx context.Context, companyID, typeID, createdByMembershipID, title string, t0Date *time.Time, plannedDate string) (recordID, workflowInstanceID string, err error)
	ComputeDue(ctx context.Context, cycleStart time.Time, deadlineDays int, durationType string) (time.Time, error)
	NewCycleID() string
	Location() *time.Location
}

// Report is preview/apply JSON output.
type Report struct {
	Mode                 string         `json:"mode"`
	Status               string         `json:"status,omitempty"`
	Environment          string         `json:"environment"`
	Scope                Scope          `json:"scope"`
	Resolved             map[string]any `json:"resolved"`
	Existing             map[string]any `json:"existing"`
	PlannedActions       []string       `json:"planned_actions"`
	SnapshotChecksum     string         `json:"snapshot_checksum"`
	ConfirmToken         string         `json:"confirm_token,omitempty"`
	Mutations            int            `json:"mutations"`
	CycleCreated         bool           `json:"cycle_created,omitempty"`
	RecordCreated        bool           `json:"record_created,omitempty"`
	TransactionCommitted bool           `json:"transaction_committed,omitempty"`
	CycleID              string         `json:"cycle_id,omitempty"`
	RecordID             string         `json:"record_id,omitempty"`
	Error                string         `json:"error,omitempty"`
}

// Engine orchestrates guarded preview/apply.
type Engine struct {
	Env    EnvGuard
	Domain Domain
}
