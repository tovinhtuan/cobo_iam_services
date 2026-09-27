package mysql

import (
	"os"
	"strings"
	"testing"
)

func TestClaimSQLMatchesContract(t *testing.T) {
	sqlText := ClaimPredicateSQL + "\n" + claimDueSQL
	for _, needle := range []string{
		"SET send_status = 'SENDING'",
		"lease_id = ?",
		"lease_until = ?",
		"updated_at = ?",
		"send_status IN ('PENDING', 'RETRYABLE_FAILED')",
		"lease_until IS NULL OR lease_until < ?",
		"next_retry_at IS NULL OR next_retry_at <= ?",
		"LIMIT 1",
	} {
		if !strings.Contains(sqlText, needle) {
			t.Fatalf("claim SQL missing %s", needle)
		}
	}
	for _, forbidden := range []string{"SELECT row", "mock-no-smtp", "@"} {
		if strings.Contains(sqlText, forbidden) {
			t.Fatalf("claim SQL contains %s", forbidden)
		}
	}
	up, err := os.ReadFile("../../../../migrations/0144_workflow_department_binding.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(up), "lease_until") {
		t.Fatal("0144 must stay free of lease columns")
	}
}
