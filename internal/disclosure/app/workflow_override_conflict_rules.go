package app

import (
	wfcapp "github.com/cobo/cobo_iam_services/internal/workflowconfig/app"
)

// Sprint 3 / Batch 4 — Workflow Override Conflict Detection, rules 3/4/5/7.
//
// Rules 1, 2, 6 are implemented inline in workflow_override_diff.go (ComputeRebaseDiff) since
// they fall directly out of the existing three-way step/field comparison. Rules 3, 4, 5, 7 need
// either a cross-field lookback on the SAME step (3, 4) or an external, static validation source
// (5, 7 — the Role Registry) that ComputeRebaseDiff's pure, dependency-free signature deliberately
// doesn't carry. These run as a SEPARATE pass, called by the service layer immediately after
// ComputeRebaseDiff, over the exact same three DiffStepInput lists — pure, no DB, no I/O (the
// Role Registry passed in is itself static and in-process, wfcapp.DefaultRoleRegistry()).
//
// Rule 8 (department existence) is explicitly NOT implemented here — see PREFLIGHT_AUDIT.md §5
// for the disclosed reason (would require a new cross-module DB dependency into
// internal/companyaccess that doesn't exist anywhere in this codebase today).

// DetectRule3And4Conflicts implements:
//   - Rule 3: global renamed a step's `stage` while the company independently changed a
//     DIFFERENT field (assignee_role_ids) on the SAME step. Per CONFLICT_DETECTION_MODEL.md rule
//     3, this is NOT a real conflict (different fields, both changes can be kept) but must still
//     be surfaced informationally — severity `info`.
//   - Rule 4: global reordered a step (display_order) while the company independently changed
//     `due_rule` on the SAME step. Per rule 4, also informational only — severity `info`.
//
// Both rules require the THIRD field to have NOT also changed on the same side (otherwise it's
// just an ordinary rule-1 same-field conflict, already handled elsewhere) — checked explicitly
// below to avoid double-reporting the same divergence under two different rule numbers.
func DetectRule3And4Conflicts(base, target, company []DiffStepInput) []PreviewConflict {
	baseByKey := indexByKey(base)
	targetByKey := indexByKey(target)
	companyByKey := indexByKey(company)

	var conflicts []PreviewConflict
	for key, b := range baseByKey {
		t, hasT := targetByKey[key]
		c, hasC := companyByKey[key]
		if !hasT || !hasC {
			continue
		}

		// Rule 3: global renamed stage, company changed assignees (not stage), informational.
		globalRenamed := t.Stage != b.Stage
		companyChangedStage := c.Stage != b.Stage
		companyChangedAssignees := !sameStringSet(c.AssigneeRoleIds, b.AssigneeRoleIds)
		if globalRenamed && !companyChangedStage && companyChangedAssignees {
			conflicts = append(conflicts, PreviewConflict{
				TemporaryConflictID: "preview-" + key + "-rule3",
				StepKey:             key,
				FieldPath:           "stage",
				Severity:            ConflictSeverityInfo,
				ConflictType:        ConflictTypeGlobalRenamedCompanyReassigned,
				GlobalOld:           b.Stage,
				GlobalNew:           t.Stage,
				CompanyValue:        c.AssigneeRoleIds,
				ResolutionOptions:   []string{ResolutionKeepCompany, ResolutionAcceptGlobal},
			})
		}

		// Rule 4: global reordered, company changed due_rule (not display_order), informational.
		globalReordered := t.DisplayOrder != b.DisplayOrder
		companyReordered := c.DisplayOrder != b.DisplayOrder
		companyChangedDueRule := c.DueRule != b.DueRule
		if globalReordered && !companyReordered && companyChangedDueRule {
			conflicts = append(conflicts, PreviewConflict{
				TemporaryConflictID: "preview-" + key + "-rule4",
				StepKey:             key,
				FieldPath:           "display_order",
				Severity:            ConflictSeverityInfo,
				ConflictType:        ConflictTypeReorderAndDueDateChanged,
				GlobalOld:           b.DisplayOrder,
				GlobalNew:           t.DisplayOrder,
				CompanyValue:        c.DueRule,
				ResolutionOptions:   []string{ResolutionKeepCompany, ResolutionAcceptGlobal},
			})
		}
	}
	return conflicts
}

// DetectRule5Conflicts implements: a brand-new global step (add_step) whose required role(s)
// cannot be resolved against the Role Registry. Per CONFLICT_DETECTION_MODEL.md rule 5, a new
// global step is "not a conflict by default" — it only becomes one via this exact check (folds
// into rule 7's same registry). registry must not be nil; callers pass
// wfcapp.DefaultRoleRegistry().
func DetectRule5Conflicts(ops []PatchOperation, registry *wfcapp.RoleRegistry) []PreviewConflict {
	var conflicts []PreviewConflict
	for _, op := range ops {
		if op.Op != OpAddStep || op.NewStep == nil {
			continue
		}
		var unresolved []string
		for _, roleID := range op.NewStep.AssigneeRoleIds {
			if _, ok := registry.GetRole(roleID); !ok {
				unresolved = append(unresolved, roleID)
			}
		}
		if len(unresolved) > 0 {
			conflicts = append(conflicts, PreviewConflict{
				TemporaryConflictID: "preview-" + op.StepKey + "-rule5",
				StepKey:             op.StepKey,
				FieldPath:           "assignee_role_ids",
				Severity:            ConflictSeverityBlocking,
				ConflictType:        ConflictTypeNewMandatoryStepUnresolvedRole,
				GlobalOld:           nil,
				GlobalNew:           op.NewStep.AssigneeRoleIds,
				CompanyValue:        nil,
				ResolutionOptions:   []string{ResolutionMergeManual},
				Reason:              ConflictReasonRoleUnresolved,
			})
		}
	}
	return conflicts
}

// DetectRule7Conflicts implements: for every step in the override's CURRENT snapshot, every
// assignee_role_ids entry must resolve against the Role Registry — independent of whether that
// step had any other diff signal at all (a role can go stale without the step itself changing).
func DetectRule7Conflicts(company []DiffStepInput, registry *wfcapp.RoleRegistry) []PreviewConflict {
	var conflicts []PreviewConflict
	for _, in := range company {
		if in.Key == "" || in.Step == nil {
			continue
		}
		var unresolved []string
		for _, roleID := range in.Step.AssigneeRoleIds {
			if _, ok := registry.GetRole(roleID); !ok {
				unresolved = append(unresolved, roleID)
			}
		}
		if len(unresolved) > 0 {
			conflicts = append(conflicts, PreviewConflict{
				TemporaryConflictID: "preview-" + in.Key + "-rule7",
				StepKey:             in.Key,
				FieldPath:           "assignee_role_ids",
				Severity:            ConflictSeverityBlocking,
				ConflictType:        ConflictTypeRoleNoLongerExists,
				GlobalOld:           nil,
				GlobalNew:           nil,
				CompanyValue:        in.Step.AssigneeRoleIds,
				ResolutionOptions:   []string{ResolutionMergeManual},
				Reason:              ConflictReasonRoleUnresolved,
			})
		}
	}
	return conflicts
}

func indexByKey(inputs []DiffStepInput) map[string]*WorkflowStepDTO {
	out := map[string]*WorkflowStepDTO{}
	for _, in := range inputs {
		if in.Key == "" {
			continue
		}
		out[in.Key] = in.Step
	}
	return out
}

func sameStringSet(a, b []string) bool {
	as := sortedUniqueStrings(a)
	bs := sortedUniqueStrings(b)
	if len(as) != len(bs) {
		return false
	}
	for i := range as {
		if as[i] != bs[i] {
			return false
		}
	}
	return true
}
