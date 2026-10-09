package app_test

import (
	"reflect"
	"testing"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	"github.com/cobo/cobo_iam_services/internal/companyaccess/configversion"
)

func planPerm(id, module string) caapp.PermissionListItem {
	return caapp.PermissionListItem{PermissionID: id, PermissionCode: id, PermissionName: id, ModuleName: module}
}

func planRole(id, roleType string, protected bool) caapp.RoleListItem {
	return caapp.RoleListItem{RoleID: id, RoleCode: id, Status: "active", RoleType: roleType, IsProtected: protected}
}

func planSnap(entries ...[2]string) configversion.RBACMatrixSnapshot {
	snap := configversion.RBACMatrixSnapshot{SchemaVersion: configversion.RBACMatrixSnapshotSchema}
	for _, e := range entries {
		snap.RolePermissions = append(snap.RolePermissions, configversion.RolePermissionEntry{RoleID: e[0], PermissionID: e[1]})
	}
	return snap
}

type planOp struct {
	Role, Perm string
	Add        bool
}

func flattenPlan(p caapp.RBACRestorePlan) []planOp {
	out := make([]planOp, 0, len(p.Ops))
	for _, op := range p.Ops {
		out = append(out, planOp{Role: op.RoleID, Perm: op.PermissionID, Add: op.Add})
	}
	return out
}

func TestComputeRBACMatrixRestorePlan(t *testing.T) {
	catalog := []caapp.PermissionListItem{
		planPerm("disclosure.view", "disclosure"),
		planPerm("deadline.view", "deadline"),
		planPerm("deadline.comment", "deadline"),
		planPerm("rbac.manage", "admin"),
		planPerm("platform.cms.view", "platform"),
		planPerm("cms.template.write", "cms"),
		planPerm("disclosure_type.config.write", "disclosure_type"), // module looks ordinary, code is on the deny list
		planPerm("dashboard.view", "dashboard"),
		planPerm("company.ownership.transfer", "org"), // high_risk grant tier, not in the critical code set
	}
	custom := planRole("custom", caapp.RoleTypeTenantCustom, false)
	global := planRole("global", caapp.RoleTypeSystemGlobal, true)
	tenantDefault := planRole("default", caapp.RoleTypeTenantDefault, true)
	customButProtected := planRole("custom_protected", caapp.RoleTypeTenantCustom, true)

	tests := []struct {
		name         string
		roles        []caapp.RoleListItem
		current      map[string][]caapp.PermissionListItem
		snap         configversion.RBACMatrixSnapshot
		want         []planOp
		wantCritical bool
	}{
		{
			name:    "custom role converges to the snapshot",
			roles:   []caapp.RoleListItem{custom},
			current: map[string][]caapp.PermissionListItem{"custom": {planPerm("disclosure.view", "disclosure"), planPerm("deadline.view", "deadline")}},
			snap:    planSnap([2]string{"custom", "disclosure.view"}, [2]string{"custom", "deadline.comment"}),
			want: []planOp{
				{"custom", "deadline.view", false},
				{"custom", "deadline.comment", true},
			},
		},
		{
			name:    "nothing to do when the custom role already matches",
			roles:   []caapp.RoleListItem{custom},
			current: map[string][]caapp.PermissionListItem{"custom": {planPerm("disclosure.view", "disclosure")}},
			snap:    planSnap([2]string{"custom", "disclosure.view"}),
			want:    []planOp{},
		},
		{
			name:  "global, tenant_default and protected roles are never reconciled",
			roles: []caapp.RoleListItem{global, tenantDefault, customButProtected},
			current: map[string][]caapp.PermissionListItem{
				"global":           {planPerm("deadline.view", "deadline")},
				"default":          {planPerm("deadline.view", "deadline")},
				"custom_protected": {planPerm("deadline.view", "deadline")},
			},
			snap: planSnap([2]string{"global", "disclosure.view"}, [2]string{"default", "disclosure.view"}, [2]string{"custom_protected", "disclosure.view"}),
			want: []planOp{},
		},
		{
			name:  "snapshot entries of roles that are not in the company list are ignored",
			roles: []caapp.RoleListItem{custom},
			current: map[string][]caapp.PermissionListItem{
				"custom": {},
			},
			snap: planSnap([2]string{"custom", "disclosure.view"}, [2]string{"someone_elses_role", "disclosure.view"}),
			want: []planOp{{"custom", "disclosure.view", true}},
		},
		{
			name:  "permissions outside the enterprise scope are never removed",
			roles: []caapp.RoleListItem{custom},
			current: map[string][]caapp.PermissionListItem{
				"custom": {planPerm("platform.cms.view", "platform"), planPerm("cms.template.write", "cms"), planPerm("disclosure_type.config.write", "disclosure_type"), planPerm("deadline.view", "deadline")},
			},
			snap: planSnap(),
			want: []planOp{{"custom", "deadline.view", false}},
		},
		{
			name:  "permissions outside the enterprise scope are never added, unknown permissions are ignored",
			roles: []caapp.RoleListItem{custom},
			current: map[string][]caapp.PermissionListItem{
				"custom": {},
			},
			snap: planSnap([2]string{"custom", "platform.cms.view"}, [2]string{"custom", "cms.template.write"}, [2]string{"custom", "not_in_catalog"}, [2]string{"custom", "deadline.view"}),
			want: []planOp{{"custom", "deadline.view", true}},
		},
		{
			name:    "adds follow the grant policy: non-grantable permissions are skipped, grantable ones are added",
			roles:   []caapp.RoleListItem{custom},
			current: map[string][]caapp.PermissionListItem{"custom": {}},
			snap: planSnap([2]string{"custom", "rbac.manage"}, [2]string{"custom", "company.ownership.transfer"},
				[2]string{"custom", "dashboard.view"}),
			want: []planOp{{"custom", "dashboard.view", true}},
		},
		{
			name:         "removing a high-risk permission is critical even when it is not in the critical code set",
			roles:        []caapp.RoleListItem{custom},
			current:      map[string][]caapp.PermissionListItem{"custom": {planPerm("company.ownership.transfer", "org")}},
			snap:         planSnap(),
			want:         []planOp{{"custom", "company.ownership.transfer", false}},
			wantCritical: true,
		},
		{
			name:         "removing a critical permission is flagged",
			roles:        []caapp.RoleListItem{custom},
			current:      map[string][]caapp.PermissionListItem{"custom": {planPerm("rbac.manage", "admin")}},
			snap:         planSnap(),
			want:         []planOp{{"custom", "rbac.manage", false}},
			wantCritical: true,
		},
		{
			name:  "a critical permission on an ignored role does not make the plan critical",
			roles: []caapp.RoleListItem{global, custom},
			current: map[string][]caapp.PermissionListItem{
				"global": {planPerm("rbac.manage", "admin")},
				"custom": {planPerm("disclosure.view", "disclosure")},
			},
			snap: planSnap([2]string{"custom", "disclosure.view"}),
			want: []planOp{},
		},
		{
			name:    "role without any snapshot entry loses its enterprise permissions",
			roles:   []caapp.RoleListItem{custom},
			current: map[string][]caapp.PermissionListItem{"custom": {planPerm("disclosure.view", "disclosure")}},
			snap:    planSnap(),
			want:    []planOp{{"custom", "disclosure.view", false}},
		},
		{
			name:    "inactive custom role is ignored",
			roles:   []caapp.RoleListItem{{RoleID: "custom", Status: "inactive", RoleType: caapp.RoleTypeTenantCustom}},
			current: map[string][]caapp.PermissionListItem{"custom": {planPerm("disclosure.view", "disclosure")}},
			snap:    planSnap(),
			want:    []planOp{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := caapp.ComputeRBACMatrixRestorePlan(tc.roles, tc.current, tc.snap, catalog)
			if gotOps := flattenPlan(got); !reflect.DeepEqual(gotOps, tc.want) {
				t.Errorf("ops = %+v, want %+v", gotOps, tc.want)
			}
			if got.TouchesCritical != tc.wantCritical {
				t.Errorf("TouchesCritical = %v, want %v", got.TouchesCritical, tc.wantCritical)
			}
		})
	}
}

// The in-memory repository reports every role permission with module "general" and code == id.
// The plan must classify through the catalog, not through that placeholder.
func TestComputeRBACMatrixRestorePlan_ClassifiesThroughCatalog(t *testing.T) {
	catalog := []caapp.PermissionListItem{
		{PermissionID: "perm_cms", PermissionCode: "cms.template.write", ModuleName: "cms"},
		{PermissionID: "perm_cms", PermissionCode: "perm_cms", ModuleName: "general"}, // synthetic duplicate
		{PermissionID: "perm_ok", PermissionCode: "deadline.view", ModuleName: "deadline"},
	}
	custom := planRole("custom", caapp.RoleTypeTenantCustom, false)
	current := map[string][]caapp.PermissionListItem{
		"custom": {
			{PermissionID: "perm_cms", PermissionCode: "perm_cms", ModuleName: "general"},
			{PermissionID: "perm_ok", PermissionCode: "perm_ok", ModuleName: "general"},
		},
	}
	got := caapp.ComputeRBACMatrixRestorePlan([]caapp.RoleListItem{custom}, current, planSnap(), catalog)
	want := []planOp{{"custom", "perm_ok", false}}
	if gotOps := flattenPlan(got); !reflect.DeepEqual(gotOps, want) {
		t.Errorf("ops = %+v, want %+v", gotOps, want)
	}
}

func directEntry(membership, code string) configversion.DirectPermissionEntry {
	return configversion.DirectPermissionEntry{MembershipID: membership, PermissionCode: code}
}

func TestComputeRBACDirectRestorePlan(t *testing.T) {
	tests := []struct {
		name         string
		current      []configversion.DirectPermissionEntry
		target       []configversion.DirectPermissionEntry
		wantRevoke   []configversion.DirectPermissionEntry
		wantGrant    []configversion.DirectPermissionEntry
		wantCritical bool
	}{
		{
			name:       "grantable grants converge to the target",
			current:    []configversion.DirectPermissionEntry{directEntry("m1", "template.workflow.override.read")},
			target:     []configversion.DirectPermissionEntry{directEntry("m1", "template.workflow.override.write")},
			wantRevoke: []configversion.DirectPermissionEntry{directEntry("m1", "template.workflow.override.read")},
			wantGrant:  []configversion.DirectPermissionEntry{directEntry("m1", "template.workflow.override.write")},
		},
		{
			name:    "unchanged grants produce no operation",
			current: []configversion.DirectPermissionEntry{directEntry("m1", "template.workflow.override.read")},
			target:  []configversion.DirectPermissionEntry{directEntry("m1", "template.workflow.override.read")},
		},
		{
			name: "codes outside the tenant-grantable list are never revoked",
			current: []configversion.DirectPermissionEntry{
				directEntry("m1", "platform.cms.view"), directEntry("m1", "ad_hoc_alert.process_control"), directEntry("m1", "ad_hoc_alert.read"),
			},
		},
		{
			name: "codes outside the tenant-grantable list are never granted",
			target: []configversion.DirectPermissionEntry{
				directEntry("m1", "platform.cms.view"), directEntry("m1", "rbac.manage"), directEntry("m1", "ad_hoc_alert.process_control"),
			},
		},
		{
			name:         "granting a critical direct permission is flagged",
			target:       []configversion.DirectPermissionEntry{directEntry("m1", "admin.membership.invite")},
			wantGrant:    []configversion.DirectPermissionEntry{directEntry("m1", "admin.membership.invite")},
			wantCritical: true,
		},
		{
			name:         "revoking a critical direct permission is flagged",
			current:      []configversion.DirectPermissionEntry{directEntry("m1", "admin.membership.invite")},
			wantRevoke:   []configversion.DirectPermissionEntry{directEntry("m1", "admin.membership.invite")},
			wantCritical: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := caapp.ComputeRBACDirectRestorePlan(tc.current, tc.target)
			if !reflect.DeepEqual(nilToEmpty(got.Revoke), nilToEmpty(tc.wantRevoke)) {
				t.Errorf("revoke = %+v, want %+v", got.Revoke, tc.wantRevoke)
			}
			if !reflect.DeepEqual(nilToEmpty(got.Grant), nilToEmpty(tc.wantGrant)) {
				t.Errorf("grant = %+v, want %+v", got.Grant, tc.wantGrant)
			}
			if got.TouchesCritical != tc.wantCritical {
				t.Errorf("TouchesCritical = %v, want %v", got.TouchesCritical, tc.wantCritical)
			}
		})
	}
}

func nilToEmpty(in []configversion.DirectPermissionEntry) []configversion.DirectPermissionEntry {
	if in == nil {
		return []configversion.DirectPermissionEntry{}
	}
	return in
}
