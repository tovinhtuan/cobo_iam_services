package app

import (
	"context"
	"testing"
	"time"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

type alwaysAllowAuth struct{}

func (alwaysAllowAuth) GetEffectiveAccess(_ context.Context, membershipID, companyID string) (*authapp.EffectiveAccessSummary, error) {
	return &authapp.EffectiveAccessSummary{
		MembershipID: membershipID,
		CompanyID:    companyID,
		Permissions:  []string{"deadline.manage", "deadline.view", "rbac.manage"},
		DataScope: authapp.EffectiveDataScope{
			HasCompanyWideAccess: true,
		},
	}, nil
}

func (alwaysAllowAuth) Authorize(_ context.Context, _ authapp.AuthorizeRequest) (*authapp.AuthorizeDecision, error) {
	return &authapp.AuthorizeDecision{Decision: authapp.DecisionAllow}, nil
}

func (alwaysAllowAuth) AuthorizeBatch(_ context.Context, _ authapp.AuthorizeBatchRequest) (*authapp.AuthorizeBatchResponse, error) {
	return &authapp.AuthorizeBatchResponse{}, nil
}

// ConfirmDeadlineAlert marks alert Done without mutating disclosure record to Published.
func TestConfirmDeadlineAlert_DoesNotRequireOrCreatePublished(t *testing.T) {
	repo := &stubRepo{
		rows: []AlertRow{{
			CompanyID: "c_001", RecordID: "r-inprog", Title: "In progress alert",
			RecordStatus: "In Progress", PlannedDate: "2026-09-25",
		}},
	}
	svc := NewService(repo, alwaysAllowAuth{}, disclosureapp.NewDeadlineCalculator(disclosureapp.NewHolidayCalendarFileProvider("configs/non_trading_days")))
	svc.(*service).now = func() time.Time { return time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC) }

	resp, err := svc.ConfirmDeadlineAlert(context.Background(), ConfirmDeadlineAlertRequest{
		Subject:        Subject{UserID: "u1", MembershipID: "m_admin_001", CompanyID: "c_001"},
		RecordID:       "r-inprog",
		IdempotencyKey: "idem-complete-1",
	})
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if resp.RecordID != "r-inprog" {
		t.Fatalf("record_id=%q", resp.RecordID)
	}

	// After confirm, list presentation must be DONE even when record stays In Progress (not Published).
	confirmedAt := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	repo.rows[0].ConfirmedAt = &confirmedAt
	repo.rows[0].ConfirmedBy = "m_admin_001"

	list, err := svc.ListDeadlineAlerts(context.Background(), ListDeadlineAlertsRequest{
		Subject: Subject{UserID: "u1", MembershipID: "m_admin_001", CompanyID: "c_001"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 || list.Items[0].Status != "DONE" {
		t.Fatalf("after confirm want DONE, got %+v", list.Items)
	}
	if repo.rows[0].RecordStatus != "In Progress" {
		t.Fatalf("confirm must not rewrite record status; got %q", repo.rows[0].RecordStatus)
	}
}

func TestIsTerminalRecordStatus_InProgressNotTerminal(t *testing.T) {
	if isTerminalRecordStatus("In Progress") || isTerminalRecordStatus("PendingReview") {
		t.Fatal("in-progress statuses must not be terminal")
	}
	if !isTerminalRecordStatus("Published") || !isTerminalRecordStatus("Completed") {
		t.Fatal("Published/Completed remain terminal for PENDING_CONFIRM derivation")
	}
}
