package workflowdept

import (
	"strings"
	"time"
)

const (
	StatusNoConfig       = "NO_CONFIG"
	StatusMatched        = "MATCHED"
	StatusMissing        = "MISSING"
	StatusMappedInactive = "MAPPED_INACTIVE"

	PathStepMapping    = "workflow_step_mapping"
	PathTypeMapping    = "disclosure_type_mapping"
	PathCompanyDefault = "company_default_mapping"
	PathExactID        = "exact_company_id"
	PathExactCode      = "exact_company_code"
	PathMissing        = "missing"
)

// OpenMapping is one effective binding row already filtered to the open window.
type OpenMapping struct {
	MappingID              string
	Version                int64
	TemplateDepartmentCode string
	DisclosureTypeID       string
	StepCode               string
	CompanyDepartmentID    string
	DepartmentName         string
	TargetActive           bool
	TargetSameCompany      bool
}

type ResolveInput struct {
	Token             string
	Kind              TokenKind
	BindingMode       bool
	DisclosureTypeID  string
	StepCode          string
	AsOf              time.Time
	Mappings          []OpenMapping
	ExactDepartmentID string
	ExactName         string
	ExactByCode       bool
}

type DepartmentResolution struct {
	Status                string
	CompanyDepartmentID   string
	CompanyDepartmentName string
	MappingID             string
	MappingVersion        int64
	MatchPath             string
	ResolvedAt            time.Time
}

// Resolve applies binding precedence. Exact id/code runs only for company
// tokens, or for any token when binding mode is off (legacy).
// A catalog token with no open mapping is MISSING. It never falls through
// to a company department_code that equals the catalog code.
func Resolve(in ResolveInput) DepartmentResolution {
	now := in.AsOf
	if now.IsZero() {
		now = time.Now().UTC()
	}
	token := strings.TrimSpace(in.Token)
	out := DepartmentResolution{Status: StatusMissing, MatchPath: PathMissing, ResolvedAt: now.UTC()}
	if token == "" {
		out.Status = StatusNoConfig
		out.MatchPath = ""
		return out
	}
	if !in.BindingMode {
		return exactLegacy(in, token, now)
	}
	if in.Kind == TokenTemplateDepartmentCode {
		if hit, ok := pickMapping(in); ok {
			return fromMapping(hit, now)
		}
		return out
	}
	if in.Kind == TokenCompanyDepartmentID || in.Kind == TokenCompanyDepartmentCode {
		return exactLegacy(in, token, now)
	}
	return out
}

func pickMapping(in ResolveInput) (OpenMapping, bool) {
	token := strings.TrimSpace(in.Token)
	typeID := strings.TrimSpace(in.DisclosureTypeID)
	step := strings.TrimSpace(in.StepCode)
	var stepHit, typeHit, defHit *OpenMapping
	for i := range in.Mappings {
		m := in.Mappings[i]
		if !strings.EqualFold(strings.TrimSpace(m.TemplateDepartmentCode), token) {
			continue
		}
		mType := strings.TrimSpace(m.DisclosureTypeID)
		mStep := strings.TrimSpace(m.StepCode)
		switch {
		case typeID != "" && step != "" && strings.EqualFold(mType, typeID) && strings.EqualFold(mStep, step):
			cp := m
			stepHit = &cp
		case typeID != "" && mStep == "" && strings.EqualFold(mType, typeID):
			cp := m
			typeHit = &cp
		case mType == "" && mStep == "":
			cp := m
			defHit = &cp
		}
	}
	switch {
	case stepHit != nil:
		return *stepHit, true
	case typeHit != nil:
		return *typeHit, true
	case defHit != nil:
		return *defHit, true
	default:
		return OpenMapping{}, false
	}
}

func fromMapping(m OpenMapping, now time.Time) DepartmentResolution {
	path := PathCompanyDefault
	if strings.TrimSpace(m.StepCode) != "" {
		path = PathStepMapping
	} else if strings.TrimSpace(m.DisclosureTypeID) != "" {
		path = PathTypeMapping
	}
	out := DepartmentResolution{
		MappingID:      m.MappingID,
		MappingVersion: m.Version,
		MatchPath:      path,
		ResolvedAt:     now.UTC(),
	}
	if !m.TargetSameCompany || strings.TrimSpace(m.CompanyDepartmentID) == "" || !m.TargetActive {
		out.Status = StatusMappedInactive
		return out
	}
	out.Status = StatusMatched
	out.CompanyDepartmentID = m.CompanyDepartmentID
	out.CompanyDepartmentName = m.DepartmentName
	return out
}

func exactLegacy(in ResolveInput, token string, now time.Time) DepartmentResolution {
	out := DepartmentResolution{Status: StatusMissing, MatchPath: PathMissing, ResolvedAt: now.UTC()}
	if id := strings.TrimSpace(in.ExactDepartmentID); id != "" {
		out.Status = StatusMatched
		out.CompanyDepartmentID = id
		out.CompanyDepartmentName = in.ExactName
		if in.ExactByCode {
			out.MatchPath = PathExactCode
		} else {
			out.MatchPath = PathExactID
		}
		return out
	}
	_ = token
	return out
}
