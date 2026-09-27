package migrations_test

import (
	"os"
	"strings"
	"testing"
)

func TestMigration0146And0147StaticContract(t *testing.T) {
	up46 := readMigration(t, "0146_workflow_department_email_delivery_safety.up.sql")
	down46 := readMigration(t, "0146_workflow_department_email_delivery_safety.down.sql")
	up47 := readMigration(t, "0147_workflow_department_email_recipient_backfill.up.sql")
	down47 := readMigration(t, "0147_workflow_department_email_recipient_backfill.down.sql")
	runner := readMigration(t, "run_dev_migrations.sh")
	prior := readMigration(t, "0144_workflow_department_binding.up.sql")

	if strings.Contains(prior, "lease_id") || strings.Contains(prior, "attempt_count") {
		t.Fatal("0144 was modified with delivery-safety columns")
	}
	for _, col := range []string{"lease_id", "lease_until", "updated_at", "attempt_count", "next_retry_at", "last_error_code", "idx_reminder_dispatch_claim"} {
		if !strings.Contains(up46, col) {
			t.Fatalf("0146 up missing %s", col)
		}
	}
	if strings.Contains(up46, "WORKFLOW_DEPARTMENT_EMAIL_BINDING_ENABLED") || strings.Contains(up47, "INSERT INTO reminder_dispatch_resolutions") {
		t.Fatal("migrations must not enable flags or insert delivery rows")
	}
	if !strings.Contains(up47, "WHERE updated_at IS NULL") {
		t.Fatal("0147 must copy updated_at only when null")
	}
	if !strings.Contains(up47, "HAVING COUNT(*) > 1") || !strings.Contains(up47, "uk_reminder_dispatch_occurrence_company") {
		t.Fatal("0147 must count duplicates before creating its unique index")
	}
	signalAt := strings.Index(up47, "SIGNAL SQLSTATE '45000'")
	alterAt := strings.Index(up47, "ADD UNIQUE KEY uk_reminder_dispatch_occurrence_company")
	if signalAt < 0 || alterAt < 0 || signalAt > alterAt {
		t.Fatal("0147 must SIGNAL on duplicates before ADD UNIQUE")
	}
	for _, line := range strings.Split(up47, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") {
			continue
		}
		if strings.Contains(trimmed, "PREPARE") {
			t.Fatal("0147 must not PREPARE SIGNAL; MySQL 8 rejects that statement")
		}
	}
	if strings.Contains(up47, "recipient_email") && strings.Contains(up47, "SELECT recipient") {
		t.Fatal("0147 must not select recipient material")
	}
	if !strings.Contains(down46, "DROP COLUMN lease_id") || strings.Contains(down46, "DROP TABLE") {
		t.Fatal("0146 down must drop owned columns only")
	}
	if !strings.Contains(down47, "uk_reminder_dispatch_occurrence_company") || strings.Contains(down47, "DROP TABLE") {
		t.Fatal("0147 down must drop only its unique index")
	}
	withoutOwned := strings.ReplaceAll(down47, "uk_reminder_dispatch_occurrence_company", "")
	if strings.Contains(withoutOwned, "uk_reminder_dispatch_occurrence") || strings.Contains(down47, "DROP COLUMN") {
		t.Fatal("0147 down must not drop the 0144 unique key or columns")
	}
	idx45 := strings.Index(runner, "0145_catalog_department_code_guard.up.sql")
	idx46 := strings.Index(runner, "0146_workflow_department_email_delivery_safety.up.sql")
	idx47 := strings.Index(runner, "0147_workflow_department_email_recipient_backfill.up.sql")
	if idx45 < 0 || idx46 < 0 || idx47 < 0 || !(idx45 < idx46 && idx46 < idx47) {
		t.Fatal("runner order must be 0145 → 0146 → 0147")
	}
	if _, err := os.Stat(migrationDir(t) + "/0144_workflow_department_binding.up.sql"); err != nil {
		t.Fatal(err)
	}
}
