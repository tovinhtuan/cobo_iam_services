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

	ChangeTypeNotificationPatch    = "notification_rule.patch"
	ChangeTypeRBACPermissionRemove = "rbac.permission.remove"
	ChangeTypeRBACDirectPermRemove = "rbac.direct_permission.remove"
	// ChangeTypeRBACMatrixRollback is a rollback of the RBAC matrix that touches a critical
	// permission and therefore waits for a second person to approve it.
	ChangeTypeRBACMatrixRollback = "rbac.matrix.rollback"
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
	// DirectRevokes are explicit instructions carried by an approval to remove direct grants.
	// A plain snapshot (version history, rollback) never has them.
	DirectRevokes []DirectPermissionEntry `json:"direct_revokes,omitempty"`
	// Explicit marks a single-change proposal (remove one role permission, remove one direct
	// grant): applying it performs exactly RoleRevokes and DirectRevokes and nothing else, instead
	// of converging the whole matrix to this snapshot.
	Explicit    bool                  `json:"explicit,omitempty"`
	RoleRevokes []RolePermissionEntry `json:"role_revokes,omitempty"`
	// PlanDigest fingerprints the plan the approver reviewed when the request was queued. Apply
	// refuses (STALE_PROPOSAL) when the plan computed from the live state differs.
	PlanDigest string `json:"plan_digest,omitempty"`
}

type RolePermissionEntry struct {
	RoleID       string `json:"role_id"`
	PermissionID string `json:"permission_id"`
}

type DirectPermissionEntry struct {
	MembershipID   string `json:"membership_id"`
	PermissionCode string `json:"permission_code"`
}
