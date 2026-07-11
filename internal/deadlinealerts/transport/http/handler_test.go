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

func (fakeSvc) ListDeadlineAlertFilterOptions(_ context.Context, _ deadlinealertsapp.Subject) (*deadlinealertsapp.DeadlineAlertFilterOptionsResponse, error) {
	return &deadlinealertsapp.DeadlineAlertFilterOptionsResponse{
		Departments:  []deadlinealertsapp.DeadlineAlertFilterOptionDTO{{ID: "d1", Name: "Pháp chế"}},
		ReportGroups: []deadlinealertsapp.DeadlineAlertFilterOptionDTO{{ID: "periodic", Name: "Thông tin định kỳ"}},
	}, nil
}

func (fakeSvc) ConfirmDeadlineAlert(_ context.Context, req deadlinealertsapp.ConfirmDeadlineAlertRequest) (*deadlinealertsapp.ConfirmDeadlineAlertResponse, error) {
	return &deadlinealertsapp.ConfirmDeadlineAlertResponse{
		RecordID:    req.RecordID,
		CompanyID:   req.Subject.CompanyID,
		ConfirmedBy: req.Subject.MembershipID,
		ConfirmedAt: "2026-05-26T00:00:00Z",
	}, nil
}

func (fakeSvc) ListDeadlineSteps(_ context.Context, _ deadlinealertsapp.Subject, recordID string) (*deadlinealertsapp.ListDeadlineStepsResponse, error) {
	return &deadlinealertsapp.ListDeadlineStepsResponse{RecordID: recordID, Steps: []deadlinealertsapp.DeadlineStepDTO{}}, nil
}

func (fakeSvc) CompleteDeadlineStep(_ context.Context, req deadlinealertsapp.CompleteStepRequest) (*deadlinealertsapp.ListDeadlineStepsResponse, error) {
	return &deadlinealertsapp.ListDeadlineStepsResponse{RecordID: req.RecordID, Steps: []deadlinealertsapp.DeadlineStepDTO{}}, nil
}

func (fakeSvc) MarkDeadlineStepIncomplete(_ context.Context, req deadlinealertsapp.MarkIncompleteStepRequest) (*deadlinealertsapp.ListDeadlineStepsResponse, error) {
	return &deadlinealertsapp.ListDeadlineStepsResponse{RecordID: req.RecordID, Steps: []deadlinealertsapp.DeadlineStepDTO{}}, nil
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

func TestConfirmDeadlineAlert_route(t *testing.T) {
	mux := http.NewServeMux()
	h := NewHandler(nil, fakeSvc{}, fakeInspector{})
	h.Register(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/company/deadline-alerts/r1/confirm", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var resp deadlinealertsapp.ConfirmDeadlineAlertResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.RecordID != "r1" || resp.CompanyID != "c1" {
		t.Fatalf("got %+v", resp)
	}
}
