package app

import (
	"context"
	"strings"
	"testing"
)

func TestSlugifyAssigneeRoleCode_PreservesUnicodeNameSeparately(t *testing.T) {
	code := SlugifyAssigneeRoleCode("Người kiểm tra pháp chế")
	if !strings.HasPrefix(code, "wf_role_") {
		t.Fatalf("code = %q, want wf_role_ prefix", code)
	}
}

func TestAssigneeRoleCatalogService_CreateUnicodeName(t *testing.T) {
	repo := NewInMemoryAssigneeRoleCatalog()
	svc := NewAssigneeRoleCatalogService(repo)
	created, err := svc.Create(context.Background(), CreateAssigneeRoleRequest{RoleName: "Người kiểm tra pháp chế"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.RoleName != "Người kiểm tra pháp chế" {
		t.Fatalf("role_name = %q", created.RoleName)
	}
	if created.RoleCode == "" {
		t.Fatal("expected generated role_code")
	}
	reg, err := svc.MergedRegistry(context.Background())
	if err != nil {
		t.Fatalf("MergedRegistry: %v", err)
	}
	if _, ok := reg.GetRole(created.RoleCode); !ok {
		t.Fatalf("registry missing %q", created.RoleCode)
	}
}

func TestAssigneeRoleCatalogService_DuplicateName(t *testing.T) {
	repo := NewInMemoryAssigneeRoleCatalog()
	svc := NewAssigneeRoleCatalogService(repo)
	_, err := svc.Create(context.Background(), CreateAssigneeRoleRequest{RoleName: "Người rà soát"})
	if err == nil {
		t.Fatal("expected duplicate against seed registry name")
	}
}

func TestAssigneeRoleCatalogService_DuplicateCustomName(t *testing.T) {
	repo := NewInMemoryAssigneeRoleCatalog()
	svc := NewAssigneeRoleCatalogService(repo)
	_, err := svc.Create(context.Background(), CreateAssigneeRoleRequest{RoleName: "Đơn vị kiểm tra"})
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err = svc.Create(context.Background(), CreateAssigneeRoleRequest{RoleName: "Đơn vị kiểm tra"})
	if err == nil {
		t.Fatal("expected duplicate custom name error")
	}
}
