package app

import (
	"context"
	"fmt"
	"net/http"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// Task wire action names exposed on TaskDTO.available_actions (Tenant UI).
const (
	TaskActionReview  = "review"
	TaskActionApprove = "approve"
	TaskActionConfirm = "confirm"
	TaskActionReject  = "reject"
)

// CanonicalAvailableTaskActions is the fixed evaluation/report order.
var CanonicalAvailableTaskActions = []string{
	TaskActionReview,
	TaskActionApprove,
	TaskActionConfirm,
	TaskActionReject,
}

// taskActionPolicyCode maps UI/wire action → authz Action used by mutation APIs.
func taskActionPolicyCode(wireAction string) string {
	switch wireAction {
	case TaskActionReview:
		return "workflow.review"
	case TaskActionApprove:
		return "workflow.approve"
	case TaskActionConfirm:
		return "workflow.confirm"
	case TaskActionReject:
		return "workflow.reject"
	default:
		return ""
	}
}

func policyActionToWire(policyAction string) string {
	switch policyAction {
	case "workflow.review":
		return TaskActionReview
	case "workflow.approve":
		return TaskActionApprove
	case "workflow.confirm":
		return TaskActionConfirm
	case "workflow.reject":
		return TaskActionReject
	default:
		return ""
	}
}

// EvaluateTaskAction is a side-effect-free check matching transitionTask authorization:
// action-specific policy Allow AND membership is task assignee AND status == pending.
// Denial is normal for available_actions (omit action); mutation maps codes to HTTP errors.
// authErr is non-nil only when the auth service itself fails (same wrap path as authorize()).
func (s *service) EvaluateTaskAction(ctx context.Context, sub Subject, task TaskDTO, wireAction string) (allowed bool, denyCode perr.Code, authErr error) {
	policyAction := taskActionPolicyCode(wireAction)
	if policyAction == "" {
		return false, perr.CodePermissionDenied, nil
	}
	if s.auth == nil {
		return false, perr.CodePermissionDenied, nil
	}
	decision, err := s.auth.Authorize(ctx, authapp.AuthorizeRequest{
		Subject: authapp.SubjectRef{UserID: sub.UserID, MembershipID: sub.MembershipID, CompanyID: sub.CompanyID},
		Action:  policyAction,
		Resource: authapp.ResourceRef{
			Type: "workflow_task",
			ID:   task.TaskID,
			Attributes: map[string]any{
				"assignee_membership_id":  task.AssigneeMembershipID,
				"assignee_membership_ids": task.AssigneeMembershipIDs,
				"workflow_state":          task.Status,
			},
		},
	})
	if err != nil {
		return false, "", fmt.Errorf("authorize workflow action: %w", err)
	}
	if decision == nil || decision.Decision != authapp.DecisionAllow {
		code := perr.CodePermissionDenied
		if decision != nil && decision.DenyReasonCode != nil {
			code = *decision.DenyReasonCode
		}
		return false, code, nil
	}
	if !IsMembershipTaskAssignee(sub.MembershipID, task.AssigneeMembershipID, task.AssigneeMembershipIDs) {
		return false, perr.CodeResponsibilityRequired, nil
	}
	if task.Status != "pending" {
		return false, perr.CodeStateConflict, nil
	}
	return true, "", nil
}

// ComputeAvailableActions returns wire actions the current membership can execute now.
// Independent evaluation per action (review ≠ reject). Side-effect-free.
func (s *service) ComputeAvailableActions(ctx context.Context, sub Subject, task TaskDTO) []string {
	out := make([]string, 0, len(CanonicalAvailableTaskActions))
	for _, wire := range CanonicalAvailableTaskActions {
		ok, _, err := s.EvaluateTaskAction(ctx, sub, task, wire)
		if err != nil || !ok {
			continue
		}
		out = append(out, wire)
	}
	return out
}

func (s *service) enrichTaskAvailableActions(ctx context.Context, sub Subject, tasks []TaskDTO) {
	for i := range tasks {
		tasks[i].AvailableActions = s.ComputeAvailableActions(ctx, sub, tasks[i])
	}
}

func httpErrorForTaskActionDeny(code perr.Code, message string) error {
	status := http.StatusForbidden
	if code == perr.CodeStateConflict {
		status = http.StatusConflict
	}
	if message == "" {
		switch code {
		case perr.CodeResponsibilityRequired:
			message = "task assignee mismatch"
		case perr.CodeStateConflict:
			message = "task is not pending"
		default:
			message = "access denied"
		}
	}
	return perr.NewHTTPError(status, code, message, nil)
}
