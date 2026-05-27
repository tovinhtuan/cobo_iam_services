// Package disclosure adapts disclosureapp.Service to adhocapp.RecordCreator.
package disclosure

import (
	"context"
	"fmt"
	"net/http"
	"time"

	adhocapp "github.com/cobo/cobo_iam_services/internal/adhoc/app"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
)

// RecordCreatorAdapter wraps disclosure and workflow services for ad-hoc approval.
type RecordCreatorAdapter struct {
	svc        disclosureapp.Service
	workflow   workflowapp.Service
	workflowOn bool
}

func NewRecordCreatorAdapter(svc disclosureapp.Service, workflowSvc workflowapp.Service, workflowEnabled bool) adhocapp.RecordCreator {
	return &RecordCreatorAdapter{svc: svc, workflow: workflowSvc, workflowOn: workflowEnabled && workflowSvc != nil}
}

func (a *RecordCreatorAdapter) CreateAndSubmitRecord(ctx context.Context, companyID, typeID, createdByMembershipID, title string, t0Date *time.Time) (string, string, error) {
	return a.CreateAndSubmitRecordWithOpts(ctx, companyID, typeID, createdByMembershipID, title, t0Date, adhocapp.CreateRecordOpts{})
}

func (a *RecordCreatorAdapter) CreateAndSubmitRecordWithOpts(ctx context.Context, companyID, typeID, createdByMembershipID, title string, t0Date *time.Time, opts adhocapp.CreateRecordOpts) (string, string, error) {
	sub := disclosureapp.Subject{
		UserID:       createdByMembershipID,
		MembershipID: createdByMembershipID,
		CompanyID:    companyID,
	}

	var snapshot []workflowapp.StepSnapshot
	var workflowSource string
	if a.workflowOn {
		effResp, err := a.svc.GetEffectiveWorkflow(ctx, disclosureapp.GetEffectiveWorkflowRequest{
			Subject: sub,
			TypeID:  typeID,
		})
		if err != nil {
			return "", "", fmt.Errorf("get effective workflow: %w", err)
		}
		workflowSource = mapWorkflowSource(effResp.Data.Source)
		snapshot = workflowapp.MapEffectiveWorkflowToSnapshot(effResp.Data.Workflow, workflowSource)
		snapshot = workflowapp.ApplyAdHocStepOverrides(snapshot, opts.StepOverrides)
		if err := workflowapp.ValidateSnapshot(snapshot); err != nil {
			return "", "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "template has no effective workflow steps", err)
		}
	}

	rec, err := a.svc.CreateRecord(ctx, disclosureapp.CreateRecordRequest{
		Subject: sub,
		Payload: disclosureapp.RecordPayload{
			TypeID:  typeID,
			Title:   title,
			Content: title,
		},
	})
	if err != nil {
		return "", "", fmt.Errorf("create record: %w", err)
	}
	if _, err := a.svc.SubmitRecord(ctx, disclosureapp.SubmitRecordRequest{
		Subject:  sub,
		RecordID: rec.RecordID,
	}); err != nil {
		return "", "", fmt.Errorf("submit record: %w", err)
	}

	instanceID := ""
	if a.workflowOn {
		wfReq := workflowapp.CreateWorkflowInstanceRequest{
			Subject: workflowapp.Subject{
				UserID:       sub.UserID,
				MembershipID: sub.MembershipID,
				CompanyID:    sub.CompanyID,
			},
			RecordID:       rec.RecordID,
			Snapshot:       snapshot,
			WorkflowSource: workflowSource,
		}
		if t0Date != nil {
			wfReq.T0Date = t0Date
			wfReq.T0Policy = "user_defined"
		}
		inst, wfErr := a.workflow.CreateWorkflowInstanceInternal(ctx, wfReq)
		if wfErr != nil {
			return rec.RecordID, "", fmt.Errorf("create workflow instance: %w", wfErr)
		}
		if inst != nil {
			instanceID = inst.WorkflowInstanceID
		}
	}
	return rec.RecordID, instanceID, nil
}

func mapWorkflowSource(source string) string {
	switch source {
	case "company_override":
		return "company_override"
	default:
		return "global_template"
	}
}
