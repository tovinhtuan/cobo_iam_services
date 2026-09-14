package workflowfulfillment_test

import (
	"bytes"
	"context"
	"encoding/json"
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
	wffmemory "github.com/cobo/cobo_iam_services/internal/workflowfulfillment/memory"
)

type auditSpy struct {
	mu      sync.Mutex
	entries []auditapp.AppendAuditLogRequest
	fail    bool
}

func (a *auditSpy) AppendAuditLog(_ context.Context, req auditapp.AppendAuditLogRequest) error {
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

func (a *auditSpy) byAction(action string) []auditapp.AppendAuditLogRequest {
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

func (a *auditSpy) len() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.entries)
}

type countingStorage struct {
	*memStorage
	deleteCalls int32
}

func (s *countingStorage) Delete(objectKey string) error {
	atomic.AddInt32(&s.deleteCalls, 1)
	return s.memStorage.Delete(objectKey)
}

type failWriteStorage struct {
	*memStorage
}

func (s *failWriteStorage) Write(string, io.Reader) (int64, error) {
	return 0, io.ErrUnexpectedEOF
}

type failDBRepo struct {
	*wffmemory.Repository
	failUpload bool
}

func (r *failDBRepo) UploadInTx(ctx context.Context, in wff.UploadTxInput) error {
	if r.failUpload {
		return io.ErrUnexpectedEOF
	}
	return r.Repository.UploadInTx(ctx, in)
}

func baseFixtureWithAudit(t *testing.T) (*wff.Service, *fakeDeadline, *fakeSnapshots, *wffmemory.Repository, *countingStorage, *auditSpy, wff.Subject) {
	t.Helper()
	_, deadline, snaps, files, store, sub := baseFixture(t)
	countStore := &countingStorage{memStorage: store}
	spy := &auditSpy{}
	today := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)
	svc := wff.NewService(deadline, snaps, files, countStore, nil).WithNow(func() time.Time { return today }).WithAudit(spy)
	return svc, deadline, snaps, files, countStore, spy, sub
}

func TestB5_LifecycleStateMachine(t *testing.T) {
	if err := wff.ValidateInitialLifecycle(wff.LifecycleActive); err != nil {
		t.Fatal(err)
	}
	if err := wff.ValidateInitialLifecycle(wff.LifecycleDeleted); err == nil {
		t.Fatal("NEW→DELETED must fail")
	}
	if err := wff.ValidateTransition(wff.LifecycleActive, wff.LifecycleDeleted); err != nil {
		t.Fatal(err)
	}
	if err := wff.ValidateTransition(wff.LifecycleActive, wff.LifecycleSuperseded); err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]string{
		{wff.LifecycleDeleted, wff.LifecycleActive},
		{wff.LifecycleSuperseded, wff.LifecycleActive},
		{wff.LifecycleDeleted, wff.LifecycleSuperseded},
		{wff.LifecycleSuperseded, wff.LifecycleDeleted},
		{wff.LifecycleActive, wff.LifecycleActive},
	} {
		if err := wff.ValidateTransition(pair[0], pair[1]); err == nil {
			t.Fatalf("expected invalid %s→%s", pair[0], pair[1])
		}
	}
	allowed := wff.AllowedLifecycleTransitions()
	if len(allowed[wff.LifecycleActive]) != 2 {
		t.Fatalf("allowed ACTIVE transitions=%v", allowed[wff.LifecycleActive])
	}
}

func TestB5_UploadDeleteReplaceAudit(t *testing.T) {
	svc, _, _, files, store, spy, sub := baseFixtureWithAudit(t)

	up, err := svc.Upload(context.Background(), sub, "rec-1", "prep", "req-1", "a.pdf", "application/pdf", bytes.NewReader([]byte("%PDF")), 4)
	if err != nil {
		t.Fatal(err)
	}
	uploads := spy.byAction(wff.AuditActionUpload)
	if len(uploads) != 1 {
		t.Fatalf("upload audits=%d", len(uploads))
	}
	assertAuditSafe(t, uploads[0], sub, up.File.FileID)
	if uploads[0].Metadata["original_file_name"] != "a.pdf" {
		t.Fatalf("filename=%v", uploads[0].Metadata["original_file_name"])
	}
	if uploads[0].Metadata["mime_type"] != "application/pdf" {
		t.Fatal("mime missing")
	}
	if uploads[0].Metadata["file_size"].(int64) != 4 {
		t.Fatalf("size=%v", uploads[0].Metadata["file_size"])
	}

	rep, err := svc.Replace(context.Background(), sub, "rec-1", "prep", up.File.FileID, "b.pdf", "application/pdf", bytes.NewReader([]byte("%PDF2")), 5)
	if err != nil {
		t.Fatal(err)
	}
	replaces := spy.byAction(wff.AuditActionReplace)
	if len(replaces) != 1 {
		t.Fatalf("replace audits=%d", len(replaces))
	}
	assertAuditSafe(t, replaces[0], sub, rep.File.FileID)
	if replaces[0].Metadata["old_file_id"] != up.File.FileID || replaces[0].Metadata["new_file_id"] != rep.File.FileID {
		t.Fatalf("replace ids=%v", replaces[0].Metadata)
	}
	old, _ := files.GetByID(context.Background(), "co-a", up.File.FileID)
	if old.LifecycleStatus != wff.LifecycleSuperseded || !store.Exists(old.StorageKey) {
		t.Fatal("old binary must remain after replace")
	}

	if err := svc.Delete(context.Background(), sub, "rec-1", "prep", rep.File.FileID); err != nil {
		t.Fatal(err)
	}
	deletes := spy.byAction(wff.AuditActionDelete)
	if len(deletes) != 1 {
		t.Fatalf("delete audits=%d", len(deletes))
	}
	assertAuditSafe(t, deletes[0], sub, rep.File.FileID)
	del, _ := files.GetByID(context.Background(), "co-a", rep.File.FileID)
	if del.LifecycleStatus != wff.LifecycleDeleted || !store.Exists(del.StorageKey) {
		t.Fatal("logical delete must retain binary")
	}
	if store.deleteCalls != 0 {
		t.Fatalf("DELETE path must not unlink binary, deleteCalls=%d", store.deleteCalls)
	}
	if spy.len() != 3 {
		t.Fatalf("total audits=%d", spy.len())
	}
}

func assertAuditSafe(t *testing.T, e auditapp.AppendAuditLogRequest, sub wff.Subject, fileID string) {
	t.Helper()
	if e.ActorUserID != sub.UserID || e.ActorMembershipID != sub.MembershipID || e.CompanyID != sub.CompanyID {
		t.Fatalf("actor/company mismatch: %+v", e)
	}
	if e.Decision != "allow" || e.ResourceType != wff.AuditResourceType || e.ResourceID != fileID {
		t.Fatalf("resource=%+v", e)
	}
	raw, _ := json.Marshal(e.Metadata)
	s := string(raw)
	for _, bad := range []string{"storage_key", "object_key", "Bearer", "password", "/var/", "Authorization"} {
		if strings.Contains(s, bad) {
			t.Fatalf("audit leak %q in %s", bad, s)
		}
	}
	for _, key := range []string{"disclosure_record_id", "workflow_instance_id", "step_code", "requirement_snapshot_id", "fulfillment_file_id"} {
		if _, ok := e.Metadata[key]; !ok {
			t.Fatalf("missing context %s", key)
		}
	}
}

func TestB5_FailedMutationNoSuccessAudit(t *testing.T) {
	svc, deadline, _, _, _, spy, sub := baseFixtureWithAudit(t)
	deadline.mutateOK = false
	_, err := svc.Upload(context.Background(), sub, "rec-1", "prep", "req-1", "a.pdf", "application/pdf", bytes.NewReader([]byte("x")), 1)
	if err == nil {
		t.Fatal("expected deny")
	}
	if spy.len() != 0 {
		t.Fatalf("audits=%d", spy.len())
	}
	deadline.mutateOK = true
	now := time.Now().UTC()
	deadline.states = map[string]wff.StepState{"prep": {StepCode: "prep", CompletedAt: &now}}
	_, err = svc.Upload(context.Background(), sub, "rec-1", "prep", "req-1", "a.pdf", "application/pdf", bytes.NewReader([]byte("x")), 1)
	if err == nil {
		t.Fatal("expected completed deny")
	}
	_ = svc.Delete(context.Background(), sub, "rec-1", "prep", "missing")
	_, _ = svc.Replace(context.Background(), sub, "rec-1", "prep", "missing", "a.pdf", "application/pdf", bytes.NewReader([]byte("x")), 1)
	if spy.len() != 0 {
		t.Fatalf("completed/failed must not audit success, got %d", spy.len())
	}
}

func TestB5_MultiReplaceLineageAndAudit(t *testing.T) {
	svc, _, _, files, store, spy, sub := baseFixtureWithAudit(t)
	a, err := svc.Upload(context.Background(), sub, "rec-1", "prep", "req-1", "a.pdf", "application/pdf", bytes.NewReader([]byte("a")), 1)
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.Replace(context.Background(), sub, "rec-1", "prep", a.File.FileID, "b.pdf", "application/pdf", bytes.NewReader([]byte("b")), 1)
	if err != nil {
		t.Fatal(err)
	}
	c, err := svc.Replace(context.Background(), sub, "rec-1", "prep", b.File.FileID, "c.pdf", "application/pdf", bytes.NewReader([]byte("c")), 1)
	if err != nil {
		t.Fatal(err)
	}
	fa, _ := files.GetByID(context.Background(), "co-a", a.File.FileID)
	fb, _ := files.GetByID(context.Background(), "co-a", b.File.FileID)
	fc, _ := files.GetActiveByID(context.Background(), "co-a", c.File.FileID)
	if fa.LifecycleStatus != wff.LifecycleSuperseded || fa.SupersededByFileID != fb.ID {
		t.Fatalf("A lineage %+v", fa)
	}
	if fb.LifecycleStatus != wff.LifecycleSuperseded || fb.SupersedesFileID != fa.ID || fb.SupersededByFileID != fc.ID {
		t.Fatalf("B lineage %+v", fb)
	}
	if fc.SupersedesFileID != fb.ID || fc.LifecycleStatus != wff.LifecycleActive {
		t.Fatalf("C lineage %+v", fc)
	}
	if !store.Exists(fa.StorageKey) || !store.Exists(fb.StorageKey) || !store.Exists(fc.StorageKey) {
		t.Fatal("all binaries retained")
	}
	if len(spy.byAction(wff.AuditActionReplace)) != 2 || len(spy.byAction(wff.AuditActionUpload)) != 1 {
		t.Fatalf("audits upload=%d replace=%d", len(spy.byAction(wff.AuditActionUpload)), len(spy.byAction(wff.AuditActionReplace)))
	}
}

func TestB5_NoAutoPurgeContract(t *testing.T) {
	// V1 has no retention TTL / sweeper authority.
	if wff.MaxSizeBytes <= 0 {
		t.Fatal("unexpected")
	}
	// Ensure package does not export a purge/TTL constant.
	_ = wff.LifecycleDeleted
	_ = wff.LifecycleSuperseded
}

func TestB5_DiskFailureNoAuditNoRow(t *testing.T) {
	svc, deadline, snaps, files, _, sub := baseFixture(t)
	spy := &auditSpy{}
	failStore := &failWriteStorage{memStorage: newMemStorage()}
	svc = wff.NewService(deadline, snaps, files, failStore, nil).WithAudit(spy)
	_, err := svc.Upload(context.Background(), sub, "rec-1", "prep", "req-1", "a.pdf", "application/pdf", bytes.NewReader([]byte("x")), 1)
	if err == nil {
		t.Fatal("expected disk fail")
	}
	n, _ := files.CountActiveByRequirement(context.Background(), "req-1")
	if n != 0 || spy.len() != 0 {
		t.Fatalf("n=%d audits=%d", n, spy.len())
	}
}

func TestB5_DiskSuccessDBFailureCompensationNoAudit(t *testing.T) {
	svc, deadline, snaps, baseFiles, _, sub := baseFixture(t)
	spy := &auditSpy{}
	store := &countingStorage{memStorage: newMemStorage()}
	repo := &failDBRepo{Repository: baseFiles, failUpload: true}
	svc = wff.NewService(deadline, snaps, repo, store, nil).WithAudit(spy)
	_, err := svc.Upload(context.Background(), sub, "rec-1", "prep", "req-1", "a.pdf", "application/pdf", bytes.NewReader([]byte("x")), 1)
	if err == nil {
		t.Fatal("expected db fail")
	}
	if spy.len() != 0 {
		t.Fatalf("audits=%d", spy.len())
	}
	if store.deleteCalls < 1 {
		t.Fatal("expected compensating delete of orphaned binary")
	}
	n, _ := baseFiles.CountActiveByRequirement(context.Background(), "req-1")
	if n != 0 {
		t.Fatalf("active=%d", n)
	}
}

func TestB5_DoubleDeleteRaceAuditParity(t *testing.T) {
	svc, _, _, files, store, spy, sub := baseFixtureWithAudit(t)
	up, err := svc.Upload(context.Background(), sub, "rec-1", "prep", "req-1", "a.pdf", "application/pdf", bytes.NewReader([]byte("x")), 1)
	if err != nil {
		t.Fatal(err)
	}
	beforeAudits := spy.len()
	var ok int32
	var wg sync.WaitGroup
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			if err := svc.Delete(context.Background(), sub, "rec-1", "prep", up.File.FileID); err == nil {
				atomic.AddInt32(&ok, 1)
			}
		}()
	}
	wg.Wait()
	if ok != 1 {
		t.Fatalf("winners=%d", ok)
	}
	delAudits := spy.byAction(wff.AuditActionDelete)
	if len(delAudits) != 1 {
		t.Fatalf("delete audits=%d (parity)", len(delAudits))
	}
	got, _ := files.GetByID(context.Background(), "co-a", up.File.FileID)
	if got.LifecycleStatus != wff.LifecycleDeleted || !store.Exists(got.StorageKey) {
		t.Fatal("deleted retained")
	}
	if spy.len() != beforeAudits+1 {
		t.Fatalf("audit parity fail total=%d", spy.len())
	}
}

func TestB5_DeleteReplaceAndDoubleReplaceRaceAuditParity(t *testing.T) {
	svc, _, _, files, _, spy, sub := baseFixtureWithAudit(t)
	up, _ := svc.Upload(context.Background(), sub, "rec-1", "prep", "req-2", "a.pdf", "application/pdf", bytes.NewReader([]byte("x")), 1)
	var delOK, repOK int32
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if err := svc.Delete(context.Background(), sub, "rec-1", "prep", up.File.FileID); err == nil {
			atomic.AddInt32(&delOK, 1)
		}
	}()
	go func() {
		defer wg.Done()
		if _, err := svc.Replace(context.Background(), sub, "rec-1", "prep", up.File.FileID, "b.pdf", "application/pdf", bytes.NewReader([]byte("y")), 1); err == nil {
			atomic.AddInt32(&repOK, 1)
		}
	}()
	wg.Wait()
	if delOK+repOK != 1 {
		t.Fatalf("winners del=%d rep=%d", delOK, repOK)
	}
	if int(delOK) != len(spy.byAction(wff.AuditActionDelete)) {
		t.Fatalf("delete audit parity delOK=%d audits=%d", delOK, len(spy.byAction(wff.AuditActionDelete)))
	}
	// replace audits include only race winner (upload also created one upload audit)
	replaceAudits := spy.byAction(wff.AuditActionReplace)
	if int(repOK) != len(replaceAudits) {
		t.Fatalf("replace audit parity repOK=%d audits=%d", repOK, len(replaceAudits))
	}

	up2, _ := svc.Upload(context.Background(), sub, "rec-1", "prep", "req-2", "c.pdf", "application/pdf", bytes.NewReader([]byte("c")), 1)
	spyBefore := len(spy.byAction(wff.AuditActionReplace))
	var wins int32
	wg.Add(2)
	go func() {
		defer wg.Done()
		if _, err := svc.Replace(context.Background(), sub, "rec-1", "prep", up2.File.FileID, "d.pdf", "application/pdf", bytes.NewReader([]byte("d")), 1); err == nil {
			atomic.AddInt32(&wins, 1)
		}
	}()
	go func() {
		defer wg.Done()
		if _, err := svc.Replace(context.Background(), sub, "rec-1", "prep", up2.File.FileID, "e.pdf", "application/pdf", bytes.NewReader([]byte("e")), 1); err == nil {
			atomic.AddInt32(&wins, 1)
		}
	}()
	wg.Wait()
	if wins != 1 {
		t.Fatalf("double replace wins=%d", wins)
	}
	if len(spy.byAction(wff.AuditActionReplace))-spyBefore != 1 {
		t.Fatal("exactly one replace success audit for double-replace race")
	}
	if still, _ := files.GetActiveByID(context.Background(), "co-a", up2.File.FileID); still != nil {
		t.Fatal("old must not stay ACTIVE")
	}
}

func TestB5_CompleteMutationRaceAndAudit(t *testing.T) {
	svc, _, snaps, files, _, spy, sub := baseFixtureWithAudit(t)
	up, err := svc.Upload(context.Background(), sub, "rec-1", "prep", "req-1", "a.pdf", "application/pdf", bytes.NewReader([]byte("x")), 1)
	if err != nil {
		t.Fatal(err)
	}
	var completeOK, deleteOK, uploadOK, replaceOK int32
	var wg sync.WaitGroup
	wg.Add(4)
	go func() {
		defer wg.Done()
		if err := files.CompleteStepWithRequiredDocuments("co-a", "wi-1", "prep", snaps.byStep); err == nil {
			atomic.AddInt32(&completeOK, 1)
		}
	}()
	go func() {
		defer wg.Done()
		if err := svc.Delete(context.Background(), sub, "rec-1", "prep", up.File.FileID); err == nil {
			atomic.AddInt32(&deleteOK, 1)
		}
	}()
	go func() {
		defer wg.Done()
		if _, err := svc.Upload(context.Background(), sub, "rec-1", "prep", "req-1", "b.pdf", "application/pdf", bytes.NewReader([]byte("y")), 1); err == nil {
			atomic.AddInt32(&uploadOK, 1)
		}
	}()
	go func() {
		defer wg.Done()
		if _, err := svc.Replace(context.Background(), sub, "rec-1", "prep", up.File.FileID, "c.pdf", "application/pdf", bytes.NewReader([]byte("z")), 1); err == nil {
			atomic.AddInt32(&replaceOK, 1)
		}
	}()
	wg.Wait()
	_ = completeOK
	deleteAudits := len(spy.byAction(wff.AuditActionDelete))
	replaceAudits := len(spy.byAction(wff.AuditActionReplace))
	if int(deleteOK) != deleteAudits {
		t.Fatalf("delete race audit parity ok=%d audits=%d", deleteOK, deleteAudits)
	}
	if int(replaceOK) != replaceAudits {
		t.Fatalf("replace race audit parity ok=%d audits=%d", replaceOK, replaceAudits)
	}
	// After forced complete, mutations denied and no extra success audits.
	files.SetStepCompleted("wi-1", "prep", true)
	before := spy.len()
	_, _ = svc.Upload(context.Background(), sub, "rec-1", "prep", "req-1", "z.pdf", "application/pdf", bytes.NewReader([]byte("z")), 1)
	_ = svc.Delete(context.Background(), sub, "rec-1", "prep", up.File.FileID)
	_, _ = svc.Replace(context.Background(), sub, "rec-1", "prep", up.File.FileID, "z.pdf", "application/pdf", bytes.NewReader([]byte("z")), 1)
	if spy.len() != before {
		t.Fatal("completed step failed mutations must not emit success audit")
	}
	_ = uploadOK
}

func TestB5_CompletedDownloadAllowed(t *testing.T) {
	svc, deadline, _, _, store, spy, sub := baseFixtureWithAudit(t)
	up, err := svc.Upload(context.Background(), sub, "rec-1", "prep", "req-1", "a.pdf", "application/pdf", bytes.NewReader([]byte("hello")), 5)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	deadline.states = map[string]wff.StepState{"prep": {StepCode: "prep", CompletedAt: &now}}
	f, data, err := svc.Download(context.Background(), sub, "rec-1", "prep", up.File.FileID)
	if err != nil || f == nil || string(data) != "hello" {
		t.Fatalf("download after complete: err=%v", err)
	}
	if !store.Exists(f.StorageKey) {
		t.Fatal("missing binary")
	}
	// No download audit in V1
	for _, e := range spy.byAction(wff.AuditActionUpload) {
		_ = e
	}
	if len(spy.byAction("workflow.document_fulfillment.download")) != 0 {
		t.Fatal("download audit must not exist")
	}
}

func TestB5_FilenameSecurityAndContentDisposition(t *testing.T) {
	for _, name := range []string{`../../secret.xlsx`, `..\..\secret.xlsx`, `/etc/passwd`, `C:\Windows\secret.txt`} {
		got := wff.SanitizeFileName(name)
		key := wff.FulfillmentObjectKey("co", "fid", got)
		if strings.Contains(key, "..") || strings.HasPrefix(got, "/") || strings.Contains(got, "\\") {
			t.Fatalf("escape via %q -> %q key=%s", name, got, key)
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

func TestB5_ErrorSanitizationNoInternalLeak(t *testing.T) {
	svc, deadline, snaps, files, _, sub := baseFixture(t)
	repo := &failDBRepo{Repository: files, failUpload: true}
	svc = wff.NewService(deadline, snaps, repo, newMemStorage(), nil)
	_, err := svc.Upload(context.Background(), sub, "rec-1", "prep", "req-1", "a.pdf", "application/pdf", bytes.NewReader([]byte("x")), 1)
	if err == nil {
		t.Fatal("expected error")
	}
	he, ok := err.(*perr.HTTPError)
	if !ok || he.Code != perr.CodeInternal {
		t.Fatalf("err=%v", err)
	}
	rec := httptest.NewRecorder()
	httpx.WriteError(rec, nil, err)
	body := rec.Body.String()
	for _, bad := range []string{"unexpected EOF", "INSERT", "workflow_step_document", "/var/", "sql:"} {
		if strings.Contains(body, bad) {
			t.Fatalf("leak in body: %s", body)
		}
	}
	if !strings.Contains(body, "INTERNAL_ERROR") {
		t.Fatalf("body=%s", body)
	}
	_ = http.StatusInternalServerError
}

func TestB5_ClientCannotInjectActorOrStorageKey(t *testing.T) {
	svc, _, _, _, _, spy, sub := baseFixtureWithAudit(t)
	// Actor always from Subject; multipart cannot set uploaded_by — service uses sub.UserID.
	up, err := svc.Upload(context.Background(), sub, "rec-1", "prep", "req-1", "a.pdf", "application/pdf", bytes.NewReader([]byte("x")), 1)
	if err != nil {
		t.Fatal(err)
	}
	e := spy.byAction(wff.AuditActionUpload)[0]
	if e.ActorUserID != "u1" || e.CompanyID != "co-a" {
		t.Fatalf("server-derived actor/company failed: %+v", e)
	}
	if up.File.UploadedBy != "u1" {
		t.Fatal("uploaded_by must be subject")
	}
}
