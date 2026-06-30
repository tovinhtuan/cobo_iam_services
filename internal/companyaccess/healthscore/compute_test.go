package healthscore_test

import (
	"testing"
	"time"

	"github.com/cobo/cobo_iam_services/internal/companyaccess/healthscore"
)

var fixedAt = time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)

func TestComputePerfectScoreEmptyChecks(t *testing.T) {
	got := healthscore.Compute(nil, fixedAt)
	if got.Value != 100 {
		t.Fatalf("value=%d want 100", got.Value)
	}
	if got.Status != "excellent" {
		t.Fatalf("status=%q want excellent", got.Status)
	}
	if got.Algorithm != healthscore.AlgorithmName {
		t.Fatalf("algorithm=%q", got.Algorithm)
	}
	if got.Max != 100 {
		t.Fatalf("max=%d", got.Max)
	}
}

func TestComputeBlockingDeductions(t *testing.T) {
	got := healthscore.Compute([]healthscore.Check{
		{Code: "a.block", Severity: "blocking"},
	}, fixedAt)
	if got.Value != 75 {
		t.Fatalf("value=%d want 75", got.Value)
	}
	if got.Status != "warning" {
		t.Fatalf("status=%q want warning", got.Status)
	}
}

func TestComputeBlockingCap(t *testing.T) {
	checks := make([]healthscore.Check, 5)
	for i := range checks {
		checks[i] = healthscore.Check{Code: "b" + string(rune('a'+i)), Severity: "blocking"}
	}
	got := healthscore.Compute(checks, fixedAt)
	if got.Value != 0 {
		t.Fatalf("value=%d want 0", got.Value)
	}
	if got.Status != "attention" {
		t.Fatalf("status=%q want attention", got.Status)
	}
	d := findDeduction(got.Deductions, "blocking")
	if d == nil || !d.Capped {
		t.Fatal("expected capped blocking deduction")
	}
}

func TestComputeWarningDeductions(t *testing.T) {
	got := healthscore.Compute([]healthscore.Check{
		{Code: "w1", Severity: "warning"},
		{Code: "w2", Severity: "warning"},
	}, fixedAt)
	if got.Value != 80 {
		t.Fatalf("value=%d want 80", got.Value)
	}
}

func TestComputeWarningCap(t *testing.T) {
	checks := make([]healthscore.Check, 6)
	for i := range checks {
		checks[i] = healthscore.Check{Code: "w" + string(rune('a'+i)), Severity: "warning"}
	}
	got := healthscore.Compute(checks, fixedAt)
	if got.Value != 50 {
		t.Fatalf("value=%d want 50", got.Value)
	}
}

func TestComputeInfoDeductions(t *testing.T) {
	got := healthscore.Compute([]healthscore.Check{
		{Code: "i1", Severity: "info"},
	}, fixedAt)
	if got.Value != 98 {
		t.Fatalf("value=%d want 98", got.Value)
	}
}

func TestComputeInfoCap(t *testing.T) {
	checks := make([]healthscore.Check, 6)
	for i := range checks {
		checks[i] = healthscore.Check{Code: "i" + string(rune('a'+i)), Severity: "info"}
	}
	got := healthscore.Compute(checks, fixedAt)
	if got.Value != 90 {
		t.Fatalf("value=%d want 90", got.Value)
	}
}

func TestComputeDedupeHighestSeverity(t *testing.T) {
	got := healthscore.Compute([]healthscore.Check{
		{Code: "dup", Severity: "warning"},
		{Code: "dup", Severity: "blocking"},
		{Code: "dup", Severity: "info"},
	}, fixedAt)
	if got.Value != 75 {
		t.Fatalf("value=%d want 75 (blocking only)", got.Value)
	}
}

func TestComputeExcludeRuntimeInfoPrefix(t *testing.T) {
	got := healthscore.Compute([]healthscore.Check{
		{Code: "info.runtime.consumer_off", Severity: "info"},
		{Code: "real.info", Severity: "info"},
	}, fixedAt)
	if got.Value != 98 {
		t.Fatalf("value=%d want 98", got.Value)
	}
}

func TestComputeFloorZero(t *testing.T) {
	checks := make([]healthscore.Check, 4)
	for i := range checks {
		checks[i] = healthscore.Check{Code: "x" + string(rune('a'+i)), Severity: "blocking"}
	}
	got := healthscore.Compute(checks, fixedAt)
	if got.Value != 0 {
		t.Fatalf("value=%d want 0", got.Value)
	}
}

func TestMapStatusBands(t *testing.T) {
	cases := []struct {
		checks []healthscore.Check
		status string
	}{
		{nil, "excellent"},
		{[]healthscore.Check{{Code: "b1", Severity: "blocking"}}, "warning"},
		{[]healthscore.Check{{Code: "b1", Severity: "blocking"}, {Code: "b2", Severity: "blocking"}}, "warning"},
		{[]healthscore.Check{{Code: "w1", Severity: "warning"}, {Code: "w2", Severity: "warning"}}, "excellent"},
		{make([]healthscore.Check, 4), "attention"},
	}
	for i, tc := range cases {
		checks := tc.checks
		if i == len(cases)-1 {
			for j := range checks {
				checks[j] = healthscore.Check{Code: "blk" + string(rune('a'+j)), Severity: "blocking"}
			}
		}
		got := healthscore.Compute(checks, fixedAt)
		if got.Status != tc.status {
			t.Fatalf("case %d: status=%q want %q (value=%d)", i, got.Status, tc.status, got.Value)
		}
	}
}

func findDeduction(ds []healthscore.Deduction, sev string) *healthscore.Deduction {
	for i := range ds {
		if ds[i].Severity == sev {
			return &ds[i]
		}
	}
	return nil
}
