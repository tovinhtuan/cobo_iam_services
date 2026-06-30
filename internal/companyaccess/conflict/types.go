package conflict

import "time"

// Severity values for conflict results (report-only in Sprint 4).
const (
	SeverityBlocking = "blocking"
	SeverityWarning  = "warning"
	SeverityInfo     = "info"
)

const DomainConflict = "conflict"

// Result is a single conflict finding aligned with configuration-health checks[].
type Result struct {
	Code         string
	Severity     string
	Domain       string
	CompanyID    string
	Title        string
	Description  string
	ActionLink   string
	Evidence     map[string]any
	ResourceType string
	ResourceID   string
}

// EvaluationInput is the engine entry envelope.
type EvaluationInput struct {
	CompanyID   string
	EvaluatedAt time.Time
}

// EvaluationOutput is the aggregate engine output.
type EvaluationOutput struct {
	CompanyID     string
	EvaluatedAt   time.Time
	Results       []Result
	RulesExecuted int
	RulesSkipped  int
	Partial       bool
}

// ConfigurationSnapshot is read-only persisted state for one company.
type ConfigurationSnapshot struct {
	CompanyID                    string
	EvaluatedAt                  time.Time
	AlertChannelPrefsRuleID      string
	AlertChannelPrefsPayload     map[string]any
	AlertChannelPrefsExists      bool
	Roles                        []RoleSnapshot
	RolePermissionCodes          map[string][]string
	StaleWorkflowOverrides       []StaleWorkflowOverrideRow
	WorkflowAssigneeRules        []WorkflowAssigneeRuleRow
	InactiveDepartmentsReferenced []InactiveDepartmentRow
	NonGrantableDirectPermissions []DirectPermissionRow
	CompanySubscriptionTier      string
	SubscriptionTierEnforced     bool
}

type RoleSnapshot struct {
	RoleID      string
	RoleCode    string
	RoleName    string
	MemberCount int
	Status      string
}

type StaleWorkflowOverrideRow struct {
	TypeID            string
	StaleStatus       string
	ActiveVersionNo   int
	LastRebaseCheckAt *time.Time
}

type WorkflowAssigneeRuleRow struct {
	RuleID   string
	RuleCode string
	Payload  map[string]any
}

type InactiveDepartmentRow struct {
	DepartmentID   string
	DepartmentName string
	MemberCount    int
}

type DirectPermissionRow struct {
	MembershipID   string
	PermissionCode string
}
