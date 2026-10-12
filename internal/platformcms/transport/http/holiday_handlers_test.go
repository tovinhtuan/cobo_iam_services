package http

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	holidayapp "github.com/cobo/cobo_iam_services/internal/holiday/app"
)

// Contract: cobo_web_design/docs/ai-cache/cms-holiday-calendar-editor-2026-10-12/02-contract.md

type fakeHolidaySvc struct {
	previewRows  []holidayapp.HolidayInputRow
	previewXLSX  bool
	replace      *holidayapp.ReplaceRequest
	deleteReq    *holidayapp.DeleteRequest
	exportSource string
}

func (f *fakeHolidaySvc) Preview(_ context.Context, year int, name string, raw []byte) (*holidayapp.PreviewResult, error) {
	f.previewXLSX = true
	return &holidayapp.PreviewResult{CalendarYear: year, SourceName: name}, nil
}
func (f *fakeHolidaySvc) PreviewRows(_ context.Context, year int, rows []holidayapp.HolidayInputRow) (*holidayapp.PreviewResult, error) {
	f.previewRows = rows
	return &holidayapp.PreviewResult{CalendarYear: year, TotalAccepted: len(rows)}, nil
}
func (f *fakeHolidaySvc) Replace(_ context.Context, req holidayapp.ReplaceRequest) (*holidayapp.ReplaceResult, error) {
	f.replace = &req
	return &holidayapp.ReplaceResult{CalendarYear: req.Year, Status: "replaced", Revision: "rev"}, nil
}
func (f *fakeHolidaySvc) Delete(_ context.Context, req holidayapp.DeleteRequest) (*holidayapp.DeleteResult, error) {
	f.deleteReq = &req
	return &holidayapp.DeleteResult{CalendarYear: req.Year, Deleted: true, Fallback: "default"}, nil
}
func (f *fakeHolidaySvc) Get(context.Context, int) (*holidayapp.CalendarView, error) {
	return &holidayapp.CalendarView{Revision: "rev", Source: "db"}, nil
}
func (f *fakeHolidaySvc) Default(_ context.Context, year int) (*holidayapp.DefaultCalendarView, error) {
	return &holidayapp.DefaultCalendarView{CalendarYear: year, Source: "default"}, nil
}
func (f *fakeHolidaySvc) ExportXLSX(_ context.Context, _ int, source string) ([]byte, error) {
	f.exportSource = source
	return []byte("PK-xlsx"), nil
}

func newHolidayHandler(perms []string) (*Handler, *fakeHolidaySvc) {
	svc := &fakeHolidaySvc{}
	return &Handler{inspector: listedFakeInspector{}, authorizer: listedFakeAuthorizer{perms: perms}, holidaySvc: svc, metrics: newCMSMetrics()}, svc
}

func doHoliday(h *Handler, method, path, contentType string, body []byte) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	w := httptest.NewRecorder()
	mux := http.NewServeMux()
	h.Register(mux)
	mux.ServeHTTP(w, req)
	return w
}

func multipartBody(t *testing.T, fields map[string]string) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "lich.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write([]byte("xlsx-bytes"))
	for k, v := range fields {
		_ = mw.WriteField(k, v)
	}
	_ = mw.Close()
	return buf.Bytes(), mw.FormDataContentType()
}

var (
	holidayViewer = []string{"platform.cms.view"}
	holidayEditor = []string{"platform.cms.view", "system.settings"}
)

const holidayPath = "/api/v1/platform/cms/holiday-calendars/2026"

func TestHolidayPreview_RoutesByContentType(t *testing.T) {
	h, svc := newHolidayHandler(holidayViewer)
	w := doHoliday(h, http.MethodPost, holidayPath+"/preview", "application/json; charset=utf-8",
		[]byte(`{"dates":[{"date":"2026-02-16","name":"Tết","day_type":"PUBLIC_HOLIDAY"}]}`))
	if w.Code != http.StatusOK || len(svc.previewRows) != 1 || svc.previewRows[0].Name != "Tết" {
		t.Fatalf("json preview: %d %s rows=%v", w.Code, w.Body.String(), svc.previewRows)
	}
	body, ct := multipartBody(t, nil)
	w = doHoliday(h, http.MethodPost, holidayPath+"/preview", ct, body)
	if w.Code != http.StatusOK || !svc.previewXLSX {
		t.Fatalf("multipart preview: %d %s", w.Code, w.Body.String())
	}
	w = doHoliday(h, http.MethodPost, holidayPath+"/preview", "text/plain", []byte("x"))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("text/plain preview: %d", w.Code)
	}
}

func TestHolidayEditorBody_FieldErrorsAndLimit(t *testing.T) {
	h, _ := newHolidayHandler(holidayViewer)
	for _, body := range []string{`{}`, `{"dates":null}`, `{"dates":"x"}`} {
		w := doHoliday(h, http.MethodPost, holidayPath+"/preview", "application/json", []byte(body))
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), `"dates"`) {
			t.Fatalf("%s: %d %s", body, w.Code, w.Body.String())
		}
	}
	w := doHoliday(h, http.MethodPost, holidayPath+"/preview", "application/json", []byte(`{"dates":[`))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("broken json: %d", w.Code)
	}
	big := `{"dates":[],"description":"` + strings.Repeat("a", holidayJSONBodyLimit) + `"}`
	w = doHoliday(h, http.MethodPost, holidayPath+"/preview", "application/json", []byte(big))
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized body: %d", w.Code)
	}
}

func TestHolidayWrites_RequireWritePermission(t *testing.T) {
	h, svc := newHolidayHandler(holidayViewer)
	w := doHoliday(h, http.MethodPut, holidayPath, "application/json", []byte(`{"dates":[{"date":"2026-01-01"}]}`))
	if w.Code != http.StatusForbidden || svc.replace != nil {
		t.Fatalf("viewer PUT: %d", w.Code)
	}
	w = doHoliday(h, http.MethodDelete, holidayPath, "", nil)
	if w.Code != http.StatusForbidden || svc.deleteReq != nil {
		t.Fatalf("viewer DELETE: %d", w.Code)
	}
	// Reads stay available to viewers.
	for _, p := range []string{holidayPath, holidayPath + "/default", holidayPath + "/export.xlsx"} {
		if w := doHoliday(h, http.MethodGet, p, "", nil); w.Code != http.StatusOK {
			t.Fatalf("viewer GET %s: %d", p, w.Code)
		}
	}
}

func TestHolidayReplace_JSONPassesRevisionAndActor(t *testing.T) {
	h, svc := newHolidayHandler(holidayEditor)
	w := doHoliday(h, http.MethodPut, holidayPath, "application/json",
		[]byte(`{"dates":[{"date":"2026-01-01"}],"description":" TB 1 ","expected_revision":""}`))
	if w.Code != http.StatusOK {
		t.Fatalf("PUT: %d %s", w.Code, w.Body.String())
	}
	r := svc.replace
	if r == nil || !r.FromEditor || r.Description != "TB 1" || r.ExpectedRevision == nil || *r.ExpectedRevision != "" {
		t.Fatalf("replace request = %+v", r)
	}
	if r.Actor.UserID != "u_cms" || r.Actor.CompanyID != "c1" || r.Actor.MembershipID != "m1" {
		t.Fatalf("actor = %+v", r.Actor)
	}
	if !strings.Contains(w.Body.String(), `"revision":"rev"`) {
		t.Fatalf("body: %s", w.Body.String())
	}

	// No expected_revision → no check (old clients).
	_ = doHoliday(h, http.MethodPut, holidayPath, "application/json", []byte(`{"dates":[{"date":"2026-01-01"}]}`))
	if svc.replace.ExpectedRevision != nil {
		t.Fatal("absent expected_revision must not be checked")
	}
}

func TestHolidayReplace_MultipartOptionalRevision(t *testing.T) {
	h, svc := newHolidayHandler(holidayEditor)
	body, ct := multipartBody(t, map[string]string{"description": "d"})
	if w := doHoliday(h, http.MethodPut, holidayPath, ct, body); w.Code != http.StatusOK {
		t.Fatalf("PUT multipart: %d %s", w.Code, w.Body.String())
	}
	if svc.replace.FromEditor || svc.replace.ExpectedRevision != nil || svc.replace.SourceName != "lich.xlsx" {
		t.Fatalf("multipart without revision = %+v", svc.replace)
	}
	body, ct = multipartBody(t, map[string]string{"expected_revision": "abc"})
	_ = doHoliday(h, http.MethodPut, holidayPath, ct, body)
	if svc.replace.ExpectedRevision == nil || *svc.replace.ExpectedRevision != "abc" {
		t.Fatalf("multipart revision = %+v", svc.replace.ExpectedRevision)
	}
}

func TestHolidayDelete_OptionalBody(t *testing.T) {
	h, svc := newHolidayHandler(holidayEditor)
	if w := doHoliday(h, http.MethodDelete, holidayPath, "", nil); w.Code != http.StatusOK || svc.deleteReq.ExpectedRevision != nil {
		t.Fatalf("DELETE no body: %d", w.Code)
	}
	w := doHoliday(h, http.MethodDelete, holidayPath, "application/json", []byte(`{"expected_revision":"r1","reason":"phát hành nhầm"}`))
	if w.Code != http.StatusOK || *svc.deleteReq.ExpectedRevision != "r1" || svc.deleteReq.Reason != "phát hành nhầm" {
		t.Fatalf("DELETE body: %d %+v", w.Code, svc.deleteReq)
	}
	if !strings.Contains(w.Body.String(), `"fallback":"default"`) {
		t.Fatalf("body: %s", w.Body.String())
	}
}

func TestHolidayExport_Headers(t *testing.T) {
	h, svc := newHolidayHandler(holidayViewer)
	w := doHoliday(h, http.MethodGet, holidayPath+"/export.xlsx?source=default", "", nil)
	if w.Code != http.StatusOK || svc.exportSource != "default" {
		t.Fatalf("export: %d source=%q", w.Code, svc.exportSource)
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.Contains(cd, `lich_khong_giao_dich_2026.xlsx`) {
		t.Fatalf("Content-Disposition = %q", cd)
	}
	if !strings.HasPrefix(w.Header().Get("Content-Type"), "application/vnd.openxmlformats") {
		t.Fatalf("Content-Type = %q", w.Header().Get("Content-Type"))
	}
}

func TestHolidayAuditActionsAreFilterable(t *testing.T) {
	for _, a := range []string{holidayapp.AuditActionReplace, holidayapp.AuditActionDelete} {
		if _, ok := cmsKnownActions[a]; !ok {
			t.Fatalf("%s missing from cmsKnownActions", a)
		}
	}
}

// A company (enterprise) admin holds rbac.manage but never platform.cms.view: every holiday route,
// read or write, must reject them before the service runs.
func TestHolidayRoutes_RejectCompanyAdminWithoutPlatformView(t *testing.T) {
	routes := []struct{ method, path, ct, body string }{
		{http.MethodGet, holidayPath, "", ""},
		{http.MethodGet, holidayPath + "/default", "", ""},
		{http.MethodGet, holidayPath + "/export.xlsx", "", ""},
		{http.MethodPost, holidayPath + "/preview", "application/json", `{"dates":[{"date":"2026-01-01"}]}`},
		{http.MethodPut, holidayPath, "application/json", `{"dates":[{"date":"2026-01-01"}]}`},
		{http.MethodDelete, holidayPath, "", ""},
	}
	for _, perms := range [][]string{{"rbac.manage"}, {"rbac.manage", "system.settings"}} {
		h, svc := newHolidayHandler(perms)
		for _, rt := range routes {
			w := doHoliday(h, rt.method, rt.path, rt.ct, []byte(rt.body))
			if w.Code != http.StatusForbidden {
				t.Fatalf("%v %s %s: %d", perms, rt.method, rt.path, w.Code)
			}
		}
		if svc.replace != nil || svc.deleteReq != nil || svc.previewRows != nil || svc.exportSource != "" {
			t.Fatalf("%v: service called despite 403", perms)
		}
	}
	// Platform operator with rbac.manage may write.
	h, svc := newHolidayHandler([]string{"platform.cms.view", "rbac.manage"})
	if w := doHoliday(h, http.MethodPut, holidayPath, "application/json", []byte(`{"dates":[{"date":"2026-01-01"}]}`)); w.Code != http.StatusOK || svc.replace == nil {
		t.Fatalf("operator PUT: %d", w.Code)
	}
}

func TestHolidayUpload_OversizedMultipartIs413(t *testing.T) {
	h, svc := newHolidayHandler(holidayViewer)
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "big.xlsx")
	_, _ = fw.Write(bytes.Repeat([]byte("x"), holidayUploadLimit+holidayMultipartOverhead+1))
	_ = mw.Close()
	w := doHoliday(h, http.MethodPost, holidayPath+"/preview", mw.FormDataContentType(), buf.Bytes())
	if w.Code != http.StatusRequestEntityTooLarge || svc.previewXLSX {
		t.Fatalf("oversized multipart: %d", w.Code)
	}
}
