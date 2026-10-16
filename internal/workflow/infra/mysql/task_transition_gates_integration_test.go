package mysql

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
	wff "github.com/cobo/cobo_iam_services/internal/workflowfulfillment"
	_ "github.com/go-sql-driver/mysql"
)

// Run against a fully migrated schema (migrations/run_dev_migrations.sh order, seeds included):
//
//	MYSQL_TEST_DSN='<user>:<pass>@tcp(127.0.0.1:3306)/cobo_iam?parseTime=true&loc=UTC' go test ./internal/workflow/infra/mysql/ -run TaskTransitionGates
func openTaskGatesDB(t *testing.T) *sql.DB {
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

type gatesAllowAuth struct{}

func (gatesAllowAuth) Authorize(context.Context, authapp.AuthorizeRequest) (*authapp.AuthorizeDecision, error) {
	return &authapp.AuthorizeDecision{Decision: authapp.DecisionAllow}, nil
}
func (gatesAllowAuth) AuthorizeBatch(context.Context, authapp.AuthorizeBatchRequest) (*authapp.AuthorizeBatchResponse, error) {
	return &authapp.AuthorizeBatchResponse{}, nil
}
func (gatesAllowAuth) GetEffectiveAccess(context.Context, string, string) (*authapp.EffectiveAccessSummary, error) {
	return nil, nil
}

const gatesCompany = "c_001" // seeded by the dev migrations

func cleanupInstance(t *testing.T, db *sql.DB, instanceID string) {
	t.Cleanup(func() {
		ctx := context.Background()
		for _, q := range []string{
			`DELETE FROM workflow_step_document_fulfillment_files WHERE workflow_instance_id = ?`,
			`DELETE FROM workflow_step_document_requirement_snapshots WHERE workflow_instance_id = ?`,
			`DELETE FROM workflow_instance_step_states WHERE workflow_instance_id = ?`,
			`DELETE FROM workflow_task_assignees WHERE task_id IN (SELECT task_id FROM workflow_tasks WHERE workflow_instance_id = ?)`,
			`DELETE FROM workflow_tasks WHERE workflow_instance_id = ?`,
			`DELETE FROM workflow_instances WHERE workflow_instance_id = ?`,
		} {
			_, _ = db.ExecContext(ctx, q, instanceID)
		}
	})
}

func onlyTask(t *testing.T, repo *Repository, instanceID string) workflowapp.TaskDTO {
	t.Helper()
	tasks, err := repo.ListTasksByInstance(context.Background(), gatesCompany, instanceID)
	if err != nil || len(tasks) != 1 {
		t.Fatalf("tasks=%v err=%v", tasks, err)
	}
	return tasks[0]
}

// bug-workflow-step-skip-2026-10-16 (WF-02): approving a task completes its step, so it must pass the
// same required-document gate (B3) as completing the step on the deadline screen.
func TestTaskTransitionGates_ApproveRunsRequiredDocumentGate(t *testing.T) {
	db := openTaskGatesDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	svc := workflowapp.NewService(repo, gatesAllowAuth{}, idgen.UUIDv7Generator{}, workflowapp.WithFlags(workflowapp.Flags{SnapshotEnabled: true}))
	assignee := seededAssignee(t, db)

	inst, err := svc.CreateWorkflowInstanceInternal(ctx, workflowapp.CreateWorkflowInstanceRequest{
		Subject:                        workflowapp.Subject{UserID: "u-gate", MembershipID: "m-gate-creator", CompanyID: gatesCompany},
		RecordID:                       "rec-gate-" + time.Now().Format("150405.000000"),
		WorkflowSource:                 workflowapp.WorkflowSourceProposalSnapshotV3,
		Snapshot:                       []workflowapp.StepSnapshot{{StepID: "s1", StepCode: "s1", DisplayOrder: 1, ProcessingDays: 1}},
		FirstTaskAssigneeMembershipIDs: []string{assignee.MembershipID},
		DocumentRequirements: []workflowapp.DocumentRequirementSnapshot{
			{StepCode: "s1", SourceDocID: "doc-bctc", Name: "Báo cáo tài chính", Required: true},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	cleanupInstance(t, db, inst.WorkflowInstanceID)
	task := onlyTask(t, repo, inst.WorkflowInstanceID)

	_, err = svc.ApproveTask(ctx, workflowapp.TaskActionRequest{Subject: assignee, TaskID: task.TaskID})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusUnprocessableEntity || he.Code != perr.CodeWorkflowStepRequiredDocumentMissing {
		t.Fatalf("approve without the required document: err=%v, want 422 %s", err, perr.CodeWorkflowStepRequiredDocumentMissing)
	}
	if got := onlyTask(t, repo, inst.WorkflowInstanceID); got.Status != "pending" {
		t.Fatalf("task status=%q after a blocked approve, want pending", got.Status)
	}

	uploadRequiredDocument(t, db, repo, inst, "s1", assignee.MembershipID)
	if _, err := svc.ApproveTask(ctx, workflowapp.TaskActionRequest{Subject: assignee, TaskID: task.TaskID}); err != nil {
		t.Fatalf("approve with the required document uploaded: %v", err)
	}
}

// v3 task assignees reference memberships(membership_id): use a seeded one.
func seededAssignee(t *testing.T, db *sql.DB) workflowapp.Subject {
	t.Helper()
	var membershipID string
	if err := db.QueryRowContext(context.Background(), `SELECT membership_id FROM memberships WHERE company_id = ? ORDER BY membership_id LIMIT 1`, gatesCompany).Scan(&membershipID); err != nil {
		t.Fatalf("seeded membership for %s: %v", gatesCompany, err)
	}
	return workflowapp.Subject{UserID: "u-gate", MembershipID: membershipID, CompanyID: gatesCompany}
}

func uploadRequiredDocument(t *testing.T, db *sql.DB, repo *Repository, inst *workflowapp.WorkflowInstanceDTO, stepCode, uploader string) {
	t.Helper()
	ctx := context.Background()
	snaps, err := repo.ListDocumentRequirementSnapshotsByInstanceStep(ctx, gatesCompany, inst.WorkflowInstanceID, stepCode)
	if err != nil || len(snaps) != 1 {
		t.Fatalf("snapshots=%v err=%v", snaps, err)
	}
	// Like UploadInTx: the upload leaves an open step-state row (completed_at NULL).
	if _, err := db.ExecContext(ctx, `
		INSERT IGNORE INTO workflow_instance_step_states (company_id, workflow_instance_id, step_code) VALUES (?, ?, ?)
	`, gatesCompany, inst.WorkflowInstanceID, stepCode); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO workflow_step_document_fulfillment_files (
			id, company_id, disclosure_record_id, workflow_instance_id, step_code, requirement_snapshot_id,
			storage_key, original_file_name, mime_type, file_size, uploaded_by, uploaded_at, lifecycle_status
		) VALUES (UUID(), ?, ?, ?, ?, ?, 'k', 'doc.pdf', 'application/pdf', 10, ?, UTC_TIMESTAMP(3), ?)
	`, gatesCompany, inst.RecordID, inst.WorkflowInstanceID, stepCode, snaps[0].ID, uploader, wff.LifecycleActive); err != nil {
		t.Fatal(err)
	}
}

func stepState(t *testing.T, db *sql.DB, instanceID, stepCode string) (completedAt sql.NullTime, completedBy sql.NullString) {
	t.Helper()
	err := db.QueryRowContext(context.Background(), `
		SELECT completed_at, completed_by_membership_id FROM workflow_instance_step_states
		WHERE workflow_instance_id = ? AND step_code = ?
	`, instanceID, stepCode).Scan(&completedAt, &completedBy)
	if err != nil && err != sql.ErrNoRows {
		t.Fatal(err)
	}
	return completedAt, completedBy
}

// Review DB-01: approving a proposal task completes its step in the step runtime. Otherwise step 2
// never becomes current there, its documents cannot be uploaded (WORKFLOW_STEP_NOT_CURRENT), and
// its task is stuck behind the required-document gate.
func TestTaskTransitionGates_ProposalTaskCompletesItsStep(t *testing.T) {
	db := openTaskGatesDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	svc := workflowapp.NewService(repo, gatesAllowAuth{}, idgen.UUIDv7Generator{}, workflowapp.WithFlags(workflowapp.Flags{SnapshotEnabled: true}))
	assignee := seededAssignee(t, db)

	inst, err := svc.CreateWorkflowInstanceInternal(ctx, workflowapp.CreateWorkflowInstanceRequest{
		Subject:        workflowapp.Subject{UserID: "u-gate", MembershipID: "m-gate-creator", CompanyID: gatesCompany},
		RecordID:       "rec-steps-" + time.Now().Format("150405.000000"),
		WorkflowSource: workflowapp.WorkflowSourceProposalSnapshotV3,
		Snapshot: []workflowapp.StepSnapshot{
			{StepID: "s1", StepCode: "s1", DisplayOrder: 1, ProcessingDays: 1},
			{StepID: "s2", StepCode: "s2", DisplayOrder: 2, ProcessingDays: 1, AssigneeMembershipIDs: []string{assignee.MembershipID}},
		},
		FirstTaskAssigneeMembershipIDs: []string{assignee.MembershipID},
		DocumentRequirements: []workflowapp.DocumentRequirementSnapshot{
			{StepCode: "s2", SourceDocID: "doc-nq", Name: "Nghị quyết", Required: true},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	cleanupInstance(t, db, inst.WorkflowInstanceID)
	pending := func() workflowapp.TaskDTO {
		t.Helper()
		tasks, err := repo.ListTasksByInstance(ctx, gatesCompany, inst.WorkflowInstanceID)
		if err != nil {
			t.Fatal(err)
		}
		for _, task := range tasks {
			if task.Status == "pending" {
				return task
			}
		}
		t.Fatalf("no pending task in %v", tasks)
		return workflowapp.TaskDTO{}
	}
	approve := func(task workflowapp.TaskDTO) error {
		_, err := svc.ApproveTask(ctx, workflowapp.TaskActionRequest{Subject: assignee, TaskID: task.TaskID})
		return err
	}

	if err := approve(pending()); err != nil {
		t.Fatalf("approve step 1 (no required document): %v", err)
	}
	at, by := stepState(t, db, inst.WorkflowInstanceID, "s1")
	if !at.Valid || by.String != assignee.MembershipID {
		t.Fatalf("step s1 completed_at=%v by=%v, want completed by the approver", at, by)
	}
	if done, _ := repo.ListCompletedStepCodes(ctx, gatesCompany, inst.WorkflowInstanceID); len(done) != 1 || done[0] != "s1" {
		t.Fatalf("completed steps=%v, want [s1] so s2 is current", done)
	}

	second := pending()
	he, ok := perr.AsHTTPError(approve(second))
	if !ok || he.HTTPStatus != http.StatusUnprocessableEntity || he.Code != perr.CodeWorkflowStepRequiredDocumentMissing {
		t.Fatalf("approve step 2 without its document: err=%v, want 422 %s", he, perr.CodeWorkflowStepRequiredDocumentMissing)
	}
	if at, _ := stepState(t, db, inst.WorkflowInstanceID, "s2"); at.Valid {
		t.Fatal("a blocked approve completed step s2")
	}

	uploadRequiredDocument(t, db, repo, inst, "s2", assignee.MembershipID)
	if err := approve(second); err != nil {
		t.Fatalf("approve step 2 with its document: %v", err)
	}
	// The upload left an open row for s2, so this completion goes through ON DUPLICATE KEY UPDATE.
	if at, by := stepState(t, db, inst.WorkflowInstanceID, "s2"); !at.Valid || by.String != assignee.MembershipID {
		t.Fatalf("step s2 completed_at=%v by=%v, want completed by the approver", at, by)
	}
	if got, _ := repo.FindInstance(ctx, gatesCompany, inst.WorkflowInstanceID); got.Status != "approved" {
		t.Fatalf("instance status=%q, want approved", got.Status)
	}
}

// completed_at is write-once: a task approved after its step was completed keeps that completion.
func TestTaskTransitionGates_ProposalTaskKeepsAnEarlierStepCompletion(t *testing.T) {
	db := openTaskGatesDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	svc := workflowapp.NewService(repo, gatesAllowAuth{}, idgen.UUIDv7Generator{}, workflowapp.WithFlags(workflowapp.Flags{SnapshotEnabled: true}))
	assignee := seededAssignee(t, db)

	inst, err := svc.CreateWorkflowInstanceInternal(ctx, workflowapp.CreateWorkflowInstanceRequest{
		Subject:                        workflowapp.Subject{UserID: "u-gate", MembershipID: "m-gate-creator", CompanyID: gatesCompany},
		RecordID:                       "rec-keep-" + time.Now().Format("150405.000000"),
		WorkflowSource:                 workflowapp.WorkflowSourceProposalSnapshotV3,
		Snapshot:                       []workflowapp.StepSnapshot{{StepID: "s1", StepCode: "s1", DisplayOrder: 1, ProcessingDays: 1}},
		FirstTaskAssigneeMembershipIDs: []string{assignee.MembershipID},
	})
	if err != nil {
		t.Fatal(err)
	}
	cleanupInstance(t, db, inst.WorkflowInstanceID)
	earlier := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if _, err := db.ExecContext(ctx, `
		INSERT INTO workflow_instance_step_states (company_id, workflow_instance_id, step_code, completed_at, completed_by_membership_id)
		VALUES (?, ?, 's1', ?, 'm-earlier')
	`, gatesCompany, inst.WorkflowInstanceID, earlier); err != nil {
		t.Fatal(err)
	}

	task := onlyTask(t, repo, inst.WorkflowInstanceID)
	if _, err := svc.ApproveTask(ctx, workflowapp.TaskActionRequest{Subject: assignee, TaskID: task.TaskID}); err != nil {
		t.Fatal(err)
	}
	at, by := stepState(t, db, inst.WorkflowInstanceID, "s1")
	if !at.Time.Equal(earlier) || by.String != "m-earlier" {
		t.Fatalf("step s1 completed_at=%v by=%v, want the earlier completion kept", at.Time, by.String)
	}
}

// bug-workflow-step-skip-2026-10-16 (WF-01): a template workflow may only be completed through its
// step-1 task once every snapshot step is completed in workflow_instance_step_states.
func TestTaskTransitionGates_TemplateCompletionNeedsEveryStepCompleted(t *testing.T) {
	db := openTaskGatesDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	svc := workflowapp.NewService(repo, gatesAllowAuth{}, idgen.UUIDv7Generator{}, workflowapp.WithFlags(workflowapp.Flags{SnapshotEnabled: true}))
	submitter := workflowapp.Subject{UserID: "u-tpl", MembershipID: "m-tpl-submitter", CompanyID: gatesCompany}

	inst, err := svc.CreateWorkflowInstanceInternal(ctx, workflowapp.CreateWorkflowInstanceRequest{
		Subject:        submitter,
		RecordID:       "rec-tpl-" + time.Now().Format("150405.000000"),
		WorkflowSource: "global_template",
		Snapshot: []workflowapp.StepSnapshot{
			{StepID: "prepare", StepCode: "prepare", DisplayOrder: 1, ProcessingDays: 2},
			{StepID: "review", StepCode: "review", DisplayOrder: 2, ProcessingDays: 1},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	cleanupInstance(t, db, inst.WorkflowInstanceID)
	task := onlyTask(t, repo, inst.WorkflowInstanceID)
	completeStep := func(code string) {
		t.Helper()
		if _, err := db.ExecContext(ctx, `
			INSERT INTO workflow_instance_step_states (company_id, workflow_instance_id, step_code, completed_at, completed_by_membership_id)
			VALUES (?, ?, ?, UTC_TIMESTAMP(), 'm-step-owner')
		`, gatesCompany, inst.WorkflowInstanceID, code); err != nil {
			t.Fatal(err)
		}
	}
	approve := func() error {
		_, err := svc.ApproveTask(ctx, workflowapp.TaskActionRequest{Subject: submitter, TaskID: task.TaskID})
		return err
	}

	for _, done := range []string{"", "prepare"} {
		if done != "" {
			completeStep(done)
		}
		he, ok := perr.AsHTTPError(approve())
		if !ok || he.HTTPStatus != http.StatusConflict {
			t.Fatalf("approve with steps open (done=%q): err=%v, want 409", done, he)
		}
		got, _ := repo.FindInstance(ctx, gatesCompany, inst.WorkflowInstanceID)
		if got.Status != "in_progress" {
			t.Fatalf("instance status=%q with steps open, want in_progress", got.Status)
		}
	}

	completeStep("review")
	if err := approve(); err != nil {
		t.Fatalf("approve after every step completed: %v", err)
	}
	got, _ := repo.FindInstance(ctx, gatesCompany, inst.WorkflowInstanceID)
	if got.Status != "approved" {
		t.Fatalf("instance status=%q after every step completed, want approved", got.Status)
	}
}

// Review DB-04: the gate must count fulfillment files as of its locks. A delete that holds the
// step-state lock (as DeleteInTx does) and commits while the approve waits for that lock must fail
// the gate. A plain read early in the approve transaction would freeze an older view and let it pass.
// Needs SELECT on performance_schema.data_lock_waits to see the lock wait.
func TestTaskTransitionGates_GateSeesADeleteCommittedWhileItWaits(t *testing.T) {
	db := openTaskGatesDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	svc := workflowapp.NewService(repo, gatesAllowAuth{}, idgen.UUIDv7Generator{}, workflowapp.WithFlags(workflowapp.Flags{SnapshotEnabled: true}))
	assignee := seededAssignee(t, db)

	inst, err := svc.CreateWorkflowInstanceInternal(ctx, workflowapp.CreateWorkflowInstanceRequest{
		Subject:                        workflowapp.Subject{UserID: "u-gate", MembershipID: "m-gate-creator", CompanyID: gatesCompany},
		RecordID:                       "rec-race-" + time.Now().Format("150405.000000"),
		WorkflowSource:                 workflowapp.WorkflowSourceProposalSnapshotV3,
		Snapshot:                       []workflowapp.StepSnapshot{{StepID: "s1", StepCode: "s1", DisplayOrder: 1, ProcessingDays: 1}},
		FirstTaskAssigneeMembershipIDs: []string{assignee.MembershipID},
		DocumentRequirements: []workflowapp.DocumentRequirementSnapshot{
			{StepCode: "s1", SourceDocID: "doc-bctc", Name: "Báo cáo tài chính", Required: true},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	cleanupInstance(t, db, inst.WorkflowInstanceID)
	uploadRequiredDocument(t, db, repo, inst, "s1", assignee.MembershipID)
	task := onlyTask(t, repo, inst.WorkflowInstanceID)

	del, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = del.Rollback() }()
	var completedAt sql.NullTime
	if err := del.QueryRowContext(ctx, `
		SELECT completed_at FROM workflow_instance_step_states WHERE workflow_instance_id = ? AND step_code = 's1' FOR UPDATE
	`, inst.WorkflowInstanceID).Scan(&completedAt); err != nil {
		t.Fatal(err)
	}
	if _, err := del.ExecContext(ctx, `
		UPDATE workflow_step_document_fulfillment_files SET lifecycle_status = ? WHERE workflow_instance_id = ?
	`, wff.LifecycleDeleted, inst.WorkflowInstanceID); err != nil {
		t.Fatal(err)
	}

	approved := make(chan error, 1)
	go func() {
		_, err := svc.ApproveTask(ctx, workflowapp.TaskActionRequest{Subject: assignee, TaskID: task.TaskID})
		approved <- err
	}()
	waitForLockWait(t, db)
	if err := del.Commit(); err != nil {
		t.Fatal(err)
	}

	select {
	case err = <-approved:
	case <-time.After(30 * time.Second):
		t.Fatal("approve did not finish after the delete committed")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusUnprocessableEntity || he.Code != perr.CodeWorkflowStepRequiredDocumentMissing {
		t.Fatalf("approve after the only required file was deleted: err=%v, want 422 %s", err, perr.CodeWorkflowStepRequiredDocumentMissing)
	}
	if at, _ := stepState(t, db, inst.WorkflowInstanceID, "s1"); at.Valid {
		t.Fatal("step s1 completed without its required document")
	}
}

func waitForLockWait(t *testing.T, db *sql.DB) {
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
	t.Fatal("approve never waited for the step-state lock")
}
