package app

import (
	"fmt"
	"sort"
	"strings"
)

func ExtractTemplateWorkflow(blocks []TemplateBlockDTO) []WorkflowStepDTO {
	for _, block := range blocks {
		if !strings.EqualFold(strings.TrimSpace(block.BlockKey), "enterprise_workflow") {
			continue
		}
		return normalizeTemplateWorkflowConfig(block.Config)
	}
	return []WorkflowStepDTO{}
}

func TemplateHasWorkflow(blocks []TemplateBlockDTO) bool {
	return len(ExtractTemplateWorkflow(blocks)) > 0
}

// ValidateTemplateWorkflowForActivation validates the enterprise_workflow block only.
// Runtime activate/publish readiness uses TEMPLATE_PINNED + ValidateWorkflowStepsForActivation.
func ValidateTemplateWorkflowForActivation(blocks []TemplateBlockDTO) error {
	return ValidateWorkflowStepsForActivation(ExtractTemplateWorkflow(blocks))
}

func normalizeTemplateWorkflowConfig(config map[string]any) []WorkflowStepDTO {
	if len(config) == 0 {
		return []WorkflowStepDTO{}
	}
	rawSteps, ok := config["steps"]
	if !ok {
		rawSteps = config["workflow"]
	}
	items, ok := rawSteps.([]any)
	if !ok {
		return []WorkflowStepDTO{}
	}
	out := make([]WorkflowStepDTO, 0, len(items))
	for index, raw := range items {
		row, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		step := WorkflowStepDTO{
			StepID:            firstNonBlankString(row["step_id"], row["id"]),
			Stage:             strings.TrimSpace(fmt.Sprint(row["stage"])),
			Description:       optionalTrimmedString(row["description"]),
			DescriptionFormat: optionalTrimmedString(row["description_format"]),
			Instructions:      optionalTrimmedString(row["instructions"]),
			DepartmentID:      strings.TrimSpace(fmt.Sprint(row["department_id"])),
			AssigneeRoleIds:   normalizeStringSlice(row["assignee_role_ids"]),
			AssigneeMembershipID: optionalTrimmedString(row["assignee_membership_id"]),
			AssigneeMembershipIDs: normalizeStringSlice(row["assignee_membership_ids"]),
			DueRule:         strings.TrimSpace(fmt.Sprint(row["due_rule"])),
			ProcessingDays:  normalizeInt(row["processing_days"]),
			DisplayOrder:    normalizeInt(row["display_order"]),
			Documents:       normalizeWorkflowDocuments(row["documents"]),
			Groups:          normalizeWorkflowGroups(row["groups"]),
		}
		if step.DescriptionFormat != "" {
			if err := ValidateWorkflowStepDescriptionFormatForPersist(step.DescriptionFormat); err != nil {
				// Corrupt legacy row: safe plain fallback on read (never panic normalize).
				step.DescriptionFormat = ""
			} else {
				norm := NormalizeWorkflowStepDescriptionFormat(step.DescriptionFormat)
				if norm == WorkflowStepDescriptionFormatPlainText {
					step.DescriptionFormat = ""
				} else {
					step.DescriptionFormat = norm
				}
			}
		}
		if step.DisplayOrder <= 0 {
			step.DisplayOrder = index + 1
		}
		if reminder := normalizeReminderConfig(row["reminder_config"]); reminder != nil {
			step.ReminderConfig = reminder
		}
		out = append(out, step)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].DisplayOrder == out[j].DisplayOrder {
			return out[i].StepID < out[j].StepID
		}
		return out[i].DisplayOrder < out[j].DisplayOrder
	})
	return out
}

func normalizeWorkflowDocuments(raw any) []WorkflowDocumentDTO {
	items, ok := raw.([]any)
	if !ok {
		return []WorkflowDocumentDTO{}
	}
	out := make([]WorkflowDocumentDTO, 0, len(items))
	for _, rawItem := range items {
		row, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, WorkflowDocumentDTO{
			DocID:            firstNonBlankString(row["doc_id"], row["code"], row["id"]),
			Name:             strings.TrimSpace(fmt.Sprint(row["name"])),
			Required:         normalizeBool(row["required"]),
			TemplateFileID:   optionalTrimmedString(row["template_file_id"]),
			TemplateFileName: optionalTrimmedString(row["template_file_name"]),
		})
	}
	return out
}

func normalizeWorkflowGroups(raw any) []WorkflowStepGroupDTO {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]WorkflowStepGroupDTO, 0, len(items))
	for index, rawItem := range items {
		row, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		group := WorkflowStepGroupDTO{
			GroupID:      strings.TrimSpace(fmt.Sprint(row["group_id"])),
			Source:       firstNonBlankString(row["source"], "manual"),
			DurationMode: firstNonBlankString(row["duration_mode"], "inherit"),
			DisplayOrder: normalizeInt(row["display_order"]),
			IsActive:     !rowHasExplicitFalse(row["is_active"]),
		}
		if group.DisplayOrder <= 0 {
			group.DisplayOrder = index + 1
		}
		if days := normalizeOptionalInt(row["processing_days"]); days != nil {
			group.ProcessingDays = days
		}
		out = append(out, group)
	}
	if len(out) == 0 {
		return nil
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].DisplayOrder == out[j].DisplayOrder {
			return out[i].GroupID < out[j].GroupID
		}
		return out[i].DisplayOrder < out[j].DisplayOrder
	})
	return out
}

func normalizeReminderConfig(raw any) *WorkflowStepReminderConfig {
	row, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	cfg := &WorkflowStepReminderConfig{
		Enabled: normalizeBool(row["enabled"]),
		Mode:    strings.TrimSpace(fmt.Sprint(row["mode"])),
	}
	for _, rawDay := range normalizeAnySlice(row["days_before"]) {
		if day := normalizeInt(rawDay); day > 0 {
			cfg.DaysBefore = append(cfg.DaysBefore, day)
		}
	}
	if !cfg.Enabled && cfg.Mode == "" && len(cfg.DaysBefore) == 0 {
		return nil
	}
	return cfg
}

func normalizeStringSlice(raw any) []string {
	items := normalizeAnySlice(raw)
	out := make([]string, 0, len(items))
	for _, item := range items {
		value := strings.TrimSpace(fmt.Sprint(item))
		if value != "" && value != "<nil>" {
			out = append(out, value)
		}
	}
	return out
}

func normalizeAnySlice(raw any) []any {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	return items
}

func normalizeInt(raw any) int {
	if v := normalizeOptionalInt(raw); v != nil {
		return *v
	}
	return 0
}

func normalizeOptionalInt(raw any) *int {
	switch value := raw.(type) {
	case int:
		return &value
	case int32:
		v := int(value)
		return &v
	case int64:
		v := int(value)
		return &v
	case float64:
		v := int(value)
		return &v
	case float32:
		v := int(value)
		return &v
	default:
		return nil
	}
}

func normalizeBool(raw any) bool {
	value, ok := raw.(bool)
	return ok && value
}

func rowHasExplicitFalse(raw any) bool {
	value, ok := raw.(bool)
	return ok && !value
}

func firstNonBlankString(values ...any) string {
	for _, raw := range values {
		value := strings.TrimSpace(fmt.Sprint(raw))
		if value != "" && value != "<nil>" {
			return value
		}
	}
	return ""
}

func optionalTrimmedString(raw any) string {
	if raw == nil {
		return ""
	}
	value := strings.TrimSpace(fmt.Sprint(raw))
	if value == "" || value == "<nil>" {
		return ""
	}
	return value
}
