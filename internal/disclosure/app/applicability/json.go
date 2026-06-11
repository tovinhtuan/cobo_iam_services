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

// ProfileFromCompanyDetail maps platform company fields to applicability profile.
func ProfileFromCompanyDetail(
	isListed, isLargePublic, isNonLargePublic, hasSubsidiaries, hasSubordinateAccountingUnits bool,
	businessSector string,
) CompanyApplicabilityProfile {
	p := CompanyApplicabilityProfile{
		IsListed:                      isListed,
		IsLargePublic:                 isLargePublic,
		IsNonLargePublic:              isNonLargePublic,
		HasSubsidiaries:               hasSubsidiaries,
		HasSubordinateAccountingUnits: hasSubordinateAccountingUnits,
	}
	if s := stringsTrim(businessSector); s != "" {
		if sector, ok := ParseBusinessSector(s); ok {
			p.BusinessSector = &sector
		}
	}
	return p
}
