package dependency

import "time"

const Source = "dependency_viewer_v1"

const (
	ObjectTypeDepartment = "department"
	ObjectTypeRole       = "role"
)

const (
	DefaultSampleLimit = 5
	MaxSampleLimit     = 20
	MaxOverrideScan    = 20
)

const (
	RelationAssignedMember         = "assigned_member"
	RelationAssignedMembershipRole = "assigned_membership_role"
	RelationDirectPermission       = "direct_permission"
	RelationAssigneeRuleReference  = "assignee_rule_reference"
	RelationWorkflowStepDepartment = "workflow_step_department"
	RelationNotificationRecipient  = "notification_recipient_policy"
)

// Sample is a single dependency sample row for the API.
type Sample map[string]any

// Group is one dependency relation bucket (ADR-043).
type Group struct {
	Domain   string   `json:"domain"`
	Relation string   `json:"relation"`
	Count    int      `json:"count"`
	Samples  []Sample `json:"samples,omitempty"`
}

// Result is returned by GET /api/v1/admin/objects/{type}/{id}/dependencies.
type Result struct {
	ObjectType      string    `json:"object_type"`
	ObjectID        string    `json:"object_id"`
	CompanyID       string    `json:"company_id"`
	Dependencies    []Group   `json:"dependencies"`
	TotalReferences int       `json:"total_references"`
	Truncated       bool      `json:"truncated"`
	Source          string    `json:"source"`
	EvaluatedAt     time.Time `json:"evaluated_at"`
}

// Query is the provider input envelope.
type Query struct {
	CompanyID     string
	ObjectType    string
	ObjectID      string
	SampleLimit   int
	IncludeCounts bool
	EvaluatedAt   time.Time
}
