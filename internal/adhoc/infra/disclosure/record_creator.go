// Package disclosure adapts disclosureapp.Service to adhocapp.RecordCreator.
package disclosure

import (
	"context"
	"errors"
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

// NewRecordCreatorAdapter returns *RecordCreatorAdapter, which satisfies both
// adhocapp.RecordCreator and disclosureapp.PeriodicRecordCreator interfaces.
func NewRecordCreatorAdapter(svc disclosureapp.Service, workflowSvc workflowapp.Service, workflowEnabled bool) *RecordCreatorAdapter {
	return &RecordCreatorAdapter{svc: svc, workflow: workflowSvc, workflowOn: workflowEnabled && workflowSvc != nil}
}

func (a *RecordCreatorAdapter) CreateAndSubmitRecord(ctx context.Context, companyID, typeID, createdByMembershipID, title string, t0Date *time.Time) (string, string, error) {
	return a.CreateAndSubmitRecordWithOpts(ctx, companyID, typeID, createdByMembershipID, title, t0Date, adhocapp.CreateRecordOpts{})
}

// CreateAndSubmitRecordWithPlannedDate is the periodic materialize path.
// It sets disclosure_records.planned_date from the cycle's due_date so that
// DeadlineAlert.dueDate can use Priority 1 (planned_date) instead of falling
// back to DynamicRule (which uses COMPANY_ESTABLISHED_DATE — wrong for periodic).
func (a *RecordCreatorAdapter) CreateAndSubmitRecordWithPlannedDate(ctx context.Context, companyID, typeID, createdByMembershipID, title string, t0Date *time.Time, plannedDate string) (string, string, error) {
	return a.CreateAndSubmitRecordWithOpts(ctx, companyID, typeID, createdByMembershipID, title, t0Date, adhocapp.CreateRecordOpts{
		PlannedDate: plannedDate,
	})
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
		Subject:  sub,
		RecordID: opts.RecordID, // ADR-1B: deterministic pre-allocated ID, empty = let the service generate one
		Payload: disclosureapp.RecordPayload{
			TypeID:      typeID,
			Title:       title,
			Content:     title,
			PlannedDate: opts.PlannedDate, // set from cycle.due_date for periodic; empty for ad-hoc
		},
	})
	if err != nil {
		if opts.RecordID != "" && errors.Is(err, disclosureapp.ErrDuplicateRecordID) {
			// ADR-1B idempotent replay: a prior attempt already created (and possibly
			// submitted/wired-up) the record under this deterministic ID. Fetch the
			// existing record — including its linked workflow instance — instead of
			// creating a second record or a duplicate workflow instance.
			existing, getErr := a.svc.GetRecord(ctx, disclosureapp.GetRecordRequest{
				Subject:  sub,
				RecordID: opts.RecordID,
			})
			if getErr != nil {
				return "", "", fmt.Errorf("fetch existing record after duplicate id: %w", getErr)
			}
			return existing.RecordID, existing.WorkflowInstanceID, nil
		}
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
