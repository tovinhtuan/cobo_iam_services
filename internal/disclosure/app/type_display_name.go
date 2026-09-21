package app

import (
	"regexp"
	"strings"
)

// curatedTypeDisplayNames maps known seed/test type_id to a business display name.
// Explicit only — never invent labels from arbitrary type IDs.
// ASCII labels avoid source-encoding drift on Windows checkouts.
var curatedTypeDisplayNames = map[string]string{
	"qa-def001-default-workflow-1788528143979": "Default Workflow",
	"qa-def006-ms-20260904213636":              "Reminder Milestones",
}

var (
	qaDefNamePrefix   = regexp.MustCompile(`(?i)^QA\s+DEF\d+\s+`)
	seedTimestampTail = regexp.MustCompile(`\s+\d{10,}$`)
)

// ResolveTypeDisplayName returns the tenant-facing label for a disclosure type.
// Prefer curated map, then normalize technical seed noise in name, else trimmed name.
func ResolveTypeDisplayName(typeID, name string) string {
	id := strings.TrimSpace(typeID)
	if mapped, ok := curatedTypeDisplayNames[id]; ok && strings.TrimSpace(mapped) != "" {
		return strings.TrimSpace(mapped)
	}
	return NormalizeTypeDisplayName(name)
}

// NormalizeTypeDisplayName cleans technical seed/test noise from a stored name
// without inventing a business label from the type_id.
func NormalizeTypeDisplayName(name string) string {
	s := strings.TrimSpace(name)
	if s == "" {
		return ""
	}
	s = qaDefNamePrefix.ReplaceAllString(s, "")
	s = seedTimestampTail.ReplaceAllString(s, "")
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(s)
}

func enrichTypeDisplayNameSummary(item *DisclosureTypeSummaryDTO) {
	if item == nil {
		return
	}
	item.DisplayName = ResolveTypeDisplayName(item.TypeID, item.Name)
}

func enrichTypeDisplayNameDetail(item *DisclosureTypeDTO) {
	if item == nil {
		return
	}
	item.DisplayName = ResolveTypeDisplayName(item.TypeID, item.Name)
}