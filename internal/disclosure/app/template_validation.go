package app

import (
	"net/http"
	"strconv"
	"strings"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

const (
	// New model — portal/company-defined template flow (BE-006)
	TemplateCategoryPeriodic  = "periodic"
	TemplateCategoryIrregular = "irregular"

	PeriodicityMonthly   = "monthly"
	PeriodicityQuarterly = "quarterly"
	PeriodicityYearly    = "yearly"
	PeriodicityDaily     = "daily"

	DeadlineStrategyFixedCycleDays = "fixed_cycle_days"
	DeadlineStrategyEventHours     = "event_relative_hours"

	// Legacy model constants — kept for compatibility adapter (BE-008) only.
	// DO NOT use in new portal/company-defined template validation paths.
	legacyTemplateCategoryCustom       = "custom"
	legacyPeriodicityEventBased        = "event_based"
	legacyPeriodicityAdHoc             = "ad_hoc"
	legacyDeadlineStrategyConfigurable = "configurable"

	// Exported legacy aliases — transitional compatibility for catalog.go and service.go.
	// These will be removed when BE-008 compatibility adapter is fully sunset.
	TemplateCategoryCustom    = legacyTemplateCategoryCustom
	PeriodicityEventBased     = legacyPeriodicityEventBased
	PeriodicityAdHoc          = legacyPeriodicityAdHoc
	DeadlineStrategyConfigurable = legacyDeadlineStrategyConfigurable
)

var (
	// portalTemplateCategoryAliases: new model only — no custom.
	portalTemplateCategoryAliases = map[string]string{
		TemplateCategoryPeriodic:  TemplateCategoryPeriodic,
		"định kỳ":                 TemplateCategoryPeriodic,
		"dinh ky":                 TemplateCategoryPeriodic,
		TemplateCategoryIrregular: TemplateCategoryIrregular,
		"bất thường":              TemplateCategoryIrregular,
		"bat thuong":              TemplateCategoryIrregular,
	}
	// legacyTemplateCategoryAliases: compatibility adapter only (BE-008).
	legacyTemplateCategoryAliases = map[string]string{
		TemplateCategoryPeriodic:     TemplateCategoryPeriodic,
		"định kỳ":                    TemplateCategoryPeriodic,
		"dinh ky":                    TemplateCategoryPeriodic,
		TemplateCategoryIrregular:    TemplateCategoryIrregular,
		"bất thường":                 TemplateCategoryIrregular,
		"bat thuong":                 TemplateCategoryIrregular,
		legacyTemplateCategoryCustom: legacyTemplateCategoryCustom,
		"tùy chỉnh":                  legacyTemplateCategoryCustom,
		"tuy chinh":                  legacyTemplateCategoryCustom,
	}
	// portalPeriodicityAliases: new model — daily added, event_based/ad_hoc removed.
	portalPeriodicityAliases = map[string]string{
		PeriodicityMonthly:   PeriodicityMonthly,
		"hàng tháng":         PeriodicityMonthly,
		"hang thang":         PeriodicityMonthly,
		PeriodicityQuarterly: PeriodicityQuarterly,
		"hàng quý":           PeriodicityQuarterly,
		"hang quy":           PeriodicityQuarterly,
		PeriodicityYearly:    PeriodicityYearly,
		"hàng năm":           PeriodicityYearly,
		"hang nam":           PeriodicityYearly,
		PeriodicityDaily:     PeriodicityDaily,
		"hàng ngày":          PeriodicityDaily,
		"hang ngay":          PeriodicityDaily,
	}
	// legacyPeriodicityAliases: compatibility adapter only (BE-008).
	legacyPeriodicityAliases = map[string]string{
		PeriodicityMonthly:       PeriodicityMonthly,
		"hàng tháng":             PeriodicityMonthly,
		"hang thang":             PeriodicityMonthly,
		PeriodicityQuarterly:     PeriodicityQuarterly,
		"hàng quý":               PeriodicityQuarterly,
		"hang quy":               PeriodicityQuarterly,
		PeriodicityYearly:        PeriodicityYearly,
		"hàng năm":               PeriodicityYearly,
		"hang nam":               PeriodicityYearly,
		legacyPeriodicityEventBased: legacyPeriodicityEventBased,
		"bat thuong":             legacyPeriodicityEventBased,
		legacyPeriodicityAdHoc:  legacyPeriodicityAdHoc,
		"theo yêu cầu":           legacyPeriodicityAdHoc,
		"theo yeu cau":           legacyPeriodicityAdHoc,
	}
	portalDeadlineStrategyAliases = map[string]string{
		DeadlineStrategyFixedCycleDays: DeadlineStrategyFixedCycleDays,
		DeadlineStrategyEventHours:     DeadlineStrategyEventHours,
	}
	// legacyDeadlineStrategyAliases: compatibility adapter only (BE-008).
	legacyDeadlineStrategyAliases = map[string]string{
		DeadlineStrategyFixedCycleDays:     DeadlineStrategyFixedCycleDays,
		DeadlineStrategyEventHours:         DeadlineStrategyEventHours,
		legacyDeadlineStrategyConfigurable: legacyDeadlineStrategyConfigurable,
	}
	allowedChannelFileTypes = map[string]struct{}{
		"PDF":  {},
		"DOCS": {},
		"XML":  {},
	}
)

func normalizeAlias(raw string, aliases map[string]string) string {
	key := strings.ToLower(strings.TrimSpace(raw))
	if key == "" {
		return ""
	}
	if v, ok := aliases[key]; ok {
		return v
	}
	return key
}

func newValidationError(fieldErrors map[string]string) error {
	return &perr.HTTPError{
		Code:       perr.CodeInvalidRequest,
		Message:    "template validation failed",
		HTTPStatus: http.StatusBadRequest,
		Details: map[string]any{
			"field_errors": fieldErrors,
		},
	}
}

// validatePortalTemplateMatrix validates template create/update requests for the portal
// company-defined template flow. Accepts only periodic|irregular; rejects custom.
// Use this for BE-004A (company-defined template lifecycle write path).
func validatePortalTemplateMatrix(req *UpsertTypeVersionRequest) error {
	fieldErrors := map[string]string{}

	req.TemplateCategory = normalizeAlias(req.TemplateCategory, portalTemplateCategoryAliases)
	req.Periodicity = normalizeAlias(req.Periodicity, portalPeriodicityAliases)
	req.DeadlineStrategy = normalizeAlias(req.DeadlineStrategy, portalDeadlineStrategyAliases)

	if req.TemplateCategory == "" {
		fieldErrors["template_category"] = "template_category is required"
	}
	if req.DeadlineRule == "" {
		fieldErrors["deadline_rule"] = "deadline_rule is required"
	}

	switch req.TemplateCategory {
	case TemplateCategoryPeriodic:
		if req.Periodicity != PeriodicityMonthly &&
			req.Periodicity != PeriodicityQuarterly &&
			req.Periodicity != PeriodicityYearly &&
			req.Periodicity != PeriodicityDaily {
			fieldErrors["periodicity"] = "periodic templates require periodicity in [monthly, quarterly, yearly, daily]"
		}
	case TemplateCategoryIrregular:
		// irregular: periodicity is not required; deadline_rule validates deadline
	case "":
		// already captured above
	default:
		fieldErrors["template_category"] = "template_category must be one of [periodic, irregular]"
	}

	if len(fieldErrors) > 0 {
		return newValidationError(fieldErrors)
	}
	validateTemplateBlocks(req, fieldErrors)
	validateTemplateBlockDescriptionLengths(req.Blocks, fieldErrors)
	if len(fieldErrors) > 0 {
		return newValidationError(fieldErrors)
	}
	return nil
}

// validateTemplateMatrix is the legacy CMS path validator.
// Kept for compatibility adapter (BE-008) and existing CMS write flows.
// DO NOT use for new portal/company-defined template write paths.
func validateTemplateMatrix(req *UpsertTypeVersionRequest) error {
	fieldErrors := map[string]string{}

	req.TemplateCategory = normalizeAlias(req.TemplateCategory, legacyTemplateCategoryAliases)
	req.Periodicity = normalizeAlias(req.Periodicity, legacyPeriodicityAliases)
	req.DeadlineStrategy = normalizeAlias(req.DeadlineStrategy, legacyDeadlineStrategyAliases)

	if req.TemplateCategory == "" {
		fieldErrors["template_category"] = "template_category is required"
	}
	if req.DeadlineRule == "" {
		fieldErrors["deadline_rule"] = "deadline_rule is required"
	}
	if req.DeadlineStrategy == "" {
		fieldErrors["deadline_strategy"] = "deadline_strategy is required"
	}

	switch req.TemplateCategory {
	case TemplateCategoryPeriodic:
		if req.Periodicity != PeriodicityMonthly && req.Periodicity != PeriodicityQuarterly && req.Periodicity != PeriodicityYearly {
			fieldErrors["periodicity"] = "periodic templates require periodicity in [monthly, quarterly, yearly]"
		}
		if req.DeadlineStrategy != DeadlineStrategyFixedCycleDays {
			fieldErrors["deadline_strategy"] = "periodic templates require deadline_strategy=fixed_cycle_days"
		}
	case TemplateCategoryIrregular:
		if req.Periodicity != legacyPeriodicityEventBased {
			fieldErrors["periodicity"] = "irregular templates require periodicity=event_based"
		}
		if req.DeadlineStrategy != DeadlineStrategyEventHours {
			fieldErrors["deadline_strategy"] = "irregular templates require deadline_strategy=event_relative_hours"
		}
	case legacyTemplateCategoryCustom:
		if req.Periodicity != PeriodicityMonthly &&
			req.Periodicity != PeriodicityQuarterly &&
			req.Periodicity != PeriodicityYearly &&
			req.Periodicity != legacyPeriodicityAdHoc {
			fieldErrors["periodicity"] = "custom templates require periodicity in [monthly, quarterly, yearly, ad_hoc]"
		}
		if req.DeadlineStrategy != legacyDeadlineStrategyConfigurable {
			fieldErrors["deadline_strategy"] = "custom templates require deadline_strategy=configurable"
		}
	default:
		if req.TemplateCategory != "" {
			fieldErrors["template_category"] = "template_category must be one of [periodic, irregular, custom]"
		}
	}

	if len(fieldErrors) > 0 {
		return newValidationError(fieldErrors)
	}
	validateTemplateBlocks(req, fieldErrors)
	validateTemplateBlockDescriptionLengths(req.Blocks, fieldErrors)
	if len(fieldErrors) > 0 {
		return newValidationError(fieldErrors)
	}
	return nil
}

func validateTemplateBlocks(req *UpsertTypeVersionRequest, fieldErrors map[string]string) {
	if len(req.Blocks) == 0 {
		return
	}
	seenKeys := map[string]int{}
	seenOrders := map[int]int{}
	for idx := range req.Blocks {
		block := &req.Blocks[idx]
		block.BlockID = strings.TrimSpace(block.BlockID)
		block.BlockKey = strings.ToLower(strings.TrimSpace(block.BlockKey))
		block.BlockType = strings.ToLower(strings.TrimSpace(block.BlockType))
		block.Title = strings.TrimSpace(block.Title)
		block.Description = strings.TrimSpace(block.Description)
		if block.Config == nil {
			block.Config = map[string]any{}
		}
		if block.Validation == nil {
			block.Validation = map[string]any{}
		}
		prefix := "blocks." + strconv.Itoa(idx)
		if block.BlockKey == "" {
			fieldErrors[prefix+".block_key"] = "block_key is required"
		} else {
			if prev, ok := seenKeys[block.BlockKey]; ok {
				fieldErrors[prefix+".block_key"] = "block_key must be unique"
				fieldErrors["blocks."+strconv.Itoa(prev)+".block_key"] = "block_key must be unique"
			} else {
				seenKeys[block.BlockKey] = idx
			}
		}
		if block.BlockType == "" {
			fieldErrors[prefix+".block_type"] = "block_type is required"
		}
		if block.Title == "" {
			fieldErrors[prefix+".title"] = "title is required"
		}
		if block.DisplayOrder <= 0 {
			fieldErrors[prefix+".display_order"] = "display_order must be > 0"
		} else {
			if prev, ok := seenOrders[block.DisplayOrder]; ok {
				fieldErrors[prefix+".display_order"] = "display_order must be unique"
				fieldErrors["blocks."+strconv.Itoa(prev)+".display_order"] = "display_order must be unique"
			} else {
				seenOrders[block.DisplayOrder] = idx
			}
		}
		validateBlockTypeSchema(prefix, block.BlockType, block.Config, block.Validation, fieldErrors)
		if block.BlockKey == "channels_and_format" {
			validateChannelsAndFormatBusinessRules(prefix, block.Config, fieldErrors)
		}
	}
	if len(req.Blocks) > 0 {
		validateMandatoryTemplateBlockKeys(seenKeys, fieldErrors)
	}
}

func validateChannelsAndFormatBusinessRules(prefix string, config map[string]any, fieldErrors map[string]string) {
	channels, ok := config["channels"].([]any)
	if !ok || len(channels) == 0 {
		fieldErrors[prefix+".config.channels"] = "channels must be a non-empty array"
	} else {
		for idx := range channels {
			row, ok := channels[idx].(map[string]any)
			if !ok {
				fieldErrors[prefix+".config.channels"] = "channels values must be objects"
				return
			}
			name := strings.TrimSpace(asString(row["name"]))
			if name == "" {
				fieldErrors[prefix+".config.channels."+strconv.Itoa(idx)+".name"] = "channel name is required"
			}
			rowFileTypes, ok := row["file_types"].([]any)
			if !ok || len(rowFileTypes) == 0 {
				fieldErrors[prefix+".config.channels."+strconv.Itoa(idx)+".file_types"] = "channel file_types must be a non-empty array"
				continue
			}
			for i := range rowFileTypes {
				ft, ok := rowFileTypes[i].(string)
				if !ok {
					fieldErrors[prefix+".config.channels."+strconv.Itoa(idx)+".file_types"] = "channel file_types values must be strings in [PDF, DOCS, XML]"
					break
				}
				if _, allowed := allowedChannelFileTypes[strings.ToUpper(strings.TrimSpace(ft))]; !allowed {
					fieldErrors[prefix+".config.channels."+strconv.Itoa(idx)+".file_types"] = "channel file_types values must be strings in [PDF, DOCS, XML]"
					break
				}
			}
		}
	}
	rawFileTypes, ok := config["file_types"].([]any)
	if !ok || len(rawFileTypes) == 0 {
		fieldErrors[prefix+".config.file_types"] = "file_types must be a non-empty array"
		return
	}
	for i := range rawFileTypes {
		ft, ok := rawFileTypes[i].(string)
		if !ok {
			fieldErrors[prefix+".config.file_types"] = "file_types values must be strings in [PDF, DOCS, XML]"
			return
		}
		if _, allowed := allowedChannelFileTypes[strings.ToUpper(strings.TrimSpace(ft))]; !allowed {
			fieldErrors[prefix+".config.file_types"] = "file_types values must be strings in [PDF, DOCS, XML]"
			return
		}
	}
}

func asString(value any) string {
	if value == nil {
		return ""
	}
	s, ok := value.(string)
	if !ok {
		return ""
	}
	return s
}

func validateMandatoryTemplateBlockKeys(seenKeys map[string]int, fieldErrors map[string]string) {
	var missing []string
	for _, mk := range MandatoryTemplateBlockKeys {
		if _, ok := seenKeys[mk]; !ok {
			missing = append(missing, mk)
		}
	}
	if len(missing) == 0 {
		return
	}
	for _, mk := range missing {
		fieldErrors["blocks.missing_"+mk] = "required mandatory block_key; additional non-mandatory blocks are allowed"
	}
}
