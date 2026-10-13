package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

func TestParseOptionalBoolQuery(t *testing.T) {
	cases := []struct {
		raw    string
		want   *bool
		wantOK bool
	}{
		{"", nil, true},
		{"  ", nil, true},
		{"true", ptrBool(true), true},
		{"1", ptrBool(true), true},
		{"FALSE", ptrBool(false), true},
		{"0", ptrBool(false), true},
		{"yes", nil, false},
	}
	for _, tc := range cases {
		got, ok := parseOptionalBoolQuery(tc.raw)
		if ok != tc.wantOK {
			t.Fatalf("%q ok=%v want %v", tc.raw, ok, tc.wantOK)
		}
		if (got == nil) != (tc.want == nil) || (got != nil && *got != *tc.want) {
			t.Fatalf("%q got=%v want=%v", tc.raw, got, tc.want)
		}
	}
}

func ptrBool(v bool) *bool { return &v }

type fakeListTypesService struct {
	disclosureapp.Service
	calls []disclosureapp.ListTypesRequest
}

func (f *fakeListTypesService) ListTypes(_ context.Context, req disclosureapp.ListTypesRequest) (*disclosureapp.ListTypesResponse, error) {
	f.calls = append(f.calls, req)
	return &disclosureapp.ListTypesResponse{Items: []disclosureapp.DisclosureTypeSummaryDTO{}, Page: req.Page, PageSize: req.PageSize}, nil
}

func TestListTypes_UnparseablePaginationReturns400(t *testing.T) {
	cases := []struct {
		name    string
		query   string
		wantMsg string
	}{
		{"page beyond int64", "page=100000000000000000000", "page must be a positive integer"},
		{"page below int64", "page=-100000000000000000000", "page must be a positive integer"},
		{"page not numeric", "page=abc", "page must be a positive integer"},
		{"page decimal", "page=1.5", "page must be a positive integer"},
		{"page_size beyond int64", "page_size=100000000000000000000", "page_size must be between 1 and 100"},
		{"page_size not numeric", "page_size=ten", "page_size must be between 1 and 100"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &fakeListTypesService{}
			h := NewHandler(svc, fakeDisclosureInspector{}, nil, nil)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/disclosure-types?"+tc.query, nil)
			req.Header.Set("Authorization", "Bearer atk_test")
			rr := httptest.NewRecorder()

			h.listTypes(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status=%d want 400; body=%s", rr.Code, rr.Body.String())
			}
			var body struct {
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body: %v; body=%s", err, rr.Body.String())
			}
			if body.Error.Code != "INVALID_REQUEST" || body.Error.Message != tc.wantMsg {
				t.Fatalf("error=%+v want INVALID_REQUEST %q", body.Error, tc.wantMsg)
			}
			if len(svc.calls) != 0 {
				t.Fatalf("service must not be called for unparseable pagination, got %d calls", len(svc.calls))
			}
		})
	}
}

func TestListTypes_PassesParsedPaginationToService(t *testing.T) {
	cases := []struct {
		name                 string
		query                string
		wantPage             int
		wantPageSize         int
		wantPageProvided     bool
		wantPageSizeProvided bool
	}{
		{"absent", "", 0, 0, false, false},
		{"blank", "page=%20&page_size=", 0, 0, false, false},
		{"explicit", "page=3&page_size=50", 3, 50, true, true},
		// Range checks (page >= 1, 1 <= page_size <= 100) stay in the service.
		{"zero passes through", "page=0&page_size=0", 0, 0, true, true},
		{"huge in-range page", "page=500000000000000000", 500000000000000000, 0, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &fakeListTypesService{}
			h := NewHandler(svc, fakeDisclosureInspector{}, nil, nil)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/disclosure-types?"+tc.query, nil)
			req.Header.Set("Authorization", "Bearer atk_test")
			rr := httptest.NewRecorder()

			h.listTypes(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("status=%d want 200; body=%s", rr.Code, rr.Body.String())
			}
			if len(svc.calls) != 1 {
				t.Fatalf("service calls=%d want 1", len(svc.calls))
			}
			got := svc.calls[0]
			if got.Page != tc.wantPage || got.PageSize != tc.wantPageSize ||
				got.PageProvided != tc.wantPageProvided || got.PageSizeProvided != tc.wantPageSizeProvided {
				t.Fatalf("got page=%d/%v page_size=%d/%v want %d/%v %d/%v",
					got.Page, got.PageProvided, got.PageSize, got.PageSizeProvided,
					tc.wantPage, tc.wantPageProvided, tc.wantPageSize, tc.wantPageSizeProvided)
			}
		})
	}
}
