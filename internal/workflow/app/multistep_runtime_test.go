package app

import (
	"context"
	"errors"
	"sync"
	"testing"

	adhocapp "github.com/cobo/cobo_iam_services/internal/adhoc/app"
)

func TestMapProposalWorkflowToSnapshot_IncludesAssigneeMembership(t *testing.T) {
	snap := &adhocapp.ProposalWorkflowSnapshot{
		SchemaVersion: 2,
		Frozen:        true,
		Steps: []adhocapp.ProposalWorkflowStep{
			{ID: "ps-a", Order: 1, Name: "A", ProcessingDays: 1, DepartmentID: "d1", AssigneeMembershipID: "m1"},
			{ID: "ps-b", Order: 2, Name: "B", ProcessingDays: 2, DepartmentID: "d2", AssigneeMembershipID: "m2"},
		},
	}
	got := MapProposalWorkflowToSnapshot(snap)
	if got[0].AssigneeMembershipID != "m1" || got[1].AssigneeMembershipID != "m2" {
		t.Fatalf("assignees %#v", got)
	}
	if got[0].Department != "d1" || got[1].ProcessingDays != 2 {
		t.Fatalf("fields %#v", got)
	}
}

func TestNextSnapshotStep_OrderedChain(t *testing.T) {
	snap := []StepSnapshot{
		{StepID: "c", StepCode: "c", DisplayOrder: 3},
		{StepID: "a", StepCode: "a", DisplayOrder: 1},
		{StepID: "b", StepCode: "b", DisplayOrder: 2},
	}
	next, ok := NextSnapshotStep(snap, "a")
	if !ok || next.StepID != "b" {
		t.Fatalf("after a: %#v ok=%v", next, ok)
	}
	next, ok = NextSnapshotStep(snap, "b")
	if !ok || next.StepID != "c" {
		t.Fatalf("after b: %#v ok=%v", next, ok)
	}
	_, ok = NextSnapshotStep(snap, "c")
	if ok {
		t.Fatal("expected no next after final")
	}
}

type countingRecordUpdater struct {
	calls int
}

func (c *countingRecordUpdater) MarkRecordApproved(_ context.Context, _, _, _ string) error {
	c.calls++
	return nil
}

type countingNotifier struct {
	calls int
}

func (c *countingNotifier) NotifyWorkflowApproved(_ context.Context, _, _, _, _ string) error {
	c.calls++
	return nil
}

func seedV2ThreeStep(t *testing.T) (*fakeWorkflowRepository, Service, *seqIDGen) {
	t.Helper()
	repo := &fakeWorkflowRepository{}
	ids := &seqIDGen{}
	svc := NewService(repo, allowAuth{}, ids, WithFlags(Flags{SnapshotEnabled: true}))
	snap := []StepSnapshot{
		{StepID: "s1", StepCode: "s1", Stage: "S1", Department: "d1", AssigneeMembershipID: "m1", DisplayOrder: 1, ProcessingDays: 1, DueRule: "T+1"},
		{StepID: "s2", StepCode: "s2", Stage: "S2", Department: "d2", AssigneeMembershipID: "m2", DisplayOrder: 2, ProcessingDays: 2, DueRule: "T+2"},
		{StepID: "s3", StepCode: "s3", Stage: "S3", Department: "d3", AssigneeMembershipID: "m3", DisplayOrder: 3, ProcessingDays: 3, DueRule: "T+3"},
	}
	_, err := svc.CreateWorkflowInstanceInternal(context.Background(), CreateWorkflowInstanceRequest{
		Subject:                       Subject{UserID: "u", MembershipID: "creator", CompanyID: "c1"},
		RecordID:                      "rec-1",
		Snapshot:                      snap,
		WorkflowSource:                WorkflowSourceProposalSnapshotV2,
		FirstTaskAssigneeMembershipID: "m1",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	return repo, svc, ids
}

func pendingTask(t *testing.T, repo *fakeWorkflowRepository, companyID, instanceID string) TaskDTO {
	t.Helper()
	tasks, err := repo.ListTasksByInstance(context.Background(), companyID, instanceID)
	if err != nil {
		t.Fatal(err)
	}
	var pending []TaskDTO
	for _, task := range tasks {
		if task.Status == "pending" {
			pending = append(pending, task)
		}
	}
	if len(pending) != 1 {
		t.Fatalf("want exactly 1 pending task, got %d (%#v)", len(pending), tasks)
	}
	return pending[0]
}

func TestV2MultiStep_ThreeStepStrongChain(t *testing.T) {
	repo, svc, _ := seedV2ThreeStep(t)
	instID := repo.createdInstance.WorkflowInstanceID

	t1 := pendingTask(t, repo, "c1", instID)
	if t1.StepCode != "s1" || t1.AssigneeMembershipID != "m1" {
		t.Fatalf("S1 task %#v", t1)
	}
	inst, _ := repo.FindInstance(context.Background(), "c1", instID)
	if inst.Status != "in_progress" || inst.CurrentStepCode != "s1" {
		t.Fatalf("after approve materialize %#v", inst)
	}

	if _, err := svc.ApproveTask(context.Background(), TaskActionRequest{
		Subject: Subject{UserID: "u", MembershipID: "m1", CompanyID: "c1"},
		TaskID:  t1.TaskID,
	}); err != nil {
		t.Fatalf("complete S1: %v", err)
	}
	t2 := pendingTask(t, repo, "c1", instID)
	if t2.StepCode != "s2" || t2.AssigneeMembershipID != "m2" {
		t.Fatalf("S2 task %#v", t2)
	}
	inst, _ = repo.FindInstance(context.Background(), "c1", instID)
	if inst.Status != "in_progress" || inst.CurrentStepCode != "s2" {
		t.Fatalf("after S1 %#v", inst)
	}

	if _, err := svc.ApproveTask(context.Background(), TaskActionRequest{
		Subject: Subject{UserID: "u", MembershipID: "m2", CompanyID: "c1"},
		TaskID:  t2.TaskID,
	}); err != nil {
		t.Fatalf("complete S2: %v", err)
	}
	t3 := pendingTask(t, repo, "c1", instID)
	if t3.StepCode != "s3" || t3.AssigneeMembershipID != "m3" {
		t.Fatalf("S3 task %#v", t3)
	}
	inst, _ = repo.FindInstance(context.Background(), "c1", instID)
	if inst.Status != "in_progress" || inst.CurrentStepCode != "s3" {
		t.Fatalf("after S2 %#v", inst)
	}

	updater := &countingRecordUpdater{}
	notifier := &countingNotifier{}
	svc = NewService(repo, allowAuth{}, &seqIDGen{n: 10},
		WithFlags(Flags{SnapshotEnabled: true}),
		WithRecordStatusUpdater(updater),
		WithWorkflowNotifier(notifier),
	)
	if _, err := svc.ApproveTask(context.Background(), TaskActionRequest{
		Subject: Subject{UserID: "u", MembershipID: "m3", CompanyID: "c1"},
		TaskID:  t3.TaskID,
	}); err != nil {
		t.Fatalf("complete S3: %v", err)
	}
	tasks, _ := repo.ListTasksByInstance(context.Background(), "c1", instID)
	for _, task := range tasks {
		if task.Status == "pending" {
			t.Fatalf("unexpected pending after final: %#v", task)
		}
	}
	inst, _ = repo.FindInstance(context.Background(), "c1", instID)
	if inst.Status != "approved" {
		t.Fatalf("final instance status=%q", inst.Status)
	}
	if updater.calls != 1 || notifier.calls != 1 {
		t.Fatalf("final side effects once: updater=%d notifier=%d", updater.calls, notifier.calls)
	}
}

func TestV2MultiStep_CustomRemovedReorderChains(t *testing.T) {
	repo := &fakeWorkflowRepository{}
	svc := NewService(repo, allowAuth{}, &seqIDGen{}, WithFlags(Flags{SnapshotEnabled: true}))
	// Template would be A,B,C — proposal is C, X(custom), A (removed B, reordered).
	snap := []StepSnapshot{
		{StepID: "c", StepCode: "c", Stage: "C", Department: "dC", AssigneeMembershipID: "mC", DisplayOrder: 1, ProcessingDays: 1},
		{StepID: "x", StepCode: "x", Stage: "X", Department: "dX", AssigneeMembershipID: "mX", DisplayOrder: 2, ProcessingDays: 2},
		{StepID: "a", StepCode: "a", Stage: "A", Department: "dA", AssigneeMembershipID: "mA", DisplayOrder: 3, ProcessingDays: 3},
	}
	_, err := svc.CreateWorkflowInstanceInternal(context.Background(), CreateWorkflowInstanceRequest{
		Subject:                       Subject{UserID: "u", MembershipID: "creator", CompanyID: "c1"},
		RecordID:                      "rec-1",
		Snapshot:                      snap,
		WorkflowSource:                WorkflowSourceProposalSnapshotV2,
		FirstTaskAssigneeMembershipID: "mC",
	})
	if err != nil {
		t.Fatal(err)
	}
	instID := repo.createdInstance.WorkflowInstanceID

	t1 := pendingTask(t, repo, "c1", instID)
	if t1.StepCode != "c" {
		t.Fatalf("want C first, got %s", t1.StepCode)
	}
	if _, err := svc.ApproveTask(context.Background(), TaskActionRequest{
		Subject: Subject{UserID: "u", MembershipID: "mC", CompanyID: "c1"}, TaskID: t1.TaskID,
	}); err != nil {
		t.Fatal(err)
	}
	t2 := pendingTask(t, repo, "c1", instID)
	if t2.StepCode != "x" || t2.AssigneeMembershipID != "mX" {
		t.Fatalf("custom X %#v", t2)
	}
	if _, err := svc.ApproveTask(context.Background(), TaskActionRequest{
		Subject: Subject{UserID: "u", MembershipID: "mX", CompanyID: "c1"}, TaskID: t2.TaskID,
	}); err != nil {
		t.Fatal(err)
	}
	t3 := pendingTask(t, repo, "c1", instID)
	if t3.StepCode != "a" {
		t.Fatalf("want A after X, got %s", t3.StepCode)
	}
	for _, task := range mustList(t, repo, "c1", instID) {
		if task.StepCode == "b" {
			t.Fatal("removed step B must never materialize")
		}
	}
	if _, err := svc.ApproveTask(context.Background(), TaskActionRequest{
		Subject: Subject{UserID: "u", MembershipID: "mA", CompanyID: "c1"}, TaskID: t3.TaskID,
	}); err != nil {
		t.Fatal(err)
	}
	inst, _ := repo.FindInstance(context.Background(), "c1", instID)
	if inst.Status != "approved" {
		t.Fatalf("status=%q", inst.Status)
	}
}

func mustList(t *testing.T, repo *fakeWorkflowRepository, companyID, instanceID string) []TaskDTO {
	t.Helper()
	tasks, err := repo.ListTasksByInstance(context.Background(), companyID, instanceID)
	if err != nil {
		t.Fatal(err)
	}
	return tasks
}

func TestV2MultiStep_DuplicateCompleteNoDuplicateNext(t *testing.T) {
	repo, svc, _ := seedV2ThreeStep(t)
	instID := repo.createdInstance.WorkflowInstanceID
	t1 := pendingTask(t, repo, "c1", instID)
	if _, err := svc.ApproveTask(context.Background(), TaskActionRequest{
		Subject: Subject{UserID: "u", MembershipID: "m1", CompanyID: "c1"}, TaskID: t1.TaskID,
	}); err != nil {
		t.Fatal(err)
	}
	before := len(mustList(t, repo, "c1", instID))
	_, err := svc.ApproveTask(context.Background(), TaskActionRequest{
		Subject: Subject{UserID: "u", MembershipID: "m1", CompanyID: "c1"}, TaskID: t1.TaskID,
	})
	if err == nil {
		t.Fatal("expected stale/duplicate complete to fail")
	}
	after := len(mustList(t, repo, "c1", instID))
	if after != before {
		t.Fatalf("duplicate created tasks: before=%d after=%d", before, after)
	}
	if pendingTask(t, repo, "c1", instID).StepCode != "s2" {
		t.Fatal("still exactly one active S2")
	}
}

func TestV2MultiStep_NextInsertFailureRollsBack(t *testing.T) {
	repo, svc, _ := seedV2ThreeStep(t)
	instID := repo.createdInstance.WorkflowInstanceID
	t1 := pendingTask(t, repo, "c1", instID)
	repo.failNextCreateTask = errors.New("insert boom")
	_, err := svc.ApproveTask(context.Background(), TaskActionRequest{
		Subject: Subject{UserID: "u", MembershipID: "m1", CompanyID: "c1"}, TaskID: t1.TaskID,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	task, _ := repo.FindTask(context.Background(), "c1", t1.TaskID)
	if task.Status != "pending" {
		t.Fatalf("current task should stay pending on rollback, got %s", task.Status)
	}
	inst, _ := repo.FindInstance(context.Background(), "c1", instID)
	if inst.Status != "in_progress" || inst.CurrentStepCode != "s1" {
		t.Fatalf("instance unchanged %#v", inst)
	}
	if len(mustList(t, repo, "c1", instID)) != 1 {
		t.Fatal("no next task on failed transition")
	}
}

func TestV2MultiStep_StaleCompleteRejected(t *testing.T) {
	repo, svc, _ := seedV2ThreeStep(t)
	instID := repo.createdInstance.WorkflowInstanceID
	t1 := pendingTask(t, repo, "c1", instID)
	if _, err := svc.ApproveTask(context.Background(), TaskActionRequest{
		Subject: Subject{UserID: "u", MembershipID: "m1", CompanyID: "c1"}, TaskID: t1.TaskID,
	}); err != nil {
		t.Fatal(err)
	}
	_, err := svc.ApproveTask(context.Background(), TaskActionRequest{
		Subject: Subject{UserID: "u", MembershipID: "m1", CompanyID: "c1"}, TaskID: t1.TaskID,
	})
	if err == nil {
		t.Fatal("expected reject stale complete")
	}
}

func TestV2MultiStep_MissingNextAssigneeFails(t *testing.T) {
	repo := &fakeWorkflowRepository{}
	svc := NewService(repo, allowAuth{}, &seqIDGen{}, WithFlags(Flags{SnapshotEnabled: true}))
	_, err := svc.CreateWorkflowInstanceInternal(context.Background(), CreateWorkflowInstanceRequest{
		Subject:  Subject{UserID: "u", MembershipID: "creator", CompanyID: "c1"},
		RecordID: "rec-1",
		Snapshot: []StepSnapshot{
			{StepID: "s1", StepCode: "s1", Department: "d1", AssigneeMembershipID: "m1", DisplayOrder: 1, ProcessingDays: 1},
			{StepID: "s2", StepCode: "s2", Department: "d2", AssigneeMembershipID: "", DisplayOrder: 2, ProcessingDays: 2},
		},
		WorkflowSource:                WorkflowSourceProposalSnapshotV2,
		FirstTaskAssigneeMembershipID: "m1",
	})
	if err != nil {
		t.Fatal(err)
	}
	t1 := pendingTask(t, repo, "c1", repo.createdInstance.WorkflowInstanceID)
	_, err = svc.ApproveTask(context.Background(), TaskActionRequest{
		Subject: Subject{UserID: "u", MembershipID: "m1", CompanyID: "c1"}, TaskID: t1.TaskID,
	})
	if err == nil {
		t.Fatal("expected missing assignee error")
	}
	task, _ := repo.FindTask(context.Background(), "c1", t1.TaskID)
	if task.Status != "pending" {
		t.Fatalf("no transition without assignee, status=%s", task.Status)
	}
}

func TestLegacy_CompleteFirstTaskCompletesInstance(t *testing.T) {
	repo := &fakeWorkflowRepository{}
	updater := &countingRecordUpdater{}
	svc := NewService(repo, allowAuth{}, &seqIDGen{},
		WithFlags(Flags{SnapshotEnabled: true}),
		WithRecordStatusUpdater(updater),
	)
	_, err := svc.CreateWorkflowInstanceInternal(context.Background(), CreateWorkflowInstanceRequest{
		Subject:  Subject{UserID: "u", MembershipID: "creator", CompanyID: "c1"},
		RecordID: "rec-1",
		Snapshot: []StepSnapshot{
			{StepID: "prepare", StepCode: "prepare", DisplayOrder: 1, ProcessingDays: 1},
			{StepID: "review", StepCode: "review", DisplayOrder: 2, ProcessingDays: 1},
		},
		WorkflowSource: "global_template",
	})
	if err != nil {
		t.Fatal(err)
	}
	t1 := pendingTask(t, repo, "c1", repo.createdInstance.WorkflowInstanceID)
	if _, err := svc.ApproveTask(context.Background(), TaskActionRequest{
		Subject: Subject{UserID: "u", MembershipID: "creator", CompanyID: "c1"}, TaskID: t1.TaskID,
	}); err != nil {
		t.Fatal(err)
	}
	inst, _ := repo.FindInstance(context.Background(), "c1", repo.createdInstance.WorkflowInstanceID)
	if inst.Status != "approved" {
		t.Fatalf("legacy still completes after first task, got %q", inst.Status)
	}
	if len(mustList(t, repo, "c1", repo.createdInstance.WorkflowInstanceID)) != 1 {
		t.Fatal("legacy must not create next task from snapshot")
	}
	if updater.calls != 1 {
		t.Fatalf("updater calls=%d", updater.calls)
	}
}

func TestV2MultiStep_ConcurrentCompleteSingleNext(t *testing.T) {
	repo, svc, _ := seedV2ThreeStep(t)
	instID := repo.createdInstance.WorkflowInstanceID
	t1 := pendingTask(t, repo, "c1", instID)

	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.ApproveTask(context.Background(), TaskActionRequest{
				Subject: Subject{UserID: "u", MembershipID: "m1", CompanyID: "c1"}, TaskID: t1.TaskID,
			})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	ok := 0
	for err := range errs {
		if err == nil {
			ok++
		}
	}
	if ok != 1 {
		t.Fatalf("want exactly 1 successful complete, got %d", ok)
	}
	if pendingTask(t, repo, "c1", instID).StepCode != "s2" {
		t.Fatal("exactly one S2 active")
	}
	if len(mustList(t, repo, "c1", instID)) != 2 {
		t.Fatalf("want 2 tasks total, got %d", len(mustList(t, repo, "c1", instID)))
	}
}
