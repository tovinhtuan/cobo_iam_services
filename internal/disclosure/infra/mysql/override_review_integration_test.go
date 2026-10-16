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

// MR-4 (PO decision Q5-B) on MySQL 8 (migration 0156): submit / approve / reject the latest override
// version only, self-approval reported, the pending list with names. Same DSN as
// departmentFilterTestDB:
//
//	MYSQL_TEST_DSN='<user>:<pass>@tcp(127.0.0.1:3306)/cobo_iam?parseTime=true' go test ./internal/disclosure/infra/mysql/ -run 'OverrideReview'

const reviewTestCompany = "c_001"

func seedReviewType(t *testing.T, db *sql.DB) (typeID, writerID, approverID string) {
	t.Helper()
	ctx := context.Background()
	stamp := time.Now().Format("150405.000000")
	typeID = "it-mr4-" + stamp
	writerID, approverID = "it-mr4-w-"+stamp, "it-mr4-a-"+stamp
	var groupID string
	if err := db.QueryRowContext(ctx, `SELECT group_id FROM disclosure_type_groups ORDER BY group_id LIMIT 1`).Scan(&groupID); err != nil {
		t.Fatalf("seeded disclosure type group: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = db.ExecContext(bg, `DELETE v FROM company_template_workflow_override_versions v JOIN company_template_workflow_overrides o ON o.override_id = v.override_id WHERE o.type_id = ?`, typeID)
		_, _ = db.ExecContext(bg, `DELETE FROM company_template_workflow_overrides WHERE type_id = ?`, typeID)
		_, _ = db.ExecContext(bg, `DELETE FROM disclosure_type_versions WHERE type_id = ?`, typeID)
		_, _ = db.ExecContext(bg, `DELETE FROM disclosure_types WHERE type_id = ?`, typeID)
		_, _ = db.ExecContext(bg, `DELETE FROM users WHERE user_id IN (?, ?)`, writerID, approverID)
	})
	if _, err := db.ExecContext(ctx, `INSERT INTO disclosure_types (type_id, company_id, group_id, status, active_version_no) VALUES (?, NULL, ?, 'active', 1)`, typeID, groupID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO disclosure_type_versions (type_id, version_no, name, category, template_category, description, updated_by)
		VALUES (?, 1, 'MR-4 review test', 'periodic', 'periodic', '', 'it-mr4')
	`, typeID); err != nil {
		t.Fatal(err)
	}
	for id, name := range map[string]string{writerID: "Người soạn MR-4", approverID: "Người duyệt MR-4"} {
		if _, err := db.ExecContext(ctx, `INSERT INTO users (user_id, login_id, full_name) VALUES (?, ?, ?)`, id, id+"@mr4.test", name); err != nil {
			t.Fatal(err)
		}
	}
	return typeID, writerID, approverID
}

func reviewDraft(t *testing.T, repo *Repository, typeID, userID string) int {
	t.Helper()
	resp, err := repo.UpsertCompanyWorkflowOverrideDraft(context.Background(), disclosureapp.UpsertCompanyWorkflowOverrideDraftRequest{
		Subject:  disclosureapp.Subject{UserID: userID, CompanyID: reviewTestCompany},
		TypeID:   typeID,
		Workflow: []disclosureapp.WorkflowStepDTO{{StepID: "s1", Stage: "Soạn thảo", Description: "Chuẩn bị hồ sơ"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return resp.DraftVersionNo
}

func wantHTTPCode(t *testing.T, err error, status int, code perr.Code) {
	t.Helper()
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != status || he.Code != code {
		t.Fatalf("err=%v, want %d %s", err, status, code)
	}
}

func TestOverrideReview_SubmitApproveRejectOnMySQL(t *testing.T) {
	db := departmentFilterTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	typeID, writerID, approverID := seedReviewType(t, db)
	writer := disclosureapp.Subject{UserID: writerID, CompanyID: reviewTestCompany}
	approver := disclosureapp.Subject{UserID: approverID, CompanyID: reviewTestCompany}

	v1 := reviewDraft(t, repo, typeID, writerID)
	sub, err := repo.SubmitCompanyWorkflowOverride(ctx, disclosureapp.SubmitCompanyWorkflowOverrideRequest{Subject: writer, TypeID: typeID, VersionNo: v1, Note: "nhờ duyệt"})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if sub.State != "pending_approval" || sub.SubmittedBy != writerID || sub.SubmittedAt == nil || sub.SubmitNote != "nhờ duyệt" || sub.DecisionNote != "" {
		t.Fatalf("submitted=%+v", sub)
	}
	view, err := repo.GetCompanyWorkflowOverride(ctx, reviewTestCompany, typeID)
	if err != nil || view.DraftVersion == nil || view.DraftVersion.State != "pending_approval" || view.DraftVersion.SubmittedBy != writerID {
		t.Fatalf("GET after submit: view=%+v err=%v", view, err)
	}
	pending, err := repo.ListPendingCompanyWorkflowOverrides(ctx, reviewTestCompany, 100)
	if err != nil {
		t.Fatal(err)
	}
	var item *disclosureapp.PendingCompanyWorkflowOverrideItem
	for i := range pending {
		if pending[i].TypeID == typeID {
			item = &pending[i]
		}
	}
	if item == nil || item.VersionNo != v1 || item.TypeName != "MR-4 review test" || item.CreatedByName != "Người soạn MR-4" || item.SubmittedByName != "Người soạn MR-4" || item.SubmittedAt == nil || item.SubmitNote != "nhờ duyệt" {
		t.Fatalf("pending item=%+v", item)
	}

	approved, err := repo.ApproveCompanyWorkflowOverride(ctx, disclosureapp.ApproveCompanyWorkflowOverrideRequest{Subject: approver, TypeID: typeID, VersionNo: v1})
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if approved.SelfApproved || approved.FromState != "pending_approval" || approved.ActiveVersionNo != v1 {
		t.Fatalf("approve=%+v", approved)
	}
	if view, _ := repo.GetCompanyWorkflowOverride(ctx, reviewTestCompany, typeID); view.DraftVersion != nil || view.Override.ActiveVersionNo != v1 {
		t.Fatalf("after approve: draft=%+v active=%d", view.DraftVersion, view.Override.ActiveVersionNo)
	}

	v2 := reviewDraft(t, repo, typeID, writerID)
	if _, err := repo.SubmitCompanyWorkflowOverride(ctx, disclosureapp.SubmitCompanyWorkflowOverrideRequest{Subject: writer, TypeID: typeID, VersionNo: v2}); err != nil {
		t.Fatal(err)
	}
	rej, err := repo.RejectCompanyWorkflowOverride(ctx, disclosureapp.RejectCompanyWorkflowOverrideRequest{Subject: approver, TypeID: typeID, VersionNo: v2, Reason: "Thiếu bước rà soát"})
	if err != nil {
		t.Fatalf("reject: %v", err)
	}
	if rej.State != "rejected" || rej.RejectedBy != approverID || rej.RejectedAt == nil || rej.DecisionNote != "Thiếu bước rà soát" || rej.SubmitNote != "" {
		t.Fatalf("rejected=%+v", rej)
	}
	_, err = repo.ApproveCompanyWorkflowOverride(ctx, disclosureapp.ApproveCompanyWorkflowOverrideRequest{Subject: approver, TypeID: typeID, VersionNo: v2})
	wantHTTPCode(t, err, http.StatusConflict, perr.CodeStateConflict)
	_, err = repo.RejectCompanyWorkflowOverride(ctx, disclosureapp.RejectCompanyWorkflowOverrideRequest{Subject: approver, TypeID: typeID, VersionNo: v2, Reason: "lần hai"})
	wantHTTPCode(t, err, http.StatusConflict, perr.CodeStateConflict)

	v3 := reviewDraft(t, repo, typeID, writerID)
	_, err = repo.SubmitCompanyWorkflowOverride(ctx, disclosureapp.SubmitCompanyWorkflowOverrideRequest{Subject: writer, TypeID: typeID, VersionNo: v2})
	wantHTTPCode(t, err, http.StatusConflict, perr.CodeStateConflict) // v2 is rejected
	own, err := repo.ApproveCompanyWorkflowOverride(ctx, disclosureapp.ApproveCompanyWorkflowOverrideRequest{Subject: writer, TypeID: typeID, VersionNo: v3})
	if err != nil {
		t.Fatalf("approve own draft: %v", err)
	}
	if !own.SelfApproved || own.FromState != "draft" {
		t.Fatalf("own approval=%+v, want self-approved from draft", own)
	}
}

func TestOverrideReview_OnlyTheLatestVersionOnMySQL(t *testing.T) {
	db := departmentFilterTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	typeID, writerID, approverID := seedReviewType(t, db)
	approver := disclosureapp.Subject{UserID: approverID, CompanyID: reviewTestCompany}

	v1 := reviewDraft(t, repo, typeID, writerID)
	v2 := reviewDraft(t, repo, typeID, writerID)
	_, err := repo.ApproveCompanyWorkflowOverride(ctx, disclosureapp.ApproveCompanyWorkflowOverrideRequest{Subject: approver, TypeID: typeID, VersionNo: v1})
	wantHTTPCode(t, err, http.StatusConflict, perr.CodeStaleEtag)

	// A rebase-apply writes a newer, active version: the open draft below it is stale (WF-08c).
	if _, err := repo.ApplyWorkflowOverrideRebase(ctx, disclosureapp.ApplyWorkflowOverrideRebaseParams{
		CompanyID: reviewTestCompany, TypeID: typeID, ExpectedActiveVersionNo: 0,
		NewSnapshot:      []disclosureapp.WorkflowStepDTO{{StepID: "s1", Stage: "Soạn thảo", Description: "Hồ sơ"}},
		NewBaseVersionNo: 1, UserID: approverID, Now: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("rebase-apply: %v", err)
	}
	_, err = repo.ApproveCompanyWorkflowOverride(ctx, disclosureapp.ApproveCompanyWorkflowOverrideRequest{Subject: approver, TypeID: typeID, VersionNo: v2})
	wantHTTPCode(t, err, http.StatusConflict, perr.CodeStaleEtag)
	if view, _ := repo.GetCompanyWorkflowOverride(ctx, reviewTestCompany, typeID); view.DraftVersion != nil {
		t.Fatalf("draft below the rebased version still shown: %+v", view.DraftVersion)
	}
	// Editing from a stale base is refused too.
	_, err = repo.UpsertCompanyWorkflowOverrideDraft(ctx, disclosureapp.UpsertCompanyWorkflowOverrideDraftRequest{
		Subject: approver, TypeID: typeID, BaseVersionNo: v2,
		Workflow: []disclosureapp.WorkflowStepDTO{{StepID: "s1", Stage: "Soạn thảo", Description: "Hồ sơ"}},
	})
	wantHTTPCode(t, err, http.StatusConflict, perr.CodeStaleEtag)
}

// MR-4 review DB-01/DB-02/DB-07 on MySQL: publish activates in the same transaction; a deleted
// draft stays as discarded and its number is not reused; a step-groups edit writes a new
// version; a rebase waits for a pending submission.
func TestOverrideReview_VersionsAreImmutableOnMySQL(t *testing.T) {
	db := departmentFilterTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	typeID, writerID, approverID := seedReviewType(t, db)
	writer := disclosureapp.Subject{UserID: writerID, CompanyID: reviewTestCompany}

	pub, err := repo.UpsertCompanyWorkflowOverrideDraft(ctx, disclosureapp.UpsertCompanyWorkflowOverrideDraftRequest{
		Subject: disclosureapp.Subject{UserID: approverID, CompanyID: reviewTestCompany}, TypeID: typeID, Publish: true,
		Workflow: []disclosureapp.WorkflowStepDTO{{StepID: "s1", Stage: "Soạn thảo", Description: "Chuẩn bị hồ sơ"}},
	})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if pub.State != "approved" || !pub.SelfApproved {
		t.Fatalf("publish response=%+v", pub)
	}
	var state, approvedBy string
	if err := db.QueryRowContext(ctx, `SELECT v.state, COALESCE(v.approved_by, '') FROM company_template_workflow_override_versions v JOIN company_template_workflow_overrides o ON o.override_id = v.override_id WHERE o.type_id = ? AND v.version_no = ?`, typeID, pub.DraftVersionNo).Scan(&state, &approvedBy); err != nil {
		t.Fatal(err)
	}
	if view, _ := repo.GetCompanyWorkflowOverride(ctx, reviewTestCompany, typeID); state != "approved" || approvedBy != approverID || view.Override.ActiveVersionNo != pub.DraftVersionNo || view.DraftVersion != nil {
		t.Fatalf("after publish: state=%s approved_by=%s view=%+v", state, approvedBy, view)
	}

	v2 := reviewDraft(t, repo, typeID, writerID)
	if _, err := repo.DeleteCompanyWorkflowOverrideDraft(ctx, disclosureapp.DeleteCompanyWorkflowOverrideDraftRequest{Subject: writer, TypeID: typeID, VersionNo: v2}); err != nil {
		t.Fatalf("delete draft: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT v.state FROM company_template_workflow_override_versions v JOIN company_template_workflow_overrides o ON o.override_id = v.override_id WHERE o.type_id = ? AND v.version_no = ?`, typeID, v2).Scan(&state); err != nil || state != "discarded" {
		t.Fatalf("deleted draft row: state=%q err=%v", state, err)
	}
	if view, _ := repo.GetCompanyWorkflowOverride(ctx, reviewTestCompany, typeID); view.DraftVersion != nil {
		t.Fatalf("a discarded draft is still shown: %+v", view.DraftVersion)
	}
	v3 := reviewDraft(t, repo, typeID, writerID)
	if v3 != v2+1 {
		t.Fatalf("draft after delete=%d, want %d (no reuse)", v3, v2+1)
	}

	edit, err := repo.UpdateWorkflowOverrideStepGroups(ctx, disclosureapp.UpdateWorkflowOverrideStepGroupsRequest{
		Subject: writer, TypeID: typeID, StepID: "s1", BaseEtag: disclosureapp.WorkflowDraftEtagFromVersion(v3), ClearAll: true,
	})
	if err != nil {
		t.Fatalf("step groups: %v", err)
	}
	if edit.DraftEtag != disclosureapp.WorkflowDraftEtagFromVersion(v3+1) {
		t.Fatalf("step groups etag=%s, want a new version v:%d", edit.DraftEtag, v3+1)
	}
	_, err = repo.ApproveCompanyWorkflowOverride(ctx, disclosureapp.ApproveCompanyWorkflowOverrideRequest{Subject: disclosureapp.Subject{UserID: approverID, CompanyID: reviewTestCompany}, TypeID: typeID, VersionNo: v3})
	wantHTTPCode(t, err, http.StatusConflict, perr.CodeStaleEtag) // the content the approver saw as v3 moved to v4

	if _, err := repo.SubmitCompanyWorkflowOverride(ctx, disclosureapp.SubmitCompanyWorkflowOverrideRequest{Subject: writer, TypeID: typeID, VersionNo: v3 + 1}); err != nil {
		t.Fatalf("submit: %v", err)
	}
	_, err = repo.UpdateWorkflowOverrideStepGroups(ctx, disclosureapp.UpdateWorkflowOverrideStepGroupsRequest{
		Subject: writer, TypeID: typeID, StepID: "s1", BaseEtag: disclosureapp.WorkflowDraftEtagFromVersion(v3 + 1), ClearAll: true,
	})
	wantHTTPCode(t, err, http.StatusConflict, perr.CodeStateConflict) // a submission is not edited
	_, err = repo.ApplyWorkflowOverrideRebase(ctx, disclosureapp.ApplyWorkflowOverrideRebaseParams{
		CompanyID: reviewTestCompany, TypeID: typeID, ExpectedActiveVersionNo: pub.DraftVersionNo,
		NewSnapshot:      []disclosureapp.WorkflowStepDTO{{StepID: "s1", Stage: "Soạn thảo", Description: "Hồ sơ"}},
		NewBaseVersionNo: 1, UserID: approverID, Now: time.Now().UTC(),
	})
	wantHTTPCode(t, err, http.StatusConflict, perr.CodeStateConflict) // decide the submission first
}
