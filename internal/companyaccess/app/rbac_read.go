package app

import "time"

// RoleListItem is the structured row for GET /api/v1/admin/roles.
type RoleListItem struct {
	RoleID          string    `json:"role_id"`
	RoleCode        string    `json:"role_code"`
	RoleName        string    `json:"role_name"`
	Description     string    `json:"description"`
	Status          string    `json:"status"`
	Scope           string    `json:"scope"`
	IsBuiltin       bool      `json:"is_builtin"`
	PermissionCount int       `json:"permission_count"`
	MemberCount     int       `json:"member_count"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// PermissionListItem is the structured row for GET /api/v1/admin/permissions.
type PermissionListItem struct {
	PermissionID   string `json:"permission_id"`
	PermissionCode string `json:"permission_code"`
	PermissionName string `json:"permission_name"`
	ModuleName     string `json:"module_name"`
	Description    string `json:"description"`
	RiskLevel      string `json:"risk_level"`
	IsGrantable    bool   `json:"is_grantable"`
}

// RolePermissionsView is returned by GET /api/v1/admin/roles/{role_id}/permissions.
type RolePermissionsView struct {
	RoleID      string               `json:"role_id"`
	Permissions []PermissionListItem `json:"permissions"`
}

type ListRolePermissionsRequest struct {
	Subject AdminSubject
	RoleID  string
}

// criticalPermissionCodes is used for risk_level classification in admin read APIs.
var criticalPermissionCodes = map[string]struct{}{
	"rbac.manage":                  {},
	"system.settings":              {},
	"admin.membership.invite":        {},
	"disclosure.publish":             {},
	"disclosure.auto_create.manage":  {},
	"company.profile.manage":         {},
}

// PermissionRiskLevel returns low|medium|high|critical for a permission code.
func PermissionRiskLevel(code string) string {
	if _, ok := criticalPermissionCodes[code]; ok {
		return "critical"
	}
	if len(code) >= 7 && code[len(code)-7:] == ".manage" {
		return "high"
	}
	if len(code) >= 7 && (code[len(code)-7:] == ".create" || code[len(code)-5:] == ".edit") {
		return "medium"
	}
	return "low"
}

// IsGrantablePermission reports whether enterprise admin may grant the permission directly.
func IsGrantablePermission(code string) bool {
	for _, g := range GrantablePermissions {
		if g == code {
			return true
		}
	}
	return false
}
