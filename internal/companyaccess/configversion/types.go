package configversion

const (
	NotificationSnapshotSchema = "notification_rule_snapshot.v1"
	RBACMatrixSnapshotSchema   = "rbac_matrix_snapshot.v1"

	AggregateNotificationRule = "notification_rule"
	AggregateRBACMatrix       = "rbac_matrix"

	SourceMutationAPI   = "mutation_api"
	SourceRollback      = "rollback"
	SourceApprovalApply = "approval_apply"
)

const (
	ApprovalSubjectConfigSnapshot = "config_snapshot"

	ApprovalStatusPending   = "pending"
	ApprovalStatusApproved  = "approved"
	ApprovalStatusRejected  = "rejected"
	ApprovalStatusCancelled = "cancelled"

	ChangeTypeNotificationPatch       = "notification_rule.patch"
	ChangeTypeRBACPermissionRemove    = "rbac.permission.remove"
	ChangeTypeRBACDirectPermRemove    = "rbac.direct_permission.remove"
)

// NotificationRuleSnapshot is the immutable post-mutation state for one notification rule.
type NotificationRuleSnapshot struct {
	SchemaVersion      string         `json:"schema_version"`
	NotificationRuleID string         `json:"notification_rule_id"`
	RuleCode           string         `json:"rule_code"`
	Status             string         `json:"status"`
	Payload            map[string]any `json:"payload"`
}

// RBACMatrixSnapshot is the immutable company RBAC matrix (roles + direct grants).
type RBACMatrixSnapshot struct {
	SchemaVersion     string                  `json:"schema_version"`
	RolePermissions   []RolePermissionEntry   `json:"role_permissions"`
	DirectPermissions []DirectPermissionEntry `json:"direct_permissions"`
}

type RolePermissionEntry struct {
	RoleID       string `json:"role_id"`
	PermissionID string `json:"permission_id"`
}

type DirectPermissionEntry struct {
	MembershipID   string `json:"membership_id"`
	PermissionCode string `json:"permission_code"`
}
