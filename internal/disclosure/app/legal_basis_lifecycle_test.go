package app_test

import (
	"context"
	"strconv"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

type seqIDGen struct{ n int }

func (g *seqIDGen) NewUUID() string {
	g.n++
	return "new-id-" + strconv.Itoa(g.n)
}

func TestDeepCopyLegalBases_Isolation(t *testing.T) {
	src := []disclosureapp.LegalBasisDTO{
		{ID: "a", Title: "T1", Summary: "S1", Code: "C1"},
		{ID: "b", Title: "T2", Summary: "S2"},
	}
	cp := disclosureapp.DeepCopyLegalBases(src)
	if len(cp) != 2 {
		t.Fatalf("len=%d", len(cp))
	}
	cp[0].Title = "MUTATED"
	cp[0].Summary = "X"
	if src[0].Title != "T1" || src[0].Summary != "S1" {
		t.Fatalf("source mutated: %+v", src[0])
	}
	src[1].Title = "SRC_MUT"
	if cp[1].Title != "T2" {
		t.Fatalf("target mutated via source: %+v", cp[1])
	}
}

func TestRegenerateLegalBasisIDs_AllNewAndUnique(t *testing.T) {
	src := []disclosureapp.LegalBasisDTO{
		{ID: "old-1", Title: "A", Summary: "sa", Authority: "BTC", IssueDate: "2020-11-16", Link: "https://example.com/a", Code: "96"},
		{ID: "old-2", Title: "B", Summary: "sb"},
	}
	idg := &seqIDGen{}
	out := disclosureapp.RegenerateLegalBasisIDs(src, idg)
	if out[0].ID == "old-1" || out[1].ID == "old-2" {
		t.Fatalf("IDs not regenerated: %#v", out)
	}
	if out[0].ID == out[1].ID {
		t.Fatal("IDs not unique")
	}
	if out[0].Title != "A" || out[0].Code != "96" || out[0].Authority != "BTC" || out[0].IssueDate != "2020-11-16" {
		t.Fatalf("fields lost: %+v", out[0])
	}
	if src[0].ID != "old-1" {
		t.Fatal("source ID mutated")
	}
}

func TestPrepareLegalBasesForNewVersion_PreserveStructured(t *testing.T) {
	ctx := context.Background()
	src := []disclosureapp.LegalBasisDTO{
		{ID: "s1", Title: "Second", Summary: "2"},
		{ID: "s0", Title: "First", Summary: "1", Code: "X"},
	}
	// Order must be preserved as stored (s1 then s0).
	idg := &seqIDGen{}
	bases, flat, dropped := disclosureapp.PrepareLegalBasesForNewVersion(
		ctx, "dt-1", src, "stale flat", nil, "", true, idg,
	)
	if dropped != 0 || len(bases) != 2 {
		t.Fatalf("bases=%d dropped=%d", len(bases), dropped)
	}
	if bases[0].Title != "Second" || bases[1].Title != "First" {
		t.Fatalf("order broken: %#v", bases)
	}
	if bases[0].ID == "s1" || bases[1].ID == "s0" {
		t.Fatalf("IDs reused: %#v", bases)
	}
	if flat != "Second\n\nFirst" {
		t.Fatalf("projection=%q", flat)
	}
	if src[0].ID != "s1" {
		t.Fatal("source changed")
	}
}

func TestPrepareLegalBasesForNewVersion_LegacyOnly(t *testing.T) {
	bases, flat, _ := disclosureapp.PrepareLegalBasesForNewVersion(
		context.Background(), "dt-1",
		[]disclosureapp.LegalBasisDTO{{Title: "", Summary: ""}},
		"  only flat  ", nil, "ignored", true, &seqIDGen{},
	)
	if len(bases) != 0 {
		t.Fatalf("expected empty structured, got %#v", bases)
	}
	if flat != "only flat" {
		t.Fatalf("flat=%q", flat)
	}
}

func TestPrepareLegalBasesForNewVersion_ProvidedRegen(t *testing.T) {
	provided := []disclosureapp.LegalBasisDTO{
		{ID: "keep-me", Title: "NĐ", Summary: "p"},
	}
	bases, flat, _ := disclosureapp.PrepareLegalBasesForNewVersion(
		context.Background(), "dt", nil, "", provided, "client flat", false, &seqIDGen{},
	)
	if len(bases) != 1 || bases[0].ID == "keep-me" {
		t.Fatalf("%#v", bases)
	}
	if flat != "NĐ" {
		t.Fatalf("flat=%q", flat)
	}
	if provided[0].ID != "keep-me" {
		t.Fatal("provided mutated")
	}
}

func TestPrepareLegalBasesForNewVersion_MixedInvalidDropped(t *testing.T) {
	src := []disclosureapp.LegalBasisDTO{
		{ID: "bad", Title: "", Summary: ""},
		{ID: "good", Title: "OK", Summary: ""},
	}
	bases, _, dropped := disclosureapp.PrepareLegalBasesForNewVersion(
		context.Background(), "dt", src, "stale", nil, "", true, &seqIDGen{},
	)
	if dropped != 1 || len(bases) != 1 || bases[0].Title != "OK" {
		t.Fatalf("bases=%#v dropped=%d", bases, dropped)
	}
	if bases[0].ID == "good" {
		t.Fatal("expected regenerated ID")
	}
}
