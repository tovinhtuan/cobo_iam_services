package migrations_test

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

const migration0129Base = "0129_workflow_step_milestones_milestone_type_varchar"

func TestMigration0129_StaticContract(t *testing.T) {
	up := readMigration(t, migration0129Base+".up.sql")
	down := readMigration(t, migration0129Base+".down.sql")
	runner := readMigration(t, "run_dev_migrations.sh")

	for _, needle := range []string{
		"ALTER TABLE workflow_step_milestones",
		"MODIFY COLUMN milestone_type VARCHAR(64)",
		"CHARACTER SET utf8mb4",
		"COLLATE utf8mb4_unicode_ci",
		"NOT NULL",
		"REGEXP_SUBSTR(milestone_id, 'due_minus_[1-9][0-9]*d')",
		"WHERE milestone_type = ''",
	} {
		if !strings.Contains(up, needle) {
			t.Fatalf("0129 up missing %q", needle)
		}
	}
	for _, needle := range []string{
		"ENUM(",
		"DROP TABLE",
		"DROP COLUMN",
		"ADD INDEX",
		"CREATE INDEX",
		"DELETE FROM",
	} {
		if strings.Contains(up, needle) {
			t.Fatalf("0129 up must not contain %q", needle)
		}
	}
	if !strings.Contains(down, "SELECT 1") {
		t.Fatal("0129 down must be a documented no-op")
	}
	if strings.Contains(down, "ENUM(") || strings.Contains(down, "MODIFY COLUMN") {
		t.Fatal("0129 down must not restore ENUM")
	}
	if !strings.Contains(runner, migration0129Base+".up.sql") {
		t.Fatal("run_dev_migrations.sh must list 0129")
	}
	idx128 := strings.Index(runner, "0128_workflow_task_assignees.up.sql")
	idx129 := strings.Index(runner, migration0129Base+".up.sql")
	if idx128 < 0 || idx129 < 0 || idx129 < idx128 {
		t.Fatal("0129 must follow 0128 in run_dev_migrations.sh")
	}
}

// TestMigration0129_Integration_EphemeralOptional runs only when MYSQL_TEST_DSN is set.
func TestMigration0129_Integration_EphemeralOptional(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("MYSQL_TEST_DSN"))
	if dsn == "" {
		t.Skip("MYSQL_TEST_DSN not set — static contract only (no shared DEV)")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("mysql open: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		t.Fatalf("mysql ping: %v", err)
	}

	table := fmt.Sprintf("t0129_%d", time.Now().UnixNano())
	_, err = db.Exec(fmt.Sprintf(`
CREATE TABLE %s (
  milestone_id VARCHAR(80) NOT NULL,
  milestone_type ENUM('before_start_5d','before_start_3d','before_start_1d','step_start','step_end')
    COLLATE utf8mb4_unicode_ci NOT NULL,
  PRIMARY KEY (milestone_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
`, table))
	if err != nil {
		t.Fatalf("create fixture: %v", err)
	}
	defer func() { _, _ = db.Exec("DROP TABLE IF EXISTS " + table) }()

	_, err = db.Exec(fmt.Sprintf(
		`INSERT INTO %s (milestone_id, milestone_type) VALUES
		 ('id_step_start_aaaa','step_start'),
		 ('id_step_end_bbbb','step_end'),
		 ('id_before_start_5d_cccc','before_start_5d'),
		 ('id_before_start_3d_dddd','before_start_3d'),
		 ('id_before_start_1d_eeee','before_start_1d')`, table))
	if err != nil {
		t.Fatalf("insert legacy: %v", err)
	}

	_, err = db.Exec("SET SESSION sql_mode = ''")
	if err != nil {
		t.Fatalf("relax sql_mode: %v", err)
	}
	_, err = db.Exec(fmt.Sprintf(
		`INSERT INTO %s (milestone_id, milestone_type) VALUES ('inst_step_due_minus_7d_ffff','due_minus_7d')`, table))
	if err != nil {
		t.Fatalf("insert truncated due_minus: %v", err)
	}

	up := readMigration(t, migration0129Base+".up.sql")
	up = strings.ReplaceAll(up, "workflow_step_milestones", table)
	for _, stmt := range splitSQLStatements(up) {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("apply rewritten 0129 on %s: %v\nstmt=%s", table, err, stmt)
		}
	}

	var colType, nullable string
	err = db.QueryRow(`
SELECT COLUMN_TYPE, IS_NULLABLE
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = 'milestone_type'
`, table).Scan(&colType, &nullable)
	if err != nil {
		t.Fatalf("column meta: %v", err)
	}
	if !strings.Contains(strings.ToLower(colType), "varchar(64)") {
		t.Fatalf("column_type=%q want varchar(64)", colType)
	}
	if nullable != "NO" {
		t.Fatalf("nullable=%s want NO", nullable)
	}

	want := map[string]string{
		"id_step_start_aaaa":            "step_start",
		"id_step_end_bbbb":              "step_end",
		"id_before_start_5d_cccc":       "before_start_5d",
		"id_before_start_3d_dddd":       "before_start_3d",
		"id_before_start_1d_eeee":       "before_start_1d",
		"inst_step_due_minus_7d_ffff":   "due_minus_7d",
	}
	for id, typeWant := range want {
		var got string
		if err := db.QueryRow(fmt.Sprintf("SELECT milestone_type FROM %s WHERE milestone_id=?", table), id).Scan(&got); err != nil {
			t.Fatalf("select %s: %v", id, err)
		}
		if got != typeWant {
			t.Fatalf("%s milestone_type=%q want %q", id, got, typeWant)
		}
	}

	for _, mtype := range []string{
		"due_minus_1d", "due_minus_2d", "due_minus_3d", "due_minus_5d", "due_minus_7d", "due_minus_90d",
	} {
		id := "new_" + mtype
		if _, err := db.Exec(fmt.Sprintf("INSERT INTO %s (milestone_id, milestone_type) VALUES (?, ?)", table), id, mtype); err != nil {
			t.Fatalf("insert %s: %v", mtype, err)
		}
		var got string
		if err := db.QueryRow(fmt.Sprintf("SELECT milestone_type FROM %s WHERE milestone_id=?", table), id).Scan(&got); err != nil {
			t.Fatalf("read %s: %v", mtype, err)
		}
		if got != mtype {
			t.Fatalf("persisted %s as %q", mtype, got)
		}
	}
}

func splitSQLStatements(sqlText string) []string {
	var out []string
	var b strings.Builder
	for _, line := range strings.Split(sqlText, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "--") {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
		if strings.HasSuffix(trim, ";") {
			stmt := strings.TrimSpace(b.String())
			if stmt != "" {
				out = append(out, stmt)
			}
			b.Reset()
		}
	}
	if rest := strings.TrimSpace(b.String()); rest != "" {
		out = append(out, rest)
	}
	return out
}
