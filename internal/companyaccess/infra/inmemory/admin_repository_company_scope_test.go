package inmemory_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// C5 (risk review 2026-10-09): the repo itself refuses to update or delete a membership that does
// not belong to the company it is called for, so a missing service check cannot touch other
// tenants' data.

func seedMember(t *testing.T, repo *cainmem.AdminRepository, id, company string) {
	t.Helper()
	if _, err := repo.CreateUser(context.Background(), caapp.UserView{
		UserID: "u_" + id, LoginID: id + "@example.com", FullName: id, AccountStatus: "active",
	}, "hash", caapp.CreateUserOptions{MembershipID: id, CompanyID: company, MembershipStatus: "active"}); err != nil {
		t.Fatalf("seed %s: %v", id, err)
	}
}

func requireNotFound(t *testing.T, name string, err error) {
	t.Helper()
	var he *perr.HTTPError
	if !errors.As(err, &he) || he.HTTPStatus != http.StatusNotFound || he.Code != perr.CodeMembershipNotFound {
		t.Errorf("%s: expected 404 %s, got %v", name, perr.CodeMembershipNotFound, err)
	}
}

func TestUpdateMembershipStatus_OtherCompany_NotFoundAndUntouched(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	seedMember(t, repo, "m_a", "c_001")

	_, err := repo.UpdateMembershipStatus(context.Background(), "c_002", "m_a", "inactive")
	requireNotFound(t, "UpdateMembershipStatus(other company)", err)
	if m, _ := repo.GetMembershipByID(context.Background(), "m_a"); m == nil || m.Status != "active" {
		t.Fatalf("membership of another company must stay active, got %+v", m)
	}

	out, err := repo.UpdateMembershipStatus(context.Background(), "c_001", "m_a", "inactive")
	if err != nil || out.Status != "inactive" {
		t.Fatalf("own company update: out=%+v err=%v", out, err)
	}
}

func TestDeleteMembership_OtherCompany_NotFoundAndUntouched(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	seedMember(t, repo, "m_a", "c_001")

	requireNotFound(t, "DeleteMembership(other company)", repo.DeleteMembership(context.Background(), "c_002", "m_a"))
	if _, err := repo.GetMembershipByID(context.Background(), "m_a"); err != nil {
		t.Fatalf("membership of another company must still exist: %v", err)
	}

	if err := repo.DeleteMembership(context.Background(), "c_001", "m_a"); err != nil {
		t.Fatalf("own company delete: %v", err)
	}
	if _, err := repo.GetMembershipByID(context.Background(), "m_a"); err == nil {
		t.Fatal("own membership must be deleted")
	}
}
