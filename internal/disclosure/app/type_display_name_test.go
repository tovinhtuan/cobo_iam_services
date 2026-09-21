package app

import "testing"

func TestResolveTypeDisplayName_curatedMap(t *testing.T) {
	got := ResolveTypeDisplayName("qa-def001-default-workflow-1788528143979", "QA DEF001 Default Workflow 1788528143979")
	if got != "Default Workflow" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveTypeDisplayName_normalizesSeedNameWithoutCuratedID(t *testing.T) {
	got := ResolveTypeDisplayName("some-other-id", "QA DEF002 Sample Template 1699999999999")
	if got != "Sample Template" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveTypeDisplayName_preservesBusinessName(t *testing.T) {
	got := ResolveTypeDisplayName("dt-001", "Bao cao tai chinh quy")
	if got != "Bao cao tai chinh quy" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveTypeDisplayName_doesNotInventFromUnknownTypeID(t *testing.T) {
	got := ResolveTypeDisplayName("dt-co-abcdef12-3456-7890-abcd-ef1234567890", "My custom type")
	if got != "My custom type" {
		t.Fatalf("got %q — must not invent from type_id", got)
	}
}

func TestEnrichTypeDisplayNameSummary(t *testing.T) {
	item := &DisclosureTypeSummaryDTO{
		TypeID: "qa-def001-default-workflow-1788528143979",
		Name:   "QA DEF001 Default Workflow 1788528143979",
	}
	enrichTypeDisplayNameSummary(item)
	if item.DisplayName != "Default Workflow" {
		t.Fatalf("DisplayName=%q", item.DisplayName)
	}
	if item.Name != "QA DEF001 Default Workflow 1788528143979" {
		t.Fatalf("Name must stay raw technical value, got %q", item.Name)
	}
}