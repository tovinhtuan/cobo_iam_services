package app_test

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
)

// --- H17: every change to what a membership may do or see drops the cached effective access ---

type ctxRecordingCache struct {
	companies []string
	ctxErrs   []error
}

func (c *ctxRecordingCache) InvalidateCompany(ctx context.Context, companyID string) error {
	c.companies = append(c.companies, companyID)
	c.ctxErrs = append(c.ctxErrs, ctx.Err())
	return nil
}

func newInvalidationFixture(t *testing.T) (*cainmem.AdminRepository, *ctxRecordingCache, caapp.AdminService, caapp.AdminSubject) {
	t.Helper()
	repo := cainmem.NewAdminRepository()
	seedMem(repo, "m-primary", "u-primary", "c-1")
	seedMem(repo, "m-target", "u-target", "c-1")
	if err := repo.SetMembershipPrimaryAdmin(context.Background(), "m-primary"); err != nil {
		t.Fatal(err)
	}
	cache := &ctxRecordingCache{}
	svc := caapp.NewAdminService(repo, fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"rbac.manage"}},
		fixedIDGen("test-id"), caapp.WithEffectiveAccessCache(cache))
	return repo, cache, svc, caapp.AdminSubject{UserID: "u-primary", MembershipID: "m-primary", CompanyID: "c-1"}
}

func TestAccessChanges_InvalidateEffectiveAccess(t *testing.T) {
	calls := map[string]func(repo *cainmem.AdminRepository, svc caapp.AdminService, sub caapp.AdminSubject) error{
		"UpdateMembership(inactive)": func(_ *cainmem.AdminRepository, svc caapp.AdminService, sub caapp.AdminSubject) error {
			_, err := svc.UpdateMembership(context.Background(), caapp.UpdateMembershipRequest{Subject: sub, MembershipID: "m-target", Status: "inactive"})
			return err
		},
		"DeleteMembership": func(_ *cainmem.AdminRepository, svc caapp.AdminService, sub caapp.AdminSubject) error {
			return svc.DeleteMembership(context.Background(), caapp.DeleteMembershipRequest{Subject: sub, MembershipID: "m-target"})
		},
		"RemoveRole": func(repo *cainmem.AdminRepository, svc caapp.AdminService, sub caapp.AdminSubject) error {
			repo.SeedRoleForCompany(caapp.RoleListItem{RoleID: "r-staff", RoleCode: "staff", RoleName: "Staff", Status: "active"}, "c-1")
			if err := repo.AddRole(context.Background(), "m-target", "r-staff"); err != nil {
				t.Fatal(err)
			}
			return svc.RemoveRole(context.Background(), caapp.RemoveRoleRequest{Subject: sub, MembershipID: "m-target", RoleID: "r-staff"})
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			repo, cache, svc, sub := newInvalidationFixture(t)
			if err := call(repo, svc, sub); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if len(cache.companies) == 0 || cache.companies[len(cache.companies)-1] != "c-1" {
				t.Fatalf("%s must drop the cached effective access of company c-1, got %v", name, cache.companies)
			}
		})
	}
}

func TestAccessChange_FailedMutationDoesNotInvalidate(t *testing.T) {
	_, cache, svc, sub := newInvalidationFixture(t)
	err := svc.DeleteMembership(context.Background(), caapp.DeleteMembershipRequest{Subject: sub, MembershipID: "m-primary"})
	if err == nil {
		t.Fatal("deleting the primary admin must fail")
	}
	if len(cache.companies) != 0 {
		t.Fatalf("a refused change must not invalidate, got %v", cache.companies)
	}
}

// failingUpsertRepo fails the second write of an org reassignment, after the first one committed.
type failingUpsertRepo struct{ *cainmem.AdminRepository }

func (r failingUpsertRepo) UpsertDepartmentMembership(context.Context, string, string, bool) error {
	return errors.New("db unavailable")
}

// PERF-26: a multi-write change that fails after its first write still drops the cache.
func TestAccessChange_PartialFailureAfterWriteStillInvalidates(t *testing.T) {
	repo, _, _, sub := newInvalidationFixture(t)
	repo.SeedDepartmentForCompany("c-1", caapp.DepartmentView{DepartmentID: "d-old", DepartmentName: "Old", Status: "active"})
	repo.SeedDepartmentForCompany("c-1", caapp.DepartmentView{DepartmentID: "d-new", DepartmentName: "New", Status: "active"})
	if err := repo.UpsertDepartmentMembership(context.Background(), "m-target", "d-old", false); err != nil {
		t.Fatal(err)
	}
	cache := &ctxRecordingCache{}
	svc := caapp.NewAdminService(failingUpsertRepo{repo}, fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"rbac.manage"}},
		fixedIDGen("test-id"), caapp.WithEffectiveAccessCache(cache))
	err := svc.UpdateMembershipOrgAssignments(context.Background(), caapp.UpdateMembershipOrgRequest{
		Subject: sub, MembershipID: "m-target", DepartmentIDs: []string{"d-new"},
	})
	if err == nil {
		t.Fatal("expected the second write to fail")
	}
	if ids, _ := repo.ListActiveMembershipDepartmentIDs(context.Background(), "m-target"); len(ids) != 0 {
		t.Fatalf("precondition: the removal of d-old must have committed, departments now %v", ids)
	}
	if len(cache.companies) == 0 {
		t.Fatal("the committed removal must drop the cached effective access")
	}
}

// CACHE-12: the change has committed, so the invalidation must run even when the client
// disconnected and the request context is canceled.
func TestAccessChange_InvalidatesEvenWhenRequestContextIsCanceled(t *testing.T) {
	_, cache, svc, sub := newInvalidationFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	_, err := svc.UpdateMembership(ctx, caapp.UpdateMembershipRequest{Subject: sub, MembershipID: "m-target", Status: "inactive"})
	cancel()
	if err != nil {
		t.Fatal(err)
	}
	ctx2, cancel2 := context.WithCancel(context.Background())
	cancel2()
	_, _ = svc.UpdateMembership(ctx2, caapp.UpdateMembershipRequest{Subject: sub, MembershipID: "m-target", Status: "active"})
	if len(cache.ctxErrs) < 2 {
		t.Fatalf("expected an invalidation for each change, got %d", len(cache.ctxErrs))
	}
	if cache.ctxErrs[1] != nil {
		t.Fatalf("invalidation ran on a canceled context: %v", cache.ctxErrs[1])
	}
}

// accessChangingMutations must defer invalidateEffectiveAccessOnSuccess. A new mutation that
// changes roles, permissions, membership status, departments, titles or teams belongs here.
var accessChangingMutations = []string{
	"UpdateMembership", "DeleteMembership",
	"AssignRole", "RemoveRole", "ReplaceMembershipPrimaryRole",
	"AssignDepartment", "RemoveDepartment", "AssignTitle", "RemoveTitle", "UpdateMembershipOrgAssignments",
	"UpdateDepartment", "DeleteDepartment", "AddDeptMember", "RemoveDeptMember",
	"UpdateTitle", "DeleteTitle", "AddTitleMember", "RemoveTitleMember",
	"AssignRolePermission", "RemoveRolePermission", "UpdateCustomRole", "InactivateCustomRole",
	"AddDirectPermission", "RemoveDirectPermission",
	"UpdateTeam", "DeleteTeam", "AddTeamMember", "RemoveTeamMember",
}

func TestAccessChangingMutations_DeferInvalidation(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	found := map[string]bool{}
	multiWrite := map[string]bool{}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range file.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || fn.Body == nil || len(fn.Body.List) == 0 {
				continue
			}
			def, ok := fn.Body.List[0].(*ast.DeferStmt)
			if !ok {
				continue
			}
			if sel, ok := def.Call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "invalidateEffectiveAccessOnSuccess" {
				found[fn.Name.Name] = true
			}
		}
		// Multi-write mutations declare `wrote := false` first and defer the IfWritten variant.
		for _, d := range file.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || fn.Body == nil || len(fn.Body.List) < 2 {
				continue
			}
			def, ok := fn.Body.List[1].(*ast.DeferStmt)
			if !ok {
				continue
			}
			if sel, ok := def.Call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "invalidateEffectiveAccessIfWritten" {
				found[fn.Name.Name] = true
				multiWrite[fn.Name.Name] = true
			}
		}
	}
	for _, name := range accessChangingMutations {
		if !found[name] {
			t.Errorf("%s must start with defer s.invalidateEffectiveAccessOnSuccess(...)", name)
		}
	}
	// PERF-26: these run several writes without a transaction; a failure after the first write
	// must still drop the cache.
	for _, name := range []string{"ReplaceMembershipPrimaryRole", "UpdateMembershipOrgAssignments"} {
		if !multiWrite[name] {
			t.Errorf("%s must defer s.invalidateEffectiveAccessIfWritten(...)", name)
		}
	}
}
