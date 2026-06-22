package app

import "testing"

func intPtr(v int) *int { return &v }

// TestComputeStaleStatus covers all 7 cases required by the task spec — the staleness comparator
// is a pure function, so every case is directly testable without DB/HTTP plumbing.
func TestComputeStaleStatus(t *testing.T) {
	cases := []struct {
		name           string
		meta           OverrideStalenessMetadata
		currentGlobal  *int
		wantStatus     string
		wantAnomalyNonEmpty bool
	}{
		{
			name:       "no override",
			meta:       OverrideStalenessMetadata{HasOverride: false},
			wantStatus: StaleStatusUnknown,
		},
		{
			name: "unknown base",
			meta: OverrideStalenessMetadata{
				HasOverride: true,
				BaseSource:  BaseSourceUnknown,
			},
			currentGlobal: intPtr(3),
			wantStatus:    StaleStatusUnknown,
		},
		{
			name: "global workflow base, current (version equal)",
			meta: OverrideStalenessMetadata{
				HasOverride:   true,
				BaseSource:    BaseSourceGlobalWorkflow,
				BaseVersionNo: intPtr(2),
			},
			currentGlobal: intPtr(2),
			wantStatus:    StaleStatusCurrent,
		},
		{
			name: "global workflow base, stale (base < current)",
			meta: OverrideStalenessMetadata{
				HasOverride:   true,
				BaseSource:    BaseSourceGlobalWorkflow,
				BaseVersionNo: intPtr(2),
			},
			currentGlobal: intPtr(6),
			wantStatus:    StaleStatusStale,
		},
		{
			name: "base version greater than current — anomaly, conservative unknown",
			meta: OverrideStalenessMetadata{
				HasOverride:   true,
				BaseSource:    BaseSourceGlobalWorkflow,
				BaseVersionNo: intPtr(9),
			},
			currentGlobal:       intPtr(2),
			wantStatus:          StaleStatusUnknown,
			wantAnomalyNonEmpty: true,
		},
		{
			name: "legacy base with active global workflow now existing -> stale",
			meta: OverrideStalenessMetadata{
				HasOverride: true,
				BaseSource:  BaseSourceGlobalTemplate,
			},
			currentGlobal: intPtr(1),
			wantStatus:    StaleStatusStale,
		},
		{
			name: "legacy base, no active global workflow -> conservative unknown",
			meta: OverrideStalenessMetadata{
				HasOverride: true,
				BaseSource:  BaseSourceGlobalTemplate,
			},
			currentGlobal: nil,
			wantStatus:    StaleStatusUnknown,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, anomaly := ComputeStaleStatus(c.meta, c.currentGlobal)
			if status != c.wantStatus {
				t.Errorf("status = %q, want %q", status, c.wantStatus)
			}
			if c.wantAnomalyNonEmpty && anomaly == "" {
				t.Errorf("expected a non-empty anomaly, got empty")
			}
			if !c.wantAnomalyNonEmpty && anomaly != "" {
				t.Errorf("expected no anomaly, got %q", anomaly)
			}
		})
	}
}

// TestComputeStaleStatus_GlobalWorkflowBaseButNoCurrentGlobal covers the edge case where a
// type's global workflow was later archived/removed entirely (no current active version at
// all) — must not guess "current" or "stale", since neither can be proven.
func TestComputeStaleStatus_GlobalWorkflowBaseButNoCurrentGlobal(t *testing.T) {
	status, anomaly := ComputeStaleStatus(OverrideStalenessMetadata{
		HasOverride:   true,
		BaseSource:    BaseSourceGlobalWorkflow,
		BaseVersionNo: intPtr(1),
	}, nil)
	if status != StaleStatusUnknown {
		t.Errorf("status = %q, want %q", status, StaleStatusUnknown)
	}
	if anomaly != "" {
		t.Errorf("expected no anomaly for this case, got %q", anomaly)
	}
}

// TestComputeStaleStatus_NeverGuessesFromUnknownBase is an explicit regression guard for the
// task's "do not guess" rule: an unknown base must NEVER resolve to current/stale, regardless of
// what the current global version is (including matching what a backfilled global_workflow row
// might coincidentally look like).
func TestComputeStaleStatus_NeverGuessesFromUnknownBase(t *testing.T) {
	for _, current := range []*int{nil, intPtr(0), intPtr(1), intPtr(99)} {
		status, _ := ComputeStaleStatus(OverrideStalenessMetadata{
			HasOverride: true,
			BaseSource:  BaseSourceUnknown,
		}, current)
		if status != StaleStatusUnknown {
			t.Errorf("with current=%v: status = %q, want %q (must never guess from unknown base)", current, status, StaleStatusUnknown)
		}
	}
}
