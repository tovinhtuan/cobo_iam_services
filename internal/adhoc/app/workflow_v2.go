package app

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

const (
	// ProposalWorkflowSchemaV2 is the proposal-owned full workflow snapshot.
	ProposalWorkflowSchemaV2 = 2
	// MaxProposalWorkflowSteps is a technical guardrail (not a product business limit).
	MaxProposalWorkflowSteps = 50
	// MinProposalWorkflowSteps required for schema v2.
	MinProposalWorkflowSteps = 1
)

// ProposalWorkflowStep is one step in a proposal-owned workflow snapshot (schema v2).
type ProposalWorkflowStep struct {
	ID                   string `json:"id"`
	SourceStepID         string `json:"source_step_id,omitempty"`
	Order                int    `json:"order"`
	Name                 string `json:"name"`
	ProcessingDays       int    `json:"processing_days"`
	DepartmentID         string `json:"department_id,omitempty"`
	AssigneeMembershipID string `json:"assignee_membership_id,omitempty"`
}

// ProposalWorkflowSnapshot is the authority for schema_version=2 proposals.
type ProposalWorkflowSnapshot struct {
	SchemaVersion    int                    `json:"schema_version"`
	DisclosureTypeID string                 `json:"disclosure_type_id,omitempty"`
	Frozen           bool                   `json:"frozen"`
	Steps            []ProposalWorkflowStep `json:"steps"`
}

// ProposalWorkflowStepInput is the create/PATCH wire item (client may send client_id or existing id).
type ProposalWorkflowStepInput struct {
	ID                   string `json:"id,omitempty"`
	ClientID             string `json:"client_id,omitempty"`
	SourceStepID         string `json:"source_step_id,omitempty"`
	Order                int    `json:"order,omitempty"`
	Name                 string `json:"name"`
	ProcessingDays       int    `json:"processing_days"`
	DepartmentID         string `json:"department_id,omitempty"`
	AssigneeMembershipID string `json:"assignee_membership_id,omitempty"`
}

// ResolveProposalWorkflowContractVersion classifies persisted JSON authority.
func ResolveProposalWorkflowContractVersion(snap *ProposalWorkflowSnapshot, legacyOverrides []WorkflowStepOverride) int {
	if snap != nil && snap.SchemaVersion == ProposalWorkflowSchemaV2 {
		return ProposalWorkflowSchemaV2
	}
	return 1
}

// DeriveLegacyStepOverrides projects v2 steps to legacy override view for dual-read responses.
func DeriveLegacyStepOverrides(steps []ProposalWorkflowStep) []WorkflowStepOverride {
	out := make([]WorkflowStepOverride, 0, len(steps))
	for _, s := range steps {
		stepID := strings.TrimSpace(s.SourceStepID)
		if stepID == "" {
			stepID = strings.TrimSpace(s.ID)
		}
		if stepID == "" {
			continue
		}
		out = append(out, WorkflowStepOverride{StepID: stepID, ProcessingDays: s.ProcessingDays})
	}
	return out
}

// NormalizeProposalWorkflowSteps validates structure and returns a normalized v2 snapshot.
// existingIDs: proposal step IDs already persisted (PATCH); empty for create.
// Order is normalized from array position to contiguous 1..N.
func NormalizeProposalWorkflowSteps(
	typeID string,
	inputs []ProposalWorkflowStepInput,
	existingIDs map[string]struct{},
	frozen bool,
	newID func() string,
) (*ProposalWorkflowSnapshot, error) {
	if len(inputs) < MinProposalWorkflowSteps {
		return nil, newAdHocFieldError(http.StatusBadRequest, perr.CodeInvalidRequest, "workflow_steps", "workflow_steps must contain at least one step")
	}
	if len(inputs) > MaxProposalWorkflowSteps {
		return nil, newAdHocFieldError(http.StatusBadRequest, perr.CodeInvalidRequest, "workflow_steps", fmt.Sprintf("workflow_steps exceeds technical maximum of %d", MaxProposalWorkflowSteps))
	}
	if newID == nil {
		newID = func() string { return uuid.NewString() }
	}
	if existingIDs == nil {
		existingIDs = map[string]struct{}{}
	}

	steps := make([]ProposalWorkflowStep, 0, len(inputs))
	usedIDs := make(map[string]struct{}, len(inputs))
	for i, in := range inputs {
		name := strings.TrimSpace(in.Name)
		if name == "" {
			return nil, newAdHocFieldError(http.StatusBadRequest, perr.CodeInvalidRequest, fmt.Sprintf("workflow_steps[%d].name", i), "name is required")
		}
		if in.ProcessingDays < 0 {
			return nil, newAdHocFieldError(http.StatusBadRequest, perr.CodeInvalidRequest, fmt.Sprintf("workflow_steps[%d].processing_days", i), "processing_days must be >= 0")
		}
		dept := strings.TrimSpace(in.DepartmentID)
		assignee := strings.TrimSpace(in.AssigneeMembershipID)
		if assignee != "" && dept == "" {
			return nil, newAdHocFieldError(http.StatusBadRequest, perr.CodeInvalidRequest, fmt.Sprintf("workflow_steps[%d].assignee_membership_id", i), "assignee_membership_id requires department_id")
		}

		id := strings.TrimSpace(in.ID)
		if id != "" {
			if _, ok := existingIDs[id]; !ok {
				// Unknown id is not trusted; allocate a new server id.
				id = ""
			}
		}
		if id == "" {
			id = newID()
		}
		if _, dup := usedIDs[id]; dup {
			id = newID()
		}
		usedIDs[id] = struct{}{}

		steps = append(steps, ProposalWorkflowStep{
			ID:                   id,
			SourceStepID:         strings.TrimSpace(in.SourceStepID),
			Order:                i + 1,
			Name:                 name,
			ProcessingDays:       in.ProcessingDays,
			DepartmentID:         dept,
			AssigneeMembershipID: assignee,
		})
	}

	return &ProposalWorkflowSnapshot{
		SchemaVersion:    ProposalWorkflowSchemaV2,
		DisclosureTypeID: strings.TrimSpace(typeID),
		Frozen:           frozen,
		Steps:            steps,
	}, nil
}
