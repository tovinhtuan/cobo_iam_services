package applicability

import (
	"fmt"
	"strings"
)

// ValidateRules enforces CMS upsert/activate rules (E01–E06).
func ValidateRules(rules *TemplateApplicabilityRules, isPeriodic bool) error {
	if rules == nil {
		return fmt.Errorf("applicability_rules is required")
	}
	if len(rules.ApplicableCompanyClasses) < 1 {
		return fmt.Errorf("at least one company class required")
	}
	for _, c := range rules.ApplicableCompanyClasses {
		if !validCompanyClasses[c] {
			return fmt.Errorf("invalid company class")
		}
	}
	if len(rules.ApplicableSectors) < 1 {
		return fmt.Errorf("at least one sector required")
	}
	for _, s := range rules.ApplicableSectors {
		if !validBusinessSectors[s] {
			return fmt.Errorf("invalid sector")
		}
	}
	if isPeriodic {
		if rules.DeadlineByStructure == nil {
			return fmt.Errorf("deadline_by_structure incomplete for periodic template")
		}
		for _, key := range requiredPeriodicStructureKeys {
			entry, ok := rules.DeadlineByStructure[key]
			if !ok {
				return fmt.Errorf("deadline_by_structure incomplete for periodic template")
			}
			if entry.Days < 0 {
				return fmt.Errorf("deadline days must be >= 0")
			}
		}
	} else if rules.DeadlineByStructure != nil {
		for _, entry := range rules.DeadlineByStructure {
			if entry.Days < 0 {
				return fmt.Errorf("deadline days must be >= 0")
			}
		}
	}
	return nil
}

// ValidateBusinessSectorPatch validates PATCH business_sector value.
func ValidateBusinessSectorPatch(raw *string) error {
	if raw == nil {
		return nil
	}
	v := strings.TrimSpace(*raw)
	if v == "" {
		return nil
	}
	if _, ok := ParseBusinessSector(v); !ok {
		return fmt.Errorf("invalid business_sector")
	}
	return nil
}

// ValidateBusinessSectorsPatch validates PATCH business_sectors array.
func ValidateBusinessSectorsPatch(raw *[]string) error {
	if raw == nil {
		return nil
	}
	_, err := NormalizeBusinessSectors(*raw)
	return err
}
