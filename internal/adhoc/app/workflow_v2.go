package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

const (
	// ProposalWorkflowSchemaV2 is the proposal-owned full workflow snapshot (singular assignee).
	ProposalWorkflowSchemaV2 = 2
	// ProposalWorkflowSchemaV3 is multi-assignee proposal-owned workflow (M0/M1).
	ProposalWorkflowSchemaV3 = 3
	// MaxProposalWorkflowSteps is a technical guardrail (not a product business limit).
	MaxProposalWorkflowSteps = 50
	// MinProposalWorkflowSteps required for schema v2/v3.
	MinProposalWorkflowSteps = 1
)

// ProposalWorkflowStep is one step in a proposal-owned workflow snapshot (v2 or v3).
//
// Assignment authority:
//   - schema_version=2: AssigneeMembershipID (singular)
//   - schema_version=3: AssigneeMembershipIDs (array; may be empty only while draft)
type ProposalWorkflowStep struct {
	ID                    string   `json:"id"`
	SourceStepID          string   `json:"source_step_id,omitempty"`
	Order                 int      `json:"order"`
	Name                  string   `json:"name"`
	ProcessingDays        int      `json:"processing_days"`
	DepartmentID          string   `json:"department_id,omitempty"`
	AssigneeMembershipID  string   `json:"assignee_membership_id,omitempty"`
	AssigneeMembershipIDs []string `json:"assignee_membership_ids,omitempty"`
}

// ProposalWorkflowSnapshot is the authority for schema_version=2|3 proposals.
type ProposalWorkflowSnapshot struct {
	SchemaVersion    int                    `json:"schema_version"`
	DisclosureTypeID string                 `json:"disclosure_type_id,omitempty"`
	Frozen           bool                   `json:"frozen"`
	Steps            []ProposalWorkflowStep `json:"steps"`
}

// ProposalWorkflowStepInput is the create/PATCH wire item (client may send client_id or existing id).
//
// AssigneeMembershipIDs uses a pointer so JSON can distinguish omit vs present [] vs present [ids].
// Whole-workflow replacement remains the PATCH contract when workflow_steps is sent.
type ProposalWorkflowStepInput struct {
	ID                    string    `json:"id,omitempty"`
	ClientID              string    `json:"client_id,omitempty"`
	SourceStepID          string    `json:"source_step_id,omitempty"`
	Order                 int       `json:"order,omitempty"`
	Name                  string    `json:"name"`
	ProcessingDays        int       `json:"processing_days"`
	DepartmentID          string    `json:"department_id,omitempty"`
	AssigneeMembershipID  string    `json:"assignee_membership_id,omitempty"`
	AssigneeMembershipIDs *[]string `json:"assignee_membership_ids,omitempty"`
}

// ResolveProposalWorkflowContractVersion classifies persisted JSON authority.
func ResolveProposalWorkflowContractVersion(snap *ProposalWorkflowSnapshot, legacyOverrides []WorkflowStepOverride) int {
	if snap != nil && snap.SchemaVersion == ProposalWorkflowSchemaV3 {
		return ProposalWorkflowSchemaV3
	}
	if snap != nil && snap.SchemaVersion == ProposalWorkflowSchemaV2 {
		return ProposalWorkflowSchemaV2
	}
	return 1
}

// EffectiveAssigneeMembershipIDs returns the assignment list for a step.
// v3: array only (may be empty in draft).
// v2 / compatibility: singular → singleton when array empty.
func EffectiveAssigneeMembershipIDs(step ProposalWorkflowStep, schemaVersion int) []string {
	ids := normalizeAssigneeIDList(step.AssigneeMembershipIDs)
	if len(ids) > 0 {
		return ids
	}
	if schemaVersion == ProposalWorkflowSchemaV3 {
		return nil
	}
	if singular := strings.TrimSpace(step.AssigneeMembershipID); singular != "" {
		return []string{singular}
	}
	return nil
}

// DeriveLegacyStepOverrides projects v2/v3 steps to legacy override view for dual-read responses.
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

// NormalizeProposalWorkflowSteps validates structure and returns a normalized v2 or v3 snapshot.
// existingIDs: proposal step IDs already persisted (PATCH); empty for create.
// Order is normalized from array position to contiguous 1..N.
//
// Schema selection:
//   - if any step wire includes assignee_membership_ids (pointer non-nil) → schema_version=3
//   - else → schema_version=2 (legacy singular clients)
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

	useV3 := false
	for _, in := range inputs {
		if in.AssigneeMembershipIDs != nil {
			useV3 = true
			break
		}
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
		singular := strings.TrimSpace(in.AssigneeMembershipID)

		if useV3 {
			if in.AssigneeMembershipIDs != nil && len(*in.AssigneeMembershipIDs) > 0 && singular != "" {
				return nil, newAdHocFieldError(http.StatusBadRequest, perr.CodeInvalidRequest, fmt.Sprintf("workflow_steps[%d]", i),
					"workflow_contract_conflict: assignee_membership_id and assignee_membership_ids cannot both be set")
			}
		} else if singular != "" && dept == "" {
			return nil, newAdHocFieldError(http.StatusBadRequest, perr.CodeInvalidRequest, fmt.Sprintf("workflow_steps[%d].assignee_membership_id", i), "assignee_membership_id requires department_id")
		}

		id := strings.TrimSpace(in.ID)
		if id != "" {
			if _, ok := existingIDs[id]; !ok {
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

		step := ProposalWorkflowStep{
			ID:             id,
			SourceStepID:   strings.TrimSpace(in.SourceStepID),
			Order:          i + 1,
			Name:           name,
			ProcessingDays: in.ProcessingDays,
			DepartmentID:   dept,
		}

		if useV3 {
			var ids []string
			var err error
			if in.AssigneeMembershipIDs != nil {
				ids, err = normalizeAssigneeIDsStrict(*in.AssigneeMembershipIDs, fmt.Sprintf("workflow_steps[%d].assignee_membership_ids", i))
				if err != nil {
					return nil, err
				}
			} else if singular != "" {
				ids = []string{singular}
			}
			if len(ids) > 0 && dept == "" {
				return nil, newAdHocFieldError(http.StatusBadRequest, perr.CodeInvalidRequest, fmt.Sprintf("workflow_steps[%d].assignee_membership_ids", i), "assignee_membership_ids requires department_id")
			}
			step.AssigneeMembershipIDs = ids
			step.AssigneeMembershipID = ""
		} else {
			step.AssigneeMembershipID = singular
		}

		steps = append(steps, step)
	}

	schema := ProposalWorkflowSchemaV2
	if useV3 {
		schema = ProposalWorkflowSchemaV3
	}
	return &ProposalWorkflowSnapshot{
		SchemaVersion:    schema,
		DisclosureTypeID: strings.TrimSpace(typeID),
		Frozen:           frozen,
		Steps:            steps,
	}, nil
}

func normalizeAssigneeIDList(raw []string) []string {
	if len(raw) == 0 {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, id := range raw {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		out = append(out, id)
	}
	return out
}

// normalizeAssigneeIDsStrict trims, rejects empty entries, rejects duplicates (reviewer convention).
func normalizeAssigneeIDsStrict(raw []string, field string) ([]string, error) {
	out := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, id := range raw {
		id = strings.TrimSpace(id)
		if id == "" {
			return nil, newAdHocFieldError(http.StatusBadRequest, perr.CodeInvalidRequest, field, field+" must not contain empty membership ids")
		}
		if _, dup := seen[id]; dup {
			return nil, newAdHocFieldError(http.StatusBadRequest, perr.CodeInvalidRequest, field, field+" must not contain duplicates")
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}

// NormalizeAndValidateWorkflowForSubmitV3 enforces department + assignment (explicit or head default).
// Returns a cloned snapshot with resolved non-empty AssigneeMembershipIDs on every step (not yet frozen).
func NormalizeAndValidateWorkflowForSubmitV3(ctx context.Context, org OrgDirectory, companyID string, snap *ProposalWorkflowSnapshot) (*ProposalWorkflowSnapshot, error) {
	if snap == nil || snap.SchemaVersion != ProposalWorkflowSchemaV3 {
		return nil, newAdHocFieldError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, "workflow.schema_version", "schema_version must be 3 for v3 submit normalization")
	}
	if org == nil {
		return nil, perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "org directory is required for v3 submit validation", nil)
	}
	out := *snap
	out.Steps = make([]ProposalWorkflowStep, len(snap.Steps))
	copy(out.Steps, snap.Steps)

	for i := range out.Steps {
		dept := strings.TrimSpace(out.Steps[i].DepartmentID)
		if dept == "" {
			return nil, newAdHocFieldError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, fmt.Sprintf("workflow_steps[%d].department_id", i), "department_required: department_id is required")
		}
		out.Steps[i].DepartmentID = dept
		ids := normalizeAssigneeIDList(out.Steps[i].AssigneeMembershipIDs)
		if len(ids) == 0 {
			headID, err := org.ResolveDepartmentHeadMembership(ctx, companyID, dept)
			if err != nil {
				return nil, mapDepartmentHeadError(err, i)
			}
			ids = []string{headID}
		}
		out.Steps[i].AssigneeMembershipIDs = ids
		out.Steps[i].AssigneeMembershipID = ""
	}
	if err := ValidateWorkflowStepOrgRefs(ctx, org, companyID, out.Steps); err != nil {
		return nil, err
	}
	for i, step := range out.Steps {
		if len(normalizeAssigneeIDList(step.AssigneeMembershipIDs)) == 0 {
			return nil, newAdHocFieldError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, fmt.Sprintf("workflow_steps[%d].assignee_membership_ids", i), "assignee_membership_ids must be non-empty after submit normalization")
		}
	}
	return &out, nil
}

func mapDepartmentHeadError(err error, stepIndex int) error {
	if err == nil {
		return nil
	}
	field := fmt.Sprintf("workflow_steps[%d].assignee_membership_ids", stepIndex)
	if he, ok := perr.AsHTTPError(err); ok {
		if he.Details == nil {
			he.Details = map[string]any{}
		}
		if _, has := he.Details["field"]; !has {
			he.Details["field"] = field
		}
		return he
	}
	return mapRepositoryError(fmt.Errorf("resolve department head: %w", err))
}
