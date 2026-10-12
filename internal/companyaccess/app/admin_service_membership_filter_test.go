package app

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func TestFoldSearch_StripsVietnameseDiacritics(t *testing.T) {
	for in, want := range map[string]string{
		"Nguyễn Văn Đức": "nguyen van duc",
		"TRẦN thị Hà":    "tran thi ha",
		"qa.Persona@X":   "qa.persona@x",
	} {
		if got := foldSearch(in); got != want {
			t.Errorf("foldSearch(%q)=%q want %q", in, got, want)
		}
	}
}

func TestNormalizeMembershipFilter(t *testing.T) {
	q, status, err := normalizeMembershipFilter("  an  ", " Pending ")
	if err != nil || q != "an" || status != MembershipStatusPending {
		t.Fatalf("got q=%q status=%q err=%v", q, status, err)
	}
	for _, tc := range []struct{ q, status, field string }{
		{strings.Repeat("ă", MembershipSearchMaxLen+1), "", "q"},
		{"", "deleted", "status"},
	} {
		_, _, err := normalizeMembershipFilter(tc.q, tc.status)
		var he *perr.HTTPError
		if !errors.As(err, &he) || he.HTTPStatus != http.StatusBadRequest || he.Code != perr.CodeInvalidRequest {
			t.Fatalf("%s: want 400 INVALID_REQUEST, got %v", tc.field, err)
		}
		fe, _ := he.Details["field_errors"].(map[string]any)
		if _, ok := fe[tc.field]; !ok {
			t.Fatalf("%s: field_errors=%v", tc.field, he.Details)
		}
	}
	// exactly the limit (runes, not bytes) is fine
	if _, _, err := normalizeMembershipFilter(strings.Repeat("ă", MembershipSearchMaxLen), ""); err != nil {
		t.Fatalf("limit rejected: %v", err)
	}
}

func TestFilterMemberships_SearchCountsThenStatus(t *testing.T) {
	all := []MembershipView{
		{UserID: "u1", FullName: "Nguyễn Văn An", LoginID: "an@cobo.test", AccountStatus: "active"},
		{UserID: "u2", FullName: "Trần Bình", LoginID: "nguyen.binh@cobo.test", AccountStatus: "pending_email_verification"},
		{UserID: "u3", FullName: "Lê Chi", LoginID: "chi@cobo.test", AccountStatus: "invited"},
		{UserID: "u4", FullName: "Phạm Dũng", LoginID: "dung@cobo.test", AccountStatus: "Suspended"},
		{UserID: "u5", FullName: "Hồ Em", LoginID: "em@cobo.test", AccountStatus: "weird"},
	}

	out, counts := filterMemberships(all, "", "")
	if len(out) != 5 || counts != (MembershipStatusCounts{All: 5, Active: 1, Invited: 1, Pending: 1, Inactive: 1}) {
		t.Fatalf("no filter: len=%d counts=%+v", len(out), counts)
	}

	// search matches name (diacritic-insensitive) or login; counts follow the search
	out, counts = filterMemberships(all, "nguyen", "")
	if len(out) != 2 || out[0].UserID != "u1" || out[1].UserID != "u2" {
		t.Fatalf("search: %+v", out)
	}
	if counts != (MembershipStatusCounts{All: 2, Active: 1, Pending: 1}) {
		t.Fatalf("search counts=%+v", counts)
	}

	// status narrows the rows but not the counts
	out, counts = filterMemberships(all, "nguyen", MembershipStatusPending)
	if len(out) != 1 || out[0].UserID != "u2" || counts.All != 2 || counts.Active != 1 {
		t.Fatalf("search+status: out=%+v counts=%+v", out, counts)
	}
	out, _ = filterMemberships(all, "", MembershipStatusInactive)
	if len(out) != 1 || out[0].UserID != "u4" {
		t.Fatalf("inactive group: %+v", out)
	}
	if len(all) != 5 || all[0].UserID != "u1" {
		t.Fatal("input slice must not be modified")
	}
}
