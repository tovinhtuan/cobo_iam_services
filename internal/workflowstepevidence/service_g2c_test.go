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
	"sync/atomic"
	"testing"
	"time"

	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
	wff "github.com/cobo/cobo_iam_services/internal/workflowfulfillment"
	wse "github.com/cobo/cobo_iam_services/internal/workflowstepevidence"
	"github.com/cobo/cobo_iam_services/internal/workflowstepevidence/memory"
	wsehttp "github.com/cobo/cobo_iam_services/internal/workflowstepevidence/transport/http"
)

type evidenceAuditSpy struct {
	mu      sync.Mutex
	entries []auditapp.AppendAuditLogRequest
	fail    bool
}

func (a *evidenceAuditSpy) AppendAuditLog(_ context.Context, req auditapp.AppendAuditLogRequest) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.fail {
		return io.ErrUnexpectedEOF
	}
	cp := req
	if req.Metadata != nil {
		md := make(map[string]any, len(req.Metadata))
		for k, v := range req.Metadata {
			md[k] = v
		}
		cp.Metadata = md
	}
	a.entries = append(a.entries, cp)
	return nil
}

func (a *evidenceAuditSpy) byAction(action string) []auditapp.AppendAuditLogRequest {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]auditapp.AppendAuditLogRequest, 0)
	for _, e := range a.entries {
		if e.Action == action {
			out = append(out, e)
		}
	}
	return out
}

func (a *evidenceAuditSpy) len() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.entries)
}

type countingEvidenceStorage struct {
	*svcMemStorage
	deleteCalls int32
}

func (s *countingEvidenceStorage) Delete(objectKey string) error {
	atomic.AddInt32(&s.deleteCalls, 1)
	return s.svcMemStorage.Delete(objectKey)
}

type failCreateRepo struct {
	*memory.Repository
	failCreate bool
}

func (r *failCreateRepo) CreateActiveInTx(ctx context.Context, in wse.CreateTxInput) error {
	if r.failCreate {
		return io.ErrUnexpectedEOF
	}
	return r.Repository.CreateActiveInTx(ctx, in)
}

func baseFixtureWithAudit(t *testing.T) (*wse.Service, *fakeEvidenceDeadline, *memory.Repository, *countingEvidenceStorage, *evidenceAuditSpy, wff.Subject) {
	t.Helper()
	d := &fakeEvidenceDeadline{viewOK: true, mutateOK: true, wf: baseWF(), company: "c_001"}
	repo := memory.NewRepository()
	store := &countingEvidenceStorage{svcMemStorage: newSvcMemStorage()}
	spy := &evidenceAuditSpy{}
	today := time.Date(2026, 9, 18, 10, 0, 0, 0, time.UTC)
	svc := wse.NewService(d, repo, store, nil).WithNow(func() time.Time { return today }).WithAudit(spy)
	return svc, d, repo, store, spy, sub()
}

func TestG2C_UploadDeleteReplaceAudit(t *testing.T) {
	svc, _, repo, store, spy, subject := baseFixtureWithAudit(t)
	ctx := context.Background()

	up, err := svc.UploadEvidenceFile(ctx, subject, "rec-1", "step-001", "a.pdf", "application/pdf", bytes.NewReader([]byte("%PDF")), 4)
	if err != nil {
		t.Fatal(err)
	}
	uploads := spy.byAction(wse.AuditActionUpload)
	if len(uploads) != 1 {
		t.Fatalf("upload audits=%d", len(uploads))
	}
	e := uploads[0]
	if e.Action != wse.AuditActionUpload || e.ResourceType != wse.AuditResourceType || e.ResourceID != up.File.FileID {
		t.Fatalf("%+v", e)
	}
	if e.ActorUserID != "u1" || e.ActorMembershipID != "m1" || e.CompanyID != "c_001" || e.Decision != "allow" {
		t.Fatalf("actor %+v", e)
	}
	for _, key := range []string{"disclosure_record_id", "workflow_instance_id", "step_code", "evidence_file_id", "operation"} {
		if _, ok := e.Metadata[key]; !ok {
			t.Fatalf("missing %s", key)
		}
	}
	raw, _ := json.Marshal(e.Metadata)
	for _, bad := range []string{"storage_key", "object_key", "Bearer", "password", "/var/", "Authorization"} {
		if strings.Contains(string(raw), bad) {
			t.Fatalf("audit leak %q in %s", bad, raw)
		}
	}

	repl, err := svc.ReplaceEvidenceFile(ctx, subject, "rec-1", "step-001", up.File.FileID, "b.pdf", "application/pdf", bytes.NewReader([]byte("bb")), 2)
	if err != nil {
		t.Fatal(err)
	}
	replaces := spy.byAction(wse.AuditActionReplace)
	if len(replaces) != 1 {
		t.Fatalf("replace audits=%d", len(replaces))
	}
	if replaces[0].Metadata["old_file_id"] != up.File.FileID || replaces[0].Metadata["new_file_id"] != repl.File.FileID {
		t.Fatalf("%+v", replaces[0].Metadata)
	}
	old, _ := repo.GetByIDInContext(ctx, wse.OwnerContext{
		CompanyID: "c_001", DisclosureRecordID: "rec-1", WorkflowInstanceID: "wi-1", StepCode: "step-001",
	}, up.File.FileID)
	if old.LifecycleStatus != wse.LifecycleSuperseded || !store.Exists(old.StorageKey) {
		t.Fatalf("superseded retention %#v", old)
	}
	if _, _, err := svc.DownloadEvidenceFile(ctx, subject, "rec-1", "step-001", up.File.FileID); err == nil {
		t.Fatal("superseded download must fail")
	}

	if err := svc.DeleteEvidenceFile(ctx, subject, "rec-1", "step-001", repl.File.FileID); err != nil {
		t.Fatal(err)
	}
	deletes := spy.byAction(wse.AuditActionDelete)
	if len(deletes) != 1 {
		t.Fatalf("delete audits=%d", len(deletes))
	}
	delRow, _ := repo.GetByIDInContext(ctx, wse.OwnerContext{
		CompanyID: "c_001", DisclosureRecordID: "rec-1", WorkflowInstanceID: "wi-1", StepCode: "step-001",
	}, repl.File.FileID)
	if delRow.LifecycleStatus != wse.LifecycleDeleted || !store.Exists(delRow.StorageKey) {
		t.Fatal("deleted binary must retain")
	}
	if _, _, err := svc.DownloadEvidenceFile(ctx, subject, "rec-1", "step-001", repl.File.FileID); err == nil {
		t.Fatal("deleted download must fail")
	}
	if len(spy.byAction("workflow.step_evidence.download")) != 0 {
		t.Fatal("download audit must not exist")
	}
	if spy.len() != 3 {
		t.Fatalf("expected exactly 3 success audits, got %d", spy.len())
	}
}

func TestG2C_FailedMutationNoSuccessAudit(t *testing.T) {
	svc, deadline, _, _, spy, subject := baseFixtureWithAudit(t)
	ctx := context.Background()
	deadline.mutateOK = false
	_, err := svc.UploadEvidenceFile(ctx, subject, "rec-1", "step-001", "a.pdf", "application/pdf", bytes.NewReader([]byte("x")), 1)
	if err == nil {
		t.Fatal("expected deny")
	}
	if spy.len() != 0 {
		t.Fatalf("audits=%d", spy.len())
	}
	deadline.mutateOK = true
	now := time.Now().UTC()
	deadline.states = map[string]wff.StepState{"step-001": {StepCode: "step-001", CompletedAt: &now}}
	_, err = svc.UploadEvidenceFile(ctx, subject, "rec-1", "step-001", "a.pdf", "application/pdf", bytes.NewReader([]byte("x")), 1)
	if err == nil {
		t.Fatal("expected completed deny")
	}
	_ = svc.DeleteEvidenceFile(ctx, subject, "rec-1", "step-001", "missing")
	_, _ = svc.ReplaceEvidenceFile(ctx, subject, "rec-1", "step-001", "missing", "a.pdf", "application/pdf", bytes.NewReader([]byte("x")), 1)
	if spy.len() != 0 {
		t.Fatalf("failed must not audit, got %d", spy.len())
	}
}

func TestG2C_AuditFailureDoesNotRollback(t *testing.T) {
	svc, _, repo, store, spy, subject := baseFixtureWithAudit(t)
	spy.fail = true
	ctx := context.Background()
	up, err := svc.UploadEvidenceFile(ctx, subject, "rec-1", "step-001", "a.pdf", "application/pdf", bytes.NewReader([]byte("aa")), 2)
	if err != nil {
		t.Fatalf("upload must succeed despite audit fail: %v", err)
	}
	n, _ := repo.CountActiveByStep(ctx, wse.OwnerContext{
		CompanyID: "c_001", DisclosureRecordID: "rec-1", WorkflowInstanceID: "wi-1", StepCode: "step-001",
	})
	if n != 1 || !store.Exists(repoMustKey(t, repo, up.File.FileID)) {
		t.Fatal("mutation retained")
	}
	spy.fail = true
	if err := svc.DeleteEvidenceFile(ctx, subject, "rec-1", "step-001", up.File.FileID); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.GetByIDInContext(ctx, wse.OwnerContext{
		CompanyID: "c_001", DisclosureRecordID: "rec-1", WorkflowInstanceID: "wi-1", StepCode: "step-001",
	}, up.File.FileID)
	if got.LifecycleStatus != wse.LifecycleDeleted {
		t.Fatal("delete retained despite audit fail")
	}
}

func repoMustKey(t *testing.T, repo *memory.Repository, fileID string) string {
	t.Helper()
	f, err := repo.GetByIDInContext(context.Background(), wse.OwnerContext{
		CompanyID: "c_001", DisclosureRecordID: "rec-1", WorkflowInstanceID: "wi-1", StepCode: "step-001",
	}, fileID)
	if err != nil || f == nil {
		t.Fatalf("missing file %s", fileID)
	}
	return f.StorageKey
}

func TestG2C_DiskFailureNoAuditNoRow(t *testing.T) {
	d := &fakeEvidenceDeadline{viewOK: true, mutateOK: true, wf: baseWF(), company: "c_001"}
	repo := memory.NewRepository()
	store := newSvcMemStorage()
	store.failWrite = true
	spy := &evidenceAuditSpy{}
	svc := wse.NewService(d, repo, store, nil).WithAudit(spy)
	_, err := svc.UploadEvidenceFile(context.Background(), sub(), "rec-1", "step-001", "a.pdf", "application/pdf", bytes.NewReader([]byte("x")), 1)
	if err == nil {
		t.Fatal("expected disk fail")
	}
	n, _ := repo.CountActiveByStep(context.Background(), wse.OwnerContext{
		CompanyID: "c_001", DisclosureRecordID: "rec-1", WorkflowInstanceID: "wi-1", StepCode: "step-001",
	})
	if n != 0 || spy.len() != 0 {
		t.Fatalf("n=%d audits=%d", n, spy.len())
	}
}

func TestG2C_DBFailureCompensationNoAudit(t *testing.T) {
	d := &fakeEvidenceDeadline{viewOK: true, mutateOK: true, wf: baseWF(), company: "c_001"}
	base := memory.NewRepository()
	repo := &failCreateRepo{Repository: base, failCreate: true}
	store := &countingEvidenceStorage{svcMemStorage: newSvcMemStorage()}
	spy := &evidenceAuditSpy{}
	svc := wse.NewService(d, repo, store, nil).WithAudit(spy)
	_, err := svc.UploadEvidenceFile(context.Background(), sub(), "rec-1", "step-001", "a.pdf", "application/pdf", bytes.NewReader([]byte("x")), 1)
	if err == nil {
		t.Fatal("expected db fail")
	}
	if spy.len() != 0 {
		t.Fatalf("audits=%d", spy.len())
	}
	if store.deleteCalls < 1 {
		t.Fatal("expected compensating delete")
	}
	n, _ := base.CountActiveByStep(context.Background(), wse.OwnerContext{
		CompanyID: "c_001", DisclosureRecordID: "rec-1", WorkflowInstanceID: "wi-1", StepCode: "step-001",
	})
	if n != 0 {
		t.Fatalf("active=%d", n)
	}
}

func TestG2C_ErrorDomainNoFulfillmentLeak(t *testing.T) {
	svc := newTestService(t, nil, nil, nil)
	ctx := context.Background()
	_, err := svc.UploadEvidenceFile(ctx, sub(), "rec-1", "step-001", "x.exe", "application/octet-stream", bytes.NewReader([]byte("x")), 1)
	assertEvidenceCode(t, err, perr.CodeWorkflowStepEvidenceFileTypeInvalid)
	_, err = svc.UploadEvidenceFile(ctx, sub(), "rec-1", "step-001", "a.pdf", "application/pdf", bytes.NewReader(make([]byte, wse.MaxSizeBytes+1)), wse.MaxSizeBytes+1)
	assertEvidenceCode(t, err, perr.CodeWorkflowStepEvidenceFileTooLarge)
	up, _ := svc.UploadEvidenceFile(ctx, sub(), "rec-1", "step-001", "a.pdf", "application/pdf", bytes.NewReader([]byte("a")), 1)
	_ = svc.DeleteEvidenceFile(ctx, sub(), "rec-1", "step-001", up.File.FileID)
	err = svc.DeleteEvidenceFile(ctx, sub(), "rec-1", "step-001", up.File.FileID)
	assertEvidenceCode(t, err, perr.CodeWorkflowStepEvidenceFileNotFound)
	_, _, err = svc.DownloadEvidenceFile(ctx, sub(), "rec-1", "step-001", up.File.FileID)
	assertEvidenceCode(t, err, perr.CodeWorkflowStepEvidenceFileNotFound)
}

func assertEvidenceCode(t *testing.T, err error, code perr.Code) {
	t.Helper()
	var he *perr.HTTPError
	if !errors.As(err, &he) || he.Code != code {
		t.Fatalf("want %s got %v", code, err)
	}
	s := string(he.Code) + he.Message
	if strings.Contains(s, "DOCUMENT_FULFILLMENT") || strings.Contains(s, "REQUIREMENT_") {
		t.Fatalf("domain leak: %s", s)
	}
}

func TestG2C_ErrorSanitizationNoInternalLeak(t *testing.T) {
	d := &fakeEvidenceDeadline{viewOK: true, mutateOK: true, wf: baseWF(), company: "c_001"}
	repo := &failCreateRepo{Repository: memory.NewRepository(), failCreate: true}
	svc := wse.NewService(d, repo, newSvcMemStorage(), nil)
	_, err := svc.UploadEvidenceFile(context.Background(), sub(), "rec-1", "step-001", "a.pdf", "application/pdf", bytes.NewReader([]byte("x")), 1)
	if err == nil {
		t.Fatal("expected error")
	}
	rec := httptest.NewRecorder()
	httpx.WriteError(rec, nil, err)
	body := rec.Body.String()
	for _, bad := range []string{"unexpected EOF", "INSERT", "workflow_step_evidence", "/var/", "sql:", "storage_key"} {
		if strings.Contains(body, bad) {
			t.Fatalf("leak in body: %s", body)
		}
	}
	if !strings.Contains(body, "INTERNAL_ERROR") {
		t.Fatalf("body=%s", body)
	}
}

func TestG2C_FilenameSecurityAndContentDisposition(t *testing.T) {
	for _, name := range []string{`../../secret.xlsx`, `..\..\secret.xlsx`, `/etc/passwd`, `C:\Windows\secret.txt`, "evil\r\nX: inject.pdf", "q\"uote.pdf"} {
		got := wff.SanitizeFileName(name)
		key := wse.EvidenceObjectKey("c_001", "fid", got)
		if strings.Contains(key, "..") || !strings.HasPrefix(key, wse.StorageNamespace+"/") {
			t.Fatalf("escape via %q -> %q key=%s", name, got, key)
		}
		if strings.ContainsAny(got, "\r\n\"\\/") {
			t.Fatalf("unsafe sanitized %q from %q", got, name)
		}
	}
	escaped := wff.EscapeContentDispositionFileName("báo cáo\r\nX: evil.pdf")
	if strings.ContainsAny(escaped, "\r\n\"") {
		t.Fatalf("CRLF/quote unsafe: %q", escaped)
	}
	vn := wff.EscapeContentDispositionFileName("báo_cáo_quý.pdf")
	if vn == "" || vn == "download" {
		t.Fatalf("unicode filename lost: %q", vn)
	}
	hdr := httptest.NewRecorder()
	hdr.Header().Set("Content-Disposition", `attachment; filename="`+escaped+`"`)
	if strings.Contains(hdr.Header().Get("Content-Disposition"), "\n") {
		t.Fatal("header injection")
	}
}

func TestG2C_SizeBoundaryAndOrphans(t *testing.T) {
	store := newSvcMemStorage()
	repo := memory.NewRepository()
	svc := newTestService(t, nil, store, repo)
	ctx := context.Background()
	justUnder := make([]byte, wse.MaxSizeBytes)
	up, err := svc.UploadEvidenceFile(ctx, sub(), "rec-1", "step-001", "ok.pdf", "application/pdf", bytes.NewReader(justUnder), int64(len(justUnder)))
	if err != nil {
		t.Fatalf("just under: %v", err)
	}
	if up.File.FileSize != wse.MaxSizeBytes {
		t.Fatalf("size=%d", up.File.FileSize)
	}
	beforeKeys := len(store.data)
	_, err = svc.UploadEvidenceFile(ctx, sub(), "rec-1", "step-001", "big.pdf", "application/pdf", bytes.NewReader(make([]byte, wse.MaxSizeBytes+1)), wse.MaxSizeBytes+1)
	if err == nil {
		t.Fatal("oversized must fail")
	}
	assertEvidenceCode(t, err, perr.CodeWorkflowStepEvidenceFileTooLarge)
	n, _ := repo.CountActiveByStep(ctx, wse.OwnerContext{
		CompanyID: "c_001", DisclosureRecordID: "rec-1", WorkflowInstanceID: "wi-1", StepCode: "step-001",
	})
	if n != 1 {
		t.Fatalf("active=%d", n)
	}
	if len(store.data) != beforeKeys {
		t.Fatalf("orphan keys before=%d after=%d", beforeKeys, len(store.data))
	}
}

func TestG2C_ExtensionMimePolicy(t *testing.T) {
	svc := newTestService(t, nil, nil, nil)
	ctx := context.Background()
	cases := []struct {
		name, mime string
	}{
		{"a.pdf", "image/png"},
		{"a.exe", "application/pdf"},
		{"noext", "application/pdf"},
		{"A.PDF", "application/pdf"}, // uppercase ok via Sanitize/Ext lower
	}
	for _, c := range cases[:3] {
		_, err := svc.UploadEvidenceFile(ctx, sub(), "rec-1", "step-001", c.name, c.mime, bytes.NewReader([]byte("x")), 1)
		if err == nil {
			t.Fatalf("%s/%s must fail", c.name, c.mime)
		}
		assertEvidenceCode(t, err, perr.CodeWorkflowStepEvidenceFileTypeInvalid)
	}
	_, err := svc.UploadEvidenceFile(ctx, sub(), "rec-1", "step-001", "A.PDF", "application/pdf", bytes.NewReader([]byte("x")), 1)
	if err != nil {
		t.Fatalf("uppercase ext: %v", err)
	}
}

func TestG2C_DoubleDeleteRaceAuditParity(t *testing.T) {
	svc, _, repo, store, spy, subject := baseFixtureWithAudit(t)
	up, err := svc.UploadEvidenceFile(context.Background(), subject, "rec-1", "step-001", "a.pdf", "application/pdf", bytes.NewReader([]byte("x")), 1)
	if err != nil {
		t.Fatal(err)
	}
	before := spy.len()
	var ok int32
	var wg sync.WaitGroup
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			if err := svc.DeleteEvidenceFile(context.Background(), subject, "rec-1", "step-001", up.File.FileID); err == nil {
				atomic.AddInt32(&ok, 1)
			}
		}()
	}
	wg.Wait()
	if ok != 1 {
		t.Fatalf("winners=%d", ok)
	}
	if len(spy.byAction(wse.AuditActionDelete)) != 1 {
		t.Fatalf("delete audits=%d", len(spy.byAction(wse.AuditActionDelete)))
	}
	got, _ := repo.GetByIDInContext(context.Background(), wse.OwnerContext{
		CompanyID: "c_001", DisclosureRecordID: "rec-1", WorkflowInstanceID: "wi-1", StepCode: "step-001",
	}, up.File.FileID)
	if got.LifecycleStatus != wse.LifecycleDeleted || !store.Exists(got.StorageKey) {
		t.Fatal("deleted retained")
	}
	if spy.len() != before+1 {
		t.Fatalf("audit parity total=%d", spy.len())
	}
}

func TestG2C_DeleteReplaceAndDoubleReplaceRaceAuditParity(t *testing.T) {
	svc, _, repo, _, spy, subject := baseFixtureWithAudit(t)
	up, _ := svc.UploadEvidenceFile(context.Background(), subject, "rec-1", "step-001", "a.pdf", "application/pdf", bytes.NewReader([]byte("x")), 1)
	var delOK, repOK int32
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if err := svc.DeleteEvidenceFile(context.Background(), subject, "rec-1", "step-001", up.File.FileID); err == nil {
			atomic.AddInt32(&delOK, 1)
		}
	}()
	go func() {
		defer wg.Done()
		if _, err := svc.ReplaceEvidenceFile(context.Background(), subject, "rec-1", "step-001", up.File.FileID, "b.pdf", "application/pdf", bytes.NewReader([]byte("y")), 1); err == nil {
			atomic.AddInt32(&repOK, 1)
		}
	}()
	wg.Wait()
	if delOK+repOK != 1 {
		t.Fatalf("winners del=%d rep=%d", delOK, repOK)
	}
	if int(delOK) != len(spy.byAction(wse.AuditActionDelete)) {
		t.Fatalf("delete audit parity")
	}
	if int(repOK) != len(spy.byAction(wse.AuditActionReplace)) {
		t.Fatalf("replace audit parity")
	}

	up2, _ := svc.UploadEvidenceFile(context.Background(), subject, "rec-1", "step-001", "c.pdf", "application/pdf", bytes.NewReader([]byte("c")), 1)
	spyBefore := len(spy.byAction(wse.AuditActionReplace))
	var wins int32
	wg.Add(2)
	go func() {
		defer wg.Done()
		if _, err := svc.ReplaceEvidenceFile(context.Background(), subject, "rec-1", "step-001", up2.File.FileID, "d.pdf", "application/pdf", bytes.NewReader([]byte("d")), 1); err == nil {
			atomic.AddInt32(&wins, 1)
		}
	}()
	go func() {
		defer wg.Done()
		if _, err := svc.ReplaceEvidenceFile(context.Background(), subject, "rec-1", "step-001", up2.File.FileID, "e.pdf", "application/pdf", bytes.NewReader([]byte("e")), 1); err == nil {
			atomic.AddInt32(&wins, 1)
		}
	}()
	wg.Wait()
	if wins != 1 {
		t.Fatalf("double replace wins=%d", wins)
	}
	if len(spy.byAction(wse.AuditActionReplace))-spyBefore != 1 {
		t.Fatal("exactly one replace success audit")
	}
	if still, _ := repo.GetActiveByIDInContext(context.Background(), wse.OwnerContext{
		CompanyID: "c_001", DisclosureRecordID: "rec-1", WorkflowInstanceID: "wi-1", StepCode: "step-001",
	}, up2.File.FileID); still != nil {
		t.Fatal("old must not stay ACTIVE")
	}
}

func TestG2C_CompleteMutationRaceAndAudit(t *testing.T) {
	svc, deadline, repo, _, spy, subject := baseFixtureWithAudit(t)
	up, err := svc.UploadEvidenceFile(context.Background(), subject, "rec-1", "step-001", "a.pdf", "application/pdf", bytes.NewReader([]byte("x")), 1)
	if err != nil {
		t.Fatal(err)
	}
	var deleteOK, uploadOK, replaceOK int32
	var wg sync.WaitGroup
	wg.Add(4)
	go func() {
		defer wg.Done()
		// Repo completion gate races with Create/Delete/ReplaceInTx RequireNotCompleted.
		repo.CompleteStep("wi-1", "step-001")
	}()
	go func() {
		defer wg.Done()
		if err := svc.DeleteEvidenceFile(context.Background(), subject, "rec-1", "step-001", up.File.FileID); err == nil {
			atomic.AddInt32(&deleteOK, 1)
		}
	}()
	go func() {
		defer wg.Done()
		if _, err := svc.UploadEvidenceFile(context.Background(), subject, "rec-1", "step-001", "b.pdf", "application/pdf", bytes.NewReader([]byte("y")), 1); err == nil {
			atomic.AddInt32(&uploadOK, 1)
		}
	}()
	go func() {
		defer wg.Done()
		if _, err := svc.ReplaceEvidenceFile(context.Background(), subject, "rec-1", "step-001", up.File.FileID, "c.pdf", "application/pdf", bytes.NewReader([]byte("z")), 1); err == nil {
			atomic.AddInt32(&replaceOK, 1)
		}
	}()
	wg.Wait()
	if int(deleteOK) != len(spy.byAction(wse.AuditActionDelete)) {
		t.Fatalf("delete race audit parity ok=%d audits=%d", deleteOK, len(spy.byAction(wse.AuditActionDelete)))
	}
	if int(replaceOK) != len(spy.byAction(wse.AuditActionReplace)) {
		t.Fatalf("replace race audit parity ok=%d audits=%d", replaceOK, len(spy.byAction(wse.AuditActionReplace)))
	}
	// After both gates are completed, mutations denied and no extra success audits.
	now := time.Now().UTC()
	deadline.states = map[string]wff.StepState{"step-001": {StepCode: "step-001", CompletedAt: &now}}
	repo.SetStepCompleted("wi-1", "step-001", true)
	before := spy.len()
	_, _ = svc.UploadEvidenceFile(context.Background(), subject, "rec-1", "step-001", "z.pdf", "application/pdf", bytes.NewReader([]byte("z")), 1)
	_ = svc.DeleteEvidenceFile(context.Background(), subject, "rec-1", "step-001", up.File.FileID)
	_, _ = svc.ReplaceEvidenceFile(context.Background(), subject, "rec-1", "step-001", up.File.FileID, "z.pdf", "application/pdf", bytes.NewReader([]byte("z")), 1)
	if spy.len() != before {
		t.Fatal("completed step failed mutations must not emit success audit")
	}
	_ = uploadOK
}

func TestG2C_HTTPSecurityMatrix(t *testing.T) {
	store := newSvcMemStorage()
	repo := memory.NewRepository()
	d := &fakeEvidenceDeadline{viewOK: true, mutateOK: true, wf: baseWF(), company: "c_001"}
	spy := &evidenceAuditSpy{}
	svc := wse.NewService(d, repo, store, nil).WithAudit(spy)
	h := wsehttp.NewHandler(nil, svc, fakeInspector{})
	mux := http.NewServeMux()
	h.Register(mux)

	// malicious filename upload
	body := &bytes.Buffer{}
	ct := multipartWriter(body, "../../etc/passwd.pdf", "%PDF")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/company/deadlines/rec-1/steps/step-001/evidence-files", body)
	req.Header.Set("Authorization", "Bearer t")
	req.Header.Set("Content-Type", ct)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != 201 {
		t.Fatalf("upload %d %s", rr.Code, rr.Body.String())
	}
	var up wse.UploadResult
	_ = json.Unmarshal(rr.Body.Bytes(), &up)
	if strings.Contains(up.File.FileName, "..") || strings.Contains(up.File.FileName, "/") {
		t.Fatalf("filename not sanitized: %q", up.File.FileName)
	}
	raw, _ := json.Marshal(up)
	if strings.Contains(string(raw), "storage_key") || strings.Contains(string(raw), "workflow-step-evidence/") {
		t.Fatalf("storage leak: %s", raw)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/company/deadlines/rec-1/steps/step-001/evidence-files/"+up.File.FileID+"/content", nil)
	req.Header.Set("Authorization", "Bearer t")
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	cd := rr.Header().Get("Content-Disposition")
	if strings.ContainsAny(cd, "\r\n") {
		t.Fatal("CD injection")
	}
	if spy.len() != 1 || len(spy.byAction(wse.AuditActionUpload)) != 1 {
		t.Fatalf("download must not add audit; audits=%d", spy.len())
	}

	// invalid type
	body = &bytes.Buffer{}
	ct = multipartWriter(body, "x.exe", "MZ")
	req = httptest.NewRequest(http.MethodPost, "/api/v1/company/deadlines/rec-1/steps/step-001/evidence-files", body)
	req.Header.Set("Authorization", "Bearer t")
	req.Header.Set("Content-Type", ct)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code == 201 || strings.Contains(rr.Body.String(), "DOCUMENT_FULFILLMENT") {
		t.Fatalf("invalid type response: %d %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "WORKFLOW_STEP_EVIDENCE_FILE_TYPE_INVALID") {
		t.Fatalf("want evidence type code: %s", rr.Body.String())
	}

	// double delete
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/company/deadlines/rec-1/steps/step-001/evidence-files/"+up.File.FileID, nil)
	req.Header.Set("Authorization", "Bearer t")
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != 204 {
		t.Fatalf("delete %d", rr.Code)
	}
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/company/deadlines/rec-1/steps/step-001/evidence-files/"+up.File.FileID, nil)
	req.Header.Set("Authorization", "Bearer t")
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != 404 || strings.Contains(rr.Body.String(), "DOCUMENT_FULFILLMENT") {
		t.Fatalf("double delete: %d %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "WORKFLOW_STEP_EVIDENCE_FILE_NOT_FOUND") {
		t.Fatalf("want evidence not found: %s", rr.Body.String())
	}

	// completed mutation
	now := time.Now().UTC()
	d.states = map[string]wff.StepState{"step-001": {StepCode: "step-001", CompletedAt: &now}}
	body = &bytes.Buffer{}
	ct = multipartWriter(body, "z.pdf", "%PDF")
	req = httptest.NewRequest(http.MethodPost, "/api/v1/company/deadlines/rec-1/steps/step-001/evidence-files", body)
	req.Header.Set("Authorization", "Bearer t")
	req.Header.Set("Content-Type", ct)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code == 201 {
		t.Fatal("completed upload must fail")
	}
}

func TestG2C_NoPhysicalPurgeInPackage(t *testing.T) {
	// Compensating Delete on NEW uncommitted keys is allowed; lifecycle success paths must not purge.
	svc, _, repo, store, _, subject := baseFixtureWithAudit(t)
	up, _ := svc.UploadEvidenceFile(context.Background(), subject, "rec-1", "step-001", "a.pdf", "application/pdf", bytes.NewReader([]byte("a")), 1)
	key := repoMustKey(t, repo, up.File.FileID)
	_ = svc.DeleteEvidenceFile(context.Background(), subject, "rec-1", "step-001", up.File.FileID)
	if !store.Exists(key) {
		t.Fatal("lifecycle delete must not purge binary")
	}
}

func TestG2C_ClientAuthorityFieldsIgnored(t *testing.T) {
	svc, _, _, _, spy, subject := baseFixtureWithAudit(t)
	up, err := svc.UploadEvidenceFile(context.Background(), subject, "rec-1", "step-001", "a.pdf", "application/pdf", bytes.NewReader([]byte("x")), 1)
	if err != nil {
		t.Fatal(err)
	}
	if up.File.UploadedBy != "u1" {
		t.Fatal("uploaded_by must be subject")
	}
	e := spy.byAction(wse.AuditActionUpload)[0]
	if e.ActorUserID != "u1" || e.CompanyID != "c_001" {
		t.Fatalf("%+v", e)
	}
}
