package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func stubAdditionalDocIDs(t *testing.T) {
	t.Helper()
	n := 0
	prev := newProposalAdditionalDocID
	newProposalAdditionalDocID = func() string { n++; return fmt.Sprintf("prop_gen%d", n) }
	t.Cleanup(func() { newProposalAdditionalDocID = prev })
}

func normalizeOneStepAdditions(t *testing.T, add *ProposalStepAdditions) (*ProposalStepAdditions, error) {
	t.Helper()
	snap, err := NormalizeProposalWorkflowSteps("type-1", []ProposalWorkflowStepInput{{Name: "A", ProcessingDays: 1, SourceStepID: "tpl-1", Additions: add}}, nil, false, nil)
	if err != nil {
		return nil, err
	}
	return snap.Steps[0].Additions, nil
}

func wantFieldError(t *testing.T, err error, status int, field string) {
	t.Helper()
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != status || he.Details["field"] != field {
		t.Fatalf("want %d on %s, got %v", status, field, err)
	}
}

func TestNormalizeStepAdditions_TrimsAndKeepsValidIDs(t *testing.T) {
	stubAdditionalDocIDs(t)
	got, err := normalizeOneStepAdditions(t, &ProposalStepAdditions{
		Note: "  Căn cứ nghị quyết 12/10  ",
		Documents: []ProposalStepAdditionalDocument{
			{DocID: "prop_keep-1", Name: "  Nghị quyết HĐQT  "},
			{Name: "Biên bản họp"},
			{DocID: "doc-tpl", Name: "Sai tiền tố"},
			{DocID: "PROP_KEEP-1", Name: "Trùng mã"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Note != "Căn cứ nghị quyết 12/10" || len(got.Documents) != 4 {
		t.Fatalf("additions=%#v", got)
	}
	wantIDs := []string{"prop_keep-1", "prop_gen1", "prop_gen2", "prop_gen3"}
	for i, d := range got.Documents {
		if d.DocID != wantIDs[i] {
			t.Fatalf("doc %d id=%q want %q", i, d.DocID, wantIDs[i])
		}
	}
	if got.Documents[0].Name != "Nghị quyết HĐQT" {
		t.Fatalf("name not trimmed: %q", got.Documents[0].Name)
	}
}

func TestNormalizeStepAdditions_NilStaysNilEmptyStaysPresent(t *testing.T) {
	if got, err := normalizeOneStepAdditions(t, nil); err != nil || got != nil {
		t.Fatalf("nil additions: got %#v err %v", got, err)
	}
	got, err := normalizeOneStepAdditions(t, &ProposalStepAdditions{Note: "   "})
	if err != nil || got == nil || got.Note != "" || got.Documents != nil {
		t.Fatalf("empty additions must stay present: got %#v err %v", got, err)
	}
	raw, _ := json.Marshal(ProposalWorkflowStep{ID: "ps1", Additions: got})
	if !strings.Contains(string(raw), `"additions":{}`) {
		t.Fatalf("empty additions must serialize as {} (submit marker): %s", raw)
	}
}

func TestNormalizeStepAdditions_Validation(t *testing.T) {
	tooMany := make([]ProposalStepAdditionalDocument, MaxProposalStepAdditionalDocuments+1)
	for i := range tooMany {
		tooMany[i] = ProposalStepAdditionalDocument{Name: fmt.Sprintf("Tài liệu %d", i)}
	}
	cases := []struct {
		name  string
		add   *ProposalStepAdditions
		field string
	}{
		{"note too long", &ProposalStepAdditions{Note: strings.Repeat("ạ", MaxProposalStepNoteLength+1)}, "workflow_steps[0].additions.note"},
		{"too many documents", &ProposalStepAdditions{Documents: tooMany}, "workflow_steps[0].additions.documents"},
		{"blank name", &ProposalStepAdditions{Documents: []ProposalStepAdditionalDocument{{Name: "A"}, {Name: "  "}}}, "workflow_steps[0].additions.documents[1].name"},
		{"name too long", &ProposalStepAdditions{Documents: []ProposalStepAdditionalDocument{{Name: strings.Repeat("ạ", MaxProposalStepDocumentNameLength+1)}}}, "workflow_steps[0].additions.documents[0].name"},
		{"duplicate name ci", &ProposalStepAdditions{Documents: []ProposalStepAdditionalDocument{{Name: "Biên bản"}, {Name: " BIÊN BẢN "}}}, "workflow_steps[0].additions.documents[1].name"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := normalizeOneStepAdditions(t, tc.add)
			wantFieldError(t, err, http.StatusBadRequest, tc.field)
		})
	}
	// Limits are inclusive.
	if _, err := normalizeOneStepAdditions(t, &ProposalStepAdditions{
		Note:      strings.Repeat("ạ", MaxProposalStepNoteLength),
		Documents: []ProposalStepAdditionalDocument{{Name: strings.Repeat("ạ", MaxProposalStepDocumentNameLength)}},
	}); err != nil {
		t.Fatalf("max lengths must pass: %v", err)
	}
}

func TestPatchDraftWorkflowInput_AdditionsIgnoreTemplateFileAndRequiredFields(t *testing.T) {
	var req PatchDraftProposalRequest
	body := `{"workflow_steps":[{"name":"A","processing_days":1,"additions":{"note":"n","documents":[{"doc_id":"prop_x","name":"D","required":false,"template_file_id":"wdt-other","template_file_name":"forged.docx"}]}}]}`
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatal(err)
	}
	snap, err := NormalizeProposalWorkflowSteps("type-1", *req.WorkflowSteps, nil, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(snap.Steps[0])
	if !strings.Contains(string(raw), `"additions":{"note":"n","documents":[{"doc_id":"prop_x","name":"D"}]}`) ||
		strings.Contains(string(raw), "wdt-other") || strings.Contains(string(raw), "forged") {
		t.Fatalf("additions must keep only note + doc id/name: %s", raw)
	}
}

func TestValidateStepAdditionsForSubmit(t *testing.T) {
	snap := &ProposalWorkflowSnapshot{Steps: []ProposalWorkflowStep{
		{ID: "ps1", SourceStepID: "tpl-1", Additions: &ProposalStepAdditions{}}, // template step: note optional
		{ID: "ps2"}, // older client: not checked
		{ID: "ps3", Additions: &ProposalStepAdditions{Note: "Mô tả"}},
		{ID: "ps4", Additions: &ProposalStepAdditions{Note: "  "}},
	}}
	err := ValidateStepAdditionsForSubmit(snap)
	wantFieldError(t, err, http.StatusUnprocessableEntity, "workflow_steps[3].additions.note")
	if !strings.Contains(err.Error(), "description_required") {
		t.Fatalf("message: %v", err)
	}
	snap.Steps[3].Additions.Note = "Đối chiếu"
	if err := ValidateStepAdditionsForSubmit(snap); err != nil {
		t.Fatal(err)
	}
}

func TestSubmitProposal_SelfAddedStepWithoutDescriptionIs422(t *testing.T) {
	repo := v3DraftForCapture(
		ProposalWorkflowStep{ID: "ps1", SourceStepID: "tpl-1", Order: 1, Name: "A", ProcessingDays: 1, DepartmentID: "dep-1", AssigneeMembershipIDs: []string{"m1"}, Additions: &ProposalStepAdditions{}},
		ProposalWorkflowStep{ID: "ps2", Order: 2, Name: "Tự thêm", ProcessingDays: 1, DepartmentID: "dep-1", AssigneeMembershipIDs: []string{"m1"}, Additions: &ProposalStepAdditions{}},
	)
	resolver := &spyStepContentResolver{catalog: sampleStepContentCatalog()}
	_, err := submitAsCreator(t, newCaptureTestService(repo, captureTestOrg(), resolver))
	wantFieldError(t, err, http.StatusUnprocessableEntity, "workflow_steps[1].additions.note")
	if repo.updateCalls != 0 || resolver.calls != 0 {
		t.Fatalf("must fail before capture/update: updates=%d resolver=%d", repo.updateCalls, resolver.calls)
	}
}

func TestSubmitProposal_KeepsAdditionsAndCapturesTemplate(t *testing.T) {
	add := &ProposalStepAdditions{Note: "Căn cứ nghị quyết 12/10", Documents: []ProposalStepAdditionalDocument{{DocID: "prop_a", Name: "Nghị quyết HĐQT 12/10"}}}
	repo := v3DraftForCapture(
		ProposalWorkflowStep{ID: "ps1", SourceStepID: "tpl-1", Order: 1, Name: "A", ProcessingDays: 1, DepartmentID: "dep-1", AssigneeMembershipIDs: []string{"m1"}, Additions: add},
		ProposalWorkflowStep{ID: "ps2", Order: 2, Name: "Tự thêm", ProcessingDays: 1, DepartmentID: "dep-1", AssigneeMembershipIDs: []string{"m1"},
			Additions: &ProposalStepAdditions{Note: "Đối chiếu số liệu", Documents: []ProposalStepAdditionalDocument{{DocID: "prop_b", Name: "Biên bản"}}}},
	)
	if _, err := submitAsCreator(t, newCaptureTestService(repo, captureTestOrg(), &spyStepContentResolver{catalog: sampleStepContentCatalog()})); err != nil {
		t.Fatal(err)
	}
	wf := repo.lastUpdate.Workflow
	if wf.Steps[0].Inherited == nil || wf.Steps[0].Additions == nil || wf.Steps[0].Additions.Note != add.Note || len(wf.Steps[0].Additions.Documents) != 1 {
		t.Fatalf("step 1 must keep additions and capture the template: %#v", wf.Steps[0])
	}
	if wf.Steps[1].Additions == nil || wf.Steps[1].Additions.Note != "Đối chiếu số liệu" {
		t.Fatalf("self-added step additions lost: %#v", wf.Steps[1])
	}
}

func TestSubmitProposal_OlderClientSelfAddedStepWithoutAdditionsStillSubmits(t *testing.T) {
	repo := v3DraftForCapture(ProposalWorkflowStep{ID: "ps1", Order: 1, Name: "Tự thêm", ProcessingDays: 1, DepartmentID: "dep-1", AssigneeMembershipIDs: []string{"m1"}})
	if _, err := submitAsCreator(t, newCaptureTestService(repo, captureTestOrg(), nil)); err != nil {
		t.Fatalf("drafts without additions keep the previous submit behaviour: %v", err)
	}
}

func TestListProposals_OmitsStepAdditions(t *testing.T) {
	snap := &ProposalWorkflowSnapshot{SchemaVersion: ProposalWorkflowSchemaV3, Steps: []ProposalWorkflowStep{
		{ID: "ps1", Order: 1, Name: "A", Additions: &ProposalStepAdditions{Note: "n", Documents: []ProposalStepAdditionalDocument{{DocID: "prop_a", Name: "D"}}}},
	}}
	repo := &listingFakeRepository{fakeRepository: &fakeRepository{}, items: []ProposalDTO{{ProposalID: "p1", CompanyID: "co", Workflow: snap}}}
	svc := NewService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{}, fakeIDGen{}, false, &fakeAuthService{decision: authapp.DecisionAllow}, newAllowValidator(), nil, noopMetrics{})
	out, err := svc.ListProposals(context.Background(), ListProposalsRequest{Subject: Subject{UserID: "u", MembershipID: "m", CompanyID: "co"}})
	if err != nil {
		t.Fatal(err)
	}
	if out.Items[0].Workflow.Steps[0].Additions != nil || snap.Steps[0].Additions == nil {
		t.Fatalf("list must drop additions without mutating the stored snapshot: %#v", out.Items[0].Workflow.Steps[0])
	}
}

func TestNormalizeStepAdditions_RejectsInvisibleCharactersAndNormalizesNFC(t *testing.T) {
	for _, tc := range []struct {
		name  string
		add   *ProposalStepAdditions
		field string
	}{
		{"zero width in name", &ProposalStepAdditions{Documents: []ProposalStepAdditionalDocument{{Name: "Quyết​định"}}}, "workflow_steps[0].additions.documents[0].name"},
		{"bidi override in name", &ProposalStepAdditions{Documents: []ProposalStepAdditionalDocument{{Name: "‮eod.exe"}}}, "workflow_steps[0].additions.documents[0].name"},
		{"newline in name", &ProposalStepAdditions{Documents: []ProposalStepAdditionalDocument{{Name: "a\nb"}}}, "workflow_steps[0].additions.documents[0].name"},
		{"control char in note", &ProposalStepAdditions{Note: "ghi\x07chú"}, "workflow_steps[0].additions.note"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := normalizeOneStepAdditions(t, tc.add)
			wantFieldError(t, err, http.StatusBadRequest, tc.field)
		})
	}
	if got, err := normalizeOneStepAdditions(t, &ProposalStepAdditions{Note: "dòng 1\n\tdòng 2"}); err != nil || got.Note != "dòng 1\n\tdòng 2" {
		t.Fatalf("newlines/tabs are allowed in notes: %#v %v", got, err)
	}
	// "Biên bản" typed decomposed (NFD) duplicates the precomposed name.
	_, err := normalizeOneStepAdditions(t, &ProposalStepAdditions{Documents: []ProposalStepAdditionalDocument{{Name: "Biên bản"}, {Name: "Biên bản"}}})
	wantFieldError(t, err, http.StatusBadRequest, "workflow_steps[0].additions.documents[1].name")
}

func TestNormalizeProposalWorkflowSteps_AdditionsTotalCaps(t *testing.T) {
	steps := make([]ProposalWorkflowStepInput, 11)
	for i := range steps {
		steps[i] = ProposalWorkflowStepInput{Name: fmt.Sprintf("B%d", i), Additions: &ProposalStepAdditions{Note: strings.Repeat("ạ", MaxProposalStepNoteLength)}}
	}
	_, err := NormalizeProposalWorkflowSteps("type-1", steps, nil, false, nil)
	wantFieldError(t, err, http.StatusBadRequest, "workflow_steps")
	if !strings.Contains(err.Error(), "additions_total_exceeded") {
		t.Fatalf("message: %v", err)
	}
	if _, err := NormalizeProposalWorkflowSteps("type-1", steps[:10], nil, false, nil); err != nil {
		t.Fatalf("20000 characters in total must pass: %v", err)
	}
	docs := func(n int) []ProposalStepAdditionalDocument {
		out := make([]ProposalStepAdditionalDocument, n)
		for i := range out {
			out[i] = ProposalStepAdditionalDocument{Name: fmt.Sprintf("Tài liệu %d", i)}
		}
		return out
	}
	many := make([]ProposalWorkflowStepInput, 6)
	for i := range many {
		many[i] = ProposalWorkflowStepInput{Name: fmt.Sprintf("B%d", i), Additions: &ProposalStepAdditions{Documents: docs(MaxProposalStepAdditionalDocuments)}}
	}
	_, err = NormalizeProposalWorkflowSteps("type-1", many, nil, false, nil)
	wantFieldError(t, err, http.StatusBadRequest, "workflow_steps")
	if _, err := NormalizeProposalWorkflowSteps("type-1", many[:5], nil, false, nil); err != nil {
		t.Fatalf("100 documents in total must pass: %v", err)
	}
}

func TestPatchDraftProposal_OlderClientKeepsSavedAdditions(t *testing.T) {
	saved := &ProposalStepAdditions{Note: "Căn cứ NQ 12/10", Documents: []ProposalStepAdditionalDocument{{DocID: "prop_a", Name: "Nghị quyết"}}}
	newRepo := func() *fakeRepository {
		return &fakeRepository{proposal: &ProposalDTO{
			ProposalID: "p1", CompanyID: "co", TypeID: "type-1", Status: StatusDraft, CreatedBy: "creator",
			Workflow: &ProposalWorkflowSnapshot{SchemaVersion: 2, DisclosureTypeID: "type-1", Steps: []ProposalWorkflowStep{
				{ID: "ps-1", SourceStepID: "tpl-1", Order: 1, Name: "One", ProcessingDays: 1, Additions: saved},
				{ID: "ps-2", Order: 2, Name: "Two", ProcessingDays: 1, Additions: &ProposalStepAdditions{Note: "Mô tả"}},
			}},
		}}
	}
	patch := func(repo *fakeRepository, steps []ProposalWorkflowStepInput) *ProposalDTO {
		t.Helper()
		svc := NewService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, &fakeAuthService{decision: authapp.DecisionAllow}, newAllowValidator(), nil, noopMetrics{})
		out, err := svc.PatchDraftProposal(context.Background(), PatchDraftProposalRequest{
			Subject: Subject{UserID: "u", MembershipID: "creator", CompanyID: "co"}, ProposalID: "p1", WorkflowSteps: &steps,
		})
		if err != nil {
			t.Fatal(err)
		}
		return out
	}

	// Older client (no additions field): saved additions survive, new steps get none.
	out := patch(newRepo(), []ProposalWorkflowStepInput{
		{ID: "ps-2", Name: "Two", ProcessingDays: 2},
		{ID: "ps-1", SourceStepID: "tpl-1", Name: "One", ProcessingDays: 3},
		{Name: "Three", ProcessingDays: 1},
	})
	if got := out.Workflow.Steps[1].Additions; got == nil || got.Note != saved.Note || len(got.Documents) != 1 {
		t.Fatalf("step ps-1 lost its additions: %#v", got)
	}
	if got := out.Workflow.Steps[0].Additions; got == nil || got.Note != "Mô tả" {
		t.Fatalf("step ps-2 lost its additions: %#v", got)
	}
	if out.Workflow.Steps[2].Additions != nil {
		t.Fatalf("new step must not inherit additions: %#v", out.Workflow.Steps[2].Additions)
	}

	// Additions-aware client: an explicit empty object clears them.
	out = patch(newRepo(), []ProposalWorkflowStepInput{
		{ID: "ps-1", SourceStepID: "tpl-1", Name: "One", ProcessingDays: 1, Additions: &ProposalStepAdditions{}},
		{ID: "ps-2", Name: "Two", ProcessingDays: 1, Additions: &ProposalStepAdditions{Note: "Mới"}},
	})
	if got := out.Workflow.Steps[0].Additions; got == nil || got.Note != "" || len(got.Documents) != 0 {
		t.Fatalf("explicit empty additions must clear: %#v", got)
	}
	if out.Workflow.Steps[1].Additions.Note != "Mới" {
		t.Fatalf("explicit additions must replace: %#v", out.Workflow.Steps[1].Additions)
	}
}
