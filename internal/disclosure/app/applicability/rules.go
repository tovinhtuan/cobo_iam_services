package applicability

// CompanyLabels returns active company class labels from profile checkboxes.
func CompanyLabels(p CompanyApplicabilityProfile) []CompanyClass {
	out := make([]CompanyClass, 0, 3)
	if p.IsListed {
		out = append(out, CompanyClassListed)
	}
	if p.IsLargePublic {
		out = append(out, CompanyClassLargePublic)
	}
	if p.IsNonLargePublic {
		out = append(out, CompanyClassNonLargePublic)
	}
	return out
}

// ResolveStructure picks the structure criterion for deadline resolution.
func ResolveStructure(p CompanyApplicabilityProfile) StructureCriterion {
	if p.HasSubsidiaries {
		return StructureHasSubsidiaries
	}
	if p.HasSubordinateAccountingUnits {
		return StructureHasSubordinateUnits
	}
	return StructureSimpleStructure
}

// IsApplicable returns whether a global template matches company profile.
// When rules is nil: pass if strictFilter is false (grace), else reject.
func IsApplicable(rules *TemplateApplicabilityRules, profile CompanyApplicabilityProfile, strictFilter bool) bool {
	if rules == nil {
		return !strictFilter
	}
	labels := CompanyLabels(profile)
	classMatch := false
	for _, label := range labels {
		for _, allowed := range rules.ApplicableCompanyClasses {
			if label == allowed {
				classMatch = true
				break
			}
		}
		if classMatch {
			break
		}
	}
	if !classMatch {
		return false
	}
	if profile.BusinessSector == nil {
		return false
	}
	sectorMatch := false
	for _, allowed := range rules.ApplicableSectors {
		if *profile.BusinessSector == allowed {
			sectorMatch = true
			break
		}
	}
	return sectorMatch
}

// ResolveDeadlineDays returns structure-based deadline days when configured.
func ResolveDeadlineDays(rules *TemplateApplicabilityRules, profile CompanyApplicabilityProfile) (int, bool) {
	if rules == nil || len(rules.DeadlineByStructure) == 0 {
		return 0, false
	}
	criterion := ResolveStructure(profile)
	entry, ok := rules.DeadlineByStructure[criterion]
	if !ok {
		return 0, false
	}
	return entry.Days, true
}

// ParseBusinessSector validates and parses sector string from API/DB.
func ParseBusinessSector(raw string) (BusinessSector, bool) {
	s := BusinessSector(raw)
	return s, validBusinessSectors[s]
}
