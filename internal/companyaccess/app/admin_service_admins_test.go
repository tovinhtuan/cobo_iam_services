package app_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// d1Repo wraps the inmemory repo, overriding team-related stubs so unit
// tests can control CountTeamsInDepartment and MemberBelongsToDepartment.
type d1Repo struct {
	*cainmem.AdminRepository
	teamCount    int
	memberInDept bool
}

func (r *d1Repo) CountTeamsInDepartment(_ context.Context, _ string) (int, error) {
	return r.teamCount, nil
}

func (r *d1Repo) MemberBelongsToDepartment(_ context.Context, _, _ string) (bool, error) {
	return r.memberInDept, nil
}

func seedMem(repo *cainmem.AdminRepository, membershipID, userID, companyID string) {
	_, _ = repo.CreateUser(context.Background(), caapp.UserView{
		UserID:  userID,
		LoginID: userID + "@test.com",
	}, "hash", caapp.CreateUserOptions{
		CompanyID:    companyID,
		MembershipID: membershipID,
	})
}

func allowedSvc(repo caapp.AdminRepository) caapp.AdminService {
	return caapp.NewAdminService(
		repo,
		fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"rbac.manage"}},
		fixedIDGen("test-id"),
	)
}

// ─── C1: AssignCompanyAdmin ───────────────────────────────────────────────────

func TestAssignCompanyAdmin_OK(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	seedMem(repo, "m-target", "u-target", "c-1")
	svc := allowedSvc(repo)

	err := svc.AssignCompanyAdmin(context.Background(), caapp.AssignCompanyAdminRequest{
		Subject:      caapp.AdminSubject{UserID: "u-caller", MembershipID: "m-caller", CompanyID: "c-1"},
		MembershipID: "m-target",
	})
	if err != nil {
		t.Fatalf("AssignCompanyAdmin err=%v", err)
	}
}

func TestAssignCompanyAdmin_MembershipNotInCompany(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	// "m-ghost" does not exist
	svc := allowedSvc(repo)

	err := svc.AssignCompanyAdmin(context.Background(), caapp.AssignCompanyAdminRequest{
		Subject:      caapp.AdminSubject{UserID: "u-caller", MembershipID: "m-caller", CompanyID: "c-1"},
		MembershipID: "m-ghost",
	})
	if err == nil {
		t.Fatal("expected error for membership not in company")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok {
		t.Fatalf("expected HTTPError got %T", err)
	}
	if he.HTTPStatus != http.StatusNotFound {
		t.Fatalf("want 404, got %d: %v", he.HTTPStatus, err)
	}
}

func TestAssignCompanyAdmin_LimitReached(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	svc := allowedSvc(repo)
	// Seed 5 admins by seeding memberships and assigning via service
	for i := 0; i < 5; i++ {
		mid := fmt.Sprintf("m-%d", i)
		uid := fmt.Sprintf("u-%d", i)
		seedMem(repo, mid, uid, "c-1")
		if err := svc.AssignCompanyAdmin(context.Background(), caapp.AssignCompanyAdminRequest{
			Subject:      caapp.AdminSubject{UserID: "u-root", MembershipID: "m-root", CompanyID: "c-1"},
			MembershipID: mid,
		}); err != nil {
			t.Fatalf("seed admin %d: %v", i, err)
		}
	}
	// 6th should be blocked
	seedMem(repo, "m-6th", "u-6th", "c-1")
	err := svc.AssignCompanyAdmin(context.Background(), caapp.AssignCompanyAdminRequest{
		Subject:      caapp.AdminSubject{UserID: "u-root", MembershipID: "m-root", CompanyID: "c-1"},
		MembershipID: "m-6th",
	})
	if err == nil {
		t.Fatal("expected ADMIN_LIMIT_REACHED")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok {
		t.Fatalf("expected HTTPError got %T", err)
	}
	if he.HTTPStatus != http.StatusConflict {
		t.Fatalf("want 409, got %d: %v", he.HTTPStatus, err)
	}
}

// ─── C1/C2: RevokeCompanyAdmin ────────────────────────────────────────────────

func TestRevokeCompanyAdmin_OK(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	seedMem(repo, "m-target", "u-target", "c-1")
	svc := allowedSvc(repo)

	err := svc.RevokeCompanyAdmin(context.Background(), caapp.RevokeCompanyAdminRequest{
		Subject:      caapp.AdminSubject{UserID: "u-caller", MembershipID: "m-caller", CompanyID: "c-1"},
		MembershipID: "m-target",
	})
	if err != nil {
		t.Fatalf("RevokeCompanyAdmin err=%v", err)
	}
}

func TestRevokeCompanyAdmin_CannotRevokePrimaryAdmin(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	seedMem(repo, "m-primary", "u-primary", "c-1")
	if err := repo.SetMembershipPrimaryAdmin(context.Background(), "m-primary"); err != nil {
		t.Fatalf("SetMembershipPrimaryAdmin: %v", err)
	}
	svc := allowedSvc(repo)

	err := svc.RevokeCompanyAdmin(context.Background(), caapp.RevokeCompanyAdminRequest{
		Subject:      caapp.AdminSubject{UserID: "u-caller", MembershipID: "m-caller", CompanyID: "c-1"},
		MembershipID: "m-primary",
	})
	if err == nil {
		t.Fatal("expected CANNOT_REVOKE_PRIMARY_ADMIN")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusConflict {
		t.Fatalf("want 409, got err=%v", err)
	}
}

// ─── C2: TransferOwnership ────────────────────────────────────────────────────

func TestTransferOwnership_OK(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	seedMem(repo, "m-primary", "u-primary", "c-1")
	seedMem(repo, "m-target", "u-target", "c-1")
	if err := repo.SetMembershipPrimaryAdmin(context.Background(), "m-primary"); err != nil {
		t.Fatalf("SetMembershipPrimaryAdmin: %v", err)
	}
	svc := allowedSvc(repo)

	err := svc.TransferOwnership(context.Background(), caapp.TransferOwnershipRequest{
		Subject:            caapp.AdminSubject{UserID: "u-primary", MembershipID: "m-primary", CompanyID: "c-1"},
		TargetMembershipID: "m-target",
	})
	if err != nil {
		t.Fatalf("TransferOwnership err=%v", err)
	}

	caller, err := repo.GetMembershipByID(context.Background(), "m-primary")
	if err != nil {
		t.Fatalf("get caller: %v", err)
	}
	if caller.IsPrimaryAdmin {
		t.Error("caller should no longer be primary admin")
	}

	target, err := repo.GetMembershipByID(context.Background(), "m-target")
	if err != nil {
		t.Fatalf("get target: %v", err)
	}
	if !target.IsPrimaryAdmin {
		t.Error("target should now be primary admin")
	}
}

func TestTransferOwnership_CallerNotPrimary(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	seedMem(repo, "m-caller", "u-caller", "c-1")
	seedMem(repo, "m-target", "u-target", "c-1")
	// m-caller is NOT primary admin (default)
	svc := allowedSvc(repo)

	err := svc.TransferOwnership(context.Background(), caapp.TransferOwnershipRequest{
		Subject:            caapp.AdminSubject{UserID: "u-caller", MembershipID: "m-caller", CompanyID: "c-1"},
		TargetMembershipID: "m-target",
	})
	if err == nil {
		t.Fatal("expected error: only primary admin can transfer ownership")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusForbidden {
		t.Fatalf("want 403, got err=%v", err)
	}
}

// ─── C3: Primary admin self-protection ────────────────────────────────────────

func TestDeleteMembership_CannotDeletePrimaryAdmin(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	seedMem(repo, "m-primary", "u-primary", "c-1")
	if err := repo.SetMembershipPrimaryAdmin(context.Background(), "m-primary"); err != nil {
		t.Fatalf("SetMembershipPrimaryAdmin: %v", err)
	}
	svc := allowedSvc(repo)

	err := svc.DeleteMembership(context.Background(), caapp.DeleteMembershipRequest{
		Subject:      caapp.AdminSubject{UserID: "u-caller", MembershipID: "m-caller", CompanyID: "c-1"},
		MembershipID: "m-primary",
	})
	if err == nil {
		t.Fatal("expected CANNOT_DELETE_PRIMARY_ADMIN")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusConflict {
		t.Fatalf("want 409, got err=%v", err)
	}
}

func TestUpdateMembership_CannotDeactivatePrimaryAdmin(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	seedMem(repo, "m-primary", "u-primary", "c-1")
	if err := repo.SetMembershipPrimaryAdmin(context.Background(), "m-primary"); err != nil {
		t.Fatalf("SetMembershipPrimaryAdmin: %v", err)
	}
	svc := allowedSvc(repo)

	_, err := svc.UpdateMembership(context.Background(), caapp.UpdateMembershipRequest{
		Subject:      caapp.AdminSubject{UserID: "u-caller", MembershipID: "m-caller", CompanyID: "c-1"},
		MembershipID: "m-primary",
		Status:       "inactive",
	})
	if err == nil {
		t.Fatal("expected CANNOT_DEACTIVATE_PRIMARY_ADMIN")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusConflict {
		t.Fatalf("want 409, got err=%v", err)
	}
}

// ─── D1: Team CRUD ────────────────────────────────────────────────────────────

func TestCreateTeam_OK(t *testing.T) {
	repo := &d1Repo{AdminRepository: cainmem.NewAdminRepository(), teamCount: 0, memberInDept: true}
	svc := allowedSvc(repo)

	out, err := svc.CreateTeam(context.Background(), caapp.CreateTeamRequest{
		Subject:      caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "c-1"},
		DepartmentID: "dept-1",
		Name:         "Team Alpha",
	})
	if err != nil {
		t.Fatalf("CreateTeam err=%v", err)
	}
	if out.Name != "Team Alpha" {
		t.Fatalf("Name=%q want Team Alpha", out.Name)
	}
}

func TestCreateTeam_LimitReached(t *testing.T) {
	repo := &d1Repo{AdminRepository: cainmem.NewAdminRepository(), teamCount: 5, memberInDept: true}
	svc := allowedSvc(repo)

	_, err := svc.CreateTeam(context.Background(), caapp.CreateTeamRequest{
		Subject:      caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "c-1"},
		DepartmentID: "dept-1",
		Name:         "Over Limit Team",
	})
	if err == nil {
		t.Fatal("expected TEAM_LIMIT_REACHED")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusConflict {
		t.Fatalf("want 409, got err=%v", err)
	}
}

func TestAddTeamMember_NotInDepartment(t *testing.T) {
	repo := &d1Repo{AdminRepository: cainmem.NewAdminRepository(), teamCount: 0, memberInDept: false}
	svc := allowedSvc(repo)

	err := svc.AddTeamMember(context.Background(), caapp.AddTeamMemberRequest{
		Subject:      caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "c-1"},
		TeamID:       "team-1",
		DepartmentID: "dept-1",
		MembershipID: "m-other",
	})
	if err == nil {
		t.Fatal("expected MEMBER_NOT_IN_DEPARTMENT")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusConflict {
		t.Fatalf("want 409, got err=%v", err)
	}
}

func TestDeleteTeam_OK(t *testing.T) {
	repo := &d1Repo{AdminRepository: cainmem.NewAdminRepository(), teamCount: 0, memberInDept: true}
	svc := allowedSvc(repo)

	err := svc.DeleteTeam(context.Background(), caapp.DeleteTeamRequest{
		Subject: caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "c-1"},
		TeamID:  "team-1",
	})
	if err != nil {
		t.Fatalf("DeleteTeam err=%v", err)
	}
}

func TestListDepartmentTeams_UsesEffectivePermissionCheck(t *testing.T) {
	repo := &d1Repo{AdminRepository: cainmem.NewAdminRepository(), teamCount: 0, memberInDept: true}
	svc := caapp.NewAdminService(
		repo,
		fakeAuthService{
			decision:    authapp.DecisionDeny,
			permissions: []string{"rbac.manage"},
		},
		fixedIDGen("test-id"),
	)

	_, err := svc.ListDepartmentTeams(context.Background(), caapp.ListDepartmentTeamsRequest{
		Subject:      caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "c-1"},
		DepartmentID: "dept-1",
	})
	if err != nil {
		t.Fatalf("ListDepartmentTeams should allow with rbac.manage in effective access, err=%v", err)
	}
}

func TestCreateTeam_DeniedWithoutRbacManage(t *testing.T) {
	repo := &d1Repo{AdminRepository: cainmem.NewAdminRepository(), teamCount: 0, memberInDept: true}
	svc := caapp.NewAdminService(
		repo,
		fakeAuthService{
			decision:    authapp.DecisionAllow,
			permissions: []string{"dashboard.view"},
		},
		fixedIDGen("test-id"),
	)

	_, err := svc.CreateTeam(context.Background(), caapp.CreateTeamRequest{
		Subject:      caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "c-1"},
		DepartmentID: "dept-1",
		Name:         "Team Beta",
	})
	if err == nil {
		t.Fatal("expected rbac.manage required")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusForbidden {
		t.Fatalf("want 403, got err=%v", err)
	}
}
