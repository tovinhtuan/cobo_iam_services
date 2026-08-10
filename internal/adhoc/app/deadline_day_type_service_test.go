package app

import (
	"context"
	"net/http"
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func TestCreateProposal_DeadlineDayType_MissingAllowed(t *testing.T) {
	repo := &fakeRepository{}
	auth := &fakeAuthService{decision: authapp.DecisionAllow}
	svc := newTestService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, auth)

	out, err := svc.CreateProposal(context.Background(), CreateProposalRequest{
		Subject:               Subject{CompanyID: "company-001", MembershipID: "member-creator", UserID: "user-001"},
		TypeID:                "dt-001",
		ReviewerMembershipIDs: []string{"member-focal"},
		ProposedDeadlineDays:  10,
	})
	if err != nil {
		t.Fatalf("CreateProposal: %v", err)
	}
	if out.ProposedDeadlineDayType != nil {
		t.Fatalf("expected nil day type on draft create, got %v", *out.ProposedDeadlineDayType)
	}
	if out.ProposedDeadlineDays == nil || *out.ProposedDeadlineDays != 10 {
		t.Fatalf("deadline days regression: %#v", out.ProposedDeadlineDays)
	}
}

func TestCreateProposal_DeadlineDayType_WorkingAndCalendar(t *testing.T) {
	auth := &fakeAuthService{decision: authapp.DecisionAllow}
	for _, want := range []ProposalDeadlineDayType{ProposalDeadlineDayTypeWorkingDays, ProposalDeadlineDayTypeCalendarDays} {
		repo := &fakeRepository{}
		svc := newTestService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, auth)
		out, err := svc.CreateProposal(context.Background(), CreateProposalRequest{
			Subject:                 Subject{CompanyID: "company-001", MembershipID: "member-creator", UserID: "user-001"},
			TypeID:                  "dt-001",
			ReviewerMembershipIDs:   []string{"member-focal"},
			ProposedDeadlineDayType: string(want),
		})
		if err != nil {
			t.Fatalf("%s: %v", want, err)
		}
		if out.ProposedDeadlineDayType == nil || *out.ProposedDeadlineDayType != want {
			t.Fatalf("%s: got %#v", want, out.ProposedDeadlineDayType)
		}
	}
}

func TestCreateProposal_DeadlineDayType_InvalidRejected(t *testing.T) {
	repo := &fakeRepository{}
	auth := &fakeAuthService{decision: authapp.DecisionAllow}
	svc := newTestService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, auth)
	_, err := svc.CreateProposal(context.Background(), CreateProposalRequest{
		Subject:                 Subject{CompanyID: "company-001", MembershipID: "member-creator", UserID: "user-001"},
		TypeID:                  "dt-001",
		ReviewerMembershipIDs:   []string{"member-focal"},
		ProposedDeadlineDayType: "working",
	})
	if err == nil {
		t.Fatal("expected invalid day type error")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("expected 400, got %#v", err)
	}
	if he.Details == nil || he.Details["field"] != "proposed_deadline_day_type" {
		t.Fatalf("expected field detail, got %#v", he.Details)
	}
	if repo.insertCalls != 0 {
		t.Fatalf("insert must not run on invalid day type")
	}
}

func TestPatchDraftProposal_DeadlineDayType_SetClearKeep(t *testing.T) {
	work := ProposalDeadlineDayTypeWorkingDays
	repo := &fakeRepository{
		proposal: &ProposalDTO{
			ProposalID:              "prop-001",
			CompanyID:               "company-001",
			TypeID:                  "dt-001",
			Status:                  StatusDraft,
			CreatedBy:               "member-creator",
			ProposedDeadlineDayType: &work,
			Workflow: &ProposalWorkflowSnapshot{
				SchemaVersion: ProposalWorkflowSchemaV2,
				Steps: []ProposalWorkflowStep{
					{ID: "s1", Order: 1, Name: "A", ProcessingDays: 2},
				},
			},
		},
	}
	auth := &fakeAuthService{decision: authapp.DecisionAllow}
	svc := newTestService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, auth)

	cal := string(ProposalDeadlineDayTypeCalendarDays)
	out, err := svc.PatchDraftProposal(context.Background(), PatchDraftProposalRequest{
		Subject:                 Subject{CompanyID: "company-001", MembershipID: "member-creator", UserID: "user-001"},
		ProposalID:              "prop-001",
		ProposedDeadlineDayType: &cal,
	})
	if err != nil {
		t.Fatalf("patch calendar: %v", err)
	}
	if out.ProposedDeadlineDayType == nil || *out.ProposedDeadlineDayType != ProposalDeadlineDayTypeCalendarDays {
		t.Fatalf("got %#v", out.ProposedDeadlineDayType)
	}
	if out.Workflow == nil || len(out.Workflow.Steps) != 1 || out.Workflow.Steps[0].ProcessingDays != 2 {
		t.Fatalf("step processing_days must remain unchanged: %#v", out.Workflow)
	}

	empty := ""
	out, err = svc.PatchDraftProposal(context.Background(), PatchDraftProposalRequest{
		Subject:                 Subject{CompanyID: "company-001", MembershipID: "member-creator", UserID: "user-001"},
		ProposalID:              "prop-001",
		ProposedDeadlineDayType: &empty,
	})
	if err != nil {
		t.Fatalf("patch clear: %v", err)
	}
	if out.ProposedDeadlineDayType != nil {
		t.Fatalf("expected nil after clear, got %v", *out.ProposedDeadlineDayType)
	}

	// omit keeps current (nil)
	out, err = svc.PatchDraftProposal(context.Background(), PatchDraftProposalRequest{
		Subject:    Subject{CompanyID: "company-001", MembershipID: "member-creator", UserID: "user-001"},
		ProposalID: "prop-001",
		ChangeNote: strPtr("note only"),
	})
	if err != nil {
		t.Fatalf("patch keep: %v", err)
	}
	if out.ProposedDeadlineDayType != nil {
		t.Fatalf("expected keep nil, got %v", *out.ProposedDeadlineDayType)
	}
	if out.ChangeNote != "note only" {
		t.Fatalf("change note: %q", out.ChangeNote)
	}
}

func TestPatchDraftProposal_SubmittedImmutable(t *testing.T) {
	repo := &fakeRepository{
		proposal: &ProposalDTO{
			ProposalID: "prop-001",
			CompanyID:  "company-001",
			TypeID:     "dt-001",
			Status:     StatusPendingFocalApproval,
			CreatedBy:  "member-creator",
		},
	}
	auth := &fakeAuthService{decision: authapp.DecisionAllow}
	svc := newTestService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, auth)
	work := string(ProposalDeadlineDayTypeWorkingDays)
	_, err := svc.PatchDraftProposal(context.Background(), PatchDraftProposalRequest{
		Subject:                 Subject{CompanyID: "company-001", MembershipID: "member-creator", UserID: "user-001"},
		ProposalID:              "prop-001",
		ProposedDeadlineDayType: &work,
	})
	if err == nil {
		t.Fatal("expected frozen draft conflict")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusConflict {
		t.Fatalf("expected 409, got %#v", err)
	}
}

func TestSubmitProposal_NormalizesNullDayTypeToCalendar(t *testing.T) {
	repo := &fakeRepository{
		proposal: &ProposalDTO{
			ProposalID: "prop-001",
			CompanyID:  "company-001",
			TypeID:     "dt-001",
			Status:     StatusDraft,
			CreatedBy:  "member-creator",
		},
		reviewers: []ReviewerDTO{{MembershipID: "member-focal"}},
	}
	auth := &fakeAuthService{decision: authapp.DecisionAllow}
	svc := newTestService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, auth)

	out, err := svc.SubmitProposal(context.Background(), ProposalActionRequest{
		Subject:    Subject{CompanyID: "company-001", MembershipID: "member-creator", UserID: "user-001"},
		ProposalID: "prop-001",
	})
	if err != nil {
		t.Fatalf("SubmitProposal: %v", err)
	}
	if !repo.lastUpdate.PersistProposedDeadlineDayType {
		t.Fatal("expected PersistProposedDeadlineDayType on submit")
	}
	if repo.lastUpdate.ProposedDeadlineDayType == nil || *repo.lastUpdate.ProposedDeadlineDayType != ProposalDeadlineDayTypeCalendarDays {
		t.Fatalf("expected CALENDAR_DAYS normalize, got %#v", repo.lastUpdate.ProposedDeadlineDayType)
	}
	if out.ProposedDeadlineDayType == nil || *out.ProposedDeadlineDayType != ProposalDeadlineDayTypeCalendarDays {
		t.Fatalf("response day type: %#v", out.ProposedDeadlineDayType)
	}
	if out.Status != StatusPendingFocalApproval {
		t.Fatalf("status: %s", out.Status)
	}
}

func TestSubmitProposal_PreservesWorkingDays(t *testing.T) {
	work := ProposalDeadlineDayTypeWorkingDays
	repo := &fakeRepository{
		proposal: &ProposalDTO{
			ProposalID:              "prop-001",
			CompanyID:               "company-001",
			TypeID:                  "dt-001",
			Status:                  StatusDraft,
			CreatedBy:               "member-creator",
			ProposedDeadlineDayType: &work,
		},
		reviewers: []ReviewerDTO{{MembershipID: "member-focal"}},
	}
	auth := &fakeAuthService{decision: authapp.DecisionAllow}
	svc := newTestService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, auth)

	out, err := svc.SubmitProposal(context.Background(), ProposalActionRequest{
		Subject:    Subject{CompanyID: "company-001", MembershipID: "member-creator", UserID: "user-001"},
		ProposalID: "prop-001",
	})
	if err != nil {
		t.Fatalf("SubmitProposal: %v", err)
	}
	if out.ProposedDeadlineDayType == nil || *out.ProposedDeadlineDayType != ProposalDeadlineDayTypeWorkingDays {
		t.Fatalf("got %#v", out.ProposedDeadlineDayType)
	}
}

func strPtr(s string) *string { return &s }
