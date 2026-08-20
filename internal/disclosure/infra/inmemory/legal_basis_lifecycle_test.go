package inmemory_test

import (
	"context"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/infra/inmemory"
)

func TestLegalBases_SameDraftPreserveIDsAndNewVersionRegenerates(t *testing.T) {
	repo := inmemory.NewRepository()
	ctx := context.Background()
	req := baseUpsert("type-lb-lifecycle")
	req.DisplayGroupCodes = nil
	req.LegalBases = []disclosureapp.LegalBasisDTO{
		{ID: "lb-a", Title: "Alpha", Summary: "A body", Code: "A1", Authority: "BTC", IssueDate: "2020-11-16", Link: "https://example.com/a"},
		{ID: "lb-b", Title: "Beta", Summary: "B body"},
	}
	req.LegalBasis = "client-flat"
	req.LegalBasesProvided = true

	v1, err := repo.UpsertTypeVersion(ctx, req)
	if err != nil {
		t.Fatalf("v1: %v", err)
	}
	d1, err := repo.GetTypeVersionDetail(ctx, "c1", "type-lb-lifecycle", v1.VersionNo)
	if err != nil {
		t.Fatal(err)
	}
	if len(d1.LegalBases) != 2 || d1.LegalBases[0].ID != "lb-a" || d1.LegalBases[1].ID != "lb-b" {
		t.Fatalf("v1 bases=%#v", d1.LegalBases)
	}

	if _, err := repo.ActivateTypeVersion(ctx, disclosureapp.ActivateTypeVersionRequest{
		Subject: testSubject(), TypeID: "type-lb-lifecycle", VersionNo: v1.VersionNo,
	}); err != nil {
		t.Fatalf("activate v1: %v", err)
	}

	// Create open draft (v2) with same structured payload — IDs must regenerate on new version.
	req.Name = "Draft 1"
	req.PreserveLegalBases = false
	req.LegalBasesProvided = true
	req.LegalBases = []disclosureapp.LegalBasisDTO{
		{ID: "lb-a", Title: "Alpha", Summary: "A body", Code: "A1", Authority: "BTC", IssueDate: "2020-11-16", Link: "https://example.com/a"},
		{ID: "lb-b", Title: "Beta", Summary: "B body"},
	}
	draftr, err := repo.UpsertTypeVersion(ctx, req)
	if err != nil {
		t.Fatalf("draft create: %v", err)
	}
	if draftr.VersionNo != 2 {
		t.Fatalf("want draft v2 got %d", draftr.VersionNo)
	}
	draftDetail, err := repo.GetTypeVersionDetail(ctx, "c1", "type-lb-lifecycle", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(draftDetail.LegalBases) != 2 {
		t.Fatalf("draft bases len=%d", len(draftDetail.LegalBases))
	}
	if draftDetail.LegalBases[0].ID == "lb-a" || draftDetail.LegalBases[1].ID == "lb-b" {
		t.Fatalf("new version must regenerate IDs, got %#v", draftDetail.LegalBases)
	}
	if draftDetail.LegalBases[0].Title != "Alpha" || draftDetail.LegalBases[1].Title != "Beta" {
		t.Fatalf("fields/order lost: %#v", draftDetail.LegalBases)
	}
	if draftDetail.LegalBases[0].Code != "A1" || draftDetail.LegalBases[0].Authority != "BTC" {
		t.Fatalf("metadata lost: %+v", draftDetail.LegalBases[0])
	}
	// Source v1 unchanged
	d1b, _ := repo.GetTypeVersionDetail(ctx, "c1", "type-lb-lifecycle", 1)
	if d1b.LegalBases[0].ID != "lb-a" || d1b.LegalBases[0].Title != "Alpha" {
		t.Fatalf("source mutated: %#v", d1b.LegalBases)
	}
	draftIDs := []string{draftDetail.LegalBases[0].ID, draftDetail.LegalBases[1].ID}

	// Same-draft overwrite with PreserveLegalBases → keep draft IDs
	req.Name = "Draft 1 overwritten"
	req.LegalBasesProvided = false
	req.PreserveLegalBases = true
	req.LegalBases = nil
	req.LegalBasis = "should-not-wipe"
	over, err := repo.UpsertTypeVersion(ctx, req)
	if err != nil {
		t.Fatalf("overwrite: %v", err)
	}
	if over.VersionNo != 2 {
		t.Fatalf("overwrite version=%d", over.VersionNo)
	}
	after, err := repo.GetTypeVersionDetail(ctx, "c1", "type-lb-lifecycle", 2)
	if err != nil {
		t.Fatal(err)
	}
	if after.LegalBases[0].ID != draftIDs[0] || after.LegalBases[1].ID != draftIDs[1] {
		t.Fatalf("same-draft preserve failed: before=%v after=%#v", draftIDs, after.LegalBases)
	}

	// Field edit on same draft preserves IDs
	req.LegalBasesProvided = true
	req.PreserveLegalBases = false
	req.LegalBases = []disclosureapp.LegalBasisDTO{
		{ID: draftIDs[0], Title: "Alpha edited", Summary: "A body", Code: "A1", Authority: "BTC", IssueDate: "2020-11-16", Link: "https://example.com/a"},
		{ID: draftIDs[1], Title: "Beta", Summary: "B body"},
	}
	if _, err := repo.UpsertTypeVersion(ctx, req); err != nil {
		t.Fatal(err)
	}
	edited, _ := repo.GetTypeVersionDetail(ctx, "c1", "type-lb-lifecycle", 2)
	if edited.LegalBases[0].ID != draftIDs[0] || edited.LegalBases[0].Title != "Alpha edited" {
		t.Fatalf("edit preserve ID failed: %#v", edited.LegalBases)
	}

	// Reorder same IDs
	req.LegalBases = []disclosureapp.LegalBasisDTO{
		{ID: draftIDs[1], Title: "Beta", Summary: "B body"},
		{ID: draftIDs[0], Title: "Alpha edited", Summary: "A body", Code: "A1"},
	}
	if _, err := repo.UpsertTypeVersion(ctx, req); err != nil {
		t.Fatal(err)
	}
	reordered, _ := repo.GetTypeVersionDetail(ctx, "c1", "type-lb-lifecycle", 2)
	if reordered.LegalBases[0].ID != draftIDs[1] || reordered.LegalBases[1].ID != draftIDs[0] {
		t.Fatalf("reorder IDs broken: %#v", reordered.LegalBases)
	}

	// Activate preserves content/IDs
	if _, err := repo.ActivateTypeVersion(ctx, disclosureapp.ActivateTypeVersionRequest{
		Subject: testSubject(), TypeID: "type-lb-lifecycle", VersionNo: 2,
	}); err != nil {
		t.Fatal(err)
	}
	act, _ := repo.GetTypeVersionDetail(ctx, "c1", "type-lb-lifecycle", 2)
	if act.LegalBases[0].ID != draftIDs[1] || act.LegalBases[1].ID != draftIDs[0] {
		t.Fatalf("activate mutated IDs: %#v", act.LegalBases)
	}

	// Post-activate new draft with Preserve → copy from prev max with NEW ids
	req.Name = "v3 draft"
	req.LegalBasesProvided = false
	req.PreserveLegalBases = true
	req.LegalBases = nil
	v3, err := repo.UpsertTypeVersion(ctx, req)
	if err != nil {
		t.Fatalf("v3: %v", err)
	}
	if v3.VersionNo != 3 {
		t.Fatalf("want v3 got %d", v3.VersionNo)
	}
	v3d, _ := repo.GetTypeVersionDetail(ctx, "c1", "type-lb-lifecycle", 3)
	if len(v3d.LegalBases) != 2 {
		t.Fatalf("preserve new version lost bases: %#v", v3d.LegalBases)
	}
	if v3d.LegalBases[0].ID == draftIDs[1] || v3d.LegalBases[1].ID == draftIDs[0] {
		t.Fatalf("v3 must regen IDs from activated v2, got %#v", v3d.LegalBases)
	}
	if v3d.LegalBases[0].Title != "Beta" || v3d.LegalBases[1].Title != "Alpha edited" {
		t.Fatalf("v3 order/fields: %#v", v3d.LegalBases)
	}
	// Activated source still intact
	act2, _ := repo.GetTypeVersionDetail(ctx, "c1", "type-lb-lifecycle", 2)
	if act2.LegalBases[0].ID != draftIDs[1] {
		t.Fatalf("activated source mutated")
	}
}

func TestLegalBases_NewVersionLegacyOnlyFlat(t *testing.T) {
	repo := inmemory.NewRepository()
	ctx := context.Background()
	req := baseUpsert("type-lb-legacy")
	req.DisplayGroupCodes = nil
	req.LegalBasis = "legacy flat text only"
	req.LegalBases = nil
	req.LegalBasesProvided = false
	req.PreserveLegalBases = true
	if _, err := repo.UpsertTypeVersion(ctx, req); err != nil {
		t.Fatal(err)
	}
	// force draft then activate path: create draft then activate then new version preserve
	req.Name = "draft"
	d, err := repo.UpsertTypeVersion(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ActivateTypeVersion(ctx, disclosureapp.ActivateTypeVersionRequest{
		Subject: testSubject(), TypeID: "type-lb-legacy", VersionNo: d.VersionNo,
	}); err != nil {
		t.Fatal(err)
	}
	req.Name = "after"
	req.PreserveLegalBases = true
	req.LegalBasesProvided = false
	v3, err := repo.UpsertTypeVersion(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	detail, _ := repo.GetTypeVersionDetail(ctx, "c1", "type-lb-legacy", v3.VersionNo)
	if len(detail.LegalBases) != 0 {
		t.Fatalf("legacy-only must not invent structured: %#v", detail.LegalBases)
	}
	if detail.LegalBasis == "" && req.LegalBasis == "" {
		// flat may come from previous via prepare when preserve — previous had flat
	}
}

func TestLegalBases_ExplicitEmptyClearsOnNewVersion(t *testing.T) {
	repo := inmemory.NewRepository()
	ctx := context.Background()
	req := baseUpsert("type-lb-clear")
	req.DisplayGroupCodes = nil
	req.LegalBases = []disclosureapp.LegalBasisDTO{{ID: "x", Title: "T", Summary: "S"}}
	req.LegalBasesProvided = true
	if _, err := repo.UpsertTypeVersion(ctx, req); err != nil {
		t.Fatal(err)
	}
	req.Name = "draft"
	d, _ := repo.UpsertTypeVersion(ctx, req)
	_, _ = repo.ActivateTypeVersion(ctx, disclosureapp.ActivateTypeVersionRequest{
		Subject: testSubject(), TypeID: "type-lb-clear", VersionNo: d.VersionNo,
	})
	req.Name = "clear"
	req.LegalBases = []disclosureapp.LegalBasisDTO{}
	req.LegalBasesProvided = true
	req.PreserveLegalBases = false
	v, err := repo.UpsertTypeVersion(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	detail, _ := repo.GetTypeVersionDetail(ctx, "c1", "type-lb-clear", v.VersionNo)
	if len(detail.LegalBases) != 0 {
		t.Fatalf("explicit [] should clear, got %#v", detail.LegalBases)
	}
}
