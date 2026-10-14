package mysql

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	_ "github.com/go-sql-driver/mysql"
)

// Runs against a migrated MySQL 8.0 (see migrations/run_dev_migrations.sh):
//
//	MYSQL_TEST_DSN='<user>:<pass>@tcp(127.0.0.1:3306)/cobo_iam?parseTime=true' go test ./internal/disclosure/infra/mysql/ -run 'DepartmentFilter|DepartmentNames'
func departmentFilterTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("MYSQL_TEST_DSN"))
	if dsn == "" {
		t.Skip("MYSQL_TEST_DSN not set")
	}
	// Same session collation as the app (config.normalizeMySQLDSN); JSON_TABLE columns take it.
	if !strings.Contains(strings.ToLower(dsn), "collation=") {
		sep := "?"
		if strings.Contains(dsn, "?") {
			sep = "&"
		}
		dsn += sep + "charset=utf8mb4&collation=utf8mb4_unicode_ci"
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Ping(); err != nil {
		t.Fatal(err)
	}
	return db
}

const (
	dfCompany      = "it-dtf-co"
	dfTenantDept   = "it-dtf-dep-hr"
	dfTypeLegal    = "it-dtf-type-legal"
	dfTypeIR       = "it-dtf-type-ir"
	dfTypeTenant   = "it-dtf-type-tenant"
	dfTemplateCode = "dept-001" // seeded by 0114_workflow_template_departments
)

func seedDepartmentFilterFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx := context.Background()
	cleanup := func() {
		for _, q := range []string{
			`DELETE FROM disclosure_type_versions WHERE type_id LIKE 'it-dtf-%'`,
			`DELETE FROM disclosure_types WHERE type_id LIKE 'it-dtf-%'`,
			`DELETE FROM departments WHERE department_id = '` + dfTenantDept + `'`,
			`DELETE FROM companies WHERE company_id = '` + dfCompany + `'`,
		} {
			if _, err := db.ExecContext(ctx, q); err != nil {
				t.Errorf("cleanup %q: %v", q, err)
			}
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.ExecContext(ctx, q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	exec(`INSERT INTO companies (company_id, company_code, company_name, status) VALUES (?, 'IT-DTF', 'IT dept filter', 'active')`, dfCompany)
	exec(`INSERT INTO departments (department_id, company_id, department_code, department_name, status) VALUES (?, ?, 'IT-DTF-HR', 'Phòng Nhân sự IT', 'active')`, dfTenantDept, dfCompany)

	addType := func(typeID string, companyID any, manifest string) {
		exec(`INSERT INTO disclosure_types (type_id, group_id, company_id, active_version_no) VALUES (?, 'group-001', ?, 1)`, typeID, companyID)
		exec(`INSERT INTO disclosure_type_versions
			(type_id, version_no, name, category, template_category, description, deadline_rule, updated_by, workflow_manifest_json, workflow_authority_mode)
			VALUES (?, 1, ?, 'Định kỳ', 'periodic', '', 'T+5', 'it', ?, 'TEMPLATE_PINNED')`, typeID, typeID, manifest)
	}
	// Owned by the test company (not global) so a shared DB never shows them to other tenants;
	// the list and the options read the caller's own types the same way as global ones.
	addType(dfTypeLegal, dfCompany, `{"steps":[{"department_id":"dept-001"},{"department_id":"dept-003"}]}`)
	addType(dfTypeIR, dfCompany, `{"steps":[{"department_id":"dept-002"}]}`)
	addType(dfTypeTenant, dfCompany, `{"steps":[{"department_id":"`+dfTenantDept+`"}]}`)
}

func TestMySQL_ListTypes_DepartmentFilterMatchesWorkflowSteps(t *testing.T) {
	db := departmentFilterTestDB(t)
	seedDepartmentFilterFixture(t, db)
	repo := NewRepository(db)

	items, _, err := repo.ListTypes(context.Background(), disclosureapp.ListTypesParams{
		CompanyID:    dfCompany,
		DepartmentID: dfTemplateCode,
		Page:         1,
		PageSize:     100,
	})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, it := range items {
		got[it.TypeID] = true
	}
	if !got[dfTypeLegal] {
		t.Fatalf("department_id=%s must return %s (its pinned workflow has a %s step); got %v", dfTemplateCode, dfTypeLegal, dfTemplateCode, got)
	}
	if got[dfTypeIR] || got[dfTypeTenant] {
		t.Fatalf("department_id=%s must not return types without such a step; got %v", dfTemplateCode, got)
	}
}

func TestMySQL_ListTypeFilterOptions_DepartmentNames(t *testing.T) {
	db := departmentFilterTestDB(t)
	seedDepartmentFilterFixture(t, db)
	repo := NewRepository(db)

	var catalogName string
	if err := db.QueryRow(`SELECT department_name FROM workflow_template_departments WHERE department_code = ?`, dfTemplateCode).Scan(&catalogName); err != nil {
		t.Fatal(err)
	}

	resp, err := repo.ListTypeFilterOptions(context.Background(), dfCompany, []string{dfTypeLegal, dfTypeIR, dfTypeTenant})
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]string{}
	for _, d := range resp.Departments {
		names[d.ID] = d.Name
	}
	if names[dfTemplateCode] != catalogName {
		t.Fatalf("%s name = %q, want template catalog name %q (options: %v)", dfTemplateCode, names[dfTemplateCode], catalogName, names)
	}
	if names[dfTenantDept] != "Phòng Nhân sự IT" {
		t.Fatalf("tenant department name = %q, want %q", names[dfTenantDept], "Phòng Nhân sự IT")
	}
	// Every option the filter offers must select at least one type (options and filter share one id space).
	for id := range names {
		if !strings.HasPrefix(id, "dept-") && id != dfTenantDept {
			continue
		}
		items, _, err := repo.ListTypes(context.Background(), disclosureapp.ListTypesParams{CompanyID: dfCompany, DepartmentID: id, Page: 1, PageSize: 100})
		if err != nil {
			t.Fatal(err)
		}
		if len(items) == 0 {
			t.Fatalf("option %q (%s) selects no type", id, names[id])
		}
	}
}
