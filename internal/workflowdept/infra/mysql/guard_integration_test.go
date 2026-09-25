package mysql

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/cobo/cobo_iam_services/internal/workflowdept"
	wdhttp "github.com/cobo/cobo_iam_services/internal/workflowdept/transport/http"
	_ "github.com/go-sql-driver/mysql"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("MYSQL_TEST_DSN"))
	if dsn == "" {
		t.Skip("MYSQL_TEST_DSN not set")
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

func TestMySQL_CatalogCodeGuardsAndMapping(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	_, _ = db.ExecContext(ctx, `DELETE FROM company_workflow_department_mappings WHERE company_id = 'co-a'`)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO workflow_template_department_code_registry (department_code) VALUES ('rollback-only')`); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	var orphan int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM workflow_template_department_code_registry WHERE department_code='rollback-only'`).Scan(&orphan); err != nil || orphan != 0 {
		t.Fatalf("rollback left registry row %d %v", orphan, err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO companies (company_id, company_code, company_name, status) VALUES ('co-a','COA','A','active')`); err != nil && !strings.Contains(err.Error(), "Duplicate") {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO departments (department_id, company_id, department_code, department_name, status) VALUES ('dep-legal','co-a','LEGAL','Pháp chế','active')`); err != nil && !strings.Contains(err.Error(), "Duplicate") {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO workflow_template_departments (department_code, department_name, display_order, is_system) VALUES ('dept-001','Phòng Pháp chế',1,1)`); err != nil && !strings.Contains(err.Error(), "Duplicate") {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE workflow_template_departments SET department_name = 'Phòng Pháp chế (sửa)' WHERE department_code = 'dept-001'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE workflow_template_departments SET department_code = 'dept-999' WHERE department_code = 'dept-001'`); err == nil {
		t.Fatal("code update should be blocked")
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM workflow_template_departments WHERE department_code = 'dept-001'`); err == nil {
		t.Fatal("delete should be blocked")
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO workflow_template_department_code_registry (department_code, retired_at) VALUES ('dept-old', NOW(3)) ON DUPLICATE KEY UPDATE retired_at = NOW(3)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO workflow_template_departments (department_code, department_name) VALUES ('dept-old','Khác')`); err == nil {
		t.Fatal("retired code reinsert should be blocked")
	}

	store := NewStore(db)
	v1, err := store.Commit(ctx, wdhttp.Write{CompanyID: "co-a", TemplateCode: "dept-001", CompanyDepartmentID: "dep-legal", ExpectedVersion: 0, Actor: "mem-1"})
	if err != nil || v1 != 1 {
		t.Fatalf("v1 %d %v", v1, err)
	}
	vSame, err := store.Commit(ctx, wdhttp.Write{CompanyID: "co-a", TemplateCode: "dept-001", CompanyDepartmentID: "dep-legal", ExpectedVersion: 1, Actor: "mem-1"})
	if err != nil || vSame != 1 {
		t.Fatalf("idempotent %d %v", vSame, err)
	}
	if _, err := store.Commit(ctx, wdhttp.Write{CompanyID: "co-a", TemplateCode: "dept-001", CompanyDepartmentID: "dep-legal", ExpectedVersion: 0, Actor: "mem-1"}); err != workflowdept.ErrVersionConflict {
		t.Fatalf("conflict %v", err)
	}

	var n int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM workflow_template_department_code_registry WHERE department_code='dept-001'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("registry %d %v", n, err)
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := store.Commit(ctx, wdhttp.Write{CompanyID: "co-a", TemplateCode: "dept-001", CompanyDepartmentID: "dep-legal", ExpectedVersion: 1, Actor: "mem-1"})
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil && err != workflowdept.ErrVersionConflict {
			t.Fatal(err)
		}
	}
	if err := store.Close(ctx, "co-a", "dept-001", "", "", 1, "mem-1"); err != nil {
		t.Fatal(err)
	}
	v2, err := store.Commit(ctx, wdhttp.Write{CompanyID: "co-a", TemplateCode: "dept-001", CompanyDepartmentID: "dep-legal", ExpectedVersion: 0, Actor: "mem-1"})
	if err != nil || v2 != 2 {
		t.Fatalf("reopen version got %d %v", v2, err)
	}
	before, after := mappingCount(t, db), 0
	if _, err := store.Suggest(ctx, "co-a", "dept-001"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Preflight(ctx, "co-a"); err != nil {
		t.Fatal(err)
	}
	after = mappingCount(t, db)
	if before != after {
		t.Fatalf("suggestion/preflight wrote rows %d -> %d", before, after)
	}
	var closed int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM company_workflow_department_mappings WHERE company_id='co-a' AND effective_to IS NOT NULL`).Scan(&closed); err != nil || closed == 0 {
		t.Fatalf("close history %d %v", closed, err)
	}
}

func TestMySQL_AdminRoleGate(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	must := func(q string) {
		t.Helper()
		if _, err := db.ExecContext(ctx, q); err != nil && !strings.Contains(err.Error(), "Duplicate") {
			t.Fatal(err)
		}
	}
	must(`INSERT INTO companies (company_id, company_code, company_name, status) VALUES ('co-auth','COAUTH','Auth','active')`)
	must(`INSERT INTO users (user_id, login_id, full_name, account_status) VALUES ('u-off','login-off','Off','inactive'), ('u-on','login-on','On','active'), ('u-dead','login-dead','Dead','active')`)
	must(`INSERT INTO memberships (membership_id, user_id, company_id, membership_status) VALUES ('m-off','u-off','co-auth','active'), ('m-on','u-on','co-auth','active'), ('m-dead','u-dead','co-auth','inactive')`)
	must(`INSERT INTO roles (role_id, company_id, role_code, role_name, status) VALUES ('r-admin','co-auth','admin_doanh_nghiep','Admin','active')`)
	must(`INSERT INTO membership_roles (membership_id, role_id, status) VALUES ('m-off','r-admin','active'), ('m-on','r-admin','active'), ('m-dead','r-admin','active')`)
	store := NewStore(db)
	off, err := store.AdminRoles(ctx, "co-auth", "m-off")
	if err != nil || len(off) != 0 {
		t.Fatalf("inactive user roles=%v err=%v", off, err)
	}
	dead, err := store.AdminRoles(ctx, "co-auth", "m-dead")
	if err != nil || len(dead) != 0 {
		t.Fatalf("inactive membership roles=%v err=%v", dead, err)
	}
	on, err := store.AdminRoles(ctx, "co-auth", "m-on")
	if err != nil || len(on) != 1 || on[0] != "admin_doanh_nghiep" {
		t.Fatalf("active admin roles=%v err=%v", on, err)
	}
	if err := store.CheckDept(ctx, "co-auth", "dep-legal"); err != workflowdept.ErrDeptNotInCompany {
		t.Fatalf("cross company dept: %v", err)
	}
}

func mappingCount(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM company_workflow_department_mappings`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
