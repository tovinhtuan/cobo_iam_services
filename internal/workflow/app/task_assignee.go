package app

import "strings"

const (
	ActorTypeSystem           = "SYSTEM"
	SystemAssigneeDisplayName = "Hệ thống"
)

// BuildTaskAssignee maps optional user/department enrichment onto a task assignee.
// System memberships (m_system_*) that do not resolve to a user are fail-soft:
// actor_type=SYSTEM and a stable display name — never panic / never omit the assignee.
func BuildTaskAssignee(membershipID, displayName, email, departmentName string) *TaskAssigneeDTO {
	membershipID = strings.TrimSpace(membershipID)
	if membershipID == "" {
		return nil
	}
	assignee := &TaskAssigneeDTO{MembershipID: membershipID}
	if v := strings.TrimSpace(displayName); v != "" {
		assignee.DisplayName = v
	}
	if v := strings.TrimSpace(email); v != "" {
		assignee.Email = v
	}
	if v := strings.TrimSpace(departmentName); v != "" {
		assignee.DepartmentName = v
	}
	if isSystemMembershipID(membershipID) && assignee.DisplayName == "" {
		assignee.ActorType = ActorTypeSystem
		assignee.DisplayName = SystemAssigneeDisplayName
	}
	return assignee
}

func isSystemMembershipID(membershipID string) bool {
	return strings.HasPrefix(strings.ToLower(membershipID), "m_system_")
}
