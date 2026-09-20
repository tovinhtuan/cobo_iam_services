package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
)

// detailAuthStub separates empty ListRows (alert list window) from detail row lookup.
type detailAuthStub struct {
	stubRepo
	detailRow *AlertRow
}

func (s *detailAuthStub) ListRows(_ context.Context, _ string, _ DeadlineAlertAccessScope) ([]AlertRow, error) {
	return nil, nil
}

func (s *detailAuthStub) HasDisclosureRecord(_ context.Context, _, recordID string) (bool, error) {
	return s.detailRow != nil && s.detailRow.RecordID == recordID, nil
}

func (s *detailAuthStub) GetAlertRowByRecordID(_ context.Context, _, recordID, _ string) (*AlertRow, error) {
	if s.detailRow != nil && s.detailRow.RecordID == recordID {
		row := *s.detailRow
		return &row, nil
	}
	return nil, nil
}

func (s *detailAuthStub) GetWorkflowInstanceByRecord(_ context.Context, _, recordID string) (*WorkflowInstanceRow, error) {
	if s.detailRow == nil || s.detailRow.RecordID != recordID {
		return nil, nil
	}
	snap, _ := json.Marshal([]workflowapp.StepSnapshot{{
		StepID: "step-001", StepCode: "step-001", DisplayOrder: 1, Department: "d_legal", ProcessingDays: 1,
	}})
	return &WorkflowInstanceRow{
		WorkflowInstanceID: "wi-1",
		T0Date:             time.Date(2026, 9, 4, 0, 0, 0, 0, time.UTC),
		Timezone:           "Asia/Ho_Chi_Minh",
		SnapshotJSON:       snap,
	}, nil
}

func TestListDeadlineSteps_materializedRecordOutsideListWindow_allowedWhenInScope(t *testing.T) {
	repo := &detailAuthStub{
		detailRow: &AlertRow{
			CompanyID:          "c_001",
			RecordID:           "rec-adhoc",
			TypeID:             "qa-irregular",
			Title:              "Materialized irregular",
			RecordStatus:       "submitted",
			RecordDepartmentID: "d_legal",
			PlannedDate:        "2026-09-20",
			WorkflowInstanceID: "wi-1",
			CurrentStepCode:    "step-001",
		},
	}
	svc := NewService(repo, &scopedDeadlineAuth{departments: []string{"d_legal"}}, disclosureapp.NewDeadlineCalculator(disclosureapp.NewHolidayCalendarFileProvider("configs/non_trading_days")))

	resp, err := svc.ListDeadlineSteps(context.Background(), Subject{
		UserID: "u1", MembershipID: "m_102", CompanyID: "c_001",
	}, "rec-adhoc")
	if err != nil {
		t.Fatalf("expected detail access for in-scope materialized record, got %v", err)
	}
	if resp == nil || resp.RecordID != "rec-adhoc" {
		t.Fatalf("unexpected resp %+v", resp)
	}
}

func TestListDeadlineSteps_wrongDepartment_dataScopeDenied(t *testing.T) {
	repo := &detailAuthStub{
		detailRow: &AlertRow{
			CompanyID:          "c_001",
			RecordID:           "rec-ir",
			RecordStatus:       "submitted",
			RecordDepartmentID: "d_ir",
			WorkflowInstanceID: "wi-1",
			CurrentStepCode:    "step-001",
		},
	}
	svc := NewService(repo, &scopedDeadlineAuth{departments: []string{"d_legal"}}, disclosureapp.NewDeadlineCalculator(disclosureapp.NewHolidayCalendarFileProvider("configs/non_trading_days")))

	_, err := svc.ListDeadlineSteps(context.Background(), Subject{
		UserID: "u1", MembershipID: "m_scoped", CompanyID: "c_001",
	}, "rec-ir")
	if err == nil {
		t.Fatal("expected DATA_SCOPE_DENIED")
	}
	var httpErr *perr.HTTPError
	if !errors.As(err, &httpErr) || httpErr.HTTPStatus != http.StatusForbidden || httpErr.Code != perr.CodeDataScopeDenied {
		t.Fatalf("want DATA_SCOPE_DENIED, got %v", err)
	}
}

type scopedDeadlineAuth struct {
	departments []string
}

func (a *scopedDeadlineAuth) GetEffectiveAccess(_ context.Context, membershipID, companyID string) (*authapp.EffectiveAccessSummary, error) {
	depts := make([]authapp.DepartmentScope, 0, len(a.departments))
	for _, id := range a.departments {
		depts = append(depts, authapp.DepartmentScope{DepartmentID: id})
	}
	return &authapp.EffectiveAccessSummary{
		CompanyID:    companyID,
		MembershipID: membershipID,
		Permissions:  []string{"deadline.view"},
		DataScope: authapp.EffectiveDataScope{
			Departments:          depts,
			HasCompanyWideAccess: false,
		},
	}, nil
}

func (a *scopedDeadlineAuth) Authorize(_ context.Context, _ authapp.AuthorizeRequest) (*authapp.AuthorizeDecision, error) {
	return &authapp.AuthorizeDecision{Decision: authapp.DecisionAllow}, nil
}

func (a *scopedDeadlineAuth) AuthorizeBatch(_ context.Context, _ authapp.AuthorizeBatchRequest) (*authapp.AuthorizeBatchResponse, error) {
	return &authapp.AuthorizeBatchResponse{}, nil
}
