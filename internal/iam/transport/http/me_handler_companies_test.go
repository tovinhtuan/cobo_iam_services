package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
)

// meFakeMembersWithCode is a regression fixture for the dashboard-smoke-fix
// incident (2026-07-02): /api/v1/me/companies must expose company_code so the
// frontend does not fall back to displaying the raw company_id/UUID as a code.
type meFakeMembersWithCode struct{}

func (meFakeMembersWithCode) GetMembershipsByUser(context.Context, string) ([]caapp.MembershipView, error) {
	return []caapp.MembershipView{
		{
			MembershipID: "m_102",
			UserID:       "u_me",
			CompanyID:    "c_001",
			CompanyCode:  "cskh_9bea",
			CompanyName:  "Company X",
			Status:       "active",
		},
	}, nil
}
func (meFakeMembersWithCode) GetActiveMembership(context.Context, string, string) (*caapp.MembershipView, error) {
	return nil, nil
}
func (meFakeMembersWithCode) GetMembershipRoles(_ context.Context, membershipID string) ([]string, error) {
	if membershipID == "m_102" {
		return []string{"Admin Doanh Nghiep"}, nil
	}
	return nil, nil
}
func (meFakeMembersWithCode) GetMembershipDepartments(context.Context, string) ([]caapp.DepartmentView, error) {
	return nil, nil
}
func (meFakeMembersWithCode) GetMembershipTitles(context.Context, string) ([]string, error) {
	return nil, nil
}

func TestMeHandler_GETCompanies_IncludesCompanyCodeAndRoles(t *testing.T) {
	base := NewHandler(slog.Default(), nil, avatarFakeInspector{claims: &iamapp.AccessTokenClaims{Sub: "u_me"}}, nil, nil, nil, nil)
	me := NewMeHandler(base, meFakeIdentity{}, meFakeMembersWithCode{}, nil, avatarProfileRepo{status: "active"}, nil, nil, nil, nil, "http://api.test")

	mux := http.NewServeMux()
	me.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/companies", nil)
	req.Header.Set("Authorization", "Bearer test")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var out struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v body=%s", err, rec.Body.String())
	}
	if len(out.Items) != 1 {
		t.Fatalf("items=%d want 1", len(out.Items))
	}
	item := out.Items[0]

	if got := item["company_code"]; got != "cskh_9bea" {
		t.Fatalf("company_code=%v want cskh_9bea (regression: must not be missing/UUID)", got)
	}
	if got := item["company_id"]; got != "c_001" {
		t.Fatalf("company_id=%v want c_001", got)
	}
	roles, ok := item["roles"].([]any)
	if !ok || len(roles) != 1 || roles[0] != "Admin Doanh Nghiep" {
		t.Fatalf("roles=%v want [Admin Doanh Nghiep]", item["roles"])
	}
}
