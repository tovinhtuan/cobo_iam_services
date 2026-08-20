package inmemory_test

import (
	"context"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/infra/inmemory"
)

func testSubject() disclosureapp.Subject {
	return disclosureapp.Subject{UserID: "u1", CompanyID: "c1"}
}

func baseUpsert(typeID string) disclosureapp.UpsertTypeVersionRequest {
	return disclosureapp.UpsertTypeVersionRequest{
		Subject:           testSubject(),
		TypeID:            typeID,
		Scope:             "global",
		GroupID:           "group-001",
		Name:              "QA Template",
		Category:          "periodic",
		TemplateCategory:  "periodic",
		DeadlineStrategy:  "fixed",
		Description:       "desc",
		DeadlineRule:      "T+5",
		DisplayGroupCodes: []string{"dg-periodic"},
		ChangeNote:        "save",
		Blocks: []disclosureapp.TemplateBlockDTO{
			{
				BlockID:   "b1",
				BlockKey:  "enterprise_workflow",
				BlockType: "workflow",
				Title:     "WF",
				Enabled:   true,
				Config: map[string]any{
					"steps": []any{
						map[string]any{
							"step_key":      "s1",
							"department_id": "d1",
							"role_id":       "r1",
							"sla_days":      1,
						},
					},
				},
			},
		},
	}
}

func TestUpsertTypeVersion_FirstSaveDoesNotActivate(t *testing.T) {
	repo := inmemory.NewRepository()
	req := baseUpsert("type-draft-first")
	req.DisplayGroupCodes = nil

	first, err := repo.UpsertTypeVersion(context.Background(), req)
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if first.IsActive || first.VersionNo != 1 {
		t.Fatalf("first save must stay draft v1 (not portal-active), got v%d active=%v", first.VersionNo, first.IsActive)
	}

	versions, err := repo.ListTypeVersions(context.Background(), "c1", "type-draft-first")
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	for _, v := range versions {
		if v.IsActive {
			t.Fatalf("no version should be portal-active after first save, got v%d", v.VersionNo)
		}
	}
}

func TestUpsertTypeVersion_OverwritesOpenDraftWithoutBump(t *testing.T) {
	repo := inmemory.NewRepository()
	req := baseUpsert("type-draft-1")
	req.DisplayGroupCodes = nil

	first, err := repo.UpsertTypeVersion(context.Background(), req)
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if first.IsActive || first.VersionNo != 1 {
		t.Fatalf("first save must stay draft v1, got v%d active=%v", first.VersionNo, first.IsActive)
	}

	req.ChangeNote = "draft-1"
	req.Name = "Draft A"
	draft1, err := repo.UpsertTypeVersion(context.Background(), req)
	if err != nil {
		t.Fatalf("overwrite draft: %v", err)
	}
	if draft1.IsActive || draft1.VersionNo != 1 {
		t.Fatalf("expected overwrite draft v1, got v%d active=%v", draft1.VersionNo, draft1.IsActive)
	}

	_, err = repo.ActivateTypeVersion(context.Background(), disclosureapp.ActivateTypeVersionRequest{
		Subject: testSubject(), TypeID: "type-draft-1", VersionNo: 1,
	})
	if err != nil {
		t.Fatalf("activate v1: %v", err)
	}

	req.ChangeNote = "draft-2"
	req.Name = "Draft B"
	draft2, err := repo.UpsertTypeVersion(context.Background(), req)
	if err != nil {
		t.Fatalf("create draft after active: %v", err)
	}
	if draft2.IsActive || draft2.VersionNo != 2 {
		t.Fatalf("expected inactive draft v2, got v%d active=%v", draft2.VersionNo, draft2.IsActive)
	}

	req.ChangeNote = "draft-2-overwrite"
	req.Name = "Draft B2"
	draft3, err := repo.UpsertTypeVersion(context.Background(), req)
	if err != nil {
		t.Fatalf("overwrite draft v2: %v", err)
	}
	if draft3.VersionNo != 2 {
		t.Fatalf("overwrite must keep version_no=2, got %d", draft3.VersionNo)
	}
	if draft3.IsActive {
		t.Fatal("overwrite must not activate draft")
	}

	versions, err := repo.ListTypeVersions(context.Background(), "c1", "type-draft-1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	draftCount := 0
	for _, v := range versions {
		if !v.IsActive && !v.IsReleased {
			draftCount++
		}
	}
	if draftCount != 1 {
		t.Fatalf("list should expose single open draft, got draftCount=%d versions=%+v", draftCount, versions)
	}
	detail, err := repo.GetTypeVersionDetail(context.Background(), "c1", "type-draft-1", 2)
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if detail.Name != "Draft B2" {
		t.Fatalf("draft content not overwritten, name=%q", detail.Name)
	}

	active, err := repo.GetTypeVersionDetail(context.Background(), "c1", "type-draft-1", 1)
	if err != nil {
		t.Fatalf("active detail: %v", err)
	}
	if active.Name != "Draft A" {
		t.Fatalf("active content changed unexpectedly: %q", active.Name)
	}
}

func TestUpsertTypeVersion_ActivateMarksReleasedAndNewDraftAfter(t *testing.T) {
	repo := inmemory.NewRepository()
	req := baseUpsert("type-draft-2")
	req.DisplayGroupCodes = nil
	if _, err := repo.UpsertTypeVersion(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	act, err := repo.ActivateTypeVersion(context.Background(), disclosureapp.ActivateTypeVersionRequest{
		Subject: testSubject(), TypeID: "type-draft-2", VersionNo: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !act.IsActive {
		t.Fatal("activate should set active")
	}
	req.Name = "Draft"
	d, err := repo.UpsertTypeVersion(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if d.IsActive || d.VersionNo != 2 {
		t.Fatalf("after activate v1, save should create draft v2, got v%d active=%v", d.VersionNo, d.IsActive)
	}
	versions, err := repo.ListTypeVersions(context.Background(), "c1", "type-draft-2")
	if err != nil {
		t.Fatal(err)
	}
	foundReleased := false
	for _, v := range versions {
		if v.VersionNo == 1 && v.IsReleased && v.IsActive {
			foundReleased = true
		}
	}
	if !foundReleased {
		t.Fatalf("activated version should be released+active: %+v", versions)
	}

	req.Name = "Post-activate draft"
	d2, err := repo.UpsertTypeVersion(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if d2.VersionNo != 2 {
		t.Fatalf("after activate, next save should overwrite open draft v2, got %d", d2.VersionNo)
	}
}
