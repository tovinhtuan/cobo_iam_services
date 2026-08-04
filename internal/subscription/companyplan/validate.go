package companyplan

import (
	"strings"
	"time"
)

// ValidateCreate checks domain rules before persistence.
func ValidateCreate(p CompanyPlan) error {
	if strings.TrimSpace(p.ID) == "" || strings.TrimSpace(p.CompanyID) == "" {
		return ErrInvalidPlan
	}
	if !ValidPlanCode(p.Code) || !ValidPlanStatus(p.Status) {
		return ErrInvalidPlan
	}
	if p.EffectiveFrom.IsZero() {
		return ErrInvalidPlan
	}
	if p.ExpiresAt != nil && !p.ExpiresAt.After(p.EffectiveFrom) {
		return ErrInvalidPlan
	}
	if strings.TrimSpace(string(p.Origin)) == "" {
		return ErrInvalidPlan
	}
	return nil
}

// NormalizeUTC returns a copy with UTC timestamps.
func NormalizeUTC(p CompanyPlan) CompanyPlan {
	out := p
	out.EffectiveFrom = p.EffectiveFrom.UTC()
	if p.ExpiresAt != nil {
		t := p.ExpiresAt.UTC()
		out.ExpiresAt = &t
	}
	if !p.CreatedAt.IsZero() {
		out.CreatedAt = p.CreatedAt.UTC()
	}
	if !p.UpdatedAt.IsZero() {
		out.UpdatedAt = p.UpdatedAt.UTC()
	}
	return out
}

// NowUTC is a thin wrapper for tests to override.
var NowUTC = func() time.Time { return time.Now().UTC() }
