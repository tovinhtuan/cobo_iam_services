package http

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	portaldashboardapp "github.com/cobo/cobo_iam_services/internal/portaldashboard/app"
	"github.com/cobo/cobo_iam_services/internal/portaldashboard/domain"
)

type fakeOverviewSvc struct {
	lastSub portaldashboardapp.Subject
	err     error
	resp    *domain.OverviewResponse
}

func (f *fakeOverviewSvc) GetOverview(_ context.Context, sub portaldashboardapp.Subject, _ domain.ParseRangeInput) (*domain.OverviewResponse, error) {
	f.lastSub = sub
	if f.err != nil {
		return nil, f.err
	}
	if f.resp != nil {
		return f.resp, nil
	}
	return &domain.OverviewResponse{
		Company:       domain.CompanyBrief{ID: sub.CompanyID},
		Range:         domain.RangeInfo{Preset: "30d"},
		LastUpdatedAt: time.Now().UTC().Format(time.RFC3339),
		Kpis:          map[string]domain.KpiMetric{},
		DeadlineHealth: domain.DeadlineHealthBlock{
			OnTimeRate:        domain.KpiMetric{Accuracy: portaldashboardapp.AccuracyUnavailable},
			OverdueAgeBuckets: []domain.OverdueBucket{},
		},
		ImmediateActions:  []domain.ImmediateActionItem{},
		FrequentLateFlows: []domain.WorkflowRiskRow{},
		DepartmentRisks:   []domain.DepartmentRiskRow{},
		RecentActivities:  []domain.RecentActivityItem{},
		Exceptions:        []domain.ExceptionItem{},
		Meta:              domain.MetaBlock{Sources: []string{}, Warnings: []string{}},
	}, nil
}

type fakeInspector struct{}

func (fakeInspector) InspectAccessToken(context.Context, string) (*iamapp.AccessTokenClaims, error) {
	return &iamapp.AccessTokenClaims{Sub: "u1", MembershipID: "m1", CompanyID: "c1"}, nil
}

func (fakeInspector) InspectPreCompanyToken(context.Context, string) (*iamapp.PreCompanyTokenClaims, error) {
	return nil, nil
}

func TestGetOverview_OK(t *testing.T) {
	svc := &fakeOverviewSvc{}
	h := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), svc, fakeInspector{})
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/company/dashboard/overview?range=30d", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if svc.lastSub.CompanyID != "c1" {
		t.Fatalf("sub: %+v", svc.lastSub)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["kpis"]; !ok {
		t.Fatalf("missing kpis: %v", body)
	}
}
