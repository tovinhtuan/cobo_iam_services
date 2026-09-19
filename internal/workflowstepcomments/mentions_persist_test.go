package workflowstepcomments_test

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	wsc "github.com/cobo/cobo_iam_services/internal/workflowstepcomments"
	"github.com/google/uuid"

	_ "github.com/go-sql-driver/mysql"
)

func mention(id, company, comment, membership string, start, end int) wsc.Mention {
	return wsc.Mention{
		ID:                    id,
		CompanyID:             company,
		CommentID:             comment,
		MentionedMembershipID: membership,
		StartOffset:           start,
		EndOffset:             end,
		CreatedAt:             time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC),
	}
}

func TestPrepareMentionsForPersist_Contract(t *testing.T) {
	_, err := wsc.PrepareMentionsForPersist("c1", "cm1", []wsc.Mention{
		mention(uuid.NewString(), "c1", "cm1", "m1", -1, 2),
	})
	if err == nil {
		t.Fatal("expected start < 0 rejected")
	}
	_, err = wsc.PrepareMentionsForPersist("c1", "cm1", []wsc.Mention{
		mention(uuid.NewString(), "c1", "cm1", "m1", 2, 2),
	})
	if err == nil {
		t.Fatal("expected end <= start rejected")
	}
	dupID1, dupID2 := uuid.NewString(), uuid.NewString()
	_, err = wsc.PrepareMentionsForPersist("c1", "cm1", []wsc.Mention{
		mention(dupID1, "c1", "cm1", "m1", 0, 2),
		mention(dupID2, "c1", "cm1", "m1", 0, 2),
	})
	if err == nil {
		t.Fatal("expected duplicate rejected")
	}
	tooMany := make([]wsc.Mention, wsc.MaxMentionsPerComment+1)
	for i := range tooMany {
		tooMany[i] = mention(uuid.NewString(), "c1", "cm1", "m"+uuid.NewString()[:8], i, i+1)
	}
	_, err = wsc.PrepareMentionsForPersist("c1", "cm1", tooMany)
	if err == nil {
		t.Fatal("expected max 20 rejected")
	}
	out, err := wsc.PrepareMentionsForPersist("c1", "cm1", []wsc.Mention{
		mention("id-b", "ignored", "ignored", "mb", 5, 8),
		mention("id-a", "ignored", "ignored", "ma", 1, 3),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || out[0].StartOffset != 1 || out[1].StartOffset != 5 {
		t.Fatalf("expected sorted by start: %+v", out)
	}
	if out[0].CompanyID != "c1" || out[0].CommentID != "cm1" {
		t.Fatal("company/comment must be forced from args")
	}
}

func TestMemoryMentionsRepository_ListReplaceDeleteIsolation(t *testing.T) {
	ctx := context.Background()
	repo := wsc.NewMemoryMentionsRepository()
	c1, c2 := "company-a", "company-b"
	cm1, cm2 := "comment-1", "comment-2"

	empty, err := repo.ListByComment(ctx, c1, cm1)
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty list: %v %#v", err, empty)
	}

	rows := []wsc.Mention{
		mention("u3", c1, cm1, "mem-c", 10, 12),
		mention("u1", c1, cm1, "mem-a", 0, 2),
		mention("u2", c1, cm1, "mem-b", 0, 5),
	}
	if err := repo.ReplaceForComment(ctx, nil, c1, cm1, rows); err != nil {
		t.Fatal(err)
	}
	got, err := repo.ListByComment(ctx, c1, cm1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 got %d", len(got))
	}
	if got[0].ID != "u1" || got[1].ID != "u2" || got[2].ID != "u3" {
		t.Fatalf("deterministic order failed: %+v", got)
	}

	// Same comment_id, other tenant must not see rows.
	cross, err := repo.ListByComment(ctx, c2, cm1)
	if err != nil || len(cross) != 0 {
		t.Fatalf("cross-tenant leak: %v %#v", err, cross)
	}

	if err := repo.ReplaceForComment(ctx, nil, c2, cm1, []wsc.Mention{
		mention("other", c2, cm1, "mem-x", 0, 1),
	}); err != nil {
		t.Fatal(err)
	}
	a, _ := repo.ListByComment(ctx, c1, cm1)
	b, _ := repo.ListByComment(ctx, c2, cm1)
	if len(a) != 3 || len(b) != 1 {
		t.Fatalf("tenant isolation broken a=%d b=%d", len(a), len(b))
	}

	if err := repo.ReplaceForComment(ctx, nil, c1, cm2, []wsc.Mention{
		mention("cm2-1", c1, cm2, "mem-z", 1, 2),
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteByComment(ctx, nil, c1, cm1); err != nil {
		t.Fatal(err)
	}
	a, _ = repo.ListByComment(ctx, c1, cm1)
	other, _ := repo.ListByComment(ctx, c1, cm2)
	otherTenant, _ := repo.ListByComment(ctx, c2, cm1)
	if len(a) != 0 || len(other) != 1 || len(otherTenant) != 1 {
		t.Fatalf("delete must be company+comment scoped: a=%d other=%d ot=%d", len(a), len(other), len(otherTenant))
	}
}

func TestMemoryMentionsRepository_ReplaceAtomicity(t *testing.T) {
	ctx := context.Background()
	repo := wsc.NewMemoryMentionsRepository()
	c, cm := "c1", "cm1"
	initial := []wsc.Mention{mention("old", c, cm, "m-old", 0, 3)}
	if err := repo.ReplaceForComment(ctx, nil, c, cm, initial); err != nil {
		t.Fatal(err)
	}

	repo.FailAfterDelete = true
	err := repo.ReplaceForComment(ctx, nil, c, cm, []wsc.Mention{
		mention("new", c, cm, "m-new", 0, 1),
	})
	if err == nil {
		t.Fatal("expected simulated failure")
	}
	got, _ := repo.ListByComment(ctx, c, cm)
	if len(got) != 1 || got[0].ID != "old" {
		t.Fatalf("replace failure must keep prior state: %+v", got)
	}

	repo.FailAfterDelete = false
	repo.FailOnInsertID = "bad"
	err = repo.ReplaceForComment(ctx, nil, c, cm, []wsc.Mention{
		mention("ok", c, cm, "m1", 0, 1),
		mention("bad", c, cm, "m2", 2, 3),
	})
	if err == nil {
		t.Fatal("expected partial insert failure")
	}
	got, _ = repo.ListByComment(ctx, c, cm)
	if len(got) != 1 || got[0].ID != "old" {
		t.Fatalf("partial insert must not leave new rows: %+v", got)
	}
}

func TestMemoryMentionsRepository_TxSnapshotRollback(t *testing.T) {
	ctx := context.Background()
	repo := wsc.NewMemoryMentionsRepository()
	c, cm := "c1", "cm1"
	if err := repo.ReplaceForComment(ctx, nil, c, cm, []wsc.Mention{
		mention("v1", c, cm, "m1", 0, 2),
	}); err != nil {
		t.Fatal(err)
	}

	snap := repo.SnapshotMentions()
	if err := repo.ReplaceForComment(ctx, nil, c, cm, []wsc.Mention{
		mention("v2", c, cm, "m2", 0, 1),
	}); err != nil {
		t.Fatal(err)
	}
	repo.RestoreMentions(snap)
	got, _ := repo.ListByComment(ctx, c, cm)
	if len(got) != 1 || got[0].ID != "v1" {
		t.Fatalf("rollback must restore prior mentions: %+v", got)
	}
}

func TestMemoryMentionsRepository_SoftDeletedParentKeepsRows(t *testing.T) {
	// Soft-delete of parent comment does not call DeleteByComment in Phase 2.
	// Mention rows remain; LIST comments layer (Phase 3) will not expose them.
	ctx := context.Background()
	comments := wsc.NewMemoryRepository()
	mentions := wsc.NewMemoryMentionsRepository()
	owner := wsc.OwnerContext{CompanyID: "c1", DisclosureRecordID: "r1", WorkflowInstanceID: "w1", StepCode: "s1"}
	cmID := uuid.NewString()
	comments.Seed(wsc.Comment{
		ID: cmID, CompanyID: "c1", DisclosureRecordID: "r1", WorkflowInstanceID: "w1", StepCode: "s1",
		AuthorUserID: "u1", AuthorMembershipID: "m1", Body: "hello @x", CreatedAt: time.Now().UTC(),
	})
	if err := mentions.ReplaceForComment(ctx, nil, "c1", cmID, []wsc.Mention{
		mention(uuid.NewString(), "c1", cmID, "m-target", 6, 8),
	}); err != nil {
		t.Fatal(err)
	}
	if err := comments.SoftDelete(ctx, owner, cmID, "m1", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	alive, total, err := comments.ListAliveByStep(ctx, owner, 1, 20)
	if err != nil || total != 0 || len(alive) != 0 {
		t.Fatalf("soft-deleted comment must disappear from LIST: total=%d alive=%d err=%v", total, len(alive), err)
	}
	still, err := mentions.ListByComment(ctx, "c1", cmID)
	if err != nil || len(still) != 1 {
		t.Fatalf("mention rows must remain after parent soft-delete (no auto hard-delete): %v %#v", err, still)
	}
}

func TestMySQLMentionsRepository_OptionalDSN(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("MYSQL_TEST_DSN"))
	if dsn == "" {
		t.Skip("MYSQL_TEST_DSN not set — memory repo covers semantics; MySQL optional")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		t.Fatal(err)
	}

	// Ensure table exists (ephemeral apply of CREATE IF NOT EXISTS from migration text).
	up, err := os.ReadFile("../../../migrations/0140_workflow_step_comment_mentions.up.sql")
	if err != nil {
		// try from module root relative to test package
		up, err = os.ReadFile("migrations/0140_workflow_step_comment_mentions.up.sql")
	}
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	for _, stmt := range splitSQLStatements(string(up)) {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("apply stmt: %v\n%s", err, stmt)
		}
	}

	repo := wsc.NewMySQLMentionsRepository(db)
	ctx := context.Background()
	c, cm := "test-co-"+uuid.NewString()[:8], uuid.NewString()
	defer func() { _ = repo.DeleteByComment(ctx, nil, c, cm) }()

	rows := []wsc.Mention{
		mention(uuid.NewString(), c, cm, "mem-b", 5, 7),
		mention(uuid.NewString(), c, cm, "mem-a", 1, 3),
	}
	if err := repo.RunInTx(ctx, func(tx *sql.Tx) error {
		return repo.ReplaceForComment(ctx, tx, c, cm, rows)
	}); err != nil {
		t.Fatal(err)
	}
	got, err := repo.ListByComment(ctx, c, cm)
	if err != nil || len(got) != 2 || got[0].MentionedMembershipID != "mem-a" {
		t.Fatalf("list/order: %v %+v", err, got)
	}

	// Explicit Begin + Rollback must leave no rows.
	cm3 := uuid.NewString()
	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.ReplaceForComment(ctx, tx, c, cm3, []wsc.Mention{
		mention(uuid.NewString(), c, cm3, "m1", 0, 2),
	}); err != nil {
		t.Fatal(err)
	}
	_ = tx.Rollback()
	got, _ = repo.ListByComment(ctx, c, cm3)
	if len(got) != 0 {
		t.Fatalf("rollback must leave no mention rows: %+v", got)
	}

	// Cross-tenant replace/delete must not touch other company rows.
	otherCo := "other-" + uuid.NewString()[:8]
	if err := repo.ReplaceForComment(ctx, nil, otherCo, cm, []wsc.Mention{
		mention(uuid.NewString(), otherCo, cm, "ox", 0, 1),
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteByComment(ctx, nil, c, cm); err != nil {
		t.Fatal(err)
	}
	stillOther, _ := repo.ListByComment(ctx, otherCo, cm)
	gone, _ := repo.ListByComment(ctx, c, cm)
	if len(gone) != 0 || len(stillOther) != 1 {
		t.Fatalf("tenant scoped delete failed gone=%d other=%d", len(gone), len(stillOther))
	}
	_ = repo.DeleteByComment(ctx, nil, otherCo, cm)
}

func splitSQLStatements(s string) []string {
	var out []string
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "--") {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
		if strings.HasSuffix(trim, ";") {
			out = append(out, b.String())
			b.Reset()
		}
	}
	if rest := strings.TrimSpace(b.String()); rest != "" {
		out = append(out, rest)
	}
	return out
}
