package app

import (
	"context"
	"net/http"
	"sync"
	"testing"

	adhocapp "github.com/cobo/cobo_iam_services/internal/adhoc/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func TestResolveTaskAssigneeMembershipIDs_Authority(t *testing.T) {
	got := ResolveTaskAssigneeMembershipIDs("legacy", []string{"m1", "m2"})
	if len(got) != 2 || got[0] != "m1" || got[1] != "m2" {
		t.Fatalf("relation must win without merge: %#v", got)
	}
	got = ResolveTaskAssigneeMembershipIDs("legacy", nil)
	if len(got) != 1 || got[0] != "legacy" {
		t.Fatalf("singular fallback: %#v", got)
	}
	if !IsMembershipTaskAssignee("m2", "", []string{"m1", "m2"}) {
		t.Fatal("m2 should be assignee")
	}
	if IsMembershipTaskAssignee("mX", "m1", nil) {
		t.Fatal("non-assignee")
	}
}

func TestMapProposalWorkflowToSnapshot_V3NoSingularShadow(t *testing.T) {
	snap := MapProposalWorkflowToSnapshot(&adhocapp.ProposalWorkflowSnapshot{
		SchemaVersion: adhocapp.ProposalWorkflowSchemaV3,
		Frozen:        true,
		Steps: []adhocapp.ProposalWorkflowStep{
			{ID: "s1", Order: 1, Name: "A", ProcessingDays: 1, DepartmentID: "d1", AssigneeMembershipIDs: []string{"m1", "m2", "m3"}},
			{ID: "s2", Order: 2, Name: "B", ProcessingDays: 1, DepartmentID: "d2", AssigneeMembershipIDs: []string{"m4"}},
		},
	})
	if len(snap) != 2 {
		t.Fatalf("len=%d", len(snap))
	}
	if snap[0].AssigneeMembershipID != "" {
		t.Fatalf("singular shadow = %q", snap[0].AssigneeMembershipID)
	}
	if len(snap[0].AssigneeMembershipIDs) != 3 {
		t.Fatalf("%#v", snap[0].AssigneeMembershipIDs)
	}
	if snap[1].AssigneeMembershipID != "" || len(snap[1].AssigneeMembershipIDs) != 1 || snap[1].AssigneeMembershipIDs[0] != "m4" {
		t.Fatalf("%#v", snap[1])
	}
}

func TestCreateWorkflowInstance_V3RelationAssignees(t *testing.T) {
	repo := &fakeWorkflowRepository{}
	svc := NewService(repo, nil, fakeWorkflowIDGen{}, WithFlags(Flags{SnapshotEnabled: true}))
	_, err := svc.CreateWorkflowInstanceInternal(context.Background(), CreateWorkflowInstanceRequest{
		Subject:  Subject{UserID: "u", MembershipID: "creator", CompanyID: "co"},
		RecordID: "rec",
		Snapshot: []StepSnapshot{
			{StepID: "s1", StepCode: "s1", DisplayOrder: 1, AssigneeMembershipIDs: []string{"m1", "m2"}},
		},
		WorkflowSource:                 WorkflowSourceProposalSnapshotV3,
		FirstTaskAssigneeMembershipIDs: []string{"m1", "m2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.createdTask.AssigneeMembershipID != "" {
		t.Fatalf("singular must be empty, got %q", repo.createdTask.AssigneeMembershipID)
	}
	if len(repo.createdTask.AssigneeMembershipIDs) != 2 {
		t.Fatalf("%#v", repo.createdTask.AssigneeMembershipIDs)
	}
	if repo.createdInstance.WorkflowSource != WorkflowSourceProposalSnapshotV3 {
		t.Fatalf("%q", repo.createdInstance.WorkflowSource)
	}
}

func TestV3AnyCompletion_SingleWinnerAndNextStep(t *testing.T) {
	repo := &fakeWorkflowRepository{}
	svc := NewService(repo, allowAuth{}, &seqIDGen{}, WithFlags(Flags{SnapshotEnabled: true}))
	_, err := svc.CreateWorkflowInstanceInternal(context.Background(), CreateWorkflowInstanceRequest{
		Subject:  Subject{UserID: "u", MembershipID: "creator", CompanyID: "co"},
		RecordID: "rec",
		Snapshot: []StepSnapshot{
			{StepID: "s1", StepCode: "s1", DisplayOrder: 1, AssigneeMembershipIDs: []string{"m1", "m2"}},
			{StepID: "s2", StepCode: "s2", DisplayOrder: 2, AssigneeMembershipIDs: []string{"m3", "m4"}},
		},
		WorkflowSource:                 WorkflowSourceProposalSnapshotV3,
		FirstTaskAssigneeMembershipIDs: []string{"m1", "m2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	taskID := repo.createdTask.TaskID

	if _, err := svc.ApproveTask(context.Background(), TaskActionRequest{
		Subject: Subject{UserID: "u1", MembershipID: "m1", CompanyID: "co"},
		TaskID:  taskID,
	}); err != nil {
		t.Fatalf("m1 approve: %v", err)
	}
	if _, err := svc.ApproveTask(context.Background(), TaskActionRequest{
		Subject: Subject{UserID: "u2", MembershipID: "m2", CompanyID: "co"},
		TaskID:  taskID,
	}); err == nil {
		t.Fatal("m2 after ANY completion must 409")
	} else if he, ok := perr.AsHTTPError(err); !ok || he.HTTPStatus != http.StatusConflict {
		t.Fatalf("want 409, got %v", err)
	}

	tasks, err := repo.ListTasksByInstance(context.Background(), "co", repo.createdInstance.WorkflowInstanceID)
	if err != nil {
		t.Fatal(err)
	}
	var s2 *TaskDTO
	for i := range tasks {
		if tasks[i].StepCode == "s2" {
			s2 = &tasks[i]
			break
		}
	}
	if s2 == nil {
		t.Fatal("next step not materialized")
	}
	if s2.AssigneeMembershipID != "" || len(s2.AssigneeMembershipIDs) != 2 {
		t.Fatalf("s2 assignees %#v / %#v", s2.AssigneeMembershipID, s2.AssigneeMembershipIDs)
	}
	if s2.AssigneeMembershipIDs[0] != "m3" || s2.AssigneeMembershipIDs[1] != "m4" {
		t.Fatalf("frozen s2 set %#v", s2.AssigneeMembershipIDs)
	}
}

func TestV3Auth_NonAssigneeDenied(t *testing.T) {
	repo := &fakeWorkflowRepository{}
	svc := NewService(repo, allowAuth{}, fakeWorkflowIDGen{}, WithFlags(Flags{SnapshotEnabled: true}))
	_, err := svc.CreateWorkflowInstanceInternal(context.Background(), CreateWorkflowInstanceRequest{
		Subject:  Subject{UserID: "u", MembershipID: "creator", CompanyID: "co"},
		RecordID: "rec",
		Snapshot: []StepSnapshot{
			{StepID: "s1", StepCode: "s1", DisplayOrder: 1, AssigneeMembershipIDs: []string{"m1", "m2"}},
		},
		WorkflowSource:                 WorkflowSourceProposalSnapshotV3,
		FirstTaskAssigneeMembershipIDs: []string{"m1", "m2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.ApproveTask(context.Background(), TaskActionRequest{
		Subject: Subject{UserID: "ux", MembershipID: "mX", CompanyID: "co"},
		TaskID:  repo.createdTask.TaskID,
	})
	if err == nil {
		t.Fatal("expected forbidden")
	}
	if he, ok := perr.AsHTTPError(err); !ok || he.HTTPStatus != http.StatusForbidden {
		t.Fatalf("want 403 got %v", err)
	}
}

func TestV3ConcurrentCompletion_SingleWinner(t *testing.T) {
	repo := &fakeWorkflowRepository{}
	svc := NewService(repo, allowAuth{}, &seqIDGen{}, WithFlags(Flags{SnapshotEnabled: true}))
	_, err := svc.CreateWorkflowInstanceInternal(context.Background(), CreateWorkflowInstanceRequest{
		Subject:  Subject{UserID: "u", MembershipID: "creator", CompanyID: "co"},
		RecordID: "rec",
		Snapshot: []StepSnapshot{
			{StepID: "s1", StepCode: "s1", DisplayOrder: 1, AssigneeMembershipIDs: []string{"m1", "m2"}},
			{StepID: "s2", StepCode: "s2", DisplayOrder: 2, AssigneeMembershipIDs: []string{"m3"}},
		},
		WorkflowSource:                 WorkflowSourceProposalSnapshotV3,
		FirstTaskAssigneeMembershipIDs: []string{"m1", "m2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	taskID := repo.createdTask.TaskID
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, mid := range []string{"m1", "m2"} {
		wg.Add(1)
		go func(membershipID string) {
			defer wg.Done()
			_, e := svc.ApproveTask(context.Background(), TaskActionRequest{
				Subject: Subject{UserID: "u", MembershipID: membershipID, CompanyID: "co"},
				TaskID:  taskID,
			})
			errs <- e
		}(mid)
	}
	wg.Wait()
	close(errs)
	okN, conflict := 0, 0
	for e := range errs {
		if e == nil {
			okN++
			continue
		}
		if he, okHE := perr.AsHTTPError(e); okHE && he.HTTPStatus == http.StatusConflict {
			conflict++
			continue
		}
		t.Fatalf("unexpected err %v", e)
	}
	if okN != 1 || conflict != 1 {
		t.Fatalf("ok=%d conflict=%d want 1/1", okN, conflict)
	}
	tasks, _ := repo.ListTasksByInstance(context.Background(), "co", repo.createdInstance.WorkflowInstanceID)
	s2Count := 0
	for _, tsk := range tasks {
		if tsk.StepCode == "s2" {
			s2Count++
		}
	}
	if s2Count != 1 {
		t.Fatalf("next step count=%d want 1", s2Count)
	}
}

func TestV2RuntimeUnchanged_Singular(t *testing.T) {
	repo := &fakeWorkflowRepository{}
	svc := NewService(repo, allowAuth{}, &seqIDGen{}, WithFlags(Flags{SnapshotEnabled: true}))
	_, err := svc.CreateWorkflowInstanceInternal(context.Background(), CreateWorkflowInstanceRequest{
		Subject:  Subject{UserID: "u", MembershipID: "creator", CompanyID: "co"},
		RecordID: "rec",
		Snapshot: []StepSnapshot{
			{StepID: "s1", StepCode: "s1", DisplayOrder: 1, AssigneeMembershipID: "m1"},
			{StepID: "s2", StepCode: "s2", DisplayOrder: 2, AssigneeMembershipID: "m2"},
		},
		WorkflowSource:                WorkflowSourceProposalSnapshotV2,
		FirstTaskAssigneeMembershipID: "m1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.createdTask.AssigneeMembershipID != "m1" || len(repo.createdTask.AssigneeMembershipIDs) != 0 {
		t.Fatalf("%#v", repo.createdTask)
	}
	if _, err := svc.ApproveTask(context.Background(), TaskActionRequest{
		Subject: Subject{UserID: "u", MembershipID: "m1", CompanyID: "co"},
		TaskID:  repo.createdTask.TaskID,
	}); err != nil {
		t.Fatal(err)
	}
	tasks, _ := repo.ListTasksByInstance(context.Background(), "co", repo.createdInstance.WorkflowInstanceID)
	found := false
	for _, tsk := range tasks {
		if tsk.StepCode == "s2" {
			found = true
			if tsk.AssigneeMembershipID != "m2" || len(tsk.AssigneeMembershipIDs) != 0 {
				t.Fatalf("%#v", tsk)
			}
		}
	}
	if !found {
		t.Fatal("missing s2")
	}
}
