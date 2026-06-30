package validation

import "time"

// Suite names (locked order).
const (
	SuiteSchema      = "schema"
	SuiteBusiness    = "business"
	SuiteDependency  = "dependency"
	SuiteConflict    = "conflict"
	SuiteRuntime     = "runtime"
	SuitePersistence = "persistence"
	SuiteAudit       = "audit"
	SuiteDispatch    = "dispatch"
)

// Locked stage order for Batch 2.
var StageOrder = []string{
	SuiteSchema,
	SuiteBusiness,
	SuiteDependency,
	SuiteConflict,
	SuiteRuntime,
	SuitePersistence,
	SuiteAudit,
	SuiteDispatch,
}

// Check is one validation finding (ADR-015).
type Check struct {
	Code       string         `json:"code"`
	Suite      string         `json:"suite"`
	Severity   string         `json:"severity"`
	Message    string         `json:"message"`
	ActionLink string         `json:"action_link,omitempty"`
	Evidence   map[string]any `json:"evidence,omitempty"`
}

// SuiteResult is the outcome of one validation stage.
type SuiteResult struct {
	Suite         string  `json:"suite"`
	Passed        bool    `json:"passed"`
	Checks        []Check `json:"checks"`
	SkippedReason string  `json:"skipped_reason,omitempty"`
}

// Summary aggregates check severities.
type Summary struct {
	Total    int `json:"total"`
	Failed   int `json:"failed"`
	Blocking int `json:"blocking"`
	Warning  int `json:"warning"`
	Info     int `json:"info"`
}

// Result is the full validation response (Batch 0 contract).
type Result struct {
	Passed      bool          `json:"passed"`
	ValidatedAt time.Time     `json:"validated_at"`
	CompanyID   string        `json:"company_id"`
	Suites      []SuiteResult `json:"suites"`
	Summary     Summary       `json:"summary"`
}

const (
	SeverityBlocking = "blocking"
	SeverityWarning  = "warning"
	SeverityInfo     = "info"
)
