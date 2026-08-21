package inmemory

import (
	"context"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

func seedGlobalDraft(t *testing.T, r *Repository, typeID, name string) {
	t.Helper()
	_, err := r.UpsertTypeVersion(context.Background(), disclosureapp.UpsertTypeVersionRequest{
		Subject:          disclosureapp.Subject{CompanyID: "c1", UserID: "u1"},
		TypeID:           typeID,
		Scope:            "global",
		GroupID:          "group-001",
		Name:             name,
		TemplateCategory: "periodic",
		ChangeNote:       "seed",
	})
	if err != nil {
		t.Fatalf("upsert %s: %v", typeID, err)
	}
}

func activateVersion(t *testing.T, r *Repository, typeID string, versionNo int) {
	t.Helper()
	if _, err := r.ActivateTypeVersion(context.Background(), disclosureapp.ActivateTypeVersionRequest{
		Subject: disclosureapp.Subject{CompanyID: "c1", UserID: "u1"}, TypeID: typeID, VersionNo: versionNo,
	}); err != nil {
		t.Fatalf("activate %s v%d: %v", typeID, versionNo, err)
	}
}

func typeIDs(items []disclosureapp.DisclosureTypeSummaryDTO) map[string]bool {
	out := map[string]bool{}
	for _, item := range items {
		out[item.TypeID] = true
	}
	return out
}

func listMgmt(t *testing.T, r *Repository, portalState string, hasOpenDraft *bool) []disclosureapp.DisclosureTypeSummaryDTO {
	t.Helper()
	items, _, err := r.ListTypes(context.Background(), disclosureapp.ListTypesParams{
		Scope: "global", ListMode: "management", PortalState: portalState, HasOpenDraft: hasOpenDraft,
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	return items
}

func TestListTypes_PortalState_ActiveNotActiveArchived(t *testing.T) {
	r := NewRepository()
	seedGlobalDraft(t, r, "dt-portal-active", "Active One")
	activateVersion(t, r, "dt-portal-active", 1)

	seedGlobalDraft(t, r, "dt-not-on-portal", "Draft Only")

	seedGlobalDraft(t, r, "dt-archived", "Archived One")
	activateVersion(t, r, "dt-archived", 1)
	if err := r.ArchiveGlobalTemplate(context.Background(), "dt-archived", "u1"); err != nil {
		t.Fatal(err)
	}

	active := typeIDs(listMgmt(t, r, disclosureapp.PortalStateActive, nil))
	if !active["dt-portal-active"] || active["dt-not-on-portal"] || active["dt-archived"] {
		t.Fatalf("active filter=%v", active)
	}

	notActive := typeIDs(listMgmt(t, r, disclosureapp.PortalStateNotActive, nil))
	if !notActive["dt-not-on-portal"] || notActive["dt-portal-active"] || notActive["dt-archived"] {
		t.Fatalf("not_active must exclude archived; got=%v", notActive)
	}

	archived := typeIDs(listMgmt(t, r, disclosureapp.PortalStateArchived, nil))
	if !archived["dt-archived"] || archived["dt-portal-active"] || archived["dt-not-on-portal"] {
		t.Fatalf("archived filter=%v", archived)
	}

	all := typeIDs(listMgmt(t, r, disclosureapp.PortalStateAll, nil))
	if !all["dt-portal-active"] || !all["dt-not-on-portal"] || !all["dt-archived"] {
		t.Fatalf("all filter=%v", all)
	}
}

func TestListTypes_PortalState_ActiveWithOpenDraft(t *testing.T) {
	r := NewRepository()
	seedGlobalDraft(t, r, "dt-active-draft", "Active+Draft")
	activateVersion(t, r, "dt-active-draft", 1)
	seedGlobalDraft(t, r, "dt-active-draft", "V2 Draft") // overwrite name on open draft / new version

	seedGlobalDraft(t, r, "dt-active-clean", "Active Clean")
	activateVersion(t, r, "dt-active-clean", 1)

	yes := true
	withDraft := typeIDs(listMgmt(t, r, disclosureapp.PortalStateActive, &yes))
	if !withDraft["dt-active-draft"] || withDraft["dt-active-clean"] {
		t.Fatalf("active+has_open_draft=%v", withDraft)
	}

	activeOnly := typeIDs(listMgmt(t, r, disclosureapp.PortalStateActive, nil))
	if !activeOnly["dt-active-draft"] || !activeOnly["dt-active-clean"] {
		t.Fatalf("active without draft filter=%v", activeOnly)
	}

	notActive := typeIDs(listMgmt(t, r, disclosureapp.PortalStateNotActive, nil))
	if notActive["dt-active-draft"] {
		t.Fatal("active+draft must not appear in not_active")
	}
}

func TestListTypes_PortalState_NotActiveWithOpenDraft(t *testing.T) {
	r := NewRepository()
	seedGlobalDraft(t, r, "dt-clone-like", "Clone Target")
	yes := true
	items := typeIDs(listMgmt(t, r, disclosureapp.PortalStateNotActive, &yes))
	if !items["dt-clone-like"] {
		t.Fatalf("not_active+has_open_draft missing clone-like: %v", items)
	}
	active := typeIDs(listMgmt(t, r, disclosureapp.PortalStateActive, nil))
	if active["dt-clone-like"] {
		t.Fatal("clone-like must not be portal-active")
	}
}

func TestListTypes_PortalState_SearchComposition(t *testing.T) {
	r := NewRepository()
	seedGlobalDraft(t, r, "dt-fin-a", "Financial Report")
	activateVersion(t, r, "dt-fin-a", 1)
	seedGlobalDraft(t, r, "dt-fin-b", "Other Report")
	activateVersion(t, r, "dt-fin-b", 1)

	items, _, err := r.ListTypes(context.Background(), disclosureapp.ListTypesParams{
		Scope: "global", ListMode: "management",
		PortalState: disclosureapp.PortalStateActive,
		Query:       "Financial",
	})
	if err != nil {
		t.Fatal(err)
	}
	ids := typeIDs(items)
	if !ids["dt-fin-a"] || ids["dt-fin-b"] {
		t.Fatalf("search+active=%v", ids)
	}
}
