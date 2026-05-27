package mysql

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cobo/cobo_iam_services/internal/marketreference/app"
)

func TestSearchUsesSymbolPrefix(t *testing.T) {
	tests := []struct {
		q    string
		want bool
	}{
		{"VIC", true},
		{"vic", true},
		{"A1", true},
		{"", false},
		{"  ", false},
		{"Vingroup Corp", false},
		{"TOO-LONG-SYMBOL", false},
		{"ACB BANK", false},
	}
	for _, tc := range tests {
		if got := SearchUsesSymbolPrefix(tc.q); got != tc.want {
			t.Errorf("SearchUsesSymbolPrefix(%q) = %v, want %v", tc.q, got, tc.want)
		}
	}
}

func TestBuildListWhere_searchModes(t *testing.T) {
	where, args := BuildListWhere(app.ListParams{Q: "VIC"})
	if where == "" || len(args) != 1 {
		t.Fatalf("symbol search: where=%q args=%v", where, args)
	}
	if args[0] != "VIC" {
		t.Fatalf("args[0] = %v, want VIC", args[0])
	}
	if !strings.Contains(where, "UPPER(e.symbol)") {
		t.Fatalf("expected symbol prefix clause, got %q", where)
	}
	if strings.Contains(where, "company_name") {
		t.Fatalf("symbol search must not use company_name: %q", where)
	}

	where, args = BuildListWhere(app.ListParams{Q: "Ngân hàng"})
	if !strings.Contains(where, "company_name") {
		t.Fatalf("expected company_name clause, got %q", where)
	}
	if args[0] != "Ngân hàng" {
		t.Fatalf("args[0] = %v", args[0])
	}
}

func TestBuildListWhere_exchangeAndHasProfile(t *testing.T) {
	hasProfile := true
	where, args := BuildListWhere(app.ListParams{Exchange: "HOSE", HasProfile: &hasProfile})
	if !strings.Contains(where, exchangeExpr) {
		t.Fatalf("expected exchange fallback expr in WHERE, got %q", where)
	}
	if args[0] != "HOSE" {
		t.Fatalf("exchange arg = %v", args[0])
	}
	if !strings.Contains(where, "p.symbol IS NOT NULL") {
		t.Fatalf("expected has_profile filter, got %q", where)
	}
}

func TestResolveExchange_fallbackFromInfo(t *testing.T) {
	info := map[string]any{"exchange": "HOSE"}
	got := ResolveExchange("", info)
	if got != "HOSE" {
		t.Fatalf("ResolveExchange = %q, want HOSE", got)
	}
	got = ResolveExchange("HNX", info)
	if got != "HNX" {
		t.Fatalf("equity column wins: got %q", got)
	}
}

func TestMapDetailFromProfile_fullJSON(t *testing.T) {
	raw := fullProfileFixture()
	info, err := parseProfileInfo(raw)
	if err != nil {
		t.Fatal(err)
	}
	updated := time.Date(2026, 5, 27, 2, 15, 0, 0, time.UTC)
	detail := mapDetailFromProfile("VIC", "Tập đoàn Vingroup", "", info, &updated)

	if !detail.HasProfile {
		t.Fatal("expected has_profile true")
	}
	if detail.Identity == nil || detail.Identity.Exchange == nil || *detail.Identity.Exchange != "HOSE" {
		t.Fatalf("identity.exchange = %v", detail.Identity)
	}
	if detail.LegalContact == nil || detail.LegalContact.TaxID == nil || *detail.LegalContact.TaxID == "" {
		t.Fatal("expected tax_id in legal_contact only on detail")
	}
	if detail.Timeline == nil || detail.Timeline.ListingDate == nil {
		t.Fatal("expected listing_date")
	}
	if detail.Leadership == nil || detail.Leadership.NumberOfEmployees == nil || *detail.Leadership.NumberOfEmployees != 12345 {
		t.Fatalf("number_of_employees = %v", detail.Leadership.NumberOfEmployees)
	}
}

func TestMapDetailFromProfile_partialMissingProfile(t *testing.T) {
	detail := app.ListedCompanyDetail{
		Symbol:      "FPT",
		CompanyName: "CTCP FPT",
		Source:      app.ProfileSourceKBS,
		HasProfile:  false,
	}
	if detail.HasProfile {
		t.Fatal("partial detail must have has_profile false")
	}
	if detail.Identity != nil || detail.LegalContact != nil {
		t.Fatal("partial detail must have nil groups")
	}
}

func TestListedCompanySummary_noTaxIDField(t *testing.T) {
	typ := reflect.TypeOf(app.ListedCompanySummary{})
	for i := 0; i < typ.NumField(); i++ {
		if typ.Field(i).Name == "TaxID" {
			t.Fatal("ListedCompanySummary must not expose tax_id")
		}
	}
}

func TestNormalizeSymbol_uppercase(t *testing.T) {
	if got := app.NormalizeSymbol(" vic "); got != "VIC" {
		t.Fatalf("NormalizeSymbol = %q, want VIC", got)
	}
}

func TestNormalizeListParams_defaultsAndMaxLimit(t *testing.T) {
	p, err := app.NormalizeListParams(app.ListParams{})
	if err != nil {
		t.Fatal(err)
	}
	if p.Page != 1 || p.Limit != 20 || p.SortBy != "symbol" || p.SortOrder != "asc" {
		t.Fatalf("defaults: %+v", p)
	}
	_, err = app.NormalizeListParams(app.ListParams{Limit: 101})
	if err != app.ErrInvalidRequest {
		t.Fatalf("limit > 100: err = %v", err)
	}
}

func fullProfileFixture() []byte {
	payload := map[string]any{
		"company_type":          "Công ty Cổ phần",
		"exchange":              "HOSE",
		"business_model":        "Bất động sản",
		"business_code":         "1234",
		"founded_date":          "2000-01-01T00:00:00Z",
		"listing_date":          "2007-11-15T00:00:00Z",
		"as_of_date":            "2026-05-27T00:00:00Z",
		"charter_capital":       1.5e10,
		"par_value":             10000.0,
		"listing_price":         40000.0,
		"listed_volume":         1e9,
		"outstanding_shares":    2e9,
		"free_float":            5e8,
		"free_float_percentage": 25.5,
		"ceo_name":              "CEO Name",
		"ceo_position":          "Tổng Giám đốc",
		"inspector_name":        "Inspector",
		"inspector_position":    "Trưởng BKS",
		"number_of_employees":   json.Number("12345"),
		"establishment_license": "GP-001",
		"tax_id":                "0100109106",
		"auditor":               "EY",
		"address":               "Hanoi",
		"phone":                 "0240000000",
		"fax":                   "0240000001",
		"email":                 "info@vic.vn",
		"website":               "https://vingroup.net",
	}
	b, _ := json.Marshal(payload)
	return b
}
