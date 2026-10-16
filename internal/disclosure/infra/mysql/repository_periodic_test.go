package mysql

import (
	"os"
	"strings"
	"testing"
)

func TestListActivePeriodicTypesSQLUsesActiveVersionNo(t *testing.T) {
	t.Helper()
	data, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatalf("read repository.go: %v", err)
	}
	src := string(data)
	if strings.Contains(src, "dtv.is_active") {
		t.Fatal("periodic type listing must not reference dtv.is_active (use dt.active_version_no)")
	}
	if !strings.Contains(src, "dtv.version_no = dt.active_version_no") {
		t.Fatal("expected join on disclosure_types.active_version_no")
	}
}

func TestListActivePeriodicTypesSQLIncludesDailyWeekly(t *testing.T) {
	data, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatalf("read repository.go: %v", err)
	}
	src := string(data)
	if !strings.Contains(src, "'daily'") || !strings.Contains(src, "'weekly'") {
		t.Fatal("periodic type listing must include daily and weekly frequency_unit values")
	}
}

func TestListActivePeriodicTypesSQLIncludesApplicableFrom(t *testing.T) {
	data, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatalf("read repository.go: %v", err)
	}
	src := string(data)
	if !strings.Contains(src, "applicable_from_mode") || !strings.Contains(src, "applicable_from_slot") {
		t.Fatal("periodic type listing must extract applicable_from_mode/slot from ACTIVE deadline_config_json")
	}
	if !strings.Contains(src, "dtv.version_no = dt.active_version_no") {
		t.Fatal("applicable_from must come from active version join")
	}
}

func TestListActivePeriodicTypesSQLIncludesApplicableTo(t *testing.T) {
	data, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatalf("read repository.go: %v", err)
	}
	src := string(data)
	if !strings.Contains(src, "applicable_to") {
		t.Fatal("periodic type listing must extract applicable_to from ACTIVE deadline_config_json")
	}
	if !strings.Contains(src, "dtv.version_no = dt.active_version_no") {
		t.Fatal("applicable_to must come from active version join")
	}
}

func TestListActivePeriodicTypesSQLIncludesPeriodicCycleGenerationLeadDays(t *testing.T) {
	data, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatalf("read repository.go: %v", err)
	}
	src := string(data)
	if !strings.Contains(src, "periodic_cycle_generation_lead_days") {
		t.Fatal("periodic type listing must extract periodic_cycle_generation_lead_days from ACTIVE deadline_config_json")
	}
}

func TestPeriodicMaterializationSQLPersistsRetryState(t *testing.T) {
	data, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatalf("read repository.go: %v", err)
	}
	src := string(data)
	for _, want := range []string{
		"materialization_state = 'PENDING' AND pc.materialized_at IS NULL",
		"materialization_state = 'RETRY' AND pc.next_attempt_at <= NOW(3)",
		"materialization_state = 'CLAIMED'",
		"func (r *Repository) MarkPeriodicCycleRetry",
		"func (r *Repository) MarkPeriodicCycleFailed",
		"Keep the legacy claim marker",
		"attempt_record_id = ?",
		"materialization_state = 'COMPLETED'",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("periodic retry state SQL missing %q", want)
		}
	}
}

// PER-01: FAILED must keep the legacy claim fence (materialized_at set). A worker binary older
// than cabbecf selects record_id IS NULL AND materialized_at IS NULL and would otherwise pick a
// FAILED cycle up again and create a Draft every tick.
func TestMarkPeriodicCycleFailedKeepsLegacyClaimFence(t *testing.T) {
	src := readGlobalWorkflowReadTestSrc(t, "repository.go")
	body := extractFunc(t, src, "func (r *Repository) MarkPeriodicCycleFailed(")
	if strings.Contains(body, "materialized_at = NULL") {
		t.Fatal("MarkPeriodicCycleFailed must not clear materialized_at (legacy claim fence)")
	}
	if !strings.Contains(body, "materialized_at = COALESCE(materialized_at, NOW(3))") {
		t.Fatal("MarkPeriodicCycleFailed must keep or set materialized_at")
	}
}

// PERF-46/PER-07: a released claim must be selectable again. PENDING with materialized_at set
// matches neither selector branch, so the cycle was stuck forever.
func TestReleasePeriodicCycleClaimRequeuesAsRetry(t *testing.T) {
	src := readGlobalWorkflowReadTestSrc(t, "repository.go")
	body := extractFunc(t, src, "func (r *Repository) ReleasePeriodicCycleClaim(")
	for _, want := range []string{"materialization_state = 'RETRY'", "next_attempt_at = NOW(3)", "materialized_at = NOW(3)"} {
		if !strings.Contains(body, want) {
			t.Fatalf("ReleasePeriodicCycleClaim must contain %q", want)
		}
	}
	if strings.Contains(body, "materialization_state = 'PENDING'") {
		t.Fatal("ReleasePeriodicCycleClaim must not set PENDING with materialized_at (unselectable)")
	}
}

// PER-01: the data fix ships with the runner list.
func TestMigration0153InDevRunner(t *testing.T) {
	data, err := os.ReadFile("../../../../migrations/run_dev_migrations.sh")
	if err != nil {
		t.Fatalf("read runner: %v", err)
	}
	if !strings.Contains(string(data), "0153_periodic_cycles_failed_claim_fence.up.sql") {
		t.Fatal("0153_periodic_cycles_failed_claim_fence.up.sql must be in migrations/run_dev_migrations.sh")
	}
	if _, err := os.Stat("../../../../migrations/0153_periodic_cycles_failed_claim_fence.up.sql"); err != nil {
		t.Fatalf("migration file: %v", err)
	}
}

// MR-3: the data fix that suspends cycles of templates archived before the release ships with the
// runner list.
func TestMigration0154InDevRunner(t *testing.T) {
	data, err := os.ReadFile("../../../../migrations/run_dev_migrations.sh")
	if err != nil {
		t.Fatalf("read runner: %v", err)
	}
	if !strings.Contains(string(data), "0154_periodic_cycles_suspend_archived_templates.up.sql") {
		t.Fatal("0154_periodic_cycles_suspend_archived_templates.up.sql must be in migrations/run_dev_migrations.sh")
	}
	if _, err := os.Stat("../../../../migrations/0154_periodic_cycles_suspend_archived_templates.up.sql"); err != nil {
		t.Fatalf("migration file: %v", err)
	}
}
