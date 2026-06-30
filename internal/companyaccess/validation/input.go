package validation

import (
	"time"

	"github.com/cobo/cobo_iam_services/internal/companyaccess/conflict"
)

// DepartmentRow is a read-only department row for schema validation.
type DepartmentRow struct {
	DepartmentID string
	Name         string
}

// Input is read-only tenant configuration for the validation pipeline.
type Input struct {
	CompanyID                    string
	ValidatedAt                  time.Time
	Snapshot                     *conflict.ConfigurationSnapshot
	ConflictOutput               conflict.EvaluationOutput
	Departments                  []DepartmentRow
	CompanyAdminCount            int
	CanonicalAlertPrefsRuleCount int
	RuntimeConsumerEnabled       bool
	SubscriptionTierEnforced     bool
}

// ValidatorDeps wires app validators without import cycle.
type ValidatorDeps struct {
	ValidatePrefs          func(map[string]any) (bool, []string)
	ValidateDepartmentName func(name string) (string, error)
}

var validators ValidatorDeps

// RegisterValidators sets validation callbacks (call once from app init).
func RegisterValidators(v ValidatorDeps) {
	validators = v
}
