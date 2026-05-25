package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	deadlinealertsapp "github.com/cobo/cobo_iam_services/internal/deadlinealerts/app"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
)

type fakeInspector struct{}

func (fakeInspector) InspectAccessToken(_ context.Context, _ string) (*iamapp.AccessTokenClaims, error) {
	return &iamapp.AccessTokenClaims{Sub: "u1", MembershipID: "m1", CompanyID: "c1"}, nil
}

func (fakeInspector) InspectPreCompanyToken(context.Context, string) (*iamapp.PreCompanyTokenClaims, error) {
	return nil, nil
}

type fakeSvc struct{}

func (fakeSvc) ListDeadlineAlerts(_ context.Context, _ deadlinealertsapp.ListDeadlineAlertsRequest) (*deadlinealertsapp.ListDeadlineAlertsResponse, error) {
	return &deadlinealertsapp.ListDeadlineAlertsResponse{
		Items: []deadlinealertsapp.DeadlineAlertDTO{
			{AlertID: "r1", RecordID: "r1", Title: "Alert", DueDate: "2026-06-01", Status: "UPCOMING"},
		},
		Page: 1, PageSize: 20, Total: 1,
	}, nil
}

func TestListDeadlineAlerts_route(t *testing.T) {
	mux := http.NewServeMux()
	h := NewHandler(nil, fakeSvc{}, fakeInspector{})
	h.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/company/deadline-alerts?page=1", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var resp deadlinealertsapp.ListDeadlineAlertsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Total != 1 || resp.Items[0].RecordID != "r1" {
		t.Fatalf("got %+v", resp)
	}
}
