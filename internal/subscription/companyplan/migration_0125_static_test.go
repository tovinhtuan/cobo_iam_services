package companyplan_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Static migration validation for 0125 + DEV seed (Phase 4 — no DEV apply).
func TestMigration0125_StaticValidation(t *testing.T) {
	root := findMigrationsRoot(t)
	upPath := filepath.Join(root, "0125_company_subscriptions.up.sql")
	downPath := filepath.Join(root, "0125_company_subscriptions.down.sql")
	seedPath := filepath.Join(root, "seed_dev_company_subscriptions.sql")
	runnerPath := filepath.Join(root, "run_dev_migrations.sh")

	up := mustRead(t, upPath)
	down := mustRead(t, downPath)
	seed := mustRead(t, seedPath)
	runner := mustRead(t, runnerPath)

	// up syntax / objects
	for _, needle := range []string{
		"CREATE TABLE company_subscriptions",
		"company_id       VARCHAR(36)",
		"REFERENCES companies (company_id)",
		"idx_company_subscriptions_lookup",
		"idx_company_subscriptions_origin",
		"effective_from   TIMESTAMP    NOT NULL",
		"expires_at       TIMESTAMP    NULL",
		"origin           VARCHAR(64)",
	} {
		if !strings.Contains(up, needle) {
			t.Fatalf("0125 up missing %q", needle)
		}
	}
	if strings.Contains(up, "INSERT INTO") {
		t.Fatal("0125 up must not contain fixture INSERTs")
	}

	// down rollback order: drop table only (FK child first — table drop is correct)
	if !strings.Contains(down, "DROP TABLE IF EXISTS company_subscriptions") {
		t.Fatal("0125 down must drop company_subscriptions")
	}

	// 0126 company status CHECK (Phase 5B): only expected SQL + optional colocated *_test.go
	matches, _ := filepath.Glob(filepath.Join(root, "0126*"))
	for _, m := range matches {
		base := filepath.Base(m)
		ok := base == "0126_companies_status_check_constraints.up.sql" ||
			base == "0126_companies_status_check_constraints.down.sql" ||
			base == "0126_companies_status_check_constraints_test.go"
		if !ok {
			t.Fatalf("unexpected 0126 migration file: %s", base)
		}
	}

	// runner ordering: 0125 then optional 0126 then seed; seed not as numbered migration
	if !strings.Contains(runner, "0125_company_subscriptions.up.sql") {
		t.Fatal("run_dev_migrations.sh must list 0125")
	}
	if !strings.Contains(runner, "seed_dev_company_subscriptions.sql") {
		t.Fatal("run_dev_migrations.sh must list DEV seed")
	}
	idx125 := strings.Index(runner, "0125_company_subscriptions.up.sql")
	idxSeed := strings.Index(runner, "seed_dev_company_subscriptions.sql")
	if idx125 < 0 || idxSeed < 0 || idxSeed < idx125 {
		t.Fatal("seed must follow 0125 in runner list")
	}
	if strings.Contains(runner, "0126_companies_status_check_constraints.up.sql") {
		idx126 := strings.Index(runner, "0126_companies_status_check_constraints.up.sql")
		if idx126 < idx125 || idx126 > idxSeed {
			t.Fatal("0126 must be listed after 0125 and before seed")
		}
	}

	// seed idempotent + cleanup contract
	if !strings.Contains(seed, "ON DUPLICATE KEY UPDATE") {
		t.Fatal("seed must be idempotent")
	}
	if !strings.Contains(seed, "origin = 'dev_fixture'") && !strings.Contains(seed, "'dev_fixture'") {
		t.Fatal("seed must use origin=dev_fixture")
	}
	if !strings.Contains(seed, "DELETE FROM company_subscriptions WHERE origin = 'dev_fixture'") {
		t.Fatal("seed header must document cleanup by origin=dev_fixture")
	}
	if strings.Contains(seed, "UPDATE companies") || strings.Contains(seed, "DELETE FROM companies") {
		t.Fatal("seed must not mutate production-like companies rows beyond subscription fixtures")
	}

	// occupying statuses / half-open documented in domain; schema supports NULL expires_at
	if !regexp.MustCompile(`expires_at\s+TIMESTAMP\s+NULL`).MatchString(up) {
		t.Fatal("expires_at must allow NULL for open-ended windows")
	}
}

func findMigrationsRoot(t *testing.T) string {
	t.Helper()
	candidates := []string{
		"migrations",
		filepath.Join("..", "..", "..", "migrations"),
	}
	// When running from package dir under internal/subscription/companyplan
	wd, _ := os.Getwd()
	candidates = append(candidates,
		filepath.Join(wd, "migrations"),
		filepath.Join(wd, "..", "..", "..", "migrations"),
		filepath.Join(wd, "..", "..", "..", "..", "migrations"),
	)
	for _, c := range candidates {
		if st, err := os.Stat(filepath.Join(c, "0125_company_subscriptions.up.sql")); err == nil && !st.IsDir() {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	t.Fatal("migrations root not found")
	return ""
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}
