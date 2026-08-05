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

const migration0126Base = "0126_companies_status_check_constraints"

func TestMigration0126_StaticContract(t *testing.T) {
	up := readMigration(t, migration0126Base+".up.sql")
	down := readMigration(t, migration0126Base+".down.sql")
	runner := readMigration(t, "run_dev_migrations.sh")

	for _, needle := range []string{
		"ALTER TABLE companies",
		"ADD CONSTRAINT chk_companies_status_valid",
		"CHECK (status COLLATE utf8mb4_bin IN ('active', 'inactive'))",
		"ADD CONSTRAINT chk_companies_verification_status_valid",
		"CHECK (verification_status COLLATE utf8mb4_bin IN ('verified', 'unverified'))",
		"utf8mb4_bin",
	} {
		if !strings.Contains(up, needle) {
			t.Fatalf("0126 up missing %q", needle)
		}
	}

	for _, needle := range []string{
		"LOWER(",
		"TRIM(",
		"UPDATE companies",
		"INSERT INTO",
		"DELETE FROM",
		"DEFAULT '",
		"ALGORITHM=",
		"LOCK=",
		"NOT ENFORCED",
		"MODIFY COLUMN",
		"CHANGE COLUMN",
		"ADD INDEX",
		"ADD KEY",
		"CREATE INDEX",
	} {
		if strings.Contains(up, needle) {
			t.Fatalf("0126 up must not contain %q", needle)
		}
	}
	// Bare IN without bin collation is insufficient under utf8mb4_unicode_ci (ACTIVE≈active).
	if strings.Contains(up, "CHECK (status IN (") || strings.Contains(up, "CHECK (verification_status IN (") {
		t.Fatal("0126 up must use COLLATE utf8mb4_bin for exact canonical enforcement")
	}

	for _, needle := range []string{
		"ALTER TABLE companies",
		"DROP CHECK chk_companies_status_valid",
		"DROP CHECK chk_companies_verification_status_valid",
	} {
		if !strings.Contains(down, needle) {
			t.Fatalf("0126 down missing %q", needle)
		}
	}
	for _, needle := range []string{"UPDATE companies", "DROP COLUMN", "MODIFY COLUMN", "ADD CONSTRAINT"} {
		if strings.Contains(down, needle) {
			t.Fatalf("0126 down must not contain %q", needle)
		}
	}

	if !strings.Contains(runner, migration0126Base+".up.sql") {
		t.Fatal("run_dev_migrations.sh must list 0126")
	}
	idx125 := strings.Index(runner, "0125_company_subscriptions.up.sql")
	idx126 := strings.Index(runner, migration0126Base+".up.sql")
	if idx125 < 0 || idx126 < 0 || idx126 < idx125 {
		t.Fatal("0126 must follow 0125 in run_dev_migrations.sh")
	}
}

func rewrite0126ForFixture(sqlText, table, chkStatus, chkVer string) string {
	sqlText = strings.ReplaceAll(sqlText, "SET NAMES utf8mb4;", "")
	sqlText = strings.ReplaceAll(sqlText, "ALTER TABLE companies", "ALTER TABLE "+table)
	sqlText = strings.ReplaceAll(sqlText, "chk_companies_status_valid", chkStatus)
	sqlText = strings.ReplaceAll(sqlText, "chk_companies_verification_status_valid", chkVer)
	return strings.TrimSpace(sqlText)
}

// TestMigration0126_Integration_EphemeralOptional runs only when MYSQL_TEST_DSN is set.
// Does NOT fall back to MYSQL_DSN / shared DEV — Phase 5B forbids shared DEV mutation.
func TestMigration0126_Integration_EphemeralOptional(t *testing.T) {
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

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	table := "t0126_" + suffix
	chkStatus := "chk_t0126_status_" + suffix
	chkVer := "chk_t0126_ver_" + suffix

	_, err = db.Exec(fmt.Sprintf(`
CREATE TABLE %s (
  company_id VARCHAR(36) NOT NULL PRIMARY KEY,
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  verification_status VARCHAR(32) NOT NULL DEFAULT 'verified'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
`, table))
	if err != nil {
		t.Fatalf("create fixture table: %v", err)
	}
	defer func() { _, _ = db.Exec("DROP TABLE IF EXISTS " + table) }()

	_, err = db.Exec(fmt.Sprintf(
		`INSERT INTO %s (company_id, status, verification_status) VALUES ('c1','active','verified'),('c2','inactive','unverified')`,
		table))
	if err != nil {
		t.Fatalf("seed valid rows: %v", err)
	}

	up := rewrite0126ForFixture(readMigration(t, migration0126Base+".up.sql"), table, chkStatus, chkVer)
	if _, err := db.Exec(up); err != nil {
		t.Fatalf("UP apply: %v", err)
	}

	var constraintCount int
	err = db.QueryRow(`
SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
WHERE CONSTRAINT_SCHEMA = DATABASE()
  AND TABLE_NAME = ?
  AND CONSTRAINT_TYPE = 'CHECK'
  AND CONSTRAINT_NAME IN (?, ?)
`, table, chkStatus, chkVer).Scan(&constraintCount)
	if err != nil {
		t.Fatalf("constraint query: %v", err)
	}
	if constraintCount != 2 {
		t.Fatalf("want 2 CHECK constraints, got %d", constraintCount)
	}

	for i, st := range []string{"active", "inactive"} {
		for j, ver := range []string{"verified", "unverified"} {
			id := fmt.Sprintf("ok-%d-%d", i, j)
			_, err := db.Exec(fmt.Sprintf(
				`INSERT INTO %s (company_id, status, verification_status) VALUES (?,?,?)`, table),
				id, st, ver)
			if err != nil {
				t.Fatalf("valid insert %s/%s: %v", st, ver, err)
			}
		}
	}

	for i, st := range []string{"suspended", "pending", "ACTIVE", " active ", "", "unknown"} {
		_, err := db.Exec(fmt.Sprintf(
			`INSERT INTO %s (company_id, status, verification_status) VALUES (?,?,?)`, table),
			fmt.Sprintf("bad-st-%d", i), st, "verified")
		if err == nil {
			t.Fatalf("expected CHECK reject status %q", st)
		}
	}

	for i, ver := range []string{"verifiedcvx", "pending", "VERIFIED", " verified ", "", "unknown"} {
		_, err := db.Exec(fmt.Sprintf(
			`INSERT INTO %s (company_id, status, verification_status) VALUES (?,?,?)`, table),
			fmt.Sprintf("bad-ver-%d", i), "active", ver)
		if err == nil {
			t.Fatalf("expected CHECK reject verification_status %q", ver)
		}
	}

	var beforeCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&beforeCount); err != nil {
		t.Fatalf("count before down: %v", err)
	}

	down := rewrite0126ForFixture(readMigration(t, migration0126Base+".down.sql"), table, chkStatus, chkVer)
	if _, err := db.Exec(down); err != nil {
		t.Fatalf("DOWN apply: %v", err)
	}

	err = db.QueryRow(`
SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
WHERE CONSTRAINT_SCHEMA = DATABASE()
  AND TABLE_NAME = ?
  AND CONSTRAINT_TYPE = 'CHECK'
  AND CONSTRAINT_NAME IN (?, ?)
`, table, chkStatus, chkVer).Scan(&constraintCount)
	if err != nil {
		t.Fatalf("constraint query after down: %v", err)
	}
	if constraintCount != 0 {
		t.Fatalf("want 0 CHECK after DOWN, got %d", constraintCount)
	}

	var afterCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&afterCount); err != nil {
		t.Fatalf("count after down: %v", err)
	}
	if afterCount != beforeCount {
		t.Fatalf("row count drifted: before=%d after=%d", beforeCount, afterCount)
	}

	_, err = db.Exec(fmt.Sprintf(
		`INSERT INTO %s (company_id, status, verification_status) VALUES ('post-down','ACTIVE','verifiedcvx')`, table))
	if err != nil {
		t.Fatalf("post-DOWN junk insert should succeed: %v", err)
	}

	var defStatus, defVer, nullableStatus, nullableVer string
	err = db.QueryRow(`
SELECT COLUMN_DEFAULT, IS_NULLABLE FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = 'status'
`, table).Scan(&defStatus, &nullableStatus)
	if err != nil {
		t.Fatalf("status column meta: %v", err)
	}
	err = db.QueryRow(`
SELECT COLUMN_DEFAULT, IS_NULLABLE FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = 'verification_status'
`, table).Scan(&defVer, &nullableVer)
	if err != nil {
		t.Fatalf("verification_status column meta: %v", err)
	}
	if defStatus != "active" || defVer != "verified" {
		t.Fatalf("defaults drifted: status=%q verification=%q", defStatus, defVer)
	}
	if nullableStatus != "NO" || nullableVer != "NO" {
		t.Fatalf("nullability drifted: status=%s verification=%s", nullableStatus, nullableVer)
	}
}
