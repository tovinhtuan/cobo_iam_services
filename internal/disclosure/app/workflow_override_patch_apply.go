package app

import (
	"fmt"
	"sort"
	"strings"
)

// Sprint 3 / Batch 5 — Patch Application Engine.
//
// ApplyPatchOperations is the pure, deterministic counterpart to ComputeRebaseDiff (Batch 3):
// given the override's CURRENT snapshot, the diff engine's computed operations, and each
// conflict's resolution (if any), produces a NEW full snapshot. No DB, no I/O. Field preservation
// is structural: every operation mutates a COPY of the matching step in place, touching ONLY the
// field(s) the operation names — every other field on every step (touched or not) survives by
// construction, never by an explicit per-field copy list that could silently omit one.

// ConflictResolutionLookup resolves what to do for a given (step_key, field_path) pair that has
// an associated conflict. ResolutionValue is the raw decoded resolution_json (only meaningful
// for merge_manual).
type ConflictResolutionLookup struct {
	Resolution      string
	ResolutionValue any
}

// ApplyPatchOperations applies ops to currentSnapshot, honoring any conflict resolution recorded
// for the (step_key, field_path) pair an operation targets. Returns the new, complete snapshot.
// keyOf must return the same bridged identity ComputeRebaseDiff/buildDiffStepInputs already use,
// so a step in currentSnapshot can be matched against an operation's StepKey.
func ApplyPatchOperations(currentSnapshot []WorkflowStepDTO, ops []PatchOperation, resolutions map[string]ConflictResolutionLookup, keyOf func(WorkflowStepDTO) string) ([]WorkflowStepDTO, error) {
	// Work on a deep-enough copy: every step value is copied before any mutation, and slice/ptr
	// fields are never aliased back into the original input — currentSnapshot itself is never
	// mutated (callers may still need it, e.g. for audit comparison).
	byKey := map[string]*WorkflowStepDTO{}
	order := []string{}
	for _, step := range currentSnapshot {
		cp := step
		key := keyOf(cp)
		byKey[key] = &cp
		order = append(order, key)
	}

	for _, op := range ops {
		resolutionKey := op.StepKey + "|" + opFieldPath(op)
		res, hasRes := resolutions[resolutionKey]

		switch op.Op {
		case OpAddStep:
			if hasRes {
				// A resolved conflict on a brand-new step's role (rule 5) — keep_company has no
				// meaning here (there is no company value yet); accept_global means add it as
				// designed; anything else is a deliberate, explicit admin override via
				// merge_manual, applied as a raw replacement of the step about to be added.
				if res.Resolution == ResolutionMergeManual {
					if v, ok := decodeStepFromAny(res.ResolutionValue); ok {
						byKey[op.StepKey] = &v
						order = append(order, op.StepKey)
						continue
					}
				}
				if res.Resolution == ResolutionKeepCompany {
					// Explicit instruction not to add this step at all.
					continue
				}
			}
			if op.NewStep == nil {
				return nil, fmt.Errorf("add_step operation for %q has no NewStep payload", op.StepKey)
			}
			cp := *op.NewStep
			byKey[op.StepKey] = &cp
			if _, existed := byKey[op.StepKey]; !existed {
				order = append(order, op.StepKey)
			} else if !containsString(order, op.StepKey) {
				order = append(order, op.StepKey)
			}

		case OpRemoveStep:
			if hasRes && res.Resolution == ResolutionKeepCompany {
				// Company explicitly chose to keep this step despite the global side wanting it
				// gone — leave it untouched.
				continue
			}
			if hasRes && res.Resolution == ResolutionCreateNewStep {
				// Rule 2: preserve the company's customized content as a new, company-only step
				// (give it a fresh identity so it never collides with the now-removed global key
				// on a future rebase).
				if existing, ok := byKey[op.StepKey]; ok {
					newKey := op.StepKey + "-preserved"
					preserved := *existing
					preserved.StepID = newKey
					byKey[newKey] = &preserved
					order = append(order, newKey)
				}
				delete(byKey, op.StepKey)
				order = removeString(order, op.StepKey)
				continue
			}
			// Clean removal (no conflict, or resolution explicitly accepts the removal /
			// mark_not_applicable) — a step the company never customized away from base, or the
			// company already doesn't have it (no-op when absent).
			delete(byKey, op.StepKey)
			order = removeString(order, op.StepKey)

		case OpUpdateStepField:
			step, ok := byKey[op.StepKey]
			if !ok {
				continue // nothing to update — step isn't in the snapshot (shouldn't happen for a clean op, defensive)
			}
			if hasRes {
				if res.Resolution == ResolutionKeepCompany {
					continue // leave the field exactly as the company has it
				}
				if res.Resolution == ResolutionMergeManual {
					if err := applyFieldValue(step, op.FieldPath, res.ResolutionValue); err != nil {
						return nil, err
					}
					continue
				}
				// accept_global (or any other resolution) falls through to the normal apply below.
			}
			if err := applyFieldValue(step, op.FieldPath, op.To); err != nil {
				return nil, err
			}

		case OpReorderStep:
			step, ok := byKey[op.StepKey]
			if !ok {
				continue
			}
			if op.NewDisplayOrder != nil {
				step.DisplayOrder = *op.NewDisplayOrder
			}

		case OpReplaceAssignees:
			step, ok := byKey[op.StepKey]
			if !ok {
				continue
			}
			if hasRes && res.Resolution == ResolutionKeepCompany {
				continue
			}
			if hasRes && res.Resolution == ResolutionMergeManual {
				if roles, ok := decodeStringSlice(res.ResolutionValue); ok {
					step.AssigneeRoleIds = roles
					continue
				}
			}
			step.AssigneeRoleIds = op.NewAssigneeRoleIds

		case OpReplaceDueRule:
			step, ok := byKey[op.StepKey]
			if !ok {
				continue
			}
			if hasRes && res.Resolution == ResolutionKeepCompany {
				continue
			}
			if hasRes && res.Resolution == ResolutionMergeManual {
				if s, ok := res.ResolutionValue.(string); ok && s != "" {
					step.DueRule = s
					continue
				}
			}
			step.DueRule = op.NewDueRule

		case OpReplaceReminders:
			step, ok := byKey[op.StepKey]
			if !ok {
				continue
			}
			if hasRes && res.Resolution == ResolutionKeepCompany {
				continue
			}
			step.ReminderConfig = op.NewReminderConfig

		default:
			return nil, fmt.Errorf("%w: unknown patch operation %q", ErrInvalidPatch, op.Op)
		}
	}

	out := make([]WorkflowStepDTO, 0, len(order))
	seen := map[string]bool{}
	for _, key := range order {
		if seen[key] {
			continue
		}
		seen[key] = true
		if step, ok := byKey[key]; ok {
			out = append(out, *step)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].DisplayOrder < out[j].DisplayOrder })
	return out, nil
}

// ErrInvalidPatch is wrapped by the service layer into a 400 INVALID_PATCH HTTP error.
var ErrInvalidPatch = fmt.Errorf("invalid patch operation")

// opFieldPath returns the field path a conflict would have been keyed under for this operation —
// mirrors exactly how workflow_override_diff.go's conflict emission sites set FieldPath for each
// operation family.
func opFieldPath(op PatchOperation) string {
	switch op.Op {
	case OpReplaceAssignees:
		return "assignee_role_ids"
	case OpReplaceDueRule:
		return "due_rule"
	case OpReplaceReminders:
		return "reminder_config"
	case OpReorderStep:
		return "display_order"
	case OpAddStep, OpRemoveStep:
		return "__step_existence__"
	default:
		return op.FieldPath
	}
}

func applyFieldValue(step *WorkflowStepDTO, fieldPath string, value any) error {
	switch fieldPath {
	case "stage":
		if s, ok := value.(string); ok {
			step.Stage = s
			return nil
		}
	case "department_id":
		if s, ok := value.(string); ok {
			step.DepartmentID = s
			return nil
		}
	case "documents":
		if docs, ok := decodeDocuments(value); ok {
			step.Documents = docs
			return nil
		}
	case "groups":
		if groups, ok := decodeGroups(value); ok {
			step.Groups = groups
			return nil
		}
	}
	return fmt.Errorf("%w: cannot apply field %q with value %v", ErrInvalidPatch, fieldPath, value)
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func removeString(list []string, s string) []string {
	out := make([]string, 0, len(list))
	for _, v := range list {
		if v != s {
			out = append(out, v)
		}
	}
	return out
}

func decodeStringSlice(v any) ([]string, bool) {
	if v == nil {
		return nil, false
	}
	raw, ok := v.([]any)
	if !ok {
		if ss, ok := v.([]string); ok {
			return ss, true
		}
		return nil, false
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out, true
}

func decodeDocuments(v any) ([]WorkflowDocumentDTO, bool) {
	raw, ok := v.([]any)
	if !ok {
		return nil, false
	}
	out := make([]WorkflowDocumentDTO, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		var d WorkflowDocumentDTO
		if s, ok := m["doc_id"].(string); ok {
			d.DocID = s
		}
		if s, ok := m["name"].(string); ok {
			d.Name = s
		}
		if b, ok := m["required"].(bool); ok {
			d.Required = b
		}
		if s, ok := m["template_file_id"].(string); ok {
			d.TemplateFileID = strings.TrimSpace(s)
		}
		if s, ok := m["template_file_name"].(string); ok {
			d.TemplateFileName = strings.TrimSpace(s)
		}
		out = append(out, d)
	}
	return out, true
}

func decodeGroups(v any) ([]WorkflowStepGroupDTO, bool) {
	raw, ok := v.([]any)
	if !ok {
		return nil, false
	}
	out := make([]WorkflowStepGroupDTO, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		var g WorkflowStepGroupDTO
		if s, ok := m["group_id"].(string); ok {
			g.GroupID = s
		}
		if s, ok := m["department_id"].(string); ok {
			g.DepartmentID = s
		}
		out = append(out, g)
	}
	return out, true
}

// fieldConflictResolutionToOp bridges a resolved FIELD-level conflict (Rule 1: "same_field_changed"
// — stage/department_id/documents/groups/assignee_role_ids/due_rule/reminder_config) into a patch
// operation the engine above can execute. This closes a real gap: the diff engine (workflow_
// override_diff.go's diffGenericField/diffDueRule/diffReminders) emits EITHER an op OR a conflict
// for a given field, never both — so a field that was conflicted never has a corresponding op in
// ComputeRebaseDiff's own output. Without this bridge, resolving such a conflict as accept_global
// or merge_manual would have nothing to apply, silently leaving the company's stale value in
// place — exactly the kind of silent-no-op this batch's own absolute rules forbid. Existence
// conflicts (FieldPath == "__step_existence__", Rule 2/6) are NOT bridged here — those are handled
// by the resolution-aware add_step/remove_step branches in ApplyPatchOperations directly, keyed
// off the ORIGINAL op (which always exists for an add/remove case).
func fieldConflictResolutionToOp(stepKey, fieldPath, resolution string, globalNew, resolutionValue any) *PatchOperation {
	if resolution == ResolutionKeepCompany || resolution == "" {
		return nil // company's existing value stands; nothing to apply
	}
	value := globalNew
	if resolution == ResolutionMergeManual {
		value = resolutionValue
	}
	switch fieldPath {
	case "due_rule":
		if s, ok := value.(string); ok {
			return &PatchOperation{Op: OpReplaceDueRule, StepKey: stepKey, FieldPath: fieldPath, NewDueRule: s}
		}
	case "assignee_role_ids":
		if roles, ok := decodeStringSlice(value); ok {
			return &PatchOperation{Op: OpReplaceAssignees, StepKey: stepKey, FieldPath: fieldPath, NewAssigneeRoleIds: roles}
		}
	case "reminder_config":
		if rc, ok := decodeReminderConfig(value); ok {
			return &PatchOperation{Op: OpReplaceReminders, StepKey: stepKey, FieldPath: fieldPath, NewReminderConfig: rc}
		}
	default: // stage, department_id, documents, groups — the generic catch-all
		return &PatchOperation{Op: OpUpdateStepField, StepKey: stepKey, FieldPath: fieldPath, To: value}
	}
	return nil
}

func decodeReminderConfig(v any) (*WorkflowStepReminderConfig, bool) {
	m, ok := v.(map[string]any)
	if !ok {
		return nil, false
	}
	rc := &WorkflowStepReminderConfig{}
	if b, ok := m["enabled"].(bool); ok {
		rc.Enabled = b
	}
	if s, ok := m["mode"].(string); ok {
		rc.Mode = s
	}
	if raw, ok := m["days_before"].([]any); ok {
		for _, item := range raw {
			if f, ok := item.(float64); ok {
				rc.DaysBefore = append(rc.DaysBefore, int(f))
			}
		}
	}
	return rc, true
}

func decodeStepFromAny(v any) (WorkflowStepDTO, bool) {
	m, ok := v.(map[string]any)
	if !ok {
		return WorkflowStepDTO{}, false
	}
	var step WorkflowStepDTO
	if s, ok := m["step_id"].(string); ok {
		step.StepID = s
	}
	if s, ok := m["stage"].(string); ok {
		step.Stage = s
	}
	if s, ok := m["department_id"].(string); ok {
		step.DepartmentID = s
	}
	if roles, ok := decodeStringSlice(m["assignee_role_ids"]); ok {
		step.AssigneeRoleIds = roles
	}
	if s, ok := m["due_rule"].(string); ok {
		step.DueRule = s
	}
	return step, step.StepID != ""
}
