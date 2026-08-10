package app

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func TestNormalizeProposalWorkflowSteps_stableOrderAndIDs(t *testing.T) {
	seq := 0
	newID := func() string {
		seq++
		return "id-" + string(rune('A'+seq-1))
	}
	snap, err := NormalizeProposalWorkflowSteps("type-1", []ProposalWorkflowStepInput{
		{Name: " B ", ProcessingDays: 2, SourceStepID: "tpl-1"},
		{Name: "A", ProcessingDays: 0},
	}, nil, false, newID)
	if err != nil {
		t.Fatal(err)
	}
	if snap.SchemaVersion != ProposalWorkflowSchemaV2 || snap.Frozen || len(snap.Steps) != 2 {
		t.Fatalf("%#v", snap)
	}
	if snap.Steps[0].Order != 1 || snap.Steps[0].Name != "B" || snap.Steps[0].ID != "id-A" {
		t.Fatalf("step0 %#v", snap.Steps[0])
	}
	if snap.Steps[1].Order != 2 || snap.Steps[1].SourceStepID != "" {
		t.Fatalf("step1 %#v", snap.Steps[1])
	}
}

func TestNormalizeProposalWorkflowSteps_preservesExistingIDs(t *testing.T) {
	existing := map[string]struct{}{"keep-1": {}}
	seq := 0
	newID := func() string {
		seq++
		return "new-" + string(rune('0'+seq))
	}
	snap, err := NormalizeProposalWorkflowSteps("t", []ProposalWorkflowStepInput{
		{ID: "keep-1", Name: "Same", ProcessingDays: 1},
		{ID: "client-forged", Name: "New", ProcessingDays: 1},
	}, existing, false, newID)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Steps[0].ID != "keep-1" {
		t.Fatalf("expected keep-1, got %s", snap.Steps[0].ID)
	}
	if snap.Steps[1].ID == "client-forged" || snap.Steps[1].ID == "" {
		t.Fatalf("forged id must be replaced, got %s", snap.Steps[1].ID)
	}
}

func TestNormalizeProposalWorkflowSteps_assigneeRequiresDepartment(t *testing.T) {
	_, err := NormalizeProposalWorkflowSteps("t", []ProposalWorkflowStepInput{
		{Name: "X", ProcessingDays: 1, AssigneeMembershipID: "m1"},
	}, nil, false, func() string { return "s1" })
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNormalizeProposalWorkflowSteps_limits(t *testing.T) {
	_, err := NormalizeProposalWorkflowSteps("t", nil, nil, false, nil)
	if err == nil {
		t.Fatal("expected min error")
	}
	inputs := make([]ProposalWorkflowStepInput, MaxProposalWorkflowSteps+1)
	for i := range inputs {
		inputs[i] = ProposalWorkflowStepInput{Name: "s", ProcessingDays: 1}
	}
	_, err = NormalizeProposalWorkflowSteps("t", inputs, nil, false, func() string { return "x" })
	if err == nil {
		t.Fatal("expected max error")
	}
}

func TestProposalWorkflowSnapshot_JSONRoundTrip(t *testing.T) {
	snap := &ProposalWorkflowSnapshot{
		SchemaVersion:    2,
		DisclosureTypeID: "t1",
		Frozen:           false,
		Steps: []ProposalWorkflowStep{{
			ID: "ps-1", SourceStepID: "tpl", Order: 1, Name: "Rà soát", ProcessingDays: 2,
			DepartmentID: "d1", AssigneeMembershipID: "m1",
		}},
	}
	raw, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	var got ProposalWorkflowSnapshot
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != 2 || got.Steps[0].AssigneeMembershipID != "m1" {
		t.Fatalf("%#v", got)
	}
	if ResolveProposalWorkflowContractVersion(&got, nil) != ProposalWorkflowSchemaV2 {
		t.Fatal("version")
	}
	if ResolveProposalWorkflowContractVersion(nil, []WorkflowStepOverride{{StepID: "s"}}) != 1 {
		t.Fatal("legacy version")
	}
}

type fakeOrgDirectory struct {
	depts   map[string]bool
	members map[string]bool
	belong  map[string]bool   // membership\x00dept
	heads   map[string]string // departmentID → head membershipID
	headErr map[string]error  // departmentID → error
}

func (f *fakeOrgDirectory) IsActiveDepartmentInCompany(_ context.Context, _, departmentID string) (bool, error) {
	return f.depts[departmentID], nil
}
func (f *fakeOrgDirectory) IsActiveMembershipInCompany(_ context.Context, _, membershipID string) (bool, error) {
	return f.members[membershipID], nil
}
func (f *fakeOrgDirectory) MemberBelongsToDepartment(_ context.Context, membershipID, departmentID string) (bool, error) {
	return f.belong[membershipID+"\x00"+departmentID], nil
}
func (f *fakeOrgDirectory) ResolveDepartmentHeadMembership(_ context.Context, _, departmentID string) (string, error) {
	if f.headErr != nil {
		if err, ok := f.headErr[departmentID]; ok {
			return "", err
		}
	}
	if f.heads == nil {
		he := perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest,
			"department_head_not_configured: department has no head_membership_id", nil)
		he.Details = map[string]any{"code": "department_head_not_configured"}
		return "", he
	}
	head, ok := f.heads[departmentID]
	if !ok || strings.TrimSpace(head) == "" {
		he := perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest,
			"department_head_not_configured: department has no head_membership_id", nil)
		he.Details = map[string]any{"code": "department_head_not_configured"}
		return "", he
	}
	return head, nil
}

func TestValidateWorkflowStepOrgRefs_tenantIsolation(t *testing.T) {
	org := &fakeOrgDirectory{
		depts:   map[string]bool{"dep-ok": true},
		members: map[string]bool{"mem-ok": true},
		belong:  map[string]bool{"mem-ok\x00dep-ok": true},
	}
	steps := []ProposalWorkflowStep{{
		ID: "1", Order: 1, Name: "A", ProcessingDays: 1,
		DepartmentID: "dep-other", AssigneeMembershipID: "mem-ok",
	}}
	err := ValidateWorkflowStepOrgRefs(context.Background(), org, "co", steps)
	if err == nil {
		t.Fatal("expected reject")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusUnprocessableEntity {
		t.Fatalf("%v", err)
	}

	steps[0].DepartmentID = "dep-ok"
	steps[0].AssigneeMembershipID = "mem-ok"
	if err := ValidateWorkflowStepOrgRefs(context.Background(), org, "co", steps); err != nil {
		t.Fatal(err)
	}

	steps[0].AssigneeMembershipID = "mem-other"
	org.members["mem-other"] = false
	if err := ValidateWorkflowStepOrgRefs(context.Background(), org, "co", steps); err == nil {
		t.Fatal("expected membership reject")
	}
}
