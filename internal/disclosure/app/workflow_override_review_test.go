package app_test

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

// MR-4 (PO decision Q5-B): activating an override version needs the approve permission, the
// approver's own version included (self_approved); a writer saves and submits; an approver
// approves or rejects a submission; only the latest version can be reviewed.

const (
	permOverrideWrite   = "template.workflow.override.write"
	permOverrideApprove = "template.workflow.override.approve"
)

// actionAuth allows exactly the listed actions.
type actionAuth struct{ allowed map[string]bool }

func (a *actionAuth) Authorize(_ context.Context, req authapp.AuthorizeRequest) (*authapp.AuthorizeDecision, error) {
	if a.allowed[req.Action] {
		return &authapp.AuthorizeDecision{Decision: authapp.DecisionAllow}, nil
	}
	code := perr.CodePermissionDenied
	return &authapp.AuthorizeDecision{Decision: authapp.DecisionDeny, DenyReasonCode: &code}, nil
}

func (a *actionAuth) AuthorizeBatch(_ context.Context, req authapp.AuthorizeBatchRequest) (*authapp.AuthorizeBatchResponse, error) {
	results := make([]authapp.AuthorizeDecision, len(req.Checks))
	for i := range req.Checks {
		results[i] = authapp.AuthorizeDecision{Decision: authapp.DecisionDeny}
	}
	return &authapp.AuthorizeBatchResponse{Results: results}, nil
}

func (a *actionAuth) GetEffectiveAccess(_ context.Context, _, _ string) (*authapp.EffectiveAccessSummary, error) {
	return &authapp.EffectiveAccessSummary{}, nil
}

func reviewServices(t *testing.T) (writer, approver disclosureapp.Service, repo *inmemory.Repository) {
	t.Helper()
	repo = inmemory.NewRepository()
	writer = disclosureapp.NewService(repo, &actionAuth{allowed: map[string]bool{permOverrideWrite: true}}, idgen.UUIDv7Generator{})
	approver = disclosureapp.NewService(repo, &actionAuth{allowed: map[string]bool{permOverrideWrite: true, permOverrideApprove: true}}, idgen.UUIDv7Generator{})
	return writer, approver, repo
}

const reviewCompany = "c-review"

var (
	writerSub   = disclosureapp.Subject{UserID: "u-writer", MembershipID: "m-writer", CompanyID: reviewCompany}
	approverSub = disclosureapp.Subject{UserID: "u-approver", MembershipID: "m-approver", CompanyID: reviewCompany}
)

func reviewWorkflow() []disclosureapp.WorkflowStepDTO {
	return []disclosureapp.WorkflowStepDTO{{StepID: "s1", Stage: "Soạn thảo", Description: "Chuẩn bị hồ sơ CBTT"}}
}

func saveDraft(t *testing.T, svc disclosureapp.Service, sub disclosureapp.Subject) int {
	t.Helper()
	resp, err := svc.UpsertCompanyWorkflowOverrideDraft(context.Background(), disclosureapp.UpsertCompanyWorkflowOverrideDraftRequest{
		Subject: sub, TypeID: staleTestTypeID, Workflow: reviewWorkflow(),
	})
	if err != nil {
		t.Fatalf("save draft: %v", err)
	}
	return resp.DraftVersionNo
}

func wantHTTP(t *testing.T, err error, status int, code perr.Code) *perr.HTTPError {
	t.Helper()
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != status || (code != "" && he.Code != code) {
		t.Fatalf("err=%v, want %d %s", err, status, code)
	}
	return he
}

func overrideView(t *testing.T, repo *inmemory.Repository) *disclosureapp.CompanyWorkflowOverrideViewDTO {
	t.Helper()
	view, err := repo.GetCompanyWorkflowOverride(context.Background(), reviewCompany, staleTestTypeID)
	if err != nil {
		t.Fatal(err)
	}
	return view
}

func TestOverrideReview_WriterSubmitsButCannotActivate(t *testing.T) {
	writer, _, repo := reviewServices(t)
	ctx := context.Background()

	// publish:true by a writer: 403 and nothing saved.
	_, err := writer.UpsertCompanyWorkflowOverrideDraft(ctx, disclosureapp.UpsertCompanyWorkflowOverrideDraftRequest{
		Subject: writerSub, TypeID: staleTestTypeID, Workflow: reviewWorkflow(), Publish: true,
	})
	wantHTTP(t, err, http.StatusForbidden, "")
	if view := overrideView(t, repo); view.Override != nil {
		t.Fatalf("a refused publish left an override behind: %+v", view.Override)
	}

	v := saveDraft(t, writer, writerSub)
	sub, err := writer.SubmitCompanyWorkflowOverride(ctx, disclosureapp.SubmitCompanyWorkflowOverrideRequest{
		Subject: writerSub, TypeID: staleTestTypeID, VersionNo: v, Note: "nhờ duyệt",
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if sub.Data.State != disclosureapp.OverrideVersionStatePendingApproval || sub.Data.SubmittedBy != writerSub.UserID || sub.Data.SubmitNote != "nhờ duyệt" || sub.Data.DecisionNote != "" {
		t.Fatalf("submitted version=%+v", sub.Data)
	}

	_, err = writer.ApproveCompanyWorkflowOverride(ctx, disclosureapp.ApproveCompanyWorkflowOverrideRequest{Subject: writerSub, TypeID: staleTestTypeID, VersionNo: v})
	wantHTTP(t, err, http.StatusForbidden, "")
	_, err = writer.RejectCompanyWorkflowOverride(ctx, disclosureapp.RejectCompanyWorkflowOverrideRequest{Subject: writerSub, TypeID: staleTestTypeID, VersionNo: v, Reason: "x"})
	wantHTTP(t, err, http.StatusForbidden, "")
	_, err = writer.ListPendingCompanyWorkflowOverrides(ctx, disclosureapp.ListPendingCompanyWorkflowOverridesRequest{Subject: writerSub})
	wantHTTP(t, err, http.StatusForbidden, "")
	if view := overrideView(t, repo); view.Override.ActiveVersionNo != 0 || view.DraftVersion.State != disclosureapp.OverrideVersionStatePendingApproval {
		t.Fatalf("after the writer's refused calls: active=%d draft=%+v", view.Override.ActiveVersionNo, view.DraftVersion)
	}
}

func TestOverrideReview_ApproverActivatesOwnVersionAsSelfApproved(t *testing.T) {
	_, approver, repo := reviewServices(t)
	ctx := context.Background()

	v := saveDraft(t, approver, approverSub)
	resp, err := approver.ApproveCompanyWorkflowOverride(ctx, disclosureapp.ApproveCompanyWorkflowOverrideRequest{Subject: approverSub, TypeID: staleTestTypeID, VersionNo: v})
	if err != nil {
		t.Fatalf("approve own draft: %v", err)
	}
	if !resp.SelfApproved || resp.FromState != disclosureapp.OverrideVersionStateDraft || resp.ActiveVersionNo != v {
		t.Fatalf("approve=%+v, want self-approved activation of v%d", resp, v)
	}

	pub, err := approver.UpsertCompanyWorkflowOverrideDraft(ctx, disclosureapp.UpsertCompanyWorkflowOverrideDraftRequest{
		Subject: approverSub, TypeID: staleTestTypeID, Workflow: reviewWorkflow(), Publish: true,
	})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if !pub.SelfApproved || pub.State != disclosureapp.OverrideVersionStateApproved {
		t.Fatalf("publish=%+v, want self-approved", pub)
	}
	if view := overrideView(t, repo); view.Override.ActiveVersionNo != pub.DraftVersionNo || view.DraftVersion != nil {
		t.Fatalf("after publish: active=%d draft=%+v", view.Override.ActiveVersionNo, view.DraftVersion)
	}
}

func TestOverrideReview_ApproverApprovesASubmission(t *testing.T) {
	writer, approver, repo := reviewServices(t)
	ctx := context.Background()
	v := saveDraft(t, writer, writerSub)
	if _, err := writer.SubmitCompanyWorkflowOverride(ctx, disclosureapp.SubmitCompanyWorkflowOverrideRequest{Subject: writerSub, TypeID: staleTestTypeID, VersionNo: v}); err != nil {
		t.Fatal(err)
	}

	pending, err := approver.ListPendingCompanyWorkflowOverrides(ctx, disclosureapp.ListPendingCompanyWorkflowOverridesRequest{Subject: approverSub})
	if err != nil {
		t.Fatal(err)
	}
	if pending.Total != 1 || pending.Items[0].TypeID != staleTestTypeID || pending.Items[0].VersionNo != v || pending.Items[0].SubmittedBy != writerSub.UserID || pending.Items[0].DraftEtag == "" {
		t.Fatalf("pending=%+v", pending)
	}
	other, err := approver.ListPendingCompanyWorkflowOverrides(ctx, disclosureapp.ListPendingCompanyWorkflowOverridesRequest{Subject: disclosureapp.Subject{UserID: "u-x", CompanyID: "c-other"}})
	if err != nil || other.Total != 0 {
		t.Fatalf("another company's pending list=%+v err=%v, want empty", other, err)
	}

	resp, err := approver.ApproveCompanyWorkflowOverride(ctx, disclosureapp.ApproveCompanyWorkflowOverrideRequest{Subject: approverSub, TypeID: staleTestTypeID, VersionNo: v})
	if err != nil {
		t.Fatalf("approve submission: %v", err)
	}
	if resp.SelfApproved || resp.FromState != disclosureapp.OverrideVersionStatePendingApproval {
		t.Fatalf("approve=%+v, want a four-eyes approval from pending_approval", resp)
	}
	if view := overrideView(t, repo); view.Override.ActiveVersionNo != v {
		t.Fatalf("active=%d, want v%d", view.Override.ActiveVersionNo, v)
	}
	after, _ := approver.ListPendingCompanyWorkflowOverrides(ctx, disclosureapp.ListPendingCompanyWorkflowOverridesRequest{Subject: approverSub})
	if after.Total != 0 {
		t.Fatalf("pending after approval=%+v", after)
	}
}

func TestOverrideReview_RejectNeedsAReasonAndASubmission(t *testing.T) {
	writer, approver, repo := reviewServices(t)
	ctx := context.Background()
	v := saveDraft(t, writer, writerSub)

	_, err := approver.RejectCompanyWorkflowOverride(ctx, disclosureapp.RejectCompanyWorkflowOverrideRequest{Subject: approverSub, TypeID: staleTestTypeID, VersionNo: v, Reason: "chưa đủ"})
	wantHTTP(t, err, http.StatusConflict, perr.CodeStateConflict) // a draft, not submitted

	if _, err := writer.SubmitCompanyWorkflowOverride(ctx, disclosureapp.SubmitCompanyWorkflowOverrideRequest{Subject: writerSub, TypeID: staleTestTypeID, VersionNo: v}); err != nil {
		t.Fatal(err)
	}
	_, err = approver.RejectCompanyWorkflowOverride(ctx, disclosureapp.RejectCompanyWorkflowOverrideRequest{Subject: approverSub, TypeID: staleTestTypeID, VersionNo: v, Reason: "  "})
	he := wantHTTP(t, err, http.StatusUnprocessableEntity, perr.CodeInvalidRequest)
	if he.Details["field"] != "reason" {
		t.Fatalf("details=%v, want field=reason", he.Details)
	}

	rej, err := approver.RejectCompanyWorkflowOverride(ctx, disclosureapp.RejectCompanyWorkflowOverrideRequest{Subject: approverSub, TypeID: staleTestTypeID, VersionNo: v, Reason: "Thiếu bước rà soát pháp lý"})
	if err != nil {
		t.Fatalf("reject: %v", err)
	}
	if rej.Data.State != disclosureapp.OverrideVersionStateRejected || rej.Data.RejectedBy != approverSub.UserID || rej.Data.DecisionNote != "Thiếu bước rà soát pháp lý" {
		t.Fatalf("rejected version=%+v", rej.Data)
	}
	_, err = approver.ApproveCompanyWorkflowOverride(ctx, disclosureapp.ApproveCompanyWorkflowOverrideRequest{Subject: approverSub, TypeID: staleTestTypeID, VersionNo: v})
	wantHTTP(t, err, http.StatusConflict, perr.CodeStateConflict)

	// Editing after a rejection starts a new draft.
	v2 := saveDraft(t, writer, writerSub)
	if view := overrideView(t, repo); view.DraftVersion == nil || view.DraftVersion.VersionNo != v2 || view.DraftVersion.State != disclosureapp.OverrideVersionStateDraft {
		t.Fatalf("draft after re-edit=%+v, want v%d draft", view.DraftVersion, v2)
	}
}

func TestOverrideReview_OnlyTheLatestVersionCanBeReviewed(t *testing.T) {
	writer, approver, repo := reviewServices(t)
	ctx := context.Background()
	v1 := saveDraft(t, writer, writerSub)
	v2 := saveDraft(t, writer, writerSub)

	_, err := approver.ApproveCompanyWorkflowOverride(ctx, disclosureapp.ApproveCompanyWorkflowOverrideRequest{Subject: approverSub, TypeID: staleTestTypeID, VersionNo: v1})
	wantHTTP(t, err, http.StatusConflict, perr.CodeStaleEtag)
	_, err = writer.SubmitCompanyWorkflowOverride(ctx, disclosureapp.SubmitCompanyWorkflowOverrideRequest{Subject: writerSub, TypeID: staleTestTypeID, VersionNo: v1})
	wantHTTP(t, err, http.StatusConflict, perr.CodeStaleEtag)

	// A rebase-apply writes a newer, active version: the older draft is stale (WF-08c).
	if _, err := repo.ApplyWorkflowOverrideRebase(ctx, disclosureapp.ApplyWorkflowOverrideRebaseParams{
		CompanyID: reviewCompany, TypeID: staleTestTypeID, ExpectedActiveVersionNo: 0,
		NewSnapshot: reviewWorkflow(), NewBaseVersionNo: 2, UserID: approverSub.UserID, Now: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	_, err = approver.ApproveCompanyWorkflowOverride(ctx, disclosureapp.ApproveCompanyWorkflowOverrideRequest{Subject: approverSub, TypeID: staleTestTypeID, VersionNo: v2})
	wantHTTP(t, err, http.StatusConflict, perr.CodeStaleEtag)
	if view := overrideView(t, repo); view.DraftVersion != nil {
		t.Fatalf("a draft below the rebased version is still shown: %+v", view.DraftVersion)
	}
}

func TestOverrideReview_SubmitNeedsAnActivatableWorkflow(t *testing.T) {
	writer, _, _ := reviewServices(t)
	ctx := context.Background()
	resp, err := writer.UpsertCompanyWorkflowOverrideDraft(ctx, disclosureapp.UpsertCompanyWorkflowOverrideDraftRequest{
		Subject: writerSub, TypeID: staleTestTypeID, Workflow: []disclosureapp.WorkflowStepDTO{{StepID: "s1", Stage: "Soạn thảo"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = writer.SubmitCompanyWorkflowOverride(ctx, disclosureapp.SubmitCompanyWorkflowOverrideRequest{Subject: writerSub, TypeID: staleTestTypeID, VersionNo: resp.DraftVersionNo})
	wantHTTP(t, err, http.StatusUnprocessableEntity, "")
}

func TestRebaseApply_WriterWithoutApproveIs403(t *testing.T) {
	writer, _, _ := reviewServices(t)
	_, err := writer.ApplyWorkflowOverrideRebase(context.Background(), disclosureapp.ApplyWorkflowOverrideRebaseRequest{
		Subject: writerSub, TypeID: staleTestTypeID, PreviewID: "any-preview",
	})
	wantHTTP(t, err, http.StatusForbidden, "")
}

func TestValidateMergeManualValue(t *testing.T) {
	cases := []struct {
		field string
		value any
		ok    bool
	}{
		{"reminder_config", map[string]any{}, false},
		{"reminder_config", map[string]any{"enabled": false}, true},
		{"reminder_config", map[string]any{"enabled": true}, false},
		{"reminder_config", map[string]any{"enabled": true, "days_before": []any{float64(3), float64(1)}}, true},
		{"reminder_config", map[string]any{"enabled": true, "days_before": []any{1.5}}, false},
		{"due_rule", map[string]any{}, false},
		{"due_rule", "T+0", false},
		{"due_rule", "T+5", true},
		{"assignee_role_ids", map[string]any{}, false},
		{"assignee_role_ids", []any{}, false},
		{"assignee_role_ids", []any{"r1", 2}, false},
		{"assignee_role_ids", []any{"r1"}, true},
		{"stage", map[string]any{}, false},
		{"stage", "Rà soát", true},
		{"stage", strings.Repeat("a", 256), false},
		// documents and groups reference company data: no merge_manual (MR-4 review ROLE-06).
		{"documents", map[string]any{}, false},
		{"documents", []any{}, false},
		{"groups", []any{}, false},
		{"department_id", "d1", false},
		{"__step_existence__", map[string]any{"step_id": "s"}, false},
		{"__step_existence__", map[string]any{}, false},
		{"__step_existence__", map[string]any{"step_id": "s", "stage": "Rà soát"}, true},
	}
	for _, tc := range cases {
		err := disclosureapp.ValidateMergeManualValue(tc.field, tc.value)
		if (err == nil) != tc.ok {
			t.Errorf("%s %v: err=%v, want ok=%v", tc.field, tc.value, err, tc.ok)
			continue
		}
		if err != nil {
			he := wantHTTP(t, err, http.StatusUnprocessableEntity, perr.CodeInvalidRequest)
			if he.Details["field"] != "resolution_value" || he.Details["field_path"] != tc.field {
				t.Errorf("%s: details=%v", tc.field, he.Details)
			}
		}
	}
}

func TestResolveConflict_MergeManualNeedsAValidValueAndAnOfferedResolution(t *testing.T) {
	svc, repo := newStalenessTestService(t, true)
	ctx := context.Background()
	companyID := "company-merge-manual"
	seedOverride(t, repo, companyID, staleTestTypeID, disclosureapp.BaseSourceGlobalWorkflow, intPtrTest(2))
	rows, err := repo.UpsertWorkflowOverrideConflicts(ctx, []disclosureapp.PersistedConflictInput{{
		PreviewConflict: disclosureapp.PreviewConflict{
			StepKey: "s", FieldPath: "reminder_config", Severity: disclosureapp.ConflictSeverityAdvisory, ConflictType: disclosureapp.ConflictTypeSameFieldChanged,
			ResolutionOptions: []string{disclosureapp.ResolutionKeepCompany, disclosureapp.ResolutionAcceptGlobal, disclosureapp.ResolutionMergeManual},
		},
		CompanyID: companyID, TypeID: staleTestTypeID, BaseVersionNo: 1, TargetVersionNo: 2, CreatedBy: "tester",
	}})
	if err != nil {
		t.Fatalf("seed conflict: %v", err)
	}
	sub := disclosureapp.Subject{UserID: "u1", CompanyID: companyID}
	resolve := func(resolution string, value any) error {
		_, err := svc.ResolveWorkflowOverrideConflict(ctx, disclosureapp.ResolveWorkflowOverrideConflictRequest{
			Subject: sub, TypeID: staleTestTypeID, ConflictID: rows[0].ID, Resolution: resolution, ResolutionValue: value,
		})
		return err
	}
	wantHTTP(t, resolve(disclosureapp.ResolutionMergeManual, map[string]any{}), http.StatusUnprocessableEntity, perr.CodeInvalidRequest)
	wantHTTP(t, resolve(disclosureapp.ResolutionMarkNotApplicable, nil), http.StatusUnprocessableEntity, perr.CodeInvalidResolution)
	if err := resolve(disclosureapp.ResolutionMergeManual, map[string]any{"enabled": true, "days_before": []any{float64(2)}}); err != nil {
		t.Fatalf("valid merge_manual: %v", err)
	}

	// A stored conflict without options (MySQL keeps none) takes them from its type (review API-04).
	bare, err := repo.UpsertWorkflowOverrideConflicts(ctx, []disclosureapp.PersistedConflictInput{{
		PreviewConflict: disclosureapp.PreviewConflict{
			StepKey: "s2", FieldPath: "assignee_role_ids", Severity: disclosureapp.ConflictSeverityBlocking, ConflictType: disclosureapp.ConflictTypeRoleNoLongerExists,
		},
		CompanyID: companyID, TypeID: staleTestTypeID, BaseVersionNo: 1, TargetVersionNo: 2, CreatedBy: "tester",
	}})
	if err != nil {
		t.Fatalf("seed conflict without options: %v", err)
	}
	_, err = svc.ResolveWorkflowOverrideConflict(ctx, disclosureapp.ResolveWorkflowOverrideConflictRequest{
		Subject: sub, TypeID: staleTestTypeID, ConflictID: bare[0].ID, Resolution: disclosureapp.ResolutionKeepCompany,
	})
	wantHTTP(t, err, http.StatusUnprocessableEntity, perr.CodeInvalidResolution)
}

func TestResolutionOptionsForConflictType(t *testing.T) {
	for ct, want := range map[string][]string{
		disclosureapp.ConflictTypeSameFieldChanged:               {disclosureapp.ResolutionKeepCompany, disclosureapp.ResolutionAcceptGlobal, disclosureapp.ResolutionMergeManual},
		disclosureapp.ConflictTypeRoleNoLongerExists:             {disclosureapp.ResolutionMergeManual},
		disclosureapp.ConflictTypeCompanyRemovedMandatoryStep:    {disclosureapp.ResolutionMarkNotApplicable, disclosureapp.ResolutionAcceptGlobal},
		disclosureapp.ConflictTypeGlobalDeletedCustomized:        {disclosureapp.ResolutionKeepCompany, disclosureapp.ResolutionCreateNewStep},
		disclosureapp.ConflictTypeGlobalRenamedCompanyReassigned: {disclosureapp.ResolutionKeepCompany, disclosureapp.ResolutionAcceptGlobal},
		"unknown": nil,
	} {
		if got := disclosureapp.ResolutionOptionsForConflictType(ct); !slices.Equal(got, want) {
			t.Errorf("%s: options=%v, want %v", ct, got, want)
		}
	}
}

// MR-4 review DB-01: publish:true saves and activates in one step, reported as self-approval.
func TestOverrideReview_PublishActivatesAtOnce(t *testing.T) {
	_, approver, repo := reviewServices(t)
	resp, err := approver.UpsertCompanyWorkflowOverrideDraft(context.Background(), disclosureapp.UpsertCompanyWorkflowOverrideDraftRequest{
		Subject: approverSub, TypeID: staleTestTypeID, Workflow: reviewWorkflow(), Publish: true,
	})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if resp.State != disclosureapp.OverrideVersionStateApproved || !resp.SelfApproved {
		t.Fatalf("publish response=%+v", resp)
	}
	if view := overrideView(t, repo); view.Override.ActiveVersionNo != resp.DraftVersionNo || view.DraftVersion != nil {
		t.Fatalf("after publish: active=%d draft=%+v", view.Override.ActiveVersionNo, view.DraftVersion)
	}
}

// MR-4 review DB-03: approving the active version again is a state conflict, not 404.
func TestOverrideReview_ApproveTwiceIsAStateConflict(t *testing.T) {
	_, approver, _ := reviewServices(t)
	ctx := context.Background()
	v := saveDraft(t, approver, approverSub)
	req := disclosureapp.ApproveCompanyWorkflowOverrideRequest{Subject: approverSub, TypeID: staleTestTypeID, VersionNo: v}
	if _, err := approver.ApproveCompanyWorkflowOverride(ctx, req); err != nil {
		t.Fatalf("approve: %v", err)
	}
	_, err := approver.ApproveCompanyWorkflowOverride(ctx, req)
	wantHTTP(t, err, http.StatusConflict, perr.CodeStateConflict)
}

// MR-4 review DB-06: the submit note survives a rejection; decision_note holds the reason.
func TestOverrideReview_SubmitNoteIsKeptAfterRejection(t *testing.T) {
	writer, approver, _ := reviewServices(t)
	ctx := context.Background()
	v := saveDraft(t, writer, writerSub)
	if _, err := writer.SubmitCompanyWorkflowOverride(ctx, disclosureapp.SubmitCompanyWorkflowOverrideRequest{Subject: writerSub, TypeID: staleTestTypeID, VersionNo: v, Note: "đã thêm bước rà soát"}); err != nil {
		t.Fatal(err)
	}
	rej, err := approver.RejectCompanyWorkflowOverride(ctx, disclosureapp.RejectCompanyWorkflowOverrideRequest{Subject: approverSub, TypeID: staleTestTypeID, VersionNo: v, Reason: "Thiếu hạn xử lý"})
	if err != nil {
		t.Fatal(err)
	}
	if rej.Data.SubmitNote != "đã thêm bước rà soát" || rej.Data.DecisionNote != "Thiếu hạn xử lý" {
		t.Fatalf("rejected version=%+v", rej.Data)
	}
}

// MR-4 review DB-02: a deleted draft is kept as discarded, so its number is never reused.
func TestOverrideReview_DeletedDraftNumberIsNotReused(t *testing.T) {
	writer, approver, repo := reviewServices(t)
	ctx := context.Background()
	v1 := saveDraft(t, writer, writerSub)
	if _, err := writer.DeleteCompanyWorkflowOverrideDraft(ctx, disclosureapp.DeleteCompanyWorkflowOverrideDraftRequest{Subject: writerSub, TypeID: staleTestTypeID, VersionNo: v1}); err != nil {
		t.Fatalf("delete draft: %v", err)
	}
	if view := overrideView(t, repo); view.DraftVersion != nil {
		t.Fatalf("a discarded draft is still shown: %+v", view.DraftVersion)
	}
	_, err := approver.ApproveCompanyWorkflowOverride(ctx, disclosureapp.ApproveCompanyWorkflowOverrideRequest{Subject: approverSub, TypeID: staleTestTypeID, VersionNo: v1})
	wantHTTP(t, err, http.StatusNotFound, "")
	if v2 := saveDraft(t, writer, writerSub); v2 == v1 {
		t.Fatalf("draft number %d reused after delete", v1)
	}
}

// MR-4 review DB-07: a rebase does not supersede a submission waiting for approval.
func TestRebaseApply_RefusedWhileASubmissionIsPending(t *testing.T) {
	writer, _, repo := reviewServices(t)
	ctx := context.Background()
	v := saveDraft(t, writer, writerSub)
	if _, err := writer.SubmitCompanyWorkflowOverride(ctx, disclosureapp.SubmitCompanyWorkflowOverrideRequest{Subject: writerSub, TypeID: staleTestTypeID, VersionNo: v}); err != nil {
		t.Fatal(err)
	}
	_, err := repo.ApplyWorkflowOverrideRebase(ctx, disclosureapp.ApplyWorkflowOverrideRebaseParams{
		CompanyID: reviewCompany, TypeID: staleTestTypeID, ExpectedActiveVersionNo: 0,
		NewSnapshot: reviewWorkflow(), NewBaseVersionNo: 1, UserID: approverSub.UserID, Now: time.Now().UTC(),
	})
	wantHTTP(t, err, http.StatusConflict, perr.CodeStateConflict)
}
