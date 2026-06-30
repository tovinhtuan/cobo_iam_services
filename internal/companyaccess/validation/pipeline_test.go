package validation_test

import (
	"testing"
	"time"

	"github.com/cobo/cobo_iam_services/internal/companyaccess/conflict"
	"github.com/cobo/cobo_iam_services/internal/companyaccess/validation"
)

func TestPipelineStageOrder(t *testing.T) {
	result := validation.Run(validation.Input{
		CompanyID:   "co-1",
		ValidatedAt: time.Now().UTC(),
		Snapshot:    &conflict.ConfigurationSnapshot{CompanyID: "co-1"},
	})
	if len(result.Suites) != len(validation.StageOrder) {
		t.Fatalf("expected %d suites, got %d", len(validation.StageOrder), len(result.Suites))
	}
	for i, want := range validation.StageOrder {
		if result.Suites[i].Suite != want {
			t.Fatalf("suite[%d]: want %q got %q", i, want, result.Suites[i].Suite)
		}
	}
}

func TestPipelinePassedWithOnlyWarnings(t *testing.T) {
	result := validation.Run(validation.Input{
		CompanyID:   "co-1",
		ValidatedAt: time.Now().UTC(),
		Snapshot: &conflict.ConfigurationSnapshot{
			CompanyID: "co-1",
			NonGrantableDirectPermissions: []conflict.DirectPermissionRow{
				{MembershipID: "m1", PermissionCode: "rbac.manage"},
			},
		},
	})
	if !result.Passed {
		t.Fatal("expected passed=true when only warnings (no blocking)")
	}
	if result.Summary.Blocking != 0 {
		t.Fatalf("expected 0 blocking, got %d", result.Summary.Blocking)
	}
}

func TestPipelineBlockingFailsPassed(t *testing.T) {
	validation.RegisterValidators(validation.ValidatorDeps{
		ValidatePrefs: func(_ map[string]any) (bool, []string) {
			return false, []string{"invalid channel"}
		},
	})
	t.Cleanup(func() { validation.RegisterValidators(validation.ValidatorDeps{}) })

	result := validation.Run(validation.Input{
		CompanyID:   "co-1",
		ValidatedAt: time.Now().UTC(),
		Snapshot: &conflict.ConfigurationSnapshot{
			CompanyID:               "co-1",
			AlertChannelPrefsExists: true,
			AlertChannelPrefsPayload: map[string]any{"channels": map[string]any{}},
		},
	})
	if result.Passed {
		t.Fatal("expected passed=false with blocking schema check")
	}
	found := false
	for _, suite := range result.Suites {
		for _, c := range suite.Checks {
			if c.Code == "schema.notification_prefs_invalid" && c.Severity == validation.SeverityBlocking {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("expected schema.notification_prefs_invalid blocking")
	}
}

func TestConflictStagePassthrough(t *testing.T) {
	conflictOut := conflict.EvaluationOutput{
		Results: []conflict.Result{{
			Code:        "conflict.workflow.override_stale",
			Severity:    conflict.SeverityWarning,
			Description: "stale override",
			ActionLink:  "/app/disclosure-types",
			Evidence:    map[string]any{"type_id": "t1"},
		}},
	}
	result := validation.Run(validation.Input{
		CompanyID:      "co-1",
		ValidatedAt:    time.Now().UTC(),
		Snapshot:       &conflict.ConfigurationSnapshot{CompanyID: "co-1"},
		ConflictOutput: conflictOut,
	})
	var conflictSuite *validation.SuiteResult
	for i := range result.Suites {
		if result.Suites[i].Suite == validation.SuiteConflict {
			conflictSuite = &result.Suites[i]
			break
		}
	}
	if conflictSuite == nil {
		t.Fatal("conflict suite missing")
	}
	if len(conflictSuite.Checks) != 1 {
		t.Fatalf("expected 1 conflict check, got %d", len(conflictSuite.Checks))
	}
	if conflictSuite.Checks[0].Code != "conflict.workflow.override_stale" {
		t.Fatalf("unexpected code %q", conflictSuite.Checks[0].Code)
	}
}

func TestAuditDispatchSkipped(t *testing.T) {
	result := validation.Run(validation.Input{
		CompanyID:   "co-1",
		ValidatedAt: time.Now().UTC(),
		Snapshot:    &conflict.ConfigurationSnapshot{CompanyID: "co-1"},
	})
	for _, suite := range result.Suites {
		if suite.Suite == validation.SuiteAudit || suite.Suite == validation.SuiteDispatch {
			if suite.SkippedReason != "not_implemented_in_workspace" {
				t.Fatalf("%s: want skipped, got %q", suite.Suite, suite.SkippedReason)
			}
		}
	}
}

func TestFilterSuitesConflictOnly(t *testing.T) {
	full := validation.Run(validation.Input{
		CompanyID:   "co-1",
		ValidatedAt: time.Now().UTC(),
		Snapshot:    &conflict.ConfigurationSnapshot{CompanyID: "co-1"},
		ConflictOutput: conflict.EvaluationOutput{
			Results: []conflict.Result{{
				Code:     "conflict.permission.grantable_violation",
				Severity: conflict.SeverityWarning,
				Title:    "grantable",
			}},
		},
	})
	filtered := validation.FilterSuites(full, []string{"conflict"})
	if len(filtered.Suites) != 1 || filtered.Suites[0].Suite != validation.SuiteConflict {
		t.Fatalf("unexpected filtered suites: %+v", filtered.Suites)
	}
}
