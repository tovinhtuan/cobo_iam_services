package app

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

func TestProjectLegalBasesToLegacy_GoldenCases(t *testing.T) {
	t.Parallel()
	cases := []struct {
		id       string
		input    []LegalBasisDTO
		expected string
		wantErr  bool
	}{
		{"P1", []LegalBasisDTO{{Title: "TT 96", Summary: ""}}, "TT 96", false},
		{"P2", []LegalBasisDTO{{Title: "", Summary: "Free text only"}}, "Free text only", false},
		{"P3", []LegalBasisDTO{{Title: "A", Summary: "x"}, {Title: "B", Summary: "y"}}, "A\n\nB", false},
		{"P4", []LegalBasisDTO{{Title: "", Summary: ""}, {Code: "x"}}, "", false},
		{"P5", []LegalBasisDTO{{Title: "", Summary: "line1\nline2"}}, "line1\nline2", false},
		{"P6", []LegalBasisDTO{{Title: "Thông tư 96/2020/TT-BTC", Summary: ""}}, "Thông tư 96/2020/TT-BTC", false},
		{"P7", []LegalBasisDTO{{Title: "Second"}, {Title: "First"}}, "Second\n\nFirst", false},
		{"P8", []LegalBasisDTO{{Code: "96", Link: "https://example.com"}}, "", false},
		{"P9", []LegalBasisDTO{{Title: "OnlyTitle", Code: "C", Link: "https://example.com", Summary: "ignored-when-title"}}, "OnlyTitle", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.id, func(t *testing.T) {
			t.Parallel()
			got, err := ProjectLegalBasesToLegacy(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got != tc.expected {
				t.Fatalf("got %q want %q", got, tc.expected)
			}
		})
	}
}

func TestProjectLegalBasesToLegacy_P10Overflow(t *testing.T) {
	t.Parallel()
	huge := strings.Repeat("あ", LegalBasisProjectionMaxRunes+1)
	_, err := ProjectLegalBasesToLegacy([]LegalBasisDTO{{Title: huge}})
	if err == nil {
		t.Fatal("expected overflow error")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != 400 {
		t.Fatalf("expected 400 HTTPError, got %#v", err)
	}
}

func TestProjectLegalBasesToLegacy_DoesNotMutateInput(t *testing.T) {
	t.Parallel()
	in := []LegalBasisDTO{{Title: "  A  ", Summary: "s", Code: "c"}}
	_, err := ProjectLegalBasesToLegacy(in)
	if err != nil {
		t.Fatal(err)
	}
	if in[0].Title != "  A  " {
		t.Fatalf("input mutated: %#v", in[0])
	}
}

func TestNormalizeLegalBasesForRead_DropsInvalidKeepsValid(t *testing.T) {
	t.Parallel()
	out, dropped := NormalizeLegalBasesForRead([]LegalBasisDTO{
		{Title: "", Summary: ""},
		{Title: "OK"},
		{Code: "only"},
		{Summary: "sum"},
	})
	if dropped != 2 || len(out) != 2 {
		t.Fatalf("dropped=%d out=%d", dropped, len(out))
	}
}

func TestValidateLegalBasesForWrite_LimitsAndDuplicate(t *testing.T) {
	t.Parallel()
	idg := idgen.UUIDv7Generator{}

	t.Run("max_items", func(t *testing.T) {
		items := make([]LegalBasisDTO, LegalBasisMaxItems+1)
		for i := range items {
			items[i] = LegalBasisDTO{Title: string(rune('A'+i%26)) + string(rune('0'+i%10))}
		}
		_, err := ValidateLegalBasesForWrite(items, idg)
		if err == nil {
			t.Fatal("expected max items error")
		}
	})

	t.Run("title_runes", func(t *testing.T) {
		_, err := ValidateLegalBasesForWrite([]LegalBasisDTO{{
			Title: strings.Repeat("あ", LegalBasisTitleMaxRunes+1),
		}}, idg)
		if err == nil {
			t.Fatal("expected title overflow")
		}
	})

	t.Run("summary_8000_ok", func(t *testing.T) {
		_, err := ValidateLegalBasesForWrite([]LegalBasisDTO{{
			Summary: strings.Repeat("b", LegalBasisSummaryMaxRunes),
		}}, idg)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("exact_duplicate_block", func(t *testing.T) {
		_, err := ValidateLegalBasesForWrite([]LegalBasisDTO{
			{Title: "Same", Code: "C", Summary: "S", Link: ""},
			{Title: "Same", Code: "C", Summary: "S", Link: ""},
		}, idg)
		if err == nil {
			t.Fatal("expected duplicate error")
		}
	})

	t.Run("same_title_allowed", func(t *testing.T) {
		_, err := ValidateLegalBasesForWrite([]LegalBasisDTO{
			{Title: "Same", Code: "A"},
			{Title: "Same", Code: "B"},
		}, idg)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("same_code_authority_allowed", func(t *testing.T) {
		_, err := ValidateLegalBasesForWrite([]LegalBasisDTO{
			{Title: "T1", Code: "96", Authority: "BTC", Summary: "a"},
			{Title: "T2", Code: "96", Authority: "BTC", Summary: "b"},
		}, idg)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invalid_link_hash", func(t *testing.T) {
		_, err := ValidateLegalBasesForWrite([]LegalBasisDTO{{Title: "T", Link: "#"}}, idg)
		if err == nil {
			t.Fatal("expected invalid link")
		}
	})

	t.Run("invalid_date", func(t *testing.T) {
		_, err := ValidateLegalBasesForWrite([]LegalBasisDTO{{Title: "T", IssueDate: "16/11/2020"}}, idg)
		if err == nil {
			t.Fatal("expected invalid date")
		}
	})

	t.Run("https_ok", func(t *testing.T) {
		_, err := ValidateLegalBasesForWrite([]LegalBasisDTO{{Title: "T", Link: "https://example.com/x"}}, idg)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("relative_path_ok", func(t *testing.T) {
		_, err := ValidateLegalBasesForWrite([]LegalBasisDTO{{Title: "T", Link: "/docs/a"}}, idg)
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestApplyLegalBasisReadCompat(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("structured_wins_and_projects", func(t *testing.T) {
		dto := &DisclosureTypeDTO{
			TypeID:     "t1",
			VersionNo:  1,
			LegalBasis: "stale flat",
			LegalBases: []LegalBasisDTO{{Title: "Canon", Summary: ""}},
		}
		ApplyLegalBasisReadCompat(ctx, dto, true, true)
		if dto.LegalBasis != "Canon" || len(dto.LegalBases) != 1 {
			t.Fatalf("%#v", dto)
		}
	})

	t.Run("legacy_fallback", func(t *testing.T) {
		dto := &DisclosureTypeDTO{
			TypeID:     "qa-type",
			LegalBasis: "full legacy text",
			LegalBases: []LegalBasisDTO{},
		}
		ApplyLegalBasisReadCompat(ctx, dto, true, true)
		if len(dto.LegalBases) != 1 {
			t.Fatalf("expected synthesize")
		}
		if dto.LegalBases[0].ID != "qa-type-lb-legacy-1" {
			t.Fatalf("id=%q", dto.LegalBases[0].ID)
		}
		if dto.LegalBases[0].Title != legalBasisLegacyTitle || dto.LegalBases[0].Summary != "full legacy text" {
			t.Fatalf("%#v", dto.LegalBases[0])
		}
		if dto.LegalBasis != "full legacy text" {
			t.Fatalf("flat=%q", dto.LegalBasis)
		}
	})

	t.Run("both_empty", func(t *testing.T) {
		dto := &DisclosureTypeDTO{LegalBases: nil, LegalBasis: ""}
		ApplyLegalBasisReadCompat(ctx, dto, true, true)
		if len(dto.LegalBases) != 0 || dto.LegalBasis != "" {
			t.Fatalf("%#v", dto)
		}
	})

	t.Run("fallback_disabled", func(t *testing.T) {
		dto := &DisclosureTypeDTO{LegalBasis: "x", LegalBases: nil}
		ApplyLegalBasisReadCompat(ctx, dto, false, true)
		if len(dto.LegalBases) != 0 {
			t.Fatalf("should not synthesize")
		}
	})
}

func TestResolveLegalBasisWrite(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	idg := idgen.UUIDv7Generator{}

	t.Run("structured_flag_on_ignores_client_flat", func(t *testing.T) {
		req := &UpsertTypeVersionRequest{
			TypeID:             "t1",
			LegalBasesProvided: true,
			LegalBases:         []LegalBasisDTO{{Title: "A", Link: "https://example.com"}},
			LegalBasis:         "client stale",
		}
		if err := ResolveLegalBasisWrite(ctx, req, true, idg); err != nil {
			t.Fatal(err)
		}
		if req.LegalBasis != "A" || req.PreserveLegalBases {
			t.Fatalf("%#v", req)
		}
	})

	t.Run("omitted_preserves", func(t *testing.T) {
		req := &UpsertTypeVersionRequest{
			TypeID:             "t1",
			LegalBasesProvided: false,
			LegalBasis:         "flat only",
		}
		if err := ResolveLegalBasisWrite(ctx, req, false, idg); err != nil {
			t.Fatal(err)
		}
		if !req.PreserveLegalBases {
			t.Fatal("expected preserve")
		}
	})

	t.Run("empty_array_clears_when_provided", func(t *testing.T) {
		req := &UpsertTypeVersionRequest{
			TypeID:             "t1",
			LegalBasesProvided: true,
			LegalBases:         []LegalBasisDTO{},
			LegalBasis:         "ignored",
		}
		if err := ResolveLegalBasisWrite(ctx, req, true, idg); err != nil {
			t.Fatal(err)
		}
		if req.LegalBasis != "" || len(req.LegalBases) != 0 || req.PreserveLegalBases {
			t.Fatalf("%#v", req)
		}
	})

	t.Run("flag_off_allows_hash_link", func(t *testing.T) {
		req := &UpsertTypeVersionRequest{
			LegalBasesProvided: true,
			LegalBases:         []LegalBasisDTO{{Title: "T", Link: "#"}},
			LegalBasis:         "flat",
		}
		if err := ResolveLegalBasisWrite(ctx, req, false, idg); err != nil {
			t.Fatal(err)
		}
		if req.LegalBasis != "flat" {
			t.Fatalf("flag off should keep client flat")
		}
	})
}

func TestSynthesizeLegacy_NoAPIMarkerField(t *testing.T) {
	t.Parallel()
	item := SynthesizeLegacyLegalBasis("tid", "body")
	// Ensure DTO has only MVP fields populated as locked.
	if item.Code != "" || item.Authority != "" || item.IssueDate != "" || item.Link != "" {
		t.Fatalf("%#v", item)
	}
	if utf8.RuneCountInString(item.Title) == 0 {
		t.Fatal("title required")
	}
}
