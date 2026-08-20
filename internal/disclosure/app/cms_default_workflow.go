package app

import (
	"fmt"
	"strings"
)

// CMS default workflow authority (Phase 1 — centralize, transitional Model A bridge).
// Precedence: ACTIVE_GLOBAL_WORKFLOW > TEMPLATE_ENTERPRISE_WORKFLOW > NONE.
// Global draft is never effective — callers must pass only active global version steps.
// Contract: docs/ai-cache/template-workflow-domain-contract-final-2026-08-20.md
//
// Wire Source values match EffectiveWorkflowDTO (no STORE_A / STORE_B in API/UI):
//   global_workflow | global_template | none

const (
	CMSDefaultSourceGlobalWorkflow     = "global_workflow"
	CMSDefaultSourceTemplateEnterprise = "global_template"
	CMSDefaultSourceNone               = "none"
)

// CMSDefaultWorkflowInput is the raw inputs for ResolveCMSDefaultWorkflow.
// ActiveGlobalOK must be true only when an ACTIVE global_workflow_versions row exists
// (draft-only global workflows must set ActiveGlobalOK=false).
type CMSDefaultWorkflowInput struct {
	ActiveGlobalSteps     []WorkflowStepDTO
	ActiveGlobalVersionNo int
	ActiveGlobalOK        bool
	EnterpriseBlocks      []TemplateBlockDTO
	EnterpriseVersionNo   int
}

// EffectiveTemplateWorkflow is the normalized CMS-default authority result.
// Distinguish HasWorkflow (non-empty steps from chosen source) vs IsValid (passes validators).
type EffectiveTemplateWorkflow struct {
	Source           string
	VersionNo        int
	Steps            []WorkflowStepDTO
	HasWorkflow      bool
	IsValid          bool
	ValidationErrors []string
}

// ResolveCMSDefaultWorkflow is the single business precedence implementation for CMS default
// workflow (excluding company override). Do not re-implement this chain in FE/BE consumers.
func ResolveCMSDefaultWorkflow(in CMSDefaultWorkflowInput) EffectiveTemplateWorkflow {
	if in.ActiveGlobalOK {
		steps := cloneWorkflowStepDTOs(in.ActiveGlobalSteps)
		out := EffectiveTemplateWorkflow{
			Source:      CMSDefaultSourceGlobalWorkflow,
			VersionNo:   in.ActiveGlobalVersionNo,
			Steps:       steps,
			HasWorkflow: len(steps) > 0,
		}
		out.applyValidation()
		return out
	}

	enterpriseSteps := ExtractTemplateWorkflow(in.EnterpriseBlocks)
	if len(enterpriseSteps) > 0 {
		out := EffectiveTemplateWorkflow{
			Source:      CMSDefaultSourceTemplateEnterprise,
			VersionNo:   in.EnterpriseVersionNo,
			Steps:       enterpriseSteps,
			HasWorkflow: true,
		}
		out.applyValidation()
		return out
	}

	return EffectiveTemplateWorkflow{
		Source:      CMSDefaultSourceNone,
		VersionNo:   0,
		Steps:       []WorkflowStepDTO{},
		HasWorkflow: false,
		IsValid:     false,
		ValidationErrors: []string{
			"workflow is required",
		},
	}
}

func (e *EffectiveTemplateWorkflow) applyValidation() {
	if !e.HasWorkflow {
		e.IsValid = false
		e.ValidationErrors = []string{"workflow is required"}
		return
	}
	if err := ValidateWorkflowStepsForActivation(e.Steps); err != nil {
		e.IsValid = false
		e.ValidationErrors = []string{err.Error()}
		return
	}
	e.IsValid = true
	e.ValidationErrors = nil
}

// ValidateWorkflowStepsForActivation validates structured workflow steps for template activate/publish.
// Used by both enterprise block validation and canonical CMS-default resolution.
func ValidateWorkflowStepsForActivation(steps []WorkflowStepDTO) error {
	if len(steps) == 0 {
		return fmt.Errorf("workflow is required")
	}
	for i, step := range steps {
		if strings.TrimSpace(step.StepID) == "" || strings.TrimSpace(step.Stage) == "" {
			return fmt.Errorf("workflow step %d is missing step_id or stage", i+1)
		}
		if strings.TrimSpace(step.DepartmentID) == "" {
			return fmt.Errorf("workflow step %d is missing department_id", i+1)
		}
		if len(step.AssigneeRoleIds) == 0 {
			return fmt.Errorf("workflow step %d is missing assignee_role_ids", i+1)
		}
		if step.ProcessingDays <= 0 {
			return fmt.Errorf("workflow step %d has invalid processing_days", i+1)
		}
		if err := ValidateWorkflowStepReminderConfigForPersist(step.ReminderConfig); err != nil {
			return fmt.Errorf("workflow step %d reminder_config: %w", i+1, err)
		}
	}
	return nil
}

func cloneWorkflowStepDTOs(in []WorkflowStepDTO) []WorkflowStepDTO {
	if len(in) == 0 {
		return []WorkflowStepDTO{}
	}
	out := make([]WorkflowStepDTO, len(in))
	copy(out, in)
	for i := range out {
		if len(out[i].AssigneeRoleIds) > 0 {
			out[i].AssigneeRoleIds = append([]string(nil), out[i].AssigneeRoleIds...)
		}
		if len(out[i].AssigneeMembershipIDs) > 0 {
			out[i].AssigneeMembershipIDs = append([]string(nil), out[i].AssigneeMembershipIDs...)
		}
		if len(out[i].Documents) > 0 {
			out[i].Documents = append([]WorkflowDocumentDTO(nil), out[i].Documents...)
		} else {
			out[i].Documents = []WorkflowDocumentDTO{}
		}
		if len(out[i].Groups) > 0 {
			out[i].Groups = append([]WorkflowStepGroupDTO(nil), out[i].Groups...)
			for j := range out[i].Groups {
				if out[i].Groups[j].ProcessingDays != nil {
					days := *out[i].Groups[j].ProcessingDays
					out[i].Groups[j].ProcessingDays = &days
				}
			}
		}
		out[i].ReminderConfig = CloneWorkflowStepReminderConfig(out[i].ReminderConfig)
	}
	return out
}
