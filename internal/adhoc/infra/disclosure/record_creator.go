// Package disclosure adapts disclosureapp.Service to adhocapp.RecordCreator.
package disclosure

import (
	"context"
	"fmt"

	adhocapp "github.com/cobo/cobo_iam_services/internal/adhoc/app"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
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

func (a *RecordCreatorAdapter) CreateAndSubmitRecord(ctx context.Context, companyID, typeID, createdByMembershipID, title string) (string, string, error) {
	sub := disclosureapp.Subject{
		UserID:       createdByMembershipID,
		MembershipID: createdByMembershipID,
		CompanyID:    companyID,
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
		inst, wfErr := a.workflow.CreateWorkflowInstanceInternal(ctx, workflowapp.CreateWorkflowInstanceRequest{
			Subject: workflowapp.Subject{
				UserID:       sub.UserID,
				MembershipID: sub.MembershipID,
				CompanyID:    sub.CompanyID,
			},
			RecordID: rec.RecordID,
		})
		if wfErr != nil {
			return rec.RecordID, "", fmt.Errorf("create workflow instance: %w", wfErr)
		}
		if inst != nil {
			instanceID = inst.WorkflowInstanceID
		}
	}
	return rec.RecordID, instanceID, nil
}
