package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

type spyStepContentResolver struct {
	calls   int
	sub     Subject
	typeID  string
	catalog *StepContentCatalog
	err     error
}

func (r *spyStepContentResolver) ResolveStepContent(_ context.Context, sub Subject, typeID string) (*StepContentCatalog, error) {
	r.calls++
	r.sub = sub
	r.typeID = typeID
	return r.catalog, r.err
}

func sampleStepContentCatalog() *StepContentCatalog {
	return &StepContentCatalog{
		WorkflowSource: "company_override",
		VersionNo:      4,
		BySourceStepID: map[string]ProposalStepInheritedContent{
			"tpl-1": {
				Description:       "<p>Đối chiếu</p>",
				DescriptionFormat: "safe_html",
				Documents: []ProposalStepDocument{
					{DocID: "doc-1", Name: "Nghị quyết HĐQT", Required: true, TemplateFileID: "wdt-1", TemplateFileName: "mau.docx"},
					{DocID: "doc-2", Name: "Sơ yếu lý lịch"},
				},
				ReminderConfig: &ProposalStepReminderConfig{Enabled: true, Mode: "days_before", DaysBefore: []int{5, 2}},
			},
		},
	}
}

func v3DraftForCapture(steps ...ProposalWorkflowStep) *fakeRepository {
	return &fakeRepository{
		proposal: &ProposalDTO{
			ProposalID: "p1", CompanyID: "co", Status: StatusDraft, TypeID: "type-1", CreatedBy: "creator",
			Workflow: &ProposalWorkflowSnapshot{SchemaVersion: ProposalWorkflowSchemaV3, DisclosureTypeID: "type-1", Steps: steps},
		},
		reviewers: []ReviewerDTO{{MembershipID: "r1"}},
	}
}

func captureTestOrg() *fakeOrgDirectory {
	return &fakeOrgDirectory{
		depts:   map[string]bool{"dep-1": true},
		members: map[string]bool{"m1": true},
		belong:  map[string]bool{"m1\x00dep-1": true},
	}
}

func newCaptureTestService(repo *fakeRepository, org *fakeOrgDirectory, resolver StepContentResolver) Service {
	svc := AttachWorkflowDeps(
		NewService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, &fakeAuthService{decision: authapp.DecisionAllow}, newAllowValidator(), nil, noopMetrics{}),
		org, nil,
	)
	if resolver != nil {
		svc = AttachStepContentCapture(svc, resolver)
	}
	return svc
}

func submitAsCreator(t *testing.T, svc Service) (*ProposalDTO, error) {
	t.Helper()
	return svc.SubmitProposal(context.Background(), ProposalActionRequest{
		Subject: Subject{UserID: "u-1", MembershipID: "creator", CompanyID: "co"}, ProposalID: "p1",
	})
}

func TestSubmitProposal_CapturesStepContentFromTemplate(t *testing.T) {
	repo := v3DraftForCapture(
		ProposalWorkflowStep{ID: "ps1", SourceStepID: "tpl-1", Order: 1, Name: "A", ProcessingDays: 1, DepartmentID: "dep-1", AssigneeMembershipIDs: []string{"m1"}},
		ProposalWorkflowStep{ID: "ps2", Order: 2, Name: "Tự thêm", ProcessingDays: 1, DepartmentID: "dep-1", AssigneeMembershipIDs: []string{"m1"}},
		ProposalWorkflowStep{ID: "ps3", SourceStepID: "tpl-gone", Order: 3, Name: "C", ProcessingDays: 1, DepartmentID: "dep-1", AssigneeMembershipIDs: []string{"m1"}},
	)
	catalog := sampleStepContentCatalog()
	resolver := &spyStepContentResolver{catalog: catalog}
	out, err := submitAsCreator(t, newCaptureTestService(repo, captureTestOrg(), resolver))
	if err != nil {
		t.Fatal(err)
	}
	if resolver.calls != 1 || resolver.sub.CompanyID != "co" || resolver.sub.MembershipID != "creator" || resolver.typeID != "type-1" {
		t.Fatalf("resolver calls=%d sub=%#v type=%q", resolver.calls, resolver.sub, resolver.typeID)
	}
	wf := repo.lastUpdate.Workflow
	if wf == nil || !wf.Frozen || out.Status != StatusPendingFocalApproval {
		t.Fatalf("workflow=%#v status=%s", wf, out.Status)
	}
	inh := wf.Steps[0].Inherited
	if inh == nil || inh.DescriptionFormat != "safe_html" || len(inh.Documents) != 2 || !inh.Documents[0].Required ||
		inh.Documents[0].TemplateFileID != "wdt-1" || inh.ReminderConfig == nil || len(inh.ReminderConfig.DaysBefore) != 2 {
		t.Fatalf("step 1 inherited=%#v", inh)
	}
	if wf.Steps[1].Inherited != nil || wf.Steps[2].Inherited != nil {
		t.Fatalf("self-added / unmatched steps must not inherit: %#v %#v", wf.Steps[1].Inherited, wf.Steps[2].Inherited)
	}
	cc := wf.ContentCapture
	if cc == nil || cc.WorkflowSource != "company_override" || cc.VersionNo != 4 || cc.CapturedAt.IsZero() ||
		len(cc.MissingSourceStepIDs) != 1 || cc.MissingSourceStepIDs[0] != "tpl-gone" {
		t.Fatalf("content_capture=%#v", cc)
	}
	// Captured content is a copy: later catalog edits must not leak into the frozen snapshot.
	tpl := catalog.BySourceStepID["tpl-1"]
	tpl.Documents[0].Name = "changed"
	tpl.ReminderConfig.DaysBefore[0] = 9
	if inh.Documents[0].Name != "Nghị quyết HĐQT" || inh.ReminderConfig.DaysBefore[0] != 5 {
		t.Fatalf("inherited content aliases the catalog: %#v", inh)
	}
}

func TestSubmitProposal_V2CapturesWithoutMutatingDraft(t *testing.T) {
	repo := &fakeRepository{
		proposal: &ProposalDTO{
			ProposalID: "p1", CompanyID: "co", TypeID: "type-1", Status: StatusDraft, CreatedBy: "creator",
			Workflow: &ProposalWorkflowSnapshot{
				SchemaVersion: ProposalWorkflowSchemaV2,
				Steps: []ProposalWorkflowStep{{
					ID: "ps-1", SourceStepID: "tpl-1", Order: 1, Name: "A", ProcessingDays: 1, DepartmentID: "dep-1", AssigneeMembershipID: "m1",
				}},
			},
		},
		reviewers: []ReviewerDTO{{MembershipID: "r1"}},
	}
	draft := repo.proposal.Workflow
	_, err := submitAsCreator(t, newCaptureTestService(repo, captureTestOrg(), &spyStepContentResolver{catalog: sampleStepContentCatalog()}))
	if err != nil {
		t.Fatal(err)
	}
	if repo.lastUpdate.Workflow.Steps[0].Inherited == nil {
		t.Fatal("v2 submit must capture step content")
	}
	if draft.Steps[0].Inherited != nil || draft.ContentCapture != nil {
		t.Fatalf("loaded draft snapshot must not be mutated: %#v", draft)
	}
}

func TestSubmitProposal_FlagOffDoesNotCaptureAndStripsStaleContent(t *testing.T) {
	repo := v3DraftForCapture(ProposalWorkflowStep{
		ID: "ps1", SourceStepID: "tpl-1", Order: 1, Name: "A", ProcessingDays: 1, DepartmentID: "dep-1", AssigneeMembershipIDs: []string{"m1"},
		Inherited: &ProposalStepInheritedContent{Description: "client supplied"},
	})
	repo.proposal.Workflow.ContentCapture = &ProposalContentCapture{WorkflowSource: "forged"}
	if _, err := submitAsCreator(t, newCaptureTestService(repo, captureTestOrg(), nil)); err != nil {
		t.Fatal(err)
	}
	wf := repo.lastUpdate.Workflow
	if wf.Steps[0].Inherited != nil || wf.ContentCapture != nil {
		t.Fatalf("flag off must not persist inherited content: %#v", wf)
	}
}

func TestSubmitProposal_StepContentUnavailableReturns503AndStaysDraft(t *testing.T) {
	repo := v3DraftForCapture(ProposalWorkflowStep{ID: "ps1", SourceStepID: "tpl-1", Order: 1, Name: "A", ProcessingDays: 1, DepartmentID: "dep-1", AssigneeMembershipIDs: []string{"m1"}})
	resolver := &spyStepContentResolver{err: errors.New("mysql: connection refused")}
	_, err := submitAsCreator(t, newCaptureTestService(repo, captureTestOrg(), resolver))
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusServiceUnavailable || he.Code != perr.CodeServiceUnavailable || he.Details["reason"] != StepContentUnavailableReason {
		t.Fatalf("want 503 step_content_unavailable, got %#v", err)
	}
	if repo.updateCalls != 0 || repo.proposal.Status != StatusDraft {
		t.Fatalf("proposal must stay draft: updates=%d status=%s", repo.updateCalls, repo.proposal.Status)
	}
	// A 5xx HTTP error from the template read is unavailable too.
	resolver.err = perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "boom", nil)
	if _, err := submitAsCreator(t, newCaptureTestService(repo, captureTestOrg(), resolver)); err == nil {
		t.Fatal("5xx resolver error must block submit")
	}
}

func TestSubmitProposal_TemplateNotFoundSubmitsWithoutCapture(t *testing.T) {
	repo := v3DraftForCapture(ProposalWorkflowStep{ID: "ps1", SourceStepID: "tpl-1", Order: 1, Name: "A", ProcessingDays: 1, DepartmentID: "dep-1", AssigneeMembershipIDs: []string{"m1"}})
	resolver := &spyStepContentResolver{err: perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "type not found", nil)}
	out, err := submitAsCreator(t, newCaptureTestService(repo, captureTestOrg(), resolver))
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != StatusPendingFocalApproval || repo.lastUpdate.Workflow.Steps[0].Inherited != nil || repo.lastUpdate.Workflow.ContentCapture != nil {
		t.Fatalf("4xx template read must submit without capture: %#v", repo.lastUpdate.Workflow)
	}
}

func TestSubmitProposal_DepartmentErrorBeforeContentResolve(t *testing.T) {
	repo := v3DraftForCapture(ProposalWorkflowStep{ID: "ps1", SourceStepID: "tpl-1", Order: 1, Name: "A", ProcessingDays: 1})
	resolver := &spyStepContentResolver{catalog: sampleStepContentCatalog()}
	_, err := submitAsCreator(t, newCaptureTestService(repo, captureTestOrg(), resolver))
	if err == nil || !strings.Contains(err.Error(), "department_required") {
		t.Fatalf("want department_required, got %v", err)
	}
	if resolver.calls != 0 {
		t.Fatalf("resolver must not run before org validation, calls=%d", resolver.calls)
	}
}

func TestSubmitProposal_OnlySelfAddedStepsSkipsResolver(t *testing.T) {
	repo := v3DraftForCapture(ProposalWorkflowStep{ID: "ps1", Order: 1, Name: "A", ProcessingDays: 1, DepartmentID: "dep-1", AssigneeMembershipIDs: []string{"m1"}})
	resolver := &spyStepContentResolver{err: errors.New("must not be called")}
	if _, err := submitAsCreator(t, newCaptureTestService(repo, captureTestOrg(), resolver)); err != nil {
		t.Fatal(err)
	}
	cc := repo.lastUpdate.Workflow.ContentCapture
	if resolver.calls != 0 || cc == nil || cc.WorkflowSource != "" || repo.lastUpdate.Workflow.Steps[0].Inherited != nil {
		t.Fatalf("calls=%d capture=%#v", resolver.calls, cc)
	}
}

func TestApplyStepContentCatalog_SharedSourceAndDuplicateMissing(t *testing.T) {
	snap := &ProposalWorkflowSnapshot{SchemaVersion: ProposalWorkflowSchemaV3, Steps: []ProposalWorkflowStep{
		{ID: "a", SourceStepID: "tpl-1"},
		{ID: "b", SourceStepID: "tpl-1"},
		{ID: "c", SourceStepID: "gone"},
		{ID: "d", SourceStepID: "gone"},
	}}
	ApplyStepContentCatalog(snap, sampleStepContentCatalog(), time.Date(2026, 10, 20, 3, 0, 0, 0, time.UTC))
	if snap.Steps[0].Inherited == nil || snap.Steps[1].Inherited == nil || snap.Steps[0].Inherited == snap.Steps[1].Inherited {
		t.Fatal("both steps sharing a source must get their own copy")
	}
	if got := snap.ContentCapture.MissingSourceStepIDs; len(got) != 1 || got[0] != "gone" {
		t.Fatalf("missing=%v", got)
	}
}

func TestProposalWorkflowSnapshot_InheritedJSONRoundTrip(t *testing.T) {
	snap := &ProposalWorkflowSnapshot{SchemaVersion: ProposalWorkflowSchemaV3, Frozen: true, Steps: []ProposalWorkflowStep{
		{ID: "ps1", SourceStepID: "tpl-1", Order: 1, Name: "A"},
		{ID: "ps2", Order: 2, Name: "B"},
	}}
	ApplyStepContentCatalog(snap, sampleStepContentCatalog(), time.Date(2026, 10, 20, 3, 0, 0, 0, time.UTC))
	raw, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"content_capture":{"captured_at":"2026-10-20T03:00:00Z"`, `"inherited":{"description"`, `"template_file_id":"wdt-1"`, `"days_before":[5,2]`} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("missing %s in %s", want, raw)
		}
	}
	if strings.Count(string(raw), `"inherited"`) != 1 {
		t.Fatalf("self-added step must omit inherited: %s", raw)
	}
	var got ProposalWorkflowSnapshot
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Steps[0].Inherited == nil || got.Steps[0].Inherited.Documents[1].Name != "Sơ yếu lý lịch" || got.ContentCapture.VersionNo != 4 {
		t.Fatalf("%#v", got)
	}
}

func TestPatchDraftWorkflowInput_IgnoresClientInheritedContent(t *testing.T) {
	var req PatchDraftProposalRequest
	body := `{"workflow_steps":[{"name":"A","processing_days":1,"source_step_id":"tpl-1","inherited":{"description":"forged"}}]}`
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatal(err)
	}
	snap, err := NormalizeProposalWorkflowSteps("type-1", *req.WorkflowSteps, nil, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Steps[0].Inherited != nil || snap.ContentCapture != nil {
		t.Fatalf("draft workflow must never carry client inherited content: %#v", snap.Steps[0])
	}
}

func TestSubmitProposal_AuthDeniedTemplateReadIsUnavailableNotSkipped(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		repo := v3DraftForCapture(ProposalWorkflowStep{ID: "ps1", SourceStepID: "tpl-1", Order: 1, Name: "A", ProcessingDays: 1, DepartmentID: "dep-1", AssigneeMembershipIDs: []string{"m1"}})
		resolver := &spyStepContentResolver{err: perr.NewHTTPError(status, perr.CodePermissionDenied, "access denied", nil)}
		_, err := submitAsCreator(t, newCaptureTestService(repo, captureTestOrg(), resolver))
		he, ok := perr.AsHTTPError(err)
		if !ok || he.HTTPStatus != http.StatusServiceUnavailable || repo.updateCalls != 0 {
			t.Fatalf("status %d: want 503 and no update, got %v (updates=%d)", status, err, repo.updateCalls)
		}
	}
}

func TestNormalizeProposalWorkflowSteps_RejectsOversizedSourceStepID(t *testing.T) {
	_, err := NormalizeProposalWorkflowSteps("type-1", []ProposalWorkflowStepInput{{Name: "A", SourceStepID: strings.Repeat("x", MaxProposalSourceStepIDLength+1)}}, nil, false, nil)
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusBadRequest || he.Details["field"] != "workflow_steps[0].source_step_id" {
		t.Fatalf("want 400 on source_step_id, got %v", err)
	}
	if _, err := NormalizeProposalWorkflowSteps("type-1", []ProposalWorkflowStepInput{{Name: "A", SourceStepID: strings.Repeat("x", MaxProposalSourceStepIDLength)}}, nil, false, nil); err != nil {
		t.Fatalf("max length must pass: %v", err)
	}
}

type listingFakeRepository struct {
	*fakeRepository
	items []ProposalDTO
}

func (f *listingFakeRepository) List(context.Context, string, []string, string, int, int) ([]ProposalDTO, int, error) {
	return f.items, len(f.items), nil
}

func TestListProposals_OmitsInheritedStepContent(t *testing.T) {
	snap := &ProposalWorkflowSnapshot{SchemaVersion: ProposalWorkflowSchemaV3, Frozen: true, Steps: []ProposalWorkflowStep{{ID: "ps1", SourceStepID: "tpl-1", Order: 1, Name: "A"}}}
	ApplyStepContentCatalog(snap, sampleStepContentCatalog(), time.Date(2026, 10, 20, 3, 0, 0, 0, time.UTC))
	repo := &listingFakeRepository{fakeRepository: &fakeRepository{}, items: []ProposalDTO{{ProposalID: "p1", CompanyID: "co", Workflow: snap}}}
	svc := NewService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{}, fakeIDGen{}, false, &fakeAuthService{decision: authapp.DecisionAllow}, newAllowValidator(), nil, noopMetrics{})
	out, err := svc.ListProposals(context.Background(), ListProposalsRequest{Subject: Subject{UserID: "u", MembershipID: "m", CompanyID: "co"}})
	if err != nil {
		t.Fatal(err)
	}
	wf := out.Items[0].Workflow
	if wf.Steps[0].Inherited != nil || wf.ContentCapture == nil {
		t.Fatalf("list must drop step bodies but keep content_capture: %#v", wf)
	}
	if snap.Steps[0].Inherited == nil {
		t.Fatal("list projection must not mutate the stored snapshot")
	}
}

type blockingStepContentResolver struct{ hadDeadline bool }

func (r *blockingStepContentResolver) ResolveStepContent(ctx context.Context, _ Subject, _ string) (*StepContentCatalog, error) {
	_, r.hadDeadline = ctx.Deadline()
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestSubmitProposal_SlowTemplateReadTimesOutAs503(t *testing.T) {
	prev := stepContentResolveTimeout
	stepContentResolveTimeout = 20 * time.Millisecond
	t.Cleanup(func() { stepContentResolveTimeout = prev })

	repo := v3DraftForCapture(ProposalWorkflowStep{ID: "ps1", SourceStepID: "tpl-1", Order: 1, Name: "A", ProcessingDays: 1, DepartmentID: "dep-1", AssigneeMembershipIDs: []string{"m1"}})
	resolver := &blockingStepContentResolver{}
	_, err := submitAsCreator(t, newCaptureTestService(repo, captureTestOrg(), resolver))
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusServiceUnavailable || !resolver.hadDeadline || repo.updateCalls != 0 {
		t.Fatalf("want bounded 503 without update, got %v (deadline=%v updates=%d)", err, resolver.hadDeadline, repo.updateCalls)
	}
}
