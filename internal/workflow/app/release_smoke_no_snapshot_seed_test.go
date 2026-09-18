package app

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Release invariant: the canonical recovery E2E smoke must not INSERT B1 snapshot rows.
func TestReleaseSmoke_NoDirectSnapshotSQLSeed(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "../../.."))
	path := filepath.Join(root, ".playwright-mcp/recovery-authoring-real-e2e-smoke.cjs")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read recovery smoke: %v", err)
	}
	forbidden := "INSERT INTO workflow_step_document_requirement_snapshots"
	if strings.Contains(string(body), forbidden) {
		t.Fatalf("recovery release smoke must not SQL-seed B1 snapshots")
	}
	b6Path := filepath.Join(root, ".playwright-mcp/b6-full-v1-release-smoke.cjs")
	if b6, err := os.ReadFile(b6Path); err == nil {
		if strings.Contains(string(b6), "INSERT INTO workflow_step_document_requirement_snapshots") {
			t.Fatalf("b6 release smoke must not contain live INSERT snapshot seed SQL")
		}
	}
	for _, want := range []string{
		"SMOKE_ASSERT_EFFECTIVE_DOCUMENTS_GT_ZERO",
		"SMOKE_ASSERT_REAL_SNAPSHOT_COUNT_GT_ZERO",
		"SMOKE_ASSERT_TENANT_UPLOAD_CTA",
		"SMOKE_ASSERT_REAL_TENANT_UPLOAD",
		"SQL_SEEDED_REQUIREMENT_SNAPSHOTS",
	} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("recovery smoke missing assert %q", want)
		}
	}
}
