package mysql

import (
	"os"
	"strings"
	"testing"
)

// The MySQL repository owns the transaction boundary. This guard prevents a
// future split that would persist a workflow instance before its first task.
func TestCreateInstanceWithFirstTaskUsesOneTransaction(t *testing.T) {
	data, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatalf("read repository.go: %v", err)
	}
	src := string(data)
	start := strings.Index(src, "func (r *Repository) CreateInstanceWithFirstTask")
	end := strings.Index(src, "func createInstanceTx")
	if start < 0 || end < 0 || end <= start {
		t.Fatal("atomic workflow materialization implementation not found")
	}
	body := src[start:end]
	instanceAt := strings.Index(body, "createInstanceTx(ctx, tx, in)")
	taskAt := strings.Index(body, "createTaskTx(ctx, tx, firstTask)")
	commitAt := strings.Index(body, "tx.Commit()")
	if instanceAt < 0 || taskAt < 0 || commitAt < 0 || !(instanceAt < taskAt && taskAt < commitAt) {
		t.Fatal("workflow instance and first task must share one transaction before commit")
	}
}

func TestRegisterSnapshotCatalogCodesQualifiesSourceColumn(t *testing.T) {
	data, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatalf("read repository.go: %v", err)
	}
	src := string(data)
	if !strings.Contains(src, "INSERT IGNORE INTO workflow_template_department_code_registry") ||
		!strings.Contains(src, "SELECT wtd.department_code") ||
		!strings.Contains(src, "FROM workflow_template_departments AS wtd") {
		t.Fatal("catalog-code registration must qualify its source column and be idempotent")
	}
}
