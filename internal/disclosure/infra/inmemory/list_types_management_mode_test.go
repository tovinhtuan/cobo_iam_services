package inmemory

import (
	"context"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

func TestListTypes_ManagementModeIncludesDraftOnlyTemplate(t *testing.T) {
	r := NewRepository()
	typeID := "draft-only-template"
	_, err := r.UpsertTypeVersion(context.Background(), disclosureapp.UpsertTypeVersionRequest{
		Subject:          disclosureapp.Subject{CompanyID: "c1", UserID: "u1"},
		TypeID:           typeID,
		Scope:            "global",
		GroupID:          "group-001",
		Name:             "Draft Only",
		TemplateCategory: "periodic",
		ChangeNote:       "initial draft",
	})
	if err != nil {
		t.Fatalf("upsert draft: %v", err)
	}

	portalItems, _, err := r.ListTypes(context.Background(), disclosureapp.ListTypesParams{
		Scope: "global",
	})
	if err != nil {
		t.Fatalf("portal list: %v", err)
	}
	for _, item := range portalItems {
		if item.TypeID == typeID {
			t.Fatal("draft-only template must not appear in portal list")
		}
	}

	mgmtItems, _, err := r.ListTypes(context.Background(), disclosureapp.ListTypesParams{
		Scope:    "global",
		ListMode: "management",
	})
	if err != nil {
		t.Fatalf("management list: %v", err)
	}
	found := false
	for _, item := range mgmtItems {
		if item.TypeID == typeID {
			found = true
			if item.ActiveVersionNo != 0 {
				t.Fatalf("expected active_version_no=0, got %d", item.ActiveVersionNo)
			}
			if item.ListedVersionNo != 1 {
				t.Fatalf("expected listed_version_no=1, got %d", item.ListedVersionNo)
			}
		}
	}
	if !found {
		t.Fatal("draft-only template must appear in CMS management list")
	}
}

func TestListTypes_PortalModeUsesActiveVersionMetadata(t *testing.T) {
	r := NewRepository()
	typeID := "active-and-draft"
	_, err := r.UpsertTypeVersion(context.Background(), disclosureapp.UpsertTypeVersionRequest{
		Subject: disclosureapp.Subject{CompanyID: "c1", UserID: "u1"}, TypeID: typeID, Scope: "global",
		GroupID: "group-001", Name: "V1 Active", TemplateCategory: "periodic", ChangeNote: "v1",
	})
	if err != nil {
		t.Fatalf("upsert v1: %v", err)
	}
	if _, err := r.ActivateTypeVersion(context.Background(), disclosureapp.ActivateTypeVersionRequest{
		Subject: disclosureapp.Subject{CompanyID: "c1", UserID: "u1"}, TypeID: typeID, VersionNo: 1,
	}); err != nil {
		t.Fatalf("activate v1: %v", err)
	}
	_, err = r.UpsertTypeVersion(context.Background(), disclosureapp.UpsertTypeVersionRequest{
		Subject: disclosureapp.Subject{CompanyID: "c1", UserID: "u1"}, TypeID: typeID, Scope: "global",
		GroupID: "group-001", Name: "V2 Draft", TemplateCategory: "periodic", ChangeNote: "v2 draft",
	})
	if err != nil {
		t.Fatalf("upsert v2 draft: %v", err)
	}

	portalItems, _, err := r.ListTypes(context.Background(), disclosureapp.ListTypesParams{Scope: "global"})
	if err != nil {
		t.Fatalf("portal list: %v", err)
	}
	var portalItem *disclosureapp.DisclosureTypeSummaryDTO
	for i := range portalItems {
		if portalItems[i].TypeID == typeID {
			portalItem = &portalItems[i]
			break
		}
	}
	if portalItem == nil {
		t.Fatal("expected portal list item")
	}
	if portalItem.Name != "V1 Active" {
		t.Fatalf("portal must show active v1 metadata, got name=%q", portalItem.Name)
	}
	if portalItem.ActiveVersionNo != 1 || portalItem.ListedVersionNo != 1 {
		t.Fatalf("portal version pointers: active=%d listed=%d", portalItem.ActiveVersionNo, portalItem.ListedVersionNo)
	}
}
