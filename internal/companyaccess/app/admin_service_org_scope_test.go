package app_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// C5 (risk review 2026-10-09): teams, departments and titles of another company answer 404 and are
// left untouched; ids of other companies must not be distinguishable by a 409 "has members".

func requireNotFoundStatus(t *testing.T, name string, err error) {
	t.Helper()
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusNotFound {
		t.Errorf("%s: expected 404, got %v", name, err)
	}
}

func TestOrgScope_DeleteTeam(t *testing.T) {
	ctx := context.Background()
	f := newMSFixture(t, msPersonaTenantAdmin)

	err := f.svc.DeleteTeam(ctx, caapp.DeleteTeamRequest{Subject: f.caller, TeamID: f.foreign.team})
	requireNotFoundStatus(t, "DeleteTeam(foreign)", err)
	if !f.repo.TeamExists(f.foreign.team) {
		t.Error("team of another company must still exist")
	}
	if got := f.repo.TeamMemberIDs(f.foreign.team); len(got) != 1 || got[0] != f.foreign.member {
		t.Errorf("members of a team of another company must be untouched, got %v", got)
	}

	// Own company: the team and its members are removed.
	if err := f.svc.DeleteTeam(ctx, caapp.DeleteTeamRequest{Subject: f.caller, TeamID: f.own.team}); err != nil {
		t.Fatalf("DeleteTeam(own): %v", err)
	}
	if f.repo.TeamExists(f.own.team) || len(f.repo.TeamMemberIDs(f.own.team)) != 0 {
		t.Error("own team and its members must be removed")
	}
}

func TestOrgScope_CreateTeam(t *testing.T) {
	ctx := context.Background()
	f := newMSFixture(t, msPersonaTenantAdmin)

	_, err := f.svc.CreateTeam(ctx, caapp.CreateTeamRequest{Subject: f.caller, DepartmentID: f.foreign.dept, Name: "Probe"})
	requireNotFoundStatus(t, "CreateTeam(foreign department)", err)
	if teams, _ := f.repo.ListDepartmentTeams(ctx, msForeign, f.foreign.dept); len(teams) != 2 {
		t.Errorf("no team may be created in a department of another company, got %d teams", len(teams))
	}

	if _, err := f.svc.CreateTeam(ctx, caapp.CreateTeamRequest{Subject: f.caller, DepartmentID: f.own.dept, Name: "Mine"}); err != nil {
		t.Fatalf("CreateTeam(own department): %v", err)
	}
}

func TestOrgScope_TeamMembers_ForeignTeam(t *testing.T) {
	ctx := context.Background()
	f := newMSFixture(t, msPersonaTenantAdmin)
	before := f.repo.TeamMemberIDs(f.foreign.teamB)

	// An own member must not be added to a team of another company.
	err := f.svc.AddTeamMember(ctx, caapp.AddTeamMemberRequest{
		Subject: f.caller, TeamID: f.foreign.teamB, DepartmentID: f.own.dept, MembershipID: f.own.member,
	})
	requireNotFoundStatus(t, "AddTeamMember(foreign team)", err)
	if got := f.repo.TeamMemberIDs(f.foreign.teamB); len(got) != len(before) {
		t.Errorf("team of another company changed: %v -> %v", before, got)
	}

	// A stray row of an own member in a team of another company must not be removable either.
	if err := f.repo.AddTeamMember(ctx, msForeign, f.foreign.teamB, f.own.member); err != nil {
		t.Fatal(err)
	}
	err = f.svc.RemoveTeamMember(ctx, caapp.RemoveTeamMemberRequest{Subject: f.caller, TeamID: f.foreign.teamB, MembershipID: f.own.member})
	requireNotFoundStatus(t, "RemoveTeamMember(foreign team)", err)
	if got := f.repo.TeamMemberIDs(f.foreign.teamB); len(got) != 1 {
		t.Errorf("RemoveTeamMember touched a team of another company: %v", got)
	}

	// Own team: add then remove still works.
	if err := f.svc.AddTeamMember(ctx, caapp.AddTeamMemberRequest{Subject: f.caller, TeamID: f.own.teamB, DepartmentID: f.own.dept, MembershipID: f.own.member}); err != nil {
		t.Fatalf("AddTeamMember(own): %v", err)
	}
	if err := f.svc.RemoveTeamMember(ctx, caapp.RemoveTeamMemberRequest{Subject: f.caller, TeamID: f.own.teamB, MembershipID: f.own.member}); err != nil {
		t.Fatalf("RemoveTeamMember(own): %v", err)
	}
}

func TestOrgScope_DeleteDepartmentAndTitle(t *testing.T) {
	ctx := context.Background()
	f := newMSFixture(t, msPersonaTenantAdmin)

	// Another company's department/title that has members: 404, not the 409 "has members".
	requireNotFoundStatus(t, "DeleteDepartment(foreign)", f.svc.DeleteDepartment(ctx, caapp.DeleteDepartmentRequest{Subject: f.caller, DepartmentID: f.foreign.dept}))
	requireNotFoundStatus(t, "DeleteTitle(foreign)", f.svc.DeleteTitle(ctx, caapp.DeleteTitleRequest{Subject: f.caller, TitleID: f.foreign.title}))
	// Another company's empty department/title: 404 as well and still active.
	requireNotFoundStatus(t, "DeleteDepartment(foreign, empty)", f.svc.DeleteDepartment(ctx, caapp.DeleteDepartmentRequest{Subject: f.caller, DepartmentID: f.foreign.deptB}))
	requireNotFoundStatus(t, "DeleteTitle(foreign, empty)", f.svc.DeleteTitle(ctx, caapp.DeleteTitleRequest{Subject: f.caller, TitleID: f.foreign.titleB}))

	// Own company keeps its behaviour: 409 while it has members, success when empty.
	for name, err := range map[string]error{
		"DeleteDepartment(own, with members)": f.svc.DeleteDepartment(ctx, caapp.DeleteDepartmentRequest{Subject: f.caller, DepartmentID: f.own.dept}),
		"DeleteTitle(own, with members)":      f.svc.DeleteTitle(ctx, caapp.DeleteTitleRequest{Subject: f.caller, TitleID: f.own.title}),
	} {
		if he, ok := perr.AsHTTPError(err); !ok || he.HTTPStatus != http.StatusConflict {
			t.Errorf("%s: expected 409, got %v", name, err)
		}
	}
	if err := f.svc.DeleteDepartment(ctx, caapp.DeleteDepartmentRequest{Subject: f.caller, DepartmentID: f.own.deptB}); err != nil {
		t.Fatalf("DeleteDepartment(own, empty): %v", err)
	}
	if err := f.svc.DeleteTitle(ctx, caapp.DeleteTitleRequest{Subject: f.caller, TitleID: f.own.titleB}); err != nil {
		t.Fatalf("DeleteTitle(own, empty): %v", err)
	}
}

// Creating a team in an inactive department of the own company keeps working as before: the check
// is about the company, not about the department status.
func TestOrgScope_CreateTeam_InactiveOwnDepartment_StillAllowed(t *testing.T) {
	ctx := context.Background()
	f := newMSFixture(t, msPersonaTenantAdmin)
	f.repo.SeedDepartmentForCompany(msOwn, caapp.DepartmentView{DepartmentID: "d_own_inactive", DepartmentName: "Inactive", Status: "inactive"})
	if _, err := f.svc.CreateTeam(ctx, caapp.CreateTeamRequest{Subject: f.caller, DepartmentID: "d_own_inactive", Name: "Team"}); err != nil {
		t.Fatalf("CreateTeam(inactive own department): %v", err)
	}
}

// A storage failure while checking the department is a server error, never a 404.
type deptLookupFailsRepo struct{ *cainmem.AdminRepository }

var errDeptLookup = errors.New("storage unavailable")

func (deptLookupFailsRepo) ListCompanyDepartments(context.Context, string) ([]caapp.DepartmentView, error) {
	return nil, errDeptLookup
}

func (deptLookupFailsRepo) DepartmentBelongsToCompany(context.Context, string, string) (bool, error) {
	return false, errDeptLookup
}

func TestOrgScope_CreateTeam_StorageErrorIsNotNotFound(t *testing.T) {
	repo := deptLookupFailsRepo{cainmem.NewAdminRepository()}
	svc := caapp.NewAdminService(repo, fakeAuthService{decision: authapp.DecisionAllow, permissions: msPersonaTenantAdmin}, fixedIDGen("id"))
	_, err := svc.CreateTeam(context.Background(), caapp.CreateTeamRequest{
		Subject: caapp.AdminSubject{UserID: "u", MembershipID: "m", CompanyID: msOwn}, DepartmentID: "d", Name: "Team",
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	if he, ok := perr.AsHTTPError(err); ok && he.HTTPStatus == http.StatusNotFound {
		t.Fatalf("a storage failure must not be reported as 404: %v", err)
	}
}

// Patching (also with an empty body, which only reads the object back) a department, title or team
// of another company answers 404 and does not return the object.
func TestOrgScope_PatchForeignObjects_NotFound(t *testing.T) {
	ctx := context.Background()
	f := newMSFixture(t, msPersonaTenantAdmin)
	name := "renamed"

	_, err := f.svc.UpdateDepartment(ctx, caapp.UpdateDepartmentRequest{Subject: f.caller, DepartmentID: f.foreign.dept})
	requireNotFoundStatus(t, "UpdateDepartment(foreign, empty body)", err)
	_, err = f.svc.UpdateDepartment(ctx, caapp.UpdateDepartmentRequest{Subject: f.caller, DepartmentID: f.foreign.dept, Name: &name})
	requireNotFoundStatus(t, "UpdateDepartment(foreign, rename)", err)
	_, err = f.svc.UpdateTitle(ctx, caapp.UpdateTitleRequest{Subject: f.caller, TitleID: f.foreign.title})
	requireNotFoundStatus(t, "UpdateTitle(foreign, empty body)", err)
	_, err = f.svc.UpdateTitle(ctx, caapp.UpdateTitleRequest{Subject: f.caller, TitleID: f.foreign.title, Name: &name})
	requireNotFoundStatus(t, "UpdateTitle(foreign, rename)", err)
	_, err = f.svc.UpdateTeam(ctx, caapp.UpdateTeamRequest{Subject: f.caller, TeamID: f.foreign.team})
	requireNotFoundStatus(t, "UpdateTeam(foreign, empty body)", err)
	_, err = f.svc.UpdateTeam(ctx, caapp.UpdateTeamRequest{Subject: f.caller, TeamID: f.foreign.team, Name: &name})
	requireNotFoundStatus(t, "UpdateTeam(foreign, rename)", err)

	// Own company keeps working.
	if _, err := f.svc.UpdateDepartment(ctx, caapp.UpdateDepartmentRequest{Subject: f.caller, DepartmentID: f.own.dept}); err != nil {
		t.Errorf("UpdateDepartment(own, empty body): %v", err)
	}
	if _, err := f.svc.UpdateTitle(ctx, caapp.UpdateTitleRequest{Subject: f.caller, TitleID: f.own.title}); err != nil {
		t.Errorf("UpdateTitle(own, empty body): %v", err)
	}
	if _, err := f.svc.UpdateTeam(ctx, caapp.UpdateTeamRequest{Subject: f.caller, TeamID: f.own.team, Name: &name}); err != nil {
		t.Errorf("UpdateTeam(own): %v", err)
	}
}
