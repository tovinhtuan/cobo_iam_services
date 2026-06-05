package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	marketapp "github.com/cobo/cobo_iam_services/internal/marketreference/app"
)

// fakeBusinessCodeRepo implements ListedCompanyReader for lookup handler tests.
type fakeBusinessCodeRepo struct {
	result marketapp.ListedCompanyDetail
	err    error
}

func (f *fakeBusinessCodeRepo) List(context.Context, marketapp.ListParams) (marketapp.ListResult, error) {
	return marketapp.ListResult{}, nil
}
func (f *fakeBusinessCodeRepo) GetBySymbol(context.Context, string) (marketapp.ListedCompanyDetail, error) {
	return marketapp.ListedCompanyDetail{}, nil
}
func (f *fakeBusinessCodeRepo) GetByBusinessCode(_ context.Context, _ string) (marketapp.ListedCompanyDetail, error) {
	return f.result, f.err
}

func newLookupHandler(t *testing.T, repo marketapp.ListedCompanyReader) *AdminHandler {
	t.Helper()
	h := &AdminHandler{}
	if repo != nil {
		h.listedLookup = marketapp.NewService(repo, nil)
	}
	return h
}

func doLookup(t *testing.T, h *AdminHandler, url string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	h.Register(mux)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

func TestListedLookup_Found(t *testing.T) {
	taxID := "0300588569"
	bizCode := "0101234567"
	addr := "10 Tân Trào"
	phone := "02854155555"
	email := "ir@example.com"
	ctype := "Công ty Cổ phần"
	exch := "HOSE"
	repo := &fakeBusinessCodeRepo{
		result: marketapp.ListedCompanyDetail{
			Symbol:      "VNM",
			CompanyName: "CTCP Sữa Việt Nam",
			HasProfile:  true,
			Identity: &marketapp.IdentityGroup{
				BusinessCode: &bizCode,
				Exchange:     &exch,
				CompanyType:  &ctype,
			},
			LegalContact: &marketapp.LegalContactGroup{
				TaxID:   &taxID,
				Address: &addr,
				Phone:   &phone,
				Email:   &email,
			},
		},
	}
	h := newLookupHandler(t, repo)
	rr := doLookup(t, h, "/api/v1/company/listed-lookup?business_code=0101234567")

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("json parse: %v", err)
	}
	if body["found"] != true {
		t.Fatalf("found=%v", body["found"])
	}
	sync, _ := body["sync"].(map[string]any)
	if sync == nil {
		t.Fatal("sync object missing")
	}
	if sync["company_name"] != "CTCP Sữa Việt Nam" {
		t.Fatalf("sync.company_name=%v", sync["company_name"])
	}
	if sync["tax_code"] != taxID {
		t.Fatalf("sync.tax_code=%v", sync["tax_code"])
	}
	if sync["registration_number"] != bizCode {
		t.Fatalf("sync.registration_number=%v", sync["registration_number"])
	}
	if sync["address"] != addr {
		t.Fatalf("sync.address=%v", sync["address"])
	}
	if body["disclaimer"] == "" || body["disclaimer"] == nil {
		t.Fatal("disclaimer missing or empty")
	}
	preview, _ := body["preview"].(map[string]any)
	if preview == nil {
		t.Fatal("preview object missing")
	}
	if preview["symbol"] != "VNM" {
		t.Fatalf("preview.symbol=%v", preview["symbol"])
	}
	// Cache-Control: public on 200
	cc := rr.Header().Get("Cache-Control")
	if !strings.HasPrefix(cc, "public") {
		t.Fatalf("Cache-Control=%q, want public", cc)
	}
	// Security header
	if rr.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("X-Content-Type-Options=%q", rr.Header().Get("X-Content-Type-Options"))
	}
}

func TestListedLookup_NotFound(t *testing.T) {
	repo := &fakeBusinessCodeRepo{err: marketapp.ErrNotFound}
	h := newLookupHandler(t, repo)
	rr := doLookup(t, h, "/api/v1/company/listed-lookup?business_code=9999999999")

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d", rr.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body["found"] != false {
		t.Fatalf("found=%v", body["found"])
	}
	// not_found still gets public cache (shorter TTL set in handler)
	cc := rr.Header().Get("Cache-Control")
	if !strings.HasPrefix(cc, "public") {
		t.Fatalf("Cache-Control=%q, want public for not_found", cc)
	}
}

func TestListedLookup_EmptyBusinessCode(t *testing.T) {
	h := newLookupHandler(t, &fakeBusinessCodeRepo{})
	rr := doLookup(t, h, "/api/v1/company/listed-lookup?business_code=")

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rr.Code)
	}
	// Error response must not be cached
	cc := rr.Header().Get("Cache-Control")
	if cc != "no-store" {
		t.Fatalf("Cache-Control=%q, want no-store on 400", cc)
	}
}

func TestListedLookup_MissingBusinessCode(t *testing.T) {
	h := newLookupHandler(t, &fakeBusinessCodeRepo{})
	rr := doLookup(t, h, "/api/v1/company/listed-lookup")

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestListedLookup_WhitespaceBusinessCode(t *testing.T) {
	h := newLookupHandler(t, &fakeBusinessCodeRepo{})
	rr := doLookup(t, h, "/api/v1/company/listed-lookup?business_code=+++")

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestListedLookup_OversizedBusinessCode(t *testing.T) {
	h := newLookupHandler(t, &fakeBusinessCodeRepo{})
	code := strings.Repeat("A", 51)
	rr := doLookup(t, h, "/api/v1/company/listed-lookup?business_code="+code)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestListedLookup_SpecialCharsRejected(t *testing.T) {
	h := newLookupHandler(t, &fakeBusinessCodeRepo{})
	// SQL injection attempt
	rr := doLookup(t, h, "/api/v1/company/listed-lookup?business_code=0101%27%20OR%201%3D1")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("SQL inject: status=%d", rr.Code)
	}
	// Log injection attempt (\n in URL-encoded form)
	rr2 := doLookup(t, h, "/api/v1/company/listed-lookup?business_code=ABC%0ADEF")
	if rr2.Code != http.StatusBadRequest {
		t.Fatalf("log inject: status=%d", rr2.Code)
	}
	// Semicolons/braces
	rr3 := doLookup(t, h, "/api/v1/company/listed-lookup?business_code=010;DROP")
	if rr3.Code != http.StatusBadRequest {
		t.Fatalf("special char: status=%d", rr3.Code)
	}
}

func TestListedLookup_ServiceUnavailable(t *testing.T) {
	repo := &fakeBusinessCodeRepo{err: marketapp.ErrUnavailable}
	h := newLookupHandler(t, repo)
	rr := doLookup(t, h, "/api/v1/company/listed-lookup?business_code=0101234567")

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	// 503 must not be cached
	cc := rr.Header().Get("Cache-Control")
	if cc != "no-store" {
		t.Fatalf("Cache-Control=%q, want no-store on 503", cc)
	}
}

func TestListedLookup_NilService503(t *testing.T) {
	h := newLookupHandler(t, nil) // nil repo → nil listedLookup
	rr := doLookup(t, h, "/api/v1/company/listed-lookup?business_code=0101234567")

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("nil service: status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestListedLookup_SyncOmitsNilFields(t *testing.T) {
	// Only company_name and tax_code; phone/email/address nil
	taxID := "0300588569"
	repo := &fakeBusinessCodeRepo{
		result: marketapp.ListedCompanyDetail{
			Symbol:      "VNM",
			CompanyName: "CTCP VNM",
			HasProfile:  true,
			LegalContact: &marketapp.LegalContactGroup{
				TaxID: &taxID,
				// Phone, Email, Address intentionally nil
			},
		},
	}
	h := newLookupHandler(t, repo)
	rr := doLookup(t, h, "/api/v1/company/listed-lookup?business_code=0300588569")

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	sync, _ := body["sync"].(map[string]any)
	if _, hasPhone := sync["phone"]; hasPhone {
		t.Fatal("nil phone must be omitted from sync")
	}
	if _, hasEmail := sync["contact_email"]; hasEmail {
		t.Fatal("nil email must be omitted from sync")
	}
	if _, hasAddr := sync["address"]; hasAddr {
		t.Fatal("nil address must be omitted from sync")
	}
	if sync["company_name"] == nil {
		t.Fatal("company_name must be present")
	}
}

func TestListedLookup_ErrorResponseShape(t *testing.T) {
	h := newLookupHandler(t, &fakeBusinessCodeRepo{})
	rr := doLookup(t, h, "/api/v1/company/listed-lookup?business_code=")
	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	errObj, ok := body["error"].(map[string]any)
	if !ok || errObj["code"] == nil {
		t.Fatalf("error response must have error.code: %s", rr.Body.String())
	}
}

func TestListedLookup_CacheHitSecondRequest(t *testing.T) {
	repo := &fakeBusinessCodeRepo{
		result: marketapp.ListedCompanyDetail{Symbol: "VNM", CompanyName: "Vinamilk", HasProfile: true},
	}
	h := newLookupHandler(t, repo)
	// First request — cache miss
	rr1 := doLookup(t, h, "/api/v1/company/listed-lookup?business_code=0300588569")
	if rr1.Code != http.StatusOK {
		t.Fatalf("first: status=%d", rr1.Code)
	}
	// Second request — must also succeed (cache hit)
	rr2 := doLookup(t, h, "/api/v1/company/listed-lookup?business_code=0300588569")
	if rr2.Code != http.StatusOK {
		t.Fatalf("second: status=%d", rr2.Code)
	}
	// Both responses have found:true
	var b1, b2 map[string]any
	_ = json.Unmarshal(rr1.Body.Bytes(), &b1)
	_ = json.Unmarshal(rr2.Body.Bytes(), &b2)
	if b1["found"] != true || b2["found"] != true {
		t.Fatalf("both must be found:true, got %v %v", b1["found"], b2["found"])
	}
}
