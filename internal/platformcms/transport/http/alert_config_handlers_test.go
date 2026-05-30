package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	platformcmsapp "github.com/cobo/cobo_iam_services/internal/platformcms/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// fakeAlertConfigSvc is a test double for AlertConfigService.
type fakeAlertConfigSvc struct {
	getResult *platformcmsapp.AlertConfigDTO
	getErr    error
	upsertErr error
	captured  *platformcmsapp.UpsertAlertConfigRequest
}

func (f *fakeAlertConfigSvc) GetAlertConfig(_ context.Context, typeID string) (*platformcmsapp.AlertConfigDTO, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.getResult != nil {
		return f.getResult, nil
	}
	return &platformcmsapp.AlertConfigDTO{
		TypeID:       typeID,
		Deadline:     platformcmsapp.AlertKindConfigDTO{},
		WorkflowStep: platformcmsapp.AlertKindConfigDTO{},
	}, nil
}

func (f *fakeAlertConfigSvc) UpsertAlertConfig(_ context.Context, req platformcmsapp.UpsertAlertConfigRequest) error {
	f.captured = &req
	return f.upsertErr
}

var _ platformcmsapp.AlertConfigService = (*fakeAlertConfigSvc)(nil)

// newAlertConfigTestHandler builds a minimal Handler with inspector + authorizer + alertConfigSvc.
func newAlertConfigTestHandler(perms []string, svc platformcmsapp.AlertConfigService) *Handler {
	return &Handler{
		inspector:      listedFakeInspector{},
		authorizer:     listedFakeAuthorizer{perms: perms},
		alertConfigSvc: svc,
		metrics:        newCMSMetrics(),
	}
}

func doAlertConfigRequest(h *Handler, method, path string, body any) *httptest.ResponseRecorder {
	var reqBody *bytes.Buffer
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(b)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}
	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Authorization", "Bearer test-token")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()

	mux := http.NewServeMux()
	h.Register(mux)
	mux.ServeHTTP(w, req)
	return w
}

// ── GET tests ──────────────────────────────────────────────────────────────

func TestGetAlertConfig_DefaultResponse(t *testing.T) {
	h := newAlertConfigTestHandler([]string{"platform.cms.view"}, &fakeAlertConfigSvc{})
	w := doAlertConfigRequest(h, http.MethodGet, "/api/v1/platform/cms/templates/dt-test/alert-config", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	data, _ := resp["data"].(map[string]any)
	if data == nil {
		t.Fatalf("missing data field in response: %s", w.Body.String())
	}
	if data["typeId"] != "dt-test" {
		t.Errorf("typeId = %v, want dt-test", data["typeId"])
	}
}

func TestGetAlertConfig_Forbidden_MissingCMSView(t *testing.T) {
	h := newAlertConfigTestHandler([]string{"disclosure.view"}, &fakeAlertConfigSvc{})
	w := doAlertConfigRequest(h, http.MethodGet, "/api/v1/platform/cms/templates/dt-test/alert-config", nil)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}

func TestGetAlertConfig_NilService_Returns503(t *testing.T) {
	h := newAlertConfigTestHandler([]string{"platform.cms.view"}, nil)
	w := doAlertConfigRequest(h, http.MethodGet, "/api/v1/platform/cms/templates/dt-test/alert-config", nil)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", w.Code)
	}
}

// ── PUT tests ──────────────────────────────────────────────────────────────

func TestPutAlertConfig_HappyPath(t *testing.T) {
	svc := &fakeAlertConfigSvc{}
	h := newAlertConfigTestHandler([]string{"platform.cms.view", "rbac.manage"}, svc)
	body := map[string]any{
		"deadline":     map[string]any{"enabled": true, "templateKey": "reminder.deadline_approaching"},
		"workflowStep": map[string]any{"enabled": true, "templateKey": "reminder.workflow_step_due"},
	}
	w := doAlertConfigRequest(h, http.MethodPut, "/api/v1/platform/cms/templates/dt-test/alert-config", body)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	data, _ := resp["data"].(map[string]any)
	if data["ok"] != true {
		t.Errorf("expected ok=true in data, got %v", data)
	}
	if svc.captured == nil || svc.captured.TypeID != "dt-test" {
		t.Errorf("captured request typeID = %v", svc.captured)
	}
}

func TestPutAlertConfig_Forbidden_MissingRbacManage(t *testing.T) {
	h := newAlertConfigTestHandler([]string{"platform.cms.view"}, &fakeAlertConfigSvc{})
	body := map[string]any{
		"deadline":     map[string]any{"enabled": false, "templateKey": ""},
		"workflowStep": map[string]any{"enabled": false, "templateKey": ""},
	}
	w := doAlertConfigRequest(h, http.MethodPut, "/api/v1/platform/cms/templates/dt-test/alert-config", body)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}

func TestPutAlertConfig_InvalidTemplateKey_Returns422(t *testing.T) {
	svc := &fakeAlertConfigSvc{
		upsertErr: perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeTemplateKeyNotFound, "template key not found", nil),
	}
	h := newAlertConfigTestHandler([]string{"platform.cms.view", "rbac.manage"}, svc)
	body := map[string]any{
		"deadline":     map[string]any{"enabled": true, "templateKey": "bad.key"},
		"workflowStep": map[string]any{"enabled": false, "templateKey": ""},
	}
	w := doAlertConfigRequest(h, http.MethodPut, "/api/v1/platform/cms/templates/dt-test/alert-config", body)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	errObj, _ := resp["error"].(map[string]any)
	if errObj["code"] != string(perr.CodeTemplateKeyNotFound) {
		t.Errorf("error code = %v, want TEMPLATE_KEY_NOT_FOUND", errObj["code"])
	}
}

func TestPutAlertConfig_TypeIDNotFound_Returns404(t *testing.T) {
	svc := &fakeAlertConfigSvc{
		upsertErr: perr.NewHTTPError(http.StatusNotFound, perr.CodeDisclosureTypeNotFound, "disclosure type not found", nil),
	}
	h := newAlertConfigTestHandler([]string{"platform.cms.view", "rbac.manage"}, svc)
	body := map[string]any{
		"deadline":     map[string]any{"enabled": false, "templateKey": ""},
		"workflowStep": map[string]any{"enabled": false, "templateKey": ""},
	}
	w := doAlertConfigRequest(h, http.MethodPut, "/api/v1/platform/cms/templates/nonexistent/alert-config", body)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	errObj, _ := resp["error"].(map[string]any)
	if errObj["code"] != string(perr.CodeDisclosureTypeNotFound) {
		t.Errorf("error code = %v, want DISCLOSURE_TYPE_NOT_FOUND", errObj["code"])
	}
}

func TestPutAlertConfig_NilService_Returns503(t *testing.T) {
	h := newAlertConfigTestHandler([]string{"platform.cms.view", "rbac.manage"}, nil)
	body := map[string]any{"deadline": map[string]any{"enabled": false, "templateKey": ""}}
	w := doAlertConfigRequest(h, http.MethodPut, "/api/v1/platform/cms/templates/dt-test/alert-config", body)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", w.Code)
	}
}

// Unused import guard
var _ = errors.New
