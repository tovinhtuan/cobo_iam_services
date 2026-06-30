package healthscore

import (
	"strings"
	"time"
)

const (
	AlgorithmName = "weighted_severity_v1"
	MaxValue      = 100
	BaseValue     = 100

	blockingPenaltyPer = 25
	blockingPenaltyCap = 100
	warningPenaltyPer  = 10
	warningPenaltyCap  = 50
	infoPenaltyPer     = 2
	infoPenaltyCap     = 10

	runtimeInfoPrefix = "info.runtime."
)

// Check is minimal input for score calculation (code + severity only).
type Check struct {
	Code     string
	Severity string
}

// Deduction is a per-severity penalty rollup for UI breakdown.
type Deduction struct {
	Severity     string `json:"severity"`
	Count        int    `json:"count"`
	PenaltyPer   int    `json:"penalty_per"`
	PenaltyTotal int    `json:"penalty_total"`
	Capped       bool   `json:"capped"`
}

// Result is the weighted_severity_v1 health score output.
type Result struct {
	Value      int         `json:"value"`
	Max        int         `json:"max"`
	Algorithm  string      `json:"algorithm"`
	ComputedAt time.Time   `json:"computed_at"`
	Status     string      `json:"status,omitempty"`
	Summary    string      `json:"summary,omitempty"`
	Deductions []Deduction `json:"deductions,omitempty"`
}

// Compute derives a health score from configuration-health checks (pure, no I/O).
func Compute(checks []Check, computedAt time.Time) Result {
	if computedAt.IsZero() {
		computedAt = time.Now().UTC()
	}
	filtered := dedupeChecks(checks)

	var blocking, warning, info int
	for _, c := range filtered {
		switch c.Severity {
		case "blocking":
			blocking++
		case "warning":
			warning++
		case "info":
			info++
		}
	}

	deductions := buildDeductions(blocking, warning, info)
	totalPenalty := 0
	for _, d := range deductions {
		totalPenalty += d.PenaltyTotal
	}

	value := BaseValue - totalPenalty
	if value < 0 {
		value = 0
	}

	status := mapStatus(value)
	return Result{
		Value:      value,
		Max:        MaxValue,
		Algorithm:  AlgorithmName,
		ComputedAt: computedAt,
		Status:     status,
		Summary:    statusSummary(status),
		Deductions: deductions,
	}
}

func dedupeChecks(checks []Check) []Check {
	best := make(map[string]Check)
	for _, c := range checks {
		if strings.HasPrefix(c.Code, runtimeInfoPrefix) {
			continue
		}
		if c.Code == "" {
			continue
		}
		if prev, ok := best[c.Code]; !ok || severityRank(c.Severity) < severityRank(prev.Severity) {
			best[c.Code] = c
		}
	}
	out := make([]Check, 0, len(best))
	for _, c := range best {
		out = append(out, c)
	}
	return out
}

func severityRank(sev string) int {
	switch sev {
	case "blocking":
		return 0
	case "warning":
		return 1
	case "info":
		return 2
	default:
		return 3
	}
}

func buildDeductions(blocking, warning, info int) []Deduction {
	var out []Deduction
	if blocking > 0 {
		raw := blocking * blockingPenaltyPer
		capped := raw > blockingPenaltyCap
		total := raw
		if capped {
			total = blockingPenaltyCap
		}
		out = append(out, Deduction{
			Severity:     "blocking",
			Count:        blocking,
			PenaltyPer:   blockingPenaltyPer,
			PenaltyTotal: total,
			Capped:       capped,
		})
	}
	if warning > 0 {
		raw := warning * warningPenaltyPer
		capped := raw > warningPenaltyCap
		total := raw
		if capped {
			total = warningPenaltyCap
		}
		out = append(out, Deduction{
			Severity:     "warning",
			Count:        warning,
			PenaltyPer:   warningPenaltyPer,
			PenaltyTotal: total,
			Capped:       capped,
		})
	}
	if info > 0 {
		raw := info * infoPenaltyPer
		capped := raw > infoPenaltyCap
		total := raw
		if capped {
			total = infoPenaltyCap
		}
		out = append(out, Deduction{
			Severity:     "info",
			Count:        info,
			PenaltyPer:   infoPenaltyPer,
			PenaltyTotal: total,
			Capped:       capped,
		})
	}
	return out
}

func mapStatus(value int) string {
	if value >= 80 {
		return "excellent"
	}
	if value >= 50 {
		return "warning"
	}
	return "attention"
}

func statusSummary(status string) string {
	switch status {
	case "excellent":
		return "Cấu hình tốt."
	case "warning":
		return "Có cảnh báo — xem checks."
	case "attention":
		return "Có vấn đề nghiêm trọng — xem checks."
	default:
		return ""
	}
}
