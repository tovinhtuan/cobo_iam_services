package app

import (
	"fmt"
	"net/http"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// CompanyOverrideAuthority is the Company side of Effective Anchor resolution.
// Active must be true and Frequency must match current CMS frequency for authority.
type CompanyOverrideAuthority struct {
	Active    bool
	Frequency string
	Anchor    AnchorConfig
}

// PreferenceToOverrideAuthority maps stored preference → resolver authority.
// Inactive or frequency-mismatched overrides yield Active=false (CMS wins) while
// preserving historical values in the returned Anchor for diagnostics/UI only.
//
// Legacy soft-bind: rows with month/day (or typed fields) but NULL binding/active
// (pre-0134 or un-backfilled) are treated as ACTIVE for the current CMS frequency
// so existing Company overrides keep working until/unless backfill runs.
func PreferenceToOverrideAuthority(pref *CompanyTypePreference, cmsFrequency string) CompanyOverrideAuthority {
	if pref == nil {
		return CompanyOverrideAuthority{}
	}
	out := CompanyOverrideAuthority{
		Frequency: pref.OverrideFrequency,
		Anchor: AnchorConfig{
			Month:          pref.CycleAnchorMonth,
			Day:            pref.CycleAnchorDay,
			Weekday:        pref.CycleAnchorWeekday,
			MonthInQuarter: pref.MonthInQuarter,
		},
	}
	cmsFreq := NormalizeFrequencyUnit(cmsFrequency)
	if pref.OverrideActive != nil {
		if !*pref.OverrideActive {
			return out
		}
		if NormalizeFrequencyUnit(pref.OverrideFrequency) != cmsFreq {
			return out
		}
		out.Active = true
		out.Frequency = cmsFreq
		return out
	}
	// Legacy: no explicit active flag — soft-bind to current CMS frequency when values exist.
	if pref.OverrideFrequency != "" && NormalizeFrequencyUnit(pref.OverrideFrequency) != cmsFreq {
		return out
	}
	if !out.Anchor.HasOverride() {
		return out
	}
	out.Active = true
	out.Frequency = cmsFreq
	return out
}

// HasActiveCompatibleOverride reports whether Company currently owns Effective Anchor.
func HasActiveCompatibleOverride(pref *CompanyTypePreference, cmsFrequency string) bool {
	auth := PreferenceToOverrideAuthority(pref, cmsFrequency)
	return auth.Active && companyOverrideHasAuthorityFields(NormalizeFrequencyUnit(cmsFrequency), auth.Anchor)
}

func companyOverrideHasAuthorityFields(freq string, a AnchorConfig) bool {
	switch freq {
	case PeriodicityDaily:
		return false
	case PeriodicityWeekly:
		return a.Weekday != nil
	case PeriodicityMonthly:
		return a.Day > 0
	case PeriodicityQuarterly:
		// V2 atomic: MiQ + day. Legacy day-only (MiQ nil) also counts for compat.
		return a.Day > 0
	case PeriodicityYearly:
		return a.Month > 0 && a.Day > 0
	default:
		return a.Month > 0 || a.Day > 0 || a.Weekday != nil || a.MonthInQuarter != nil
	}
}

// ValidateCompanyCycleAnchorOverride validates an explicit Company override write
// against CMS frequency authority. Clear is handled by the caller (skip validation).
// Company cannot submit/override frequency — cmsFrequency is authoritative.
func ValidateCompanyCycleAnchorOverride(cmsFrequency string, req UpsertCompanyTypePreferenceRequest) error {
	freq := NormalizeFrequencyUnit(cmsFrequency)
	switch freq {
	case PeriodicityDaily:
		if req.CycleAnchorWeekday != nil || req.MonthInQuarter != nil || req.CycleAnchorMonth > 0 || req.CycleAnchorDay > 0 {
			return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest,
				"daily schedule has no company cycle-anchor override; use clear_cycle_anchor to inherit CMS", nil)
		}
		return nil
	case PeriodicityWeekly:
		if req.CycleAnchorWeekday == nil {
			return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest,
				"weekly company override requires cycle_anchor_weekday", nil)
		}
		return ValidateCycleAnchorWeekday(req.CycleAnchorWeekday)
	case PeriodicityMonthly:
		if req.CycleAnchorDay <= 0 {
			return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest,
				"monthly company override requires cycle_anchor_day (1..31)", nil)
		}
		return ValidateCycleAnchorDay(req.CycleAnchorDay)
	case PeriodicityQuarterly:
		if req.MonthInQuarter == nil || req.CycleAnchorDay <= 0 {
			return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest,
				"quarterly company override requires month_in_quarter and cycle_anchor_day atomically", nil)
		}
		if err := ValidateMonthInQuarter(req.MonthInQuarter); err != nil {
			return err
		}
		return ValidateCycleAnchorDay(req.CycleAnchorDay)
	case PeriodicityYearly:
		if req.CycleAnchorMonth <= 0 || req.CycleAnchorDay <= 0 {
			return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest,
				"yearly company override requires cycle_anchor_month and cycle_anchor_day atomically", nil)
		}
		if req.CycleAnchorMonth < 1 || req.CycleAnchorMonth > 12 {
			return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest,
				fmt.Sprintf("cycle_anchor_month must be between 1 and 12 (got %d)", req.CycleAnchorMonth), nil)
		}
		return ValidateCycleAnchorDay(req.CycleAnchorDay)
	default:
		// Unknown frequency: keep legacy month/day validation only when provided.
		if err := ValidateCycleAnchorDay(req.CycleAnchorDay); err != nil {
			return err
		}
		if err := ValidateCycleAnchorWeekday(req.CycleAnchorWeekday); err != nil {
			return err
		}
		return ValidateMonthInQuarter(req.MonthInQuarter)
	}
}

// BuildCompanyOverrideWrite constructs repository write values for an explicit override.
// Frequency-irrelevant fields are cleared so only one typed authority is active.
func BuildCompanyOverrideWrite(cmsFrequency string, req UpsertCompanyTypePreferenceRequest) CompanyTypePreference {
	freq := NormalizeFrequencyUnit(cmsFrequency)
	active := true
	out := CompanyTypePreference{
		CompanyID:         req.Subject.CompanyID,
		TypeID:            req.TypeID,
		AutoCreateEnabled: req.AutoCreateEnabled,
		UpdatedBy:         req.Subject.MembershipID,
		OverrideFrequency: freq,
		OverrideActive:    &active,
	}
	switch freq {
	case PeriodicityWeekly:
		out.CycleAnchorWeekday = req.CycleAnchorWeekday
	case PeriodicityMonthly:
		out.CycleAnchorDay = req.CycleAnchorDay
	case PeriodicityQuarterly:
		out.MonthInQuarter = req.MonthInQuarter
		out.CycleAnchorDay = req.CycleAnchorDay
	case PeriodicityYearly:
		out.CycleAnchorMonth = req.CycleAnchorMonth
		out.CycleAnchorDay = req.CycleAnchorDay
	case PeriodicityDaily:
		// no override fields
	default:
		// Legacy fallback: persist month/day if present.
		out.CycleAnchorMonth = req.CycleAnchorMonth
		out.CycleAnchorDay = req.CycleAnchorDay
		out.CycleAnchorWeekday = req.CycleAnchorWeekday
		out.MonthInQuarter = req.MonthInQuarter
	}
	return out
}

// CompanyOverrideWriteTouchesAnchor reports whether the request intends to set/clear
// schedule override (vs auto_create-only patch that must not materialize inherit).
func CompanyOverrideWriteTouchesAnchor(req UpsertCompanyTypePreferenceRequest) bool {
	if req.ClearCycleAnchor {
		return true
	}
	return req.CycleAnchorWeekday != nil ||
		req.MonthInQuarter != nil ||
		req.CycleAnchorMonth > 0 ||
		req.CycleAnchorDay > 0
}
