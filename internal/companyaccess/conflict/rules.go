package conflict

const (
	codeOverrideStale              = "conflict.workflow.override_stale"
	codeNotificationPrefsInvalid   = "conflict.notification.prefs_invalid"
	codeCriticalRoleEmpty          = "conflict.permission.critical_role_empty"
	codeGrantableViolation         = "conflict.permission.grantable_violation"
	codeInactiveDepartmentRef      = "conflict.org.department_inactive_referenced"
	codeAssigneeRoleMissing        = "conflict.workflow.assignee_role_missing"
	codeRoleUnassignedInWorkflow   = "conflict.rbac.role_unassigned_in_workflow"
	codeTierPrefsMismatch          = "conflict.subscription.tier_prefs_mismatch"

	actionLinkNotifications = "/app/admin?tab=notifications"
	actionLinkRBAC          = "/app/admin?tab=rbac"
	actionLinkOrg           = "/app/admin?tab=org"
	actionLinkWorkflowStale = "/app/disclosure-types"
)

type OverrideStaleDetector struct{}

func (d *OverrideStaleDetector) Code() string { return codeOverrideStale }

func (d *OverrideStaleDetector) Detect(snapshot *ConfigurationSnapshot) []Result {
	if snapshot == nil {
		return nil
	}
	var out []Result
	for _, row := range snapshot.StaleWorkflowOverrides {
		if row.StaleStatus != "stale" {
			continue
		}
		evidence := map[string]any{
			"type_id":           row.TypeID,
			"stale_status":      row.StaleStatus,
			"active_version_no": row.ActiveVersionNo,
		}
		if row.LastRebaseCheckAt != nil {
			evidence["last_rebase_check_at"] = row.LastRebaseCheckAt.UTC().Format("2006-01-02T15:04:05Z")
		}
		out = append(out, Result{
			Code:        codeOverrideStale,
			Severity:    SeverityWarning,
			Domain:      DomainConflict,
			CompanyID:   snapshot.CompanyID,
			Title:       "Workflow override đang lỗi thời",
			Description: "Có override workflow công ty đang ở trạng thái stale — cần rebase trên portal disclosure.",
			ActionLink:  actionLinkWorkflowStale,
			Evidence:    evidence,
			ResourceType: "disclosure_type",
			ResourceID:   row.TypeID,
		})
	}
	return out
}

type NotificationPrefsInvalidDetector struct{}

func (d *NotificationPrefsInvalidDetector) Code() string { return codeNotificationPrefsInvalid }

func (d *NotificationPrefsInvalidDetector) Detect(snapshot *ConfigurationSnapshot) []Result {
	if snapshot == nil || !snapshot.AlertChannelPrefsExists {
		return nil
	}
	valid, issues := validators.ValidatePrefs(snapshot.AlertChannelPrefsPayload)
	if valid {
		return nil
	}
	return []Result{{
		Code:        codeNotificationPrefsInvalid,
		Severity:    SeverityBlocking,
		Domain:      DomainConflict,
		CompanyID:   snapshot.CompanyID,
		Title:       "Cấu hình kênh cảnh báo không hợp lệ",
		Description: "Payload notification_rules cho alert channel prefs không pass validation.",
		ActionLink:  actionLinkNotifications,
		Evidence: map[string]any{
			"rule_id": snapshot.AlertChannelPrefsRuleID,
			"issues":  issues,
		},
		ResourceType: "notification_rule",
		ResourceID:   snapshot.AlertChannelPrefsRuleID,
	}}
}

type CriticalRoleEmptyDetector struct{}

func (d *CriticalRoleEmptyDetector) Code() string { return codeCriticalRoleEmpty }

func (d *CriticalRoleEmptyDetector) Detect(snapshot *ConfigurationSnapshot) []Result {
	if snapshot == nil {
		return nil
	}
	var out []Result
	for _, role := range snapshot.Roles {
		if role.Status != "" && role.Status != "active" {
			continue
		}
		if role.MemberCount > 0 {
			continue
		}
		perms := snapshot.RolePermissionCodes[role.RoleID]
		hasCritical := false
		for _, p := range perms {
			if validators.PermissionRiskLevel(p) == "critical" {
				hasCritical = true
				break
			}
		}
		if !hasCritical {
			continue
		}
		out = append(out, Result{
			Code:        codeCriticalRoleEmpty,
			Severity:    SeverityWarning,
			Domain:      DomainConflict,
			CompanyID:   snapshot.CompanyID,
			Title:       "Vai trò quan trọng chưa có thành viên",
			Description: "Vai trò có quyền critical không có membership nào được gán.",
			ActionLink:  actionLinkRBAC,
			Evidence: map[string]any{
				"role_id":   role.RoleID,
				"role_code": role.RoleCode,
				"member_count": role.MemberCount,
			},
			ResourceType: "role",
			ResourceID:   role.RoleID,
		})
	}
	return out
}

type GrantableViolationDetector struct{}

func (d *GrantableViolationDetector) Code() string { return codeGrantableViolation }

func (d *GrantableViolationDetector) Detect(snapshot *ConfigurationSnapshot) []Result {
	if snapshot == nil {
		return nil
	}
	var out []Result
	for _, row := range snapshot.NonGrantableDirectPermissions {
		if validators.IsGrantablePermission(row.PermissionCode) {
			continue
		}
		out = append(out, Result{
			Code:        codeGrantableViolation,
			Severity:    SeverityWarning,
			Domain:      DomainConflict,
			CompanyID:   snapshot.CompanyID,
			Title:       "Direct permission không grantable",
			Description: "Có direct grant cho permission ngoài danh sách grantable — dữ liệu drift.",
			ActionLink:  actionLinkRBAC,
			Evidence: map[string]any{
				"membership_id":   row.MembershipID,
				"permission_code": row.PermissionCode,
			},
			ResourceType: "membership_direct_permission",
			ResourceID:   row.MembershipID + ":" + row.PermissionCode,
		})
	}
	return out
}

type InactiveDepartmentReferencedDetector struct{}

func (d *InactiveDepartmentReferencedDetector) Code() string { return codeInactiveDepartmentRef }

func (d *InactiveDepartmentReferencedDetector) Detect(snapshot *ConfigurationSnapshot) []Result {
	if snapshot == nil {
		return nil
	}
	var out []Result
	for _, row := range snapshot.InactiveDepartmentsReferenced {
		if row.MemberCount <= 0 {
			continue
		}
		out = append(out, Result{
			Code:        codeInactiveDepartmentRef,
			Severity:    SeverityWarning,
			Domain:      DomainConflict,
			CompanyID:   snapshot.CompanyID,
			Title:       "Phòng ban inactive vẫn có thành viên",
			Description: "Phòng ban đã inactive nhưng vẫn còn membership active gắn vào.",
			ActionLink:  actionLinkOrg,
			Evidence: map[string]any{
				"department_id":   row.DepartmentID,
				"department_name": row.DepartmentName,
				"member_count":    row.MemberCount,
			},
			ResourceType: "department",
			ResourceID:   row.DepartmentID,
		})
	}
	return out
}

type AssigneeRoleMissingDetector struct{}

func (d *AssigneeRoleMissingDetector) Code() string { return codeAssigneeRoleMissing }

func (d *AssigneeRoleMissingDetector) Detect(snapshot *ConfigurationSnapshot) []Result {
	if snapshot == nil {
		return nil
	}
	roleIDs := roleIDSet(snapshot.Roles)
	var out []Result
	for _, rule := range snapshot.WorkflowAssigneeRules {
		for _, roleID := range extractRoleIDsFromPayload(rule.Payload) {
			if roleID == "" {
				continue
			}
			if _, ok := roleIDs[roleID]; ok {
				continue
			}
			out = append(out, Result{
				Code:        codeAssigneeRoleMissing,
				Severity:    SeverityWarning,
				Domain:      DomainConflict,
				CompanyID:   snapshot.CompanyID,
				Title:       "Workflow assignee tham chiếu role không tồn tại",
				Description: "Rule workflow assignee tham chiếu role_id không thuộc công ty.",
				ActionLink:  actionLinkRBAC,
				Evidence: map[string]any{
					"workflow_assignee_rule_id": rule.RuleID,
					"rule_code":                 rule.RuleCode,
					"role_id":                   roleID,
				},
				ResourceType: "workflow_assignee_rule",
				ResourceID:   rule.RuleID,
			})
		}
	}
	return out
}

type RoleUnassignedInWorkflowDetector struct{}

func (d *RoleUnassignedInWorkflowDetector) Code() string { return codeRoleUnassignedInWorkflow }

func (d *RoleUnassignedInWorkflowDetector) Detect(snapshot *ConfigurationSnapshot) []Result {
	if snapshot == nil {
		return nil
	}
	rolesByID := make(map[string]RoleSnapshot, len(snapshot.Roles))
	for _, r := range snapshot.Roles {
		rolesByID[r.RoleID] = r
	}
	var out []Result
	for _, rule := range snapshot.WorkflowAssigneeRules {
		for _, roleID := range extractRoleIDsFromPayload(rule.Payload) {
			if roleID == "" {
				continue
			}
			role, ok := rolesByID[roleID]
			if !ok {
				continue
			}
			if role.MemberCount > 0 {
				continue
			}
			out = append(out, Result{
				Code:        codeRoleUnassignedInWorkflow,
				Severity:    SeverityWarning,
				Domain:      DomainConflict,
				CompanyID:   snapshot.CompanyID,
				Title:       "Role trong workflow chưa có thành viên",
				Description: "Workflow assignee rule tham chiếu role tồn tại nhưng chưa có membership.",
				ActionLink:  actionLinkRBAC,
				Evidence: map[string]any{
					"workflow_assignee_rule_id": rule.RuleID,
					"role_id":                   roleID,
					"member_count":              0,
				},
				ResourceType: "role",
				ResourceID:   roleID,
			})
		}
	}
	return out
}

type TierPrefsMismatchDetector struct{}

func (d *TierPrefsMismatchDetector) Code() string { return codeTierPrefsMismatch }

func (d *TierPrefsMismatchDetector) Detect(snapshot *ConfigurationSnapshot) []Result {
	if snapshot == nil || !snapshot.AlertChannelPrefsExists {
		return nil
	}
	if snapshot.SubscriptionTierEnforced {
		return nil
	}
	if !hasEnabledPremiumSchedule(snapshot.AlertChannelPrefsPayload) {
		return nil
	}
	tier := normalizeTierLabel(snapshot.CompanySubscriptionTier)
	if tier != "" && tier != "free" {
		return nil
	}
	return []Result{{
		Code:        codeTierPrefsMismatch,
		Severity:    SeverityInfo,
		Domain:      DomainConflict,
		CompanyID:   snapshot.CompanyID,
		Title:       "Lịch premium bật khi tier Free",
		Description: "Có schedule premium_only đang bật trong khi tier công ty là Free và enforcement chưa kích hoạt.",
		ActionLink:  actionLinkNotifications,
		Evidence: map[string]any{
			"company_tier":                  tier,
			"subscription_tier_enforced": snapshot.SubscriptionTierEnforced,
			"premium_schedules_enabled":  true,
		},
	}}
}

func roleIDSet(roles []RoleSnapshot) map[string]struct{} {
	out := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		out[r.RoleID] = struct{}{}
	}
	return out
}

func extractRoleIDsFromPayload(payload map[string]any) []string {
	if payload == nil {
		return nil
	}
	var ids []string
	if s, ok := payload["role_id"].(string); ok && s != "" {
		ids = append(ids, s)
	}
	if s, ok := payload["assignee_role_id"].(string); ok && s != "" {
		ids = append(ids, s)
	}
	if arr, ok := payload["assignee_role_ids"].([]any); ok {
		for _, item := range arr {
			if s, ok := item.(string); ok && s != "" {
				ids = append(ids, s)
			}
		}
	}
	return ids
}

func hasEnabledPremiumSchedule(payload map[string]any) bool {
	schedules, ok := payload["schedules"].([]any)
	if !ok {
		return false
	}
	for _, item := range schedules {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		enabled, _ := m["enabled"].(bool)
		if !enabled {
			continue
		}
		premium, _ := m["premium_only"].(bool)
		if premium {
			return true
		}
	}
	return false
}

func normalizeTierLabel(tier string) string {
	switch tier {
	case "premium", "Premium", "PREMIUM":
		return "premium"
	case "enterprise", "Enterprise", "ENTERPRISE":
		return "enterprise"
	default:
		return "free"
	}
}
