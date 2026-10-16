package mysql

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// MR-3b, AC-13 (contract v1.2 §2.2, PO-01): "Việc của tôi" shows the tasks of open Draft
// obligations (periodic records stay Draft while their workflow runs). Draft records that are not
// open (cycle not opened yet) stay out, and records past Draft keep showing as before.
//
//	MYSQL_TEST_DSN='<user>:<pass>@tcp(127.0.0.1:3306)/cobo_iam?parseTime=true&loc=UTC' go test ./internal/personalops/infra/mysql/ -run MineObligation

const mineCompany = "c_001"

func mineTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("MYSQL_TEST_DSN"))
	if dsn == "" {
		t.Skip("MYSQL_TEST_DSN not set")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Ping(); err != nil {
		t.Fatal(err)
	}
	return db
}

// seedMineRecord inserts a record (submitted unless Draft) with one pending task assigned to
// membership, and, when openAt is not empty, the periodic cycle that created it.
func seedMineRecord(t *testing.T, db *sql.DB, typeID, suffix, status, openAt, membership string) string {
	t.Helper()
	ctx := context.Background()
	stamp := time.Now().Format("150405.000000")
	recordID := "m3b-" + suffix + "-" + stamp // VARCHAR(36)
	instanceID := "m3bwi-" + suffix + "-" + stamp
	t.Cleanup(func() {
		for _, q := range []string{
			`DELETE FROM workflow_tasks WHERE workflow_instance_id = ?`,
			`DELETE FROM workflow_instances WHERE workflow_instance_id = ?`,
		} {
			_, _ = db.ExecContext(context.Background(), q, instanceID)
		}
		_, _ = db.ExecContext(context.Background(), `DELETE FROM periodic_cycles WHERE record_id = ?`, recordID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM disclosure_records WHERE record_id = ?`, recordID)
	})
	if _, err := db.ExecContext(ctx, `
		INSERT INTO disclosure_records (record_id, company_id, type_id, department_id, title, content, status, created_by, updated_by, planned_date, submitted_at)
		VALUES (?, ?, ?, 'general', 'MR-3b mine', 'MR-3b', ?, 'it-mr3b', 'it-mr3b', '2026-12-31', IF(? = 'Draft', NULL, NOW(3)))
	`, recordID, mineCompany, typeID, status, status); err != nil {
		t.Fatal(err)
	}
	if openAt != "" {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO periodic_cycles (cycle_id, type_id, company_id, cycle_label, cycle_start, open_at, due_date, record_id, materialization_state, materialized_at)
			VALUES (?, ?, ?, ?, ?, ?, '2026-12-31', ?, 'COMPLETED', NOW(3))
		`, recordID+"-c", typeID, mineCompany, recordID, openAt, openAt, recordID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO workflow_instances (workflow_instance_id, company_id, record_id, status, current_step_code, created_by)
		VALUES (?, ?, ?, 'in_progress', 'step-1', 'it-mr3b')
	`, instanceID, mineCompany, recordID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO workflow_tasks (task_id, company_id, workflow_instance_id, step_code, status, assignee_membership_id)
		VALUES (?, ?, ?, 'step-1', 'pending', ?)
	`, instanceID+"-t", mineCompany, instanceID, membership); err != nil {
		t.Fatal(err)
	}
	return recordID
}

func seedMineType(t *testing.T, db *sql.DB) string {
	t.Helper()
	ctx := context.Background()
	typeID := "it-mr3b-mine-" + time.Now().Format("150405.000000")
	var groupID string
	if err := db.QueryRowContext(ctx, `SELECT group_id FROM disclosure_type_groups ORDER BY group_id LIMIT 1`).Scan(&groupID); err != nil {
		t.Fatalf("seeded disclosure type group: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM disclosure_types WHERE type_id = ?`, typeID)
	})
	if _, err := db.ExecContext(ctx, `
		INSERT INTO disclosure_types (type_id, company_id, group_id, status, active_version_no) VALUES (?, NULL, ?, 'active', 0)
	`, typeID, groupID); err != nil {
		t.Fatal(err)
	}
	return typeID
}

func TestMineObligation_TasksOfOpenDraftRecordsAreMine(t *testing.T) {
	db := mineTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	// A real membership of the company: ListMineRecords keeps the companies of the memberships.
	var membership string
	if err := db.QueryRowContext(ctx, `SELECT membership_id FROM memberships WHERE company_id = ? ORDER BY membership_id LIMIT 1`, mineCompany).Scan(&membership); err != nil {
		t.Fatalf("seeded membership of %s: %v", mineCompany, err)
	}
	typeID := seedMineType(t, db)
	future := time.Now().AddDate(0, 1, 0).Format("2006-01-02")
	want := map[string]bool{
		seedMineRecord(t, db, typeID, "draft", "Draft", "", membership):            true,  // open, irregular
		seedMineRecord(t, db, typeID, "opened", "Draft", "2026-01-01", membership): true,  // open periodic cycle
		seedMineRecord(t, db, typeID, "notyet", "Draft", future, membership):       false, // cycle not opened yet
		seedMineRecord(t, db, typeID, "review", "PendingReview", "", membership):   true,  // unchanged behavior
	}

	tasks, err := repo.ListMineOpenTasks(ctx, []string{membership}, 50)
	if err != nil {
		t.Fatalf("ListMineOpenTasks: %v", err)
	}
	gotTasks := map[string]bool{}
	for _, task := range tasks {
		gotTasks[task.RecordID] = true
	}
	records, err := repo.ListMineRecords(ctx, []string{membership})
	if err != nil {
		t.Fatalf("ListMineRecords: %v", err)
	}
	gotRecords := map[string]bool{}
	for _, rec := range records {
		gotRecords[rec.RecordID] = true
	}
	for recordID, mine := range want {
		if gotTasks[recordID] != mine {
			t.Errorf("task of %s in my tasks=%v, want %v", recordID, gotTasks[recordID], mine)
		}
		if gotRecords[recordID] != mine {
			t.Errorf("%s in my records=%v, want %v", recordID, gotRecords[recordID], mine)
		}
	}
}
