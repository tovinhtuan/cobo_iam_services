package app

import (
	"fmt"
	"sort"
	"strings"

	adhocapp "github.com/cobo/cobo_iam_services/internal/adhoc/app"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	workflowerrs "github.com/cobo/cobo_iam_services/internal/workflow/errs"
)

// ErrEmptyWorkflowSnapshot is re-exported for callers that already depend on workflow/app.
var ErrEmptyWorkflowSnapshot = workflowerrs.ErrEmptyWorkflowSnapshot

// WorkflowSourceProposalSnapshotV2 labels instance snapshots built from frozen ad-hoc proposal workflow v2.
const WorkflowSourceProposalSnapshotV2 = "proposal_snapshot_v2"

// WorkflowSourceProposalSnapshotV3 labels instance snapshots built from frozen ad-hoc proposal workflow v3 (multi-assignee).
const WorkflowSourceProposalSnapshotV3 = "proposal_snapshot_v3"

// MapProposalWorkflowToSnapshot maps a frozen proposal schema-v2|v3 workflow into instance StepSnapshot rows.
// Proposal step id is runtime step_id/step_code authority; source_step_id is provenance only (ignored for identity).
// processing_days and department_id come from the frozen proposal — never from a live template.
// Step content (description, documents, reminder_config) comes only from the content captured at submit.
// v3: AssigneeMembershipIDs only (singular empty). v2: singular only.
func MapProposalWorkflowToSnapshot(snap *adhocapp.ProposalWorkflowSnapshot) []StepSnapshot {
	if snap == nil || len(snap.Steps) == 0 {
		return nil
	}
	out := make([]StepSnapshot, 0, len(snap.Steps))
	for _, step := range snap.Steps {
		stepID := strings.TrimSpace(step.ID)
		if stepID == "" {
			continue
		}
		name := strings.TrimSpace(step.Name)
		dept := strings.TrimSpace(step.DepartmentID)
		dueRule := ""
		if step.ProcessingDays > 0 {
			dueRule = fmt.Sprintf("T+%d", step.ProcessingDays)
		}
		// Proposals without captured content have no reminder_config. Flow B already seeded
		// scheduled milestones for timeline-enabled ad-hoc instances; freeze DEFAULT
		// [3,1] at map time so milestone build does not late-resolve later defaults.
		defaultDays := append([]int(nil), disclosureapp.DefaultWorkflowStepReminderDays...)
		row := StepSnapshot{
			StepID:                stepID,
			StepCode:              stepID,
			Stage:                 name,
			Department:            dept,
			DueRule:               dueRule,
			DisplayOrder:          step.Order,
			ProcessingDays:        step.ProcessingDays,
			EffectiveReminderDays: defaultDays,
			SourceStepID:          strings.TrimSpace(step.SourceStepID),
		}
		if inh := step.Inherited; inh != nil {
			row.Description = inh.Description
			row.DescriptionFormat = inh.DescriptionFormat
			row.Documents = mapProposalStepDocuments(inh.Documents)
			row.ReminderConfig = proposalReminderConfig(inh.ReminderConfig)
			res := disclosureapp.ResolveWorkflowStepReminderRule(row.ReminderConfig)
			row.EffectiveReminderDays = append([]int{}, res.EffectiveDays...)
		}
		if snap.SchemaVersion == adhocapp.ProposalWorkflowSchemaV3 {
			row.AssigneeMembershipIDs = adhocapp.EffectiveAssigneeMembershipIDs(step, adhocapp.ProposalWorkflowSchemaV3)
			// NO_V3_FIRST_ASSIGNEE_SHADOW_AUTHORITY — leave singular empty.
		} else {
			row.AssigneeMembershipID = strings.TrimSpace(step.AssigneeMembershipID)
		}
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].DisplayOrder == out[j].DisplayOrder {
			return out[i].StepID < out[j].StepID
		}
		return out[i].DisplayOrder < out[j].DisplayOrder
	})
	return out
}

func mapProposalStepDocuments(docs []adhocapp.ProposalStepDocument) []StepDocumentSnapshot {
	if len(docs) == 0 {
		return nil
	}
	out := make([]StepDocumentSnapshot, 0, len(docs))
	for _, d := range docs {
		tplID := strings.TrimSpace(d.TemplateFileID)
		tplName := strings.TrimSpace(d.TemplateFileName)
		if tplID == "" {
			tplName = ""
		}
		out = append(out, StepDocumentSnapshot{
			DocID:            strings.TrimSpace(d.DocID),
			Name:             strings.TrimSpace(d.Name),
			Required:         d.Required,
			TemplateFileID:   tplID,
			TemplateFileName: tplName,
		})
	}
	return out
}

func proposalReminderConfig(cfg *adhocapp.ProposalStepReminderConfig) *disclosureapp.WorkflowStepReminderConfig {
	if cfg == nil {
		return nil
	}
	return disclosureapp.CloneWorkflowStepReminderConfig(&disclosureapp.WorkflowStepReminderConfig{
		Enabled:    cfg.Enabled,
		Mode:       cfg.Mode,
		DaysBefore: cfg.DaysBefore,
	})
}

// MapEffectiveWorkflowToSnapshot converts effective workflow steps into frozen snapshot rows.
func MapEffectiveWorkflowToSnapshot(steps []disclosureapp.WorkflowStepDTO, workflowSource string) []StepSnapshot {
	_ = strings.TrimSpace(workflowSource)
	out := make([]StepSnapshot, 0, len(steps))
	for _, step := range steps {
		stepID := strings.TrimSpace(step.StepID)
		if stepID == "" {
			continue
		}
		stepCode := stepID
		assigneeRole := ""
		if len(step.AssigneeRoleIds) > 0 {
			assigneeRole = strings.Join(step.AssigneeRoleIds, ",")
		}
		department := strings.TrimSpace(step.DepartmentID)
		if department == "" {
			department = strings.TrimSpace(step.Stage)
		}
		dueRule := strings.TrimSpace(step.DueRule)
		if dueRule == "" {
			dueRule = "T+N"
		}
		res := disclosureapp.ResolveWorkflowStepReminderRule(step.ReminderConfig)
		effective := append([]int{}, res.EffectiveDays...)
		row := StepSnapshot{
			StepID:                stepID,
			StepCode:              stepCode,
			Stage:                 strings.TrimSpace(step.Stage),
			Department:            department,
			AssigneeRole:          assigneeRole,
			DueRule:               dueRule,
			DisplayOrder:          step.DisplayOrder,
			ProcessingDays:        step.ProcessingDays,
			ReminderConfig:        disclosureapp.CloneWorkflowStepReminderConfig(step.ReminderConfig),
			EffectiveReminderDays: effective,
		}
		if ids := ResolveTaskAssigneeMembershipIDs("", step.AssigneeMembershipIDs); len(ids) > 0 {
			row.AssigneeMembershipIDs = ids
		} else if singular := strings.TrimSpace(step.AssigneeMembershipID); singular != "" {
			row.AssigneeMembershipID = singular
		}
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].DisplayOrder == out[j].DisplayOrder {
			return out[i].StepID < out[j].StepID
		}
		return out[i].DisplayOrder < out[j].DisplayOrder
	})
	return out
}

// ApplyAdHocStepOverrides returns a copy of snapshot with processing_days patched by step_id.
func ApplyAdHocStepOverrides(snapshot []StepSnapshot, overrides []adhocapp.WorkflowStepOverride) []StepSnapshot {
	if len(snapshot) == 0 || len(overrides) == 0 {
		return append([]StepSnapshot(nil), snapshot...)
	}
	byStep := make(map[string]int, len(overrides))
	for _, o := range overrides {
		id := strings.TrimSpace(o.StepID)
		if id == "" {
			continue
		}
		byStep[id] = o.ProcessingDays
	}
	out := make([]StepSnapshot, len(snapshot))
	for i, step := range snapshot {
		out[i] = step
		if days, ok := byStep[step.StepID]; ok {
			out[i].ProcessingDays = days
		}
	}
	return out
}

// FirstStepCode returns the step_code of the snapshot step with the smallest display_order.
func FirstStepCode(snapshot []StepSnapshot) string {
	if len(snapshot) == 0 {
		return ""
	}
	best := snapshot[0]
	for _, step := range snapshot[1:] {
		if step.DisplayOrder < best.DisplayOrder {
			best = step
		} else if step.DisplayOrder == best.DisplayOrder && step.StepID < best.StepID {
			best = step
		}
	}
	code := strings.TrimSpace(best.StepCode)
	if code == "" {
		code = strings.TrimSpace(best.StepID)
	}
	return code
}

// ValidateSnapshot ensures at least one step exists when a snapshot is required, and that
// step identities (step_id, and step_code when set) are unique. It is the common sink of
// every workflow source (template, company override, CMS, ad-hoc proposal).
func ValidateSnapshot(snapshot []StepSnapshot) error {
	if len(snapshot) == 0 {
		return fmt.Errorf("%w", workflowerrs.ErrEmptyWorkflowSnapshot)
	}
	seenID := make(map[string]int, len(snapshot))
	seenCode := make(map[string]int, len(snapshot))
	for i, s := range snapshot {
		if id := strings.ToLower(strings.TrimSpace(s.StepID)); id != "" {
			if prev, ok := seenID[id]; ok {
				return fmt.Errorf("%w: step_id %q at positions %d and %d", workflowerrs.ErrDuplicateWorkflowStep, strings.TrimSpace(s.StepID), prev, i)
			}
			seenID[id] = i
		}
		if code := strings.ToLower(strings.TrimSpace(s.StepCode)); code != "" {
			if prev, ok := seenCode[code]; ok {
				return fmt.Errorf("%w: step_code %q at positions %d and %d", workflowerrs.ErrDuplicateWorkflowStep, strings.TrimSpace(s.StepCode), prev, i)
			}
			seenCode[code] = i
		}
	}
	return nil
}

// SnapshotValidationMessage returns the user-facing message for a ValidateSnapshot error,
// falling back to emptyMessage for an empty workflow.
func SnapshotValidationMessage(err error, emptyMessage string) string {
	if workflowerrs.IsDuplicateWorkflowStep(err) {
		return "workflow step_id must be unique"
	}
	return emptyMessage
}

// IsEmptyEffectiveWorkflow reports whether err is an empty effective workflow (ad-hoc 4xx or worker skip).
func IsEmptyEffectiveWorkflow(err error) bool {
	return workflowerrs.IsEmptyEffectiveWorkflow(err)
}
