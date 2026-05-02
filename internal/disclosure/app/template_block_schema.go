package app

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Block-type-specific config/validation guards (mirror cobo_web_design templateBlockSchema.ts).
const (
	BlockTypeText       = "text"
	BlockTypeTextarea   = "textarea"
	BlockTypeRichText   = "rich_text"
	BlockTypeTable      = "table"
	BlockTypeChecklist  = "checklist"
	BlockTypeNumber     = "number"
	BlockTypeDate       = "date"
	BlockTypeAttachment = "attachment"
	BlockTypeSection    = "section"
)

// Hard allowlist: canonical types only (after alias resolution). Aliases: textarea, long_text -> text.
var templateBlockTypesAllowed = map[string]struct{}{
	BlockTypeText:       {},
	BlockTypeRichText:   {},
	BlockTypeTable:      {},
	BlockTypeChecklist:  {},
	BlockTypeNumber:     {},
	BlockTypeDate:       {},
	BlockTypeAttachment: {},
	BlockTypeSection:    {},
}

const templateBlockTypesAllowedHint = "attachment, checklist, date, number, rich_text, section, table, text; aliases: textarea|long_text -> text"

func canonicalBlockType(blockType string) string {
	bt := strings.ToLower(strings.TrimSpace(blockType))
	switch bt {
	case BlockTypeTextarea, "long_text":
		return BlockTypeText
	default:
		return bt
	}
}

func validateBlockTypeSchema(prefix, blockType string, config map[string]any, validation map[string]any, fieldErrors map[string]string) {
	if strings.TrimSpace(blockType) == "" {
		return
	}
	canonical := canonicalBlockType(blockType)
	if _, ok := templateBlockTypesAllowed[canonical]; !ok {
		fieldErrors[prefix+".block_type"] = fmt.Sprintf("unsupported block_type %q; allowed: %s", blockType, templateBlockTypesAllowedHint)
		return
	}
	switch canonical {
	case BlockTypeText:
		validateTextBlockSchema(prefix, config, fieldErrors)
	case BlockTypeRichText:
		validateRichTextBlockSchema(prefix, config, fieldErrors)
	case BlockTypeTable:
		validateTableBlockSchema(prefix, config, fieldErrors)
	case BlockTypeChecklist:
		validateChecklistBlockSchema(prefix, config, fieldErrors)
	case BlockTypeNumber:
		validateNumberBlockSchema(prefix, validation, fieldErrors)
	case BlockTypeDate:
		validateDateBlockSchema(prefix, validation, fieldErrors)
	case BlockTypeAttachment:
		validateAttachmentBlockSchema(prefix, config, fieldErrors)
	case BlockTypeSection:
	}
}

func validateTextBlockSchema(prefix string, config map[string]any, fieldErrors map[string]string) {
	if v, ok := config["max_length"]; ok && v != nil {
		if _, ok := positiveIntegerFromAny(v); !ok {
			fieldErrors[prefix+".config.max_length"] = "max_length must be a positive integer"
		}
	}
}

func validateRichTextBlockSchema(prefix string, config map[string]any, fieldErrors map[string]string) {
	validateTextBlockSchema(prefix, config, fieldErrors)
	if raw, ok := config["allow_html"]; ok && raw != nil {
		if _, isBool := raw.(bool); !isBool {
			fieldErrors[prefix+".config.allow_html"] = "allow_html must be boolean"
		}
	}
}

func validateTableBlockSchema(prefix string, config map[string]any, fieldErrors map[string]string) {
	raw, ok := config["columns"]
	if !ok {
		fieldErrors[prefix+".config.columns"] = "table blocks require config.columns as a non-empty array"
		return
	}
	arr, ok := raw.([]any)
	if !ok || len(arr) == 0 {
		fieldErrors[prefix+".config.columns"] = "table blocks require config.columns as a non-empty array"
		return
	}
	for idx, item := range arr {
		if !tableColumnItemValid(item) {
			fieldErrors[fmt.Sprintf("%s.config.columns[%d]", prefix, idx)] = "each column needs a non-empty key, id, or field"
		}
	}
	var minV, maxV int64
	var minOK, maxOK bool
	if v, ok := config["min_rows"]; ok && v != nil {
		n, ok := nonNegativeIntegerFromAny(v)
		if !ok {
			fieldErrors[prefix+".config.min_rows"] = "min_rows must be a non-negative integer"
		} else {
			minV, minOK = n, true
		}
	}
	if v, ok := config["max_rows"]; ok && v != nil {
		n, ok := positiveIntegerFromAny(v)
		if !ok {
			fieldErrors[prefix+".config.max_rows"] = "max_rows must be a positive integer"
		} else {
			maxV, maxOK = n, true
		}
	}
	if minOK && maxOK && minV > maxV {
		fieldErrors[prefix+".config.min_rows"] = "min_rows must be <= max_rows"
	}
}

func tableColumnItemValid(item any) bool {
	tm, ok := item.(map[string]any)
	if !ok {
		return false
	}
	for _, k := range []string{"key", "id", "field"} {
		if s, ok := tm[k].(string); ok && strings.TrimSpace(s) != "" {
			return true
		}
	}
	return false
}

func validateChecklistBlockSchema(prefix string, config map[string]any, fieldErrors map[string]string) {
	raw, ok := config["options"]
	if !ok {
		fieldErrors[prefix+".config.options"] = "checklist blocks require config.options as a non-empty array"
		return
	}
	arr, ok := raw.([]any)
	if !ok || len(arr) == 0 {
		fieldErrors[prefix+".config.options"] = "checklist blocks require config.options as a non-empty array"
		return
	}
	for idx, item := range arr {
		if !checklistOptionItemHasLabel(item) {
			fieldErrors[fmt.Sprintf("%s.config.options[%d]", prefix, idx)] = "each checklist option needs a label, id, or non-empty text"
		}
	}
}

func checklistOptionItemHasLabel(item any) bool {
	switch v := item.(type) {
	case string:
		return strings.TrimSpace(v) != ""
	case map[string]any:
		if s, ok := v["label"].(string); ok && strings.TrimSpace(s) != "" {
			return true
		}
		if s, ok := v["title"].(string); ok && strings.TrimSpace(s) != "" {
			return true
		}
		if s, ok := v["id"].(string); ok && strings.TrimSpace(s) != "" {
			return true
		}
		return false
	default:
		return false
	}
}

func validateNumberBlockSchema(prefix string, validation map[string]any, fieldErrors map[string]string) {
	for _, key := range []string{"min", "max", "step"} {
		raw, ok := validation[key]
		if !ok || raw == nil {
			continue
		}
		if _, ok := numberFromAny(raw); !ok {
			fieldErrors[prefix+".validation."+key] = key + " must be numeric"
		}
	}
}

func validateDateBlockSchema(prefix string, validation map[string]any, fieldErrors map[string]string) {
	if raw, ok := validation["max_future_days"]; ok && raw != nil {
		if _, ok := positiveIntegerFromAny(raw); !ok {
			fieldErrors[prefix+".validation.max_future_days"] = "max_future_days must be a positive integer"
		}
	}
}

func validateAttachmentBlockSchema(prefix string, config map[string]any, fieldErrors map[string]string) {
	if raw, ok := config["max_files"]; ok && raw != nil {
		if _, ok := positiveIntegerFromAny(raw); !ok {
			fieldErrors[prefix+".config.max_files"] = "max_files must be a positive integer"
		}
	}
}

func nonNegativeIntegerFromAny(v any) (int64, bool) {
	n, ok := numberFromAny(v)
	if !ok || math.Trunc(n) != n || n < 0 {
		return 0, false
	}
	return int64(n), true
}

func positiveIntegerFromAny(v any) (int64, bool) {
	n, ok := numberFromAny(v)
	if !ok || n <= 0 || math.Trunc(n) != n {
		return 0, false
	}
	return int64(n), true
}

func numberFromAny(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		return f, err == nil
	default:
		return 0, false
	}
}
