package app

import (
	"context"
	"net/http"
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// selectiveAuth allows all actions except those listed in denyByAction (policy action → deny code).
type selectiveAuth struct {
	denyByAction map[string]perr.Code
}

func (a selectiveAuth) Authorize(_ context.Context, req authapp.AuthorizeRequest) (*authapp.AuthorizeDecision, error) {
	if code, ok := a.denyByAction[req.Action]; ok {
		c := code
		return &authapp.AuthorizeDecision{Decision: authapp.DecisionDeny, DenyReasonCode: &c}, nil
	}
	return &authapp.AuthorizeDecision{Decision: authapp.DecisionAllow}, nil
}
func (selectiveAuth) AuthorizeBatch(_ context.Context, _ authapp.AuthorizeBatchRequest) (*authapp.AuthorizeBatchResponse, error) {
	return &authapp.AuthorizeBatchResponse{}, nil
}
func (selectiveAuth) GetEffectiveAccess(_ context.Context, _, _ string) (*authapp.EffectiveAccessSummary, error) {
	return nil, nil
}

func fixturePendingTask(assignee string) TaskDTO {
	return TaskDTO{
		TaskID:               "task-1",
		CompanyID:            "c1",
		WorkflowInstanceID:   "wf-1",
		StepCode:             "step-1",
		AssigneeMembershipID: assignee,
		Status:               "pending",
	}
}

func TestComputeAvailableActions_Matrix(t *testing.T) {
	subAssignee := Subject{UserID: "u", MembershipID: "m_102", CompanyID: "c1"}
	subNonAssignee := Subject{UserID: "u", MembershipID: "m_102", CompanyID: "c1"}
	taskAssigned := fixturePendingTask("m_102")
	taskOther := fixturePendingTask("m_system_worker")
	reviewed := fixturePendingTask("m_102")
	reviewed.Status = "reviewed"

	reviewOnlyAuth := selectiveAuth{denyByAction: map[string]perr.Code{
		"workflow.reject":  perr.CodePermissionDenied,
		"workflow.approve": perr.CodePermissionDenied,
		"workflow.confirm": perr.CodePermissionDenied,
	}}
	allAuth := allowAuth{}
	denyAllAuth := selectiveAuth{denyByAction: map[string]perr.Code{
		"workflow.review":  perr.CodePermissionDenied,
		"workflow.reject":  perr.CodePermissionDenied,
		"workflow.approve": perr.CodePermissionDenied,
		"workflow.confirm": perr.CodePermissionDenied,
	}}

	cases := []struct {
		name string
		auth authapp.Service
		sub  Subject
		task TaskDTO
		want []string
	}{
		{
			name: "B1 pending review policy assignee match → review present",
			auth: reviewOnlyAuth,
			sub:  subAssignee,
			task: taskAssigned,
			want: []string{TaskActionReview},
		},
		{
			name: "B2 pending review policy non-assignee → review absent",
			auth: reviewOnlyAuth,
			sub:  subNonAssignee,
			task: taskOther,
			want: []string{},
		},
		{
			name: "B3 reject policy denied → reject absent (review-only)",
			auth: reviewOnlyAuth,
			sub:  subAssignee,
			task: taskAssigned,
			want: []string{TaskActionReview},
		},
		{
			name: "B4 DEV-like admin non-assignee → []",
			auth: allAuth, // coarse permission would allow; assignee blocks
			sub:  Subject{UserID: "admin", MembershipID: "m_102", CompanyID: "c1"},
			task: taskOther,
			want: []string{},
		},
		{
			name: "B5 non-pending → []",
			auth: allAuth,
			sub:  subAssignee,
			task: reviewed,
			want: []string{},
		},
		{
			name: "B6 review allowed / reject denied → [review]",
			auth: reviewOnlyAuth,
			sub:  subAssignee,
			task: taskAssigned,
			want: []string{TaskActionReview},
		},
		{
			name: "both review+reject when policies allow and assignee matches",
			auth: selectiveAuth{denyByAction: map[string]perr.Code{
				"workflow.approve": perr.CodePermissionDenied,
				"workflow.confirm": perr.CodePermissionDenied,
			}},
			sub:  subAssignee,
			task: taskAssigned,
			want: []string{TaskActionReview, TaskActionReject},
		},
		{
			name: "deny all policies → []",
			auth: denyAllAuth,
			sub:  subAssignee,
			task: taskAssigned,
			want: []string{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewService(&fakeWorkflowRepository{}, tc.auth, fakeWorkflowIDGen{}).(*service)
			got := svc.ComputeAvailableActions(context.Background(), tc.sub, tc.task)
			if len(got) != len(tc.want) {
				t.Fatalf("got %#v want %#v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("got %#v want %#v", got, tc.want)
				}
			}
		})
	}
}

func TestAvailableActionsMutationAuthParity(t *testing.T) {
	auth := selectiveAuth{denyByAction: map[string]perr.Code{
		"workflow.reject":  perr.CodePermissionDenied,
		"workflow.approve": perr.CodePermissionDenied,
		"workflow.confirm": perr.CodePermissionDenied,
	}}
	svc := NewService(&fakeWorkflowRepository{}, auth, fakeWorkflowIDGen{}).(*service)
	sub := Subject{UserID: "u", MembershipID: "m_assignee", CompanyID: "c1"}
	task := fixturePendingTask("m_assignee")

	actions := svc.ComputeAvailableActions(context.Background(), sub, task)
	if len(actions) != 1 || actions[0] != TaskActionReview {
		t.Fatalf("available=%#v", actions)
	}
	ok, _, err := svc.EvaluateTaskAction(context.Background(), sub, task, TaskActionReview)
	if err != nil || !ok {
		t.Fatalf("mutation review should allow: ok=%v err=%v", ok, err)
	}
	ok, code, err := svc.EvaluateTaskAction(context.Background(), sub, task, TaskActionReject)
	if err != nil || ok || code != perr.CodePermissionDenied {
		t.Fatalf("mutation reject should deny permission: ok=%v code=%s err=%v", ok, code, err)
	}

	nonAssignee := fixturePendingTask("m_system_worker")
	actions = svc.ComputeAvailableActions(context.Background(), sub, nonAssignee)
	if len(actions) != 0 {
		t.Fatalf("non-assignee available=%#v", actions)
	}
	ok, code, err = svc.EvaluateTaskAction(context.Background(), sub, nonAssignee, TaskActionReview)
	if err != nil || ok || code != perr.CodeResponsibilityRequired {
		t.Fatalf("mutation review non-assignee: ok=%v code=%s err=%v", ok, code, err)
	}
}

func TestReviewTask_MutationAuthUnchanged_NonAssignee(t *testing.T) {
	repo := &fakeWorkflowRepository{}
	repo.tasks = map[string]TaskDTO{
		"c1:task-1": fixturePendingTask("m_system_worker"),
	}
	svc := NewService(repo, allowAuth{}, fakeWorkflowIDGen{})
	_, err := svc.ReviewTask(context.Background(), TaskActionRequest{
		Subject: Subject{UserID: "u", MembershipID: "m_102", CompanyID: "c1"},
		TaskID:  "task-1",
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusForbidden || he.Code != perr.CodeResponsibilityRequired {
		t.Fatalf("want 403 RESPONSIBILITY_REQUIRED, got %#v", err)
	}
}

func TestRejectTask_MutationAuthUnchanged_PolicyDenied(t *testing.T) {
	repo := &fakeWorkflowRepository{}
	repo.tasks = map[string]TaskDTO{
		"c1:task-1": fixturePendingTask("m_102"),
	}
	auth := selectiveAuth{denyByAction: map[string]perr.Code{
		"workflow.reject": perr.CodePermissionDenied,
	}}
	svc := NewService(repo, auth, fakeWorkflowIDGen{})
	_, err := svc.RejectTask(context.Background(), TaskActionRequest{
		Subject: Subject{UserID: "u", MembershipID: "m_102", CompanyID: "c1"},
		TaskID:  "task-1",
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusForbidden || he.Code != perr.CodePermissionDenied {
		t.Fatalf("want 403 PERMISSION_DENIED, got %#v", err)
	}
}

func TestListInstanceTasks_EnrichesAvailableActions(t *testing.T) {
	assigneeTask := fixturePendingTask("m_102")
	assigneeTask.TaskID = "task-assignee"
	nonAssigneeTask := fixturePendingTask("m_system_worker")
	nonAssigneeTask.TaskID = "task-other"
	repo := &fakeWorkflowRepository{}
	repo.tasks = map[string]TaskDTO{
		"c1:task-assignee": assigneeTask,
		"c1:task-other":    nonAssigneeTask,
	}
	auth := selectiveAuth{denyByAction: map[string]perr.Code{
		"workflow.reject":  perr.CodePermissionDenied,
		"workflow.approve": perr.CodePermissionDenied,
		"workflow.confirm": perr.CodePermissionDenied,
	}}
	svc := NewService(repo, auth, fakeWorkflowIDGen{})
	got, err := svc.ListInstanceTasks(context.Background(), ListInstanceTasksRequest{
		Subject:            Subject{UserID: "u", MembershipID: "m_102", CompanyID: "c1"},
		WorkflowInstanceID: "wf-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len=%d", len(got))
	}
	byID := map[string]TaskDTO{}
	for _, tsk := range got {
		byID[tsk.TaskID] = tsk
	}
	if len(byID["task-assignee"].AvailableActions) != 1 || byID["task-assignee"].AvailableActions[0] != TaskActionReview {
		t.Fatalf("assignee task actions=%#v", byID["task-assignee"].AvailableActions)
	}
	if len(byID["task-other"].AvailableActions) != 0 {
		t.Fatalf("non-assignee actions=%#v", byID["task-other"].AvailableActions)
	}
}

func TestReminderAdminFallbackDoesNotGrantWorkflowActions(t *testing.T) {
	// Reminder COMPANY_ADMIN fallback is unrelated: available_actions only uses
	// policy + assignee + pending. Admin membership without assignee → [].
	svc := NewService(&fakeWorkflowRepository{}, allowAuth{}, fakeWorkflowIDGen{}).(*service)
	got := svc.ComputeAvailableActions(context.Background(),
		Subject{UserID: "admin", MembershipID: "m_admin", CompanyID: "c1"},
		fixturePendingTask("m_system_worker"),
	)
	if len(got) != 0 {
		t.Fatalf("reminder/admin fallback must not grant actions: %#v", got)
	}
}
