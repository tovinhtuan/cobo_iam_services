package app

import (
	"net/http"
	"strings"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// PermissionGrantTier classifies permissions for role-matrix mutation boundary.
type PermissionGrantTier string

const (
	GrantTierSystemOnly            PermissionGrantTier = "system_only"
	GrantTierTenantAdminOnly       PermissionGrantTier = "tenant_admin_only"
	GrantTierGrantable             PermissionGrantTier = "grantable"
	GrantTierHighRisk              PermissionGrantTier = "high_risk"
	GrantTierDeprecatedOrUnknown   PermissionGrantTier = "deprecated_or_unknown"
)

// PermissionGrantPolicy is the static conservative grant policy for a permission code.
type PermissionGrantPolicy struct {
	PermissionCode             string              `json:"permission_code"`
	ModuleName                 string              `json:"module_name"`
	GrantTier                  PermissionGrantTier `json:"grant_tier"`
	AllowedOnCustomRole        bool                `json:"allowed_on_custom_role"`
	AllowedOnTenantDefaultEdit bool                `json:"allowed_on_tenant_default_edit"`
	RiskLevel                  string              `json:"risk_level"`
	Reason                     string              `json:"reason"`
}

// GrantablePermissionItem is returned by GET /api/v1/admin/rbac/grantable-permissions.
type GrantablePermissionItem struct {
	PermissionCode             string              `json:"permission_code"`
	PermissionName             string              `json:"permission_name,omitempty"`
	ModuleName                 string              `json:"module_name,omitempty"`
	GrantTier                  PermissionGrantTier `json:"grant_tier"`
	RiskLevel                  string              `json:"risk_level,omitempty"`
	AllowedOnCustomRole        bool                `json:"allowed_on_custom_role"`
	AllowedOnTenantDefaultEdit bool                `json:"allowed_on_tenant_default_edit"`
	Reason                     string              `json:"reason,omitempty"`
}

var staticGrantPolicies = map[string]PermissionGrantPolicy{
	"platform.cms.view":                  policy("platform.cms.view", "platform", GrantTierSystemOnly, false, "Platform CMS gate"),
	"cms.template.read":                  policy("cms.template.read", "cms", GrantTierSystemOnly, false, "Cross-tenant template"),
	"cms.template.write":                 policy("cms.template.write", "cms", GrantTierSystemOnly, false, "CMS mutation"),
	"cms.template.activate":              policy("cms.template.activate", "cms", GrantTierSystemOnly, false, "CMS lifecycle"),
	"cms.template.archive":               policy("cms.template.archive", "cms", GrantTierSystemOnly, false, "CMS lifecycle"),
	"cms.template.config.write":          policy("cms.template.config.write", "cms", GrantTierSystemOnly, false, "Global template config"),
	"disclosure_type.config.read":        policy("disclosure_type.config.read", "cms", GrantTierSystemOnly, false, "Platform type config"),
	"disclosure_type.config.write":       policy("disclosure_type.config.write", "cms", GrantTierSystemOnly, false, "Platform type config"),
	"rbac.manage":                        policy("rbac.manage", "admin", GrantTierTenantAdminOnly, false, "Privilege escalation"),
	"system.settings":                    policy("system.settings", "admin", GrantTierTenantAdminOnly, false, "Config approval authority"),
	"admin.role.permission.assign":       policy("admin.role.permission.assign", "admin", GrantTierTenantAdminOnly, false, "Meta RBAC mutation"),
	"admin.role.permission.remove":       policy("admin.role.permission.remove", "admin", GrantTierTenantAdminOnly, false, "Meta RBAC mutation"),
	"company.profile.manage":             policy("company.profile.manage", "company", GrantTierTenantAdminOnly, false, "Critical company profile tier"),
	"disclosure.auto_create.manage":      policy("disclosure.auto_create.manage", "disclosure", GrantTierTenantAdminOnly, false, "Auto-create prefs"),
	"ad_hoc_alert.process_control":       policy("ad_hoc_alert.process_control", "ad_hoc", GrantTierTenantAdminOnly, false, "Process controller assign"),
	"admin.membership.invite":            policy("admin.membership.invite", "admin", GrantTierHighRisk, false, "Invite new members"),
	"company.ownership.transfer":         policy("company.ownership.transfer", "org", GrantTierHighRisk, false, "Transfer primary admin"),
	"disclosure.publish":                 policy("disclosure.publish", "disclosure", GrantTierHighRisk, false, "Submit to SSC"),
	"disclosure_type.publish":            policy("disclosure_type.publish", "disclosure", GrantTierHighRisk, false, "Template publish tenant"),
	"workflow.step.override":             policy("workflow.step.override", "workflow", GrantTierHighRisk, false, "Override step"),
	"template.workflow.override.reset":   policy("template.workflow.override.reset", "template", GrantTierHighRisk, false, "Reset overrides"),
	"alert.channels.manage":              policy("alert.channels.manage", "notification", GrantTierHighRisk, false, "Notification channels"),
	"ad_hoc_alert.admin_review":          policy("ad_hoc_alert.admin_review", "ad_hoc", GrantTierDeprecatedOrUnknown, false, "Deprecated permission"),
	"recipient.view":                     policy("recipient.view", "admin", GrantTierGrantable, true, "Read recipients list"),
	"recipient.manage":                   policy("recipient.manage", "admin", GrantTierGrantable, true, "Manage recipients"),
	"company.view":                       policy("company.view", "company", GrantTierGrantable, true, "Read company profile"),
	"company.edit":                       policy("company.edit", "company", GrantTierGrantable, true, "Edit company profile"),
	"dept.manage":                        policy("dept.manage", "org", GrantTierGrantable, true, "Org structure"),
	"disclosure.view":                    policy("disclosure.view", "disclosure", GrantTierGrantable, true, "Read CBTT"),
	"disclosure.create":                  policy("disclosure.create", "disclosure", GrantTierGrantable, true, "Create draft"),
	"disclosure.edit":                    policy("disclosure.edit", "disclosure", GrantTierGrantable, true, "Edit draft"),
	"disclosure.approve":                 policy("disclosure.approve", "disclosure", GrantTierGrantable, true, "Approve workflow"),
	"disclosure_type.manage":             policy("disclosure_type.manage", "disclosure", GrantTierGrantable, true, "Tenant CBTT types"),
	"workflow.read":                      policy("workflow.read", "workflow", GrantTierGrantable, true, "View workflow"),
	"workflow.review":                    policy("workflow.review", "workflow", GrantTierGrantable, true, "Review step"),
	"workflow.approve":                   policy("workflow.approve", "workflow", GrantTierGrantable, true, "Approve step"),
	"workflow.confirm":                   policy("workflow.confirm", "workflow", GrantTierGrantable, true, "Confirm step"),
	"deadline.view":                      policy("deadline.view", "deadline", GrantTierGrantable, true, "View deadlines"),
	"deadline.create":                    policy("deadline.create", "workflow", GrantTierGrantable, true, "Create workflow/deadline"),
	"deadline.assign":                    policy("deadline.assign", "workflow", GrantTierGrantable, true, "Assign tasks"),
	"deadline.manage":                    policy("deadline.manage", "workflow", GrantTierGrantable, true, "Manage/confirm"),
	"dashboard.view":                     policy("dashboard.view", "dashboard", GrantTierGrantable, true, "Dashboard access"),
	"ad_hoc_alert.read":                  policy("ad_hoc_alert.read", "ad_hoc", GrantTierGrantable, true, "View ad-hoc"),
	"ad_hoc_alert.propose":               policy("ad_hoc_alert.propose", "ad_hoc", GrantTierGrantable, true, "Propose"),
	"ad_hoc_alert.focal_review":          policy("ad_hoc_alert.focal_review", "ad_hoc", GrantTierGrantable, true, "Focal review"),
	"template.workflow.override.read":    policy("template.workflow.override.read", "template", GrantTierGrantable, true, "Read overrides"),
	"template.workflow.override.write":   policy("template.workflow.override.write", "template", GrantTierGrantable, true, "Write overrides"),
	"template.workflow.override.approve": policy("template.workflow.override.approve", "template", GrantTierGrantable, true, "Approve overrides"),
}

func policy(code, module string, tier PermissionGrantTier, custom bool, reason string) PermissionGrantPolicy {
	return PermissionGrantPolicy{
		PermissionCode:             code,
		ModuleName:                 module,
		GrantTier:                  tier,
		AllowedOnCustomRole:        custom,
		AllowedOnTenantDefaultEdit: false,
		RiskLevel:                  grantPolicyRiskLevel(tier, code),
		Reason:                     reason,
	}
}

func grantPolicyRiskLevel(tier PermissionGrantTier, code string) string {
	switch tier {
	case GrantTierHighRisk:
		return "high"
	case GrantTierTenantAdminOnly:
		return "critical"
	case GrantTierSystemOnly, GrantTierDeprecatedOrUnknown:
		return "medium"
	default:
		return PermissionRiskLevel(code)
	}
}

// LookupGrantPolicy returns the static conservative policy for a permission code.
// Unknown codes default to deprecated_or_unknown with allowedOnCustomRole=false.
func LookupGrantPolicy(permissionCode string) PermissionGrantPolicy {
	code := strings.TrimSpace(permissionCode)
	if strings.HasPrefix(code, "admin.notification_rule.") {
		return policy(code, "admin", GrantTierDeprecatedOrUnknown, false, "Not in permissions catalog")
	}
	if p, ok := staticGrantPolicies[code]; ok {
		return p
	}
	return PermissionGrantPolicy{
		PermissionCode:             code,
		ModuleName:                 "general",
		GrantTier:                  GrantTierDeprecatedOrUnknown,
		AllowedOnCustomRole:        false,
		AllowedOnTenantDefaultEdit: false,
		RiskLevel:                  "medium",
		Reason:                     "Permission not classified; blocked by default",
	}
}

// IsRoleProtectedForMutation reports whether role-permission mutations must be rejected (D3).
func IsRoleProtectedForMutation(role *RoleListItem) bool {
	if role == nil {
		return true
	}
	if role.IsProtected {
		return true
	}
	switch strings.TrimSpace(role.RoleType) {
	case RoleTypeSystemGlobal, RoleTypeTenantDefault:
		return true
	}
	return false
}

// ErrProtectedRoleReadOnly is returned when mutating a protected/default role.
func ErrProtectedRoleReadOnly() error {
	return perr.NewHTTPError(
		http.StatusForbidden,
		perr.CodeProtectedRoleReadOnly,
		"Vai trò mặc định không thể chỉnh sửa. Hãy nhân bản thành vai trò tùy chỉnh.",
		nil,
	)
}

// ValidateAssignPermissionGrant enforces D2/D10 mutation boundary for AssignRolePermission.
func ValidateAssignPermissionGrant(policy PermissionGrantPolicy) error {
	switch policy.GrantTier {
	case GrantTierGrantable:
		if !policy.AllowedOnCustomRole {
			return errPermissionNotGrantable()
		}
		return nil
	case GrantTierHighRisk:
		return errHighRiskPermissionRequiresApproval()
	default:
		return errPermissionNotGrantable()
	}
}

func errPermissionNotGrantable() error {
	return perr.NewHTTPError(
		http.StatusUnprocessableEntity,
		perr.CodePermissionNotGrantable,
		"Quyền này không được cấp cho vai trò tùy chỉnh.",
		nil,
	)
}

func errHighRiskPermissionRequiresApproval() error {
	return perr.NewHTTPError(
		http.StatusUnprocessableEntity,
		perr.CodeHighRiskPermissionRequiresApproval,
		"Quyền nguy cơ cao cần quy trình phê duyệt.",
		nil,
	)
}

// BuildGrantableCatalogItems merges DB permissions with static grant policy.
func BuildGrantableCatalogItems(dbPerms []PermissionListItem) []GrantablePermissionItem {
	seen := make(map[string]struct{}, len(dbPerms)+len(staticGrantPolicies))
	out := make([]GrantablePermissionItem, 0, len(dbPerms)+8)

	appendItem := func(code, name, module string) {
		if code == "" {
			return
		}
		if _, ok := seen[code]; ok {
			return
		}
		seen[code] = struct{}{}
		pol := LookupGrantPolicy(code)
		if module != "" {
			pol.ModuleName = module
		}
		out = append(out, GrantablePermissionItem{
			PermissionCode:             code,
			PermissionName:             name,
			ModuleName:                 pol.ModuleName,
			GrantTier:                  pol.GrantTier,
			RiskLevel:                  pol.RiskLevel,
			AllowedOnCustomRole:        pol.AllowedOnCustomRole,
			AllowedOnTenantDefaultEdit: pol.AllowedOnTenantDefaultEdit,
			Reason:                     pol.Reason,
		})
	}

	for _, p := range dbPerms {
		appendItem(strings.TrimSpace(p.PermissionCode), p.PermissionName, p.ModuleName)
	}
	// CODE_ONLY entries useful for policy display when absent from DB.
	for code, pol := range staticGrantPolicies {
		if pol.GrantTier == GrantTierTenantAdminOnly || pol.GrantTier == GrantTierSystemOnly {
			appendItem(code, code, pol.ModuleName)
		}
	}

	return out
}
