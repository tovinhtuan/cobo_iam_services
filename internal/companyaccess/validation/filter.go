package validation

import "strings"

// FilterSuites returns a copy of result limited to requested suite names.
// Empty suites or "all" returns the full result unchanged.
func FilterSuites(r Result, suites []string) Result {
	if len(suites) == 0 || hasAllSuite(suites) {
		return r
	}
	want := make(map[string]struct{}, len(suites))
	for _, s := range suites {
		s = strings.TrimSpace(strings.ToLower(s))
		if s == "" || s == "all" {
			return r
		}
		want[s] = struct{}{}
	}
	filtered := make([]SuiteResult, 0, len(want))
	var checks []Check
	for _, suite := range r.Suites {
		if _, ok := want[suite.Suite]; !ok {
			continue
		}
		filtered = append(filtered, suite)
		checks = append(checks, suite.Checks...)
	}
	summary := summarize(checks)
	return Result{
		Passed:      summary.Blocking == 0,
		ValidatedAt: r.ValidatedAt,
		CompanyID:   r.CompanyID,
		Suites:      filtered,
		Summary:     summary,
	}
}

func hasAllSuite(suites []string) bool {
	for _, s := range suites {
		if strings.TrimSpace(strings.ToLower(s)) == "all" {
			return true
		}
	}
	return false
}
