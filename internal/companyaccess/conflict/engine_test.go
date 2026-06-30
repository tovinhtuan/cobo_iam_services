package conflict_test

import (
	"testing"
	"time"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	"github.com/cobo/cobo_iam_services/internal/companyaccess/conflict"
)

func TestRegistryDeterministicOrder(t *testing.T) {
	a := conflict.RegistryCodes()
	b := conflict.RegistryCodes()
	if len(a) != len(b) {
		t.Fatalf("registry length mismatch")
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("registry order not deterministic at %d: %q vs %q", i, a[i], b[i])
		}
	}
	expected := []string{
		"conflict.workflow.override_stale",
		"conflict.notification.prefs_invalid",
		"conflict.permission.critical_role_empty",
		"conflict.permission.grantable_violation",
		"conflict.org.department_inactive_referenced",
		"conflict.workflow.assignee_role_missing",
		"conflict.rbac.role_unassigned_in_workflow",
		"conflict.subscription.tier_prefs_mismatch",
	}
	if len(a) != len(expected) {
		t.Fatalf("expected %d rules, got %d", len(expected), len(a))
	}
	for i := range expected {
		if a[i] != expected[i] {
			t.Fatalf("index %d: want %q got %q", i, expected[i], a[i])
		}
	}
}

func TestEngineEmptySnapshot(t *testing.T) {
	engine := conflict.DefaultEngine()
	out := engine.Evaluate(conflict.EvaluationInput{
		CompanyID:   "co-1",
		EvaluatedAt: time.Now().UTC(),
	}, &conflict.ConfigurationSnapshot{CompanyID: "co-1"})
	if len(out.Results) != 0 {
		t.Fatalf("expected no checks, got %d", len(out.Results))
	}
}

func TestEngineAggregatesMultipleRules(t *testing.T) {
	conflict.RegisterValidators(conflict.ValidatorDeps{
		ValidatePrefs: caapp.ValidateAlertChannelPrefsPayload,
	})
	snap := &conflict.ConfigurationSnapshot{
		CompanyID: "co-1",
		AlertChannelPrefsExists: true,
		AlertChannelPrefsRuleID: "nr-1",
		AlertChannelPrefsPayload: map[string]any{
			"channels": map[string]any{},
		},
		StaleWorkflowOverrides: []conflict.StaleWorkflowOverrideRow{{
			TypeID: "t1", StaleStatus: "stale", ActiveVersionNo: 1,
		}},
	}
	out := conflict.DefaultEngine().Evaluate(conflict.EvaluationInput{CompanyID: "co-1"}, snap)
	if len(out.Results) < 2 {
		t.Fatalf("expected at least 2 conflicts, got %d", len(out.Results))
	}
}

func TestMergeSortDeterministic(t *testing.T) {
	results := []conflict.Result{
		{Code: "conflict.b", Severity: conflict.SeverityWarning, ResourceID: "2"},
		{Code: "conflict.a", Severity: conflict.SeverityBlocking, ResourceID: ""},
		{Code: "conflict.a", Severity: conflict.SeverityWarning, ResourceID: "1"},
	}
	merged := conflict.MergeAndSort(results)
	if merged[0].Severity != conflict.SeverityBlocking {
		t.Fatalf("blocking should sort first")
	}
	if merged[0].Code != "conflict.a" {
		t.Fatalf("expected conflict.a first, got %s", merged[0].Code)
	}
}

func TestRuleDoesNotMutateSnapshot(t *testing.T) {
	payload := map[string]any{"channels": map[string]any{}}
	snap := &conflict.ConfigurationSnapshot{
		CompanyID:                "co-1",
		AlertChannelPrefsExists:  true,
		AlertChannelPrefsRuleID:  "nr-1",
		AlertChannelPrefsPayload: payload,
	}
	before := len(payload)
	conflict.DefaultEngine().Evaluate(conflict.EvaluationInput{CompanyID: "co-1"}, snap)
	if len(payload) != before {
		t.Fatal("snapshot payload was mutated")
	}
}

func TestNotificationPrefsInvalidMapsValidator(t *testing.T) {
	conflict.RegisterValidators(conflict.ValidatorDeps{
		ValidatePrefs:         caapp.ValidateAlertChannelPrefsPayload,
		PermissionRiskLevel:   caapp.PermissionRiskLevel,
		IsGrantablePermission: caapp.IsGrantablePermission,
	})
	valid, issues := caapp.ValidateAlertChannelPrefsPayload(map[string]any{"channels": map[string]any{}})
	if valid {
		t.Fatal("expected invalid payload")
	}
	snap := &conflict.ConfigurationSnapshot{
		CompanyID:               "co-1",
		AlertChannelPrefsExists: true,
		AlertChannelPrefsRuleID: "nr-1",
		AlertChannelPrefsPayload: map[string]any{"channels": map[string]any{}},
	}
	out := conflict.DefaultEngine().Evaluate(conflict.EvaluationInput{CompanyID: "co-1"}, snap)
	found := false
	for _, r := range out.Results {
		if r.Code == "conflict.notification.prefs_invalid" {
			found = true
			if r.Severity != conflict.SeverityBlocking {
				t.Fatalf("expected blocking, got %s", r.Severity)
			}
			if iss, ok := r.Evidence["issues"].([]string); !ok || len(iss) != len(issues) {
				t.Fatalf("expected issues in evidence")
			}
		}
	}
	if !found {
		t.Fatal("prefs_invalid conflict not found")
	}
}

func TestOverrideStaleMapsRow(t *testing.T) {
	snap := &conflict.ConfigurationSnapshot{
		CompanyID: "co-1",
		StaleWorkflowOverrides: []conflict.StaleWorkflowOverrideRow{{
			TypeID: "type-1", StaleStatus: "stale", ActiveVersionNo: 2,
		}},
	}
	out := conflict.DefaultEngine().Evaluate(conflict.EvaluationInput{CompanyID: "co-1"}, snap)
	if len(out.Results) != 1 || out.Results[0].Code != "conflict.workflow.override_stale" {
		t.Fatalf("unexpected results: %+v", out.Results)
	}
}

func TestCriticalRoleEmptyWarning(t *testing.T) {
	conflict.RegisterValidators(conflict.ValidatorDeps{
		ValidatePrefs:         caapp.ValidateAlertChannelPrefsPayload,
		PermissionRiskLevel:   caapp.PermissionRiskLevel,
		IsGrantablePermission: caapp.IsGrantablePermission,
	})
	snap := &conflict.ConfigurationSnapshot{
		CompanyID: "co-1",
		Roles: []conflict.RoleSnapshot{{
			RoleID: "r1", RoleCode: "admin", MemberCount: 0, Status: "active",
		}},
		RolePermissionCodes: map[string][]string{
			"r1": {"rbac.manage"},
		},
	}
	out := conflict.DefaultEngine().Evaluate(conflict.EvaluationInput{CompanyID: "co-1"}, snap)
	if len(out.Results) != 1 || out.Results[0].Code != "conflict.permission.critical_role_empty" {
		t.Fatalf("unexpected: %+v", out.Results)
	}
}

func TestUnknownDataHandledSafely(t *testing.T) {
	out := conflict.DefaultEngine().Evaluate(conflict.EvaluationInput{CompanyID: "co-1"}, nil)
	if out.Results == nil {
		t.Fatal("results should be empty slice or nil-safe")
	}
}

func TestCompanyScopingPreserved(t *testing.T) {
	snap := &conflict.ConfigurationSnapshot{
		CompanyID: "co-tenant-a",
		StaleWorkflowOverrides: []conflict.StaleWorkflowOverrideRow{{
			TypeID: "t1", StaleStatus: "stale", ActiveVersionNo: 1,
		}},
	}
	out := conflict.DefaultEngine().Evaluate(conflict.EvaluationInput{CompanyID: "co-tenant-a"}, snap)
	if out.CompanyID != "co-tenant-a" {
		t.Fatalf("company_id not preserved")
	}
	if out.Results[0].CompanyID != "co-tenant-a" {
		t.Fatalf("result company_id not preserved")
	}
}
