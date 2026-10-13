package http_test

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
)

func TestListMemberships_Handler_UsesFlatContractAndPagination(t *testing.T) {
	h, repo := setupHandler("c-test", "Test Corp")
	_, err := repo.CreateUser(context.Background(), caapp.UserView{
		UserID:        "u-member-1",
		LoginID:       "member.one@example.com",
		FullName:      "Member One",
		AccountStatus: "active",
	}, "hash", caapp.CreateUserOptions{
		MembershipID:     "m-member-1",
		CompanyID:        "c-test",
		MembershipStatus: "active",
	})
	if err != nil {
		t.Fatalf("CreateUser member 1: %v", err)
	}
	_, err = repo.CreateUser(context.Background(), caapp.UserView{
		UserID:        "u-member-2",
		LoginID:       "member.two@example.com",
		FullName:      "Member Two",
		AccountStatus: "active",
	}, "hash", caapp.CreateUserOptions{
		MembershipID:     "m-member-2",
		CompanyID:        "c-test",
		MembershipStatus: "active",
	})
	if err != nil {
		t.Fatalf("CreateUser member 2: %v", err)
	}

	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/api/v1/admin/companies/c-test/memberships?page=2&page_size=1", nil)
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d want 200; body=%s", w.Code, w.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	items, ok := body["items"].([]any)
	if !ok {
		t.Fatalf("items type=%T want []any; body=%v", body["items"], body)
	}
	if len(items) != 1 {
		t.Fatalf("len(items)=%d want 1; body=%v", len(items), body)
	}
	if body["total"] != float64(2) {
		t.Fatalf("total=%v want 2", body["total"])
	}
	if body["page"] != float64(2) {
		t.Fatalf("page=%v want 2", body["page"])
	}
	if body["page_size"] != float64(1) {
		t.Fatalf("page_size=%v want 1", body["page_size"])
	}
}

// Regression: page outside int made Atoi clamp to MaxInt; (page-1)*page_size then
// overflowed negative and all[start:end] panicked, so the client got no response.
func TestListMemberships_Handler_HugePageReturnsEmptyPage(t *testing.T) {
	cases := []struct {
		name     string
		query    string
		wantPage int
	}{
		{"page outside int", "page=100000000000000000000&page_size=20", math.MaxInt},
		{"overflow page", "page=500000000000000000&page_size=20", 500000000000000000},
		{"max int page", "page=9223372036854775807&page_size=100", math.MaxInt},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, repo := setupHandler("c-test", "Test Corp")
			if _, err := repo.CreateUser(context.Background(), caapp.UserView{
				UserID: "u-member-1", LoginID: "member.one@example.com", FullName: "Member One", AccountStatus: "active",
			}, "hash", caapp.CreateUserOptions{
				MembershipID: "m-member-1", CompanyID: "c-test", MembershipStatus: "active",
			}); err != nil {
				t.Fatalf("CreateUser: %v", err)
			}
			mux := http.NewServeMux()
			h.Register(mux)

			req := httptest.NewRequest("GET", "/api/v1/admin/companies/c-test/memberships?"+tc.query, nil)
			req.Header.Set("Authorization", "Bearer tok")
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("status=%d want 200; body=%s", w.Code, w.Body.String())
			}
			var body struct {
				Items []any `json:"items"`
				Total int   `json:"total"`
				Page  int   `json:"page"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}
			if body.Items == nil || len(body.Items) != 0 || body.Total != 1 || body.Page != tc.wantPage {
				t.Fatalf("items=%v total=%d page=%d want [] / 1 / %d", body.Items, body.Total, body.Page, tc.wantPage)
			}
		})
	}
}
