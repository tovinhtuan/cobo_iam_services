package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/infra/inmemory"
	disclosurehttp "github.com/cobo/cobo_iam_services/internal/disclosure/transport/http"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

type cmsAuthMock struct {
	perms []string
}

func (a *cmsAuthMock) GetEffectiveAccess(_ context.Context, _, _ string) (*authapp.EffectiveAccessSummary, error) {
	return &authapp.EffectiveAccessSummary{Permissions: a.perms}, nil
}
func (a *cmsAuthMock) Authorize(_ context.Context, _ authapp.AuthorizeRequest) (*authapp.AuthorizeDecision, error) {
	return nil, nil
}
func (a *cmsAuthMock) AuthorizeBatch(_ context.Context, _ authapp.AuthorizeBatchRequest) (*authapp.AuthorizeBatchResponse, error) {
	return nil, nil
}

type testTokenInspector struct {
	sub          string
	membershipID string
	companyID    string
}

func (t testTokenInspector) InspectAccessToken(_ context.Context, tok string) (*iamapp.AccessTokenClaims, error) {
	if strings.TrimSpace(tok) == "" {
		return nil, perr.NewHTTPError(http.StatusUnauthorized, perr.CodeSessionExpired, "invalid access token", nil)
	}
	return &iamapp.AccessTokenClaims{
		Sub:          t.sub,
		MembershipID: t.membershipID,
		CompanyID:    t.companyID,
	}, nil
}

func (testTokenInspector) InspectPreCompanyToken(context.Context, string) (*iamapp.PreCompanyTokenClaims, error) {
	return nil, nil
}

type testAuditSpy struct {
	events []auditapp.AppendAuditLogRequest
}

func (s *testAuditSpy) AppendAuditLog(_ context.Context, req auditapp.AppendAuditLogRequest) error {
	s.events = append(s.events, req)
	return nil
}

func setupImportTestServer() (http.Handler, *testAuditSpy) {
	_ = os.Setenv("CMS_TEMPLATE_IMPORT_SIGNING_SECRET", "test-cms-template-import-secret-for-suite")
	repo := inmemory.NewRepository()
	auth := &cmsAuthMock{perms: []string{"platform.cms.view", "cms.template.write"}}
	svc := disclosureapp.NewService(repo, auth, idgen.UUIDv7Generator{})
	inspector := testTokenInspector{sub: "admin-platform-1", membershipID: "mem-1", companyID: "cobo-platform"}
	auditSpy := &testAuditSpy{}
	h := disclosurehttp.NewHandler(svc, inspector, nil, auditSpy)

	mux := http.NewServeMux()
	h.Register(mux)
	return mux, auditSpy
}

func makeMultipartFileReq(fieldName, fileName string, content []byte) *http.Request {
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	part, _ := w.CreateFormFile(fieldName, fileName)
	_, _ = part.Write(content)
	_ = w.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/platform/cms/templates/import/validate", &b)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer valid_atk")
	return req
}

// ─── POSITIVE TESTS (P1 — P12) ────────────────────────────────────────────────

func TestCmsValidateTemplateImport_PositiveVariants(t *testing.T) {
	handler, auditSpy := setupImportTestServer()

	t.Run("P1: valid periodic template with quarterly schedule", func(t *testing.T) {
		payload := `{
			"schema_version": "1.0",
			"template": {
				"name": "Báo cáo tài chính Quý P1",
				"template_category": "periodic",
				"periodicity": "quarterly",
				"deadline_rule": "T+20",
				"display_group_codes": ["display_groups_003"],
				"deadline_config": {
					"frequency_unit": "QUARTERLY",
					"cycle_anchor_day": 31,
					"month_in_quarter": 3,
					"duration_type": "CALENDAR_DAYS"
				},
				"workflow": {
					"steps": [
						{
							"stage": "Lập báo cáo",
							"department_id": "dept-003",
							"assignee_role_ids": ["reviewer"],
							"processing_days": 5
						},
						{
							"stage": "Phê duyệt",
							"department_id": "dept-001",
							"assignee_role_ids": ["approver"],
							"processing_days": 2
						}
					]
				}
			}
		}`
		req := makeMultipartFileReq("file", "p1.json", []byte(payload))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		var resp disclosureapp.ValidateTemplateImportResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)

		if !resp.ParseValid || !resp.DomainValid || !resp.CanConfirm || !resp.ActivationReady {
			t.Errorf("P1 expected all valid flags true: %+v", resp)
		}
		if resp.ValidationToken == "" {
			t.Error("P1 expected validation token")
		}
	})

	t.Run("P2: valid irregular template", func(t *testing.T) {
		payload := `{
			"schema_version": "1.0",
			"template": {
				"name": "Công bố thông tin bất thường P2",
				"template_category": "irregular",
				"deadline_rule": "Trong vòng 24 giờ kể từ khi sự kiện xảy ra",
				"workflow": {
					"steps": [
						{
							"stage": "Xác nhận sự kiện",
							"department_id": "dept-002",
							"assignee_role_ids": ["approver"],
							"processing_days": 1
						}
					]
				}
			}
		}`
		req := makeMultipartFileReq("file", "p2.json", []byte(payload))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		var resp disclosureapp.ValidateTemplateImportResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)

		if !resp.DomainValid || !resp.CanConfirm {
			t.Errorf("P2 failed: %+v", resp)
		}
		if resp.Preview.ResolvedGroupID != disclosureapp.DefaultIrregularGroupID {
			t.Errorf("P2 group_id = %q, want %q", resp.Preview.ResolvedGroupID, disclosureapp.DefaultIrregularGroupID)
		}
	})

	t.Run("P3: valid DAILY frequency", func(t *testing.T) {
		payload := `{
			"schema_version": "1.0",
			"template": {
				"name": "Báo cáo ngày P3",
				"template_category": "periodic",
				"periodicity": "daily",
				"deadline_rule": "Hàng ngày",
				"deadline_config": {
					"frequency_unit": "DAILY",
					"duration_type": "CALENDAR_DAYS"
				},
				"workflow": {
					"steps": [{"stage": "Duyệt", "department_id": "dept-001", "assignee_role_ids": ["approver"], "processing_days": 1}]
				}
			}
		}`
		req := makeMultipartFileReq("file", "p3.json", []byte(payload))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("P3 status = %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("P4: valid WEEKLY frequency", func(t *testing.T) {
		payload := `{
			"schema_version": "1.0",
			"template": {
				"name": "Báo cáo tuần P4",
				"template_category": "periodic",
				"periodicity": "weekly",
				"deadline_rule": "Thứ 6 hàng tuần",
				"deadline_config": {
					"frequency_unit": "WEEKLY",
					"cycle_anchor_weekday": "friday",
					"duration_type": "CALENDAR_DAYS"
				},
				"workflow": {
					"steps": [{"stage": "Duyệt", "department_id": "dept-001", "assignee_role_ids": ["approver"], "processing_days": 1}]
				}
			}
		}`
		req := makeMultipartFileReq("file", "p4.json", []byte(payload))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("P4 status = %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("P5: valid MONTHLY frequency", func(t *testing.T) {
		payload := `{
			"schema_version": "1.0",
			"template": {
				"name": "Báo cáo tháng P5",
				"template_category": "periodic",
				"periodicity": "monthly",
				"deadline_rule": "T+10",
				"deadline_config": {
					"frequency_unit": "MONTHLY",
					"cycle_anchor_day": 15,
					"duration_type": "CALENDAR_DAYS"
				},
				"workflow": {
					"steps": [{"stage": "Duyệt", "department_id": "dept-001", "assignee_role_ids": ["approver"], "processing_days": 1}]
				}
			}
		}`
		req := makeMultipartFileReq("file", "p5.json", []byte(payload))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("P5 status = %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("P6: valid QUARTERLY frequency", func(t *testing.T) {
		payload := `{
			"schema_version": "1.0",
			"template": {
				"name": "Báo cáo quý P6",
				"template_category": "periodic",
				"periodicity": "quarterly",
				"deadline_rule": "T+20",
				"deadline_config": {
					"frequency_unit": "QUARTERLY",
					"cycle_anchor_day": 30,
					"month_in_quarter": 3
				},
				"workflow": {
					"steps": [{"stage": "Duyệt", "department_id": "dept-001", "assignee_role_ids": ["approver"], "processing_days": 1}]
				}
			}
		}`
		req := makeMultipartFileReq("file", "p6.json", []byte(payload))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("P6 status = %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("P7: valid YEARLY frequency", func(t *testing.T) {
		payload := `{
			"schema_version": "1.0",
			"template": {
				"name": "Báo cáo thường niên P7",
				"template_category": "periodic",
				"periodicity": "yearly",
				"deadline_rule": "T+90",
				"deadline_config": {
					"frequency_unit": "YEARLY",
					"cycle_anchor_day": 31
				},
				"workflow": {
					"steps": [{"stage": "Duyệt", "department_id": "dept-001", "assignee_role_ids": ["approver"], "processing_days": 1}]
				}
			}
		}`
		req := makeMultipartFileReq("file", "p7.json", []byte(payload))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("P7 status = %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("P8: CALENDAR_DAYS duration type", func(t *testing.T) {
		payload := `{
			"schema_version": "1.0",
			"template": {
				"name": "Lịch theo ngày dương lịch P8",
				"template_category": "periodic",
				"periodicity": "monthly",
				"deadline_rule": "T+5",
				"deadline_config": {
					"frequency_unit": "MONTHLY",
					"cycle_anchor_day": 1,
					"duration_type": "CALENDAR_DAYS",
					"deadline_duration_type": "CALENDAR_DAYS"
				},
				"workflow": {
					"steps": [{"stage": "Duyệt", "department_id": "dept-001", "assignee_role_ids": ["approver"], "processing_days": 1}]
				}
			}
		}`
		req := makeMultipartFileReq("file", "p8.json", []byte(payload))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("P8 status = %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("P9: WORKING_DAYS duration type", func(t *testing.T) {
		payload := `{
			"schema_version": "1.0",
			"template": {
				"name": "Lịch theo ngày làm việc P9",
				"template_category": "periodic",
				"periodicity": "monthly",
				"deadline_rule": "T+5 ngày làm việc",
				"deadline_config": {
					"frequency_unit": "MONTHLY",
					"cycle_anchor_day": 1,
					"duration_type": "WORKING_DAYS",
					"deadline_duration_type": "WORKING_DAYS"
				},
				"workflow": {
					"steps": [{"stage": "Duyệt", "department_id": "dept-001", "assignee_role_ids": ["approver"], "processing_days": 1}]
				}
			}
		}`
		req := makeMultipartFileReq("file", "p9.json", []byte(payload))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("P9 status = %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("P10: valid past ApplicableTo emits warning and allows Draft", func(t *testing.T) {
		payload := `{
			"schema_version": "1.0",
			"template": {
				"name": "Template hết hạn P10",
				"template_category": "periodic",
				"periodicity": "monthly",
				"deadline_rule": "T+5",
				"deadline_config": {
					"frequency_unit": "MONTHLY",
					"cycle_anchor_day": 1,
					"applicable_to": "2024-01-01"
				},
				"workflow": {
					"steps": [{"stage": "Duyệt", "department_id": "dept-001", "assignee_role_ids": ["approver"], "processing_days": 1}]
				}
			}
		}`
		req := makeMultipartFileReq("file", "p10.json", []byte(payload))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("P10 status = %d: %s", rec.Code, rec.Body.String())
		}
		var resp disclosureapp.ValidateTemplateImportResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)

		if !resp.DomainValid {
			t.Errorf("P10 expected DomainValid=true for past ApplicableTo, got false")
		}
		if len(resp.Warnings) == 0 {
			t.Error("P10 expected warning for past ApplicableTo")
		}
		if resp.ActivationReady {
			t.Error("P10 expected ActivationReady=false for past ApplicableTo")
		}
	})

	t.Run("P11: unknown department sets mapping_required=true and can_confirm=false", func(t *testing.T) {
		payload := `{
			"schema_version": "1.0",
			"template": {
				"name": "Template phòng ban lạ P11",
				"template_category": "periodic",
				"periodicity": "monthly",
				"deadline_rule": "T+5",
				"deadline_config": {
					"frequency_unit": "MONTHLY",
					"cycle_anchor_day": 1
				},
				"workflow": {
					"steps": [
						{
							"stage": "Rà soát",
							"department_id": "foreign-dept-999",
							"department_name": "Phòng Ban Ngoài",
							"assignee_role_ids": ["approver"],
							"processing_days": 2
						}
					]
				}
			}
		}`
		req := makeMultipartFileReq("file", "p11.json", []byte(payload))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("P11 status = %d: %s", rec.Code, rec.Body.String())
		}
		var resp disclosureapp.ValidateTemplateImportResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)

		if !resp.DomainValid {
			t.Errorf("P11 expected DomainValid=true, got false")
		}
		if !resp.MappingRequired {
			t.Errorf("P11 expected MappingRequired=true, got false")
		}
		if resp.CanConfirm {
			t.Errorf("P11 expected CanConfirm=false, got true")
		}
	})

	t.Run("P12: complete activation-ready template", func(t *testing.T) {
		payload := `{
			"schema_version": "1.0",
			"template": {
				"name": "Template chuẩn chỉnh P12",
				"template_category": "periodic",
				"periodicity": "monthly",
				"deadline_rule": "T+5",
				"deadline_config": {
					"frequency_unit": "MONTHLY",
					"cycle_anchor_day": 15
				},
				"workflow": {
					"steps": [
						{"stage": "Bước 1", "department_id": "dept-003", "assignee_role_ids": ["reviewer"], "processing_days": 2},
						{"stage": "Bước 2", "department_id": "dept-001", "assignee_role_ids": ["approver"], "processing_days": 1}
					]
				}
			}
		}`
		req := makeMultipartFileReq("file", "p12.json", []byte(payload))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("P12 status = %d: %s", rec.Code, rec.Body.String())
		}
		var resp disclosureapp.ValidateTemplateImportResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)

		if !resp.ActivationReady {
			t.Errorf("P12 expected ActivationReady=true, got false: blockers=%v", resp.ActivationBlockers)
		}
	})

	// Assert no audit log was written across all positive tests
	if len(auditSpy.events) != 0 {
		t.Errorf("IMPORT_AUDIT_EVENT_WRITTEN = true (%d events), want 0", len(auditSpy.events))
	}
}

// ─── NEGATIVE FILE & TRANSPORT TESTS (N1 — N10) ───────────────────────────────

func TestCmsValidateTemplateImport_NegativeTransport(t *testing.T) {
	handler, _ := setupImportTestServer()

	t.Run("N1: no file field returns 400 Bad Request", func(t *testing.T) {
		var b bytes.Buffer
		w := multipart.NewWriter(&b)
		_ = w.WriteField("other_field", "value")
		_ = w.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/v1/platform/cms/templates/import/validate", &b)
		req.Header.Set("Content-Type", w.FormDataContentType())
		req.Header.Set("Authorization", "Bearer valid_atk")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("N1 code = %d, want 400", rec.Code)
		}
	})

	t.Run("N2: multiple file parts returns 400 Bad Request", func(t *testing.T) {
		var b bytes.Buffer
		w := multipart.NewWriter(&b)
		p1, _ := w.CreateFormFile("file", "f1.json")
		p1.Write([]byte(`{"schema_version":"1.0"}`))
		p2, _ := w.CreateFormFile("file", "f2.json")
		p2.Write([]byte(`{"schema_version":"1.0"}`))
		_ = w.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/v1/platform/cms/templates/import/validate", &b)
		req.Header.Set("Content-Type", w.FormDataContentType())
		req.Header.Set("Authorization", "Bearer valid_atk")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("N2 code = %d, want 400", rec.Code)
		}
	})

	t.Run("N3: zero-byte empty file returns 400 Bad Request", func(t *testing.T) {
		req := makeMultipartFileReq("file", "empty.json", []byte{})
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("N3 code = %d, want 400", rec.Code)
		}
	})

	t.Run("N4: non-json extension returns 415 Unsupported Media Type", func(t *testing.T) {
		req := makeMultipartFileReq("file", "template.txt", []byte(`{"schema_version":"1.0"}`))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnsupportedMediaType {
			t.Errorf("N4 code = %d, want 415", rec.Code)
		}
	})

	t.Run("N5: clearly binary file (zip header) returns 415", func(t *testing.T) {
		zipBytes := []byte("PK\x03\x04\x14\x00\x00\x00\x08\x00fake zip content")
		req := makeMultipartFileReq("file", "malicious.json", zipBytes)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnsupportedMediaType {
			t.Errorf("N5 code = %d, want 415", rec.Code)
		}
	})

	t.Run("N6: malformed multipart body returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/platform/cms/templates/import/validate", strings.NewReader("not a multipart body"))
		req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary123")
		req.Header.Set("Authorization", "Bearer valid_atk")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("N6 code = %d, want 400", rec.Code)
		}
	})

	t.Run("N7: boundary size test (2097151 allowed, 2097152 allowed, 2097153 rejected 413)", func(t *testing.T) {
		// Helper to create valid JSON with exact size
		makeExactSizeJSON := func(totalBytes int) []byte {
			prefix := []byte(`{"schema_version":"1.0","template":{"name":"T","template_category":"periodic","deadline_rule":"T+5","description":"`)
			suffix := []byte(`"}}`)
			paddingLen := totalBytes - len(prefix) - len(suffix)
			if paddingLen < 0 {
				panic("totalBytes too small")
			}
			padding := bytes.Repeat([]byte("A"), paddingLen)
			result := append(prefix, padding...)
			return append(result, suffix...)
		}

		// 1. 2,097,151 bytes -> allowed
		b2097151 := makeExactSizeJSON(2097151)
		req1 := makeMultipartFileReq("file", "b1.json", b2097151)
		rec1 := httptest.NewRecorder()
		handler.ServeHTTP(rec1, req1)
		if rec1.Code == http.StatusRequestEntityTooLarge {
			t.Errorf("2,097,151 bytes got 413, expected allowed")
		}

		// 2. 2,097,152 bytes (exactly 2 MiB) -> allowed
		b2097152 := makeExactSizeJSON(2097152)
		req2 := makeMultipartFileReq("file", "b2.json", b2097152)
		rec2 := httptest.NewRecorder()
		handler.ServeHTTP(rec2, req2)
		if rec2.Code == http.StatusRequestEntityTooLarge {
			t.Errorf("2,097,152 bytes got 413, expected allowed")
		}

		// 3. 2,097,153 bytes (2 MiB + 1 byte) -> 413 Payload Too Large
		b2097153 := makeExactSizeJSON(2097153)
		req3 := makeMultipartFileReq("file", "b3.json", b2097153)
		rec3 := httptest.NewRecorder()
		handler.ServeHTTP(rec3, req3)
		if rec3.Code != http.StatusRequestEntityTooLarge {
			t.Errorf("2,097,153 bytes code = %d, want 413", rec3.Code)
		}
	})

	t.Run("N8: invalid JSON syntax returns parse_valid=false", func(t *testing.T) {
		req := makeMultipartFileReq("file", "invalid_syntax.json", []byte(`{"schema_version":"1.0", "template": { broken json`))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("N8 status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		var resp disclosureapp.ValidateTemplateImportResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)

		if resp.ParseValid {
			t.Error("N8 expected ParseValid=false, got true")
		}
	})

	t.Run("N9: multiple JSON documents concatenated rejected", func(t *testing.T) {
		concat := []byte(`{"schema_version":"1.0","template":{"name":"T1","template_category":"periodic","deadline_rule":"T+5"}}{"schema_version":"1.0","template":{"name":"T2","template_category":"periodic","deadline_rule":"T+5"}}`)
		req := makeMultipartFileReq("file", "concat.json", concat)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		var resp disclosureapp.ValidateTemplateImportResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)

		if resp.ParseValid {
			t.Error("N9 expected ParseValid=false, got true")
		}
		found := false
		for _, e := range resp.Errors {
			if e.Code == "MULTIPLE_JSON_VALUES_REJECTED" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("N9 expected MULTIPLE_JSON_VALUES_REJECTED, got %v", resp.Errors)
		}
	})

	t.Run("N10: unsupported schema_version returns 422", func(t *testing.T) {
		req := makeMultipartFileReq("file", "v2.json", []byte(`{"schema_version":"9.9","template":{"name":"T","template_category":"periodic","deadline_rule":"T+5"}}`))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("N10 code = %d, want 422", rec.Code)
		}
	})

	t.Run("Raw JSON Content-Type rejected with 415", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/platform/cms/templates/import/validate", strings.NewReader(`{"schema_version":"1.0"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer valid_atk")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnsupportedMediaType {
			t.Errorf("raw application/json code = %d, want 415", rec.Code)
		}
	})
}

// ─── SECURITY & STRICT SCHEMA TESTS ──────────────────────────────────────────

func TestCmsValidateTemplateImport_SecurityAndStrictSchema(t *testing.T) {
	handler, _ := setupImportTestServer()

	t.Run("unknown top-level field rejected", func(t *testing.T) {
		payload := `{"schema_version":"1.0","malicious_field":"hack","template":{"name":"T","template_category":"periodic","deadline_rule":"T+5"}}`
		req := makeMultipartFileReq("file", "unknown.json", []byte(payload))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		var resp disclosureapp.ValidateTemplateImportResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp.ParseValid {
			t.Error("expected ParseValid=false for unknown top-level field")
		}
	})

	t.Run("runtime forbidden field is_released rejected", func(t *testing.T) {
		payload := `{"schema_version":"1.0","template":{"name":"T","template_category":"periodic","deadline_rule":"T+5","is_released":true}}`
		req := makeMultipartFileReq("file", "forbidden.json", []byte(payload))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		var resp disclosureapp.ValidateTemplateImportResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp.ParseValid {
			t.Error("expected ParseValid=false for runtime field is_released")
		}
	})

	t.Run("runtime forbidden field resolved_due_at rejected", func(t *testing.T) {
		payload := `{"schema_version":"1.0","template":{"name":"T","template_category":"periodic","deadline_rule":"T+5","resolved_due_at":"2026-09-08"}}`
		req := makeMultipartFileReq("file", "forbidden.json", []byte(payload))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		var resp disclosureapp.ValidateTemplateImportResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp.ParseValid {
			t.Error("expected ParseValid=false for runtime field resolved_due_at")
		}
	})

	t.Run("suspicious filename with traversal path handled safely in memory", func(t *testing.T) {
		payload := `{"schema_version":"1.0","template":{"name":"T","template_category":"periodic","deadline_rule":"T+5"}}`
		req := makeMultipartFileReq("file", "../../../../../etc/passwd.json", []byte(payload))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for in-memory stream of filename with dots, got %d", rec.Code)
		}
	})

	t.Run("HTML/script payload in description preserved safely without injection", func(t *testing.T) {
		payload := `{"schema_version":"1.0","template":{"name":"T","template_category":"periodic","deadline_rule":"T+5","description":"<script>alert(1)</script>"}}`
		req := makeMultipartFileReq("file", "xss.json", []byte(payload))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}
		var resp disclosureapp.ValidateTemplateImportResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp.Preview.NormalizedTemplate.Description != "<script>alert(1)</script>" {
			t.Errorf("description = %q", resp.Preview.NormalizedTemplate.Description)
		}
	})
}

// ---------------------------------------------------------------------------
// Confirm Template Import HTTP Handler Tests (Phase C)
// ---------------------------------------------------------------------------

func TestCmsConfirmTemplateImport_E2EAndHandlerMatrix(t *testing.T) {
	handler, auditSpy := setupImportTestServer()

	// 1. Validate real file to get real token and normalized template
	validPayload := `{
		"schema_version": "1.0",
		"template": {
			"type_id": "source-periodic-template",
			"name": "Báo cáo Tài chính Quý E2E",
			"template_category": "periodic",
			"periodicity": "quarterly",
			"deadline_rule": "T+15",
			"group_id": "group-001",
			"display_group_codes": ["display_groups_003"],
			"deadline_config": {
				"frequency_unit": "QUARTERLY",
				"cycle_anchor_day": 1,
				"cycle_anchor_weekday": "monday",
				"month_in_quarter": 3,
				"applicable_from_mode": "NEXT_SLOT",
				"applicable_to": "2029-12-31",
				"duration_type": "CALENDAR_DAYS",
				"deadline_days": 15
			},
			"workflow": {
				"steps": [
					{
						"stage": "Lập báo cáo",
						"department_id": "dept-001",
						"department_name": "Phòng Kế toán",
						"assignee_role_ids": ["creator"],
						"processing_days": 10,
						"display_order": 1,
						"documents": [
							{
								"name": "Bảng cân đối",
								"required": true,
								"template_file_name": "can_doi.xlsx"
							}
						]
					}
				]
			}
		}
	}`

	valReq := makeMultipartFileReq("file", "valid_e2e.json", []byte(validPayload))
	valRec := httptest.NewRecorder()
	handler.ServeHTTP(valRec, valReq)

	if valRec.Code != http.StatusOK {
		t.Fatalf("Validate failed with status %d: %s", valRec.Code, valRec.Body.String())
	}

	var valResp disclosureapp.ValidateTemplateImportResponse
	if err := json.Unmarshal(valRec.Body.Bytes(), &valResp); err != nil {
		t.Fatalf("unmarshal validate response failed: %v", err)
	}
	if !valResp.CanConfirm || valResp.ValidationToken == "" {
		t.Fatalf("expected CanConfirm=true with valid token, got CanConfirm=%v, token=%q", valResp.CanConfirm, valResp.ValidationToken)
	}

	validToken := valResp.ValidationToken
	normalizedTemplate := *valResp.Preview.NormalizedTemplate

	// 2. Happy Path E2E Confirm -> 201 Created
	t.Run("H1_e2e_valid_token_confirm_creates_draft_v1", func(t *testing.T) {
		confirmBody := disclosureapp.ConfirmTemplateImportRequest{
			ValidationToken:    validToken,
			TargetTypeID:       "e2e-import-target-1",
			TargetName:         normalizedTemplate.Name,
			NormalizedTemplate: normalizedTemplate,
		}
		bodyBytes, _ := json.Marshal(confirmBody)
		req := httptest.NewRequest("POST", "/api/v1/platform/cms/templates/import/confirm", bytes.NewReader(bodyBytes))
		req.Header.Set("Authorization", "Bearer valid-token")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp disclosureapp.ConfirmTemplateImportResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal confirm response failed: %v", err)
		}
		if resp.TypeID != "e2e-import-target-1" {
			t.Errorf("TypeID = %q, want e2e-import-target-1", resp.TypeID)
		}
		if resp.VersionNo != 1 {
			t.Errorf("VersionNo = %d, want 1", resp.VersionNo)
		}
		if resp.IsActive != false {
			t.Errorf("IsActive = %v, want false", resp.IsActive)
		}
		if resp.IsReleased != false {
			t.Errorf("IsReleased = %v, want false", resp.IsReleased)
		}
		if resp.PortalState != "not_active" {
			t.Errorf("PortalState = %q, want not_active", resp.PortalState)
		}
		if resp.RootStatus != "active" {
			t.Errorf("RootStatus = %q, want active", resp.RootStatus)
		}

		// Verify audit event logged exactly once
		var importAudit *auditapp.AppendAuditLogRequest
		for i := range auditSpy.events {
			if auditSpy.events[i].Action == "disclosure.type.import" && auditSpy.events[i].ResourceID == "e2e-import-target-1" {
				importAudit = &auditSpy.events[i]
				break
			}
		}
		if importAudit == nil {
			t.Fatal("expected audit event for disclosure.type.import, found none")
		}
		if importAudit.ResourceType != "disclosure_type" {
			t.Errorf("audit resource_type = %q, want disclosure_type", importAudit.ResourceType)
		}
		meta := importAudit.Metadata
		if meta["creation_mode"] != "TEMPLATE_IMPORT" {
			t.Errorf("audit creation_mode = %v, want TEMPLATE_IMPORT", meta["creation_mode"])
		}
		if meta["version_no"] != 1 {
			t.Errorf("audit version_no = %v, want 1", meta["version_no"])
		}
		if meta["schema_version"] != disclosureapp.TemplateImportSchemaVersion {
			t.Errorf("audit schema_version = %v, want %s", meta["schema_version"], disclosureapp.TemplateImportSchemaVersion)
		}
	})

	// 3. Duplicate Replay -> 409 Conflict (no second audit event)
	t.Run("duplicate_replay_same_target_returns_409", func(t *testing.T) {
		initialAuditCount := len(auditSpy.events)
		confirmBody := disclosureapp.ConfirmTemplateImportRequest{
			ValidationToken:    validToken,
			TargetTypeID:       "e2e-import-target-1", // same target ID already created
			TargetName:         normalizedTemplate.Name,
			NormalizedTemplate: normalizedTemplate,
		}
		bodyBytes, _ := json.Marshal(confirmBody)
		req := httptest.NewRequest("POST", "/api/v1/platform/cms/templates/import/confirm", bytes.NewReader(bodyBytes))
		req.Header.Set("Authorization", "Bearer valid-token")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("expected 409 Conflict on duplicate replay, got %d: %s", rec.Code, rec.Body.String())
		}
		if len(auditSpy.events) != initialAuditCount {
			t.Errorf("duplicate replay must not create second audit event: count before=%d, after=%d", initialAuditCount, len(auditSpy.events))
		}
	})

	// 4. Malformed JSON Body -> 400 Bad Request
	t.Run("malformed_json_body_returns_400", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/platform/cms/templates/import/confirm", strings.NewReader("{not valid json"))
		req.Header.Set("Authorization", "Bearer valid-token")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request on malformed JSON, got %d", rec.Code)
		}
	})

	// 5. Auth Failure: missing token -> 401
	t.Run("missing_auth_header_returns_401", func(t *testing.T) {
		confirmBody := disclosureapp.ConfirmTemplateImportRequest{
			ValidationToken:    validToken,
			TargetTypeID:       "auth-fail-target",
			TargetName:         normalizedTemplate.Name,
			NormalizedTemplate: normalizedTemplate,
		}
		bodyBytes, _ := json.Marshal(confirmBody)
		req := httptest.NewRequest("POST", "/api/v1/platform/cms/templates/import/confirm", bytes.NewReader(bodyBytes))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized on missing auth header, got %d", rec.Code)
		}
	})

	// 6. Tampered Validation Token -> 422 Unprocessable Entity (INVALID_IMPORT_TOKEN)
	t.Run("tampered_validation_token_returns_422", func(t *testing.T) {
		confirmBody := disclosureapp.ConfirmTemplateImportRequest{
			ValidationToken:    validToken + "TAMPERED",
			TargetTypeID:       "tampered-tok-target",
			TargetName:         normalizedTemplate.Name,
			NormalizedTemplate: normalizedTemplate,
		}
		bodyBytes, _ := json.Marshal(confirmBody)
		req := httptest.NewRequest("POST", "/api/v1/platform/cms/templates/import/confirm", bytes.NewReader(bodyBytes))
		req.Header.Set("Authorization", "Bearer valid-token")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected 422 Unprocessable Entity on tampered validation token, got %d", rec.Code)
		}
	})

	// 7. Payload Hash Mismatch -> 422 Unprocessable Entity (INVALID_IMPORT_TOKEN)
	t.Run("payload_hash_mismatch_returns_422", func(t *testing.T) {
		tamperedTemplate := normalizedTemplate
		tamperedTemplate.Description = "Mô tả bị sửa lén sau validate"

		confirmBody := disclosureapp.ConfirmTemplateImportRequest{
			ValidationToken:    validToken,
			TargetTypeID:       "hash-mismatch-target",
			TargetName:         tamperedTemplate.Name,
			NormalizedTemplate: tamperedTemplate,
		}
		bodyBytes, _ := json.Marshal(confirmBody)
		req := httptest.NewRequest("POST", "/api/v1/platform/cms/templates/import/confirm", bytes.NewReader(bodyBytes))
		req.Header.Set("Authorization", "Bearer valid-token")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected 422 Unprocessable Entity on payload hash mismatch, got %d", rec.Code)
		}
	})

	// 8. Target Name Mismatch -> 400 Bad Request
	t.Run("target_name_mismatch_returns_400", func(t *testing.T) {
		confirmBody := disclosureapp.ConfirmTemplateImportRequest{
			ValidationToken:    validToken,
			TargetTypeID:       "target-name-diff-target",
			TargetName:         "Tên khác với template",
			NormalizedTemplate: normalizedTemplate,
		}
		bodyBytes, _ := json.Marshal(confirmBody)
		req := httptest.NewRequest("POST", "/api/v1/platform/cms/templates/import/confirm", bytes.NewReader(bodyBytes))
		req.Header.Set("Authorization", "Bearer valid-token")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request on target_name mismatch, got %d", rec.Code)
		}
	})
}

func TestCmsDownloadTemplateImportExample(t *testing.T) {
	handler, _ := setupImportTestServer()

	t.Run("authorized_download_200_attachment", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/platform/cms/templates/import/example", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		ct := rec.Header().Get("Content-Type")
		if !strings.HasPrefix(ct, "application/json") {
			t.Fatalf("Content-Type=%q", ct)
		}
		cd := rec.Header().Get("Content-Disposition")
		wantCD := `attachment; filename="cobo-template-import-example-v1.0.json"`
		if cd != wantCD {
			t.Fatalf("Content-Disposition=%q want %q", cd, wantCD)
		}
		var env map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
			t.Fatalf("payload not JSON: %v", err)
		}
		if env["schema_version"] != "1.0" {
			t.Fatalf("schema_version=%v", env["schema_version"])
		}
	})

	t.Run("missing_token_unauthorized", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/platform/cms/templates/import/example", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status=%d want 401", rec.Code)
		}
	})

	t.Run("forbidden_without_write_permission", func(t *testing.T) {
		_ = os.Setenv("CMS_TEMPLATE_IMPORT_SIGNING_SECRET", "test-cms-template-import-secret-for-suite")
		repo := inmemory.NewRepository()
		auth := &cmsAuthMock{perms: []string{"platform.cms.view"}}
		svc := disclosureapp.NewService(repo, auth, idgen.UUIDv7Generator{})
		inspector := testTokenInspector{sub: "admin-platform-1", membershipID: "mem-1", companyID: "cobo-platform"}
		h := disclosurehttp.NewHandler(svc, inspector, nil, &testAuditSpy{})
		mux := http.NewServeMux()
		h.Register(mux)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/platform/cms/templates/import/example", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status=%d want 403 body=%s", rec.Code, rec.Body.String())
		}
	})
}

