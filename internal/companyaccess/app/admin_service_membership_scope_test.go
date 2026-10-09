package app_test

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	"github.com/cobo/cobo_iam_services/internal/companyaccess/configversion"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

// C5 (risk review 2026-10-09): service methods keyed by membership_id, or by the id of a team,
// department or title, must only act on objects of the company in the access token. Objects of
// another company answer 404 and are left untouched.

const (
	msOwn     = "c_001"
	msForeign = "c_002"
	msPerm    = "template.workflow.override.read" // grantable, no approval needed on removal
)

// objects of one company used by the table tests.
type msObjs struct {
	company       string
	member        string
	role, roleB   string
	dept, deptB   string
	title, titleB string
	team, teamB   string
}

type msFixture struct {
	svc     caapp.AdminService
	repo    *cainmem.AdminRepository
	caller  caapp.AdminSubject
	own     msObjs
	foreign msObjs
}

func msSeedCompany(t *testing.T, repo *cainmem.AdminRepository, company, tag string) msObjs {
	t.Helper()
	ctx := context.Background()
	o := msObjs{
		company: company, member: "m_" + tag,
		role: "role_" + tag + "_a", roleB: "role_" + tag + "_b",
		dept: "d_" + tag + "_a", deptB: "d_" + tag + "_b",
		title: "t_" + tag + "_a", titleB: "t_" + tag + "_b",
		team: "team_" + tag + "_a", teamB: "team_" + tag + "_b",
	}
	if _, err := repo.CreateUser(ctx, caapp.UserView{
		UserID: "u_" + tag, LoginID: tag + "@example.com", FullName: tag, AccountStatus: "active",
	}, "hash", caapp.CreateUserOptions{MembershipID: o.member, CompanyID: company, MembershipStatus: "active"}); err != nil {
		t.Fatalf("seed member %s: %v", tag, err)
	}
	for _, id := range []string{o.role, o.roleB} {
		repo.SeedRole(caapp.RoleListItem{RoleID: id, RoleCode: id, RoleName: id, Status: "active", Scope: "company", RoleType: caapp.RoleTypeTenantCustom})
		if err := repo.AddRolePermission(ctx, id, "disclosure.view"); err != nil {
			t.Fatalf("seed role permission: %v", err)
		}
	}
	for _, id := range []string{o.dept, o.deptB} {
		repo.SeedDepartmentForCompany(company, caapp.DepartmentView{DepartmentID: id, DepartmentName: id, Status: "active"})
	}
	for _, id := range []string{o.title, o.titleB} {
		repo.SeedTitleForCompany(company, caapp.TitleView{TitleID: id, TitleName: id, Status: "active"})
	}
	repo.SeedTeam(company, o.dept, o.team, o.team)
	repo.SeedTeam(company, o.dept, o.teamB, o.teamB)
	// Initial state of the member: one role, department, title, direct permission and team.
	if err := repo.AddRole(ctx, o.member, o.role); err != nil {
		t.Fatal(err)
	}
	if err := repo.AddDepartment(ctx, o.member, o.dept); err != nil {
		t.Fatal(err)
	}
	if err := repo.AddTitle(ctx, o.member, o.title); err != nil {
		t.Fatal(err)
	}
	if err := repo.InsertDirectPermission(ctx, o.member, company, msPerm, "seed"); err != nil {
		t.Fatal(err)
	}
	if err := repo.AddTeamMember(ctx, company, o.team, o.member); err != nil {
		t.Fatal(err)
	}
	return o
}

func newMSFixture(t *testing.T, perms []string) *msFixture {
	t.Helper()
	repo := cainmem.NewAdminRepository()
	caller := caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: msOwn}
	seedInviteScopedSubject(t, repo, caller)
	f := &msFixture{
		repo:    repo,
		caller:  caller,
		own:     msSeedCompany(t, repo, msOwn, "own"),
		foreign: msSeedCompany(t, repo, msForeign, "foreign"),
	}
	f.svc = caapp.NewAdminService(repo, fakeAuthService{decision: authapp.DecisionAllow, permissions: perms}, idgen.UUIDv7Generator{})
	return f
}

// snapshot captures everything the tests expect to stay unchanged for a membership.
func (f *msFixture) snapshot(t *testing.T, o msObjs) string {
	t.Helper()
	ctx := context.Background()
	m, err := f.repo.GetMembershipByID(ctx, o.member)
	if err != nil {
		return "membership gone"
	}
	roles, _ := f.repo.ListMembershipRoles(ctx, o.member)
	roleIDs := make([]string, 0, len(roles))
	for _, r := range roles {
		roleIDs = append(roleIDs, r.RoleID)
	}
	sort.Strings(roleIDs)
	depts, _ := f.repo.ListActiveMembershipDepartmentIDs(ctx, o.member)
	sort.Strings(depts)
	titles, _ := f.repo.ListActiveMembershipTitleIDs(ctx, o.member)
	sort.Strings(titles)
	perms, _ := f.repo.ListActiveDirectPermissions(ctx, o.member)
	permCodes := make([]string, 0, len(perms))
	for _, p := range perms {
		permCodes = append(permCodes, p.PermissionCode)
	}
	sort.Strings(permCodes)
	var teams []string
	for _, team := range []string{o.team, o.teamB} {
		for _, id := range f.repo.TeamMemberIDs(team) {
			if id == o.member {
				teams = append(teams, team)
			}
		}
	}
	return fmt.Sprintf("status=%s roles=%v depts=%v titles=%v perms=%v teams=%v", m.Status, roleIDs, depts, titles, permCodes, teams)
}

type msCase struct {
	name      string
	needsRBAC bool // the method requires rbac.manage itself (not reachable with an invite-only caller)
	call      func(f *msFixture, o msObjs) error
	// prep adjusts the fixture through the repo before the snapshot is taken (optional).
	prep func(f *msFixture, o msObjs)
}

func msCases() []msCase {
	ctx := context.Background()
	return []msCase{
		{"UpdateMembership", false, func(f *msFixture, o msObjs) error {
			_, err := f.svc.UpdateMembership(ctx, caapp.UpdateMembershipRequest{Subject: f.caller, MembershipID: o.member, Status: "inactive"})
			return err
		}, nil},
		{"DeleteMembership", false, func(f *msFixture, o msObjs) error {
			return f.svc.DeleteMembership(ctx, caapp.DeleteMembershipRequest{Subject: f.caller, MembershipID: o.member})
		}, nil},
		{"AssignRole", false, func(f *msFixture, o msObjs) error {
			return f.svc.AssignRole(ctx, caapp.AssignRoleRequest{Subject: f.caller, MembershipID: o.member, RoleID: o.roleB})
		}, func(f *msFixture, o msObjs) {
			// AssignRole only accepts a membership without a primary role: drop the seeded role first.
			_ = f.repo.RemoveRole(ctx, o.member, o.role)
		}},
		{"RemoveRole", false, func(f *msFixture, o msObjs) error {
			return f.svc.RemoveRole(ctx, caapp.RemoveRoleRequest{Subject: f.caller, MembershipID: o.member, RoleID: o.role})
		}, nil},
		{"AssignDepartment", false, func(f *msFixture, o msObjs) error {
			return f.svc.AssignDepartment(ctx, caapp.AssignDepartmentRequest{Subject: f.caller, MembershipID: o.member, DepartmentID: o.deptB})
		}, nil},
		{"RemoveDepartment", false, func(f *msFixture, o msObjs) error {
			return f.svc.RemoveDepartment(ctx, caapp.RemoveDepartmentRequest{Subject: f.caller, MembershipID: o.member, DepartmentID: o.dept})
		}, nil},
		{"AssignTitle", false, func(f *msFixture, o msObjs) error {
			return f.svc.AssignTitle(ctx, caapp.AssignTitleRequest{Subject: f.caller, MembershipID: o.member, TitleID: o.titleB})
		}, nil},
		{"RemoveTitle", false, func(f *msFixture, o msObjs) error {
			return f.svc.RemoveTitle(ctx, caapp.RemoveTitleRequest{Subject: f.caller, MembershipID: o.member, TitleID: o.title})
		}, nil},
		{"AddDeptMember", false, func(f *msFixture, o msObjs) error {
			return f.svc.AddDeptMember(ctx, caapp.AddDeptMemberRequest{Subject: f.caller, DepartmentID: o.deptB, MembershipID: o.member})
		}, nil},
		{"RemoveDeptMember", false, func(f *msFixture, o msObjs) error {
			return f.svc.RemoveDeptMember(ctx, caapp.RemoveDeptMemberRequest{Subject: f.caller, DepartmentID: o.dept, MembershipID: o.member})
		}, nil},
		{"AddDirectPermission", true, func(f *msFixture, o msObjs) error {
			return f.svc.AddDirectPermission(ctx, caapp.AddDirectPermissionRequest{Subject: f.caller, MembershipID: o.member, PermissionCode: "template.workflow.override.write"})
		}, nil},
		{"RemoveDirectPermission", true, func(f *msFixture, o msObjs) error {
			return f.svc.RemoveDirectPermission(ctx, caapp.RemoveDirectPermissionRequest{Subject: f.caller, MembershipID: o.member, PermissionCode: msPerm})
		}, nil},
		{"ListDirectPermissions", true, func(f *msFixture, o msObjs) error {
			_, err := f.svc.ListDirectPermissions(ctx, caapp.ListDirectPermissionsRequest{Subject: f.caller, MembershipID: o.member})
			return err
		}, nil},
		{"AddTeamMember", true, func(f *msFixture, o msObjs) error {
			return f.svc.AddTeamMember(ctx, caapp.AddTeamMemberRequest{Subject: f.caller, TeamID: o.teamB, DepartmentID: o.dept, MembershipID: o.member})
		}, nil},
		{"RemoveTeamMember", true, func(f *msFixture, o msObjs) error {
			return f.svc.RemoveTeamMember(ctx, caapp.RemoveTeamMemberRequest{Subject: f.caller, TeamID: o.team, MembershipID: o.member})
		}, nil},
		{"RemoveTitleMember", true, func(f *msFixture, o msObjs) error {
			return f.svc.RemoveTitleMember(ctx, caapp.RemoveTitleMemberRequest{Subject: f.caller, TitleID: o.title, MembershipID: o.member})
		}, nil},
		{"UpdateMembershipOrgAssignments", false, func(f *msFixture, o msObjs) error {
			return f.svc.UpdateMembershipOrgAssignments(ctx, caapp.UpdateMembershipOrgRequest{
				Subject: f.caller, MembershipID: o.member, DepartmentIDs: []string{o.deptB}, TitleIDs: []string{o.titleB},
			})
		}, nil},
	}
}

func requireMembershipNotFound(t *testing.T, name string, err error) {
	t.Helper()
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusNotFound || he.Code != perr.CodeMembershipNotFound {
		t.Errorf("%s: expected 404 %s, got %v", name, perr.CodeMembershipNotFound, err)
	}
}

var (
	msPersonaTenantAdmin = []string{"rbac.manage", "admin.membership.invite", "admin.membership.update", "admin.membership.delete"}
	// Invite-only caller: no rbac.manage, reaches the "company" scope through the legacy invite permission.
	msPersonaInviteOnly = []string{"admin.membership.invite"}
)

// A membership of another company must be rejected with 404 and left untouched.
func TestMembershipScope_ForeignMembership_NotFoundAndUntouched(t *testing.T) {
	personas := map[string][]string{"tenantAdmin": msPersonaTenantAdmin, "inviteOnly": msPersonaInviteOnly}
	for pname, perms := range personas {
		for _, c := range msCases() {
			if c.needsRBAC && pname == "inviteOnly" {
				continue
			}
			t.Run(pname+"/"+c.name, func(t *testing.T) {
				f := newMSFixture(t, perms)
				if c.prep != nil {
					c.prep(f, f.foreign)
				}
				before := f.snapshot(t, f.foreign)
				err := c.call(f, f.foreign)
				requireMembershipNotFound(t, c.name, err)
				if after := f.snapshot(t, f.foreign); after != before {
					t.Errorf("%s changed the membership of another company:\n before: %s\n after:  %s", c.name, before, after)
				}
			})
		}
	}
}

// The same operations on a membership of the caller's own company keep working.
func TestMembershipScope_OwnMembership_StillWorks(t *testing.T) {
	for _, c := range msCases() {
		t.Run(c.name, func(t *testing.T) {
			f := newMSFixture(t, msPersonaTenantAdmin)
			if c.prep != nil {
				c.prep(f, f.own)
			}
			before := f.snapshot(t, f.own)
			if err := c.call(f, f.own); err != nil {
				t.Fatalf("%s on own company: %v", c.name, err)
			}
			if after := f.snapshot(t, f.own); after == before && c.name != "ListDirectPermissions" {
				t.Errorf("%s did not change the membership of the own company", c.name)
			}
		})
	}
}

func TestMembershipScope_NotFoundDoesNotRevealExistence(t *testing.T) {
	f := newMSFixture(t, msPersonaTenantAdmin)
	foreign := f.svc.DeleteMembership(context.Background(), caapp.DeleteMembershipRequest{Subject: f.caller, MembershipID: f.foreign.member})
	missing := f.svc.DeleteMembership(context.Background(), caapp.DeleteMembershipRequest{Subject: f.caller, MembershipID: "m_does_not_exist"})
	fe, _ := perr.AsHTTPError(foreign)
	me, _ := perr.AsHTTPError(missing)
	if fe == nil || me == nil || fe.HTTPStatus != me.HTTPStatus || fe.Code != me.Code || !strings.EqualFold(fe.Message, me.Message) {
		t.Fatalf("a membership of another company must be indistinguishable from a missing one: %v vs %v", foreign, missing)
	}
}

// Methods that are not part of the table because their own-company flow needs extra fixture
// (primary admin, role lookups). Only the cross-company side is asserted.
func TestMembershipScope_OtherEntryPoints_ForeignMembership(t *testing.T) {
	ctx := context.Background()
	f := newMSFixture(t, msPersonaTenantAdmin)
	f.repo.SeedRole(caapp.RoleListItem{RoleID: "role_primary_x", RoleCode: "user_thuong", RoleName: "user_thuong", Status: "active", Scope: "company", RoleType: caapp.RoleTypeTenantDefault})
	// TransferOwnership only reaches the target check for the primary admin.
	if err := f.repo.SetMembershipPrimaryAdmin(ctx, f.caller.MembershipID); err != nil {
		t.Fatal(err)
	}
	before := f.snapshot(t, f.foreign)
	cases := map[string]error{
		"ReplaceMembershipPrimaryRole": f.svc.ReplaceMembershipPrimaryRole(ctx, caapp.ReplaceMembershipPrimaryRoleRequest{Subject: f.caller, MembershipID: f.foreign.member, RoleID: "role_primary_x"}),
		"RevokeCompanyAdmin":           f.svc.RevokeCompanyAdmin(ctx, caapp.RevokeCompanyAdminRequest{Subject: f.caller, MembershipID: f.foreign.member}),
		"AssignCompanyAdmin":           f.svc.AssignCompanyAdmin(ctx, caapp.AssignCompanyAdminRequest{Subject: f.caller, MembershipID: f.foreign.member}),
		"TransferOwnership":            f.svc.TransferOwnership(ctx, caapp.TransferOwnershipRequest{Subject: f.caller, TargetMembershipID: f.foreign.member}),
	}
	for name, err := range cases {
		requireMembershipNotFound(t, name, err)
	}
	if after := f.snapshot(t, f.foreign); after != before {
		t.Errorf("membership of another company changed:\n before: %s\n after:  %s", before, after)
	}
}

// Submitting a direct-permission removal for a membership of another company is refused up front,
// before any approval request is queued.
func TestMembershipScope_ConfigApproval_DirectPermRemove_ForeignMembership(t *testing.T) {
	f := newMSFixture(t, msPersonaTenantAdmin)
	_, err := f.svc.SubmitConfigApproval(context.Background(), caapp.SubmitConfigApprovalRequest{
		Subject:    f.caller,
		ChangeType: configversion.ChangeTypeRBACDirectPermRemove,
		Proposed:   map[string]any{"membership_id": f.foreign.member, "permission_code": msPerm},
	})
	requireMembershipNotFound(t, "SubmitConfigApproval", err)
}

// Primary-role replacement and user creation with a role must not hand out a role that carries
// platform-tier permissions either (same rule as AssignRole), unless the caller is a platform operator.
func TestMembershipScope_PlatformCapableRole_PrimaryRoleAndCreateUser(t *testing.T) {
	ctx := context.Background()
	f := newMSFixture(t, msPersonaTenantAdmin)
	f.repo.SeedRole(caapp.RoleListItem{RoleID: "role_platform_like", RoleCode: "ops_like", RoleName: "ops_like", Status: "active", Scope: "company", RoleType: caapp.RoleTypeTenantCustom})
	if err := f.repo.AddRolePermission(ctx, "role_platform_like", "platform.cms.view"); err != nil {
		t.Fatal(err)
	}

	err := f.svc.ReplaceMembershipPrimaryRole(ctx, caapp.ReplaceMembershipPrimaryRoleRequest{Subject: f.caller, MembershipID: f.own.member, RoleID: "role_platform_like"})
	if he, ok := perr.AsHTTPError(err); !ok || he.HTTPStatus != http.StatusForbidden {
		t.Errorf("ReplaceMembershipPrimaryRole: expected 403, got %v", err)
	}
	if roles, _ := f.repo.ListMembershipRoles(ctx, f.own.member); len(roles) != 1 || roles[0].RoleID != f.own.role {
		t.Errorf("primary role must be unchanged, got %+v", roles)
	}

	_, err = f.svc.CreateUser(ctx, caapp.CreateUserRequest{
		Subject: f.caller, LoginID: "platform.like@example.com", Password: "StrongPass123!", FullName: "Probe",
		CompanyID: msOwn, RoleID: "role_platform_like",
	})
	if he, ok := perr.AsHTTPError(err); !ok || he.HTTPStatus != http.StatusForbidden {
		t.Errorf("CreateUser with a platform-capable role: expected 403, got %v", err)
	}

	// An ordinary role is still accepted.
	if _, err := f.svc.CreateUser(ctx, caapp.CreateUserRequest{
		Subject: f.caller, LoginID: "ordinary@example.com", Password: "StrongPass123!", FullName: "Ordinary",
		CompanyID: msOwn, RoleID: f.own.roleB,
	}); err != nil {
		t.Errorf("CreateUser with an ordinary role: %v", err)
	}
}
