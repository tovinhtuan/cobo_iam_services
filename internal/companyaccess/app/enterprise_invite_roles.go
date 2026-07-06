package app

import "strings"

// EnterpriseInviteRoleDenylist excludes roles that must not appear on enterprise invite/create pickers.
var EnterpriseInviteRoleDenylist = map[string]struct{}{
	"dept_lead":              {},
	"admin_web":              {},
	"cms_operator":           {},
	"full_access":            {},
	"truong_phong_ban":       {},
	"truong_nhom":            {},
	"self_reg_company_owner": {},
}

func IsEnterpriseInviteRoleDenied(roleCode string) bool {
	_, denied := EnterpriseInviteRoleDenylist[strings.TrimSpace(strings.ToLower(roleCode))]
	return denied
}

func FilterEnterpriseInviteRoles(items []InviteRoleOption) []InviteRoleOption {
	if len(items) == 0 {
		return items
	}
	out := make([]InviteRoleOption, 0, len(items))
	for _, item := range items {
		if IsEnterpriseInviteRoleDenied(item.RoleCode) {
			continue
		}
		out = append(out, item)
	}
	return out
}

func isEnterpriseInvitePermission(code string) bool {
	code = strings.TrimSpace(code)
	if code == "" {
		return false
	}
	if strings.HasPrefix(code, "cms.") || strings.HasPrefix(code, "platform.") {
		return false
	}
	return IsEnterprisePermission(code, "")
}
