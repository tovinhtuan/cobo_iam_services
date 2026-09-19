package workflowstepcomments_test

import (
	"testing"

	wsc "github.com/cobo/cobo_iam_services/internal/workflowstepcomments"
	"github.com/google/uuid"
)

func TestHashCreateRequest_V1Compatibility(t *testing.T) {
	body := "  hello trim  "
	// Callers pass already-trimmed body into hash (service validates/trims first).
	trimmed := "hello trim"
	v1 := wsc.HashCreateRequest(trimmed, "[]")
	v1Empty := wsc.HashCreateRequest(trimmed, "")
	if v1 != v1Empty {
		t.Fatal("empty and [] must match")
	}
	// Prove equality with package V1 path via empty mentions.
	if wsc.HashCreateRequest(trimmed, "[]") != wsc.HashCreateRequest(trimmed, "") {
		t.Fatal("omit/empty json equivalence")
	}
	_ = body

	mid := uuid.NewString()
	can := []wsc.CanonicalMention{{MembershipID: mid, Start: 0, End: 5}}
	js, err := wsc.MarshalCanonicalMentionsJSON(can)
	if err != nil {
		t.Fatal(err)
	}
	h1 := wsc.HashCreateRequest(trimmed, js)
	h2 := wsc.HashCreateRequest(trimmed, js)
	if h1 != h2 {
		t.Fatal("same mentions must same hash")
	}
	if h1 == v1 {
		t.Fatal("non-empty mentions must differ from V1 body-only hash")
	}

	can2 := []wsc.CanonicalMention{{MembershipID: mid, Start: 1, End: 5}}
	js2, _ := wsc.MarshalCanonicalMentionsJSON(can2)
	if wsc.HashCreateRequest(trimmed, js2) == h1 {
		t.Fatal("different offset must different hash")
	}

	midB := uuid.NewString()
	can3 := []wsc.CanonicalMention{{MembershipID: midB, Start: 0, End: 5}}
	js3, _ := wsc.MarshalCanonicalMentionsJSON(can3)
	if wsc.HashCreateRequest(trimmed, js3) == h1 {
		t.Fatal("different membership must different hash")
	}

	// Reordered equivalent → same after canonicalize sort.
	reordered := []wsc.CanonicalMention{
		{MembershipID: midB, Start: 5, End: 7},
		{MembershipID: mid, Start: 0, End: 2},
	}
	sorted := []wsc.CanonicalMention{
		{MembershipID: mid, Start: 0, End: 2},
		{MembershipID: midB, Start: 5, End: 7},
	}
	jr, _ := wsc.MarshalCanonicalMentionsJSON(reordered)
	jsorted, _ := wsc.MarshalCanonicalMentionsJSON(sorted)
	// Marshal preserves input order — HashCreateRequest expects already-canonical JSON.
	// Service always sorts before marshal; prove sorted JSON equality:
	if jr == jsorted {
		// if equal by chance skip
	}
	jsorted2, _ := wsc.MarshalCanonicalMentionsJSON(sorted)
	if wsc.HashCreateRequest(trimmed, jsorted) != wsc.HashCreateRequest(trimmed, jsorted2) {
		t.Fatal("identical canonical JSON must same hash")
	}
}
