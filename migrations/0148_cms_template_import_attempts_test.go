package migrations_test

import (
	"strings"
	"testing"
)

func TestMigration0148ImportAttemptsExpandOnly(t *testing.T) {
	up := readMigration(t, "0148_cms_template_import_attempts.up.sql")
	down := readMigration(t, "0148_cms_template_import_attempts.down.sql")
	runner := readMigration(t, "run_dev_migrations.sh")
	if !strings.Contains(up, "CREATE TABLE IF NOT EXISTS cms_template_import_attempts") {
		t.Fatal("up must create cms_template_import_attempts")
	}
	if strings.Contains(strings.ToLower(up), "validation_token ") || strings.Contains(up, "raw_json") {
		t.Fatal("up must not store a raw token or raw json column")
	}
	if strings.Contains(up, "validation_token_sha256") == false || strings.Contains(up, "file_sha256") == false {
		t.Fatal("up must store hashes")
	}
	drops := strings.Count(strings.ToUpper(down), "DROP TABLE")
	if drops != 1 || !strings.Contains(down, "DROP TABLE IF EXISTS cms_template_import_attempts") {
		t.Fatalf("down must drop only the new table: %s", down)
	}
	if !strings.Contains(runner, "0148_cms_template_import_attempts.up.sql") {
		t.Fatal("runner missing 0148")
	}
	if strings.Index(runner, "0147_workflow_department_email_recipient_backfill.up.sql") > strings.Index(runner, "0148_cms_template_import_attempts.up.sql") {
		t.Fatal("0148 must follow 0147")
	}
}
