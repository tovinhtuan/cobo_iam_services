package inmemory

import (
	"context"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

func listMgmtWorkflow(t *testing.T, r *Repository, hasWorkflow *bool) map[string]disclosureapp.DisclosureTypeSummaryDTO {
	t.Helper()
	items, _, err := r.ListTypes(context.Background(), disclosureapp.ListTypesParams{
		Scope: "global", ListMode: "management", PortalState: disclosureapp.PortalStateActive, HasWorkflow: hasWorkflow,
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	out := map[string]disclosureapp.DisclosureTypeSummaryDTO{}
	for _, item := range items {
		out[item.TypeID] = item
	}
	return out
}

func TestListTypes_Management_OpenDraftVersionNo(t *testing.T) {
	r := NewRepository()
	seedGlobalDraft(t, r, "dt-active-draft", "Active+Draft")
	activateVersion(t, r, "dt-active-draft", 1)
	seedGlobalDraft(t, r, "dt-active-draft", "V2 Draft")
	seedGlobalDraft(t, r, "dt-active-clean", "Active Clean")
	activateVersion(t, r, "dt-active-clean", 1)

	items := listMgmtWorkflow(t, r, nil)
	if got := items["dt-active-draft"].OpenDraftVersionNo; got != 2 {
		t.Fatalf("open draft version = %d, want 2", got)
	}
	if got := items["dt-active-clean"].OpenDraftVersionNo; got != 0 {
		t.Fatalf("clean template open draft version = %d, want 0", got)
	}

	// The portal list (non-management) never carries the CMS-only field.
	portal, _, err := r.ListTypes(context.Background(), disclosureapp.ListTypesParams{Scope: "global"})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range portal {
		if item.OpenDraftVersionNo != 0 {
			t.Fatalf("portal list leaked open_draft_version_no for %s", item.TypeID)
		}
	}
}

func TestListTypes_HasWorkflowFilter(t *testing.T) {
	r := NewRepository()
	seedGlobalDraft(t, r, "dt-no-workflow", "No workflow")
	activateVersion(t, r, "dt-no-workflow", 1)

	no := false
	yes := true
	if _, ok := listMgmtWorkflow(t, r, &no)["dt-no-workflow"]; !ok {
		t.Fatal("has_workflow=false must include an active template without workflow")
	}
	if _, ok := listMgmtWorkflow(t, r, &yes)["dt-no-workflow"]; ok {
		t.Fatal("has_workflow=true must exclude an active template without workflow")
	}
}
