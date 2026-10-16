package mysql

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	deadlinealertsapp "github.com/cobo/cobo_iam_services/internal/deadlinealerts/app"
	_ "github.com/go-sql-driver/mysql"
)

// MR-3 (PO decision Q1-B): an open obligation stays in the alerts tab after its template is
// archived, flagged template_archived, with its name and deadline config from the version it was
// archived from. Against a migrated MySQL 8.0:
//
//	MYSQL_TEST_DSN='<user>:<pass>@tcp(127.0.0.1:3306)/cobo_iam?parseTime=true&loc=UTC' go test ./internal/deadlinealerts/infra/mysql/ -run ObligationOpen

func obligationTestDB(t *testing.T) *sql.DB {
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

const obligationCompany = "c_001"

// seedObligationTemplate inserts a global template with version 1, in the given lifecycle:
// "active" (version 1 active), "archived" (archived from version 1) or "never_activated".
func seedObligationTemplate(t *testing.T, db *sql.DB, suffix, lifecycle string) string {
	t.Helper()
	ctx := context.Background()
	typeID := "it-mr3-oo-" + suffix + "-" + time.Now().Format("150405.000000")
	var groupID string
	if err := db.QueryRowContext(ctx, `SELECT group_id FROM disclosure_type_groups ORDER BY group_id LIMIT 1`).Scan(&groupID); err != nil {
		t.Fatalf("seeded disclosure type group: %v", err)
	}
	t.Cleanup(func() {
		for _, q := range []string{
			`DELETE FROM periodic_cycles WHERE type_id = ?`,
			`DELETE FROM disclosure_records WHERE type_id = ?`,
			`DELETE FROM disclosure_type_versions WHERE type_id = ?`,
			`DELETE FROM disclosure_types WHERE type_id = ?`,
		} {
			_, _ = db.ExecContext(context.Background(), q, typeID)
		}
	})
	status, active, archivedFrom := "active", 1, any(nil)
	switch lifecycle {
	case "archived":
		status, active, archivedFrom = "archived", 0, 1
	case "never_activated":
		active = 0
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO disclosure_types (type_id, company_id, group_id, status, active_version_no, archived_from_version_no)
		VALUES (?, NULL, ?, ?, ?, ?)
	`, typeID, groupID, status, active, archivedFrom); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO disclosure_type_versions (type_id, version_no, name, category, template_category, description, updated_by, deadline_config_json)
		VALUES (?, 1, ?, 'irregular', 'irregular', '', 'it-mr3', JSON_OBJECT('template_category', 'irregular'))
	`, typeID, "MR-3 "+suffix); err != nil {
		t.Fatal(err)
	}
	return typeID
}

func seedDraft(t *testing.T, db *sql.DB, typeID string) string {
	t.Helper()
	recordID := typeID + "-rec"
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO disclosure_records (record_id, company_id, type_id, department_id, title, content, status, created_by, updated_by, planned_date)
		VALUES (?, ?, ?, 'general', 'MR-3 obligation', 'MR-3', 'Draft', 'it-mr3', 'it-mr3', '2026-12-31')
	`, recordID, obligationCompany, typeID); err != nil {
		t.Fatal(err)
	}
	return recordID
}

func TestObligationOpen_ArchivedTemplateKeepsItsOpenObligation(t *testing.T) {
	db := obligationTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	archived := seedObligationTemplate(t, db, "arch", "archived")
	never := seedObligationTemplate(t, db, "never", "never_activated")
	active := seedObligationTemplate(t, db, "act", "active")
	want := map[string]bool{seedDraft(t, db, archived): true, seedDraft(t, db, never): false, seedDraft(t, db, active): false}

	rows, err := repo.ListRows(ctx, obligationCompany, deadlinealertsapp.DeadlineAlertAccessScope{CompanyID: obligationCompany, CanViewAll: true})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]deadlinealertsapp.AlertRow{}
	for _, row := range rows {
		if _, ok := want[row.RecordID]; ok {
			seen[row.RecordID] = row
		}
	}
	for recordID, archivedFlag := range want {
		row, ok := seen[recordID]
		if !ok {
			t.Fatalf("open obligation %s missing from the alerts list", recordID)
		}
		if row.TemplateArchived != archivedFlag {
			t.Fatalf("%s template_archived=%v, want %v", recordID, row.TemplateArchived, archivedFlag)
		}
		if row.TypeName == "" && recordID != never+"-rec" {
			t.Fatalf("%s lost its template name", recordID)
		}
	}

	page, total, err := repo.ListRowsPage(ctx, obligationCompany, deadlinealertsapp.DeadlineAlertAccessScope{CompanyID: obligationCompany, CanViewAll: true}, 1, 100)
	if err != nil {
		t.Fatal(err)
	}
	if total != len(rows) || len(page) == 0 {
		t.Fatalf("count=%d list=%d, want the count to match the list", total, len(rows))
	}

	cfg, err := repo.GetTypeDeadlineConfig(ctx, obligationCompany, archived)
	if err != nil || cfg == nil {
		t.Fatalf("deadline config of an archived template: cfg=%v err=%v, want the archived-from version's config", cfg, err)
	}
}

func TestObligationOpen_NextCyclesSkipSuspendedAndSkipped(t *testing.T) {
	db := obligationTestDB(t)
	repo := NewRepository(db)
	typeID := seedObligationTemplate(t, db, "next", "active")
	for state, id := range map[string]string{"PENDING": "-p", "SUSPENDED": "-s", "SKIPPED": "-k"} {
		if _, err := db.ExecContext(context.Background(), `
			INSERT INTO periodic_cycles (cycle_id, type_id, company_id, cycle_label, cycle_start, open_at, due_date, materialization_state)
			VALUES (?, ?, ?, ?, '2027-01-01', '2027-01-01', '2027-01-31', ?)
		`, typeID+id, typeID, obligationCompany, typeID+id, state); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := repo.ListNextAlertCycles(context.Background(), obligationCompany, "2026-10-16")
	if err != nil {
		t.Fatal(err)
	}
	var mine []string
	for _, row := range rows {
		if row.TypeID == typeID {
			mine = append(mine, row.CycleID)
		}
	}
	if len(mine) != 1 || mine[0] != typeID+"-p" {
		t.Fatalf("next cycles=%v, want only the PENDING one", mine)
	}
}

// seedAdHocRecord inserts a record of typeID in the given status (submitted when not Draft) and,
// when proposalStatus is not empty, the ad-hoc proposal that created it.
func seedAdHocRecord(t *testing.T, db *sql.DB, typeID, suffix, recordStatus, proposalStatus string) string {
	t.Helper()
	ctx := context.Background()
	recordID := "m3b-" + suffix + "-" + time.Now().Format("150405.000000") // record_id is VARCHAR(36)
	if _, err := db.ExecContext(ctx, `
		INSERT INTO disclosure_records (record_id, company_id, type_id, department_id, title, content, status, created_by, updated_by, submitted_at)
		VALUES (?, ?, ?, 'general', 'MR-3b ad-hoc', 'MR-3b', ?, 'it-mr3b', 'it-mr3b', IF(? = 'Draft', NULL, NOW(3)))
	`, recordID, obligationCompany, typeID, recordStatus, recordStatus); err != nil {
		t.Fatal(err)
	}
	if proposalStatus != "" {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO ad_hoc_proposals (proposal_id, company_id, type_id, record_id, status, proposed_workflow_json, created_by, final_deadline_date)
			VALUES (?, ?, ?, ?, ?, JSON_ARRAY(), 'it-mr3b', '2026-12-20')
		`, recordID+"-p", obligationCompany, typeID, recordID, proposalStatus); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_, _ = db.ExecContext(context.Background(), `DELETE FROM ad_hoc_proposals WHERE proposal_id = ?`, recordID+"-p")
		})
	}
	return recordID
}

// MR-3b, AC-11 (contract v1.2 §2.2, PO Q-12 a′): an approved ad-hoc is an open obligation from
// approval until its workflow completes, although finalize auto-submits its record.
func TestObligationOpen_ApprovedAdHocIsOpenUntilItsWorkflowCompletes(t *testing.T) {
	db := obligationTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	typeID := seedObligationTemplate(t, db, "adhoc", "active")
	want := map[string]bool{
		seedAdHocRecord(t, db, typeID, "review", "PendingReview", "approved"):   true,
		seedAdHocRecord(t, db, typeID, "approved", "Approved", "approved"):      false,
		seedAdHocRecord(t, db, typeID, "published", "Published", "approved"):    false,
		seedAdHocRecord(t, db, typeID, "completed", "Completed", "approved"):    false,
		seedAdHocRecord(t, db, typeID, "rejected", "PendingReview", "rejected"): false,
		seedAdHocRecord(t, db, typeID, "noprop", "PendingReview", ""):           false,
	}
	scope := deadlinealertsapp.DeadlineAlertAccessScope{CompanyID: obligationCompany, CanViewAll: true}
	rows, err := repo.ListRows(ctx, obligationCompany, scope)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]deadlinealertsapp.AlertRow{}
	for _, row := range rows {
		if _, ok := want[row.RecordID]; ok {
			seen[row.RecordID] = row
		}
	}
	for recordID, open := range want {
		row, listed := seen[recordID]
		if listed != open {
			t.Fatalf("%s listed=%v, want %v", recordID, listed, open)
		}
		if open && row.AdHocDeadlineDate != "2026-12-20" {
			t.Fatalf("%s ad-hoc due=%q, want the approved final deadline 2026-12-20", recordID, row.AdHocDeadlineDate)
		}
	}
	_, total, err := repo.ListRowsPage(ctx, obligationCompany, scope, 1, 100)
	if err != nil {
		t.Fatal(err)
	}
	if total != len(rows) {
		t.Fatalf("count=%d list=%d, want the count to match the list", total, len(rows))
	}
}

// The alerts list only shows obligations with a due date (contract v1.2 §5.2) and its count must
// match it: an approved ad-hoc whose proposal carries no deadline input (legacy rows on DEV) is
// left out of both, so `total` equals the items (MR-3b smoke: total 88 vs 85 items).
func TestObligationOpen_AdHocWithoutDeadlineIsNeitherListedNorCounted(t *testing.T) {
	db := obligationTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	typeID := seedObligationTemplate(t, db, "nodue", "active")
	withDue := seedAdHocRecord(t, db, typeID, "due", "PendingReview", "approved")
	noDue := seedAdHocRecord(t, db, typeID, "nodue", "PendingReview", "approved")
	if _, err := db.ExecContext(ctx, `UPDATE ad_hoc_proposals SET final_deadline_date = NULL, proposed_t0_date = NULL, proposed_deadline_days = NULL, proposed_deadline_date = NULL WHERE record_id = ?`, noDue); err != nil {
		t.Fatal(err)
	}
	scope := deadlinealertsapp.DeadlineAlertAccessScope{CompanyID: obligationCompany, CanViewAll: true}
	rows, err := repo.ListRows(ctx, obligationCompany, scope)
	if err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	for _, row := range rows {
		listed[row.RecordID] = true
	}
	if !listed[withDue] || listed[noDue] {
		t.Fatalf("with due listed=%v, without due listed=%v; want only the one with a due date", listed[withDue], listed[noDue])
	}
	_, total, err := repo.ListRowsPage(ctx, obligationCompany, scope, 1, 100)
	if err != nil {
		t.Fatal(err)
	}
	if total != len(rows) {
		t.Fatalf("count=%d list=%d, want the count to match the list", total, len(rows))
	}
}
