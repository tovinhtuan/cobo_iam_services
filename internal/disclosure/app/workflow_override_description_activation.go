package app

import (
	"fmt"
	"net/http"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// ValidateCompanyWorkflowOverrideStepDescriptionsForActivation rejects semantic-empty
// descriptions at approve / apply / publish boundaries. Draft save must NOT call this.
// Uses the same IsWorkflowStepDescriptionSemanticEmpty helper as template Activate readiness.
func ValidateCompanyWorkflowOverrideStepDescriptionsForActivation(steps []WorkflowStepDTO) error {
	fieldErrors := map[string]string{}
	var firstMsg string
	for i, step := range steps {
		if !IsWorkflowStepDescriptionSemanticEmpty(step.Description, step.DescriptionFormat) {
			continue
		}
		msg := workflowStepDescriptionBlockerMessage(step.Stage, i)
		fieldErrors[fmt.Sprintf("workflow[%d].description", i)] = msg
		if firstMsg == "" {
			firstMsg = msg
		}
	}
	if len(fieldErrors) == 0 {
		return nil
	}
	return &perr.HTTPError{
		Code:       perr.Code(ActivationBlockerWorkflowStepDescriptionRequired),
		Message:    firstMsg,
		HTTPStatus: http.StatusUnprocessableEntity,
		Details: map[string]any{
			"field_errors": fieldErrors,
		},
	}
}
