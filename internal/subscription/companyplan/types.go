package companyplan

import "time"

// PlanCode is the commercial plan code stored on company_subscriptions.
type PlanCode string

const (
	PlanCodePremium    PlanCode = "PREMIUM"
	PlanCodeEnterprise PlanCode = "ENTERPRISE"
	PlanCodeFree       PlanCode = "FREE"
)

// PlanStatus is the commercial subscription lifecycle status.
type PlanStatus string

const (
	PlanStatusActive    PlanStatus = "ACTIVE"
	PlanStatusTrial     PlanStatus = "TRIAL"
	PlanStatusExpired   PlanStatus = "EXPIRED"
	PlanStatusSuspended PlanStatus = "SUSPENDED"
	PlanStatusCancelled PlanStatus = "CANCELLED"
)

// PlanSource is the wire/domain origin of a company plan for Portal consumers.
// Case C always uses COMPANY_SUBSCRIPTION (never member-max entitlement).
type PlanSource string

const (
	PlanSourceCompanySubscription PlanSource = "COMPANY_SUBSCRIPTION"
)

// RecordOrigin is DB provenance (fixture/seed/admin), distinct from PlanSource.
type RecordOrigin string

const (
	RecordOriginDevFixture          RecordOrigin = "dev_fixture"
	RecordOriginManual              RecordOrigin = "manual"
	RecordOriginPlatformAdminManual RecordOrigin = "platform_admin_manual"
)

// CompanyPlan is the domain model for a commercial company subscription row.
type CompanyPlan struct {
	ID            string
	CompanyID     string
	Code          PlanCode
	Status        PlanStatus
	EffectiveFrom time.Time
	ExpiresAt     *time.Time
	Origin        RecordOrigin
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// DisplayName returns a human label for known codes; unknown codes echo the code string.
func (c PlanCode) DisplayName() string {
	switch c {
	case PlanCodePremium:
		return "Premium"
	case PlanCodeEnterprise:
		return "Enterprise"
	case PlanCodeFree:
		return "Free"
	default:
		return string(c)
	}
}

// WireSource always returns COMPANY_SUBSCRIPTION for Case C paid-plan responses.
func (p CompanyPlan) WireSource() PlanSource {
	return PlanSourceCompanySubscription
}

// IsOccupyingStatus reports whether the status participates in overlap rejection.
func IsOccupyingStatus(s PlanStatus) bool {
	switch s {
	case PlanStatusActive, PlanStatusTrial, PlanStatusSuspended:
		return true
	default:
		return false
	}
}

// ValidPlanCode reports whether code is a known commercial code.
func ValidPlanCode(c PlanCode) bool {
	switch c {
	case PlanCodePremium, PlanCodeEnterprise, PlanCodeFree:
		return true
	default:
		return false
	}
}

// ValidPaidManualPlanCode is the Platform Admin payment-activation target set.
// FREE is the no-row fallback, not a paid activation SKU.
func ValidPaidManualPlanCode(c PlanCode) bool {
	switch c {
	case PlanCodePremium, PlanCodeEnterprise:
		return true
	default:
		return false
	}
}

// ValidPlanStatus reports whether status is a known lifecycle value.
func ValidPlanStatus(s PlanStatus) bool {
	switch s {
	case PlanStatusActive, PlanStatusTrial, PlanStatusExpired, PlanStatusSuspended, PlanStatusCancelled:
		return true
	default:
		return false
	}
}
