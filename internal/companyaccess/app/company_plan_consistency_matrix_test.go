package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	iamhttp "github.com/cobo/cobo_iam_services/internal/iam/transport/http"
	"github.com/cobo/cobo_iam_services/internal/subscription/companyplan"
)

type stubPlanReader struct {
	plan *companyplan.CompanyPlan
	err  error
}

func (s stubPlanReader) GetEffectivePlan(context.Context, string, time.Time) (*companyplan.CompanyPlan, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.plan == nil {
		return nil, nil
	}
	cp := *s.plan
	return &cp, nil
}
func (s stubPlanReader) GetEffectivePlans(_ context.Context, ids []string, at time.Time) (map[string]*companyplan.CompanyPlan, error) {
	if s.err != nil {
		return nil, s.err
	}
	out := map[string]*companyplan.CompanyPlan{}
	for _, id := range ids {
		if s.plan != nil && s.plan.CompanyID == id {
			cp := *s.plan
			out[id] = &cp
		}
	}
	return out, nil
}

func assertPlanEqual(t *testing.T, own *companyplan.PlanDTO, me any) {
	t.Helper()
	raw, _ := json.Marshal(me)
	if own == nil {
		if string(raw) != "null" {
			t.Fatalf("want null, got %s", raw)
		}
		return
	}
	var got companyplan.PlanDTO
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Code != own.Code || got.DisplayName != own.DisplayName || got.Status != own.Status || got.Source != own.Source {
		t.Fatalf("own=%+v me=%+v", own, got)
	}
}

func TestCompanyPlan_ConsistencyMatrix(t *testing.T) {
	at := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	exp := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name string
		plan *companyplan.CompanyPlan
		err  error
	}{
		{name: "ACTIVE_Premium", plan: &companyplan.CompanyPlan{ID: "p", CompanyID: "c_001", Code: companyplan.PlanCodePremium, Status: companyplan.PlanStatusActive, EffectiveFrom: from, ExpiresAt: &exp}},
		{name: "TRIAL", plan: &companyplan.CompanyPlan{ID: "p", CompanyID: "c_001", Code: companyplan.PlanCodePremium, Status: companyplan.PlanStatusTrial, EffectiveFrom: from, ExpiresAt: &exp}},
		{name: "EXPIRED", plan: &companyplan.CompanyPlan{ID: "p", CompanyID: "c_001", Code: companyplan.PlanCodePremium, Status: companyplan.PlanStatusExpired, EffectiveFrom: from, ExpiresAt: &exp}},
		{name: "SUSPENDED", plan: &companyplan.CompanyPlan{ID: "p", CompanyID: "c_001", Code: companyplan.PlanCodePremium, Status: companyplan.PlanStatusSuspended, EffectiveFrom: from, ExpiresAt: &exp}},
		{name: "no_plan", plan: nil},
		{name: "unknown_code", plan: &companyplan.CompanyPlan{ID: "p", CompanyID: "c_001", Code: companyplan.PlanCode("GOLD"), Status: companyplan.PlanStatusActive, EffectiveFrom: from, ExpiresAt: &exp}},
		{name: "repository_error", err: errors.New("db_down")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reader := stubPlanReader{plan: tc.plan, err: tc.err}
			adminRepo := cainmem.NewAdminRepository()
			seedCompany(adminRepo, "c_001", "Company X")
			adminSvc := caapp.NewAdminService(adminRepo, fakeAuthService{decision: authapp.DecisionAllow}, fixedIDGen("x"),
				caapp.WithCompanyPlanReader(reader),
				caapp.WithCompanyPlanNow(func() time.Time { return at }),
			)
			own, ownErr := adminSvc.GetOwnCompany(context.Background(), caapp.GetOwnCompanyRequest{
				Subject: caapp.AdminSubject{UserID: "u_me", MembershipID: "m_102", CompanyID: "c_001"},
			})

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

			if tc.err != nil {
				if ownErr == nil {
					t.Fatal("GetOwnCompany want error")
				}
				if rec.Code != http.StatusInternalServerError {
					t.Fatalf("me status=%d want 500", rec.Code)
				}
				return
			}
			if ownErr != nil {
				t.Fatal(ownErr)
			}
			if rec.Code != http.StatusOK {
				t.Fatalf("me status=%d body=%s", rec.Code, rec.Body.String())
			}
			var meOut struct {
				Items []map[string]any `json:"items"`
			}
			_ = json.Unmarshal(rec.Body.Bytes(), &meOut)
			if len(meOut.Items) != 1 {
				t.Fatalf("items=%d", len(meOut.Items))
			}
			if _, ok := meOut.Items[0]["plan"]; !ok {
				t.Fatal("plan key missing on me item")
			}
			rawOwn, _ := json.Marshal(own)
			var ownMap map[string]any
			_ = json.Unmarshal(rawOwn, &ownMap)
			if _, ok := ownMap["plan"]; !ok {
				t.Fatal("plan key missing on GetOwnCompany")
			}
			assertPlanEqual(t, own.Plan, meOut.Items[0]["plan"])
		})
	}
}
