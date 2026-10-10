package app_test

import (
	"context"
	"testing"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	iaminmem "github.com/cobo/cobo_iam_services/internal/iam/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// H3: a session whose membership was deactivated (or removed) after login must not get
// new tokens from refresh.
func TestRefresh_FailsWhenMembershipIsNoLongerActive(t *testing.T) {
	for _, status := range []string{"inactive", "suspended", ""} {
		t.Run("status="+status, func(t *testing.T) {
			ctx := context.Background()
			id := &testSeqID{}
			members := &cainmem.MembershipQueryService{ByUser: map[string][]caapp.MembershipView{
				"u_single": {{MembershipID: "m_010", UserID: "u_single", CompanyID: "c_010", CompanyName: "Solo", Status: "active"}},
			}}
			svc := iamapp.NewService(testCred(), iaminmem.NewSessionRepository(), iaminmem.NewTokenManager(id), members, id)
			login, err := svc.Login(ctx, iamapp.LoginRequest{LoginID: "single@example.com", Password: "secret"})
			if err != nil {
				t.Fatal(err)
			}
			if status == "" {
				members.ByUser["u_single"] = nil // membership removed
			} else {
				members.ByUser["u_single"][0].Status = status
			}

			_, err = svc.Refresh(ctx, iamapp.RefreshRequest{RefreshToken: login.Session.RefreshToken})
			he, ok := perr.AsHTTPError(err)
			if !ok || he.HTTPStatus != 401 || he.Code != perr.CodeSessionExpired {
				t.Fatalf("refresh with an inactive membership: got %#v, want 401 SESSION_EXPIRED", err)
			}
		})
	}
}
