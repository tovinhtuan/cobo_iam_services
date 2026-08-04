package app_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	iamhttp "github.com/cobo/cobo_iam_services/internal/iam/transport/http"
	"github.com/cobo/cobo_iam_services/internal/subscription/companyplan"
)

// Consistency: same company_id + same resolve `at` → GetOwnCompany.plan == /me/companies item.plan.
func TestCompanyPlan_Consistency_GetOwnCompanyAndMeCompanies(t *testing.T) {
	at := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	plans := companyplan.NewMemoryRepository()
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	_ = plans.Create(context.Background(), companyplan.CompanyPlan{
		ID: "p1", CompanyID: "c_001", Code: companyplan.PlanCodePremium, Status: companyplan.PlanStatusSuspended,
		EffectiveFrom: from, Origin: companyplan.RecordOriginDevFixture,
	})
	reader := companyplan.NewService(plans)

	adminRepo := cainmem.NewAdminRepository()
	seedCompany(adminRepo, "c_001", "Company X")
	adminSvc := caapp.NewAdminService(adminRepo, fakeAuthService{decision: authapp.DecisionAllow}, fixedIDGen("x"),
		caapp.WithCompanyPlanReader(reader),
		caapp.WithCompanyPlanNow(func() time.Time { return at }),
	)
	own, err := adminSvc.GetOwnCompany(context.Background(), caapp.GetOwnCompanyRequest{
		Subject: caapp.AdminSubject{UserID: "u_me", MembershipID: "m_102", CompanyID: "c_001"},
	})
	if err != nil {
		t.Fatal(err)
	}

	base := iamhttp.NewHandler(slog.Default(), nil, consistencyInspector{}, nil, nil, nil, nil)
	me := iamhttp.NewMeHandler(base, consistencyIdentity{}, consistencyMembers{}, nil, nil, nil, nil, nil, nil, "http://api.test")
	me.WithCompanyPlanReader(reader)
	me.WithCompanyPlanNow(func() time.Time { return at })
	mux := http.NewServeMux()
	me.Register(mux)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/companies", nil)
	req.Header.Set("Authorization", "Bearer test")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("me status=%d body=%s", rec.Code, rec.Body.String())
	}
	var meOut struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &meOut); err != nil {
		t.Fatal(err)
	}
	if len(meOut.Items) != 1 {
		t.Fatalf("items=%d", len(meOut.Items))
	}
	mePlanRaw, _ := json.Marshal(meOut.Items[0]["plan"])
	var mePlan companyplan.PlanDTO
	if err := json.Unmarshal(mePlanRaw, &mePlan); err != nil {
		t.Fatalf("me plan decode: %v raw=%s", err, mePlanRaw)
	}
	if own.Plan == nil {
		t.Fatal("own plan nil")
	}
	if mePlan.Code != own.Plan.Code || mePlan.DisplayName != own.Plan.DisplayName || mePlan.Status != own.Plan.Status || mePlan.Source != own.Plan.Source {
		t.Fatalf("inconsistent plan: own=%+v me=%+v", own.Plan, mePlan)
	}
}

type consistencyInspector struct{}

func (consistencyInspector) InspectAccessToken(context.Context, string) (*iamapp.AccessTokenClaims, error) {
	return &iamapp.AccessTokenClaims{Sub: "u_me", CompanyID: "c_001", MembershipID: "m_102"}, nil
}
func (consistencyInspector) InspectPreCompanyToken(context.Context, string) (*iamapp.PreCompanyTokenClaims, error) {
	return nil, nil
}

type consistencyIdentity struct{}

func (consistencyIdentity) GetByUserID(context.Context, string) (*iamapp.AuthenticatedUser, error) {
	return &iamapp.AuthenticatedUser{UserID: "u_me", LoginID: "u@x", FullName: "U"}, nil
}

type consistencyMembers struct{}

func (consistencyMembers) GetMembershipsByUser(context.Context, string) ([]caapp.MembershipView, error) {
	return []caapp.MembershipView{{
		MembershipID: "m_102", UserID: "u_me", CompanyID: "c_001", CompanyCode: "c001", CompanyName: "Company X", Status: "active",
	}}, nil
}
func (consistencyMembers) GetActiveMembership(context.Context, string, string) (*caapp.MembershipView, error) {
	return nil, nil
}
func (consistencyMembers) GetMembershipRoles(context.Context, string) ([]string, error) {
	return nil, nil
}
func (consistencyMembers) GetMembershipDepartments(context.Context, string) ([]caapp.DepartmentView, error) {
	return nil, nil
}
func (consistencyMembers) GetMembershipTitles(context.Context, string) ([]string, error) {
	return nil, nil
}
