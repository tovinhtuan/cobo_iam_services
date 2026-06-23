package app

import (
	"reflect"
	"sort"
)

// Sprint 3 / Batch 3 — Workflow Override Rebase Preview.
//
// This file owns ONLY the pure diff/patch computation (no DB, no HTTP, no persistence). It is
// deliberately decoupled from how callers obtain the three input step lists — see
// workflow_override_rebase_preview.go for the bridging logic that builds DiffStepInput lists from
// real base/target global manifests and the override's current snapshot (PREFLIGHT_AUDIT.md §5).
//
// Three-way comparison model (PATCH_MODEL_OPTIONS.md §1): for each bridged step identity, compare
// the BASE value, the TARGET (current active global) value, and the COMPANY (override's current
// snapshot) value of every field. Per field:
//   - company_changed  := company != base
//   - global_changed   := target != base
//   - both changed, same new value   -> no real divergence; treated as a clean global_changed op
//   - both changed, different values -> CONFLICT (rule 1)
//   - only global changed            -> clean update_step_field/specialized op, origin=global_changed
//   - only company changed           -> nothing to do (company's customization already reflects
//                                        their intent; rebasing doesn't need to touch this field)
//   - neither changed                -> no operation

// Patch operation types (op field values, PATCH_MODEL_OPTIONS.md §6).
const (
	OpAddStep          = "add_step"
	OpRemoveStep       = "remove_step"
	OpUpdateStepField  = "update_step_field"
	OpReorderStep      = "reorder_step"
	OpReplaceAssignees = "replace_assignees"
	OpReplaceDueRule   = "replace_due_rule"
	OpReplaceReminders = "replace_reminders"
)

// Origin tags (PATCH_MODEL_OPTIONS.md §6).
const (
	OriginGlobalChanged  = "global_changed"
	OriginCompanyChanged = "company_changed"
	OriginBothChanged    = "both_changed"
)

// Conflict severities (CONFLICT_DETECTION_MODEL.md).
const (
	ConflictSeverityBlocking = "blocking"
	ConflictSeverityAdvisory = "advisory"
)

// Conflict reasons.
const (
	ConflictReasonStepIdentityUnclear = "STEP_IDENTITY_UNCLEAR"
	ConflictReasonRoleUnresolved      = "ROLE_UNRESOLVED"
)

// Resolution option values (CONFLICT_DETECTION_MODEL.md).
const (
	ResolutionKeepCompany       = "keep_company"
	ResolutionAcceptGlobal      = "accept_global"
	ResolutionMergeManual       = "merge_manual"
	ResolutionCreateNewStep     = "create_new_step"
	ResolutionMarkNotApplicable = "mark_not_applicable"
)

// PatchOperation is one entry in a rebase preview's patch_operations[]. Exactly one operation
// "shape" (the field group matching Op) is populated at a time; the rest stay zero-valued and are
// omitted from JSON via `omitempty`.
type PatchOperation struct {
	Op      string `json:"op"`
	StepKey string `json:"step_key"`
	Origin  string `json:"origin"`

	// add_step
	NewStep *WorkflowStepDTO `json:"new_step,omitempty"`

	// update_step_field (stage, department_id, documents, groups — the generic catch-all)
	FieldPath string `json:"field_path,omitempty"`
	From      any    `json:"from,omitempty"`
	To        any    `json:"to,omitempty"`

	// reorder_step
	OldDisplayOrder *int `json:"old_display_order,omitempty"`
	NewDisplayOrder *int `json:"new_display_order,omitempty"`

	// replace_assignees
	OldAssigneeRoleIds []string `json:"old_assignee_role_ids,omitempty"`
	NewAssigneeRoleIds []string `json:"new_assignee_role_ids,omitempty"`

	// replace_due_rule
	OldDueRule string `json:"old_due_rule,omitempty"`
	NewDueRule string `json:"new_due_rule,omitempty"`

	// replace_reminders
	OldReminderConfig *WorkflowStepReminderConfig `json:"old_reminder_config,omitempty"`
	NewReminderConfig *WorkflowStepReminderConfig `json:"new_reminder_config,omitempty"`
}

// PreviewConflict is one informational, non-persisted conflict entry (CONFLICT_DETECTION_MODEL.md
// output shape). `TemporaryConflictID` is preview-scoped only — it has no durable identity and is
// never written to any table (Batch 4's job).
type PreviewConflict struct {
	TemporaryConflictID string   `json:"temporary_conflict_id"`
	StepKey             string   `json:"step_key"`
	FieldPath           string   `json:"field_path"`
	Severity            string   `json:"severity"`
	GlobalOld           any      `json:"global_old"`
	GlobalNew           any      `json:"global_new"`
	CompanyValue        any      `json:"company_value"`
	ResolutionOptions   []string `json:"resolution_options"`
	Reason              string   `json:"reason,omitempty"`
}

// DiffStepInput is one bridged step on one side (base/target/company) of the three-way
// comparison. Key is the resolved stable identity (PREFLIGHT_AUDIT.md §5) — empty Key means
// identity could not be determined for this step (triggers a STEP_IDENTITY_UNCLEAR conflict,
// Patch Engine Requirement #3) and is excluded from normal field comparison. Step is nil when
// this side has no step for this Key at all (used by add_step/remove_step detection).
type DiffStepInput struct {
	Key  string
	Step *WorkflowStepDTO
}

// ComputeRebaseDiff is the pure, deterministic entry point. base/target/company are independent
// lists (no shared ordering requirement) — every aggregation here is done via maps and then
// sorted before returning, so the output is reproducible across repeated calls. No DB access, no
// I/O, no mutation of any input slice/struct.
func ComputeRebaseDiff(base, target, company []DiffStepInput) ([]PatchOperation, []PreviewConflict) {
	// Always non-nil: a nil slice serializes to JSON `null`, which every consumer (the FE modal,
	// API clients) must otherwise null-check defensively. An empty array is a cleaner contract.
	ops := []PatchOperation{}
	conflicts := []PreviewConflict{}

	baseByKey := map[string]*WorkflowStepDTO{}
	targetByKey := map[string]*WorkflowStepDTO{}
	companyByKey := map[string]*WorkflowStepDTO{}

	for _, in := range base {
		if in.Key == "" {
			conflicts = append(conflicts, unclearIdentityConflict("base", in.Step))
			continue
		}
		baseByKey[in.Key] = in.Step
	}
	for _, in := range target {
		if in.Key == "" {
			conflicts = append(conflicts, unclearIdentityConflict("target", in.Step))
			continue
		}
		targetByKey[in.Key] = in.Step
	}
	for _, in := range company {
		if in.Key == "" {
			conflicts = append(conflicts, unclearIdentityConflict("company", in.Step))
			continue
		}
		companyByKey[in.Key] = in.Step
	}

	allKeys := map[string]bool{}
	for k := range baseByKey {
		allKeys[k] = true
	}
	for k := range targetByKey {
		allKeys[k] = true
	}
	for k := range companyByKey {
		allKeys[k] = true
	}

	for key := range allKeys {
		b, hasB := baseByKey[key]
		t, hasT := targetByKey[key]
		c, hasC := companyByKey[key]

		switch {
		case hasC && !hasB && !hasT:
			// Company-only step (added by the company, never existed in any global version).
			// Nothing to rebase for it — it survives untouched. Not an operation, not a conflict.
			continue

		case hasB && !hasT && hasC:
			// Rule 2 / Rule 6: global deleted this step. If the company's current value still
			// equals the base value (company never touched it), this is rule 6 territory only if
			// we think of "removed by global" as requiring a decision either way; per
			// CONFLICT_DETECTION_MODEL.md rule 2, ANY company customization on a step the global
			// side deleted is a conflict, severity blocking, resolution create_new_step/keep_company.
			conflicts = append(conflicts, PreviewConflict{
				TemporaryConflictID: "preview-" + key + "-existence",
				StepKey:             key,
				FieldPath:           "__step_existence__",
				Severity:            ConflictSeverityBlocking,
				GlobalOld:           b,
				GlobalNew:           nil,
				CompanyValue:        c,
				ResolutionOptions:   []string{ResolutionKeepCompany, ResolutionCreateNewStep},
			})
			continue

		case hasB && !hasT && !hasC:
			// Global deleted the step AND the company's current snapshot has already dropped it
			// too (rule 6: the company removed/never carried this step forward). Informational
			// remove_step entry — global_changed, no conflict (nothing of the company's is lost).
			ops = append(ops, PatchOperation{Op: OpRemoveStep, StepKey: key, Origin: OriginGlobalChanged})
			continue

		case !hasB && hasT && !hasC:
			// Global added a brand-new step the company's override doesn't have at all yet.
			ops = append(ops, PatchOperation{Op: OpAddStep, StepKey: key, Origin: OriginGlobalChanged, NewStep: t})
			continue

		case !hasB && hasT && hasC:
			// No base reference exists for this key, yet both target and company carry it; we
			// cannot establish whether the company's content originated from this global step or
			// is coincidentally keyed the same. Treat conservatively as company-original (no base
			// to diff against) — informational only, not silently merged.
			continue

		case hasB && hasT && !hasC:
			// Company removed a step from their own customization that the base/target still
			// has. Rule 6 in spirit (company-initiated removal) — not flagged as a conflict since
			// it's the company's own deliberate action, not a global-vs-company collision.
			continue
		}

		// hasB && hasT && hasC: full three-way comparison, field by field.
		stepOps, stepConflicts := diffStepFields(key, b, t, c)
		ops = append(ops, stepOps...)
		conflicts = append(conflicts, stepConflicts...)

		if hasB && hasT {
			if reorderOp := diffDisplayOrder(key, b, t); reorderOp != nil {
				ops = append(ops, *reorderOp)
			}
		}
	}

	sortPatchOperations(ops)
	sortConflicts(conflicts)
	return ops, conflicts
}

func unclearIdentityConflict(side string, step *WorkflowStepDTO) PreviewConflict {
	stepID := ""
	if step != nil {
		stepID = step.StepID
	}
	return PreviewConflict{
		TemporaryConflictID: "preview-unclear-" + side + "-" + stepID,
		StepKey:             stepID,
		FieldPath:           "__step_existence__",
		Severity:            ConflictSeverityBlocking,
		ResolutionOptions:   []string{ResolutionMergeManual},
		Reason:              ConflictReasonStepIdentityUnclear,
	}
}

// diffStepFields compares every field PATCH_MODEL_OPTIONS.md §4 lists, EXCEPT display_order
// (handled separately by diffDisplayOrder, since reorder is a distinct op type, not a
// update_step_field entry).
func diffStepFields(key string, base, target, company *WorkflowStepDTO) ([]PatchOperation, []PreviewConflict) {
	var ops []PatchOperation
	var conflicts []PreviewConflict

	// stage
	if op, conf := diffGenericField(key, "stage", base.Stage, target.Stage, company.Stage); op != nil {
		ops = append(ops, *op)
	} else if conf != nil {
		conflicts = append(conflicts, *conf)
	}

	// department_id
	if op, conf := diffGenericField(key, "department_id", base.DepartmentID, target.DepartmentID, company.DepartmentID); op != nil {
		ops = append(ops, *op)
	} else if conf != nil {
		conflicts = append(conflicts, *conf)
	}

	// documents (compared as a normalized, order-preserving list — nil and [] are equivalent)
	if op, conf := diffGenericField(key, "documents", normalizeDocuments(base.Documents), normalizeDocuments(target.Documents), normalizeDocuments(company.Documents)); op != nil {
		ops = append(ops, *op)
	} else if conf != nil {
		conflicts = append(conflicts, *conf)
	}

	// groups (same normalization approach)
	if op, conf := diffGenericField(key, "groups", normalizeGroups(base.Groups), normalizeGroups(target.Groups), normalizeGroups(company.Groups)); op != nil {
		ops = append(ops, *op)
	} else if conf != nil {
		conflicts = append(conflicts, *conf)
	}

	// assignee_role_ids — specialized op, compared as a SET (PATCH_MODEL_OPTIONS.md §3)
	if op, conf := diffAssignees(key, base.AssigneeRoleIds, target.AssigneeRoleIds, company.AssigneeRoleIds); op != nil {
		ops = append(ops, *op)
	} else if conf != nil {
		conflicts = append(conflicts, *conf)
	}

	// due_rule — specialized op, compared by canonical string value
	if op, conf := diffDueRule(key, base.DueRule, target.DueRule, company.DueRule); op != nil {
		ops = append(ops, *op)
	} else if conf != nil {
		conflicts = append(conflicts, *conf)
	}

	// reminder_config — specialized op, compared by normalized deep equality
	if op, conf := diffReminders(key, canonicalizeReminderConfigForDiff(base.ReminderConfig), canonicalizeReminderConfigForDiff(target.ReminderConfig), canonicalizeReminderConfigForDiff(company.ReminderConfig)); op != nil {
		ops = append(ops, *op)
	} else if conf != nil {
		conflicts = append(conflicts, *conf)
	}

	return ops, conflicts
}

// diffGenericField implements the three-way classification shared by every plain-value field
// (stage, department_id, documents, groups). Returns at most one of (op, conflict) — never both.
func diffGenericField(key, fieldPath string, base, target, company any) (*PatchOperation, *PreviewConflict) {
	companyChanged := !deepEqual(company, base)
	globalChanged := !deepEqual(target, base)

	switch {
	case !companyChanged && !globalChanged:
		return nil, nil
	case companyChanged && !globalChanged:
		// Company customized this field; global never touched it. Nothing to rebase.
		return nil, nil
	case !companyChanged && globalChanged:
		return &PatchOperation{
			Op: OpUpdateStepField, StepKey: key, FieldPath: fieldPath,
			From: base, To: target, Origin: OriginGlobalChanged,
		}, nil
	default: // both changed
		if deepEqual(target, company) {
			// Coincidentally converged on the same value — no real divergence.
			return &PatchOperation{
				Op: OpUpdateStepField, StepKey: key, FieldPath: fieldPath,
				From: base, To: target, Origin: OriginBothChanged,
			}, nil
		}
		return nil, &PreviewConflict{
			TemporaryConflictID: "preview-" + key + "-" + fieldPath,
			StepKey:             key,
			FieldPath:           fieldPath,
			Severity:            ConflictSeverityAdvisory,
			GlobalOld:           base,
			GlobalNew:           target,
			CompanyValue:        company,
			ResolutionOptions:   []string{ResolutionKeepCompany, ResolutionAcceptGlobal, ResolutionMergeManual},
		}
	}
}

func diffAssignees(key string, base, target, company []string) (*PatchOperation, *PreviewConflict) {
	baseSet := sortedUniqueStrings(base)
	targetSet := sortedUniqueStrings(target)
	companySet := sortedUniqueStrings(company)

	companyChanged := !reflect.DeepEqual(companySet, baseSet)
	globalChanged := !reflect.DeepEqual(targetSet, baseSet)

	switch {
	case !companyChanged && !globalChanged:
		return nil, nil
	case companyChanged && !globalChanged:
		return nil, nil
	case !companyChanged && globalChanged:
		return &PatchOperation{
			Op: OpReplaceAssignees, StepKey: key, Origin: OriginGlobalChanged,
			OldAssigneeRoleIds: baseSet, NewAssigneeRoleIds: targetSet,
		}, nil
	default:
		if reflect.DeepEqual(targetSet, companySet) {
			return &PatchOperation{
				Op: OpReplaceAssignees, StepKey: key, Origin: OriginBothChanged,
				OldAssigneeRoleIds: baseSet, NewAssigneeRoleIds: targetSet,
			}, nil
		}
		return nil, &PreviewConflict{
			TemporaryConflictID: "preview-" + key + "-assignee_role_ids",
			StepKey:             key,
			FieldPath:           "assignee_role_ids",
			Severity:            ConflictSeverityAdvisory,
			GlobalOld:           baseSet,
			GlobalNew:           targetSet,
			CompanyValue:        companySet,
			ResolutionOptions:   []string{ResolutionKeepCompany, ResolutionAcceptGlobal, ResolutionMergeManual},
		}
	}
}

func diffDueRule(key, base, target, company string) (*PatchOperation, *PreviewConflict) {
	companyChanged := company != base
	globalChanged := target != base
	switch {
	case !companyChanged && !globalChanged:
		return nil, nil
	case companyChanged && !globalChanged:
		return nil, nil
	case !companyChanged && globalChanged:
		return &PatchOperation{Op: OpReplaceDueRule, StepKey: key, Origin: OriginGlobalChanged, OldDueRule: base, NewDueRule: target}, nil
	default:
		if target == company {
			return &PatchOperation{Op: OpReplaceDueRule, StepKey: key, Origin: OriginBothChanged, OldDueRule: base, NewDueRule: target}, nil
		}
		return nil, &PreviewConflict{
			TemporaryConflictID: "preview-" + key + "-due_rule",
			StepKey:             key, FieldPath: "due_rule", Severity: ConflictSeverityAdvisory,
			GlobalOld: base, GlobalNew: target, CompanyValue: company,
			ResolutionOptions: []string{ResolutionKeepCompany, ResolutionAcceptGlobal, ResolutionMergeManual},
		}
	}
}

func diffReminders(key string, base, target, company *WorkflowStepReminderConfig) (*PatchOperation, *PreviewConflict) {
	companyChanged := !reminderConfigsEqual(company, base)
	globalChanged := !reminderConfigsEqual(target, base)
	switch {
	case !companyChanged && !globalChanged:
		return nil, nil
	case companyChanged && !globalChanged:
		return nil, nil
	case !companyChanged && globalChanged:
		return &PatchOperation{Op: OpReplaceReminders, StepKey: key, Origin: OriginGlobalChanged, OldReminderConfig: base, NewReminderConfig: target}, nil
	default:
		if reminderConfigsEqual(target, company) {
			return &PatchOperation{Op: OpReplaceReminders, StepKey: key, Origin: OriginBothChanged, OldReminderConfig: base, NewReminderConfig: target}, nil
		}
		return nil, &PreviewConflict{
			TemporaryConflictID: "preview-" + key + "-reminder_config",
			StepKey:             key, FieldPath: "reminder_config", Severity: ConflictSeverityAdvisory,
			GlobalOld: base, GlobalNew: target, CompanyValue: company,
			ResolutionOptions: []string{ResolutionKeepCompany, ResolutionAcceptGlobal, ResolutionMergeManual},
		}
	}
}

// diffDisplayOrder is rule 4 (CONFLICT_DETECTION_MODEL.md): order changes are never a conflict on
// their own (display_order has no semantic overlap with any other field) — purely informational,
// origin classified the same three-way way as every other field but always emitted as an
// operation, never a conflict.
func diffDisplayOrder(key string, base, target *WorkflowStepDTO) *PatchOperation {
	if base.DisplayOrder == target.DisplayOrder {
		return nil
	}
	oldOrder := base.DisplayOrder
	newOrder := target.DisplayOrder
	return &PatchOperation{
		Op: OpReorderStep, StepKey: key, Origin: OriginGlobalChanged,
		OldDisplayOrder: &oldOrder, NewDisplayOrder: &newOrder,
	}
}

func deepEqual(a, b any) bool {
	return reflect.DeepEqual(a, b)
}

func reminderConfigsEqual(a, b *WorkflowStepReminderConfig) bool {
	na := canonicalizeReminderConfigForDiff(a)
	nb := canonicalizeReminderConfigForDiff(b)
	return reflect.DeepEqual(na, nb)
}

// canonicalizeReminderConfigForDiff collapses nil into the canonical "no reminder" value so a step that has
// never had reminder_config set doesn't falsely register as "changed" against one explicitly set
// to disabled with no days (Hash Contract §5's nil-vs-empty principle, applied here too).
func canonicalizeReminderConfigForDiff(c *WorkflowStepReminderConfig) *WorkflowStepReminderConfig {
	if c == nil {
		return &WorkflowStepReminderConfig{Enabled: false, Mode: "", DaysBefore: []int{}}
	}
	out := *c
	if out.DaysBefore == nil {
		out.DaysBefore = []int{}
	}
	return &out
}

func normalizeDocuments(docs []WorkflowDocumentDTO) []WorkflowDocumentDTO {
	if docs == nil {
		return []WorkflowDocumentDTO{}
	}
	return docs
}

func normalizeGroups(groups []WorkflowStepGroupDTO) []WorkflowStepGroupDTO {
	if groups == nil {
		return []WorkflowStepGroupDTO{}
	}
	return groups
}

func sortedUniqueStrings(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

func sortPatchOperations(ops []PatchOperation) {
	sort.SliceStable(ops, func(i, j int) bool {
		if ops[i].StepKey != ops[j].StepKey {
			return ops[i].StepKey < ops[j].StepKey
		}
		return ops[i].Op < ops[j].Op
	})
}

func sortConflicts(conflicts []PreviewConflict) {
	sort.SliceStable(conflicts, func(i, j int) bool {
		if conflicts[i].StepKey != conflicts[j].StepKey {
			return conflicts[i].StepKey < conflicts[j].StepKey
		}
		return conflicts[i].FieldPath < conflicts[j].FieldPath
	})
}
