package app

import "testing"

func TestEnrichTemplateBlockDisplayNames_mandatoryKey(t *testing.T) {
	blocks := []TemplateBlockDTO{
		{BlockKey: "legal_basis", Title: "Custom VI title", Description: "x"},
	}
	EnrichTemplateBlockDisplayNames(blocks)
	if blocks[0].NameVI != "Custom VI title" {
		t.Fatalf("NameVI: want Custom VI title, got %q", blocks[0].NameVI)
	}
	if blocks[0].NameEN != "Legal basis" {
		t.Fatalf("NameEN: want Legal basis, got %q", blocks[0].NameEN)
	}
}

func TestEnrichTemplateBlockDisplayNames_prefersExistingBilingual(t *testing.T) {
	blocks := []TemplateBlockDTO{
		{BlockKey: "deadline", Title: "", NameEN: "From DB EN", NameVI: "From DB VI"},
	}
	EnrichTemplateBlockDisplayNames(blocks)
	if blocks[0].NameEN != "From DB EN" || blocks[0].NameVI != "From DB VI" {
		t.Fatalf("expected preserved NameEN/NameVI, got %+v", blocks[0])
	}
}
