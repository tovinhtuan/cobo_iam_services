package mysql

import (
	"context"
	"database/sql"
	"net/http"
	"testing"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// MR-3 (PO decision Q1-B): archiving a global template suspends its cycles without a record in the
// same transaction; a restore backfills only the chosen ones and skips the rest; the archive impact
// counts open obligations per company. Same DSN as departmentFilterTestDB:
//
//	MYSQL_TEST_DSN='<user>:<pass>@tcp(127.0.0.1:3306)/cobo_iam?parseTime=true' go test ./internal/disclosure/infra/mysql/ -run 'ArchiveCycles'

func seedArchiveTemplate(t *testing.T, db *sql.DB) string {
	t.Helper()
	ctx := context.Background()
	typeID := "it-mr3-" + time.Now().Format("150405.000000")
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
	if _, err := db.ExecContext(ctx, `
		INSERT INTO disclosure_types (type_id, company_id, group_id, status, active_version_no) VALUES (?, NULL, ?, 'active', 1)
	`, typeID, groupID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO disclosure_type_versions (type_id, version_no, name, category, template_category, description, updated_by)
		VALUES (?, 1, 'MR-3 archive test', 'periodic', 'periodic', '', 'it-mr3')
	`, typeID); err != nil {
		t.Fatal(err)
	}
	return typeID
}

func seedCycle(t *testing.T, db *sql.DB, typeID, cycleID, companyID, state string, recordID any) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO periodic_cycles (cycle_id, type_id, company_id, cycle_label, due_date, record_id, materialization_state, materialized_at)
		VALUES (?, ?, ?, ?, '2026-11-30', ?, ?, IF(? = 'PENDING', NULL, NOW(3)))
	`, cycleID, typeID, companyID, cycleID, recordID, state, state); err != nil {
		t.Fatal(err)
	}
}

func cycleState(t *testing.T, db *sql.DB, cycleID string) (state string, fenced bool) {
	t.Helper()
	var materializedAt sql.NullTime
	if err := db.QueryRowContext(context.Background(), `SELECT materialization_state, materialized_at FROM periodic_cycles WHERE cycle_id = ?`, cycleID).Scan(&state, &materializedAt); err != nil {
		t.Fatal(err)
	}
	return state, materializedAt.Valid
}

func seedOpenRecord(t *testing.T, db *sql.DB, typeID, recordID, companyID string, submitted bool) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO disclosure_records (record_id, company_id, type_id, department_id, title, content, status, created_by, updated_by, submitted_at)
		VALUES (?, ?, ?, 'general', 'MR-3', 'MR-3', 'Draft', 'it-mr3', 'it-mr3', IF(?, NOW(3), NULL))
	`, recordID, companyID, typeID, submitted); err != nil {
		t.Fatal(err)
	}
}

func archiveForTest(t *testing.T, repo *Repository, typeID string) *disclosureapp.ArchiveGlobalTemplateResult {
	t.Helper()
	res, err := repo.ArchiveGlobalTemplate(context.Background(), disclosureapp.ArchiveGlobalTemplateParams{TypeID: typeID, UpdatedBy: "it-mr3", Reason: "MR-3 test"})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// restoreToday is after every seeded due date, so seeded cycles count as missed unless a test
// sets a later open date (setCycleOpenAt).
const restoreToday = "2026-12-31"

func restoreForTest(repo *Repository, typeID string, backfill ...string) (*disclosureapp.RestoreGlobalTemplateResult, error) {
	from := 1
	return repo.RestoreGlobalTemplate(context.Background(), disclosureapp.RestoreGlobalTemplateParams{
		TypeID: typeID, UpdatedBy: "it-mr3", ExpectedFromVersionNo: &from, RestoreActiveVersionNo: 1, BackfillCycleIDs: backfill,
		TodayHCM: restoreToday,
	})
}

func setCycleOpenAt(t *testing.T, db *sql.DB, cycleID, openAt string) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), `UPDATE periodic_cycles SET open_at = ? WHERE cycle_id = ?`, openAt, cycleID); err != nil {
		t.Fatal(err)
	}
}

func TestArchiveCycles_ArchiveSuspendsCyclesWithoutARecord(t *testing.T) {
	db := departmentFilterTestDB(t)
	repo := NewRepository(db)
	typeID := seedArchiveTemplate(t, db)
	seedCycle(t, db, typeID, typeID+"-p", "c_001", "PENDING", nil)
	seedCycle(t, db, typeID, typeID+"-r", "c_001", "RETRY", nil)
	seedCycle(t, db, typeID, typeID+"-c", "c_002", "CLAIMED", nil)
	seedCycle(t, db, typeID, typeID+"-d", "c_002", "COMPLETED", typeID+"-rec")

	if res := archiveForTest(t, repo, typeID); res.SuspendedCycles != 2 {
		t.Fatalf("suspended_cycles=%d, want 2 (PENDING and RETRY)", res.SuspendedCycles)
	}
	for id, want := range map[string]string{"-p": "SUSPENDED", "-r": "SUSPENDED", "-c": "CLAIMED", "-d": "COMPLETED"} {
		state, fenced := cycleState(t, db, typeID+id)
		if state != want {
			t.Fatalf("cycle %s state=%s, want %s", id, state, want)
		}
		if want == "SUSPENDED" && !fenced {
			t.Fatalf("cycle %s lost its materialized_at fence", id)
		}
	}
	if again := archiveForTest(t, repo, typeID); !again.AlreadyArchived || again.SuspendedCycles != 0 {
		t.Fatalf("second archive=%+v, want a no-op", again)
	}
}

func TestArchiveCycles_RestoreBackfillsOnlyTheChosenCycles(t *testing.T) {
	db := departmentFilterTestDB(t)
	repo := NewRepository(db)
	typeID := seedArchiveTemplate(t, db)
	seedCycle(t, db, typeID, typeID+"-a", "c_001", "PENDING", nil)
	seedCycle(t, db, typeID, typeID+"-b", "c_001", "RETRY", nil)
	archiveForTest(t, repo, typeID)

	res, err := restoreForTest(repo, typeID, typeID+"-a")
	if err != nil {
		t.Fatal(err)
	}
	if res.BackfilledCycles != 1 || res.SkippedCycles != 1 || res.RestoredMode != "active" {
		t.Fatalf("restore=%+v, want 1 backfilled and 1 skipped", res)
	}
	if state, fenced := cycleState(t, db, typeID+"-a"); state != "PENDING" || fenced {
		t.Fatalf("backfilled cycle state=%s fenced=%v, want claimable PENDING", state, fenced)
	}
	if state, _ := cycleState(t, db, typeID+"-b"); state != "SKIPPED" {
		t.Fatalf("other cycle state=%s, want SKIPPED", state)
	}
}

func TestArchiveCycles_RestoreRejectsACycleThatIsNotSuspended(t *testing.T) {
	db := departmentFilterTestDB(t)
	repo := NewRepository(db)
	typeID := seedArchiveTemplate(t, db)
	seedCycle(t, db, typeID, typeID+"-a", "c_001", "PENDING", nil)
	seedCycle(t, db, typeID, typeID+"-c", "c_001", "CLAIMED", nil)
	archiveForTest(t, repo, typeID)

	_, err := restoreForTest(repo, typeID, typeID+"-a", typeID+"-c")
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusUnprocessableEntity || he.Details["field"] != "backfill_cycle_ids" {
		t.Fatalf("err=%v, want 422 on backfill_cycle_ids", err)
	}
	if state, _ := cycleState(t, db, typeID+"-a"); state != "SUSPENDED" {
		t.Fatalf("cycle state=%s after a refused restore, want SUSPENDED (rolled back)", state)
	}
	var status string
	if err := db.QueryRowContext(context.Background(), `SELECT status FROM disclosure_types WHERE type_id = ?`, typeID).Scan(&status); err != nil || status != "archived" {
		t.Fatalf("template status=%q err=%v, want still archived", status, err)
	}
}

func TestArchiveCycles_ImpactCountsOpenObligationsAndCycles(t *testing.T) {
	db := departmentFilterTestDB(t)
	repo := NewRepository(db)
	typeID := seedArchiveTemplate(t, db)
	seedOpenRecord(t, db, typeID, typeID+"-r1", "c_001", false)
	seedOpenRecord(t, db, typeID, typeID+"-r2", "c_001", false)
	seedOpenRecord(t, db, typeID, typeID+"-r3", "c_002", false)
	seedOpenRecord(t, db, typeID, typeID+"-r4", "c_002", true) // submitted: not open
	seedCycle(t, db, typeID, typeID+"-p", "c_001", "PENDING", nil)
	today := restoreToday // after the seeded due date: the suspended cycle counts as missed

	before, err := repo.GetTemplateArchiveImpact(context.Background(), typeID, today)
	if err != nil {
		t.Fatal(err)
	}
	if before.OpenRecords.Total != 3 || len(before.OpenRecords.Companies) != 2 || before.OpenRecords.Companies[0].CompanyID != "c_001" || before.OpenRecords.Companies[0].OpenCount != 2 {
		t.Fatalf("open records=%+v, want 3 (c_001: 2, c_002: 1)", before.OpenRecords)
	}
	if before.Cycles.Pending != 1 || len(before.Cycles.Suspended) != 0 {
		t.Fatalf("cycles before archive=%+v, want 1 pending", before.Cycles)
	}

	archiveForTest(t, repo, typeID)
	after, err := repo.GetTemplateArchiveImpact(context.Background(), typeID, today)
	if err != nil {
		t.Fatal(err)
	}
	if after.Status != "archived" || after.OpenRecords.Total != 3 {
		t.Fatalf("after archive status=%s open=%d, want archived with the same 3 open obligations", after.Status, after.OpenRecords.Total)
	}
	if after.Cycles.Pending != 0 || len(after.Cycles.Suspended) != 1 || after.Cycles.SuspendedTotal != 1 || after.Cycles.Suspended[0].CycleID != typeID+"-p" {
		t.Fatalf("cycles after archive=%+v, want the suspended cycle listed", after.Cycles)
	}
	if _, err := repo.GetTemplateArchiveImpact(context.Background(), "it-mr3-missing", today); err == nil {
		t.Fatal("impact of an unknown template must be 404")
	}
}

func TestArchiveCycles_MarkSuspendedOnlyMovesAClaimedCycleOfAnArchivedTemplate(t *testing.T) {
	db := departmentFilterTestDB(t)
	repo := NewRepository(db)
	typeID := seedArchiveTemplate(t, db)
	seedCycle(t, db, typeID, typeID+"-c", "c_001", "CLAIMED", nil)
	seedCycle(t, db, typeID, typeID+"-p", "c_001", "PENDING", nil)

	// Template active (e.g. restored after the worker got its 409): nothing changes, the caller retries.
	if ok, err := repo.MarkPeriodicCycleSuspended(context.Background(), typeID+"-c", ""); err != nil || ok {
		t.Fatalf("suspend on an active template ok=%v err=%v, want false without error", ok, err)
	}
	if state, _ := cycleState(t, db, typeID+"-c"); state != "CLAIMED" {
		t.Fatalf("claimed cycle state=%s, want CLAIMED while the template is active", state)
	}

	archiveForTest(t, repo, typeID)
	if ok, err := repo.MarkPeriodicCycleSuspended(context.Background(), typeID+"-c", ""); err != nil || !ok {
		t.Fatalf("suspend on an archived template ok=%v err=%v, want true", ok, err)
	}
	if state, fenced := cycleState(t, db, typeID+"-c"); state != "SUSPENDED" || !fenced {
		t.Fatalf("claimed cycle state=%s fenced=%v, want SUSPENDED and fenced", state, fenced)
	}
	// The archive already suspended the PENDING cycle; the worker's suspend only moves CLAIMED ones.
	if ok, err := repo.MarkPeriodicCycleSuspended(context.Background(), typeID+"-p", ""); err != nil || ok {
		t.Fatalf("suspend of a non-claimed cycle ok=%v err=%v, want false", ok, err)
	}
}

func TestArchiveCycles_RestoreRejectsACycleOfAnotherTemplate(t *testing.T) {
	db := departmentFilterTestDB(t)
	repo := NewRepository(db)
	typeID := seedArchiveTemplate(t, db)
	otherID := seedArchiveTemplate(t, db)
	seedCycle(t, db, typeID, typeID+"-a", "c_001", "PENDING", nil)
	seedCycle(t, db, otherID, otherID+"-x", "c_001", "PENDING", nil)
	archiveForTest(t, repo, typeID)
	archiveForTest(t, repo, otherID)

	_, err := restoreForTest(repo, typeID, otherID+"-x")
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusUnprocessableEntity || he.Details["field"] != "backfill_cycle_ids" {
		t.Fatalf("err=%v, want 422 on backfill_cycle_ids", err)
	}
	for _, id := range []string{typeID + "-a", otherID + "-x"} {
		if state, _ := cycleState(t, db, id); state != "SUSPENDED" {
			t.Fatalf("cycle %s state=%s after a refused restore, want SUSPENDED", id, state)
		}
	}
}

func TestArchiveCycles_ImpactOfACompanyTemplateIsNotFound(t *testing.T) {
	db := departmentFilterTestDB(t)
	repo := NewRepository(db)
	typeID := seedArchiveTemplate(t, db)
	if _, err := db.ExecContext(context.Background(), `UPDATE disclosure_types SET company_id = 'c_001' WHERE type_id = ?`, typeID); err != nil {
		t.Fatal(err)
	}
	_, err := repo.GetTemplateArchiveImpact(context.Background(), typeID, time.Now().Format("2006-01-02"))
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusNotFound {
		t.Fatalf("err=%v, want 404 for a company template", err)
	}
}

// A restore resumes the suspended cycles that have not opened yet (nothing was missed) and only
// asks about the missed ones; an immediate archive + restore therefore loses no upcoming period.
func TestArchiveCycles_RestoreResumesCyclesThatHaveNotOpened(t *testing.T) {
	db := departmentFilterTestDB(t)
	repo := NewRepository(db)
	typeID := seedArchiveTemplate(t, db)
	seedCycle(t, db, typeID, typeID+"-up", "c_001", "PENDING", nil)
	seedCycle(t, db, typeID, typeID+"-missed", "c_001", "PENDING", nil)
	setCycleOpenAt(t, db, typeID+"-up", "2027-01-15")
	setCycleOpenAt(t, db, typeID+"-missed", "2026-12-01")
	archiveForTest(t, repo, typeID)

	impact, err := repo.GetTemplateArchiveImpact(context.Background(), typeID, restoreToday)
	if err != nil {
		t.Fatal(err)
	}
	if impact.Cycles.AutoResume != 1 || impact.Cycles.SuspendedTotal != 1 || len(impact.Cycles.Suspended) != 1 || impact.Cycles.Suspended[0].CycleID != typeID+"-missed" {
		t.Fatalf("impact cycles=%+v, want 1 upcoming and only the missed cycle listed", impact.Cycles)
	}

	res, err := restoreForTest(repo, typeID)
	if err != nil {
		t.Fatal(err)
	}
	if res.ResumedCycles != 1 || res.SkippedCycles != 1 || res.BackfilledCycles != 0 {
		t.Fatalf("restore=%+v, want 1 resumed and 1 skipped", res)
	}
	if state, fenced := cycleState(t, db, typeID+"-up"); state != "PENDING" || fenced {
		t.Fatalf("upcoming cycle state=%s fenced=%v, want claimable PENDING", state, fenced)
	}
	if state, _ := cycleState(t, db, typeID+"-missed"); state != "SKIPPED" {
		t.Fatalf("missed cycle state=%s, want SKIPPED", state)
	}
}

// Cycles that escaped the archive (an older worker or the lease reaper put them back to RETRY or
// PENDING) are listed by the impact and resolved by the restore like the suspended ones.
func TestArchiveCycles_RestoreResolvesCyclesThatEscapedTheArchive(t *testing.T) {
	db := departmentFilterTestDB(t)
	repo := NewRepository(db)
	typeID := seedArchiveTemplate(t, db)
	archiveForTest(t, repo, typeID)
	seedCycle(t, db, typeID, typeID+"-retry", "c_001", "RETRY", nil)
	seedCycle(t, db, typeID, typeID+"-pending", "c_002", "PENDING", nil)

	impact, err := repo.GetTemplateArchiveImpact(context.Background(), typeID, restoreToday)
	if err != nil {
		t.Fatal(err)
	}
	if impact.Cycles.SuspendedTotal != 2 {
		t.Fatalf("impact cycles=%+v, want both leftover cycles listed", impact.Cycles)
	}
	res, err := restoreForTest(repo, typeID, typeID+"-retry")
	if err != nil {
		t.Fatal(err)
	}
	if res.BackfilledCycles != 1 || res.SkippedCycles != 1 {
		t.Fatalf("restore=%+v, want 1 backfilled and 1 skipped", res)
	}
	if state, fenced := cycleState(t, db, typeID+"-retry"); state != "PENDING" || fenced {
		t.Fatalf("chosen cycle state=%s fenced=%v, want claimable PENDING", state, fenced)
	}
	if state, _ := cycleState(t, db, typeID+"-pending"); state != "SKIPPED" {
		t.Fatalf("leftover cycle state=%s, want SKIPPED: not created in bulk", state)
	}
}

// A cycle whose record an earlier attempt already created (workflow step failed, then archive) is
// resumed by a restore without a choice: the worker finishes that record (deterministic id), while
// skipping it would leave an orphan Draft open forever.
func TestArchiveCycles_RestoreResumesACycleWhoseRecordExists(t *testing.T) {
	db := departmentFilterTestDB(t)
	repo := NewRepository(db)
	typeID := seedArchiveTemplate(t, db)
	seedCycle(t, db, typeID, typeID+"-partial", "c_001", "RETRY", nil)
	seedOpenRecord(t, db, typeID, typeID+"-rec", "c_001", false)
	if _, err := db.ExecContext(context.Background(), `UPDATE periodic_cycles SET attempt_record_id = ? WHERE cycle_id = ?`, typeID+"-rec", typeID+"-partial"); err != nil {
		t.Fatal(err)
	}
	archiveForTest(t, repo, typeID)

	impact, err := repo.GetTemplateArchiveImpact(context.Background(), typeID, restoreToday)
	if err != nil {
		t.Fatal(err)
	}
	if impact.Cycles.AutoResume != 1 || impact.Cycles.SuspendedTotal != 0 {
		t.Fatalf("impact cycles=%+v, want the partial cycle resumed automatically, not offered", impact.Cycles)
	}
	res, err := restoreForTest(repo, typeID)
	if err != nil {
		t.Fatal(err)
	}
	if res.ResumedCycles != 1 || res.SkippedCycles != 0 {
		t.Fatalf("restore=%+v, want the partial cycle resumed", res)
	}
	if state, _ := cycleState(t, db, typeID+"-partial"); state != "PENDING" {
		t.Fatalf("partial cycle state=%s, want PENDING", state)
	}
}
