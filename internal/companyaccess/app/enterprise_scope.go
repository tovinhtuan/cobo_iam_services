package app

import (
	"context"
	"encoding/json"

	"github.com/cobo/cobo_iam_services/internal/companyaccess/configversion"
)

// EnterpriseDenyModules lists module_name values whose permissions must never appear
// in enterprise RBAC: neither the list API response nor the assign mutation.
// Source of truth: migrations/0034, 0058, 0009 — module_name seeded in permissions table.
var EnterpriseDenyModules = map[string]struct{}{
	"cms":      {},
	"platform": {},
}

// EnterpriseDenyCodes is an explicit code-level deny list for enterprise RBAC.
// Belt-and-suspenders: catches any permission whose module_name might drift in a migration
// or be returned as "general" by the in-memory repo during tests.
var EnterpriseDenyCodes = map[string]struct{}{
	"cms.template.read":            {},
	"cms.template.write":           {},
	"cms.template.activate":        {},
	"cms.template.archive":         {},
	"cms.template.config.write":    {},
	"disclosure_type.config.read":  {},
	"disclosure_type.config.write": {},
	"platform.cms.view":            {},
}

// IsEnterprisePermission returns true when the permission is safe to expose and assign
// in the Enterprise RBAC context (/api/v1/admin/permissions and role-permission mutations).
//
// A permission is blocked when:
//   - its module_name is in EnterpriseDenyModules, OR
//   - its code is in EnterpriseDenyCodes (belt-and-suspenders for module drift).
//
// Permissions that are NOT blocked (confirmed enterprise-scoped):
//   - disclosure_type.manage        (enterprise company manages its own CBTT types)
//   - template.workflow.override.*  (enterprise overrides its own workflow templates)
//   - disclosure.*                  (core CBTT operations)
//   - rbac.manage, system.settings  (enterprise admin controls)
//   - dept.manage, company.*        (enterprise organisation)
func IsEnterprisePermission(code, moduleName string) bool {
	if _, blocked := EnterpriseDenyModules[moduleName]; blocked {
		return false
	}
	if _, blocked := EnterpriseDenyCodes[code]; blocked {
		return false
	}
	return true
}

// filterEnterpriseRBACSnapshotJSON removes out-of-enterprise-scope permissions from a
// raw RBACMatrixSnapshot JSON before it is written to the versioning store, config export,
// approval proposal, or rollback target.
//
// Strategy:
//   - For role_permissions entries: look up each permission_id in the catalog via
//     listPermsFn; apply IsEnterprisePermission. If the ID is unknown (e.g. stale catalog),
//     the entry is kept (fail-open) to avoid accidental data loss.
//   - For direct_permissions entries: filter by EnterpriseDenyCodes on permission_code.
//
// If the raw bytes cannot be unmarshalled (corrupted snapshot), the original bytes are
// returned unchanged to preserve the record.
func filterEnterpriseRBACSnapshotJSON(
	ctx context.Context,
	listPermsFn func(context.Context) ([]PermissionListItem, error),
	raw []byte,
) ([]byte, error) {
	var snap configversion.RBACMatrixSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return raw, nil
	}

	permByID := map[string]PermissionListItem{}
	if all, err := listPermsFn(ctx); err == nil {
		for _, p := range all {
			// Prefer catalog entries where Code ≠ ID (real seeded/DB entries).
			// In the in-memory test repo, AddRolePermission adds synthetic entries
			// where Code == ID; those should not overwrite real entries.
			if existing, ok := permByID[p.PermissionID]; ok && existing.PermissionCode != existing.PermissionID {
				continue
			}
			permByID[p.PermissionID] = p
		}
	}

	filteredRP := make([]configversion.RolePermissionEntry, 0, len(snap.RolePermissions))
	for _, e := range snap.RolePermissions {
		if item, ok := permByID[e.PermissionID]; ok {
			if !IsEnterprisePermission(item.PermissionCode, item.ModuleName) {
				continue
			}
		}
		filteredRP = append(filteredRP, e)
	}
	snap.RolePermissions = filteredRP

	filteredDP := make([]configversion.DirectPermissionEntry, 0, len(snap.DirectPermissions))
	for _, d := range snap.DirectPermissions {
		if _, blocked := EnterpriseDenyCodes[d.PermissionCode]; blocked {
			continue
		}
		filteredDP = append(filteredDP, d)
	}
	snap.DirectPermissions = filteredDP

	return json.Marshal(snap)
}
