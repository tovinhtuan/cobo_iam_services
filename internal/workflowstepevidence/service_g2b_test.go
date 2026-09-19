package workflowstepevidence_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
	wff "github.com/cobo/cobo_iam_services/internal/workflowfulfillment"
	wffmem "github.com/cobo/cobo_iam_services/internal/workflowfulfillment/memory"
	wse "github.com/cobo/cobo_iam_services/internal/workflowstepevidence"
	"github.com/cobo/cobo_iam_services/internal/workflowstepevidence/memory"
	wsehttp "github.com/cobo/cobo_iam_services/internal/workflowstepevidence/transport/http"
)

type svcMemStorage struct {
	mu        sync.Mutex
	data      map[string][]byte
	failWrite bool
	failRead  bool
}

func newSvcMemStorage() *svcMemStorage { return &svcMemStorage{data: map[string][]byte{}} }

func (s *svcMemStorage) Write(objectKey string, body io.Reader) (int64, error) {
	if s.failWrite {
		return 0, errors.New("disk write failed")
	}
	b, err := io.ReadAll(body)
	if err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[objectKey] = b
	return int64(len(b)), nil
}

func (s *svcMemStorage) Read(objectKey string) ([]byte, error) {
	if s.failRead {
		return nil, errors.New("disk read failed")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.data[objectKey]
	if !ok {
		return nil, errors.New("missing")
	}
	return append([]byte(nil), b...), nil
}

func (s *svcMemStorage) Delete(objectKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, objectKey)
	return nil
}

func (s *svcMemStorage) Exists(objectKey string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.data[objectKey]
	return ok
}

type fakeEvidenceDeadline struct {
	viewOK   bool
	mutateOK bool
	wf       wff.WorkflowContext
	states   map[string]wff.StepState
	company  string
}

func (f *fakeEvidenceDeadline) AuthorizeView(context.Context, wff.Subject) error {
	if !f.viewOK {
		return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "deadline.view permission required", nil)
	}
	return nil
}

func (f *fakeEvidenceDeadline) AuthorizeMutation(_ context.Context, sub wff.Subject, _ string) error {
	if !f.viewOK {
		return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "deadline.view permission required", nil)
	}
	if !f.mutateOK {
		return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "deadline.manage permission required", nil)
	}
	return nil
}

func (f *fakeEvidenceDeadline) LoadWorkflowForRecord(_ context.Context, sub wff.Subject, recordID string) (wff.WorkflowContext, error) {
	if f.company != "" && sub.CompanyID != f.company {
		return wff.WorkflowContext{}, perr.NewHTTPError(http.StatusForbidden, perr.CodeDataScopeDenied, "record outside data scope", nil)
	}
	wf := f.wf
	if wf.RecordID == "" {
		wf.RecordID = recordID
	}
	if wf.CompanyID == "" {
		wf.CompanyID = sub.CompanyID
	}
	return wf, nil
}

func (f *fakeEvidenceDeadline) ListStepStates(context.Context, string) (map[string]wff.StepState, error) {
	if f.states == nil {
		return map[string]wff.StepState{}, nil
	}
	return f.states, nil
}

func baseWF() wff.WorkflowContext {
	// Fixed T0 so authority checks stay stable when tests freeze `now` (e.g. G2C WithNow).
	// ProcessingDays keeps step-001 current for typical 2026 fixtures and sequential unlock.
	return wff.WorkflowContext{
		WorkflowInstanceID: "wi-1",
		CompanyID:          "c_001",
		RecordID:           "rec-1",
		T0Date:             time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		Timezone:           "Asia/Ho_Chi_Minh",
		SnapshotJSONSteps: []workflowapp.StepSnapshot{
			{StepCode: "step-001", StepID: "step-001", DisplayOrder: 1, ProcessingDays: 60},
			{StepCode: "step-002", StepID: "step-002", DisplayOrder: 2, ProcessingDays: 60},
		},
	}
}

func newTestService(t *testing.T, d *fakeEvidenceDeadline, store *svcMemStorage, repo *memory.Repository) *wse.Service {
	t.Helper()
	if d == nil {
		d = &fakeEvidenceDeadline{viewOK: true, mutateOK: true, wf: baseWF(), company: "c_001"}
	}
	if store == nil {
		store = newSvcMemStorage()
	}
	if repo == nil {
		repo = memory.NewRepository()
	}
	return wse.NewService(d, repo, store, nil)
}

func sub() wff.Subject {
	return wff.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c_001"}
}

func TestService_ZeroRequirementListUpload(t *testing.T) {
	svc := newTestService(t, nil, nil, nil)
	ctx := context.Background()
	list, err := svc.ListEvidenceFiles(ctx, sub(), "rec-1", "step-001")
	if err != nil {
		t.Fatal(err)
	}
	if list.Files == nil || len(list.Files) != 0 {
		t.Fatalf("files=%v", list.Files)
	}
	if !list.Capabilities.CanUpload {
		t.Fatal("can_upload want true for zero-requirement current step")
	}
	up, err := svc.UploadEvidenceFile(ctx, sub(), "rec-1", "step-001", "a.pdf", "application/pdf", bytes.NewReader([]byte("%PDF")), 4)
	if err != nil {
		t.Fatal(err)
	}
	if up.File.FileID == "" || strings.Contains(up.File.FileName, "storage") {
		t.Fatalf("%#v", up.File)
	}
	list, _ = svc.ListEvidenceFiles(ctx, sub(), "rec-1", "step-001")
	if len(list.Files) != 1 || list.Files[0].Capabilities.CanDelete != true {
		t.Fatalf("%#v", list.Files)
	}
	raw, _ := json.Marshal(list)
	if strings.Contains(string(raw), "storage_key") {
		t.Fatal("storage_key leaked")
	}
}

func TestService_ListExcludesDeletedSuperseded(t *testing.T) {
	store := newSvcMemStorage()
	repo := memory.NewRepository()
	svc := newTestService(t, nil, store, repo)
	ctx := context.Background()
	a, _ := svc.UploadEvidenceFile(ctx, sub(), "rec-1", "step-001", "a.pdf", "application/pdf", bytes.NewReader([]byte("aa")), 2)
	b, _ := svc.UploadEvidenceFile(ctx, sub(), "rec-1", "step-001", "b.pdf", "application/pdf", bytes.NewReader([]byte("bb")), 2)
	_ = svc.DeleteEvidenceFile(ctx, sub(), "rec-1", "step-001", a.File.FileID)
	_, _ = svc.ReplaceEvidenceFile(ctx, sub(), "rec-1", "step-001", b.File.FileID, "c.pdf", "application/pdf", bytes.NewReader([]byte("cc")), 2)
	list, _ := svc.ListEvidenceFiles(ctx, sub(), "rec-1", "step-001")
	if len(list.Files) != 1 || list.Files[0].FileName != "c.pdf" {
		t.Fatalf("%#v", list.Files)
	}
}

func TestService_DuplicateFilenameNoOverwrite(t *testing.T) {
	store := newSvcMemStorage()
	svc := newTestService(t, nil, store, nil)
	ctx := context.Background()
	a, _ := svc.UploadEvidenceFile(ctx, sub(), "rec-1", "step-001", "proof.pdf", "application/pdf", bytes.NewReader([]byte("1")), 1)
	b, _ := svc.UploadEvidenceFile(ctx, sub(), "rec-1", "step-001", "proof.pdf", "application/pdf", bytes.NewReader([]byte("2")), 1)
	if a.File.FileID == b.File.FileID {
		t.Fatal("ids must differ")
	}
	if len(store.data) != 2 {
		t.Fatalf("keys=%d", len(store.data))
	}
}

func TestService_FilePolicyAndLimit(t *testing.T) {
	svc := newTestService(t, nil, nil, nil)
	ctx := context.Background()
	_, err := svc.UploadEvidenceFile(ctx, sub(), "rec-1", "step-001", "x.exe", "application/octet-stream", bytes.NewReader([]byte("x")), 1)
	if err == nil {
		t.Fatal("exe must fail")
	}
	for i := 0; i < 10; i++ {
		name := "f" + string(rune('a'+i)) + ".pdf"
		if _, err := svc.UploadEvidenceFile(ctx, sub(), "rec-1", "step-001", name, "application/pdf", bytes.NewReader([]byte("x")), 1); err != nil {
			t.Fatalf("%d: %v", i, err)
		}
	}
	_, err = svc.UploadEvidenceFile(ctx, sub(), "rec-1", "step-001", "overflow.pdf", "application/pdf", bytes.NewReader([]byte("x")), 1)
	var he *perr.HTTPError
	if !errors.As(err, &he) || he.Code != perr.CodeWorkflowStepEvidenceFileLimitReached {
		t.Fatalf("limit: %v", err)
	}
}

func TestService_ReplaceAtLimitAndFailurePreservesOld(t *testing.T) {
	store := newSvcMemStorage()
	repo := memory.NewRepository()
	svc := newTestService(t, nil, store, repo)
	ctx := context.Background()
	var first *wse.UploadResult
	for i := 0; i < 10; i++ {
		name := "r" + string(rune('a'+i)) + ".pdf"
		up, err := svc.UploadEvidenceFile(ctx, sub(), "rec-1", "step-001", name, "application/pdf", bytes.NewReader([]byte("x")), 1)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = up
		}
	}
	repl, err := svc.ReplaceEvidenceFile(ctx, sub(), "rec-1", "step-001", first.File.FileID, "new.pdf", "application/pdf", bytes.NewReader([]byte("yy")), 2)
	if err != nil {
		t.Fatalf("replace at 10: %v", err)
	}
	list, _ := svc.ListEvidenceFiles(ctx, sub(), "rec-1", "step-001")
	if len(list.Files) != 10 {
		t.Fatalf("n=%d", len(list.Files))
	}
	old, _ := repo.GetByIDInContext(ctx, wse.OwnerContext{
		CompanyID: "c_001", DisclosureRecordID: "rec-1", WorkflowInstanceID: "wi-1", StepCode: "step-001",
	}, first.File.FileID)
	if old.LifecycleStatus != wse.LifecycleSuperseded {
		t.Fatalf("%#v", old)
	}
	_ = repl
	store.failWrite = true
	target := list.Files[1].FileID
	_, err = svc.ReplaceEvidenceFile(ctx, sub(), "rec-1", "step-001", target, "fail.pdf", "application/pdf", bytes.NewReader([]byte("z")), 1)
	if err == nil {
		t.Fatal("expected fail")
	}
	still, _ := repo.GetActiveByIDInContext(ctx, wse.OwnerContext{
		CompanyID: "c_001", DisclosureRecordID: "rec-1", WorkflowInstanceID: "wi-1", StepCode: "step-001",
	}, target)
	if still == nil {
		t.Fatal("old must remain ACTIVE")
	}
}

func TestService_DownloadDeleteCompletedViewOnly(t *testing.T) {
	store := newSvcMemStorage()
	repo := memory.NewRepository()
	d := &fakeEvidenceDeadline{viewOK: true, mutateOK: true, wf: baseWF(), company: "c_001"}
	svc := newTestService(t, d, store, repo)
	ctx := context.Background()
	up, _ := svc.UploadEvidenceFile(ctx, sub(), "rec-1", "step-001", "a.pdf", "application/pdf", bytes.NewReader([]byte("aa")), 2)
	f, data, err := svc.DownloadEvidenceFile(ctx, sub(), "rec-1", "step-001", up.File.FileID)
	if err != nil || string(data) != "aa" || f.StorageKey == "" {
		t.Fatalf("download %#v %v", f, err)
	}

	// completed
	completed := time.Now().UTC()
	d.states = map[string]wff.StepState{"step-001": {StepCode: "step-001", CompletedAt: &completed}}
	list, _ := svc.ListEvidenceFiles(ctx, sub(), "rec-1", "step-001")
	if list.Capabilities.CanUpload || list.Files[0].Capabilities.CanDelete {
		t.Fatalf("completed caps %#v", list)
	}
	if _, _, err := svc.DownloadEvidenceFile(ctx, sub(), "rec-1", "step-001", up.File.FileID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UploadEvidenceFile(ctx, sub(), "rec-1", "step-001", "b.pdf", "application/pdf", bytes.NewReader([]byte("b")), 1); err == nil {
		t.Fatal("upload denied")
	}
	if err := svc.DeleteEvidenceFile(ctx, sub(), "rec-1", "step-001", up.File.FileID); err == nil {
		t.Fatal("delete denied")
	}

	// reset incomplete, view-only
	d.states = nil
	d.mutateOK = false
	list, _ = svc.ListEvidenceFiles(ctx, sub(), "rec-1", "step-001")
	if list.Capabilities.CanUpload {
		t.Fatal("view-only upload")
	}
	if _, err := svc.UploadEvidenceFile(ctx, sub(), "rec-1", "step-001", "c.pdf", "application/pdf", bytes.NewReader([]byte("c")), 1); err == nil {
		t.Fatal("view-only mutate")
	}
}

func TestService_ContextBindingAndCrossCompany(t *testing.T) {
	store := newSvcMemStorage()
	repo := memory.NewRepository()
	d := &fakeEvidenceDeadline{viewOK: true, mutateOK: true, wf: baseWF(), company: "c_001"}
	svc := newTestService(t, d, store, repo)
	ctx := context.Background()
	up, _ := svc.UploadEvidenceFile(ctx, sub(), "rec-1", "step-001", "a.pdf", "application/pdf", bytes.NewReader([]byte("aa")), 2)

	if _, _, err := svc.DownloadEvidenceFile(ctx, sub(), "rec-1", "step-002", up.File.FileID); err == nil {
		t.Fatal("wrong step")
	}
	d.wf.RecordID = "rec-other"
	if _, _, err := svc.DownloadEvidenceFile(ctx, sub(), "rec-other", "step-001", up.File.FileID); err == nil {
		t.Fatal("wrong record")
	}
	d.wf = baseWF()
	d.wf.WorkflowInstanceID = "wi-other"
	if _, _, err := svc.DownloadEvidenceFile(ctx, sub(), "rec-1", "step-001", up.File.FileID); err == nil {
		t.Fatal("wrong instance")
	}
	d.wf = baseWF()
	other := wff.Subject{UserID: "u2", MembershipID: "m2", CompanyID: "c_002"}
	d.company = "c_001"
	if _, err := svc.ListEvidenceFiles(ctx, other, "rec-1", "step-001"); err == nil {
		t.Fatal("cross company list")
	}
}

func TestService_FutureStepDenied(t *testing.T) {
	svc := newTestService(t, nil, nil, nil)
	ctx := context.Background()
	_, err := svc.UploadEvidenceFile(ctx, sub(), "rec-1", "step-002", "a.pdf", "application/pdf", bytes.NewReader([]byte("a")), 1)
	var he *perr.HTTPError
	if !errors.As(err, &he) || he.Code != perr.CodeWorkflowStepNotCurrent {
		t.Fatalf("%v", err)
	}
}

func TestService_B3IsolationStill(t *testing.T) {
	snaps := []workflowapp.DocumentRequirementSnapshot{{ID: "req-1", Required: true, Name: "R", SourceDocID: "d1"}}
	fulfill := wffmem.NewRepository()
	store := newSvcMemStorage()
	repo := memory.NewRepository()
	svc := newTestService(t, nil, store, repo)
	_, _ = svc.UploadEvidenceFile(context.Background(), sub(), "rec-1", "step-001", "g.pdf", "application/pdf", bytes.NewReader([]byte("g")), 1)
	err := fulfill.CompleteStepWithRequiredDocuments("c_001", "wi-1", "step-001", snaps)
	if err == nil {
		t.Fatal("generic must not satisfy B3")
	}
	err = fulfill.CompleteStepWithRequiredDocuments("c_001", "wi-1", "step-001", nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestService_UploadDiskFailNoRow(t *testing.T) {
	store := newSvcMemStorage()
	store.failWrite = true
	repo := memory.NewRepository()
	svc := newTestService(t, nil, store, repo)
	_, err := svc.UploadEvidenceFile(context.Background(), sub(), "rec-1", "step-001", "a.pdf", "application/pdf", bytes.NewReader([]byte("a")), 1)
	if err == nil {
		t.Fatal("expected disk fail")
	}
	n, _ := repo.CountActiveByStep(context.Background(), wse.OwnerContext{
		CompanyID: "c_001", DisclosureRecordID: "rec-1", WorkflowInstanceID: "wi-1", StepCode: "step-001",
	})
	if n != 0 {
		t.Fatalf("n=%d", n)
	}
}

func TestService_DoubleDeleteHTTPContract(t *testing.T) {
	svc := newTestService(t, nil, nil, nil)
	ctx := context.Background()
	up, _ := svc.UploadEvidenceFile(ctx, sub(), "rec-1", "step-001", "a.pdf", "application/pdf", bytes.NewReader([]byte("a")), 1)
	if err := svc.DeleteEvidenceFile(ctx, sub(), "rec-1", "step-001", up.File.FileID); err != nil {
		t.Fatal(err)
	}
	err := svc.DeleteEvidenceFile(ctx, sub(), "rec-1", "step-001", up.File.FileID)
	var he *perr.HTTPError
	if !errors.As(err, &he) || he.HTTPStatus != http.StatusNotFound || he.Code != perr.CodeWorkflowStepEvidenceFileNotFound {
		t.Fatalf("double delete contract: %v", err)
	}
}

type fakeInspector struct{}

func (fakeInspector) InspectAccessToken(context.Context, string) (*iamapp.AccessTokenClaims, error) {
	return &iamapp.AccessTokenClaims{Sub: "u1", MembershipID: "m1", CompanyID: "c_001"}, nil
}

func (fakeInspector) InspectPreCompanyToken(context.Context, string) (*iamapp.PreCompanyTokenClaims, error) {
	return &iamapp.PreCompanyTokenClaims{Sub: "u1"}, nil
}

func TestHTTP_RoutesIntegration(t *testing.T) {
	store := newSvcMemStorage()
	repo := memory.NewRepository()
	d := &fakeEvidenceDeadline{viewOK: true, mutateOK: true, wf: baseWF(), company: "c_001"}
	svc := newTestService(t, d, store, repo)
	h := wsehttp.NewHandler(nil, svc, fakeInspector{})
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/company/deadlines/rec-1/steps/step-001/evidence-files", nil)
	req.Header.Set("Authorization", "Bearer t")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("list %d %s", rr.Code, rr.Body.String())
	}
	var list wse.ListResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &list)
	if !list.Capabilities.CanUpload || list.Files == nil {
		t.Fatalf("%#v", list)
	}

	body := &bytes.Buffer{}
	// minimal multipart
	mw := multipartWriter(body, "a.pdf", "%PDF-ok")
	req = httptest.NewRequest(http.MethodPost, "/api/v1/company/deadlines/rec-1/steps/step-001/evidence-files", body)
	req.Header.Set("Authorization", "Bearer t")
	req.Header.Set("Content-Type", mw)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != 201 {
		t.Fatalf("upload %d %s", rr.Code, rr.Body.String())
	}
	var up wse.UploadResult
	_ = json.Unmarshal(rr.Body.Bytes(), &up)
	if up.File.FileID == "" {
		t.Fatal("no file id")
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/company/deadlines/rec-1/steps/step-001/evidence-files/"+up.File.FileID+"/content", nil)
	req.Header.Set("Authorization", "Bearer t")
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 || !strings.Contains(rr.Header().Get("Content-Disposition"), "a.pdf") {
		t.Fatalf("download %d %v", rr.Code, rr.Header())
	}
	if strings.Contains(rr.Body.String(), "workflow-step-evidence") {
		t.Fatal("path leak")
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/v1/company/deadlines/rec-1/steps/step-001/evidence-files/"+up.File.FileID, nil)
	req.Header.Set("Authorization", "Bearer t")
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != 204 {
		t.Fatalf("delete %d", rr.Code)
	}
}

func multipartWriter(buf *bytes.Buffer, filename, content string) string {
	boundary := "----testboundary"
	buf.WriteString("--" + boundary + "\r\n")
	buf.WriteString(`Content-Disposition: form-data; name="file"; filename="` + filename + `"` + "\r\n")
	buf.WriteString("Content-Type: application/pdf\r\n\r\n")
	buf.WriteString(content)
	buf.WriteString("\r\n--" + boundary + "--\r\n")
	return "multipart/form-data; boundary=" + boundary
}
