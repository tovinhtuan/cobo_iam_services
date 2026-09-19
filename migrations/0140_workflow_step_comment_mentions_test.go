package migrations_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const migration0140Base = "0140_workflow_step_comment_mentions"

func TestMigration0140_StaticContract(t *testing.T) {
	up := readMigration(t, migration0140Base+".up.sql")
	down := readMigration(t, migration0140Base+".down.sql")
	runner := readMigration(t, "run_dev_migrations.sh")

	for _, needle := range []string{
		"CREATE TABLE IF NOT EXISTS workflow_step_comment_mentions",
		"id VARCHAR(36) NOT NULL",
		"company_id VARCHAR(36) NOT NULL",
		"comment_id VARCHAR(36) NOT NULL",
		"mentioned_membership_id VARCHAR(36) NOT NULL",
		"start_offset INT NOT NULL",
		"end_offset INT NOT NULL",
		"created_at DATETIME(3) NOT NULL",
		"PRIMARY KEY (id)",
		"KEY idx_wscm_company_comment",
		"(company_id, comment_id)",
		"KEY idx_wscm_company_mentioned_created",
		"(company_id, mentioned_membership_id, created_at)",
		"KEY idx_wscm_company_id",
		"(company_id, id)",
		"SET NAMES utf8mb4",
		"PRODUCTION_ROLLBACK=forward_fix_or_flag_off",
		"DEV_DOWN=DROP_TABLE_ALLOWED_WITH_CONFIRMATION",
	} {
		if !strings.Contains(up, needle) {
			t.Fatalf("0140 up missing %q", needle)
		}
	}

	for _, forbidden := range []string{
		"FOREIGN KEY",
		"REFERENCES ",
		"UNIQUE KEY",
		"display_name",
		"email",
		"phone",
		"wsc_",
	} {
		if strings.Contains(up, forbidden) {
			t.Fatalf("0140 up must not contain %q", forbidden)
		}
	}

	if !strings.Contains(down, "DROP TABLE IF EXISTS workflow_step_comment_mentions") {
		t.Fatal("0140 down must drop workflow_step_comment_mentions")
	}
	if !strings.Contains(down, "PRODUCTION_ROLLBACK=forward_fix_or_flag_off") {
		t.Fatal("0140 down must document production rollback policy")
	}
	if !strings.Contains(down, "DEV_DOWN=DROP_TABLE_ALLOWED_WITH_CONFIRMATION") {
		t.Fatal("0140 down must document DEV down policy")
	}

	// 0138/0139 must remain untouched on disk.
	for _, prior := range []string{
		"0138_deadline_comment_permission.up.sql",
		"0138_deadline_comment_permission.down.sql",
		"0139_workflow_step_comments.up.sql",
		"0139_workflow_step_comments.down.sql",
	} {
		path := filepath.Join(migrationDir(t), prior)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("prior migration missing (must not delete): %s", prior)
		}
	}

	for _, file := range []string{
		"0138_deadline_comment_permission.up.sql",
		"0139_workflow_step_comments.up.sql",
		migration0140Base + ".up.sql",
	} {
		if !strings.Contains(runner, file) {
			t.Fatalf("run_dev_migrations.sh must list %s", file)
		}
	}
	idx137 := strings.Index(runner, "0137_workflow_step_evidence_files.up.sql")
	idx138 := strings.Index(runner, "0138_deadline_comment_permission.up.sql")
	idx139 := strings.Index(runner, "0139_workflow_step_comments.up.sql")
	idx140 := strings.Index(runner, migration0140Base+".up.sql")
	if idx137 < 0 || idx138 < 0 || idx139 < 0 || idx140 < 0 {
		t.Fatal("runner must include 0137–0140")
	}
	if !(idx137 < idx138 && idx138 < idx139 && idx139 < idx140) {
		t.Fatal("runner order must be 0137 → 0138 → 0139 → 0140")
	}
}
