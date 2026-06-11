package applicability

// CompanyClass — Chiều 2 visibility.
type CompanyClass string

const (
	CompanyClassListed         CompanyClass = "listed"
	CompanyClassLargePublic    CompanyClass = "large_public"
	CompanyClassNonLargePublic CompanyClass = "non_large_public"
)

// BusinessSector — Chiều 3 visibility.
type BusinessSector string

const (
	BusinessSectorCommercial    BusinessSector = "commercial"
	BusinessSectorService       BusinessSector = "service"
	BusinessSectorManufacturing BusinessSector = "manufacturing"
)

// StructureCriterion — Chiều 1 deadline (simple_structure is derived only).
type StructureCriterion string

const (
	StructureHasSubsidiaries    StructureCriterion = "has_subsidiaries"
	StructureHasSubordinateUnits StructureCriterion = "has_subordinate_units"
	StructureSimpleStructure    StructureCriterion = "simple_structure"
)

var (
	validCompanyClasses = map[CompanyClass]bool{
		CompanyClassListed: true, CompanyClassLargePublic: true, CompanyClassNonLargePublic: true,
	}
	validBusinessSectors = map[BusinessSector]bool{
		BusinessSectorCommercial: true, BusinessSectorService: true, BusinessSectorManufacturing: true,
	}
	validStructureCriteria = map[StructureCriterion]bool{
		StructureHasSubsidiaries: true, StructureHasSubordinateUnits: true, StructureSimpleStructure: true,
	}
	requiredPeriodicStructureKeys = []StructureCriterion{
		StructureHasSubsidiaries, StructureHasSubordinateUnits, StructureSimpleStructure,
	}
)

// StructureDeadlineEntry holds calendar days after period end.
type StructureDeadlineEntry struct {
	Days int `json:"days"`
}

// TemplateApplicabilityRules is persisted on disclosure_type_versions.applicability_rules_json.
type TemplateApplicabilityRules struct {
	ApplicableCompanyClasses []CompanyClass                                    `json:"applicable_company_classes"`
	ApplicableSectors        []BusinessSector                                  `json:"applicable_sectors"`
	DeadlineByStructure      map[StructureCriterion]StructureDeadlineEntry     `json:"deadline_by_structure,omitempty"`
}

// CompanyApplicabilityProfile is the company self-declaration subset.
type CompanyApplicabilityProfile struct {
	IsListed                      bool
	IsLargePublic                 bool
	IsNonLargePublic              bool
	HasSubsidiaries               bool
	HasSubordinateAccountingUnits bool
	BusinessSector                *BusinessSector
}
