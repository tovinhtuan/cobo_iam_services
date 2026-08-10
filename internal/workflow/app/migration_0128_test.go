package app_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMigration0128_SourceShape(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	// internal/workflow/app → repo root
	root := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "../../.."))
	up := filepath.Join(root, "migrations/0128_workflow_task_assignees.up.sql")
	down := filepath.Join(root, "migrations/0128_workflow_task_assignees.down.sql")
	upBody, err := os.ReadFile(up)
	if err != nil {
		t.Fatal(err)
	}
	s := string(upBody)
	for _, want := range []string{
		"MODIFY COLUMN assignee_membership_id VARCHAR(36) NULL",
		"CREATE TABLE IF NOT EXISTS workflow_task_assignees",
		"PRIMARY KEY (task_id, membership_id)",
		"idx_workflow_task_assignees_membership",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("up migration missing %q", want)
		}
	}
	if strings.Contains(s, "INSERT INTO workflow_task_assignees") {
		t.Fatal("NO_TASK_ASSIGNEE_BACKFILL violated")
	}
	downBody, err := os.ReadFile(down)
	if err != nil {
		t.Fatal(err)
	}
	ds := strings.ToLower(string(downBody))
	if strings.Contains(ds, "modify") && strings.Contains(ds, "not null") {
		t.Fatal("dangerous down restoring NOT NULL")
	}
	if strings.Contains(ds, "drop table") {
		t.Fatal("dangerous down dropping relation table")
	}
}
