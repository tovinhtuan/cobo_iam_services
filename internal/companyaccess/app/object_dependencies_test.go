package app_test

import (
	"context"
	"net/http"
	"testing"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	"github.com/cobo/cobo_iam_services/internal/companyaccess/dependency"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func newDependencyTestService(repo *cainmem.AdminRepository, perms []string) caapp.AdminService {
	return caapp.NewAdminService(
		repo,
		healthAuthService{permissions: perms},
		fixedIDGen("id-1"),
		caapp.WithConflictSnapshotReader(cainmem.NewConflictSnapshotReader(repo)),
		caapp.WithDependencyReader(cainmem.NewDependencyReader(repo)),
	)
}

func TestGetObjectDependenciesDepartment(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	repo.SeedDepartment(caapp.DepartmentView{DepartmentID: "d1", Name: "Ops", Status: "active"})
	repo.SeedDepartmentMember("co-1", "d1", "m1")
	svc := newDependencyTestService(repo, []string{"rbac.manage"})
	sub := caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "co-1"}
	out, err := svc.GetObjectDependencies(context.Background(), caapp.GetObjectDependenciesRequest{
		Subject: sub, ObjectType: dependency.ObjectTypeDepartment, ObjectID: "d1",
	})
	if err != nil {
		t.Fatalf("GetObjectDependencies: %v", err)
	}
	if out.ObjectID != "d1" || out.CompanyID != "co-1" {
		t.Fatalf("unexpected ids: %+v", out)
	}
	if out.Source != dependency.Source {
		t.Fatalf("source %q", out.Source)
	}
	if out.TotalReferences < 1 {
		t.Fatal("expected references")
	}
}

func TestGetObjectDependenciesForbidden(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	svc := newDependencyTestService(repo, []string{"disclosure.view"})
	sub := caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "co-1"}
	_, err := svc.GetObjectDependencies(context.Background(), caapp.GetObjectDependenciesRequest{
		Subject: sub, ObjectType: dependency.ObjectTypeDepartment, ObjectID: "d1",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusForbidden {
		t.Fatalf("expected 403, got %v", err)
	}
}

func TestGetObjectDependenciesInvalidSampleLimit(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	repo.SeedDepartment(caapp.DepartmentView{DepartmentID: "d1", Name: "Ops", Status: "active"})
	svc := newDependencyTestService(repo, []string{"rbac.manage"})
	sub := caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "co-1"}
	_, err := svc.GetObjectDependencies(context.Background(), caapp.GetObjectDependenciesRequest{
		Subject: sub, ObjectType: dependency.ObjectTypeDepartment, ObjectID: "d1", SampleLimit: 99,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("expected 400, got %v", err)
	}
}

func TestParseObjectDependenciesQuery(t *testing.T) {
	limit, counts, err := caapp.ParseObjectDependenciesQuery("10", "false")
	if err != nil || limit != 10 || counts {
		t.Fatalf("got limit=%d counts=%v err=%v", limit, counts, err)
	}
	_, _, err = caapp.ParseObjectDependenciesQuery("abc", "")
	if err == nil {
		t.Fatal("expected parse error")
	}
}
