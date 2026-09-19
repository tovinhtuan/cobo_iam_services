package workflowstepcomments_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idempotency"
	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
	wff "github.com/cobo/cobo_iam_services/internal/workflowfulfillment"
	wsc "github.com/cobo/cobo_iam_services/internal/workflowstepcomments"
)

type fakeCommentDeadline struct {
	viewOK    bool
	commentOK bool
	perms     map[string]bool
	wf        wff.WorkflowContext
	states    map[string]wff.StepState
	company   string
	stepStat  string // override Classify path via states + snapshot

	// Optional per-membership deny lists for mention-notify recipient authz tests.
	viewDenyMembership  map[string]bool
	scopeDenyMembership map[string]bool
}

func (f *fakeCommentDeadline) AuthorizeView(_ context.Context, sub wff.Subject) error {
	if f.viewDenyMembership[sub.MembershipID] {
		return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "deadline.view permission required", nil)
	}
	if !f.viewOK {
		return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "deadline.view permission required", nil)
	}
	return nil
}

func (f *fakeCommentDeadline) AuthorizeMutation(context.Context, wff.Subject, string) error {
	return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "deadline.manage permission required", nil)
}

func (f *fakeCommentDeadline) AuthorizeComment(context.Context, wff.Subject) error {
	if !f.commentOK {
		return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "deadline.comment permission required", nil)
	}
	return nil
}

func (f *fakeCommentDeadline) HasPermission(_ context.Context, _ wff.Subject, code string) (bool, error) {
	return f.perms[code], nil
}

func (f *fakeCommentDeadline) LoadWorkflowForRecord(_ context.Context, sub wff.Subject, recordID string) (wff.WorkflowContext, error) {
	if f.scopeDenyMembership[sub.MembershipID] {
		return wff.WorkflowContext{}, perr.NewHTTPError(http.StatusForbidden, perr.CodeDataScopeDenied, "record outside data scope", nil)
	}
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

func (f *fakeCommentDeadline) ListStepStates(context.Context, string) (map[string]wff.StepState, error) {
	if f.states == nil {
		return map[string]wff.StepState{}, nil
	}
	return f.states, nil
}

type memIdem struct {
	mu    sync.Mutex
	rows  map[string]idemRow
	failC bool
}

type idemRow struct {
	hash   string
	status string
	body   []byte
	http   int
}

func newMemIdem() *memIdem {
	return &memIdem{rows: map[string]idemRow{}}
}

func (m *memIdem) TryReserve(_ context.Context, p idempotency.Params) (idempotency.Result, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := p.Scope + "|" + p.Key
	if row, ok := m.rows[key]; ok {
		if row.status == "in_progress" {
			return idempotency.Result{Conflict: true}, nil
		}
		if row.hash != p.RequestHash {
			return idempotency.Result{Conflict: true}, nil
		}
		return idempotency.Result{Replay: true, ReplayHTTPStatus: row.http, ReplayBody: row.body}, nil
	}
	m.rows[key] = idemRow{hash: p.RequestHash, status: "in_progress"}
	return idempotency.Result{ReservationID: key}, nil
}

func (m *memIdem) Complete(_ context.Context, reservationID string, responseEnvelopeJSON []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failC {
		return errors.New("complete failed")
	}
	var env idempotency.Envelope
	_ = json.Unmarshal(responseEnvelopeJSON, &env)
	row := m.rows[reservationID]
	row.status = "completed"
	row.body = env.Body
	row.http = env.HTTPStatus
	m.rows[reservationID] = row
	return nil
}

func (m *memIdem) Abandon(_ context.Context, reservationID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rows, reservationID)
	return nil
}

type failAudit struct{ calls int }

func (f *failAudit) AppendAuditLog(context.Context, auditapp.AppendAuditLogRequest) error {
	f.calls++
	return errors.New("audit down")
}

func baseWF(status string) wff.WorkflowContext {
	return wff.WorkflowContext{
		WorkflowInstanceID: "wi-1",
		CompanyID:          "c1",
		RecordID:           "r1",
		RecordStatus:       status,
		T0Date:             time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		Timezone:           "Asia/Ho_Chi_Minh",
		SnapshotJSONSteps: []workflowapp.StepSnapshot{
			{StepCode: "s1", DisplayOrder: 1, ProcessingDays: 5},
		},
	}
}

func newSvc(t *testing.T, d *fakeCommentDeadline, repo *wsc.MemoryRepository, idem idempotency.Store) *wsc.Service {
	t.Helper()
	if d.stepStat == "" {
		d.stepStat = "current"
	}
	// Mark step current via empty states + T0 covering today — Classify uses ComputeDeadlineSteps.
	// For deterministic tests, seed CompletedAt only when needed; otherwise force via completed state.
	svc := wsc.NewService(d, repo, nil).WithIdempotency(idem).WithNow(func() time.Time {
		return time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	})
	return svc
}

func TestMemoryRepo_TenantIsolationAndSoftDelete(t *testing.T) {
	repo := wsc.NewMemoryRepository()
	now := time.Now().UTC()
	ownerA := wsc.OwnerContext{CompanyID: "c1", DisclosureRecordID: "r1", WorkflowInstanceID: "wi1", StepCode: "s1"}
	ownerB := wsc.OwnerContext{CompanyID: "c2", DisclosureRecordID: "r1", WorkflowInstanceID: "wi1", StepCode: "s1"}
	repo.Seed(wsc.Comment{ID: "1", CompanyID: "c1", DisclosureRecordID: "r1", WorkflowInstanceID: "wi1", StepCode: "s1", Body: "a", CreatedAt: now})
	repo.Seed(wsc.Comment{ID: "2", CompanyID: "c2", DisclosureRecordID: "r1", WorkflowInstanceID: "wi1", StepCode: "s1", Body: "b", CreatedAt: now.Add(time.Second)})
	del := now.Add(2 * time.Second)
	repo.Seed(wsc.Comment{ID: "3", CompanyID: "c1", DisclosureRecordID: "r1", WorkflowInstanceID: "wi1", StepCode: "s1", Body: "gone", CreatedAt: now, DeletedAt: &del})

	items, total, err := repo.ListAliveByStep(context.Background(), ownerA, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(items) != 1 || items[0].ID != "1" {
		t.Fatalf("got total=%d items=%v", total, items)
	}
	itemsB, totalB, err := repo.ListAliveByStep(context.Background(), ownerB, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if totalB != 1 || itemsB[0].ID != "2" {
		t.Fatalf("tenant B isolation failed: %+v", itemsB)
	}
}

func TestMemoryRepo_PathMismatchAndPagination(t *testing.T) {
	repo := wsc.NewMemoryRepository()
	now := time.Now().UTC()
	owner := wsc.OwnerContext{CompanyID: "c1", DisclosureRecordID: "r1", WorkflowInstanceID: "wi1", StepCode: "s1"}
	for i := 0; i < 5; i++ {
		repo.Seed(wsc.Comment{
			ID: string(rune('a' + i)), CompanyID: "c1", DisclosureRecordID: "r1",
			WorkflowInstanceID: "wi1", StepCode: "s1", Body: "x", CreatedAt: now.Add(time.Duration(i) * time.Second),
		})
	}
	page1, total, err := repo.ListAliveByStep(context.Background(), owner, 1, 2)
	if err != nil || total != 5 || len(page1) != 2 {
		t.Fatalf("page1 total=%d n=%d err=%v", total, len(page1), err)
	}
	got, err := repo.GetByIDInContext(context.Background(), wsc.OwnerContext{
		CompanyID: "c1", DisclosureRecordID: "r1", WorkflowInstanceID: "wi1", StepCode: "other",
	}, "a")
	if err == nil && got != nil {
		t.Fatal("expected path mismatch error")
	}
}

func TestList_RequiresViewAndReturnsCapabilities(t *testing.T) {
	repo := wsc.NewMemoryRepository()
	d := &fakeCommentDeadline{
		viewOK: false, commentOK: true,
		wf: baseWF("Draft"),
		states: map[string]wff.StepState{},
	}
	svc := newSvc(t, d, repo, nil)
	sub := wff.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"}
	_, err := svc.ListComments(context.Background(), sub, "r1", "s1", 1, 20)
	if err == nil {
		t.Fatal("expected forbidden")
	}
	d.viewOK = true
	d.commentOK = true
	// Force step classification: mark s1 completed so status=completed (create allowed)
	completed := time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC)
	d.states = map[string]wff.StepState{"s1": {StepCode: "s1", CompletedAt: &completed}}
	resp, err := svc.ListComments(context.Background(), sub, "r1", "s1", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Total != 0 || !resp.Capabilities.CanCreate {
		t.Fatalf("empty list can_create want true got %+v", resp)
	}
}

func TestCreate_PlainUUIDFitsVARCHAR36(t *testing.T) {
	repo := wsc.NewMemoryRepository()
	completed := time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC)
	d := &fakeCommentDeadline{
		viewOK: true, commentOK: true,
		wf:     baseWF("In Progress"),
		states: map[string]wff.StepState{"s1": {StepCode: "s1", CompletedAt: &completed}},
	}
	svc := newSvc(t, d, repo, nil)
	sub := wff.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"}
	resp, status, err := svc.CreateComment(context.Background(), sub, "r1", "s1", "id-format-key", "plain uuid id", nil)
	if err != nil || status != http.StatusCreated {
		t.Fatalf("create status=%d err=%v", status, err)
	}
	id := resp.Comment.CommentID
	if len(id) != wsc.CommentIDLength {
		t.Fatalf("comment_id length=%d want %d (id=%q)", len(id), wsc.CommentIDLength, id)
	}
	if len(id) > 36 {
		t.Fatalf("comment_id exceeds VARCHAR(36): %q", id)
	}
	if len(id) >= 4 && id[:4] == "wsc_" {
		t.Fatalf("comment_id must not use wsc_ prefix: %q", id)
	}
	owner := wsc.OwnerContext{
		CompanyID: "c1", DisclosureRecordID: "r1", WorkflowInstanceID: "wi-1", StepCode: "s1",
	}
	alive, err := repo.GetByIDInContext(context.Background(), owner, id)
	if err != nil || alive == nil || alive.ID != id {
		t.Fatalf("list path should return same comment_id: err=%v got=%v", err, alive)
	}
	if err := svc.DeleteComment(context.Background(), sub, "r1", "s1", id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	items, total, err := repo.ListAliveByStep(context.Background(), owner, 1, 20)
	if err != nil || total != 0 || len(items) != 0 {
		t.Fatalf("soft-deleted comment must leave list empty: total=%d n=%d err=%v", total, len(items), err)
	}
	// Soft-deleted ID remains addressable only as already-deleted (idempotent delete path)
	if err := svc.DeleteComment(context.Background(), sub, "r1", "s1", id); err == nil {
		t.Fatal("expected already-deleted on second delete with same id")
	}
}

func TestCreate_IdempotencyReplayAndConflict(t *testing.T) {
	repo := wsc.NewMemoryRepository()
	idem := newMemIdem()
	completed := time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC)
	d := &fakeCommentDeadline{
		viewOK: true, commentOK: true,
		wf:     baseWF("In Progress"),
		states: map[string]wff.StepState{"s1": {StepCode: "s1", CompletedAt: &completed}},
	}
	svc := newSvc(t, d, repo, idem)
	sub := wff.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"}
	r1, status, err := svc.CreateComment(context.Background(), sub, "r1", "s1", "key-1", "hello", nil)
	if err != nil || status != http.StatusCreated {
		t.Fatalf("create1 status=%d err=%v", status, err)
	}
	r2, status2, err := svc.CreateComment(context.Background(), sub, "r1", "s1", "key-1", "hello", nil)
	if err != nil || status2 != http.StatusCreated {
		t.Fatalf("replay status=%d err=%v", status2, err)
	}
	if r1.Comment.CommentID != r2.Comment.CommentID {
		t.Fatalf("replay should return same comment")
	}
	_, _, err = svc.CreateComment(context.Background(), sub, "r1", "s1", "key-1", "other", nil)
	if err == nil {
		t.Fatal("expected hash conflict")
	}
	// different actor independent
	sub2 := wff.Subject{UserID: "u2", MembershipID: "m2", CompanyID: "c1"}
	_, _, err = svc.CreateComment(context.Background(), sub2, "r1", "s1", "key-1", "hello", nil)
	if err != nil {
		t.Fatalf("different actor should be independent: %v", err)
	}
}

func TestCreate_AuthzBeforeReserveAndTerminalFreeze(t *testing.T) {
	repo := wsc.NewMemoryRepository()
	idem := newMemIdem()
	d := &fakeCommentDeadline{
		viewOK: true, commentOK: false,
		wf: baseWF("Draft"),
		states: map[string]wff.StepState{
			"s1": {StepCode: "s1", CompletedAt: ptrTime(time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC))},
		},
	}
	svc := newSvc(t, d, repo, idem)
	sub := wff.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"}
	_, _, err := svc.CreateComment(context.Background(), sub, "r1", "s1", "k", "body", nil)
	if err == nil {
		t.Fatal("expected authz fail")
	}
	if len(idem.rows) != 0 {
		t.Fatal("authz fail must not reserve idempotency")
	}
	d.commentOK = true
	d.wf.RecordStatus = "published"
	_, _, err = svc.CreateComment(context.Background(), sub, "r1", "s1", "k2", "body", nil)
	if err == nil {
		t.Fatal("expected terminal freeze")
	}
}

func TestEditDelete_AuthorWindowAndAdminOverride(t *testing.T) {
	repo := wsc.NewMemoryRepository()
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	d := &fakeCommentDeadline{
		viewOK: true, commentOK: true,
		perms:  map[string]bool{},
		wf:     baseWF("Draft"),
		states: map[string]wff.StepState{"s1": {StepCode: "s1", CompletedAt: ptrTime(now.Add(-48 * time.Hour))}},
	}
	svc := wsc.NewService(d, repo, nil).WithNow(func() time.Time { return now })
	sub := wff.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"}
	created := now.Add(-2 * time.Hour)
	repo.Seed(wsc.Comment{
		ID: "cmt1", CompanyID: "c1", DisclosureRecordID: "r1", WorkflowInstanceID: "wi-1", StepCode: "s1",
		AuthorUserID: "u1", AuthorMembershipID: "m1", Body: "old", CreatedAt: created,
	})
	resp, err := svc.UpdateComment(context.Background(), sub, "r1", "s1", "cmt1", "new", nil)
	if err != nil || resp.Comment.Body != "new" {
		t.Fatalf("author edit: %v %+v", err, resp)
	}
	// outside 24h
	old := now.Add(-48 * time.Hour)
	repo.Seed(wsc.Comment{
		ID: "cmt2", CompanyID: "c1", DisclosureRecordID: "r1", WorkflowInstanceID: "wi-1", StepCode: "s1",
		AuthorUserID: "u1", AuthorMembershipID: "m1", Body: "old", CreatedAt: old,
	})
	_, err = svc.UpdateComment(context.Background(), sub, "r1", "s1", "cmt2", "x", nil)
	if err == nil {
		t.Fatal("expected window deny")
	}
	d.perms["rbac.manage"] = true
	_, err = svc.UpdateComment(context.Background(), sub, "r1", "s1", "cmt2", "admin", nil)
	if err != nil {
		t.Fatalf("admin override: %v", err)
	}
	// admin cannot bypass terminal freeze
	d.wf.RecordStatus = "completed"
	_, err = svc.UpdateComment(context.Background(), sub, "r1", "s1", "cmt2", "nope", nil)
	if err == nil {
		t.Fatal("admin must not bypass freeze")
	}
	err = svc.DeleteComment(context.Background(), sub, "r1", "s1", "cmt1")
	if err == nil {
		t.Fatal("delete on terminal should fail")
	}
	d.wf.RecordStatus = "Draft"
	if err := svc.DeleteComment(context.Background(), sub, "r1", "s1", "cmt1"); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteComment(context.Background(), sub, "r1", "s1", "cmt1"); err == nil {
		t.Fatal("second delete should conflict")
	}
}

func TestAuditFailureDoesNotRollback(t *testing.T) {
	repo := wsc.NewMemoryRepository()
	idem := newMemIdem()
	aud := &failAudit{}
	d := &fakeCommentDeadline{
		viewOK: true, commentOK: true,
		wf:     baseWF("Draft"),
		states: map[string]wff.StepState{"s1": {StepCode: "s1", CompletedAt: ptrTime(time.Now())}},
	}
	svc := wsc.NewService(d, repo, nil).WithIdempotency(idem).WithAudit(aud).WithNow(func() time.Time {
		return time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	})
	sub := wff.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"}
	resp, _, err := svc.CreateComment(context.Background(), sub, "r1", "s1", "ak", "audited", nil)
	if err != nil {
		t.Fatal(err)
	}
	if aud.calls != 1 {
		t.Fatalf("audit calls=%d", aud.calls)
	}
	items, total, _ := repo.ListAliveByStep(context.Background(), wsc.OwnerContext{
		CompanyID: "c1", DisclosureRecordID: "r1", WorkflowInstanceID: "wi-1", StepCode: "s1",
	}, 1, 20)
	if total != 1 || items[0].ID != resp.Comment.CommentID {
		t.Fatal("mutation must remain after audit fail")
	}
}

func TestStatusHelpers(t *testing.T) {
	if !wscExportIsRecordMutateAllowed("Draft") || !wscExportIsRecordMutateAllowed("PendingReview") || !wscExportIsRecordMutateAllowed("In Progress") {
		t.Fatal("allowlist")
	}
	if wscExportIsRecordMutateAllowed("approved") || wscExportIsRecordMutateAllowed("published") {
		t.Fatal("fail-closed")
	}
}

// re-export via test helpers in same package would be better — use status via create behavior instead.
func wscExportIsRecordMutateAllowed(status string) bool {
	repo := wsc.NewMemoryRepository()
	d := &fakeCommentDeadline{
		viewOK: true, commentOK: true,
		wf:     baseWF(status),
		states: map[string]wff.StepState{"s1": {StepCode: "s1", CompletedAt: ptrTime(time.Now())}},
	}
	svc := wsc.NewService(d, repo, nil).WithNow(func() time.Time { return time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC) })
	sub := wff.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"}
	_, _, err := svc.CreateComment(context.Background(), sub, "r1", "s1", "st-"+status, "x", nil)
	return err == nil
}

func ptrTime(t time.Time) *time.Time { return &t }
