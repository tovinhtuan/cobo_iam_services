package app

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/cobo/cobo_iam_services/internal/companyaccess/configversion"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// RBACRestoreOp is one role_permissions change needed to bring a role to a snapshot.
type RBACRestoreOp struct {
	RoleID         string
	PermissionID   string
	PermissionCode string
	Add            bool // false = remove
}

// RBACRestorePlan is the full set of changes a rollback or an approval apply may perform.
type RBACRestorePlan struct {
	Ops []RBACRestoreOp
	// TouchesCritical is true when the plan adds or removes a critical permission. Such a
	// rollback is routed through the approval queue instead of being applied directly.
	TouchesCritical bool
}

// ComputeRBACMatrixRestorePlan decides what restoring an RBAC matrix snapshot may change.
// It is the single place that defines the boundary of a restore, shared by every repository:
//
//   - only tenant_custom, non-protected, active roles of the company are reconciled (roles
//     holds the roles visible to the company, so global and tenant_default roles are skipped
//     here, and roles of other companies never appear);
//   - only enterprise-scope permissions are added or removed: cms/platform permissions, deny
//     listed codes, and permissions the catalog does not know are left exactly as they are.
//
// current maps role_id to the permissions the role holds now; catalog is the permission
// catalog used to classify snapshot entries and to resolve placeholder metadata.
func ComputeRBACMatrixRestorePlan(
	roles []RoleListItem,
	current map[string][]PermissionListItem,
	snap configversion.RBACMatrixSnapshot,
	catalog []PermissionListItem,
) RBACRestorePlan {
	byID := make(map[string]PermissionListItem, len(catalog))
	for _, p := range catalog {
		// Prefer real catalog entries (code != id) over synthetic id==code placeholders.
		if existing, ok := byID[p.PermissionID]; ok && existing.PermissionCode != existing.PermissionID {
			continue
		}
		byID[p.PermissionID] = p
	}
	resolve := func(permissionID string, fallback *PermissionListItem) (PermissionListItem, bool) {
		if item, ok := byID[permissionID]; ok {
			return item, true
		}
		if fallback != nil {
			return *fallback, true
		}
		return PermissionListItem{}, false
	}

	wantByRole := make(map[string]map[string]struct{})
	for _, e := range snap.RolePermissions {
		set := wantByRole[e.RoleID]
		if set == nil {
			set = map[string]struct{}{}
			wantByRole[e.RoleID] = set
		}
		set[e.PermissionID] = struct{}{}
	}

	eligible := restorableRoles(roles)

	plan := RBACRestorePlan{Ops: []RBACRestoreOp{}}
	for _, role := range eligible {
		want := map[string]PermissionListItem{}
		for permID := range wantByRole[role.RoleID] {
			item, ok := resolve(permID, nil)
			if !ok || !IsEnterprisePermission(item.PermissionCode, item.ModuleName) {
				continue
			}
			want[permID] = item
		}
		have := map[string]PermissionListItem{}
		for i := range current[role.RoleID] {
			p := current[role.RoleID][i]
			item, _ := resolve(p.PermissionID, &p)
			have[p.PermissionID] = item
		}

		var removes, adds []RBACRestoreOp
		for permID, item := range have {
			if !IsEnterprisePermission(item.PermissionCode, item.ModuleName) {
				continue
			}
			if _, keep := want[permID]; !keep {
				removes = append(removes, RBACRestoreOp{RoleID: role.RoleID, PermissionID: permID, PermissionCode: item.PermissionCode})
			}
		}
		for permID, item := range want {
			if _, ok := have[permID]; ok {
				continue
			}
			// A restore grants nothing that AssignRolePermission would refuse for a custom role.
			if !grantableOnCustomRole(item.PermissionCode) {
				continue
			}
			adds = append(adds, RBACRestoreOp{RoleID: role.RoleID, PermissionID: permID, PermissionCode: item.PermissionCode, Add: true})
		}
		sort.Slice(removes, func(i, j int) bool { return removes[i].PermissionID < removes[j].PermissionID })
		sort.Slice(adds, func(i, j int) bool { return adds[i].PermissionID < adds[j].PermissionID })
		for _, op := range append(removes, adds...) {
			if isCriticalForRestore(op.PermissionCode) {
				plan.TouchesCritical = true
			}
			plan.Ops = append(plan.Ops, op)
		}
	}
	return plan
}

// grantableOnCustomRole mirrors the check AssignRolePermission applies before it grants a
// permission to a tenant_custom role.
func grantableOnCustomRole(code string) bool {
	policy := LookupGrantPolicy(code)
	return policy.GrantTier == GrantTierGrantable && policy.AllowedOnCustomRole
}

// isCriticalForRestore reports whether adding or removing the permission through a restore
// needs a second person. It is the critical code set plus every permission the grant policy
// classifies as tenant_admin_only or high_risk, so the two definitions cannot drift apart.
func isCriticalForRestore(code string) bool {
	if isCriticalPermissionCode(code) {
		return true
	}
	switch LookupGrantPolicy(code).GrantTier {
	case GrantTierTenantAdminOnly, GrantTierHighRisk:
		return true
	}
	return false
}

// RBACDirectRestorePlan lists the direct permission grants a restore may revoke or grant.
type RBACDirectRestorePlan struct {
	Revoke          []configversion.DirectPermissionEntry
	Grant           []configversion.DirectPermissionEntry
	TouchesCritical bool
}

// ComputeRBACDirectRestorePlan decides how direct grants converge from current to target.
// Only permissions a tenant admin may grant directly (GrantablePermissions) are considered, so
// a restore never revokes or grants platform/cms permissions, non-grantable permissions
// seeded by migrations, or anything else the direct-permission API cannot manage. Grants that
// are already in place are not repeated.
func ComputeRBACDirectRestorePlan(current, target []configversion.DirectPermissionEntry) RBACDirectRestorePlan {
	managed := func(code string) bool {
		if _, denied := EnterpriseDenyCodes[code]; denied {
			return false
		}
		return IsGrantablePermission(code)
	}
	key := func(d configversion.DirectPermissionEntry) string { return d.MembershipID + ":" + d.PermissionCode }
	have := make(map[string]struct{}, len(current))
	for _, d := range current {
		have[key(d)] = struct{}{}
	}
	want := make(map[string]struct{}, len(target))
	for _, d := range target {
		want[key(d)] = struct{}{}
	}

	plan := RBACDirectRestorePlan{Revoke: []configversion.DirectPermissionEntry{}, Grant: []configversion.DirectPermissionEntry{}}
	seen := map[string]struct{}{}
	for _, d := range current {
		if _, dup := seen["r"+key(d)]; dup || !managed(d.PermissionCode) {
			continue
		}
		seen["r"+key(d)] = struct{}{}
		if _, keep := want[key(d)]; !keep {
			plan.Revoke = append(plan.Revoke, d)
		}
	}
	for _, d := range target {
		if _, dup := seen["g"+key(d)]; dup || !managed(d.PermissionCode) {
			continue
		}
		seen["g"+key(d)] = struct{}{}
		if _, ok := have[key(d)]; !ok {
			plan.Grant = append(plan.Grant, d)
		}
	}
	order := func(list []configversion.DirectPermissionEntry) {
		sort.Slice(list, func(i, j int) bool { return key(list[i]) < key(list[j]) })
	}
	order(plan.Revoke)
	order(plan.Grant)
	for _, d := range append(append([]configversion.DirectPermissionEntry{}, plan.Revoke...), plan.Grant...) {
		if isCriticalForRestore(d.PermissionCode) {
			plan.TouchesCritical = true
		}
	}
	return plan
}

// restorableRoles keeps the roles a restore is allowed to change: active, non-protected
// tenant_custom roles, sorted by role_id.
func restorableRoles(roles []RoleListItem) []RoleListItem {
	out := make([]RoleListItem, 0, len(roles))
	for _, role := range roles {
		item := role
		FinalizeRoleListItem(&item)
		if item.RoleType != RoleTypeTenantCustom || item.IsProtected || !strings.EqualFold(strings.TrimSpace(item.Status), "active") {
			continue
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RoleID < out[j].RoleID })
	return out
}

// RBACRestorePlanReader is the part of AdminRepository needed to plan a restore.
type RBACRestorePlanReader interface {
	ListRoles(ctx context.Context, companyID string) ([]RoleListItem, error)
	ListRolePermissions(ctx context.Context, companyID, roleID string) (*RolePermissionsView, error)
	ListPermissions(ctx context.Context) ([]PermissionListItem, error)
}

// BuildRBACRestorePlan reads the current state through r and plans how to restore the raw
// snapshot for one company. It does not write anything.
func BuildRBACRestorePlan(ctx context.Context, r RBACRestorePlanReader, companyID string, rawSnapshot []byte) (RBACRestorePlan, error) {
	var snap configversion.RBACMatrixSnapshot
	if err := json.Unmarshal(rawSnapshot, &snap); err != nil {
		return RBACRestorePlan{}, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid snapshot_json", nil)
	}
	roles, err := r.ListRoles(ctx, companyID)
	if err != nil {
		return RBACRestorePlan{}, err
	}
	catalog, err := r.ListPermissions(ctx)
	if err != nil {
		return RBACRestorePlan{}, err
	}
	current := make(map[string][]PermissionListItem)
	for _, role := range restorableRoles(roles) {
		view, err := r.ListRolePermissions(ctx, companyID, role.RoleID)
		if err != nil {
			return RBACRestorePlan{}, err
		}
		current[role.RoleID] = view.Permissions
	}
	return ComputeRBACMatrixRestorePlan(roles, current, snap, catalog), nil
}
