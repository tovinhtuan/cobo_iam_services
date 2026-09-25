package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/cobo/cobo_iam_services/internal/workflowdept"
)

type memStore struct {
	mu   sync.Mutex
	rows map[string]memRow
}

type memRow struct {
	version int64
	dept    string
	open    bool
}

func key(company, code, typeID, step string) string {
	return company + "|" + code + "|" + typeID + "|" + step
}

func (m *memStore) Commit(_ context.Context, in Write) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.rows == nil {
		m.rows = map[string]memRow{}
	}
	k := key(in.CompanyID, in.TemplateCode, in.DisclosureTypeID, in.StepCode)
	row, ok := m.rows[k]
	has := ok && row.open
	ver, dept := int64(0), ""
	if has {
		ver, dept = row.version, row.dept
	}
	action, err := workflowdept.PlanWrite(ver, has, dept, in.CompanyDepartmentID, in.ExpectedVersion)
	if err != nil {
		return 0, err
	}
	switch action {
	case workflowdept.ActionCreate:
		m.rows[k] = memRow{version: 1, dept: in.CompanyDepartmentID, open: true}
		return 1, nil
	case workflowdept.ActionReplace:
		row.version++
		row.dept = in.CompanyDepartmentID
		row.open = true
		m.rows[k] = row
		return row.version, nil
	default:
		return ver, nil
	}
}

func (m *memStore) Close(_ context.Context, companyID, templateCode, typeID, stepCode string, version int64, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := key(companyID, templateCode, typeID, stepCode)
	row, ok := m.rows[k]
	if !ok || !row.open {
		return workflowdept.ErrNotFound
	}
	if _, err := workflowdept.PlanClose(row.version, true, version); err != nil {
		return err
	}
	row.open = false
	m.rows[k] = row
	return nil
}

func (m *memStore) Suggest(context.Context, string, string) ([]Suggestion, error) {
	return []Suggestion{{DepartmentID: "uuid-legal", DepartmentName: "Pháp chế", Class: "PREFIX_VARIANT"}}, nil
}

func (m *memStore) Preflight(context.Context, string) ([]PreflightRow, error) {
	return nil, nil
}

func (m *memStore) List(_ context.Context, companyID string) ([]Item, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Item
	prefix := companyID + "|"
	for k, row := range m.rows {
		if len(k) < len(prefix) || k[:len(prefix)] != prefix || !row.open {
			continue
		}
		out = append(out, Item{CompanyDepartmentID: row.dept, Version: row.version, Status: "active"})
	}
	return out, nil
}

func handler() (Handler, *memStore) {
	store := &memStore{}
	h := Handler{
		Session: func(r *http.Request) (Actor, error) {
			return Actor{CompanyID: r.Header.Get("X-Company"), MembershipID: r.Header.Get("X-Member"), Roles: []string{r.Header.Get("X-Role")}}, nil
		},
		Code: func(_ context.Context, code string) (bool, error) { return code == "dept-001", nil },
		Dept: func(_ context.Context, companyID, departmentID string) error {
			if companyID == "c1" && departmentID == "uuid-legal" {
				return nil
			}
			return workflowdept.ErrDeptNotInCompany
		},
		Store: store,
	}
	return h, store
}

func put(t *testing.T, h Handler, path, body, role string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	h.Register(mux)
	req := httptest.NewRequest(http.MethodPut, path, bytes.NewBufferString(body))
	req.Header.Set("X-Company", "c1")
	req.Header.Set("X-Member", "m1")
	req.Header.Set("X-Role", role)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestPutForbiddenWithoutTenantAdminRole(t *testing.T) {
	h, _ := handler()
	rec := put(t, h, "/api/v1/admin/workflow-department-mappings/default/dept-001", `{"company_department_id":"uuid-legal","expected_version":0}`, "platform_admin")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestPutIgnoresBodyCompanyAndName(t *testing.T) {
	h, store := handler()
	rec := put(t, h, "/api/v1/admin/workflow-department-mappings/default/dept-001", `{"company_department_id":"uuid-legal","expected_version":0,"company_id":"other","department_name":"Pháp chế"}`, "admin_doanh_nghiep")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if store.rows[key("c1", "dept-001", "", "")].dept != "uuid-legal" {
		t.Fatalf("stored %+v", store.rows)
	}
	if _, ok := store.rows[key("other", "dept-001", "", "")]; ok {
		t.Fatal("body company_id must not create a row")
	}
}

func TestPutCrossTenantDepartmentRejected(t *testing.T) {
	h, _ := handler()
	rec := put(t, h, "/api/v1/admin/workflow-department-mappings/default/dept-001", `{"company_department_id":"other-company-dept","expected_version":0}`, "admin_doanh_nghiep")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestPutVersionConflictAndIdempotent(t *testing.T) {
	h, _ := handler()
	first := put(t, h, "/api/v1/admin/workflow-department-mappings/default/dept-001", `{"company_department_id":"uuid-legal","expected_version":0}`, "admin_doanh_nghiep")
	if first.Code != http.StatusOK {
		t.Fatal(first.Body.String())
	}
	again := put(t, h, "/api/v1/admin/workflow-department-mappings/default/dept-001", `{"company_department_id":"uuid-legal","expected_version":1}`, "admin_doanh_nghiep")
	if again.Code != http.StatusOK {
		t.Fatalf("idempotent %d %s", again.Code, again.Body.String())
	}
	clash := put(t, h, "/api/v1/admin/workflow-department-mappings/default/dept-001", `{"company_department_id":"uuid-legal","expected_version":9}`, "admin_doanh_nghiep")
	if clash.Code != http.StatusConflict {
		t.Fatalf("conflict %d", clash.Code)
	}
}
