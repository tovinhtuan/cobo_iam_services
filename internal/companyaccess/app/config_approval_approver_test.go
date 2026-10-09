package app_test

// Who may decide a configuration approval (follow-up of ROLE-01): a member other than the
// requester, in the same company, who holds rbac.manage or system.settings. No company has to
// own system.settings for approvals to be decidable.

import (
	"context"
	"testing"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	"github.com/cobo/cobo_iam_services/internal/companyaccess/configversion"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// queuePendingApproval has the requester queue one notification-rule approval and returns its id.
func queuePendingApproval(t *testing.T, repo *cainmem.AdminRepository, requester caapp.AdminSubject, ruleID string, svc caapp.AdminService) string {
	t.Helper()
	_ = svc.UpdateNotificationRule(context.Background(), caapp.UpdateNotificationRuleRequest{
		Subject: requester, RuleID: ruleID, PayloadPatch: map[string]any{"channels": map[string]any{"sms": map[string]any{"enabled": true}}},
	})
	list, err := svc.ListConfigApprovals(context.Background(), caapp.ListConfigApprovalsRequest{Subject: requester, Status: configversion.ApprovalStatusPending})
	if err != nil || len(list.Items) == 0 {
		t.Fatalf("expected a pending approval: %v", err)
	}
	return list.Items[0].ApprovalID
}

func TestConfigApproval_DeciderPermissions(t *testing.T) {
	cases := []struct {
		name  string
		perms []string
		ok    bool
	}{
		{"rbac.manage only", []string{"rbac.manage"}, true},
		{"system.settings only", []string{"system.settings"}, true},
		{"neither permission", []string{"dashboard.view"}, false},
	}
	decisions := map[string]func(svc caapp.AdminService, sub caapp.AdminSubject, id string) error{
		"approve": func(svc caapp.AdminService, sub caapp.AdminSubject, id string) error {
			_, err := svc.ApproveConfigApproval(context.Background(), caapp.ApproveConfigApprovalRequest{Subject: sub, ApprovalID: id})
			return err
		},
		"reject": func(svc caapp.AdminService, sub caapp.AdminSubject, id string) error {
			_, err := svc.RejectConfigApproval(context.Background(), caapp.RejectConfigApprovalRequest{Subject: sub, ApprovalID: id, RejectReason: "no"})
			return err
		},
		"cancel by another member": func(svc caapp.AdminService, sub caapp.AdminSubject, id string) error {
			_, err := svc.CancelConfigApproval(context.Background(), caapp.CancelConfigApprovalRequest{Subject: sub, ApprovalID: id})
			return err
		},
	}
	for decision, decide := range decisions {
		for _, tc := range cases {
			t.Run(decision+"/"+tc.name, func(t *testing.T) {
				repo := cainmem.NewAdminRepository()
				requester := seedApprovalAdmin(t, repo)
				requesterSvc := newApprovalSvc(t, repo)
				id := queuePendingApproval(t, repo, requester, createAlertPrefsRule(t, requesterSvc, requester), requesterSvc)

				other := caapp.AdminSubject{UserID: "u_other", MembershipID: "m_other", CompanyID: requester.CompanyID}
				seedInviteScopedSubject(t, repo, other)
				err := decide(newApprovalSvc(t, repo, tc.perms...), other, id)
				if tc.ok && err != nil {
					t.Fatalf("a member with %v must be able to %s, got %v", tc.perms, decision, err)
				}
				if !tc.ok {
					requireHTTPCode(t, err, 403, perr.CodePermissionDenied)
				}
			})
		}
	}
}

func TestConfigApproval_RequesterWithRbacManageCannotApproveOwnRequest(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	requester := seedApprovalAdmin(t, repo)
	svc := newApprovalSvc(t, repo, "rbac.manage", "admin.notification_rule.update")
	id := queuePendingApproval(t, repo, requester, createAlertPrefsRule(t, svc, requester), svc)
	_, err := svc.ApproveConfigApproval(context.Background(), caapp.ApproveConfigApprovalRequest{Subject: requester, ApprovalID: id})
	requireHTTPCode(t, err, 403, perr.CodeSelfApprovalNotAllowed)
}
