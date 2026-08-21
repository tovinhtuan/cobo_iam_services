package app_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/infra/inmemory"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func fourSteps(prefix string) []disclosureapp.GlobalWorkflowStepInput {
	return []disclosureapp.GlobalWorkflowStepInput{
		{StepID: prefix + "-step-1", Stage: "Thu thập dữ liệu", Description: "desc-1", Instructions: "ins-1", DepartmentID: "d1", AssigneeRoleIds: []string{"r1"}, DueRule: "T+1", ProcessingDays: 1, DisplayOrder: 1},
		{StepID: prefix + "-step-2", Stage: "Rà soát", Description: "desc-2", Instructions: "ins-2", DepartmentID: "d2", AssigneeRoleIds: []string{"r2"}, DueRule: "T+2", ProcessingDays: 2, DisplayOrder: 2},
		{StepID: prefix + "-step-3", Stage: "Phê duyệt", Description: "desc-3", Instructions: "ins-3", DepartmentID: "d3", AssigneeRoleIds: []string{"r3"}, DueRule: "T+3", ProcessingDays: 3, DisplayOrder: 3},
		{StepID: prefix + "-step-4", Stage: "Nộp & lưu", Description: "desc-4", Instructions: "ins-4", DepartmentID: "d4", AssigneeRoleIds: []string{"r4"}, DueRule: "T+4", ProcessingDays: 4, DisplayOrder: 4},
	}
}

func mustUpsertWF(t *testing.T, svc disclosureapp.Service, typeID string, steps []disclosureapp.GlobalWorkflowStepInput) *disclosureapp.GlobalWorkflowDTO {
	t.Helper()
	wf, err := svc.CmsUpsertGlobalWorkflow(context.Background(), disclosureapp.CmsUpsertGlobalWorkflowRequest{
		Subject: testSubjectWF, TypeID: typeID, ChangeNote: "Lưu bước", Steps: steps,
	})
	if err != nil {
		t.Fatalf("CmsUpsertGlobalWorkflow: %v", err)
	}
	return wf
}

func mustGetWF(t *testing.T, svc disclosureapp.Service, typeID string) *disclosureapp.GlobalWorkflowDTO {
	t.Helper()
	got, err := svc.CmsGetGlobalWorkflow(context.Background(), disclosureapp.CmsGetGlobalWorkflowRequest{
		Subject: testSubjectWF, TypeID: typeID,
	})
	if err != nil {
		t.Fatalf("CmsGetGlobalWorkflow: %v", err)
	}
	if got.Data == nil {
		t.Fatal("expected workflow projection")
	}
	return got.Data
}

func workflowActiveVersionNo(t *testing.T, repo *inmemory.Repository, typeID string) int {
	t.Helper()
	versions, err := repo.ListTypeVersions(context.Background(), testSubjectWF.CompanyID, typeID)
	if err != nil {
		t.Fatalf("ListTypeVersions: %v", err)
	}
	for _, v := range versions {
		if v.IsActive {
			return v.VersionNo
		}
	}
	return 0
}

func TestWorkflowStepSave_T1T2_NewDraftPersistsAndReloads(t *testing.T) {
	const typeID = "dt-step-save-t1"
	svc, repo := newSeededWFService(t, typeID)
	steps := fourSteps(typeID)
	steps[0].Instructions = "patch-step-1"
	wf := mustUpsertWF(t, svc, typeID, steps)
	if len(wf.Steps) != 4 {
		t.Fatalf("persist steps=%d want 4", len(wf.Steps))
	}
	if wf.Steps[0].Instructions != "patch-step-1" {
		t.Fatalf("step1 instructions=%q", wf.Steps[0].Instructions)
	}
	got := mustGetWF(t, svc, typeID)
	if got.Steps[0].Instructions != "patch-step-1" {
		t.Fatalf("reload lost step1: %q", got.Steps[0].Instructions)
	}
	if got.Steps[0].StepKey != got.Steps[0].StepID {
		t.Fatalf("GET identity must match configuration (step_key==step_id): key=%q id=%q", got.Steps[0].StepKey, got.Steps[0].StepID)
	}
	if n := workflowActiveVersionNo(t, repo, typeID); n != 0 {
		t.Fatalf("new draft must not move active pointer, got %d", n)
	}
}

func TestWorkflowStepSave_T3T4_OtherStepsPreserved(t *testing.T) {
	const typeID = "dt-step-save-t3"
	svc, _ := newSeededWFService(t, typeID)
	steps := fourSteps(typeID)
	mustUpsertWF(t, svc, typeID, steps)

	steps[0].Instructions = "edited-1"
	mustUpsertWF(t, svc, typeID, steps)
	got := mustGetWF(t, svc, typeID)
	if got.Steps[0].Instructions != "edited-1" {
		t.Fatalf("step1=%q", got.Steps[0].Instructions)
	}
	for i, want := range []string{"Rà soát", "Phê duyệt", "Nộp & lưu"} {
		if got.Steps[i+1].Stage != want {
			t.Fatalf("step %d stage=%q want %q", i+2, got.Steps[i+1].Stage, want)
		}
	}

	steps = fourSteps(typeID)
	steps[0].Instructions = "edited-1"
	steps[1].Instructions = "edited-2"
	mustUpsertWF(t, svc, typeID, steps)
	got = mustGetWF(t, svc, typeID)
	if got.Steps[0].Instructions != "edited-1" {
		t.Fatalf("step1 lost after step2 edit: %q", got.Steps[0].Instructions)
	}
	if got.Steps[1].Instructions != "edited-2" {
		t.Fatalf("step2=%q", got.Steps[1].Instructions)
	}
	if len(got.Steps) != 4 {
		t.Fatalf("dropped other steps: %d", len(got.Steps))
	}
}

func TestWorkflowStepSave_T5_N1_PreservesTemplateFieldsOnPartialSave(t *testing.T) {
	const typeID = "dt-step-save-t5"
	svc, repo := newSeededWFService(t, typeID)
	ctx := context.Background()
	detail, err := repo.GetTypeVersionDetail(ctx, testSubjectWF.CompanyID, typeID, 1)
	if err != nil {
		t.Fatalf("seed detail: %v", err)
	}
	_, err = svc.UpsertTypeVersion(ctx, disclosureapp.UpsertTypeVersionRequest{
		Subject: testSubjectWF, TypeID: typeID, Scope: "global", GroupID: detail.GroupID,
		Name: detail.Name, Category: detail.Category, TemplateCategory: detail.TemplateCategory,
		DeadlineStrategy: detail.DeadlineStrategy, DeadlineRule: detail.DeadlineRule, Periodicity: detail.Periodicity,
		Applicability: "HOSE+NYSE", LegalBasis: "TT 96/2020", ReportContent: "body-content",
		Blocks: detail.Blocks, DisplayGroupCodes: detail.DisplayGroupCodes, ChangeNote: "populate fields",
		LegalBasesProvided: true, ApplicabilityRules: detail.ApplicabilityRules,
	})
	if err != nil {
		t.Fatalf("populate fields: %v", err)
	}

	mustUpsertWF(t, svc, typeID, fourSteps(typeID))
	after, err := repo.GetTypeVersionDetail(ctx, testSubjectWF.CompanyID, typeID, 1)
	if err != nil {
		t.Fatalf("after workflow save: %v", err)
	}
	if after.Applicability != "HOSE+NYSE" || after.LegalBasis != "TT 96/2020" || after.ReportContent != "body-content" {
		t.Fatalf("template fields clobbered: applicability=%q legal=%q content=%q", after.Applicability, after.LegalBasis, after.ReportContent)
	}
	if after.DeadlineRule != detail.DeadlineRule {
		t.Fatalf("deadline_rule changed: %q -> %q", detail.DeadlineRule, after.DeadlineRule)
	}

	emptyBlocks := append([]disclosureapp.TemplateBlockDTO(nil), after.Blocks...)
	for i := range emptyBlocks {
		if strings.EqualFold(emptyBlocks[i].BlockKey, "enterprise_workflow") {
			emptyBlocks[i].Config = map[string]any{"steps": []any{}}
		}
	}
	_, err = svc.UpsertTypeVersion(ctx, disclosureapp.UpsertTypeVersionRequest{
		Subject: testSubjectWF, TypeID: typeID, Scope: "global", GroupID: after.GroupID,
		Name: after.Name, Category: after.Category, TemplateCategory: after.TemplateCategory,
		DeadlineStrategy: after.DeadlineStrategy, DeadlineRule: after.DeadlineRule, Periodicity: after.Periodicity,
		Applicability: after.Applicability, LegalBasis: after.LegalBasis, ReportContent: after.ReportContent,
		Blocks: emptyBlocks, DisplayGroupCodes: after.DisplayGroupCodes, ChangeNote: "Lưu nháp empty workflow",
		LegalBasesProvided: true, ApplicabilityRules: after.ApplicabilityRules,
	})
	if err != nil {
		t.Fatalf("empty workflow type upsert: %v", err)
	}
	preserved := mustGetWF(t, svc, typeID)
	if len(preserved.Steps) != 4 {
		t.Fatalf("N1 omitted workflow erased steps: %d", len(preserved.Steps))
	}
}

func TestWorkflowStepSave_T6T7T8T9_ActiveDraftIsolation(t *testing.T) {
	const typeID = "dt-step-save-t6"
	svc, repo := newSeededWFService(t, typeID)
	ctx := context.Background()
	original := fourSteps(typeID)
	mustUpsertWF(t, svc, typeID, original)
	if _, err := svc.ActivateTypeVersion(ctx, disclosureapp.ActivateTypeVersionRequest{
		Subject: testSubjectWF, TypeID: typeID, VersionNo: 1,
	}); err != nil {
		t.Fatalf("activate v1: %v", err)
	}
	activeBefore := workflowActiveVersionNo(t, repo, typeID)
	if activeBefore != 1 {
		t.Fatalf("active=%d want 1", activeBefore)
	}
	v1, err := repo.GetTypeVersionDetail(ctx, testSubjectWF.CompanyID, typeID, 1)
	if err != nil {
		t.Fatalf("v1: %v", err)
	}
	v1Hash := v1.WorkflowSemanticHash
	v1Candidate := v1.PublicationCandidateHash
	v1Released := false
	versions, _ := repo.ListTypeVersions(ctx, testSubjectWF.CompanyID, typeID)
	for _, v := range versions {
		if v.VersionNo == 1 {
			v1Released = v.IsReleased
		}
	}

	edited := fourSteps(typeID)
	edited[0].Instructions = "draft-v2-only"
	mustUpsertWF(t, svc, typeID, edited)

	if got := workflowActiveVersionNo(t, repo, typeID); got != 1 {
		t.Fatalf("save step moved active pointer: %d", got)
	}
	v1After, err := repo.GetTypeVersionDetail(ctx, testSubjectWF.CompanyID, typeID, 1)
	if err != nil {
		t.Fatalf("v1 after: %v", err)
	}
	if v1After.WorkflowSemanticHash != v1Hash || v1After.PublicationCandidateHash != v1Candidate {
		t.Fatal("active v1 hashes changed on draft step save")
	}
	versions, _ = repo.ListTypeVersions(ctx, testSubjectWF.CompanyID, typeID)
	for _, v := range versions {
		if v.VersionNo == 1 && v.IsReleased != v1Released {
			t.Fatalf("is_released changed on active v1: %v -> %v", v1Released, v.IsReleased)
		}
	}
	portal, err := repo.GetTypeDetail(ctx, testSubjectWF.CompanyID, typeID)
	if err != nil {
		t.Fatalf("portal GetTypeDetail: %v", err)
	}
	if portal.VersionNo != 1 {
		t.Fatalf("portal publication moved to v%d", portal.VersionNo)
	}
	got := mustGetWF(t, svc, typeID)
	if got.Steps[0].Instructions != "draft-v2-only" {
		t.Fatalf("draft mutation missing: %q", got.Steps[0].Instructions)
	}
}

func TestWorkflowStepSave_T10T11_HashesRecomputed(t *testing.T) {
	const typeID = "dt-step-save-t10"
	svc, repo := newSeededWFService(t, typeID)
	ctx := context.Background()
	mustUpsertWF(t, svc, typeID, fourSteps(typeID))
	before, err := repo.GetTypeVersionDetail(ctx, testSubjectWF.CompanyID, typeID, 1)
	if err != nil {
		t.Fatalf("before: %v", err)
	}
	edited := fourSteps(typeID)
	edited[0].Instructions = "hash-change"
	mustUpsertWF(t, svc, typeID, edited)
	after, err := repo.GetTypeVersionDetail(ctx, testSubjectWF.CompanyID, typeID, 1)
	if err != nil {
		t.Fatalf("after: %v", err)
	}
	if after.WorkflowSemanticHash == "" || after.WorkflowSemanticHash == before.WorkflowSemanticHash {
		t.Fatalf("workflow_manifest_hash not recomputed: before=%q after=%q", before.WorkflowSemanticHash, after.WorkflowSemanticHash)
	}
	if after.PublicationCandidateHash == "" || after.PublicationCandidateHash == before.PublicationCandidateHash {
		t.Fatalf("publication_candidate_hash not recomputed: before=%q after=%q", before.PublicationCandidateHash, after.PublicationCandidateHash)
	}
}

func TestWorkflowStepSave_T12_InvalidShapeDoesNotCorrupt(t *testing.T) {
	const typeID = "dt-step-save-t12"
	svc, repo := newSeededWFService(t, typeID)
	ctx := context.Background()
	mustUpsertWF(t, svc, typeID, fourSteps(typeID))
	before, err := repo.GetTypeVersionDetail(ctx, testSubjectWF.CompanyID, typeID, 1)
	if err != nil {
		t.Fatalf("before: %v", err)
	}
	bad := fourSteps(typeID)
	bad[0].Stage = ""
	_, err = svc.CmsUpsertGlobalWorkflow(ctx, disclosureapp.CmsUpsertGlobalWorkflowRequest{
		Subject: testSubjectWF, TypeID: typeID, Steps: bad,
	})
	if err == nil {
		t.Fatal("expected invalid step rejection")
	}
	after, err := repo.GetTypeVersionDetail(ctx, testSubjectWF.CompanyID, typeID, 1)
	if err != nil {
		t.Fatalf("after: %v", err)
	}
	if after.WorkflowSemanticHash != before.WorkflowSemanticHash || after.PublicationCandidateHash != before.PublicationCandidateHash {
		t.Fatal("invalid save mutated hashes")
	}
	got := mustGetWF(t, svc, typeID)
	if len(got.Steps) != 4 || got.Steps[0].Stage != "Thu thập dữ liệu" {
		t.Fatalf("invalid save corrupted draft: %+v", got.Steps)
	}
}

func TestWorkflowStepSave_N3N4N5_DoesNotPublishOrRestoreLegacy(t *testing.T) {
	const typeID = "dt-step-save-n3"
	svc, repo := newSeededWFService(t, typeID)
	ctx := context.Background()
	mustUpsertWF(t, svc, typeID, fourSteps(typeID))
	count, err := repo.CountGlobalWorkflowsByTypeId(ctx, typeID)
	if err != nil {
		t.Fatalf("count global: %v", err)
	}
	if count != 0 {
		t.Fatalf("save step touched global workflow rows: %d", count)
	}
	_, _, ok, err := repo.GetActiveGlobalWorkflow(ctx, typeID)
	if err != nil {
		t.Fatalf("GetActiveGlobalWorkflow: %v", err)
	}
	if ok {
		t.Fatal("save step must not create/activate global workflow")
	}
	if n := workflowActiveVersionNo(t, repo, typeID); n != 0 {
		t.Fatalf("save step published template active=%d", n)
	}
	eff, err := repo.GetEffectiveWorkflow(ctx, testSubjectWF.CompanyID, typeID)
	if err == nil && strings.EqualFold(eff.Source, "global_workflow") {
		t.Fatal("save step restored legacy global_workflow runtime source")
	}
}

func TestWorkflowStepSave_N6_DBFailureKeepsHashes(t *testing.T) {
	const typeID = "dt-step-save-n6"
	ctx := context.Background()
	inner := inmemory.NewRepository()
	seedTemplateDraft(t, inner, typeID)
	good := disclosureapp.NewService(inner, nil, idgen.UUIDv7Generator{})
	mustUpsertWF(t, good, typeID, fourSteps(typeID))
	before, err := inner.GetTypeVersionDetail(ctx, testSubjectWF.CompanyID, typeID, 1)
	if err != nil {
		t.Fatalf("before: %v", err)
	}
	failing := &failingUpsertRepo{Repository: inner}
	svc := disclosureapp.NewService(failing, nil, idgen.UUIDv7Generator{})
	edited := fourSteps(typeID)
	edited[0].Instructions = "should-not-land"
	_, err = svc.CmsUpsertGlobalWorkflow(ctx, disclosureapp.CmsUpsertGlobalWorkflowRequest{
		Subject: testSubjectWF, TypeID: typeID, Steps: edited,
	})
	if err == nil {
		t.Fatal("expected db failure")
	}
	after, err := inner.GetTypeVersionDetail(ctx, testSubjectWF.CompanyID, typeID, 1)
	if err != nil {
		t.Fatalf("after: %v", err)
	}
	if after.WorkflowSemanticHash != before.WorkflowSemanticHash || after.PublicationCandidateHash != before.PublicationCandidateHash {
		t.Fatal("failed save left hashes inconsistent")
	}
}

func TestWorkflowStepSave_CMSEditorRedactsEnterpriseSteps(t *testing.T) {
	const typeID = "dt-step-save-redact"
	svc, repo := newSeededWFService(t, typeID)
	ctx := context.Background()
	mustUpsertWF(t, svc, typeID, fourSteps(typeID))
	repoDetail, err := repo.GetTypeVersionDetail(ctx, testSubjectWF.CompanyID, typeID, 1)
	if err != nil {
		t.Fatalf("repo detail: %v", err)
	}
	if len(disclosureapp.ExtractTemplateWorkflow(repoDetail.Blocks)) != 4 {
		t.Fatalf("repo must keep pinned steps, got %d", len(disclosureapp.ExtractTemplateWorkflow(repoDetail.Blocks)))
	}
	editor, err := svc.GetTypeVersionDetail(ctx, disclosureapp.GetTypeVersionDetailRequest{
		Subject: testSubjectWF, TypeID: typeID, VersionNo: 1,
	})
	if err != nil {
		t.Fatalf("editor detail: %v", err)
	}
	if n := len(disclosureapp.ExtractTemplateWorkflow(editor.Blocks)); n != 0 {
		t.Fatalf("CMS editor GET must redact enterprise_workflow steps so Lưu bước is enabled, got %d", n)
	}
	if !editor.HasWorkflow {
		t.Fatal("redacted editor GET must still report has_workflow from pinned manifest")
	}
}

type failingUpsertRepo struct {
	*inmemory.Repository
}

func (r *failingUpsertRepo) UpsertTypeVersion(context.Context, disclosureapp.UpsertTypeVersionRequest) (*disclosureapp.UpsertTypeVersionResponse, error) {
	return nil, errors.New("simulated db failure")
}

func TestWorkflowStepSave_DuplicateStepKeyRejected(t *testing.T) {
	const typeID = "dt-step-save-dup"
	svc, _ := newSeededWFService(t, typeID)
	steps := fourSteps(typeID)
	steps[1].StepID = steps[0].StepID
	steps[1].StepKey = steps[0].StepID
	_, err := svc.CmsUpsertGlobalWorkflow(context.Background(), disclosureapp.CmsUpsertGlobalWorkflowRequest{
		Subject: testSubjectWF, TypeID: typeID, Steps: steps,
	})
	if err == nil {
		t.Fatal("expected duplicate step_key rejection")
	}
	if herr, ok := err.(*perr.HTTPError); !ok || herr.HTTPStatus != 400 {
		t.Fatalf("want 400, got %v", err)
	}
}

// mysqlNoActiveEffectiveRepo mirrors MySQL GetEffectiveWorkflow when
// disclosure_types.active_version_no=0: GetTypeDetail INNER JOIN 404s.
type mysqlNoActiveEffectiveRepo struct {
	*inmemory.Repository
}

func (r *mysqlNoActiveEffectiveRepo) GetEffectiveWorkflow(ctx context.Context, companyID, typeID string) (*disclosureapp.EffectiveWorkflowDTO, error) {
	has, err := r.HasActiveEnterpriseWorkflow(ctx, companyID, typeID)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "disclosure type not found", nil)
	}
	return r.Repository.GetEffectiveWorkflow(ctx, companyID, typeID)
}

type catalogAuth struct {
	perms []string
}

func (a *catalogAuth) GetEffectiveAccess(_ context.Context, _, _ string) (*authapp.EffectiveAccessSummary, error) {
	return &authapp.EffectiveAccessSummary{Permissions: append([]string(nil), a.perms...)}, nil
}

func (a *catalogAuth) Authorize(_ context.Context, _ authapp.AuthorizeRequest) (*authapp.AuthorizeDecision, error) {
	return &authapp.AuthorizeDecision{Decision: authapp.DecisionAllow}, nil
}

func (a *catalogAuth) AuthorizeBatch(_ context.Context, req authapp.AuthorizeBatchRequest) (*authapp.AuthorizeBatchResponse, error) {
	out := make([]authapp.AuthorizeDecision, len(req.Checks))
	for i := range out {
		out[i] = authapp.AuthorizeDecision{Decision: authapp.DecisionAllow}
	}
	return &authapp.AuthorizeBatchResponse{Results: out}, nil
}

func TestWorkflowStepSave_CMSEditorEffectivePreview_DoesNotOpenRuntime(t *testing.T) {
	const typeID = "dt-step-save-cms-preview"
	inner := inmemory.NewRepository()
	seedTemplateDraft(t, inner, typeID)
	writer := disclosureapp.NewService(inner, nil, idgen.UUIDv7Generator{})
	mustUpsertWF(t, writer, typeID, fourSteps(typeID))

	repo := &mysqlNoActiveEffectiveRepo{Repository: inner}
	cms := disclosureapp.NewService(repo, &catalogAuth{perms: []string{"platform.cms.view"}}, idgen.UUIDv7Generator{})
	company := disclosureapp.NewService(repo, &catalogAuth{perms: []string{}}, idgen.UUIDv7Generator{})
	ctx := context.Background()

	preview, err := cms.GetEffectiveWorkflow(ctx, disclosureapp.GetEffectiveWorkflowRequest{
		Subject: testSubjectWF, TypeID: typeID,
	})
	if err != nil {
		t.Fatalf("CMS unpublished preview: %v", err)
	}
	if preview.Data.Source != "global_template" {
		t.Fatalf("preview source=%q", preview.Data.Source)
	}
	if len(preview.Data.Workflow) != 4 {
		t.Fatalf("preview steps=%d want 4", len(preview.Data.Workflow))
	}

	hasRuntime, err := inner.HasActiveEnterpriseWorkflow(ctx, testSubjectWF.CompanyID, typeID)
	if err != nil {
		t.Fatalf("HasActiveEnterpriseWorkflow: %v", err)
	}
	if hasRuntime {
		t.Fatal("CMS preview must not imply Portal runtime authority")
	}

	_, err = company.GetEffectiveWorkflow(ctx, disclosureapp.GetEffectiveWorkflowRequest{
		Subject: testSubjectWF, TypeID: typeID,
	})
	if err == nil {
		t.Fatal("company/portal GetEffectiveWorkflow must stay 404 until activate")
	}
	if herr, ok := err.(*perr.HTTPError); !ok || herr.HTTPStatus != http.StatusNotFound {
		t.Fatalf("want 404 for company catalog, got %v", err)
	}

	if _, err := writer.ActivateTypeVersion(ctx, disclosureapp.ActivateTypeVersionRequest{
		Subject: testSubjectWF, TypeID: typeID, VersionNo: 1,
	}); err != nil {
		t.Fatalf("activate: %v", err)
	}
	hasRuntime, err = inner.HasActiveEnterpriseWorkflow(ctx, testSubjectWF.CompanyID, typeID)
	if err != nil || !hasRuntime {
		t.Fatalf("after activate HasActiveEnterpriseWorkflow=%v err=%v", hasRuntime, err)
	}
	published, err := company.GetEffectiveWorkflow(ctx, disclosureapp.GetEffectiveWorkflowRequest{
		Subject: testSubjectWF, TypeID: typeID,
	})
	if err != nil {
		t.Fatalf("company after activate: %v", err)
	}
	if published.Data.VersionNo != 1 || len(published.Data.Workflow) != 4 {
		t.Fatalf("published effective version=%d steps=%d", published.Data.VersionNo, len(published.Data.Workflow))
	}
}
