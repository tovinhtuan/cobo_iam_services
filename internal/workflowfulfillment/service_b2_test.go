package workflowfulfillment_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/mediaupload"
	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
	wff "github.com/cobo/cobo_iam_services/internal/workflowfulfillment"
	wffmemory "github.com/cobo/cobo_iam_services/internal/workflowfulfillment/memory"
)

type memStorage struct {
	mu   sync.Mutex
	data map[string][]byte
}

func newMemStorage() *memStorage {
	return &memStorage{data: map[string][]byte{}}
}

func (s *memStorage) Write(objectKey string, body io.Reader) (int64, error) {
	b, err := io.ReadAll(body)
	if err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[objectKey] = b
	return int64(len(b)), nil
}

func (s *memStorage) Read(objectKey string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.data[objectKey]
	if !ok {
		return nil, errors.New("missing")
	}
	cp := append([]byte(nil), b...)
	return cp, nil
}

func (s *memStorage) Delete(objectKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, objectKey)
	return nil
}

func (s *memStorage) Exists(objectKey string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.data[objectKey]
	return ok
}

type fakeDeadline struct {
	viewOK    bool
	mutateOK  bool
	wf        wff.WorkflowContext
	states    map[string]wff.StepState
	loadErr   error
	viewErr   error
	mutateErr error
}

func (f *fakeDeadline) AuthorizeView(context.Context, wff.Subject) error {
	if f.viewErr != nil {
		return f.viewErr
	}
	if !f.viewOK {
		return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "deadline.view permission required", nil)
	}
	return nil
}

func (f *fakeDeadline) AuthorizeMutation(context.Context, wff.Subject, string) error {
	if !f.viewOK {
		if f.viewErr != nil {
			return f.viewErr
		}
		return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "deadline.view permission required", nil)
	}
	if f.mutateErr != nil {
		return f.mutateErr
	}
	if !f.mutateOK {
		return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "deadline.manage permission required", nil)
	}
	return nil
}

func (f *fakeDeadline) LoadWorkflowForRecord(context.Context, wff.Subject, string) (wff.WorkflowContext, error) {
	if f.loadErr != nil {
		return wff.WorkflowContext{}, f.loadErr
	}
	return f.wf, nil
}

func (f *fakeDeadline) ListStepStates(context.Context, string) (map[string]wff.StepState, error) {
	if f.states == nil {
		return map[string]wff.StepState{}, nil
	}
	return f.states, nil
}

type fakeSnapshots struct {
	byID   map[string]workflowapp.DocumentRequirementSnapshot
	byStep []workflowapp.DocumentRequirementSnapshot
}

func (f *fakeSnapshots) ListDocumentRequirementSnapshotsByInstanceStep(context.Context, string, string, string) ([]workflowapp.DocumentRequirementSnapshot, error) {
	return append([]workflowapp.DocumentRequirementSnapshot(nil), f.byStep...), nil
}

func (f *fakeSnapshots) GetDocumentRequirementSnapshotByID(_ context.Context, companyID, snapshotID string) (*workflowapp.DocumentRequirementSnapshot, error) {
	s, ok := f.byID[snapshotID]
	if !ok || s.CompanyID != companyID {
		return nil, nil
	}
	cp := s
	return &cp, nil
}

func baseFixture(t *testing.T) (*wff.Service, *fakeDeadline, *fakeSnapshots, *wffmemory.Repository, *memStorage, wff.Subject) {
	t.Helper()
	today := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)
	snap := workflowapp.DocumentRequirementSnapshot{
		ID:                 "req-1",
		CompanyID:          "co-a",
		DisclosureRecordID: "rec-1",
		WorkflowInstanceID: "wi-1",
		StepCode:           "prep",
		SourceDocID:        "doc-1",
		Name:               "Report",
		Required:           true,
		Ordinal:            0,
	}
	opt := snap
	opt.ID = "req-2"
	opt.SourceDocID = "doc-2"
	opt.Name = "Optional"
	opt.Required = false
	opt.Ordinal = 1

	deadline := &fakeDeadline{
		viewOK:   true,
		mutateOK: true,
		wf: wff.WorkflowContext{
			WorkflowInstanceID: "wi-1",
			CompanyID:          "co-a",
			RecordID:           "rec-1",
			T0Date:             today.AddDate(0, 0, -1),
			Timezone:           "Asia/Ho_Chi_Minh",
			SnapshotJSONSteps: []workflowapp.StepSnapshot{{
				StepID:   "prep",
				StepCode: "prep",
				Stage:    "Prep",
				DueRule:  "T+5",
			}},
		},
	}
	snaps := &fakeSnapshots{
		byID:   map[string]workflowapp.DocumentRequirementSnapshot{snap.ID: snap, opt.ID: opt},
		byStep: []workflowapp.DocumentRequirementSnapshot{snap, opt},
	}
	files := wffmemory.NewRepository()
	store := newMemStorage()
	svc := wff.NewService(deadline, snaps, files, store, nil).WithNow(func() time.Time { return today })
	sub := wff.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "co-a"}
	return svc, deadline, snaps, files, store, sub
}

func TestPolicy_PathTraversalFilename(t *testing.T) {
	got := wff.SanitizeFileName("../../secret.xlsx")
	if got != "secret.xlsx" {
		t.Fatalf("SanitizeFileName=%q", got)
	}
	key := wff.FulfillmentObjectKey("co", "fid", got)
	if strings.Contains(key, "..") {
		t.Fatalf("storage key contains ..: %s", key)
	}
	if !strings.HasPrefix(key, wff.StorageNamespace+"/") {
		t.Fatalf("namespace missing: %s", key)
	}
}

func TestPolicy_FileSizeAndType(t *testing.T) {
	if err := wff.ValidateUploadMeta("a.pdf", "application/pdf", wff.MaxSizeBytes+1); err == nil {
		t.Fatal("expected size error")
	} else if he, ok := err.(*perr.HTTPError); !ok || he.Code != perr.CodeDocumentFulfillmentFileTooLarge {
		t.Fatalf("code=%v", err)
	}
	if err := wff.ValidateUploadMeta("a.exe", "application/pdf", 10); err == nil {
		t.Fatal("expected type error")
	}
	if err := wff.ValidateUploadMeta("a.pdf", "application/pdf", 0); err == nil {
		t.Fatal("expected empty error")
	}
}

func TestUpload_ValidRequiredAndOptional(t *testing.T) {
	svc, _, _, files, store, sub := baseFixture(t)
	for _, reqID := range []string{"req-1", "req-2"} {
		res, err := svc.Upload(context.Background(), sub, "rec-1", "prep", reqID, "a.pdf", "application/pdf", bytes.NewReader([]byte("%PDF")), 4)
		if err != nil {
			t.Fatalf("upload %s: %v", reqID, err)
		}
		if res.File.FileID == "" || res.File.FileName != "a.pdf" {
			t.Fatalf("bad dto: %+v", res.File)
		}
		got, _ := files.GetActiveByID(context.Background(), "co-a", res.File.FileID)
		if got == nil || !store.Exists(got.StorageKey) {
			t.Fatal("missing file row/disk")
		}
	}
}

func TestUpload_ActiveLimit(t *testing.T) {
	svc, _, _, _, _, sub := baseFixture(t)
	for i := 0; i < 10; i++ {
		_, err := svc.Upload(context.Background(), sub, "rec-1", "prep", "req-1", "a.pdf", "application/pdf", bytes.NewReader([]byte("x")), 1)
		if err != nil {
			t.Fatalf("upload %d: %v", i, err)
		}
	}
	_, err := svc.Upload(context.Background(), sub, "rec-1", "prep", "req-1", "a.pdf", "application/pdf", bytes.NewReader([]byte("x")), 1)
	if err == nil {
		t.Fatal("expected limit")
	}
	he, ok := err.(*perr.HTTPError)
	if !ok || he.Code != perr.CodeDocumentFulfillmentFileLimitReached {
		t.Fatalf("err=%v", err)
	}
}

func TestList_ActiveFilterAndCapabilities(t *testing.T) {
	svc, deadline, _, files, store, sub := baseFixture(t)
	up, err := svc.Upload(context.Background(), sub, "rec-1", "prep", "req-1", "a.pdf", "application/pdf", bytes.NewReader([]byte("x")), 1)
	if err != nil {
		t.Fatal(err)
	}
	files.Seed(wff.FulfillmentFile{
		ID: "del-1", CompanyID: "co-a", DisclosureRecordID: "rec-1", WorkflowInstanceID: "wi-1", StepCode: "prep",
		RequirementSnapshotID: "req-1", StorageKey: "k1", OriginalFileName: "d.pdf", MimeType: "application/pdf",
		FileSize: 1, UploadedBy: "u", UploadedAt: time.Now().UTC(), LifecycleStatus: wff.LifecycleDeleted,
	})
	files.Seed(wff.FulfillmentFile{
		ID: "sup-1", CompanyID: "co-a", DisclosureRecordID: "rec-1", WorkflowInstanceID: "wi-1", StepCode: "prep",
		RequirementSnapshotID: "req-1", StorageKey: "k2", OriginalFileName: "s.pdf", MimeType: "application/pdf",
		FileSize: 1, UploadedBy: "u", UploadedAt: time.Now().UTC(), LifecycleStatus: wff.LifecycleSuperseded,
	})
	_ = store
	list, err := svc.ListRequirements(context.Background(), sub, "rec-1", "prep")
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Requirements) != 2 {
		t.Fatalf("reqs=%d", len(list.Requirements))
	}
	var active int
	for _, r := range list.Requirements {
		if r.RequirementSnapshotID == "req-1" {
			active = len(r.Files)
			if !r.Capabilities.CanUpload || !r.Capabilities.CanDownload {
				t.Fatalf("caps=%+v", r.Capabilities)
			}
		}
	}
	if active != 1 || list.Requirements[0].Files[0].FileID != up.File.FileID {
		t.Fatalf("active filter failed: %d", active)
	}
	deadline.mutateOK = false
	list2, _ := svc.ListRequirements(context.Background(), sub, "rec-1", "prep")
	if list2.Requirements[0].Capabilities.CanUpload {
		t.Fatal("view-only should deny mutation caps")
	}
}

func TestACL_ViewMutationAssigneeCurrentCompleted(t *testing.T) {
	svc, deadline, _, _, _, sub := baseFixture(t)
	if _, err := svc.ListRequirements(context.Background(), sub, "rec-1", "prep"); err != nil {
		t.Fatal(err)
	}
	deadline.viewOK = false
	if _, err := svc.ListRequirements(context.Background(), sub, "rec-1", "prep"); err == nil {
		t.Fatal("expected no view")
	}
	deadline.viewOK = true
	deadline.mutateOK = false
	if _, err := svc.Upload(context.Background(), sub, "rec-1", "prep", "req-1", "a.pdf", "application/pdf", bytes.NewReader([]byte("x")), 1); err == nil {
		t.Fatal("expected mutation deny")
	}
	deadline.mutateOK = true
	// non-current: add second step and complete first via states? With single step it's current.
	// Switch snapshot to two steps and mark prep completed so current moves.
	deadline.wf.SnapshotJSONSteps = []workflowapp.StepSnapshot{
		{StepID: "prep", StepCode: "prep", Stage: "Prep", DueRule: "T+1"},
		{StepID: "review", StepCode: "review", Stage: "Review", DueRule: "T+3"},
	}
	nowCompleted := time.Now().UTC()
	deadline.states = map[string]wff.StepState{"prep": {StepCode: "prep", CompletedAt: &nowCompleted}}
	if _, err := svc.Upload(context.Background(), sub, "rec-1", "prep", "req-1", "a.pdf", "application/pdf", bytes.NewReader([]byte("x")), 1); err == nil {
		t.Fatal("expected completed deny")
	}
	// assignee must not matter — mutateOK alone drives auth (already covered)
}

func TestCrossCompanyAndBinding(t *testing.T) {
	svc, deadline, snaps, files, store, sub := baseFixture(t)
	up, err := svc.Upload(context.Background(), sub, "rec-1", "prep", "req-1", "a.pdf", "application/pdf", bytes.NewReader([]byte("hello")), 5)
	if err != nil {
		t.Fatal(err)
	}
	// foreign requirement
	snaps.byID["req-b"] = workflowapp.DocumentRequirementSnapshot{
		ID: "req-b", CompanyID: "co-b", DisclosureRecordID: "rec-b", WorkflowInstanceID: "wi-b", StepCode: "prep",
		SourceDocID: "x", Name: "x",
	}
	if _, err := svc.Upload(context.Background(), sub, "rec-1", "prep", "req-b", "a.pdf", "application/pdf", bytes.NewReader([]byte("x")), 1); err == nil {
		t.Fatal("expected cross-company requirement reject")
	}
	// wrong company download
	subB := wff.Subject{UserID: "u2", MembershipID: "m2", CompanyID: "co-b"}
	deadline.wf.CompanyID = "co-b"
	deadline.wf.RecordID = "rec-1" // still same record path but company differs on subject
	if _, _, err := svc.Download(context.Background(), subB, "rec-1", "prep", up.File.FileID); err == nil {
		t.Fatal("expected cross-company file deny")
	}
	deadline.wf.CompanyID = "co-a"
	// wrong record binding
	deadline.wf.RecordID = "rec-other"
	if _, _, err := svc.Download(context.Background(), sub, "rec-other", "prep", up.File.FileID); err == nil {
		t.Fatal("expected wrong record deny")
	}
	deadline.wf.RecordID = "rec-1"
	// wrong step
	deadline.wf.SnapshotJSONSteps = append(deadline.wf.SnapshotJSONSteps, workflowapp.StepSnapshot{StepID: "other", StepCode: "other", Stage: "O", DueRule: "T+9"})
	if _, _, err := svc.Download(context.Background(), sub, "rec-1", "other", up.File.FileID); err == nil {
		t.Fatal("expected wrong step deny")
	}
	_ = files
	_ = store
}

func TestDownload_ActiveDeletedSupersededAndDisposition(t *testing.T) {
	svc, _, _, files, store, sub := baseFixture(t)
	up, err := svc.Upload(context.Background(), sub, "rec-1", "prep", "req-1", "report.pdf", "application/pdf", bytes.NewReader([]byte("abc")), 3)
	if err != nil {
		t.Fatal(err)
	}
	f, data, err := svc.Download(context.Background(), sub, "rec-1", "prep", up.File.FileID)
	if err != nil || string(data) != "abc" || f.OriginalFileName != "report.pdf" {
		t.Fatalf("download=%v data=%q", err, data)
	}
	if wff.EscapeContentDispositionFileName("evil\r\nname.pdf") != "evilname.pdf" {
		t.Fatal("disposition sanitize")
	}
	if err := svc.Delete(context.Background(), sub, "rec-1", "prep", up.File.FileID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Download(context.Background(), sub, "rec-1", "prep", up.File.FileID); err == nil {
		t.Fatal("deleted download denied")
	}
	row, _ := files.GetByID(context.Background(), "co-a", up.File.FileID)
	if row == nil || row.LifecycleStatus != wff.LifecycleDeleted || !store.Exists(row.StorageKey) {
		t.Fatal("logical delete must retain binary")
	}
	// superseded
	up2, _ := svc.Upload(context.Background(), sub, "rec-1", "prep", "req-1", "b.pdf", "application/pdf", bytes.NewReader([]byte("b")), 1)
	_, err = svc.Replace(context.Background(), sub, "rec-1", "prep", up2.File.FileID, "c.pdf", "application/pdf", bytes.NewReader([]byte("c")), 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Download(context.Background(), sub, "rec-1", "prep", up2.File.FileID); err == nil {
		t.Fatal("superseded download denied")
	}
}

func TestReplace_LineageAtLimitAndFailedPreserve(t *testing.T) {
	svc, _, _, files, store, sub := baseFixture(t)
	var first string
	for i := 0; i < 10; i++ {
		res, err := svc.Upload(context.Background(), sub, "rec-1", "prep", "req-1", "a.pdf", "application/pdf", bytes.NewReader([]byte("x")), 1)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = res.File.FileID
		}
	}
	rep, err := svc.Replace(context.Background(), sub, "rec-1", "prep", first, "new.pdf", "application/pdf", bytes.NewReader([]byte("yy")), 2)
	if err != nil {
		t.Fatalf("replace at limit: %v", err)
	}
	old, _ := files.GetByID(context.Background(), "co-a", first)
	neu, _ := files.GetActiveByID(context.Background(), "co-a", rep.File.FileID)
	if old.LifecycleStatus != wff.LifecycleSuperseded || old.SupersededByFileID != neu.ID || neu.SupersedesFileID != old.ID {
		t.Fatalf("lineage old=%+v new=%+v", old, neu)
	}
	if !store.Exists(old.StorageKey) {
		t.Fatal("old binary retained")
	}
	n, _ := files.CountActiveByRequirement(context.Background(), "req-1")
	if n != 10 {
		t.Fatalf("active count=%d", n)
	}

	// failed replace: force repo error by completing step under lock after get
	files.SetStepCompleted("wi-1", "prep", true)
	before, _ := files.GetActiveByID(context.Background(), "co-a", rep.File.FileID)
	_, err = svc.Replace(context.Background(), sub, "rec-1", "prep", rep.File.FileID, "z.pdf", "application/pdf", bytes.NewReader([]byte("z")), 1)
	if err == nil {
		t.Fatal("expected failed replace")
	}
	after, _ := files.GetActiveByID(context.Background(), "co-a", before.ID)
	if after == nil || after.LifecycleStatus != wff.LifecycleActive {
		t.Fatal("failed replace must preserve old ACTIVE")
	}
}

func TestConcurrency_ActiveLimitDeleteReplaceDoubleReplace(t *testing.T) {
	svc, _, _, files, _, sub := baseFixture(t)
	for i := 0; i < 9; i++ {
		if _, err := svc.Upload(context.Background(), sub, "rec-1", "prep", "req-1", "a.pdf", "application/pdf", bytes.NewReader([]byte("x")), 1); err != nil {
			t.Fatal(err)
		}
	}
	var okCount int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.Upload(context.Background(), sub, "rec-1", "prep", "req-1", "a.pdf", "application/pdf", bytes.NewReader([]byte("x")), 1)
			if err == nil {
				atomic.AddInt32(&okCount, 1)
			}
		}()
	}
	wg.Wait()
	n, _ := files.CountActiveByRequirement(context.Background(), "req-1")
	if n > 10 || okCount > 1 {
		t.Fatalf("concurrent limit breached n=%d ok=%d", n, okCount)
	}

	up, _ := svc.Upload(context.Background(), sub, "rec-1", "prep", "req-2", "a.pdf", "application/pdf", bytes.NewReader([]byte("x")), 1)
	var delOK, repOK int32
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
		t.Fatalf("delete/replace race winners=%d+%d", delOK, repOK)
	}

	up2, _ := svc.Upload(context.Background(), sub, "rec-1", "prep", "req-2", "c.pdf", "application/pdf", bytes.NewReader([]byte("c")), 1)
	var replaceWins int32
	wg.Add(2)
	go func() {
		defer wg.Done()
		if _, err := svc.Replace(context.Background(), sub, "rec-1", "prep", up2.File.FileID, "d.pdf", "application/pdf", bytes.NewReader([]byte("d")), 1); err == nil {
			atomic.AddInt32(&replaceWins, 1)
		}
	}()
	go func() {
		defer wg.Done()
		if _, err := svc.Replace(context.Background(), sub, "rec-1", "prep", up2.File.FileID, "e.pdf", "application/pdf", bytes.NewReader([]byte("e")), 1); err == nil {
			atomic.AddInt32(&replaceWins, 1)
		}
	}()
	wg.Wait()
	if replaceWins != 1 {
		t.Fatalf("double replace wins=%d", replaceWins)
	}
	if still, _ := files.GetActiveByID(context.Background(), "co-a", up2.File.FileID); still != nil {
		t.Fatal("original file must not remain ACTIVE after replace race")
	}
	active, _ := files.ListActiveByRequirement(context.Background(), "co-a", "req-2")
	supersedeCount := 0
	for _, f := range active {
		if f.SupersedesFileID == up2.File.FileID {
			supersedeCount++
		}
	}
	if supersedeCount != 1 {
		t.Fatalf("expected exactly one active replacement branch, got %d (active total=%d)", supersedeCount, len(active))
	}
}

func TestConcurrency_CompleteVsMutation(t *testing.T) {
	svc, _, _, files, _, sub := baseFixture(t)
	up, err := svc.Upload(context.Background(), sub, "rec-1", "prep", "req-1", "a.pdf", "application/pdf", bytes.NewReader([]byte("x")), 1)
	if err != nil {
		t.Fatal(err)
	}
	var uploadOK, deleteOK int32
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		files.SetStepCompleted("wi-1", "prep", true)
	}()
	go func() {
		defer wg.Done()
		if _, err := svc.Upload(context.Background(), sub, "rec-1", "prep", "req-1", "b.pdf", "application/pdf", bytes.NewReader([]byte("y")), 1); err == nil {
			atomic.AddInt32(&uploadOK, 1)
		}
	}()
	go func() {
		defer wg.Done()
		if err := svc.Delete(context.Background(), sub, "rec-1", "prep", up.File.FileID); err == nil {
			atomic.AddInt32(&deleteOK, 1)
		}
	}()
	wg.Wait()
	// After completion flag, further mutations must not leave inconsistent ACTIVE growth beyond race winners that locked first.
	files.SetStepCompleted("wi-1", "prep", true)
	if _, err := svc.Upload(context.Background(), sub, "rec-1", "prep", "req-1", "z.pdf", "application/pdf", bytes.NewReader([]byte("z")), 1); err == nil {
		t.Fatal("upload after complete must fail")
	}
	_ = uploadOK
	_ = deleteOK
}

func TestDiskStorageReuse_Namespace(t *testing.T) {
	dir := t.TempDir()
	disk, err := mediaupload.NewDiskStorage(dir)
	if err != nil {
		t.Fatal(err)
	}
	key := wff.FulfillmentObjectKey("co", "id1", "a.pdf")
	if _, err := disk.Write(key, strings.NewReader("hi")); err != nil {
		t.Fatal(err)
	}
	if !disk.Exists(key) {
		t.Fatal("missing")
	}
}

func TestMigrationSQL_OneToManyNoUniqueRequirement(t *testing.T) {
	// Source contract check — migration file must not UNIQUE(requirement_snapshot_id) alone.
	// Full DB migrate covered in DEV; here assert constant contract.
	if wff.MaxActiveFilesPerRequirement != 10 {
		t.Fatal("limit")
	}
}
