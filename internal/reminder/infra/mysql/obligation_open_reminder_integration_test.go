package mysql

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	reminderapp "github.com/cobo/cobo_iam_services/internal/reminder/app"
	_ "github.com/go-sql-driver/mysql"
)

// MR-3b (contract v1.2 §2.2): deadline reminders (scope DISCLOSURE) follow ObligationOpen, the
// definition the alerts tab uses. An approved ad-hoc whose record was auto-submitted is reminded; a
// Draft whose periodic cycle has not opened is not; and a record that stopped being open after its
// occurrence was created is flagged so dispatch can close the occurrence instead of sending it.
//
//	MYSQL_TEST_DSN='<user>:<pass>@tcp(127.0.0.1:3306)/cobo_iam?parseTime=true&loc=UTC' go test ./internal/reminder/infra/mysql/ -run ReminderObligation

const reminderCompany = "c_001"

func reminderTestDB(t *testing.T) *sql.DB {
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

func seedReminderType(t *testing.T, db *sql.DB) string {
	t.Helper()
	ctx := context.Background()
	typeID := "it-mr3b-rem-" + time.Now().Format("150405.000000")
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

// seedReminderRecord inserts a record (submitted unless Draft/NotStarted) with a due date in the past, a
// "1 day before" deadline reminder config, an optional ad-hoc proposal and an optional cycle.
func seedReminderRecord(t *testing.T, db *sql.DB, typeID, suffix, status, proposalStatus, openAt string) string {
	t.Helper()
	ctx := context.Background()
	recordID := "m3b-" + suffix + "-" + time.Now().Format("150405.000000") // VARCHAR(36)
	t.Cleanup(func() {
		for _, q := range []string{
			`DELETE FROM reminder_occurrences WHERE disclosure_id = ?`,
			`DELETE FROM reminder_configs WHERE disclosure_id = ?`,
			`DELETE FROM ad_hoc_proposals WHERE record_id = ?`,
			`DELETE FROM periodic_cycles WHERE record_id = ?`,
			`DELETE FROM disclosure_records WHERE record_id = ?`,
		} {
			_, _ = db.ExecContext(context.Background(), q, recordID)
		}
	})
	if _, err := db.ExecContext(ctx, `
		INSERT INTO disclosure_records (record_id, company_id, type_id, department_id, title, content, status, created_by, updated_by, planned_date, submitted_at)
		VALUES (?, ?, ?, 'general', 'MR-3b reminder', 'MR-3b', ?, 'it-mr3b', 'it-mr3b', '2026-10-01', IF(? IN ('Draft', 'NotStarted'), NULL, NOW(3)))
	`, recordID, reminderCompany, typeID, status, status); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO reminder_configs (disclosure_id, scope_type, scope_id, enabled, mode, days_before_json, recipient_type, recipients_json, updated_by)
		VALUES (?, 'DISCLOSURE', ?, 1, 'DaysBefore', JSON_ARRAY(1), 'Individuals', JSON_ARRAY('qa@cobo.test'), 'it-mr3b')
	`, recordID, recordID); err != nil {
		t.Fatal(err)
	}
	if proposalStatus != "" {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO ad_hoc_proposals (proposal_id, company_id, type_id, record_id, status, proposed_workflow_json, created_by)
			VALUES (?, ?, ?, ?, ?, JSON_ARRAY(), 'it-mr3b')
		`, recordID+"-p", reminderCompany, typeID, recordID, proposalStatus); err != nil {
			t.Fatal(err)
		}
	}
	if openAt != "" {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO periodic_cycles (cycle_id, type_id, company_id, cycle_label, cycle_start, open_at, due_date, record_id, materialization_state, materialized_at)
			VALUES (?, ?, ?, ?, ?, ?, '2026-10-01', ?, 'COMPLETED', NOW(3))
		`, recordID+"-c", typeID, reminderCompany, recordID, openAt, openAt, recordID); err != nil {
			t.Fatal(err)
		}
	}
	return recordID
}

func occurrenceCount(t *testing.T, db *sql.DB, recordID string) int {
	t.Helper()
	var n int
	if err := db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM reminder_occurrences WHERE disclosure_id = ?`, recordID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestReminderObligation_DeadlineRemindersFollowObligationOpen(t *testing.T) {
	db := reminderTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	typeID := seedReminderType(t, db)
	future := time.Now().AddDate(0, 1, 0).Format("2006-01-02")
	// The ad-hoc rows carry a planned date (e.g. set by hand on the record): finalize leaves it NULL,
	// so the finalize flow gets no deadline reminder yet (follow-up with DUE-06, see adhocNoDate).
	notStarted := seedReminderRecord(t, db, typeID, "notstart", "NotStarted", "", "")
	stepOnNotYet := seedReminderRecord(t, db, typeID, "stepnotyet", "Draft", "", future)
	adhocNoDate := seedReminderRecord(t, db, typeID, "adhocnull", "PendingReview", "approved", "")
	if _, err := db.ExecContext(ctx, `UPDATE disclosure_records SET planned_date = NULL WHERE record_id = ?`, adhocNoDate); err != nil {
		t.Fatal(err)
	}
	// A workflow-step config keeps the previous rule (not submitted), whatever ObligationOpen says.
	if _, err := db.ExecContext(ctx, `UPDATE reminder_configs SET scope_type = 'WORKFLOW_STEP', scope_id = CONCAT(disclosure_id, ':step-1') WHERE disclosure_id = ?`, stepOnNotYet); err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		seedReminderRecord(t, db, typeID, "draft", "Draft", "", ""):                 true,  // open, unchanged
		seedReminderRecord(t, db, typeID, "adhoc", "PendingReview", "approved", ""): true,  // approved ad-hoc (AC-11)
		seedReminderRecord(t, db, typeID, "adhocdone", "Published", "approved", ""): false, // workflow completed
		seedReminderRecord(t, db, typeID, "notyet", "Draft", "", future):            false, // cycle not opened yet
		seedReminderRecord(t, db, typeID, "submitted", "PendingReview", "", ""):     false, // periodic submitted, unchanged
		notStarted:   true,  // CMS global record: previous rule until the PO decides
		stepOnNotYet: true,  // workflow-step config: previous rule
		adhocNoDate:  false, // no planned_date (finalize flow): known gap, follow-up
	}
	if _, err := repo.MaterializeDueOccurrences(ctx, time.Now()); err != nil {
		t.Fatalf("MaterializeDueOccurrences: %v", err)
	}
	for recordID, reminded := range want {
		if got := occurrenceCount(t, db, recordID) > 0; got != reminded {
			t.Errorf("%s reminded=%v, want %v", recordID, got, reminded)
		}
	}

	// The Draft is submitted after its occurrence was created: dispatch must see it closed.
	var draftID, adhocID string
	for recordID, reminded := range want {
		if reminded && strings.HasPrefix(recordID, "m3b-draft-") {
			draftID = recordID
		}
		if reminded && strings.HasPrefix(recordID, "m3b-adhoc-") {
			adhocID = recordID
		}
	}
	if _, err := db.ExecContext(ctx, `UPDATE disclosure_records SET status = 'PendingReview', submitted_at = NOW(3) WHERE record_id = ?`, draftID); err != nil {
		t.Fatal(err)
	}
	candidates, err := repo.ListDispatchCandidates(ctx, time.Now(), 10000)
	if err != nil {
		t.Fatalf("ListDispatchCandidates: %v", err)
	}
	closed := map[string]bool{}
	for _, c := range candidates {
		if c.ScopeType == reminderapp.ScopeTypeDisclosure && (c.RecordID == draftID || c.RecordID == adhocID || c.RecordID == notStarted) {
			closed[c.RecordID] = c.ObligationClosed
		}
	}
	if v, ok := closed[notStarted]; !ok || v {
		t.Errorf("NotStarted candidate closed=%v listed=%v, want listed and open", v, ok)
	}
	if v, ok := closed[draftID]; !ok || !v {
		t.Errorf("submitted record candidate closed=%v listed=%v, want listed and closed", v, ok)
	}
	if v, ok := closed[adhocID]; !ok || v {
		t.Errorf("approved ad-hoc candidate closed=%v listed=%v, want listed and open", v, ok)
	}
}

// A skip path closes an occurrence only while it is still dispatchable: a row a concurrent
// dispatch already marked SENT keeps its status (review PERF-02 / DB-07).
func TestReminderObligation_SkipNeverOverwritesASentOccurrence(t *testing.T) {
	db := reminderTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	typeID := seedReminderType(t, db)
	recordID := seedReminderRecord(t, db, typeID, "sent", "Draft", "", "")
	if _, err := repo.MaterializeDueOccurrences(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	var occurrenceID string
	if err := db.QueryRowContext(ctx, `SELECT occurrence_id FROM reminder_occurrences WHERE disclosure_id = ? LIMIT 1`, recordID).Scan(&occurrenceID); err != nil {
		t.Fatalf("occurrence of %s: %v", recordID, err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE reminder_occurrences SET status = 'SENT' WHERE occurrence_id = ?`, occurrenceID); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateDispatchResult(ctx, reminderapp.DispatchResultInput{
		OccurrenceID: occurrenceID, Status: reminderapp.ReminderStatusFailed,
		LastErrorCode: reminderapp.ReminderSkipObligationClosed, OnlyIfDispatchable: true,
	}); err != nil {
		t.Fatal(err)
	}
	var status string
	if err := db.QueryRowContext(ctx, `SELECT status FROM reminder_occurrences WHERE occurrence_id = ?`, occurrenceID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "SENT" {
		t.Fatalf("status=%s, want SENT kept", status)
	}
}
