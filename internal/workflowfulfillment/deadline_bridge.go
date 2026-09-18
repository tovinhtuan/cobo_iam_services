package workflowfulfillment

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	deadlinealertsapp "github.com/cobo/cobo_iam_services/internal/deadlinealerts/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
)

// DeadlineBridge adapts deadlinealerts repository + auth for fulfillment ACL/context.
type DeadlineBridge struct {
	Repo deadlinealertsapp.Repository
	Auth authapp.Service
	Now  func() time.Time
}

func (b *DeadlineBridge) now() time.Time {
	if b.Now != nil {
		return b.Now()
	}
	return time.Now()
}

func (b *DeadlineBridge) AuthorizeView(ctx context.Context, sub Subject) error {
	decision, err := b.Auth.Authorize(ctx, authapp.AuthorizeRequest{
		Subject: authapp.SubjectRef{
			UserID:       sub.UserID,
			MembershipID: sub.MembershipID,
			CompanyID:    sub.CompanyID,
		},
		Action: "deadline.view",
		Resource: authapp.ResourceRef{
			Type: "disclosure_record",
			ID:   "",
			Attributes: map[string]any{
				"workflow_state": "*",
			},
		},
	})
	if err != nil {
		return fmt.Errorf("authorize deadline.view: %w", err)
	}
	if decision.Decision != authapp.DecisionAllow {
		return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "deadline.view permission required", nil)
	}
	return nil
}

func (b *DeadlineBridge) AuthorizeMutation(ctx context.Context, sub Subject, recordID string) error {
	if err := b.AuthorizeView(ctx, sub); err != nil {
		return err
	}
	decision, err := b.Auth.Authorize(ctx, authapp.AuthorizeRequest{
		Subject: authapp.SubjectRef{
			UserID:       sub.UserID,
			MembershipID: sub.MembershipID,
			CompanyID:    sub.CompanyID,
		},
		Action: "deadline.confirm",
		Resource: authapp.ResourceRef{
			Type: "disclosure_record",
			ID:   strings.TrimSpace(recordID),
			Attributes: map[string]any{
				"workflow_state": "*",
			},
		},
	})
	if err != nil {
		return fmt.Errorf("authorize deadline.confirm: %w", err)
	}
	if decision.Decision != authapp.DecisionAllow {
		return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "deadline.manage permission required", nil)
	}
	return nil
}

func (b *DeadlineBridge) LoadWorkflowForRecord(ctx context.Context, sub Subject, recordID string) (WorkflowContext, error) {
	recordID = strings.TrimSpace(recordID)
	if strings.TrimSpace(sub.CompanyID) == "" {
		return WorkflowContext{}, perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeCompanyContextRequired, "company_id is required", nil)
	}
	if recordID == "" {
		return WorkflowContext{}, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "record_id is required", nil)
	}
	exists, err := b.Repo.HasDisclosureRecord(ctx, sub.CompanyID, recordID)
	if err != nil {
		return WorkflowContext{}, err
	}
	if !exists {
		return WorkflowContext{}, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "record not found", nil)
	}
	eff, err := b.Auth.GetEffectiveAccess(ctx, sub.MembershipID, sub.CompanyID)
	if err != nil {
		return WorkflowContext{}, fmt.Errorf("resolve effective access: %w", err)
	}
	scope := deadlinealertsapp.ResolveDeadlineAlertAccessScope(eff)
	rows, err := b.Repo.ListRows(ctx, sub.CompanyID, scope)
	if err != nil {
		return WorkflowContext{}, err
	}
	var row deadlinealertsapp.AlertRow
	found := false
	for _, r := range rows {
		if r.RecordID == recordID {
			row = r
			found = true
			break
		}
	}
	if !found || !scope.AllowsRow(row) {
		return WorkflowContext{}, perr.NewHTTPError(http.StatusForbidden, perr.CodeDataScopeDenied, "record outside data scope", nil)
	}
	wfRow, err := b.Repo.GetWorkflowInstanceByRecord(ctx, sub.CompanyID, recordID)
	if err != nil {
		return WorkflowContext{}, err
	}
	if wfRow == nil || strings.TrimSpace(wfRow.WorkflowInstanceID) == "" {
		return WorkflowContext{}, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "workflow instance not found", nil)
	}
	var snapshot []workflowapp.StepSnapshot
	if len(wfRow.SnapshotJSON) > 0 {
		if err := json.Unmarshal(wfRow.SnapshotJSON, &snapshot); err != nil {
			return WorkflowContext{}, perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, "invalid workflow snapshot", err)
		}
	}
	if len(snapshot) == 0 && strings.TrimSpace(row.TypeID) != "" {
		fallback, err := b.Repo.GetEffectiveWorkflowSnapshot(ctx, sub.CompanyID, row.TypeID)
		if err != nil {
			return WorkflowContext{}, err
		}
		if len(fallback) > 0 {
			snapshot = fallback
		}
	}
	t0 := wfRow.T0Date
	if t0.IsZero() {
		if pd := strings.TrimSpace(row.PlannedDate); pd != "" {
			if parsed, err := time.Parse("2006-01-02", pd); err == nil {
				t0 = parsed
			}
		}
	}
	if t0.IsZero() {
		t0 = b.now()
	}
	tz := strings.TrimSpace(wfRow.Timezone)
	if tz == "" {
		tz = "Asia/Ho_Chi_Minh"
	}
	return WorkflowContext{
		WorkflowInstanceID: wfRow.WorkflowInstanceID,
		CompanyID:          sub.CompanyID,
		RecordID:           recordID,
		T0Date:             t0,
		Timezone:           tz,
		SnapshotJSONSteps:  snapshot,
	}, nil
}

func (b *DeadlineBridge) ListStepStates(ctx context.Context, workflowInstanceID string) (map[string]StepState, error) {
	raw, err := b.Repo.ListStepStates(ctx, workflowInstanceID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]StepState, len(raw))
	for k, v := range raw {
		out[k] = StepState{
			StepCode:                       v.StepCode,
			CompletedAt:                    v.CompletedAt,
			CompletedByMembershipID:        v.CompletedByMembershipID,
			MarkedIncompleteAt:             v.MarkedIncompleteAt,
			MarkedIncompleteByMembershipID: v.MarkedIncompleteByMembershipID,
			IncompleteReason:               v.IncompleteReason,
			DelayDaysApplied:               v.DelayDaysApplied,
		}
	}
	return out, nil
}

// EvaluateStepAuthority reports whether stepCode is the BE current step and/or completed.
// Canonical helper shared by B2 fulfillment and Generic Step Evidence (G2B).
func EvaluateStepAuthority(wf WorkflowContext, states map[string]StepState, stepCode string, now time.Time) (isCurrent, isCompleted bool, err error) {
	return evaluateStepAuthority(wf, states, stepCode, now)
}

func evaluateStepAuthority(wf WorkflowContext, states map[string]StepState, stepCode string, now time.Time) (isCurrent, isCompleted bool, err error) {
	stepCode = strings.TrimSpace(stepCode)
	if stepCode == "" {
		return false, false, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "step_code is required", nil)
	}
	found := false
	for _, snap := range wf.SnapshotJSONSteps {
		code := strings.TrimSpace(snap.StepCode)
		if code == "" {
			code = strings.TrimSpace(snap.StepID)
		}
		if code == stepCode {
			found = true
			break
		}
	}
	if !found && len(wf.SnapshotJSONSteps) > 0 {
		return false, false, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "step not found", nil)
	}
	daStates := make(map[string]deadlinealertsapp.StepRuntimeState, len(states))
	for k, v := range states {
		daStates[k] = deadlinealertsapp.StepRuntimeState{
			StepCode:                       v.StepCode,
			CompletedAt:                    v.CompletedAt,
			CompletedByMembershipID:        v.CompletedByMembershipID,
			MarkedIncompleteAt:             v.MarkedIncompleteAt,
			MarkedIncompleteByMembershipID: v.MarkedIncompleteByMembershipID,
			IncompleteReason:               v.IncompleteReason,
			DelayDaysApplied:               v.DelayDaysApplied,
		}
	}
	resp, err := deadlinealertsapp.ComputeDeadlineSteps(deadlinealertsapp.WorkflowInstanceContext{
		WorkflowInstanceID: wf.WorkflowInstanceID,
		CompanyID:          wf.CompanyID,
		RecordID:           wf.RecordID,
		T0Date:             wf.T0Date,
		Snapshot:           wf.SnapshotJSONSteps,
		Timezone:           wf.Timezone,
	}, daStates, now, wf.Timezone, true)
	if err != nil {
		return false, false, err
	}
	for _, s := range resp.Steps {
		if s.StepCode == stepCode {
			isCompleted = s.IsCompleted
			break
		}
	}
	isCurrent = resp.CurrentStepCode == stepCode
	if st, ok := states[stepCode]; ok && st.CompletedAt != nil {
		isCompleted = true
	}
	return isCurrent, isCompleted, nil
}
