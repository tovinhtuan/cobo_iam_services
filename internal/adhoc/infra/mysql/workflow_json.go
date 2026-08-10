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

func marshalProposalWorkflowPayload(p adhocapp.ProposalDTO, embedDays *int) (string, error) {
	if p.Workflow != nil && (p.Workflow.SchemaVersion == adhocapp.ProposalWorkflowSchemaV2 ||
		p.Workflow.SchemaVersion == adhocapp.ProposalWorkflowSchemaV3) {
		raw, err := json.Marshal(p.Workflow)
		if err != nil {
			return "", err
		}
		return string(raw), nil
	}
	return marshalProposedWorkflowJSON(p.StepOverrides, embedDays)
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
	// Schema v2 snapshot — handled by decodeProposalWorkflowPayload; here treat as empty legacy.
	var probe map[string]json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &probe); err == nil {
		if _, ok := probe["schema_version"]; ok {
			return []adhocapp.WorkflowStepOverride{}, nil, nil
		}
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

func decodeProposalWorkflowPayload(raw string) (steps []adhocapp.WorkflowStepOverride, days *int, snap *adhocapp.ProposalWorkflowSnapshot, err error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "null" {
		return []adhocapp.WorkflowStepOverride{}, nil, nil, nil
	}
	if strings.HasPrefix(trimmed, "[") {
		steps, days, err = unmarshalProposedWorkflowJSON(trimmed)
		return steps, days, nil, err
	}
	var probe struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal([]byte(trimmed), &probe); err != nil {
		return nil, nil, nil, err
	}
	if probe.SchemaVersion == adhocapp.ProposalWorkflowSchemaV2 || probe.SchemaVersion == adhocapp.ProposalWorkflowSchemaV3 {
		var snap adhocapp.ProposalWorkflowSnapshot
		if err := json.Unmarshal([]byte(trimmed), &snap); err != nil {
			return nil, nil, nil, err
		}
		if snap.Steps == nil {
			snap.Steps = []adhocapp.ProposalWorkflowStep{}
		}
		return adhocapp.DeriveLegacyStepOverrides(snap.Steps), nil, &snap, nil
	}
	steps, days, err = unmarshalProposedWorkflowJSON(trimmed)
	return steps, days, nil, err
}
