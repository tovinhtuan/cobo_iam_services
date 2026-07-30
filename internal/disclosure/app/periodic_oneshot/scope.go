package periodic_oneshot

import (
	"fmt"
	"strings"
)

// Phase-locked exact allowlist (outside reusable domain seed logic).
const (
	AllowedTypeID    = "qa-monthly-deadline-alert-202607-1785382733"
	AllowedCompanyID = "c_001"
	AllowedPeriod    = "2026-07"

	ExpectedDeadlineMode = "PERIODIC"
	ExpectedDeadlineDays = 23
	ExpectedDurationUnit = "WORKING_DAYS"
	ExpectedDueDate      = "2026-07-31"
	ExpectedPeriodStart  = "2026-07-01"
	ExpectedPeriodEnd    = "2026-07-31"
	ExpectedFreqUnit     = "monthly"
)

// Scope is the exact one-shot materialization target.
type Scope struct {
	TypeID    string
	CompanyID string
	Period    string
}

func (s Scope) Normalize() Scope {
	return Scope{
		TypeID:    strings.TrimSpace(s.TypeID),
		CompanyID: strings.TrimSpace(s.CompanyID),
		Period:    strings.TrimSpace(s.Period),
	}
}

// ValidateAllowlist refuses anything outside the phase allowlist.
func ValidateAllowlist(s Scope) error {
	s = s.Normalize()
	if s.TypeID == "" || s.CompanyID == "" || s.Period == "" {
		return fmt.Errorf("MATERIALIZATION_SCOPE_NOT_ALLOWED: empty type/company/period")
	}
	if strings.Contains(s.TypeID, ",") || strings.Contains(s.CompanyID, ",") || strings.Contains(s.Period, ",") {
		return fmt.Errorf("MATERIALIZATION_SCOPE_NOT_ALLOWED: lists not permitted")
	}
	if strings.EqualFold(s.TypeID, "all") || strings.EqualFold(s.CompanyID, "all") || strings.EqualFold(s.Period, "all") {
		return fmt.Errorf("MATERIALIZATION_SCOPE_NOT_ALLOWED: wildcard all")
	}
	if s.TypeID != AllowedTypeID || s.CompanyID != AllowedCompanyID || s.Period != AllowedPeriod {
		return fmt.Errorf("MATERIALIZATION_SCOPE_NOT_ALLOWED: scope must be type=%s company=%s period=%s",
			AllowedTypeID, AllowedCompanyID, AllowedPeriod)
	}
	return nil
}
