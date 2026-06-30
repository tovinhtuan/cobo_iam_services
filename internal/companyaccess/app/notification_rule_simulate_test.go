package app_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

type stubDispatchSimulator struct {
	lastInput caapp.NotificationDispatchSimulateInput
	result    *caapp.NotificationDispatchSimulateResult
	err       error
}

func (s *stubDispatchSimulator) SimulateDispatch(ctx context.Context, in caapp.NotificationDispatchSimulateInput) (*caapp.NotificationDispatchSimulateResult, error) {
	s.lastInput = in
	if s.result != nil {
		return s.result, s.err
	}
	return &caapp.NotificationDispatchSimulateResult{
		SimulationID: in.SimulationID,
		WouldSend:    true,
		Outcome:      "WOULD_SEND",
		Channel:      "email",
		DispatchPath: "notification_rules_consumer",
		EvaluatedAt:  time.Now().UTC(),
		Trace:        []caapp.NotificationDispatchTraceStep{{Step: "decision", Status: "pass", Detail: "WOULD_SEND"}},
	}, s.err
}

func TestAdminService_SimulateNotificationRule_ForbidsCompanyID(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	svc := caapp.NewAdminService(repo, fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"admin.notification_rule.list"}}, fixedIDGen("sim-1"), caapp.WithDispatchSimulator(&stubDispatchSimulator{}))
	sub := caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"}
	_, err := svc.SimulateNotificationRule(context.Background(), caapp.SimulateNotificationRuleRequest{
		Subject: sub,
		Body: caapp.SimulateNotificationRuleBody{
			EventType:   "deadline",
			DueAt:       "2026-07-07T00:00:00Z",
			ScheduledAt: "2026-06-30T00:00:00Z",
			ScopeType:   "DISCLOSURE",
			ScopeID:     "disc-1",
		},
	}, map[string]any{"company_id": "other-co"})
	if err == nil {
		t.Fatal("expected error for company_id override")
	}
	he, ok := err.(*perr.HTTPError)
	if !ok || he.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("err=%v", err)
	}
}

func TestAdminService_SimulateNotificationRule_PermissionDenied(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	svc := caapp.NewAdminService(repo, fakeAuthService{decision: authapp.DecisionDeny}, fixedIDGen("sim-2"), caapp.WithDispatchSimulator(&stubDispatchSimulator{}))
	sub := caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"}
	_, err := svc.SimulateNotificationRule(context.Background(), caapp.SimulateNotificationRuleRequest{
		Subject: sub,
		Body: caapp.SimulateNotificationRuleBody{
			EventType:   "deadline",
			DueAt:       "2026-07-07T00:00:00Z",
			ScheduledAt: "2026-06-30T00:00:00Z",
			ScopeType:   "DISCLOSURE",
			ScopeID:     "disc-1",
		},
	}, nil)
	if err == nil {
		t.Fatal("expected forbidden")
	}
	he, ok := err.(*perr.HTTPError)
	if !ok || he.HTTPStatus != http.StatusForbidden {
		t.Fatalf("err=%v", err)
	}
}

func TestAdminService_SimulateNotificationRule_PassesTenantCompany(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	stub := &stubDispatchSimulator{}
	svc := caapp.NewAdminService(repo, fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"admin.notification_rule.list"}}, fixedIDGen("sim-3"), caapp.WithDispatchSimulator(stub))
	sub := caapp.AdminSubject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"}
	_, err := svc.SimulateNotificationRule(context.Background(), caapp.SimulateNotificationRuleRequest{
		Subject: sub,
		Body: caapp.SimulateNotificationRuleBody{
			EventType:   "deadline",
			DueAt:       "2026-07-07T00:00:00Z",
			ScheduledAt: "2026-06-30T00:00:00Z",
			ScopeType:   "DISCLOSURE",
			ScopeID:     "disc-1",
		},
	}, nil)
	if err != nil {
		t.Fatalf("SimulateNotificationRule: %v", err)
	}
	if stub.lastInput.CompanyID != "c1" {
		t.Fatalf("company_id=%q", stub.lastInput.CompanyID)
	}
}
