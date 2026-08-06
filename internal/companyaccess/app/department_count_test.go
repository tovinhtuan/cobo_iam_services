package app_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	"github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
)

func TestGetCompanyPlatform_DepartmentCount_ActiveOnly(t *testing.T) {
	repo := inmemory.NewAdminRepository()
	repo.SeedCompany(caapp.PlatformCompanyDetail{
		CompanyID:   "c-own",
		CompanyCode: "OWN",
		CompanyName: "Own Co",
		Status:      "active",
		MemberCount: 2,
	})
	repo.SeedCompany(caapp.PlatformCompanyDetail{
		CompanyID:   "c-other",
		CompanyCode: "OTH",
		CompanyName: "Other Co",
		Status:      "active",
	})
	repo.SeedDepartmentForCompany("c-own", caapp.DepartmentView{DepartmentID: "d1", Name: "Legal", Status: "active"})
	repo.SeedDepartmentForCompany("c-own", caapp.DepartmentView{DepartmentID: "d2", Name: "Finance", Status: "active"})
	repo.SeedDepartmentForCompany("c-own", caapp.DepartmentView{DepartmentID: "d3", Name: "Old", Status: "inactive"})
	repo.SeedDepartmentForCompany("c-other", caapp.DepartmentView{DepartmentID: "d4", Name: "Other Legal", Status: "active"})
	// Empty status excluded (MySQL: status = 'active' exact)
	repo.SeedDepartmentForCompany("c-own", caapp.DepartmentView{DepartmentID: "d5", Name: "Ops", Status: ""})
	repo.SeedDepartmentForCompany("c-own", caapp.DepartmentView{DepartmentID: "d6", Name: "DeletedSoft", Status: "inactive"})

	out, err := repo.GetCompanyPlatform(context.Background(), "c-own")
	if err != nil {
		t.Fatalf("GetCompanyPlatform: %v", err)
	}
	if out.DepartmentCount != 2 {
		t.Fatalf("DepartmentCount=%d want 2 (active only; empty/inactive excluded)", out.DepartmentCount)
	}
	if out.MemberCount != 2 {
		t.Fatalf("MemberCount drifted: %d", out.MemberCount)
	}

	other, err := repo.GetCompanyPlatform(context.Background(), "c-other")
	if err != nil {
		t.Fatalf("other: %v", err)
	}
	if other.DepartmentCount != 1 {
		t.Fatalf("other company DepartmentCount=%d want 1", other.DepartmentCount)
	}
}

func TestGetCompanyPlatform_DepartmentCount_Zero(t *testing.T) {
	repo := inmemory.NewAdminRepository()
	repo.SeedCompany(caapp.PlatformCompanyDetail{
		CompanyID:   "c-empty",
		CompanyCode: "EMP",
		CompanyName: "Empty",
		Status:      "active",
	})
	out, err := repo.GetCompanyPlatform(context.Background(), "c-empty")
	if err != nil {
		t.Fatalf("GetCompanyPlatform: %v", err)
	}
	if out.DepartmentCount != 0 {
		t.Fatalf("DepartmentCount=%d want 0", out.DepartmentCount)
	}
}

func TestPlatformCompanyDetail_DepartmentCount_JSONAdditive(t *testing.T) {
	d := caapp.PlatformCompanyDetail{
		CompanyID:       "c1",
		CompanyCode:     "C1",
		CompanyName:     "Co",
		Status:          "active",
		MemberCount:     1,
		DisclosureCount: 2,
		TemplateCount:   3,
		DepartmentCount: 10,
		BusinessSectors: []string{},
	}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["department_count"]; !ok {
		t.Fatal("department_count missing from JSON")
	}
	if m["department_count"].(float64) != 10 {
		t.Fatalf("department_count=%v want 10", m["department_count"])
	}
	if m["member_count"].(float64) != 1 {
		t.Fatal("member_count changed")
	}
	if m["disclosure_count"].(float64) != 2 {
		t.Fatal("disclosure_count changed")
	}
}

func TestGetCompanyPlatform_DepartmentCount_TenActive(t *testing.T) {
	repo := inmemory.NewAdminRepository()
	repo.SeedCompany(caapp.PlatformCompanyDetail{
		CompanyID: "c10", CompanyCode: "T10", CompanyName: "Ten", Status: "active",
	})
	for i := 0; i < 10; i++ {
		id := fmt.Sprintf("d-%02d", i)
		repo.SeedDepartmentForCompany("c10", caapp.DepartmentView{
			DepartmentID: id, Name: id, Status: "active",
		})
	}
	out, err := repo.GetCompanyPlatform(context.Background(), "c10")
	if err != nil {
		t.Fatal(err)
	}
	if out.DepartmentCount != 10 {
		t.Fatalf("DepartmentCount=%d want 10", out.DepartmentCount)
	}
}
