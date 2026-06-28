package app

import (
	"fmt"
	"net/http"
	"strings"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

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
	}
	return nil
}
