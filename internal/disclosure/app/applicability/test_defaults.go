package applicability

// DefaultGlobalRules returns valid rules for tests and backfill parity.
func DefaultGlobalRules(periodic bool) *TemplateApplicabilityRules {
	rules := &TemplateApplicabilityRules{
		ApplicableCompanyClasses: []CompanyClass{
			CompanyClassListed, CompanyClassLargePublic, CompanyClassNonLargePublic,
		},
		ApplicableSectors: []BusinessSector{
			BusinessSectorCommercial, BusinessSectorService, BusinessSectorManufacturing,
		},
	}
	if periodic {
		rules.DeadlineByStructure = map[StructureCriterion]StructureDeadlineEntry{
			StructureHasSubsidiaries:     {Days: 30},
			StructureHasSubordinateUnits: {Days: 30},
			StructureSimpleStructure:     {Days: 20},
		}
	}
	return rules
}
