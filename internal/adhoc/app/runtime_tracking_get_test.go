package app

import (
	"context"
	"testing"
)

type fakeRuntimeReader struct {
	code   string
	status string
	tasks  []RuntimeTaskView
	err    error
}

func (f *fakeRuntimeReader) FindInstanceRuntime(context.Context, string, string) (string, string, error) {
	return f.code, f.status, f.err
}

func (f *fakeRuntimeReader) ListInstanceTasks(context.Context, string, string) ([]RuntimeTaskView, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.tasks, nil
}

func TestGetProposal_EmbedsRuntimeTracking_PreservesSelfRead(t *testing.T) {
	repo := &trackingListRepo{items: []ProposalDTO{{
		ProposalID:         "p1",
		CompanyID:          "co",
		CreatedBy:          "mem-creator",
		Status:             StatusApproved,
		WorkflowInstanceID: "wi-1",
		Workflow: &ProposalWorkflowSnapshot{
			SchemaVersion: ProposalWorkflowSchemaV3,
			Frozen:        true,
			Steps: []ProposalWorkflowStep{
				{ID: "s1", Order: 1, Name: "One", AssigneeMembershipIDs: []string{"a", "b"}},
			},
		},
	}}}
	svc := NewService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, proposeOnlyAuth(), newAllowValidator(), nil, noopMetrics{})
	svc = AttachRuntimeTracking(svc, &fakeRuntimeReader{
		code:   "s1",
		status: "in_progress",
		tasks: []RuntimeTaskView{{
			TaskID: "t1", StepCode: "s1", Status: "pending",
			AssigneeMembershipIDs: []string{"a", "b"},
		}},
	})

	got, err := svc.GetProposal(context.Background(), GetProposalRequest{
		Subject:    Subject{CompanyID: "co", MembershipID: "mem-creator", UserID: "u1"},
		ProposalID: "p1",
	})
	if err != nil {
		t.Fatalf("GetProposal: %v", err)
	}
	if got.Tracking == nil || got.Tracking.CurrentStep == nil {
		t.Fatalf("expected tracking: %+v", got.Tracking)
	}
	if len(got.Tracking.CurrentStep.Assignees) != 2 {
		t.Fatalf("assignees=%d", len(got.Tracking.CurrentStep.Assignees))
	}
}

func TestGetProposal_TrackingDoesNotBroadenOtherCreator(t *testing.T) {
	repo := &trackingListRepo{items: []ProposalDTO{{
		ProposalID: "p1",
		CompanyID:  "co",
		CreatedBy:  "mem-other",
		Status:     StatusApproved,
		Workflow: &ProposalWorkflowSnapshot{
			SchemaVersion: ProposalWorkflowSchemaV3,
			Frozen:        true,
			Steps:         []ProposalWorkflowStep{{ID: "s1", Order: 1, Name: "One", AssigneeMembershipIDs: []string{"a"}}},
		},
	}}}
	svc := NewService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, proposeOnlyAuth(), newAllowValidator(), nil, noopMetrics{})
	svc = AttachRuntimeTracking(svc, &fakeRuntimeReader{code: "s1", status: "in_progress"})

	_, err := svc.GetProposal(context.Background(), GetProposalRequest{
		Subject:    Subject{CompanyID: "co", MembershipID: "mem-creator", UserID: "u1"},
		ProposalID: "p1",
	})
	if err == nil {
		t.Fatal("expected forbid other creator")
	}
}
