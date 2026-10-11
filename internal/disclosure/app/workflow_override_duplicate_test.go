package app

import (
	"errors"
	"net/http"
	"testing"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func TestValidateCompanyWorkflowOverrideSteps_RejectsDuplicateStepID(t *testing.T) {
	err := ValidateCompanyWorkflowOverrideSteps([]WorkflowStepDTO{
		{StepID: "s1", Stage: "Soạn"},
		{StepID: " s1 ", Stage: "Duyệt"},
	})
	var he *perr.HTTPError
	if !errors.As(err, &he) || he.HTTPStatus != http.StatusBadRequest || he.Code != perr.CodeWorkflowOverrideInvalid {
		t.Fatalf("want 400 %s, got %v", perr.CodeWorkflowOverrideInvalid, err)
	}
	fe, _ := he.Details["field_errors"].(map[string]string)
	if _, ok := fe["workflow[1].step_id"]; !ok {
		t.Fatalf("field error on workflow[1].step_id expected, got %v", he.Details)
	}
}

// A generated "<type>-step-N" id can collide with an explicit id sent by the client.
func TestFacadeStepIdentities_GeneratedIDCollision(t *testing.T) {
	req := CmsUpsertGlobalWorkflowRequest{TypeID: "T", Steps: []GlobalWorkflowStepInput{
		{StepID: "", Stage: "Soạn"},
		{StepID: "T-step-1", Stage: "Duyệt"},
	}}
	if err := rejectDuplicateIncomingIdentities(req.Steps); err != nil {
		t.Fatalf("incoming ids are distinct before generation: %v", err)
	}
	if err := normalizeFacadeStepIdentities(&req, &DisclosureTypeDTO{}); err == nil {
		t.Fatal("generated id colliding with an explicit id must be rejected")
	}
}
