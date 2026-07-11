package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
)

func (s *service) ListDeadlineSteps(ctx context.Context, sub Subject, recordID string) (*ListDeadlineStepsResponse, error) {
	if strings.TrimSpace(sub.CompanyID) == "" {
		return nil, perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeCompanyContextRequired, "company_id is required", nil)
	}
	recordID = strings.TrimSpace(recordID)
	if recordID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "record_id is required", nil)
	}
	if err := s.authorizeView(ctx, sub); err != nil {
		return nil, err
	}
	wf, row, err := s.loadWorkflowForRecord(ctx, sub, recordID)
	if err != nil {
		return nil, err
	}
	states, err := s.repo.ListStepStates(ctx, wf.WorkflowInstanceID)
	if err != nil {
		return nil, err
	}
	canManage := s.canManageDeadlineSteps(ctx, sub, recordID) == nil
	now := s.now()
	resp, err := ComputeDeadlineSteps(*wf, states, now, wf.Timezone, canManage)
	if err != nil {
		return nil, err
	}
	if err := s.enrichStepDepartmentNames(ctx, sub.CompanyID, &resp); err != nil {
		return nil, err
	}
	_ = row
	return &resp, nil
}

func (s *service) CompleteDeadlineStep(ctx context.Context, req CompleteStepRequest) (*ListDeadlineStepsResponse, error) {
	if err := s.authorizeStepAction(ctx, req.Subject, req.RecordID); err != nil {
		return nil, err
	}
	wf, _, err := s.loadWorkflowForRecord(ctx, req.Subject, req.RecordID)
	if err != nil {
		return nil, err
	}
	stepCode := strings.TrimSpace(req.StepCode)
	if stepCode == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "step_code is required", nil)
	}
	if !snapshotHasStep(wf.Snapshot, stepCode) {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "step not found", nil)
	}
	states, err := s.repo.ListStepStates(ctx, wf.WorkflowInstanceID)
	if err != nil {
		return nil, err
	}
	now := s.now()
	resp, err := ComputeDeadlineSteps(*wf, states, now, wf.Timezone, true)
	if err != nil {
		return nil, err
	}
	if resp.CurrentStepCode != stepCode {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "step is not current or is locked", nil)
	}
	if st, ok := states[stepCode]; ok && st.CompletedAt != nil {
		return s.ListDeadlineSteps(ctx, req.Subject, req.RecordID)
	}
	at := now.UTC()
	if err := s.repo.UpsertStepCompleted(ctx, wf.CompanyID, wf.WorkflowInstanceID, stepCode, req.Subject.MembershipID, at); err != nil {
		return nil, err
	}
	return s.ListDeadlineSteps(ctx, req.Subject, req.RecordID)
}

func (s *service) MarkDeadlineStepIncomplete(ctx context.Context, req MarkIncompleteStepRequest) (*ListDeadlineStepsResponse, error) {
	if err := s.authorizeStepAction(ctx, req.Subject, req.RecordID); err != nil {
		return nil, err
	}
	wf, _, err := s.loadWorkflowForRecord(ctx, req.Subject, req.RecordID)
	if err != nil {
		return nil, err
	}
	stepCode := strings.TrimSpace(req.StepCode)
	if stepCode == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "step_code is required", nil)
	}
	if !snapshotHasStep(wf.Snapshot, stepCode) {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "step not found", nil)
	}
	delayDays := req.DelayDays
	if delayDays <= 0 {
		delayDays = 1
	}
	states, err := s.repo.ListStepStates(ctx, wf.WorkflowInstanceID)
	if err != nil {
		return nil, err
	}
	now := s.now()
	resp, err := ComputeDeadlineSteps(*wf, states, now, wf.Timezone, true)
	if err != nil {
		return nil, err
	}
	if resp.CurrentStepCode != stepCode {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "step is not current or is locked", nil)
	}
	if st, ok := states[stepCode]; ok {
		if st.CompletedAt != nil {
			return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "completed step cannot be marked incomplete", nil)
		}
		if st.MarkedIncompleteAt != nil && st.DelayDaysApplied > 0 {
			return s.ListDeadlineSteps(ctx, req.Subject, req.RecordID)
		}
	}
	at := now.UTC()
	if err := s.repo.UpsertStepIncomplete(ctx, wf.CompanyID, wf.WorkflowInstanceID, stepCode, req.Subject.MembershipID, strings.TrimSpace(req.Reason), delayDays, at); err != nil {
		return nil, err
	}
	return s.ListDeadlineSteps(ctx, req.Subject, req.RecordID)
}

func (s *service) loadWorkflowForRecord(ctx context.Context, sub Subject, recordID string) (*WorkflowInstanceContext, AlertRow, error) {
	exists, err := s.repo.HasDisclosureRecord(ctx, sub.CompanyID, recordID)
	if err != nil {
		return nil, AlertRow{}, err
	}
	if !exists {
		return nil, AlertRow{}, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "record not found", nil)
	}
	eff, err := s.auth.GetEffectiveAccess(ctx, sub.MembershipID, sub.CompanyID)
	if err != nil {
		return nil, AlertRow{}, fmt.Errorf("resolve effective access: %w", err)
	}
	scope := ResolveDeadlineAlertAccessScope(eff)
	rows, err := s.repo.ListRows(ctx, sub.CompanyID, scope)
	if err != nil {
		return nil, AlertRow{}, err
	}
	var row AlertRow
	found := false
	for _, r := range rows {
		if r.RecordID == recordID {
			row = r
			found = true
			break
		}
	}
	if !found || !scope.AllowsRow(row) {
		return nil, AlertRow{}, perr.NewHTTPError(http.StatusForbidden, perr.CodeDataScopeDenied, "record outside data scope", nil)
	}
	wfRow, err := s.repo.GetWorkflowInstanceByRecord(ctx, sub.CompanyID, recordID)
	if err != nil {
		return nil, row, err
	}
	if wfRow == nil || strings.TrimSpace(wfRow.WorkflowInstanceID) == "" {
		return nil, row, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "workflow instance not found", nil)
	}
	var snapshot []workflowapp.StepSnapshot
	if len(wfRow.SnapshotJSON) > 0 {
		if err := json.Unmarshal(wfRow.SnapshotJSON, &snapshot); err != nil {
			return nil, row, perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, "invalid workflow snapshot", err)
		}
	}
	if len(snapshot) == 0 && strings.TrimSpace(row.TypeID) != "" {
		fallback, err := s.repo.GetEffectiveWorkflowSnapshot(ctx, sub.CompanyID, row.TypeID)
		if err != nil {
			return nil, row, err
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
		t0 = s.now()
	}
	tz := strings.TrimSpace(wfRow.Timezone)
	if tz == "" {
		tz = "Asia/Ho_Chi_Minh"
	}
	return &WorkflowInstanceContext{
		WorkflowInstanceID: wfRow.WorkflowInstanceID,
		CompanyID:          sub.CompanyID,
		RecordID:           recordID,
		T0Date:             t0,
		Snapshot:           snapshot,
		Timezone:           tz,
	}, row, nil
}

func (s *service) authorizeStepAction(ctx context.Context, sub Subject, recordID string) error {
	if err := s.authorizeView(ctx, sub); err != nil {
		return err
	}
	return s.canManageDeadlineSteps(ctx, sub, recordID)
}

func (s *service) canManageDeadlineSteps(ctx context.Context, sub Subject, recordID string) error {
	decision, err := s.auth.Authorize(ctx, authapp.AuthorizeRequest{
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

func snapshotHasStep(snapshot []workflowapp.StepSnapshot, stepCode string) bool {
	for _, snap := range snapshot {
		if stepCodeFromSnapshot(snap) == stepCode {
			return true
		}
	}
	return false
}


func (s *service) enrichStepDepartmentNames(ctx context.Context, companyID string, resp *ListDeadlineStepsResponse) error {
	if resp == nil || len(resp.Steps) == 0 {
		return nil
	}
	companyDepts, err := s.repo.ListCompanyDepartments(ctx, companyID)
	if err != nil {
		return err
	}
	templateDepts, err := s.repo.ListTemplateDepartments(ctx)
	if err != nil {
		return err
	}
	dict := NewDepartmentDict(companyDepts, templateDepts)
	for i := range resp.Steps {
		raw := strings.TrimSpace(resp.Steps[i].DepartmentName)
		if raw == "" {
			continue
		}
		if label := strings.TrimSpace(dict.ResolveLabel(raw)); label != "" {
			resp.Steps[i].DepartmentName = label
			continue
		}
		if LooksLikeTechnicalDepartmentRef(raw) {
			resp.Steps[i].DepartmentName = ""
		}
	}
	return nil
}
