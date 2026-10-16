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
