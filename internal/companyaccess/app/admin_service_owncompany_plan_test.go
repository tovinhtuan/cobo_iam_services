package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/subscription/companyplan"
)

type planErrReader struct{ err error }

func (p planErrReader) GetEffectivePlan(context.Context, string, time.Time) (*companyplan.CompanyPlan, error) {
	return nil, p.err
}
func (p planErrReader) GetEffectivePlans(context.Context, []string, time.Time) (map[string]*companyplan.CompanyPlan, error) {
	return nil, p.err
}

func fixedPlanAt() time.Time {
	return time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
}

func seedPlan(t *testing.T, repo *companyplan.MemoryRepository, id, companyID string, code companyplan.PlanCode, status companyplan.PlanStatus) {
	t.Helper()
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := repo.Create(context.Background(), companyplan.CompanyPlan{
		ID: id, CompanyID: companyID, Code: code, Status: status,
		EffectiveFrom: from, Origin: companyplan.RecordOriginDevFixture,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestGetOwnCompany_Plan_ActivePremium(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	seedCompany(repo, "c-own", "CoBo VN")
	plans := companyplan.NewMemoryRepository()
	seedPlan(t, plans, "p1", "c-own", companyplan.PlanCodePremium, companyplan.PlanStatusActive)
	svc := caapp.NewAdminService(repo, fakeAuthService{decision: authapp.DecisionAllow}, fixedIDGen("x"),
		caapp.WithCompanyPlanReader(companyplan.NewService(plans)),
		caapp.WithCompanyPlanNow(fixedPlanAt),
	)
	out, err := svc.GetOwnCompany(context.Background(), caapp.GetOwnCompanyRequest{
		Subject: caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "c-own"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Plan == nil || out.Plan.Code != "PREMIUM" || out.Plan.Status != "ACTIVE" || out.Plan.Source != "COMPANY_SUBSCRIPTION" || out.Plan.DisplayName != "Premium" {
		t.Fatalf("plan=%+v", out.Plan)
	}
	if out.CompanyName != "CoBo VN" {
		t.Fatal("backward-compatible company_name broken")
	}
}

func TestGetOwnCompany_Plan_NonActiveStatusesPreserved(t *testing.T) {
	cases := []companyplan.PlanStatus{
		companyplan.PlanStatusTrial,
		companyplan.PlanStatusExpired,
		companyplan.PlanStatusSuspended,
	}
	for _, st := range cases {
		t.Run(string(st), func(t *testing.T) {
			repo := cainmem.NewAdminRepository()
			seedCompany(repo, "c-own", "CoBo VN")
			plans := companyplan.NewMemoryRepository()
			from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			exp := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
			p := companyplan.CompanyPlan{
				ID: "p1", CompanyID: "c-own", Code: companyplan.PlanCodePremium, Status: st,
				EffectiveFrom: from, Origin: companyplan.RecordOriginDevFixture,
			}
			if st == companyplan.PlanStatusExpired {
				past := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
				p.ExpiresAt = &past
				// expired window does not cover fixedPlanAt → plan null; seed a covering EXPIRED?
				// Contract: covering record with real status. EXPIRED covering at `at` means expires_at > at.
				// Use SUSPENDED-style covering with EXPIRED status + future expires for wire status test.
				p.ExpiresAt = &exp
			} else {
				p.ExpiresAt = &exp
			}
			if err := plans.Create(context.Background(), p); err != nil {
				t.Fatal(err)
			}
			svc := caapp.NewAdminService(repo, fakeAuthService{decision: authapp.DecisionAllow}, fixedIDGen("x"),
				caapp.WithCompanyPlanReader(companyplan.NewService(plans)),
				caapp.WithCompanyPlanNow(fixedPlanAt),
			)
			out, err := svc.GetOwnCompany(context.Background(), caapp.GetOwnCompanyRequest{
				Subject: caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "c-own"},
			})
			if err != nil {
				t.Fatal(err)
			}
			if out.Plan == nil || out.Plan.Status != string(st) {
				t.Fatalf("want status %s, got %+v", st, out.Plan)
			}
		})
	}
}

func TestGetOwnCompany_Plan_NullWhenNoRecord(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	seedCompany(repo, "c-own", "CoBo VN")
	plans := companyplan.NewMemoryRepository()
	svc := caapp.NewAdminService(repo, fakeAuthService{decision: authapp.DecisionAllow}, fixedIDGen("x"),
		caapp.WithCompanyPlanReader(companyplan.NewService(plans)),
		caapp.WithCompanyPlanNow(fixedPlanAt),
	)
	out, err := svc.GetOwnCompany(context.Background(), caapp.GetOwnCompanyRequest{
		Subject: caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "c-own"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Plan != nil {
		t.Fatalf("want plan null, got %+v", out.Plan)
	}
	raw, _ := json.Marshal(out)
	if !json.Valid(raw) || !containsJSONNullPlan(raw) {
		t.Fatalf("JSON must include plan:null, got %s", raw)
	}
}

func containsJSONNullPlan(raw []byte) bool {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return false
	}
	v, ok := m["plan"]
	return ok && v == nil
}

func TestGetOwnCompany_Plan_ReaderErrorStrict(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	seedCompany(repo, "c-own", "CoBo VN")
	svc := caapp.NewAdminService(repo, fakeAuthService{decision: authapp.DecisionAllow}, fixedIDGen("x"),
		caapp.WithCompanyPlanReader(planErrReader{err: errors.New("db_down")}),
	)
	_, err := svc.GetOwnCompany(context.Background(), caapp.GetOwnCompanyRequest{
		Subject: caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "c-own"},
	})
	if err == nil {
		t.Fatal("STRICT: expected error")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusInternalServerError {
		t.Fatalf("want 500 INTERNAL, got %v", err)
	}
}

func TestGetOwnCompany_Plan_NoCrossCompanyLeak(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	seedCompany(repo, "c-own", "Mine")
	seedCompany(repo, "c-other", "Other")
	plans := companyplan.NewMemoryRepository()
	seedPlan(t, plans, "p-other", "c-other", companyplan.PlanCodePremium, companyplan.PlanStatusActive)
	svc := caapp.NewAdminService(repo, fakeAuthService{decision: authapp.DecisionAllow}, fixedIDGen("x"),
		caapp.WithCompanyPlanReader(companyplan.NewService(plans)),
		caapp.WithCompanyPlanNow(fixedPlanAt),
	)
	out, err := svc.GetOwnCompany(context.Background(), caapp.GetOwnCompanyRequest{
		Subject: caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "c-own"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Plan != nil {
		t.Fatalf("must not leak other company plan: %+v", out.Plan)
	}
}

func TestMapCompanyPlanReadError_Strict(t *testing.T) {
	err := caapp.MapCompanyPlanReadError(errors.New("boom"))
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusInternalServerError || he.Code != perr.CodeInternal {
		t.Fatalf("got %v", err)
	}
}
