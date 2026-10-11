package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

// capturingUpsertService records the UpsertTypeVersion request decoded by the handler.
type capturingUpsertService struct {
	fakeDisclosureHandlerService
	upsertCalls int
	lastUpsert  disclosureapp.UpsertTypeVersionRequest
	archiveReq  *disclosureapp.CmsArchiveTemplateRequest
}

func (f *capturingUpsertService) UpsertTypeVersion(_ context.Context, req disclosureapp.UpsertTypeVersionRequest) (*disclosureapp.UpsertTypeVersionResponse, error) {
	f.upsertCalls++
	f.lastUpsert = req
	return &disclosureapp.UpsertTypeVersionResponse{TypeID: req.TypeID, VersionNo: 1}, nil
}

func (f *capturingUpsertService) CmsArchiveTemplate(_ context.Context, req disclosureapp.CmsArchiveTemplateRequest) (*disclosureapp.CmsArchiveTemplateResponse, error) {
	f.archiveReq = &req
	return &disclosureapp.CmsArchiveTemplateResponse{TypeID: req.TypeID}, nil
}

func doUpsert(t *testing.T, svc *capturingUpsertService, body string) *httptest.ResponseRecorder {
	t.Helper()
	h := NewHandler(svc, fakeDisclosureInspector{}, nil, &fakeDisclosureAuditService{})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/disclosure-types/type-new", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer atk_test")
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("type_id", "type-new")
	rr := httptest.NewRecorder()
	h.upsertTypeVersion(rr, req)
	return rr
}

func TestUpsertTypeVersion_CreateOnlyFlagFromBody(t *testing.T) {
	svc := &capturingUpsertService{}
	rr := doUpsert(t, svc, `{"name":"A","group_id":"g","create_only":true}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !svc.lastUpsert.CreateOnly {
		t.Fatal("expected CreateOnly=true when body has create_only:true")
	}
}

func TestUpsertTypeVersion_CreateOnlyDefaultsFalse(t *testing.T) {
	svc := &capturingUpsertService{}
	for _, body := range []string{`{"name":"A","group_id":"g"}`, `{"name":"A","group_id":"g","create_only":false}`} {
		rr := doUpsert(t, svc, body)
		if rr.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
		}
		if svc.lastUpsert.CreateOnly {
			t.Fatalf("expected CreateOnly=false for body %s", body)
		}
	}
}

func TestUpsertTypeVersion_NonBooleanCreateOnlyRejected(t *testing.T) {
	svc := &capturingUpsertService{}
	for _, body := range []string{`{"name":"A","create_only":"true"}`, `{"name":"A","create_only":1}`} {
		rr := doUpsert(t, svc, body)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("body %s: status=%d want 400", body, rr.Code)
		}
	}
	if svc.upsertCalls != 0 {
		t.Fatal("must not fail open into a plain upsert")
	}
}

func TestUpsertTypeVersion_MarksOmittedNarrativeFields(t *testing.T) {
	svc := &capturingUpsertService{}
	rr := doUpsert(t, svc, `{"name":"A","group_id":"g","special_cases":"","beneficiaries":"x"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	got := append([]string(nil), svc.lastUpsert.NarrativeFieldsOmitted...)
	sort.Strings(got)
	want := []string{"applicability", "implementation_notes", "receiving_authorities"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("NarrativeFieldsOmitted=%v want %v", got, want)
	}
}

func TestUpsertTypeVersion_MalformedJSONReturns400(t *testing.T) {
	svc := &capturingUpsertService{}
	rr := doUpsert(t, svc, `{"name":`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400; body=%s", rr.Code, rr.Body.String())
	}
	if svc.upsertCalls != 0 {
		t.Fatal("service must not be called on malformed JSON")
	}
}

func TestUpsertTypeVersion_OversizedBodyReturns413(t *testing.T) {
	svc := &capturingUpsertService{}
	big := `{"name":"` + strings.Repeat("a", int(maxTemplateWriteBodyBytes)+1) + `"}`
	rr := doUpsert(t, svc, big)
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d want 413", rr.Code)
	}
	if svc.upsertCalls != 0 {
		t.Fatal("service must not be called on oversized body")
	}
}

func TestCmsArchiveTemplate_EmptyBodyOKMalformedBodyRejected(t *testing.T) {
	h := func(svc *capturingUpsertService, body string) *httptest.ResponseRecorder {
		handler := NewHandler(svc, fakeDisclosureInspector{}, nil, &fakeDisclosureAuditService{})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/platform/cms/templates/type-1/archive", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer atk_test")
		req.SetPathValue("type_id", "type-1")
		rr := httptest.NewRecorder()
		handler.cmsArchiveTemplate(rr, req)
		return rr
	}
	okSvc := &capturingUpsertService{}
	if rr := h(okSvc, ""); rr.Code != http.StatusOK {
		t.Fatalf("empty body status=%d body=%s", rr.Code, rr.Body.String())
	}
	if rr := h(&capturingUpsertService{}, " 	 "); rr.Code != http.StatusOK {
		t.Fatalf("whitespace body status=%d body=%s", rr.Code, rr.Body.String())
	}
	if okSvc.archiveReq == nil {
		t.Fatal("expected archive service call for empty body")
	}
	badSvc := &capturingUpsertService{}
	if rr := h(badSvc, `{"reason":`); rr.Code != http.StatusBadRequest {
		t.Fatalf("malformed body status=%d want 400", rr.Code)
	}
	if badSvc.archiveReq != nil {
		t.Fatal("archive must not run on malformed body")
	}
}
