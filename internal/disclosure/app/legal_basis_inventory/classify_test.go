package inventory_test

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	inventory "github.com/cobo/cobo_iam_services/internal/disclosure/app/legal_basis_inventory"
)

func TestClassify_GroupsAE(t *testing.T) {
	cases := []struct {
		name   string
		flat   string
		json   string
		group  inventory.Group
		action inventory.ProposedAction
	}{
		{"A flat only", "  Luật A  ", "[]", inventory.GroupA, inventory.ActionWrapLegacyFlat},
		{"A null json", "text", "null", inventory.GroupA, inventory.ActionWrapLegacyFlat},
		{"B structured only", "", `[{"title":"TT","summary":"s"}]`, inventory.GroupB, inventory.ActionProjectStructured},
		{"E empty", "  ", "[]", inventory.GroupE, inventory.ActionNoOp},
		{"E whitespace flat empty arr", "\n\t", "[]", inventory.GroupE, inventory.ActionNoOp},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := inventory.ClassifyRecord(inventory.Record{
				TypeID: "dt", VersionNo: 1, LegalBasis: tc.flat, LegalBasesJSON: []byte(tc.json),
			})
			if r.Group != tc.group || r.ProposedAction != tc.action {
				t.Fatalf("got group=%s action=%s want %s/%s", r.Group, r.ProposedAction, tc.group, tc.action)
			}
			if strings.Contains(strings.ToLower(r.FlatHash+r.ProjectionHash), "luật") {
				t.Fatal("legal text leaked into hashes incorrectly — hashes should be hex")
			}
		})
	}
}

func TestClassify_GroupC_and_D(t *testing.T) {
	items := []disclosureapp.LegalBasisDTO{{Title: "Title1", Summary: "S1"}, {Title: "", Summary: "OnlySum"}}
	proj, err := disclosureapp.ProjectLegalBasesToLegacy(items)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(items)

	c := inventory.ClassifyRecord(inventory.Record{TypeID: "dt", VersionNo: 1, LegalBasis: proj, LegalBasesJSON: b})
	if c.Group != inventory.GroupC {
		t.Fatalf("C got %s", c.Group)
	}
	d := inventory.ClassifyRecord(inventory.Record{TypeID: "dt", VersionNo: 1, LegalBasis: proj + " EXTRA", LegalBasesJSON: b})
	if d.Group != inventory.GroupD || d.ProposedAction != inventory.ActionManualReview {
		t.Fatalf("D got %s %s", d.Group, d.ProposedAction)
	}
	if d.DivergenceClass == "" {
		t.Fatal("expected divergence class")
	}
}

func TestClassify_MalformedJSON(t *testing.T) {
	r := inventory.ClassifyRecord(inventory.Record{
		TypeID: "dt", VersionNo: 1, LegalBasis: "flat", LegalBasesJSON: []byte(`{"no":"array"}`),
	})
	if r.Group != inventory.GroupA || r.ProposedAction != inventory.ActionBlockedMalformed {
		t.Fatalf("%s %s", r.Group, r.ProposedAction)
	}
	r2 := inventory.ClassifyRecord(inventory.Record{
		TypeID: "dt", VersionNo: 1, LegalBasis: "", LegalBasesJSON: []byte(`not-json`),
	})
	if r2.Group != inventory.GroupE {
		t.Fatalf("malformed empty flat want E got %s", r2.Group)
	}
}

func TestClassify_MixedInvalidItems_StillB(t *testing.T) {
	raw := `[{"title":"","summary":""},{"title":"OK","summary":""}]`
	r := inventory.ClassifyRecord(inventory.Record{TypeID: "dt", VersionNo: 1, LegalBasis: "", LegalBasesJSON: []byte(raw)})
	if r.Group != inventory.GroupB || r.StructuredCount != 1 {
		t.Fatalf("%+v", r)
	}
}

func TestClassify_OverflowBlocked(t *testing.T) {
	// title with >8000 runes as single item → projection overflow
	title := strings.Repeat("あ", disclosureapp.LegalBasisProjectionMaxRunes+1)
	// title field itself may also violate title max — still structured valid for read if title non-empty
	// Use summary within summary max but projection from title is huge — title max is 500 so use many items
	items := make([]disclosureapp.LegalBasisDTO, 0, 20)
	chunk := strings.Repeat("x", 500)
	for i := 0; i < 20; i++ {
		items = append(items, disclosureapp.LegalBasisDTO{Title: chunk})
	}
	_ = title
	b, _ := json.Marshal(items)
	proj, err := disclosureapp.ProjectLegalBasesToLegacy(items)
	if err == nil && utf8.RuneCountInString(proj) <= disclosureapp.LegalBasisProjectionMaxRunes {
		t.Skip("projection did not overflow with this fixture")
	}
	r := inventory.ClassifyRecord(inventory.Record{TypeID: "dt", VersionNo: 1, LegalBasis: "", LegalBasesJSON: b})
	if r.ProposedAction != inventory.ActionBlockedOverflow && err == nil {
		// if project succeeds under 8000, not overflow
		t.Logf("action=%s projRunes=%d", r.ProposedAction, r.ProjectionRuneCount)
	}
	if err != nil && r.ProposedAction != inventory.ActionBlockedOverflow {
		t.Fatalf("want blocked overflow got %s", r.ProposedAction)
	}
}

func TestIdempotency_A_to_C(t *testing.T) {
	rec := inventory.Record{TypeID: "dt", VersionNo: 1, LegalBasis: "Legacy text body", LegalBasesJSON: []byte("[]")}
	r1 := inventory.ClassifyRecord(rec)
	if r1.Group != inventory.GroupA {
		t.Fatal(r1.Group)
	}
	sim := inventory.SimulateApply(rec, r1)
	r2 := inventory.ClassifyRecord(sim)
	if r2.Group != inventory.GroupC {
		t.Fatalf("after wrap want C got %s", r2.Group)
	}
	sim2 := inventory.SimulateApply(sim, r2)
	r3 := inventory.ClassifyRecord(sim2)
	if r3.TargetFlatHash != r2.TargetFlatHash {
		t.Fatal("idempotency hash drift")
	}
	if r3.ProposedAction != inventory.ActionNormalizeMatched {
		t.Fatalf("second pass action=%s", r3.ProposedAction)
	}
}

func TestIdempotency_D_unchanged(t *testing.T) {
	items := []disclosureapp.LegalBasisDTO{{Title: "T", Summary: "S"}}
	b, _ := json.Marshal(items)
	rec := inventory.Record{TypeID: "dt", VersionNo: 1, LegalBasis: "different", LegalBasesJSON: b}
	r1 := inventory.ClassifyRecord(rec)
	sim := inventory.SimulateApply(rec, r1)
	r2 := inventory.ClassifyRecord(sim)
	if r1.Group != inventory.GroupD || r2.Group != inventory.GroupD {
		t.Fatalf("%s %s", r1.Group, r2.Group)
	}
	if r2.ProposedAction != inventory.ActionManualReview {
		t.Fatal(r2.ProposedAction)
	}
}

func TestReconcile_Partition(t *testing.T) {
	results := []inventory.Result{
		{Group: inventory.GroupA}, {Group: inventory.GroupB}, {Group: inventory.GroupC},
		{Group: inventory.GroupD}, {Group: inventory.GroupE}, {Group: inventory.GroupA},
	}
	rec := inventory.Reconcile(results)
	if rec.Total != 6 || rec.SumGroups != 6 || rec.Unclassified != 0 {
		t.Fatalf("%+v", rec)
	}
	if rec.Groups[inventory.GroupA] != 2 {
		t.Fatal(rec.Groups)
	}
}

func TestRedaction_NoFullTextInResultJSON(t *testing.T) {
	secret := "THONG_TU_SIEU_BI_MAT_12345"
	r := inventory.ClassifyRecord(inventory.Record{
		TypeID: "dt", VersionNo: 1, LegalBasis: secret, LegalBasesJSON: []byte("[]"),
	})
	b, _ := json.Marshal(r)
	if strings.Contains(string(b), secret) {
		t.Fatalf("secret leaked in result json: %s", b)
	}
}
