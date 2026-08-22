package app

import (
	"fmt"
	"net/http"
	"strings"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

const maxWorkflowOverrideStepInstructionsLen = 2000
const maxWorkflowOverrideStepDescriptionLen = 2000

// ValidateCompanyWorkflowOverrideSteps enforces activation rules for company workflow overrides.
// Reused by draft upsert, approve, and rebase-apply — single source of validation truth.
func ValidateCompanyWorkflowOverrideSteps(steps []WorkflowStepDTO) error {
	if len(steps) == 0 {
		return perr.NewHTTPError(
			http.StatusBadRequest,
			perr.CodeWorkflowOverrideEmpty,
			"Workflow override must contain at least one step before it can be activated.",
			nil,
		)
	}
	for i := range steps {
		steps[i].StepID = strings.TrimSpace(steps[i].StepID)
		steps[i].Stage = strings.TrimSpace(steps[i].Stage)
		if steps[i].StepID == "" || steps[i].Stage == "" {
			return &perr.HTTPError{
				Code:       perr.CodeWorkflowOverrideInvalid,
				Message:    fmt.Sprintf("workflow step %d is invalid: step_id and stage are required", i),
				HTTPStatus: http.StatusBadRequest,
				Details: map[string]any{
					"field_errors": map[string]string{
						fmt.Sprintf("workflow[%d]", i): "step_id and stage are required",
					},
				},
			}
		}
		if len(steps[i].Instructions) > maxWorkflowOverrideStepInstructionsLen {
			return &perr.HTTPError{
				Code:       perr.CodeWorkflowOverrideInvalid,
				Message:    fmt.Sprintf("workflow step %d instructions exceeds maximum length", i),
				HTTPStatus: http.StatusBadRequest,
				Details: map[string]any{
					"field_errors": map[string]string{
						fmt.Sprintf("workflow[%d].instructions", i): "instructions must be at most 2000 characters",
					},
				},
			}
		}
		if len(steps[i].Description) > maxWorkflowOverrideStepDescriptionLen {
			return &perr.HTTPError{
				Code:       perr.CodeWorkflowOverrideInvalid,
				Message:    fmt.Sprintf("workflow step %d description exceeds maximum length", i),
				HTTPStatus: http.StatusBadRequest,
				Details: map[string]any{
					"field_errors": map[string]string{
						fmt.Sprintf("workflow[%d].description", i): "description must be at most 2000 characters",
					},
				},
			}
		}
		if err := ValidateWorkflowStepDescriptionFormatForPersist(steps[i].DescriptionFormat); err != nil {
			return &perr.HTTPError{
				Code:       perr.CodeWorkflowOverrideInvalid,
				Message:    fmt.Sprintf("workflow step %d has invalid description_format", i),
				HTTPStatus: http.StatusBadRequest,
				Details: map[string]any{
					"field_errors": map[string]string{
						fmt.Sprintf("workflow[%d].description_format", i): "description_format must be plain_text or safe_html",
					},
					"step_index": i,
					"field":      "description_format",
				},
			}
		}
		if strings.TrimSpace(steps[i].DescriptionFormat) != "" {
			steps[i].DescriptionFormat = NormalizeWorkflowStepDescriptionFormat(steps[i].DescriptionFormat)
			if steps[i].DescriptionFormat == WorkflowStepDescriptionFormatPlainText {
				steps[i].DescriptionFormat = ""
			}
		}
	}
	return nil
}
