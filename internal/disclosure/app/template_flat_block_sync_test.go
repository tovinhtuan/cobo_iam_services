package app

import (
	"testing"

	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

type stubID struct{}

func (stubID) NewUUID() string { return "uuid-test" }

func TestApplyTemplateFlatBlockSync_materializesFromFlatWhenBlocksEmpty(t *testing.T) {
	t.Parallel()
	req := &UpsertTypeVersionRequest{
		TypeID:           "dt-x",
		GroupID:          "g",
		Name:             "n",
		TemplateCategory: TemplateCategoryCustom,
		DeadlineStrategy: DeadlineStrategyConfigurable,
		DeadlineRule:     "T+5",
		Periodicity:      PeriodicityMonthly,
		LegalBasis:       "Luật A",
		ReportContent:    "Báo cáo B",
		Blocks:           nil,
	}
	ApplyTemplateFlatBlockSync(req, stubID{})
	if len(req.Blocks) != len(MandatoryTemplateBlockKeys) {
		t.Fatalf("expected %d blocks, got %d", len(MandatoryTemplateBlockKeys), len(req.Blocks))
	}
	if req.Blocks[0].BlockKey != "legal_basis" || req.Blocks[0].Description != "Luật A" {
		t.Fatalf("first block: %+v", req.Blocks[0])
	}
	if req.LegalBasis != "Luật A" {
		t.Fatalf("mirror legal_basis: %q", req.LegalBasis)
	}
}

func TestApplyTemplateFlatBlockSync_flatOverridesMandatoryDescription(t *testing.T) {
	t.Parallel()
	req := &UpsertTypeVersionRequest{
		TypeID:           "dt-x",
		GroupID:          "g",
		Name:             "n",
		TemplateCategory: TemplateCategoryCustom,
		DeadlineStrategy: DeadlineStrategyConfigurable,
		DeadlineRule:     "T+5",
		Periodicity:      PeriodicityMonthly,
		LegalBasis:       "From flat",
		Blocks: []TemplateBlockDTO{
			{
				BlockID: "b1", BlockKey: "legal_basis", BlockType: "rich_text", Title: "T",
				Description: "old", Config: map[string]any{"max_length": 8000, "allow_html": false},
				Validation: map[string]any{}, DisplayOrder: 1, Enabled: true,
			},
		},
	}
	ApplyTemplateFlatBlockSync(req, idgen.UUIDv7Generator{})
	if req.Blocks[0].Description != "From flat" {
		t.Fatalf("expected flat to win, got %q", req.Blocks[0].Description)
	}
}

func TestApplyTemplateFlatBlockSync_skipsRebuildOnDuplicateKeys(t *testing.T) {
	t.Parallel()
	req := &UpsertTypeVersionRequest{
		TypeID:           "dt-x",
		GroupID:          "g",
		Name:             "n",
		TemplateCategory: TemplateCategoryCustom,
		DeadlineStrategy: DeadlineStrategyConfigurable,
		DeadlineRule:     "T+5",
		Periodicity:      PeriodicityMonthly,
		Blocks: []TemplateBlockDTO{
			{BlockID: "a", BlockKey: "dup", BlockType: "text", Title: "A", Config: map[string]any{}, Validation: map[string]any{}, DisplayOrder: 1, Enabled: true},
			{BlockID: "b", BlockKey: "dup", BlockType: "text", Title: "B", Config: map[string]any{}, Validation: map[string]any{}, DisplayOrder: 2, Enabled: true},
		},
	}
	ApplyTemplateFlatBlockSync(req, stubID{})
	if len(req.Blocks) != 2 {
		t.Fatalf("expected rebuild skipped, got %d blocks", len(req.Blocks))
	}
}
