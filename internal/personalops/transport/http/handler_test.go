package http_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	personalopsapp "github.com/cobo/cobo_iam_services/internal/personalops/app"
	"github.com/cobo/cobo_iam_services/internal/personalops/domain"
	personalopshttp "github.com/cobo/cobo_iam_services/internal/personalops/transport/http"
)

type fakeInspector struct {
	claims *iamapp.AccessTokenClaims
	err    error
}

func (f fakeInspector) InspectAccessToken(context.Context, string) (*iamapp.AccessTokenClaims, error) {
	return f.claims, f.err
}
func (fakeInspector) InspectPreCompanyToken(context.Context, string) (*iamapp.PreCompanyTokenClaims, error) {
	return nil, perr.NewHTTPError(http.StatusUnauthorized, perr.CodeSessionExpired, "no", nil)
}

type fakeSvc struct {
	resp *domain.OverviewResponse
	err  error
}

func (f fakeSvc) GetOperationalOverview(context.Context, personalopsapp.Subject) (*domain.OverviewResponse, error) {
	return f.resp, f.err
}

func TestHandler_requiresAuth(t *testing.T) {
	h := personalopshttp.NewHandler(slog.Default(), fakeSvc{}, fakeInspector{err: perr.NewHTTPError(http.StatusUnauthorized, perr.CodeSessionExpired, "auth", nil)})
	mux := http.NewServeMux()
	h.Register(mux)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/operational-overview", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandler_okEmptyArrays(t *testing.T) {
	zero := 0
	h := personalopshttp.NewHandler(slog.Default(), fakeSvc{resp: &domain.OverviewResponse{
		Profile: domain.ProfileBrief{UserID: "u1", DisplayName: "A"},
		Kpis: domain.KpiBlock{
			LinkedCompanies: domain.Metric{Value: &zero, Accuracy: "exact"},
			ActiveRoles:     domain.Metric{Value: &zero, Accuracy: "exact"},
			AssignedAlerts:  domain.Metric{Value: &zero, Accuracy: "exact"},
			OverdueAlerts:   domain.Metric{Value: &zero, Accuracy: "exact"},
		},
		Meta: domain.MetaBlock{GeneratedAt: "2026-07-10T00:00:00Z"},
	}}, fakeInspector{claims: &iamapp.AccessTokenClaims{Sub: "u1", MembershipID: "m1", CompanyID: "c1"}})
	mux := http.NewServeMux()
	h.Register(mux)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/operational-overview", nil)
	req.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"company_overviews", "my_tasks", "role_assignments", "admin_scopes", "activities"} {
		v, ok := body[key].([]any)
		if !ok {
			t.Fatalf("%s not array: %#v", key, body[key])
		}
		if v == nil {
			t.Fatalf("%s nil", key)
		}
	}
}
