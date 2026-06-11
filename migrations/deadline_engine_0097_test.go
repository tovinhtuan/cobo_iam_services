package migrations_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	_ "github.com/go-sql-driver/mysql"
)

func migrationDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Dir(file)
}

func readMigration(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(migrationDir(t), name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(b)
}

// Static guards — always run without MySQL.
func TestMigration0097_UpSQLRequiredSteps(t *testing.T) {
	up := readMigration(t, "0097_deadline_engine_v2_prepare.up.sql")
	for _, fragment := range []string{
		"use_structure_deadline",
		"deadline_days",
		"deadline_day_type",
		"'calendar'",
		"deadline_by_structure.simple_structure.days",
		"company_id IS NULL",
		"active_version_no > 0",
		"applicability_rules_json IS NOT NULL",
		"template_category",
		"deadline_config_json",
	} {
		if !strings.Contains(up, fragment) {
			t.Fatalf("0097 up.sql missing %q", fragment)
		}
	}
	if strings.Contains(up, "DROP COLUMN") {
		t.Fatal("0097 up.sql must not drop columns")
	}
}

func TestMigration0097_DownSQLPreservesStructure(t *testing.T) {
	down := readMigration(t, "0097_deadline_engine_v2_prepare.down.sql")
	for _, fragment := range []string{
		"JSON_REMOVE",
		"$.deadline_days",
		"$.deadline_day_type",
		"$.use_structure_deadline",
	} {
		if !strings.Contains(down, fragment) {
			t.Fatalf("0097 down.sql missing %q", fragment)
		}
	}
	if strings.Contains(down, "$.deadline_by_structure") {
		t.Fatal("down.sql must not remove deadline_by_structure")
	}
}

// MT-03 fallback logic mirror (SQL COALESCE chain).
func TestMigration0097_FallbackDeadlineDays(t *testing.T) {
	tests := []struct {
		name       string
		configDays int
		simpleDays int
		want       int
	}{
		{"config wins", 25, 20, 25},
		{"simple when config zero", 0, 20, 20},
		{"default 30", 0, 0, 30},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveDeadlineDaysFallback(tc.configDays, tc.simpleDays)
			if got != tc.want {
				t.Fatalf("got %d want %d", got, tc.want)
			}
		})
	}
}

func resolveDeadlineDaysFallback(configDays, simpleStructureDays int) int {
	if configDays > 0 {
		return configDays
	}
	if simpleStructureDays > 0 {
		return simpleStructureDays
	}
	return 30
}

func openMySQLForMigrationTest(t *testing.T) *sql.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("MYSQL_TEST_DSN"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("MYSQL_DSN"))
	}
	if dsn == "" {
		t.Skip("MYSQL_TEST_DSN or MYSQL_DSN not set — skipping integration migration tests")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Skipf("mysql open: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Skipf("mysql ping: %v", err)
	}
	return db
}

// MT-01, MT-09: apply 0097 on DB already at >=0095; idempotent second apply.
func TestMigration0097_Integration_ApplyIdempotent(t *testing.T) {
	db := openMySQLForMigrationTest(t)
	defer db.Close()

	up := readMigration(t, "0097_deadline_engine_v2_prepare.up.sql")
	if _, err := db.Exec(up); err != nil {
		t.Fatalf("MT-01 first apply: %v", err)
	}
	if _, err := db.Exec(up); err != nil {
		t.Fatalf("MT-09 second apply: %v", err)
	}
}
