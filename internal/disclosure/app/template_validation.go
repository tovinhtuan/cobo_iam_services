package app

import (
	"net/http"
	"strconv"
	"strings"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

const (
	TemplateCategoryPeriodic  = "periodic"
	TemplateCategoryIrregular = "irregular"
	TemplateCategoryCustom    = "custom"

	PeriodicityMonthly    = "monthly"
	PeriodicityQuarterly  = "quarterly"
	PeriodicityYearly     = "yearly"
	PeriodicityEventBased = "event_based"
	PeriodicityAdHoc      = "ad_hoc"

	DeadlineStrategyFixedCycleDays = "fixed_cycle_days"
	DeadlineStrategyEventHours     = "event_relative_hours"
	DeadlineStrategyConfigurable   = "configurable"
)

var (
	templateCategoryAliases = map[string]string{
		TemplateCategoryPeriodic:  TemplateCategoryPeriodic,
		"định kỳ":                 TemplateCategoryPeriodic,
		"dinh ky":                 TemplateCategoryPeriodic,
		TemplateCategoryIrregular: TemplateCategoryIrregular,
		"bất thường":              TemplateCategoryIrregular,
		"bat thuong":              TemplateCategoryIrregular,
		TemplateCategoryCustom:    TemplateCategoryCustom,
		"tùy chỉnh":               TemplateCategoryCustom,
		"tuy chinh":               TemplateCategoryCustom,
	}
	periodicityAliases = map[string]string{
		PeriodicityMonthly:    PeriodicityMonthly,
		"hàng tháng":          PeriodicityMonthly,
		"hang thang":          PeriodicityMonthly,
		PeriodicityQuarterly:  PeriodicityQuarterly,
		"hàng quý":            PeriodicityQuarterly,
		"hang quy":            PeriodicityQuarterly,
		PeriodicityYearly:     PeriodicityYearly,
		"hàng năm":            PeriodicityYearly,
		"hang nam":            PeriodicityYearly,
		PeriodicityEventBased: PeriodicityEventBased,
		"bất thường":          PeriodicityEventBased,
		"bat thuong":          PeriodicityEventBased,
		PeriodicityAdHoc:      PeriodicityAdHoc,
		"theo yêu cầu":        PeriodicityAdHoc,
		"theo yeu cau":        PeriodicityAdHoc,
	}
	deadlineStrategyAliases = map[string]string{
		DeadlineStrategyFixedCycleDays: DeadlineStrategyFixedCycleDays,
		DeadlineStrategyEventHours:     DeadlineStrategyEventHours,
		DeadlineStrategyConfigurable:   DeadlineStrategyConfigurable,
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

func validateTemplateMatrix(req *UpsertTypeVersionRequest) error {
	fieldErrors := map[string]string{}

	req.TemplateCategory = normalizeAlias(req.TemplateCategory, templateCategoryAliases)
	req.Periodicity = normalizeAlias(req.Periodicity, periodicityAliases)
	req.DeadlineStrategy = normalizeAlias(req.DeadlineStrategy, deadlineStrategyAliases)

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
		if req.Periodicity != PeriodicityEventBased {
			fieldErrors["periodicity"] = "irregular templates require periodicity=event_based"
		}
		if req.DeadlineStrategy != DeadlineStrategyEventHours {
			fieldErrors["deadline_strategy"] = "irregular templates require deadline_strategy=event_relative_hours"
		}
	case TemplateCategoryCustom:
		if req.Periodicity != PeriodicityMonthly &&
			req.Periodicity != PeriodicityQuarterly &&
			req.Periodicity != PeriodicityYearly &&
			req.Periodicity != PeriodicityAdHoc {
			fieldErrors["periodicity"] = "custom templates require periodicity in [monthly, quarterly, yearly, ad_hoc]"
		}
		if req.DeadlineStrategy != DeadlineStrategyConfigurable {
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
	}
}
