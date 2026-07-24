package applicability

import (
	"fmt"
	"strings"
)

// CanonicalBusinessSectorOrder is the stable order used when normalizing multi-select.
var CanonicalBusinessSectorOrder = []BusinessSector{
	BusinessSectorCommercial,
	BusinessSectorService,
	BusinessSectorManufacturing,
}

// NormalizeBusinessSectors trims, dedupes, and sorts input into canonical enum order.
// Empty input yields an empty slice (clear). Unknown codes return an error.
func NormalizeBusinessSectors(input []string) ([]BusinessSector, error) {
	seen := make(map[BusinessSector]bool, len(input))
	for _, raw := range input {
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
		}
		sector, ok := ParseBusinessSector(v)
		if !ok {
			return nil, fmt.Errorf("invalid business_sector %q", v)
		}
		seen[sector] = true
	}
	out := make([]BusinessSector, 0, len(seen))
	for _, sector := range CanonicalBusinessSectorOrder {
		if seen[sector] {
			out = append(out, sector)
		}
	}
	return out, nil
}

// BusinessSectorsToStrings converts sectors to string slice for JSON/DTO.
func BusinessSectorsToStrings(sectors []BusinessSector) []string {
	if len(sectors) == 0 {
		return []string{}
	}
	out := make([]string, len(sectors))
	for i, s := range sectors {
		out[i] = string(s)
	}
	return out
}

// PrimaryBusinessSector returns the first canonical sector or empty string.
func PrimaryBusinessSector(sectors []BusinessSector) string {
	if len(sectors) == 0 {
		return ""
	}
	return string(sectors[0])
}
