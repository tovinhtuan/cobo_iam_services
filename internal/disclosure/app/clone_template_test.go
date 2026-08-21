package app_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

func mustActivateSource(t *testing.T, svc disclosureapp.Service, typeID string, versionNo int) {
	t.Helper()
	_, err := svc.ActivateTypeVersion(context.Background(), disclosureapp.ActivateTypeVersionRequest{
		Subject: testSubjectWF, TypeID: typeID, VersionNo: versionNo, Reason: "clone-source",
	})
	if err != nil {
		t.Fatalf("ActivateTypeVersion: %v", err)
	}
}

func seedActiveCloneSource(t *testing.T, typeID string, steps []disclosureapp.GlobalWorkflowStepInput) (disclosureapp.Service, *inmemory.Repository) {
	t.Helper()
	svc, repo := newSeededWFService(t, typeID)
	mustUpsertWF(t, svc, typeID, steps)
	mustActivateSource(t, svc, typeID, 1)
	return svc, repo
}

func httpErrStatus(err error) int {
	var he *perr.HTTPError
	if ok := asHTTPError(err, &he); ok {
		return he.HTTPStatus
	}
	return 0
}

func asHTTPError(err error, out **perr.HTTPError) bool {
	if err == nil {
		return false
	}
	if he, ok := err.(*perr.HTTPError); ok {
		*out = he
		return true
	}
	return false
}

func TestCloneTypeFromActive_T1T2_ActiveToDraftV1(t *testing.T) {
	const sourceID = "dt-clone-src-t1"
	const targetID = "dt-clone-tgt-t1"
	svc, repo := seedActiveCloneSource(t, sourceID, fourSteps(sourceID))

	resp, err := svc.CloneTypeFromActive(context.Background(), disclosureapp.CloneTypeFromActiveRequest{
		Subject: testSubjectWF, SourceTypeID: sourceID, TargetTypeID: targetID,
		TargetName: "Clone Target T1", ExpectedSourceVersionNo: 1,
	})
	if err != nil {
		t.Fatalf("CloneTypeFromActive: %v", err)
	}
	if resp.TypeID != targetID || resp.VersionNo != 1 || resp.IsActive {
		t.Fatalf("response=%+v want target draft v1", resp)
	}
	if n := workflowActiveVersionNo(t, repo, targetID); n != 0 {
		t.Fatalf("target active_version_no=%d want 0", n)
	}
	detail, err := repo.GetTypeVersionDetail(context.Background(), testSubjectWF.CompanyID, targetID, 1)
	if err != nil {
		t.Fatalf("GetTypeVersionDetail: %v", err)
	}
	if detail.WorkflowAuthorityMode != disclosureapp.WorkflowAuthorityTemplatePinned {
		t.Fatalf("authority=%q", detail.WorkflowAuthorityMode)
	}
	if got := len(disclosureapp.ExtractTemplateWorkflow(detail.Blocks)); got != 4 {
		t.Fatalf("workflow steps=%d want 4", got)
	}
}

func TestCloneTypeFromActive_T5T6T7_WorkflowDeepCopyAndHashes(t *testing.T) {
	const sourceID = "dt-clone-src-hash"
	const targetID = "dt-clone-tgt-hash"
	svc, repo := seedActiveCloneSource(t, sourceID, fourSteps(sourceID))

	src, err := repo.GetTypeVersionDetail(context.Background(), testSubjectWF.CompanyID, sourceID, 1)
	if err != nil {
		t.Fatalf("source detail: %v", err)
	}

	_, err = svc.CloneTypeFromActive(context.Background(), disclosureapp.CloneTypeFromActiveRequest{
		Subject: testSubjectWF, SourceTypeID: sourceID, TargetTypeID: targetID,
		TargetName: "Hash Target", ExpectedSourceVersionNo: 1,
	})
	if err != nil {
		t.Fatalf("clone: %v", err)
	}
	tgt, err := repo.GetTypeVersionDetail(context.Background(), testSubjectWF.CompanyID, targetID, 1)
	if err != nil {
		t.Fatalf("target detail: %v", err)
	}
	if tgt.WorkflowSemanticHash == "" || tgt.PublicationCandidateHash == "" {
		t.Fatal("target hashes empty")
	}
	if tgt.PublicationCandidateHash == src.PublicationCandidateHash {
		t.Fatal("publication_candidate_hash must be recomputed for new type_id/name")
	}
	// Same workflow content may share semantic hash; type_id is not in workflow hash.
	if len(disclosureapp.ExtractTemplateWorkflow(tgt.Blocks)) != len(disclosureapp.ExtractTemplateWorkflow(src.Blocks)) {
		t.Fatal("step count mismatch")
	}
}

func TestCloneTypeFromActive_T3T4T18_SourceTargetIsolation(t *testing.T) {
	const sourceID = "dt-clone-src-iso"
	const targetID = "dt-clone-tgt-iso"
	svc, repo := seedActiveCloneSource(t, sourceID, fourSteps(sourceID))

	_, err := svc.CloneTypeFromActive(context.Background(), disclosureapp.CloneTypeFromActiveRequest{
		Subject: testSubjectWF, SourceTypeID: sourceID, TargetTypeID: targetID,
		TargetName: "Iso Target", ExpectedSourceVersionNo: 1,
	})
	if err != nil {
		t.Fatalf("clone: %v", err)
	}

	// Edit target workflow (creates draft overwrite still v1 while inactive).
	edited := fourSteps(targetID)
	edited[0].Instructions = "target-only-edit"
	mustUpsertWF(t, svc, targetID, edited)

	srcAfter, err := repo.GetTypeVersionDetail(context.Background(), testSubjectWF.CompanyID, sourceID, 1)
	if err != nil {
		t.Fatalf("source after target edit: %v", err)
	}
	srcSteps := disclosureapp.ExtractTemplateWorkflow(srcAfter.Blocks)
	if len(srcSteps) != 4 || srcSteps[0].Instructions == "target-only-edit" {
		t.Fatalf("source mutated by target edit: %+v", srcSteps[0])
	}

	srcHash := srcAfter.PublicationCandidateHash

	// Publish source v2 with different step count.
	v2 := fourSteps(sourceID)
	v2 = append(v2, disclosureapp.GlobalWorkflowStepInput{
		StepID: sourceID + "-step-5", Stage: "Extra", DepartmentID: "d5",
		AssigneeRoleIds: []string{"r5"}, DueRule: "T+5", ProcessingDays: 5, DisplayOrder: 5,
	})
	mustUpsertWF(t, svc, sourceID, v2)
	mustActivateSource(t, svc, sourceID, 2)

	tgt, err := repo.GetTypeVersionDetail(context.Background(), testSubjectWF.CompanyID, targetID, 1)
	if err != nil {
		t.Fatalf("target after source publish: %v", err)
	}
	if got := len(disclosureapp.ExtractTemplateWorkflow(tgt.Blocks)); got != 4 {
		t.Fatalf("target steps=%d want 4 after source grew", got)
	}
	_ = srcHash
}

func TestCloneTypeFromActive_T10_ClearsMembershipAssignees(t *testing.T) {
	const sourceID = "dt-clone-src-assignee"
	const targetID = "dt-clone-tgt-assignee"
	svc, repo := newSeededWFService(t, sourceID)
	mustUpsertWF(t, svc, sourceID, fourSteps(sourceID))

	detail, err := repo.GetTypeVersionDetail(context.Background(), testSubjectWF.CompanyID, sourceID, 1)
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	wfSteps := disclosureapp.ExtractTemplateWorkflow(detail.Blocks)
	if len(wfSteps) == 0 {
		t.Fatal("expected source steps")
	}
	wfSteps[0].AssigneeMembershipID = "m-tenant-1"
	wfSteps[0].AssigneeMembershipIDs = []string{"m-tenant-1", "m-tenant-2"}
	blocks, err := replaceWorkflowForTest(detail.Blocks, wfSteps)
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	upsert := disclosureapp.UpsertTypeVersionRequest{
		Subject: testSubjectWF, TypeID: sourceID, Scope: "global", GroupID: detail.GroupID,
		Name: detail.Name, Category: detail.Category, TemplateCategory: detail.TemplateCategory,
		DeadlineStrategy: detail.DeadlineStrategy, DeadlineRule: detail.DeadlineRule,
		Periodicity: detail.Periodicity, DisplayGroupCodes: detail.DisplayGroupCodes,
		ApplicabilityRules: detail.ApplicabilityRules, Blocks: blocks, Description: detail.Description,
	}
	cand, err := disclosureapp.BuildTemplatePublicationCandidate(upsert)
	if err != nil {
		t.Fatalf("candidate: %v", err)
	}
	upsert.PublicationCandidate = &cand
	if _, err := repo.UpsertTypeVersion(context.Background(), upsert); err != nil {
		t.Fatalf("repo upsert: %v", err)
	}
	mustActivateSource(t, svc, sourceID, 1)

	_, err = svc.CloneTypeFromActive(context.Background(), disclosureapp.CloneTypeFromActiveRequest{
		Subject: testSubjectWF, SourceTypeID: sourceID, TargetTypeID: targetID,
		TargetName: "Assignee Target", ExpectedSourceVersionNo: 1,
	})
	if err != nil {
		t.Fatalf("clone: %v", err)
	}
	tgt, err := repo.GetTypeVersionDetail(context.Background(), testSubjectWF.CompanyID, targetID, 1)
	if err != nil {
		t.Fatalf("target: %v", err)
	}
	for i, step := range disclosureapp.ExtractTemplateWorkflow(tgt.Blocks) {
		if step.AssigneeMembershipID != "" || len(step.AssigneeMembershipIDs) > 0 {
			t.Fatalf("step %d still has membership assignees: %+v", i, step)
		}
		if len(step.AssigneeRoleIds) == 0 {
			t.Fatalf("step %d lost assignee_role_ids", i)
		}
	}
}

// replaceWorkflowForTest builds enterprise_workflow steps using JSON round-trip of WorkflowStepDTO.
func replaceWorkflowForTest(blocks []disclosureapp.TemplateBlockDTO, steps []disclosureapp.WorkflowStepDTO) ([]disclosureapp.TemplateBlockDTO, error) {
	raw, err := json.Marshal(steps)
	if err != nil {
		return nil, err
	}
	var projected []any
	if err := json.Unmarshal(raw, &projected); err != nil {
		return nil, err
	}
	out := make([]disclosureapp.TemplateBlockDTO, 0, len(blocks))
	for _, b := range blocks {
		next := b
		if strings.EqualFold(strings.TrimSpace(b.BlockKey), "enterprise_workflow") {
			next.Config = map[string]any{"steps": projected}
		}
		out = append(out, next)
	}
	return out, nil
}

func TestCloneTypeFromActive_T12_DuplicateTarget409(t *testing.T) {
	const sourceID = "dt-clone-src-dup"
	const targetID = "dt-clone-tgt-dup"
	svc, repo := seedActiveCloneSource(t, sourceID, fourSteps(sourceID))
	seedTemplateDraft(t, repo, targetID)

	_, err := svc.CloneTypeFromActive(context.Background(), disclosureapp.CloneTypeFromActiveRequest{
		Subject: testSubjectWF, SourceTypeID: sourceID, TargetTypeID: targetID,
		TargetName: "Dup", ExpectedSourceVersionNo: 1,
	})
	if httpErrStatus(err) != http.StatusConflict {
		t.Fatalf("want 409, got %v (status=%d)", err, httpErrStatus(err))
	}
}

func TestCloneTypeFromActive_T13_NoActive422(t *testing.T) {
	const sourceID = "dt-clone-src-draft"
	svc, _ := newSeededWFService(t, sourceID)
	mustUpsertWF(t, svc, sourceID, fourSteps(sourceID))

	_, err := svc.CloneTypeFromActive(context.Background(), disclosureapp.CloneTypeFromActiveRequest{
		Subject: testSubjectWF, SourceTypeID: sourceID, TargetTypeID: "dt-clone-tgt-draft",
		TargetName: "X", ExpectedSourceVersionNo: 1,
	})
	if httpErrStatus(err) != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %v (status=%d)", err, httpErrStatus(err))
	}
}

func TestCloneTypeFromActive_N3_SourceVersionRace409(t *testing.T) {
	const sourceID = "dt-clone-src-race"
	svc, _ := seedActiveCloneSource(t, sourceID, fourSteps(sourceID))
	// Move active to v2.
	mustUpsertWF(t, svc, sourceID, fourSteps(sourceID+"-v2"))
	mustActivateSource(t, svc, sourceID, 2)

	_, err := svc.CloneTypeFromActive(context.Background(), disclosureapp.CloneTypeFromActiveRequest{
		Subject: testSubjectWF, SourceTypeID: sourceID, TargetTypeID: "dt-clone-tgt-race",
		TargetName: "Race", ExpectedSourceVersionNo: 1, // stale
	})
	if httpErrStatus(err) != http.StatusConflict {
		t.Fatalf("want 409, got %v (status=%d)", err, httpErrStatus(err))
	}
}

func TestCloneTypeFromActive_T14_Unauthorized403(t *testing.T) {
	const sourceID = "dt-clone-src-auth"
	svcWrite, repo := seedActiveCloneSource(t, sourceID, fourSteps(sourceID))
	_ = svcWrite
	denied := disclosureapp.NewService(repo, &catalogAuth{perms: []string{}}, idgen.UUIDv7Generator{})
	_, err := denied.CloneTypeFromActive(context.Background(), disclosureapp.CloneTypeFromActiveRequest{
		Subject: testSubjectWF, SourceTypeID: sourceID, TargetTypeID: "dt-clone-tgt-auth",
		TargetName: "Auth", ExpectedSourceVersionNo: 1,
	})
	if httpErrStatus(err) != http.StatusForbidden {
		t.Fatalf("want 403, got %v (status=%d)", err, httpErrStatus(err))
	}
}

func TestCloneTypeFromActive_T19_ExplicitNoWorkflow(t *testing.T) {
	const sourceID = "dt-clone-src-nowf"
	const targetID = "dt-clone-tgt-nowf"
	repo := inmemory.NewRepository()
	seedTemplateDraft(t, repo, sourceID)
	svc := disclosureapp.NewService(repo, nil, idgen.UUIDv7Generator{})
	// Activate via repo (bypasses service empty-workflow gate) while keeping TEMPLATE_PINNED.
	if _, err := repo.ActivateTypeVersion(context.Background(), disclosureapp.ActivateTypeVersionRequest{
		Subject: testSubjectWF, TypeID: sourceID, VersionNo: 1,
	}); err != nil {
		t.Fatalf("repo activate: %v", err)
	}

	resp, err := svc.CloneTypeFromActive(context.Background(), disclosureapp.CloneTypeFromActiveRequest{
		Subject: testSubjectWF, SourceTypeID: sourceID, TargetTypeID: targetID,
		TargetName: "NoWF Target", ExpectedSourceVersionNo: 1,
	})
	if err != nil {
		t.Fatalf("clone NO_WORKFLOW: %v", err)
	}
	if resp.VersionNo != 1 || resp.IsActive {
		t.Fatalf("unexpected resp %+v", resp)
	}
	tgt, err := repo.GetTypeVersionDetail(context.Background(), testSubjectWF.CompanyID, targetID, 1)
	if err != nil {
		t.Fatalf("target: %v", err)
	}
	if tgt.WorkflowAuthorityMode != disclosureapp.WorkflowAuthorityTemplatePinned {
		t.Fatalf("authority=%q", tgt.WorkflowAuthorityMode)
	}
	if n := len(disclosureapp.ExtractTemplateWorkflow(tgt.Blocks)); n != 0 {
		t.Fatalf("want empty workflow, got %d", n)
	}
}

func TestCloneTypeFromActive_N20_InvalidSourceWorkflow422(t *testing.T) {
	const sourceID = "dt-clone-src-badwf"
	const targetID = "dt-clone-tgt-badwf"
	repo := inmemory.NewRepository()
	seedTemplateDraft(t, repo, sourceID)
	svc := disclosureapp.NewService(repo, nil, idgen.UUIDv7Generator{})

	detail, err := repo.GetTypeVersionDetail(context.Background(), testSubjectWF.CompanyID, sourceID, 1)
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	badSteps := []disclosureapp.WorkflowStepDTO{
		{StepID: "bad-1", Stage: "Bad", DepartmentID: "", AssigneeRoleIds: []string{"r1"}, ProcessingDays: 1, DisplayOrder: 1},
	}
	blocks, err := replaceWorkflowForTest(detail.Blocks, badSteps)
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	upsert := disclosureapp.UpsertTypeVersionRequest{
		Subject: testSubjectWF, TypeID: sourceID, Scope: "global", GroupID: detail.GroupID,
		Name: detail.Name, Category: detail.Category, TemplateCategory: detail.TemplateCategory,
		DeadlineStrategy: detail.DeadlineStrategy, DeadlineRule: detail.DeadlineRule,
		Periodicity: detail.Periodicity, DisplayGroupCodes: detail.DisplayGroupCodes,
		ApplicabilityRules: detail.ApplicabilityRules, Blocks: blocks, Description: detail.Description,
	}
	cand, err := disclosureapp.BuildTemplatePublicationCandidate(upsert)
	if err != nil {
		t.Fatalf("candidate: %v", err)
	}
	upsert.PublicationCandidate = &cand
	if _, err := repo.UpsertTypeVersion(context.Background(), upsert); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if _, err := repo.ActivateTypeVersion(context.Background(), disclosureapp.ActivateTypeVersionRequest{
		Subject: testSubjectWF, TypeID: sourceID, VersionNo: 1,
	}); err != nil {
		t.Fatalf("activate: %v", err)
	}

	beforeExists, _ := repo.TypeExists(context.Background(), targetID)
	_, err = svc.CloneTypeFromActive(context.Background(), disclosureapp.CloneTypeFromActiveRequest{
		Subject: testSubjectWF, SourceTypeID: sourceID, TargetTypeID: targetID,
		TargetName: "BadWF", ExpectedSourceVersionNo: 1,
	})
	if httpErrStatus(err) != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %v (status=%d)", err, httpErrStatus(err))
	}
	afterExists, _ := repo.TypeExists(context.Background(), targetID)
	if !beforeExists && afterExists {
		t.Fatal("target must not be created on invalid source workflow")
	}
}

func TestCloneTypeFromActive_N1_SourceNotFound404(t *testing.T) {
	svc := newWFService()
	_, err := svc.CloneTypeFromActive(context.Background(), disclosureapp.CloneTypeFromActiveRequest{
		Subject: testSubjectWF, SourceTypeID: "missing-type", TargetTypeID: "tgt",
		TargetName: "X", ExpectedSourceVersionNo: 1,
	})
	if httpErrStatus(err) != http.StatusNotFound {
		t.Fatalf("want 404, got %v (status=%d)", err, httpErrStatus(err))
	}
}

func TestCloneTypeFromActive_N12_NoAutoPublish(t *testing.T) {
	const sourceID = "dt-clone-src-nopub"
	const targetID = "dt-clone-tgt-nopub"
	svc, repo := seedActiveCloneSource(t, sourceID, fourSteps(sourceID))
	_, err := svc.CloneTypeFromActive(context.Background(), disclosureapp.CloneTypeFromActiveRequest{
		Subject: testSubjectWF, SourceTypeID: sourceID, TargetTypeID: targetID,
		TargetName: "NoPub", ExpectedSourceVersionNo: 1,
	})
	if err != nil {
		t.Fatalf("clone: %v", err)
	}
	if n := workflowActiveVersionNo(t, repo, targetID); n != 0 {
		t.Fatalf("auto-published active=%d", n)
	}
	// Portal GetTypeDetail should 404 while inactive.
	if _, err := repo.GetTypeDetail(context.Background(), testSubjectWF.CompanyID, targetID); err == nil {
		t.Fatal("portal GetTypeDetail should fail for inactive clone target")
	}
}
