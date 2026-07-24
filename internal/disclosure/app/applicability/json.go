package applicability

import (
	"encoding/json"
	"strings"
)

func stringsTrim(s string) string {
	return strings.TrimSpace(s)
}

// ParseRulesJSON decodes applicability_rules_json from DB/API.
func ParseRulesJSON(raw []byte) (*TemplateApplicabilityRules, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var rules TemplateApplicabilityRules
	if err := json.Unmarshal(raw, &rules); err != nil {
		return nil, err
	}
	return &rules, nil
}

// MarshalRulesJSON encodes rules for persistence.
func MarshalRulesJSON(rules *TemplateApplicabilityRules) ([]byte, error) {
	if rules == nil {
		return nil, nil
	}
	return json.Marshal(rules)
}

// ParseBusinessSectorsJSON decodes companies.business_sectors JSON; nil/empty → empty slice.
func ParseBusinessSectorsJSON(raw []byte) ([]BusinessSector, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var rawStrings []string
	if err := json.Unmarshal(raw, &rawStrings); err != nil {
		return nil, err
	}
	return NormalizeBusinessSectors(rawStrings)
}

// ProfileFromCompanyDetail maps platform company fields to applicability profile.
// Prefer businessSectorsJSON when present; fall back to legacy single businessSector string.
func ProfileFromCompanyDetail(
	isListed, isLargePublic, isNonLargePublic, hasSubsidiaries, hasSubordinateAccountingUnits bool,
	businessSector string,
	businessSectorsJSON ...[]byte,
) CompanyApplicabilityProfile {
	p := CompanyApplicabilityProfile{
		IsListed:                      isListed,
		IsLargePublic:                 isLargePublic,
		IsNonLargePublic:              isNonLargePublic,
		HasSubsidiaries:               hasSubsidiaries,
		HasSubordinateAccountingUnits: hasSubordinateAccountingUnits,
		BusinessSectors:               []BusinessSector{},
	}
	if len(businessSectorsJSON) > 0 && len(businessSectorsJSON[0]) > 0 {
		if sectors, err := ParseBusinessSectorsJSON(businessSectorsJSON[0]); err == nil && sectors != nil {
			p.BusinessSectors = sectors
		}
	}
	if len(p.BusinessSectors) == 0 {
		if s := stringsTrim(businessSector); s != "" {
			if sector, ok := ParseBusinessSector(s); ok {
				p.BusinessSectors = []BusinessSector{sector}
			}
		}
	}
	if len(p.BusinessSectors) > 0 {
		primary := p.BusinessSectors[0]
		p.BusinessSector = &primary
	}
	return p
}
