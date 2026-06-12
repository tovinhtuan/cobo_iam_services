package workflow

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
)

// Bootstrap creates workflow instances for portal disclosure submit.
type Bootstrap struct {
	disclosure disclosureapp.Service
	workflow   workflowapp.Service
	enabled    bool
}

func NewBootstrap(disclosure disclosureapp.Service, workflow workflowapp.Service, enabled bool) *Bootstrap {
	return &Bootstrap{disclosure: disclosure, workflow: workflow, enabled: enabled && workflow != nil}
}

func (b *Bootstrap) EnsureOnSubmit(ctx context.Context, sub disclosureapp.Subject, rec disclosureapp.RecordDTO) (string, error) {
	if !b.enabled || strings.TrimSpace(rec.TypeID) == "" {
		return "", nil
	}
	effResp, err := b.disclosure.GetEffectiveWorkflow(ctx, disclosureapp.GetEffectiveWorkflowRequest{
		Subject: sub,
		TypeID:  rec.TypeID,
	})
	if err != nil {
		return "", fmt.Errorf("get effective workflow: %w", err)
	}
	workflowSource := "global_template"
	if effResp.Data.Source == "company_override" {
		workflowSource = "company_override"
	}
	snapshot := workflowapp.MapEffectiveWorkflowToSnapshot(effResp.Data.Workflow, workflowSource)
	if err := workflowapp.ValidateSnapshot(snapshot); err != nil {
		return "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "template has no effective workflow steps", err)
	}
	var t0 *time.Time
	if rec.PlannedDate != "" {
		if parsed, parseErr := time.Parse("2006-01-02", rec.PlannedDate); parseErr == nil {
			t0 = &parsed
		}
	}
	inst, err := b.workflow.CreateWorkflowInstanceInternal(ctx, workflowapp.CreateWorkflowInstanceRequest{
		Subject: workflowapp.Subject{
			UserID:       sub.UserID,
			MembershipID: sub.MembershipID,
			CompanyID:    sub.CompanyID,
		},
		RecordID:       rec.RecordID,
		Snapshot:       snapshot,
		WorkflowSource: workflowSource,
		T0Date:         t0,
		T0Policy:       "user_defined",
	})
	if err != nil {
		return "", err
	}
	if inst == nil {
		return "", nil
	}
	return inst.WorkflowInstanceID, nil
}
