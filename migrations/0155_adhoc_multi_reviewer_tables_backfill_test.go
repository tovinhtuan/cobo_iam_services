package migrations_test

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

const migration0155Base = "0155_adhoc_multi_reviewer_tables_backfill"

// Files that exist in migrations/ but must stay out of the runner; reasons are in the comment
// above the list in run_dev_migrations.sh.
var runnerExcludedMigrations = map[string]bool{
	"0038_system_worker_membership.up.sql": true,
	"0098_adhoc_multi_reviewer.up.sql":     true,
}

// runnerList parses the list the way deploy-dev.sh does: lines inside the
// MIGRATIONS="..." block that end in .sql.
func runnerList(t *testing.T) []string {
	t.Helper()
	var out []string
	in := false
	for _, line := range strings.Split(readMigration(t, "run_dev_migrations.sh"), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, `MIGRATIONS="`):
			in = true
		case in && strings.HasPrefix(line, `"`):
			return out
		case in && strings.HasSuffix(line, ".sql"):
			out = append(out, line)
		}
	}
	t.Fatal("MIGRATIONS block not found in run_dev_migrations.sh")
	return nil
}

func TestRunnerListsEveryUpMigration(t *testing.T) {
	listed := map[string]bool{}
	for _, f := range runnerList(t) {
		if listed[f] {
			t.Errorf("runner lists %s twice", f)
		}
		listed[f] = true
		if _, err := os.Stat(migrationDir(t) + "/" + f); err != nil {
			t.Errorf("runner lists %s, which is not in migrations/: %v", f, err)
		}
	}
	entries, err := os.ReadDir(migrationDir(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		switch {
		case runnerExcludedMigrations[name] && listed[name]:
			t.Errorf("%s is excluded on purpose but listed in run_dev_migrations.sh", name)
		case !runnerExcludedMigrations[name] && !listed[name]:
			t.Errorf("%s is not listed in run_dev_migrations.sh, so neither the runner nor deploy-dev.sh applies it", name)
		}
	}
}

func TestRunnerOrderOfLateListedMigrations(t *testing.T) {
	pos := map[string]int{}
	for i, f := range runnerList(t) {
		pos[f] = i
	}
	for _, seq := range [][]string{
		{"0097_deadline_engine_v2_prepare.up.sql", "0099_workflow_step_instructions.up.sql", "0100_workflow_step_key.up.sql"},
		{"0149_cms_template_import_attempt_lease.up.sql", "0150_template_builder_oauth_authorization_codes.up.sql", "0151_periodic_cycle_materialization_state.up.sql"},
		{"0153_periodic_cycles_failed_claim_fence.up.sql", migration0155Base + ".up.sql", "seed_dev_company_subscriptions.sql"},
	} {
		for i := 1; i < len(seq); i++ {
			a, okA := pos[seq[i-1]]
			b, okB := pos[seq[i]]
			if !okA || !okB || a >= b {
				t.Errorf("runner must list %s before %s", seq[i-1], seq[i])
			}
		}
	}
}

func TestRunnerPreflightFor0099(t *testing.T) {
	runner := readMigration(t, "run_dev_migrations.sh")
	// 0099 is a plain ADD COLUMN: an env that has the column without a ledger row must be
	// marked, not re-applied (ERROR 1060 would stop the runner).
	if !regexp.MustCompile(`0099_workflow_step_instructions\.up\.sql\)\s*\n[^\n]*\n\s*if \[ "\$\(column_exists global_workflow_steps instructions\)" -gt 0 \]`).MatchString(runner) {
		t.Fatal("preflight_schema_drift must mark 0099 ledger-only when global_workflow_steps.instructions exists")
	}
}

func TestMigration0155ReplacesBroken0098Backfill(t *testing.T) {
	old := readMigration(t, "0098_adhoc_multi_reviewer.up.sql")
	up := readMigration(t, migration0155Base+".up.sql")
	down := readMigration(t, migration0155Base+".down.sql")

	// Same DDL as 0098, so a DB built from the runner matches DEV, where 0098 created them.
	ddl := regexp.MustCompile(`(?s)CREATE TABLE IF NOT EXISTS ad_hoc_proposal_\w+ \(.*?\) ENGINE=InnoDB[^;]*;`).FindAllString(old, -1)
	if len(ddl) != 2 {
		t.Fatalf("expected 2 CREATE TABLE statements in 0098, got %d", len(ddl))
	}
	for _, stmt := range ddl {
		if !strings.Contains(up, stmt) {
			t.Errorf("0155 must repeat 0098 DDL verbatim:\n%s", stmt)
		}
	}

	if !strings.Contains(up, "SELECT p.proposal_id, p.company_id, p.process_controller_id, 0, NOW(3)") {
		t.Error("0155 backfill must select proposal_id (ad_hoc_proposals has no id column)")
	}
	if regexp.MustCompile(`(?i)SELECT\s+id\s*,`).MatchString(up) {
		t.Error("0155 must not repeat the broken `SELECT id` from 0098")
	}
	if !strings.Contains(up, "AND NOT EXISTS (\n    SELECT 1 FROM ad_hoc_proposal_reviewers r WHERE r.proposal_id = p.proposal_id") {
		t.Error("0155 backfill must skip proposals that already have reviewers (keeps the quorum, makes reruns no-ops)")
	}
	upper := strings.ToUpper(up)
	for _, kw := range []string{"DROP TABLE", "DELETE FROM", "UPDATE ", "ALTER TABLE", "TRUNCATE"} {
		if strings.Contains(upper, kw) {
			t.Errorf("0155 up must be additive only; found %q", strings.TrimSpace(kw))
		}
	}
	if strings.Contains(strings.ToUpper(down), "DROP TABLE") || strings.Contains(strings.ToUpper(down), "DELETE FROM") {
		t.Error("0155 down must not drop the tables or delete reviewer rows")
	}
}
