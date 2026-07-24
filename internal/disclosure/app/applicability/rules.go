package applicability

import "strings"

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
	sectors := profile.BusinessSectors
	if len(sectors) == 0 && profile.BusinessSector != nil {
		sectors = []BusinessSector{*profile.BusinessSector}
	}
	if len(sectors) == 0 {
		return false
	}
	for _, companySector := range sectors {
		for _, allowed := range rules.ApplicableSectors {
			if companySector == allowed {
				return true
			}
		}
	}
	return false
}

// ResolveDeadlineDays returns effective N from applicability rules.
// deadline_days is the default; deadline_by_structure applies only when use_structure_deadline=true.
func ResolveDeadlineDays(rules *TemplateApplicabilityRules, profile CompanyApplicabilityProfile) (int, bool) {
	if rules == nil {
		return 0, false
	}
	if !rules.UseStructureDeadline {
		if rules.DeadlineDays <= 0 {
			return 0, false
		}
		return rules.DeadlineDays, true
	}
	if len(rules.DeadlineByStructure) > 0 {
		criterion := ResolveStructure(profile)
		if entry, ok := rules.DeadlineByStructure[criterion]; ok && entry.Days > 0 {
			return entry.Days, true
		}
	}
	if rules.DeadlineDays > 0 {
		return rules.DeadlineDays, true
	}
	return 0, false
}

// ResolveDeadlineDurationType maps applicability deadline_day_type to calculator duration type.
// Empty deadline_day_type defaults to calendar days (contract I-18).
func ResolveDeadlineDurationType(rules *TemplateApplicabilityRules) string {
	if rules == nil {
		return "CALENDAR_DAYS"
	}
	switch strings.ToLower(strings.TrimSpace(rules.DeadlineDayType)) {
	case "working":
		return "WORKING_DAYS"
	default:
		return "CALENDAR_DAYS"
	}
}

// ParseBusinessSector validates and parses sector string from API/DB.
func ParseBusinessSector(raw string) (BusinessSector, bool) {
	s := BusinessSector(raw)
	return s, validBusinessSectors[s]
}
