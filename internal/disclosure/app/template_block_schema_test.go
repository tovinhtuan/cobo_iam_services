package app

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func blockFieldPrefix(blockIndexZeroBased int) string {
	return "blocks." + strconv.Itoa(blockIndexZeroBased)
}

func validSixMandatoryTemplateBlocksForTests() []TemplateBlockDTO {
	return []TemplateBlockDTO{
		{
			BlockID: "tid-m1", BlockKey: "legal_basis", BlockType: "rich_text", Title: "LB",
			Config: map[string]any{"max_length": 8000, "allow_html": false}, Validation: map[string]any{},
			DisplayOrder: 1, Enabled: true,
		},
		{
			BlockID: "tid-m2", BlockKey: "disclosure_content", BlockType: "rich_text", Title: "DC",
			Config: map[string]any{"max_length": 10000, "allow_html": true}, Validation: map[string]any{},
			DisplayOrder: 2, Enabled: true,
		},
		{
			BlockID: "tid-m3", BlockKey: "deadline", BlockType: "text", Title: "DL",
			Config: map[string]any{"max_length": 4000}, Validation: map[string]any{},
			DisplayOrder: 3, Enabled: true,
		},
		{
			BlockID: "tid-m4", BlockKey: "channels_and_format", BlockType: "rich_text", Title: "CF",
			Config: map[string]any{"max_length": 12000, "allow_html": false}, Validation: map[string]any{},
			DisplayOrder: 4, Enabled: true,
		},
		{
			BlockID: "tid-m5", BlockKey: "legal_risks", BlockType: "rich_text", Title: "LR",
			Config: map[string]any{"max_length": 8000, "allow_html": false}, Validation: map[string]any{},
			DisplayOrder: 5, Enabled: true,
		},
		{
			BlockID: "tid-m6", BlockKey: "enterprise_workflow", BlockType: "rich_text", Title: "EW",
			Config: map[string]any{"max_length": 12000, "allow_html": true}, Validation: map[string]any{},
			DisplayOrder: 6, Enabled: true,
		},
	}
}

func Test_validateTemplateMatrix_checklistRequiresOptionsArray(t *testing.T) {
	t.Parallel()
	blocks := append(validSixMandatoryTemplateBlocksForTests(), TemplateBlockDTO{
		BlockID:      "b7",
		BlockKey:     "docs",
		BlockType:    "checklist",
		Title:        "Docs",
		Config:       map[string]any{"allow_custom_items": false},
		Validation:   map[string]any{},
		DisplayOrder: 7,
		Enabled:      true,
	})
	req := &UpsertTypeVersionRequest{
		TypeID:           "dt-schema-test",
		GroupID:          "group-006",
		Name:             "Schema checklist",
		TemplateCategory: TemplateCategoryCustom,
		DeadlineStrategy: DeadlineStrategyConfigurable,
		DeadlineRule:     "T+5",
		Periodicity:      PeriodicityMonthly,
		Blocks:           blocks,
	}
	err := validateTemplateMatrix(req)
	if err == nil {
		t.Fatal("expected validation error")
	}
	var he *perr.HTTPError
	if !errors.As(err, &he) || he.Details == nil {
		t.Fatalf("expected HTTPError with details, got %v", err)
	}
	found := false
	raw := he.Details["field_errors"]
	switch fe := raw.(type) {
	case map[string]any:
		for k := range fe {
			if strings.Contains(k, "config.options") {
				found = true
				break
			}
		}
	case map[string]string:
		for k := range fe {
			if strings.Contains(k, "config.options") {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatalf("expected field_errors key containing config.options, got %#v", raw)
	}
}

func Test_validateTemplateMatrix_checklistOptionsAllowNonEmptyItems(t *testing.T) {
	t.Parallel()
	blocks := append(validSixMandatoryTemplateBlocksForTests(), TemplateBlockDTO{
		BlockID:   "b7",
		BlockKey:  "docs",
		BlockType: "checklist",
		Title:     "Docs",
		Config: map[string]any{
			"allow_custom_items": false,
			"options":            []any{"A", map[string]any{"label": "B"}},
		},
		Validation:   map[string]any{},
		DisplayOrder: 7,
		Enabled:      true,
	})
	req := &UpsertTypeVersionRequest{
		TypeID:           "dt-schema-test-ok",
		GroupID:          "group-006",
		Name:             "Ok checklist",
		TemplateCategory: TemplateCategoryCustom,
		DeadlineStrategy: DeadlineStrategyConfigurable,
		DeadlineRule:     "T+5",
		Periodicity:      PeriodicityMonthly,
		Blocks:           blocks,
	}
	if err := validateTemplateMatrix(req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func Test_validateTemplateMatrix_unknownBlockTypeRejected(t *testing.T) {
	t.Parallel()
	blocks := append(validSixMandatoryTemplateBlocksForTests(), TemplateBlockDTO{
		BlockID:      "b7",
		BlockKey:     "x",
		BlockType:    "wizard_block",
		Title:        "T",
		Config:       map[string]any{},
		Validation:   map[string]any{},
		DisplayOrder: 7,
		Enabled:      true,
	})
	req := &UpsertTypeVersionRequest{
		TypeID:           "dt-unknown-bt",
		GroupID:          "group-006",
		Name:             "Bad type",
		TemplateCategory: TemplateCategoryCustom,
		DeadlineStrategy: DeadlineStrategyConfigurable,
		DeadlineRule:     "T+5",
		Periodicity:      PeriodicityMonthly,
		Blocks:           blocks,
	}
	err := validateTemplateMatrix(req)
	if err == nil {
		t.Fatal("expected validation error")
	}
	var he *perr.HTTPError
	if !errors.As(err, &he) || he.Details == nil {
		t.Fatalf("expected HTTPError, got %v", err)
	}
	if !fieldErrorsHasKey(he.Details["field_errors"], blockFieldPrefix(6)+".block_type") {
		t.Fatalf("expected %s.block_type in %#v", blockFieldPrefix(6), he.Details["field_errors"])
	}
}

func Test_validateTemplateMatrix_textareaAliasPassesAsText(t *testing.T) {
	t.Parallel()
	blocks := append(validSixMandatoryTemplateBlocksForTests(), TemplateBlockDTO{
		BlockID:      "b7",
		BlockKey:     "body",
		BlockType:    "textarea",
		Title:        "Body",
		Config:       map[string]any{"max_length": 4000},
		Validation:   map[string]any{},
		DisplayOrder: 7,
		Enabled:      true,
	})
	req := &UpsertTypeVersionRequest{
		TypeID:           "dt-textarea-alias",
		GroupID:          "group-006",
		Name:             "Textarea",
		TemplateCategory: TemplateCategoryCustom,
		DeadlineStrategy: DeadlineStrategyConfigurable,
		DeadlineRule:     "T+5",
		Periodicity:      PeriodicityMonthly,
		Blocks:           blocks,
	}
	if err := validateTemplateMatrix(req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func Test_validateTemplateMatrix_tableRequiresColumns(t *testing.T) {
	t.Parallel()
	blocks := append(validSixMandatoryTemplateBlocksForTests(), TemplateBlockDTO{
		BlockID:      "b7",
		BlockKey:     "grid",
		BlockType:    "table",
		Title:        "Grid",
		Config:       map[string]any{},
		Validation:   map[string]any{},
		DisplayOrder: 7,
		Enabled:      true,
	})
	req := &UpsertTypeVersionRequest{
		TypeID:           "dt-table-bad",
		GroupID:          "group-006",
		Name:             "Table",
		TemplateCategory: TemplateCategoryCustom,
		DeadlineStrategy: DeadlineStrategyConfigurable,
		DeadlineRule:     "T+5",
		Periodicity:      PeriodicityMonthly,
		Blocks:           blocks,
	}
	err := validateTemplateMatrix(req)
	if err == nil {
		t.Fatal("expected validation error")
	}
	var he *perr.HTTPError
	if !errors.As(err, &he) {
		t.Fatalf("expected HTTPError, got %v", err)
	}
	if !fieldErrorsHasKey(he.Details["field_errors"], blockFieldPrefix(6)+".config.columns") {
		t.Fatalf("expected %s.config.columns in %#v", blockFieldPrefix(6), he.Details["field_errors"])
	}
}

func Test_validateTemplateMatrix_requiresMandatoryKeysExtrasAllowed(t *testing.T) {
	t.Parallel()
	mb := validSixMandatoryTemplateBlocksForTests()[:5] // omit enterprise_workflow
	req := &UpsertTypeVersionRequest{
		TypeID:           "dt-missing-keys",
		GroupID:          "group-006",
		Name:             "Incomplete",
		TemplateCategory: TemplateCategoryCustom,
		DeadlineStrategy: DeadlineStrategyConfigurable,
		DeadlineRule:     "T+5",
		Periodicity:      PeriodicityMonthly,
		Blocks: append(mb, TemplateBlockDTO{
			BlockID: "extra", BlockKey: "optional_extra", BlockType: "section", Title: "X",
			Config: map[string]any{}, Validation: map[string]any{}, DisplayOrder: 6, Enabled: true,
		}),
	}
	err := validateTemplateMatrix(req)
	if err == nil {
		t.Fatal("expected validation error")
	}
	var he *perr.HTTPError
	if !errors.As(err, &he) || he.Details == nil {
		t.Fatalf("expected HTTPError with details")
	}
	if !fieldErrorsHasKey(he.Details["field_errors"], "blocks.missing_enterprise_workflow") {
		t.Fatalf("expected missing enterprise_workflow, got %#v", he.Details["field_errors"])
	}
}

func fieldErrorsHasKey(raw any, key string) bool {
	switch fe := raw.(type) {
	case map[string]any:
		_, ok := fe[key]
		return ok
	case map[string]string:
		_, ok := fe[key]
		return ok
	default:
		return false
	}
}
