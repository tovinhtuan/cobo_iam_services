package http

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	"github.com/cobo/cobo_iam_services/internal/subscription/companyplan"
)

type meFakeMembersMixed struct{}

func (meFakeMembersMixed) GetMembershipsByUser(context.Context, string) ([]caapp.MembershipView, error) {
	return []caapp.MembershipView{
		{MembershipID: "m1", UserID: "u_me", CompanyID: "c_001", CompanyCode: "c001", CompanyName: "Premium Co", Status: "active"},
		{MembershipID: "m2", UserID: "u_me", CompanyID: "c_002", CompanyCode: "c002", CompanyName: "Free Co", Status: "active"},
	}, nil
}
func (meFakeMembersMixed) GetActiveMembership(context.Context, string, string) (*caapp.MembershipView, error) {
	return nil, nil
}
func (meFakeMembersMixed) GetMembershipRoles(context.Context, string) ([]string, error) {
	return nil, nil
}
func (meFakeMembersMixed) GetMembershipDepartments(context.Context, string) ([]caapp.DepartmentView, error) {
	return nil, nil
}
func (meFakeMembersMixed) GetMembershipTitles(context.Context, string) ([]string, error) {
	return nil, nil
}

type meFakeMembersEmpty struct{}

func (meFakeMembersEmpty) GetMembershipsByUser(context.Context, string) ([]caapp.MembershipView, error) {
	return nil, nil
}
func (meFakeMembersEmpty) GetActiveMembership(context.Context, string, string) (*caapp.MembershipView, error) {
	return nil, nil
}
func (meFakeMembersEmpty) GetMembershipRoles(context.Context, string) ([]string, error) {
	return nil, nil
}
func (meFakeMembersEmpty) GetMembershipDepartments(context.Context, string) ([]caapp.DepartmentView, error) {
	return nil, nil
}
func (meFakeMembersEmpty) GetMembershipTitles(context.Context, string) ([]string, error) {
	return nil, nil
}

type countingPlanReader struct {
	inner  companyplan.Reader
	batch  atomic.Int64
	single atomic.Int64
}

func (c *countingPlanReader) GetEffectivePlan(ctx context.Context, companyID string, at time.Time) (*companyplan.CompanyPlan, error) {
	c.single.Add(1)
	return c.inner.GetEffectivePlan(ctx, companyID, at)
}
func (c *countingPlanReader) GetEffectivePlans(ctx context.Context, companyIDs []string, at time.Time) (map[string]*companyplan.CompanyPlan, error) {
	c.batch.Add(1)
	return c.inner.GetEffectivePlans(ctx, companyIDs, at)
}

type errPlanReader struct{ err error }

func (e errPlanReader) GetEffectivePlan(context.Context, string, time.Time) (*companyplan.CompanyPlan, error) {
	return nil, e.err
}
func (e errPlanReader) GetEffectivePlans(context.Context, []string, time.Time) (map[string]*companyplan.CompanyPlan, error) {
	return nil, e.err
}

func fixedMePlanAt() time.Time {
	return time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
}

func TestMeHandler_GETCompanies_Plan_MixedBatchNoN1(t *testing.T) {
	plans := companyplan.NewMemoryRepository()
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	_ = plans.Create(context.Background(), companyplan.CompanyPlan{
		ID: "p1", CompanyID: "c_001", Code: companyplan.PlanCodePremium, Status: companyplan.PlanStatusActive,
		EffectiveFrom: from, Origin: companyplan.RecordOriginDevFixture,
	})
	// c_003 has premium but is NOT in memberships — must not appear / leak
	_ = plans.Create(context.Background(), companyplan.CompanyPlan{
		ID: "p3", CompanyID: "c_003", Code: companyplan.PlanCodePremium, Status: companyplan.PlanStatusActive,
		EffectiveFrom: from, Origin: companyplan.RecordOriginDevFixture,
	})
	counter := &countingPlanReader{inner: companyplan.NewService(plans)}

	base := NewHandler(slog.Default(), nil, avatarFakeInspector{claims: &iamapp.AccessTokenClaims{Sub: "u_me"}}, nil, nil, nil, nil)
	me := NewMeHandler(base, meFakeIdentity{}, meFakeMembersMixed{}, nil, avatarProfileRepo{status: "active"}, nil, nil, nil, nil, "http://api.test")
	me.WithCompanyPlanReader(counter)
	me.WithCompanyPlanNow(fixedMePlanAt)

	mux := http.NewServeMux()
	me.Register(mux)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/companies", nil)
	req.Header.Set("Authorization", "Bearer test")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if counter.batch.Load() != 1 || counter.single.Load() != 0 {
		t.Fatalf("want 1 batch / 0 single, got batch=%d single=%d", counter.batch.Load(), counter.single.Load())
	}

	var out struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Items) != 2 {
		t.Fatalf("items=%d", len(out.Items))
	}
	byID := map[string]map[string]any{}
	for _, it := range out.Items {
		byID[it["company_id"].(string)] = it
	}
	p1, ok := byID["c_001"]["plan"].(map[string]any)
	if !ok || p1["code"] != "PREMIUM" || p1["status"] != "ACTIVE" {
		t.Fatalf("c_001 plan=%v", byID["c_001"]["plan"])
	}
	if byID["c_002"]["plan"] != nil {
		t.Fatalf("c_002 want plan null, got %v", byID["c_002"]["plan"])
	}
	if _, leak := byID["c_003"]; leak {
		t.Fatal("must not include non-membership company")
	}
}

func TestMeHandler_GETCompanies_Plan_EmptyMembershipsNoReaderCall(t *testing.T) {
	counter := &countingPlanReader{inner: companyplan.NewService(companyplan.NewMemoryRepository())}
	base := NewHandler(slog.Default(), nil, avatarFakeInspector{claims: &iamapp.AccessTokenClaims{Sub: "u_me"}}, nil, nil, nil, nil)
	me := NewMeHandler(base, meFakeIdentity{}, meFakeMembersEmpty{}, nil, avatarProfileRepo{status: "active"}, nil, nil, nil, nil, "http://api.test")
	me.WithCompanyPlanReader(counter)

	mux := http.NewServeMux()
	me.Register(mux)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/companies", nil)
	req.Header.Set("Authorization", "Bearer test")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if counter.batch.Load() != 0 {
		t.Fatalf("empty memberships must not call batch reader, calls=%d", counter.batch.Load())
	}
	var out struct {
		Items []any `json:"items"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if len(out.Items) != 0 {
		t.Fatalf("items=%v", out.Items)
	}
}

func TestMeHandler_GETCompanies_Plan_ReaderErrorStrict(t *testing.T) {
	base := NewHandler(slog.Default(), nil, avatarFakeInspector{claims: &iamapp.AccessTokenClaims{Sub: "u_me"}}, nil, nil, nil, nil)
	me := NewMeHandler(base, meFakeIdentity{}, meFakeMembersMixed{}, nil, avatarProfileRepo{status: "active"}, nil, nil, nil, nil, "http://api.test")
	me.WithCompanyPlanReader(errPlanReader{err: errors.New("db_down")})

	mux := http.NewServeMux()
	me.Register(mux)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/companies", nil)
	req.Header.Set("Authorization", "Bearer test")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("STRICT status=%d want 500 body=%s", rec.Code, rec.Body.String())
	}
}

func TestMeHandler_GETCompanies_IncludesCompanyCode_StillHasPlanKey(t *testing.T) {
	base := NewHandler(slog.Default(), nil, avatarFakeInspector{claims: &iamapp.AccessTokenClaims{Sub: "u_me"}}, nil, nil, nil, nil)
	me := NewMeHandler(base, meFakeIdentity{}, meFakeMembersWithCode{}, nil, avatarProfileRepo{status: "active"}, nil, nil, nil, nil, "http://api.test")
	me.WithCompanyPlanReader(companyplan.NewService(companyplan.NewMemoryRepository()))
	me.WithCompanyPlanNow(fixedMePlanAt)

	mux := http.NewServeMux()
	me.Register(mux)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/companies", nil)
	req.Header.Set("Authorization", "Bearer test")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var out struct {
		Items []map[string]any `json:"items"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if len(out.Items) != 1 {
		t.Fatal(out.Items)
	}
	if _, ok := out.Items[0]["plan"]; !ok {
		t.Fatal("plan key must always exist")
	}
	if out.Items[0]["plan"] != nil {
		t.Fatalf("want null plan, got %v", out.Items[0]["plan"])
	}
}
