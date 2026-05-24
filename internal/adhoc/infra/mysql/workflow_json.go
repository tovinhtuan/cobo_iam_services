package mysql

import (
	"encoding/json"
	"strings"

	adhocapp "github.com/cobo/cobo_iam_services/internal/adhoc/app"
)

type proposedWorkflowEnvelope struct {
	StepOverrides        []adhocapp.WorkflowStepOverride `json:"step_overrides"`
	ProposedDeadlineDays *int                            `json:"proposed_deadline_days,omitempty"`
}

func marshalProposedWorkflowJSON(steps []adhocapp.WorkflowStepOverride, embedDays *int) (string, error) {
	if steps == nil {
		steps = []adhocapp.WorkflowStepOverride{}
	}
	if embedDays != nil {
		raw, err := json.Marshal(proposedWorkflowEnvelope{
			StepOverrides:        steps,
			ProposedDeadlineDays: embedDays,
		})
		if err != nil {
			return "", err
		}
		return string(raw), nil
	}
	raw, err := json.Marshal(steps)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func unmarshalProposedWorkflowJSON(raw string) ([]adhocapp.WorkflowStepOverride, *int, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "null" {
		return []adhocapp.WorkflowStepOverride{}, nil, nil
	}
	if strings.HasPrefix(trimmed, "[") {
		var steps []adhocapp.WorkflowStepOverride
		if err := json.Unmarshal([]byte(trimmed), &steps); err != nil {
			return nil, nil, err
		}
		if steps == nil {
			steps = []adhocapp.WorkflowStepOverride{}
		}
		return steps, nil, nil
	}
	var env proposedWorkflowEnvelope
	if err := json.Unmarshal([]byte(trimmed), &env); err != nil {
		return nil, nil, err
	}
	if env.StepOverrides == nil {
		env.StepOverrides = []adhocapp.WorkflowStepOverride{}
	}
	return env.StepOverrides, env.ProposedDeadlineDays, nil
}
