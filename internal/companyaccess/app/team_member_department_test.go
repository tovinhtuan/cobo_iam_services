package app_test

import (
	"context"
	"testing"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// ROLE-10 / RP-11: the "member belongs to the department" rule uses the team's own department,
// not the department_id sent in the body.
func newTeamDeptFixture(t *testing.T) (*cainmem.AdminRepository, caapp.AdminService) {
	t.Helper()
	repo := cainmem.NewAdminRepository()
	seedMem(repo, "m-admin", "u-admin", "c-1")
	seedMem(repo, "m-x", "u-x", "c-1")
	repo.SeedDepartmentForCompany("c-1", caapp.DepartmentView{DepartmentID: "dept-x", DepartmentName: "X", Status: "active"})
	repo.SeedDepartmentForCompany("c-1", caapp.DepartmentView{DepartmentID: "dept-y", DepartmentName: "Y", Status: "active"})
	if err := repo.UpsertDepartmentMembership(context.Background(), "m-x", "dept-x", false); err != nil {
		t.Fatal(err)
	}
	repo.SeedTeam("c-1", "dept-y", "team-y", "Team Y")
	return repo, allowedSvc(repo)
}

func adminSub() caapp.AdminSubject {
	return caapp.AdminSubject{UserID: "u-admin", MembershipID: "m-admin", CompanyID: "c-1"}
}

func TestAddTeamMember_BodyDepartmentCannotBypassTeamDepartment(t *testing.T) {
	_, svc := newTeamDeptFixture(t)
	err := svc.AddTeamMember(context.Background(), caapp.AddTeamMemberRequest{
		Subject: adminSub(), TeamID: "team-y", DepartmentID: "dept-x", MembershipID: "m-x",
	})
	requireHTTPCode(t, err, 400, perr.CodeInvalidRequest)
}

func TestAddTeamMember_UsesTeamDepartment(t *testing.T) {
	_, svc := newTeamDeptFixture(t)
	err := svc.AddTeamMember(context.Background(), caapp.AddTeamMemberRequest{
		Subject: adminSub(), TeamID: "team-y", MembershipID: "m-x",
	})
	requireHTTPCode(t, err, 409, perr.CodeStateConflict)
}

func TestAddTeamMember_MemberOfTeamDepartmentAccepted(t *testing.T) {
	repo, svc := newTeamDeptFixture(t)
	if err := repo.UpsertDepartmentMembership(context.Background(), "m-x", "dept-y", false); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddTeamMember(context.Background(), caapp.AddTeamMemberRequest{
		Subject: adminSub(), TeamID: "team-y", DepartmentID: "dept-y", MembershipID: "m-x",
	}); err != nil {
		t.Fatalf("member of the team's department: %v", err)
	}
}
