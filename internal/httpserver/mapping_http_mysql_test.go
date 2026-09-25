package httpserver_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	iamtokenopaque "github.com/cobo/cobo_iam_services/internal/iam/infra/token/opaque"
	_ "github.com/go-sql-driver/mysql"
)

func mappingTestDB(t *testing.T) *sql.DB {
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

func TestMappingHTTP_WithMySQLPool(t *testing.T) {
	db := mappingTestDB(t)
	ctx := context.Background()
	exec := func(q string) {
		t.Helper()
		if _, err := db.ExecContext(ctx, q); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO companies (company_id, company_code, company_name, status) VALUES ('co-map','COMAP','Map Co','active'), ('co-other','COOTH','Other','active')`)
	exec(`INSERT INTO users (user_id, login_id, full_name, account_status) VALUES ('u-map','login-map','Map','active'), ('u-off','login-map-off','Off','inactive'), ('u-b','login-map-b','B','active')`)
	exec(`INSERT INTO memberships (membership_id, user_id, company_id, membership_status) VALUES ('m-map','u-map','co-map','active'), ('m-off','u-off','co-map','active'), ('m-b','u-b','co-other','active')`)
	exec(`INSERT INTO roles (role_id, company_id, role_code, role_name, status) VALUES ('r-map','co-map','admin_doanh_nghiep','Admin','active'), ('r-user','co-map','user_thuong','User','active'), ('r-b','co-other','admin_doanh_nghiep','Admin B','active')`)
	exec(`INSERT INTO membership_roles (membership_id, role_id, status) VALUES ('m-map','r-map','active'), ('m-off','r-user','active'), ('m-b','r-b','active')`)
	exec(`INSERT INTO departments (department_id, company_id, department_code, department_name, status) VALUES ('dep-map','co-map','LEGAL','Pháp chế','active'), ('dep-other','co-other','LEGAL2','Khác','active')`)

	var typeID string
	if err := db.QueryRowContext(ctx, `SELECT type_id FROM disclosure_types LIMIT 1`).Scan(&typeID); err != nil {
		t.Fatal(err)
	}
	exec(`INSERT INTO company_template_workflow_overrides (override_id, company_id, type_id, active_version_no, status, created_by, updated_by) VALUES ('ov-map','co-map','` + typeID + `',1,'active','u-map','u-map')`)
	exec(`INSERT INTO company_template_workflow_override_versions (override_id, version_no, workflow_json, state, created_by) VALUES ('ov-map',1,'[{"step_id":"step-legal","department_id":"dept-001","stage":"Legal","due_rule":"T+1","display_order":1}]','active','u-map')`)

	id := &staticID{}
	tm := iamtokenopaque.NewManager(id)
	token := func(user, membership, company string) string {
		t.Helper()
		raw, _, err := tm.IssueAccessToken(ctx, iamapp.AccessTokenClaims{Sub: user, MembershipID: membership, CompanyID: company})
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	admin := token("u-map", "m-map", "co-map")
	srv := httptest.NewServer(newTestHandlerWithDeps(t, db, testAPIConfig(), tm))
	defer srv.Close()

	call := func(method, path, body, tok string) *http.Response {
		t.Helper()
		req, err := http.NewRequest(method, srv.URL+path, bytes.NewBufferString(body))
		if err != nil {
			t.Fatal(err)
		}
		if tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}

	res := call(http.MethodGet, "/api/v1/admin/workflow-department-mappings", "", "")
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no token status=%d", res.StatusCode)
	}
	res.Body.Close()

	offTok := token("u-off", "m-off", "co-map")
	res = call(http.MethodGet, "/api/v1/admin/workflow-department-mappings", "", offTok)
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("non-admin status=%d body=%s", res.StatusCode, readBody(t, res.Body))
	}

	res = call(http.MethodPut, "/api/v1/admin/workflow-department-mappings/default/dept-001",
		`{"company_department_id":"dep-map","expected_version":0,"company_id":"co-other","department_name":"ignored","effective_from":"2000-01-01T00:00:00Z"}`, admin)
	body := readBody(t, res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("put status=%d body=%s", res.StatusCode, body)
	}
	var created struct {
		Version int64 `json:"version"`
	}
	if err := json.Unmarshal([]byte(body), &created); err != nil || created.Version != 1 {
		t.Fatalf("version %+v %s", created, body)
	}
	var storedCompany, storedDept string
	if err := db.QueryRowContext(ctx, `SELECT company_id, company_department_id FROM company_workflow_department_mappings WHERE template_department_code='dept-001' AND disclosure_type_id='' AND step_code='' AND company_id='co-map' AND effective_to IS NULL`).Scan(&storedCompany, &storedDept); err != nil {
		t.Fatal(err)
	}
	if storedCompany != "co-map" || storedDept != "dep-map" {
		t.Fatalf("row company=%s dept=%s", storedCompany, storedDept)
	}

	res = call(http.MethodPut, "/api/v1/admin/workflow-department-mappings/default/dept-001", `{"company_department_id":"dep-map","expected_version":0}`, admin)
	if res.StatusCode != http.StatusConflict || !strings.Contains(readBody(t, res.Body), "MAPPING_VERSION_CONFLICT") {
		t.Fatalf("conflict status=%d", res.StatusCode)
	}

	res = call(http.MethodPut, "/api/v1/admin/workflow-department-mappings/default/dept-001", `{"company_department_id":"dep-other","expected_version":1}`, admin)
	if res.StatusCode != http.StatusBadRequest || !strings.Contains(readBody(t, res.Body), "DEPARTMENT_NOT_IN_COMPANY") {
		t.Fatalf("cross dept status=%d body=%s", res.StatusCode, readBody(t, res.Body))
	}

	other := token("u-b", "m-b", "co-other")
	res = call(http.MethodGet, "/api/v1/admin/workflow-department-mappings", "", other)
	otherBody := readBody(t, res.Body)
	if res.StatusCode != http.StatusOK || strings.Contains(otherBody, "dep-map") {
		t.Fatalf("company B must not see A mapping: %d %s", res.StatusCode, otherBody)
	}

	res = call(http.MethodPut, "/api/v1/admin/disclosure-types/"+typeID+"/workflow-steps/step-legal/department-mapping/dept-002", `{"company_department_id":"dep-map","expected_version":0}`, admin)
	if res.StatusCode != http.StatusBadRequest || !strings.Contains(readBody(t, res.Body), "STEP_TOKEN_MISMATCH") {
		t.Fatalf("mismatch status=%d body=%s", res.StatusCode, readBody(t, res.Body))
	}
	res = call(http.MethodPut, "/api/v1/admin/disclosure-types/"+typeID+"/department-mapping/dept-001", `{"company_department_id":"dep-map","expected_version":0}`, admin)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("type scope status=%d body=%s", res.StatusCode, readBody(t, res.Body))
	}

	res = call(http.MethodDelete, "/api/v1/admin/workflow-department-mappings/default/dept-001", `{"expected_version":1}`, admin)
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status=%d body=%s", res.StatusCode, readBody(t, res.Body))
	}
	var open int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM company_workflow_department_mappings WHERE company_id='co-map' AND template_department_code='dept-001' AND disclosure_type_id='' AND step_code='' AND effective_to IS NULL`).Scan(&open); err != nil || open != 0 {
		t.Fatalf("open after delete %d %v", open, err)
	}
	var hist int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM company_workflow_department_mappings WHERE company_id='co-map' AND template_department_code='dept-001' AND effective_to IS NOT NULL`).Scan(&hist); err != nil || hist == 0 {
		t.Fatalf("history %d %v", hist, err)
	}
}
