package dependency_test

import (
	"context"
	"testing"
	"time"

	"github.com/cobo/cobo_iam_services/internal/companyaccess/conflict"
	"github.com/cobo/cobo_iam_services/internal/companyaccess/dependency"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
)

type stubReader struct {
	deptOK bool
	roleOK bool
}

func (s stubReader) DepartmentBelongsToCompany(context.Context, string, string) (bool, error) {
	return s.deptOK, nil
}
func (s stubReader) RoleAccessibleByCompany(context.Context, string, string) (bool, error) {
	return s.roleOK, nil
}
func (s stubReader) ListDepartmentMemberSamples(context.Context, string, string, int) ([]dependency.Sample, int, error) {
	return []dependency.Sample{{"membership_id": "m1", "display_name": "A"}}, 1, nil
}
func (s stubReader) ListRoleMembershipSamples(context.Context, string, string, int) ([]dependency.Sample, int, error) {
	return nil, 0, nil
}
func (s stubReader) ListRolePermissionSamples(context.Context, string, string, int) ([]dependency.Sample, int, error) {
	return nil, 0, nil
}
func (s stubReader) ListWorkflowOverrideStepsForDepartment(context.Context, string, string, int) ([]dependency.Sample, int, bool, error) {
	return nil, 0, false, nil
}

func TestProviderDepartmentDeterministic(t *testing.T) {
	snap := conflict.ConfigurationSnapshot{
		CompanyID: "co-1",
		WorkflowAssigneeRules: []conflict.WorkflowAssigneeRuleRow{{
			RuleID: "r1", RuleCode: "war.test",
			Payload: map[string]any{"department_id": "d1"},
		}},
	}
	_ = snap
	p := dependency.Provider{Reader: stubReader{deptOK: true}}
	out, err := p.Resolve(context.Background(), dependency.Query{
		CompanyID: "co-1", ObjectType: dependency.ObjectTypeDepartment, ObjectID: "d1",
		SampleLimit: 5, IncludeCounts: true, EvaluatedAt: time.Unix(0, 0).UTC(),
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if out.Source != dependency.Source {
		t.Fatalf("source: %q", out.Source)
	}
	if out.TotalReferences < 1 {
		t.Fatal("expected member reference")
	}
}

func TestProviderInvalidObjectType(t *testing.T) {
	p := dependency.Provider{Reader: stubReader{}}
	_, err := p.Resolve(context.Background(), dependency.Query{
		CompanyID: "co-1", ObjectType: "title", ObjectID: "x",
	})
	if err != dependency.ErrInvalidObjectType {
		t.Fatalf("expected invalid object type, got %v", err)
	}
}

func TestInmemoryDepartmentMembers(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	repo.SeedDepartment(caapp.DepartmentView{DepartmentID: "d1", Name: "HR", Status: "active"})
	repo.SeedDepartmentMember("co-1", "d1", "m1")
	reader := cainmem.NewDependencyReader(repo)
	samples, total, err := reader.ListDepartmentMemberSamples(context.Background(), "co-1", "d1", 5)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(samples) != 1 {
		t.Fatalf("samples=%d total=%d", len(samples), total)
	}
}
