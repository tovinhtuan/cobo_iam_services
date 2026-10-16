package mysql

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	adhocapp "github.com/cobo/cobo_iam_services/internal/adhoc/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	_ "github.com/go-sql-driver/mysql"
)

// MR-2 (bug-adhoc-v12-2026-10-16): ad-hoc vote rules of business contract v1.2 §3.2, against a
// fully migrated schema (migrations/run_dev_migrations.sh order, seeds included):
//
//	MYSQL_TEST_DSN='<user>:<pass>@tcp(127.0.0.1:3306)/cobo_iam?parseTime=true&loc=UTC' go test ./internal/adhoc/infra/mysql/ -run AdhocVote
const adhocTestCompany = "c_001"

func openAdhocTestDB(t *testing.T) *sql.DB {
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

func seedPendingProposal(t *testing.T, db *sql.DB, repo *Repository, reviewers ...string) string {
	t.Helper()
	ctx := context.Background()
	var typeID string
	if err := db.QueryRowContext(ctx, `SELECT type_id FROM disclosure_types ORDER BY type_id LIMIT 1`).Scan(&typeID); err != nil {
		t.Fatalf("seeded disclosure type: %v", err)
	}
	id := "prop-mr2-" + time.Now().Format("150405.000000")
	t.Cleanup(func() {
		for _, q := range []string{
			`DELETE FROM ad_hoc_proposal_approvals WHERE proposal_id = ?`,
			`DELETE FROM ad_hoc_proposal_reviewers WHERE proposal_id = ?`,
			`DELETE FROM ad_hoc_proposals WHERE proposal_id = ?`,
		} {
			_, _ = db.ExecContext(context.Background(), q, id)
		}
	})
	t0 := "2026-11-01"
	if _, err := repo.Insert(ctx, adhocapp.ProposalDTO{
		ProposalID: id, CompanyID: adhocTestCompany, TypeID: typeID, Status: adhocapp.StatusPendingFocalApproval,
		CreatedBy: "m-mr2-creator", ProposedT0Date: &t0, ReviewerMembershipIDs: reviewers,
	}); err != nil {
		t.Fatal(err)
	}
	return id
}

func str(v string) *string { return &v }

func datePart(p *string) string {
	if v := deref(p); len(v) >= 10 {
		return v[:10]
	}
	return deref(p)
}

func deref(p *string) string {
	if p == nil {
		return "<nil>"
	}
	return *p
}

// H1: for the final dates and the note, the value entered last wins and a blank field keeps the
// value entered before it, whoever entered it.
func TestAdhocVote_BlankFinalFieldsKeepTheEarlierEntry(t *testing.T) {
	db := openAdhocTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	id := seedPendingProposal(t, db, repo, "m-mr2-a", "m-mr2-b", "m-mr2-c")
	vote := func(member string, t0, due *string, note string) *adhocapp.VoteReservation {
		t.Helper()
		r, err := repo.ReserveVote(ctx, adhocapp.ReserveVoteInput{
			CompanyID: adhocTestCompany, ProposalID: id, ActorMembershipID: member, ActorUserID: "u-" + member,
			FinalT0Date: t0, FinalDeadlineDate: due, AdjustmentNote: note,
		})
		if err != nil {
			t.Fatalf("vote by %s: %v", member, err)
		}
		return r
	}
	expect := func(step string, r *adhocapp.VoteReservation, t0, due, note string) {
		t.Helper()
		if deref(r.LastFinalT0Date) != t0 || deref(r.LastFinalDeadlineDate) != due || r.LastAdjustmentNote != note {
			t.Fatalf("%s: final t0=%s due=%s note=%q, want t0=%s due=%s note=%q",
				step, deref(r.LastFinalT0Date), deref(r.LastFinalDeadlineDate), r.LastAdjustmentNote, t0, due, note)
		}
	}

	expect("A enters every field", vote("m-mr2-a", str("2026-11-02"), str("2026-11-20"), "Dời theo lịch họp"), "2026-11-02", "2026-11-20", "Dời theo lịch họp")
	expect("A votes again with blanks", vote("m-mr2-a", nil, nil, ""), "2026-11-02", "2026-11-20", "Dời theo lịch họp")
	expect("B leaves every field blank", vote("m-mr2-b", nil, nil, ""), "2026-11-02", "2026-11-20", "Dời theo lịch họp")
	last := vote("m-mr2-c", str("2026-11-05"), nil, "")
	expect("C enters only T0", last, "2026-11-05", "2026-11-20", "Dời theo lịch họp")
	if !last.IsLastVote {
		t.Fatal("C's vote should be the last one")
	}

	// The proposal carries the current values, so the approval modal can show them (v1.2 §3.2).
	// With parseTime=true a DATE scanned as a string is RFC 3339, hence the date prefix.
	p, err := repo.FindByID(ctx, adhocTestCompany, id)
	if err != nil {
		t.Fatal(err)
	}
	if datePart(p.FinalT0Date) != "2026-11-05" || datePart(p.FinalDeadlineDate) != "2026-11-20" || p.AdjustmentNote != "Dời theo lịch họp" {
		t.Fatalf("proposal final t0=%s due=%s note=%q, want the current values", deref(p.FinalT0Date), deref(p.FinalDeadlineDate), p.AdjustmentNote)
	}
}

func castVote(t *testing.T, repo *Repository, id, member string) *adhocapp.VoteReservation {
	t.Helper()
	r, err := repo.ReserveVote(context.Background(), adhocapp.ReserveVoteInput{
		CompanyID: adhocTestCompany, ProposalID: id, ActorMembershipID: member, ActorUserID: "u-" + member,
	})
	if err != nil {
		t.Fatalf("vote by %s: %v", member, err)
	}
	return r
}

func proposalStatus(t *testing.T, db *sql.DB, id string) (status string, approvals int) {
	t.Helper()
	if err := db.QueryRowContext(context.Background(), `
		SELECT p.status, (SELECT COUNT(*) FROM ad_hoc_proposal_approvals a WHERE a.proposal_id = p.proposal_id)
		FROM ad_hoc_proposals p WHERE p.proposal_id = ?
	`, id).Scan(&status, &approvals); err != nil {
		t.Fatal(err)
	}
	return status, approvals
}

func assertAllReviewersApproved(t *testing.T, err error) {
	t.Helper()
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusConflict || he.Details["reason"] != adhocapp.ReasonAllReviewersApproved {
		t.Fatalf("err=%v, want 409 with reason %s", err, adhocapp.ReasonAllReviewersApproved)
	}
}

func refuseTransition(repo *Repository, id, status string) error {
	_, _, err := repo.UpdateStatus(context.Background(), adhocapp.StatusUpdate{
		ProposalID: id, CompanyID: adhocTestCompany, Status: status, FromStatus: adhocapp.StatusPendingFocalApproval,
		ActorMembershipID: "m-mr2-a", RejectReason: "late", RefuseIfAllApproved: true,
	})
	return err
}

// H2: once every reviewer has approved, reject and cancel get 409.
func TestAdhocVote_RejectAndCancelRefusedOnceEveryReviewerApproved(t *testing.T) {
	db := openAdhocTestDB(t)
	repo := NewRepository(db)
	id := seedPendingProposal(t, db, repo, "m-mr2-a", "m-mr2-b")
	castVote(t, repo, id, "m-mr2-a")
	if !castVote(t, repo, id, "m-mr2-b").IsLastVote {
		t.Fatal("B's vote should be the last one")
	}
	for _, target := range []string{adhocapp.StatusRejected, adhocapp.StatusCancelled} {
		assertAllReviewersApproved(t, refuseTransition(repo, id, target))
	}
	if status, _ := proposalStatus(t, db, id); status != adhocapp.StatusPendingFocalApproval {
		t.Fatalf("status=%q, want pending_focal_approval", status)
	}
}

// H2: a reject that runs while the last vote is being written waits for it and is then refused.
// It must not decide from counts read before it holds the proposal row lock.
func TestAdhocVote_RejectWaitsForTheLastVoteInFlight(t *testing.T) {
	db := openAdhocTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	id := seedPendingProposal(t, db, repo, "m-mr2-a", "m-mr2-b")
	castVote(t, repo, id, "m-mr2-a")

	// Like ReserveVote: lock the proposal row, then write the last vote.
	vtx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = vtx.Rollback() }()
	var status string
	if err := vtx.QueryRowContext(ctx, `SELECT status FROM ad_hoc_proposals WHERE proposal_id = ? FOR UPDATE`, id).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if _, err := vtx.ExecContext(ctx, `
		INSERT INTO ad_hoc_proposal_approvals (proposal_id, company_id, membership_id, approved_at) VALUES (?, ?, 'm-mr2-b', UTC_TIMESTAMP(3))
	`, id, adhocTestCompany); err != nil {
		t.Fatal(err)
	}

	rejected := make(chan error, 1)
	go func() { rejected <- refuseTransition(repo, id, adhocapp.StatusRejected) }()
	waitForAdhocLockWait(t, db)
	if err := vtx.Commit(); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-rejected:
	case <-time.After(30 * time.Second):
		t.Fatal("reject did not finish after the vote committed")
	}
	assertAllReviewersApproved(t, err)
}

// H3: a failed finalize releases its vote while the proposal is pending; a finalize then refuses to
// approve without every vote; once approved, nothing is released.
func TestAdhocVote_ReleasedVoteBlocksFinalizeUntilCastAgain(t *testing.T) {
	db := openAdhocTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	id := seedPendingProposal(t, db, repo, "m-mr2-a", "m-mr2-b")
	castVote(t, repo, id, "m-mr2-a")
	castVote(t, repo, id, "m-mr2-b")
	finalize := func() error {
		_, _, err := repo.CompleteFinalize(ctx, adhocapp.StatusUpdate{
			ProposalID: id, CompanyID: adhocTestCompany, Status: adhocapp.StatusApproved,
			FromStatus: adhocapp.StatusPendingFocalApproval, ActorMembershipID: "m-mr2-b",
		})
		return err
	}

	if released, err := repo.ReleaseVote(ctx, adhocTestCompany, id, "m-mr2-b", adhocapp.AdHocRecordID(adhocTestCompany, id)); err != nil || !released {
		t.Fatalf("release before any record: released=%v err=%v, want released", released, err)
	}
	if _, approvals := proposalStatus(t, db, id); approvals != 1 {
		t.Fatalf("approvals=%d after release, want 1", approvals)
	}
	if he, ok := perr.AsHTTPError(finalize()); !ok || he.HTTPStatus != http.StatusConflict {
		t.Fatalf("finalize with a released vote: err=%v, want 409", he)
	}

	castVote(t, repo, id, "m-mr2-b")
	if err := finalize(); err != nil {
		t.Fatalf("finalize with every vote: %v", err)
	}
	if released, err := repo.ReleaseVote(ctx, adhocTestCompany, id, "m-mr2-b", adhocapp.AdHocRecordID(adhocTestCompany, id)); err != nil || released {
		t.Fatalf("release after approval: released=%v err=%v, want a no-op", released, err)
	}
	if status, approvals := proposalStatus(t, db, id); status != adhocapp.StatusApproved || approvals != 2 {
		t.Fatalf("status=%q approvals=%d, want approved with both votes kept", status, approvals)
	}
}

func waitForAdhocLockWait(t *testing.T, db *sql.DB) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		var waiting int
		if err := db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM performance_schema.data_lock_waits`).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting > 0 {
			return
		}
	}
	t.Fatal("reject never waited for the proposal row lock")
}

// seedFinalizeRecord inserts the proposal's deterministic disclosure record, as a finalize that got
// as far as CreateRecord leaves it.
func seedFinalizeRecord(t *testing.T, db *sql.DB, proposalID string) {
	t.Helper()
	recordID := adhocapp.AdHocRecordID(adhocTestCompany, proposalID)
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM disclosure_records WHERE record_id = ?`, recordID)
	})
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO disclosure_records (record_id, company_id, department_id, title, content, status, created_by, updated_by)
		VALUES (?, ?, 'general', 'MR-2 finalize record', 'MR-2', 'PendingReview', 'm-mr2-creator', 'm-mr2-creator')
	`, recordID, adhocTestCompany); err != nil {
		t.Fatal(err)
	}
}

// H3 (review BES-01/DB-02): once finalize created its record, a failed finalize keeps the votes, so
// reject/cancel stay closed and a retry finishes the proposal instead of orphaning the record.
func TestAdhocVote_FinalizeRecordKeepsTheVotes(t *testing.T) {
	db := openAdhocTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	id := seedPendingProposal(t, db, repo, "m-mr2-a", "m-mr2-b")
	castVote(t, repo, id, "m-mr2-a")
	castVote(t, repo, id, "m-mr2-b")
	seedFinalizeRecord(t, db, id)

	released, err := repo.ReleaseVote(ctx, adhocTestCompany, id, "m-mr2-b", adhocapp.AdHocRecordID(adhocTestCompany, id))
	if err != nil || released {
		t.Fatalf("release with the record present: released=%v err=%v, want the vote kept", released, err)
	}
	if _, approvals := proposalStatus(t, db, id); approvals != 2 {
		t.Fatalf("approvals=%d, want both kept", approvals)
	}
	assertAllReviewersApproved(t, refuseTransition(repo, id, adhocapp.StatusRejected))
}

// Review DB-03: once finalize created its record from the current final values, a vote may not
// change them (the workflow runs on them); a blank or identical vote still goes through.
func TestAdhocVote_FinalValuesFreezeOnceTheRecordExists(t *testing.T) {
	db := openAdhocTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	id := seedPendingProposal(t, db, repo, "m-mr2-a", "m-mr2-b")
	recordID := adhocapp.AdHocRecordID(adhocTestCompany, id)
	vote := func(member string, t0 *string) error {
		_, err := repo.ReserveVote(ctx, adhocapp.ReserveVoteInput{
			CompanyID: adhocTestCompany, ProposalID: id, ActorMembershipID: member, ActorUserID: "u-" + member,
			FinalT0Date: t0, FinalizeRecordID: recordID,
		})
		return err
	}
	if err := vote("m-mr2-a", str("2026-11-02")); err != nil {
		t.Fatal(err)
	}
	if err := vote("m-mr2-b", nil); err != nil {
		t.Fatal(err)
	}
	seedFinalizeRecord(t, db, id)

	he, ok := perr.AsHTTPError(vote("m-mr2-b", str("2026-11-09")))
	if !ok || he.HTTPStatus != http.StatusConflict || he.Details["reason"] != adhocapp.ReasonFinalValuesLocked {
		t.Fatalf("changing T0 after the record exists: err=%v, want 409 %s", he, adhocapp.ReasonFinalValuesLocked)
	}
	for name, t0 := range map[string]*string{"blank": nil, "same value": str("2026-11-02")} {
		if err := vote("m-mr2-b", t0); err != nil {
			t.Fatalf("%s vote after the record exists: %v", name, err)
		}
	}
}

// Review BES-04/DB-04: a repeat vote keeps the reviewer's first approval time and comment.
func TestAdhocVote_RepeatVoteKeepsApprovalTimeAndComment(t *testing.T) {
	db := openAdhocTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	id := seedPendingProposal(t, db, repo, "m-mr2-a", "m-mr2-b")
	if _, err := repo.ReserveVote(ctx, adhocapp.ReserveVoteInput{
		CompanyID: adhocTestCompany, ProposalID: id, ActorMembershipID: "m-mr2-a", ActorUserID: "u-a", Comment: "Đồng ý",
	}); err != nil {
		t.Fatal(err)
	}
	read := func() (time.Time, sql.NullString) {
		var at time.Time
		var comment sql.NullString
		if err := db.QueryRowContext(ctx, `SELECT approved_at, comment FROM ad_hoc_proposal_approvals WHERE proposal_id = ? AND membership_id = 'm-mr2-a'`, id).Scan(&at, &comment); err != nil {
			t.Fatal(err)
		}
		return at, comment
	}
	firstAt, _ := read()
	time.Sleep(20 * time.Millisecond)
	castVote(t, repo, id, "m-mr2-a")
	at, comment := read()
	if !at.Equal(firstAt) || comment.String != "Đồng ý" {
		t.Fatalf("after a repeat vote approved_at=%v comment=%q, want %v and the first comment", at, comment.String, firstAt)
	}
}

// Review DB-01/BES-02: finalize is serialized per proposal across connections.
func TestAdhocVote_FinalizeLockSerializesFinalize(t *testing.T) {
	db := openAdhocTestDB(t)
	ctx := context.Background()
	wait := 1
	repo := NewRepository(db)
	repo.finalizeLockWaitSeconds = &wait

	unlock, err := repo.LockFinalize(ctx, adhocTestCompany, "prop-mr2-lock")
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.LockFinalize(ctx, adhocTestCompany, "prop-mr2-lock")
	if he, ok := perr.AsHTTPError(err); !ok || he.HTTPStatus != http.StatusConflict || he.Details["reason"] != adhocapp.ReasonFinalizeInProgress {
		t.Fatalf("second finalize while the first holds the lock: err=%v, want 409 %s", err, adhocapp.ReasonFinalizeInProgress)
	}
	other, err := repo.LockFinalize(ctx, adhocTestCompany, "prop-mr2-other")
	if err != nil {
		t.Fatalf("another proposal must not wait: %v", err)
	}
	other()
	unlock()
	again, err := repo.LockFinalize(ctx, adhocTestCompany, "prop-mr2-lock")
	if err != nil {
		t.Fatalf("after release: %v", err)
	}
	again()
}
