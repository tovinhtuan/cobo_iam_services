package inventory_test

import (
	"strings"
	"testing"

	inventory "github.com/cobo/cobo_iam_services/internal/disclosure/app/legal_basis_inventory"
)

func TestValidateReadOnlySQL_Allowlist(t *testing.T) {
	ok := []string{
		"SELECT 1",
		"  SELECT VERSION()  ",
		"SHOW GRANTS",
		"DESCRIBE disclosure_type_versions",
		"DESC disclosure_types",
		"EXPLAIN SELECT 1",
		"START TRANSACTION READ ONLY",
		"BEGIN",
		"COMMIT",
		"ROLLBACK",
		"SET TRANSACTION READ ONLY",
		"/* c */ SELECT a FROM t WHERE id=1",
		"SELECT * FROM t LIMIT 10;",
	}
	for _, q := range ok {
		if err := inventory.ValidateReadOnlySQL(q); err != nil {
			t.Fatalf("expected allow %q: %v", q, err)
		}
	}
}

func TestValidateReadOnlySQL_Forbidden(t *testing.T) {
	bad := []string{
		"INSERT INTO t VALUES (1)",
		"UPDATE t SET a=1",
		"DELETE FROM t",
		"ALTER TABLE t ADD c INT",
		"CREATE TABLE x (i INT)",
		"DROP TABLE t",
		"TRUNCATE t",
		"LOCK TABLES t READ",
		"SELECT * FROM t FOR UPDATE",
		"SELECT * FROM t FOR SHARE",
		"SELECT 1; SELECT 2",
		"CALL proc()",
		"REPLACE INTO t VALUES (1)",
		"GRANT SELECT ON *.* TO u",
		"CREATE TEMPORARY TABLE t (i INT)",
		"SELECT * INTO OUTFILE '/tmp/x' FROM t",
		"",
		"   ",
	}
	for _, q := range bad {
		if err := inventory.ValidateReadOnlySQL(q); err == nil {
			t.Fatalf("expected reject %q", q)
		}
	}
}

func TestValidateReadOnlySQL_CommentStripStillRejectsWrite(t *testing.T) {
	q := "/* SELECT 1 */ UPDATE t SET a=1"
	err := inventory.ValidateReadOnlySQL(q)
	if err == nil {
		t.Fatal("expected reject write after comment")
	}
	if !strings.Contains(err.Error(), "UPDATE") && !strings.Contains(err.Error(), "allowlisted") && !strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("unexpected err: %v", err)
	}
}
