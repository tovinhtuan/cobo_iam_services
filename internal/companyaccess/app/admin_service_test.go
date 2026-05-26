package app_test

import (
	"context"
	"net/http"
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

type fixedIDGen string

func (g fixedIDGen) NewUUID() string { return string(g) }

type fakeAuthService struct {
	decision authapp.Decision
	permissions []string
}

func (f fakeAuthService) Authorize(_ context.Context, req authapp.AuthorizeRequest) (*authapp.AuthorizeDecision, error) {
	if f.decision != authapp.DecisionAllow {
		return &authapp.AuthorizeDecision{Decision: f.decision}, nil
	}
	switch req.Action {
	case "admin.membership.create", "admin.membership.list":
		for _, p := range f.permissions {
			if p == "admin.membership.invite" || p == "rbac.manage" || p == "system.settings" {
				return &authapp.AuthorizeDecision{Decision: authapp.DecisionAllow}, nil
			}
		}
		return &authapp.AuthorizeDecision{Decision: authapp.DecisionDeny}, nil
	default:
		return &authapp.AuthorizeDecision{Decision: authapp.DecisionAllow}, nil
	}
}

func (f fakeAuthService) AuthorizeBatch(_ context.Context, _ authapp.AuthorizeBatchRequest) (*authapp.AuthorizeBatchResponse, error) {
	return &authapp.AuthorizeBatchResponse{}, nil
}

func (f fakeAuthService) GetEffectiveAccess(_ context.Context, _, _ string) (*authapp.EffectiveAccessSummary, error) {
	return &authapp.EffectiveAccessSummary{Permissions: f.permissions}, nil
}

func TestAdminService_CreateUser_OK(t *testing.T) {
	svc := caapp.NewAdminService(
		cainmem.NewAdminRepository(),
		fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"system.settings"}},
		fixedIDGen("u_new"),
	)

	out, err := svc.CreateUser(context.Background(), caapp.CreateUserRequest{
		Subject:  caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"},
		LoginID:  "  New.User@Example.com ",
		Password: "StrongPass123!",
		FullName: " New User ",
		Email:    "new.user@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser err=%v", err)
	}
	if out.UserID != "u_new" {
		t.Fatalf("UserID=%q want u_new", out.UserID)
	}
	if out.LoginID != "new.user@example.com" {
		t.Fatalf("LoginID=%q want lowercased", out.LoginID)
	}
	if out.AccountStatus != "active" {
		t.Fatalf("AccountStatus=%q want active", out.AccountStatus)
	}
}

func TestAdminService_CreateUser_WithOptionalMembership(t *testing.T) {
	svc := caapp.NewAdminService(
		cainmem.NewAdminRepository(),
		fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"system.settings"}},
		fixedIDGen("fixed-id"),
	)

	out, err := svc.CreateUser(context.Background(), caapp.CreateUserRequest{
		Subject:          caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"},
		LoginID:          "member.user@example.com",
		Password:         "StrongPass123!",
		FullName:         "Member User",
		CompanyID:        "c_001",
		MembershipStatus: "active",
	})
	if err != nil {
		t.Fatalf("CreateUser err=%v", err)
	}
	if out.MembershipID == "" {
		t.Fatal("expected membership_id in response")
	}
	if out.CompanyID != "c_001" {
		t.Fatalf("company_id=%q want c_001", out.CompanyID)
	}
}

func TestAdminService_CreateUser_EnterpriseAdminForcesCurrentCompany(t *testing.T) {
	svc := caapp.NewAdminService(
		cainmem.NewAdminRepository(),
		fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"system.settings"}},
		fixedIDGen("fixed-id"),
	)

	out, err := svc.CreateUser(context.Background(), caapp.CreateUserRequest{
		Subject:   caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"},
		LoginID:   "force.company@example.com",
		Password:  "StrongPass123!",
		FullName:  "Force Company",
		CompanyID: "",
	})
	if err != nil {
		t.Fatalf("CreateUser err=%v", err)
	}
	if out.CompanyID != "c_001" {
		t.Fatalf("company_id=%q want c_001", out.CompanyID)
	}
}

func TestAdminService_CreateUser_EnterpriseAdminCannotCreateOtherCompany(t *testing.T) {
	svc := caapp.NewAdminService(
		cainmem.NewAdminRepository(),
		fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"system.settings"}},
		fixedIDGen("fixed-id"),
	)

	_, err := svc.CreateUser(context.Background(), caapp.CreateUserRequest{
		Subject:   caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"},
		LoginID:   "cross.company@example.com",
		Password:  "StrongPass123!",
		FullName:  "Cross Company",
		CompanyID: "c_002",
	})
	if err == nil {
		t.Fatal("expected permission denied")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok {
		t.Fatalf("expected HTTPError got %T", err)
	}
	if he.HTTPStatus != http.StatusForbidden {
		t.Fatalf("status=%d want 403", he.HTTPStatus)
	}
}

func TestAdminService_CreateUser_WebAdminCanCreateOtherCompany(t *testing.T) {
	svc := caapp.NewAdminService(
		cainmem.NewAdminRepository(),
		fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"system.settings", "rbac.manage"}},
		fixedIDGen("fixed-id"),
	)

	out, err := svc.CreateUser(context.Background(), caapp.CreateUserRequest{
		Subject:   caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"},
		LoginID:   "web.admin.create@example.com",
		Password:  "StrongPass123!",
		FullName:  "Web Admin Create",
		CompanyID: "c_002",
	})
	if err != nil {
		t.Fatalf("CreateUser err=%v", err)
	}
	if out.CompanyID != "c_002" {
		t.Fatalf("company_id=%q want c_002", out.CompanyID)
	}
}

func TestAdminService_CreateUser_WebAdmin_NoMembershipWhenCompanyOmitted(t *testing.T) {
	svc := caapp.NewAdminService(
		cainmem.NewAdminRepository(),
		fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"system.settings", "rbac.manage"}},
		fixedIDGen("web-user-no-co"),
	)

	out, err := svc.CreateUser(context.Background(), caapp.CreateUserRequest{
		Subject:   caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"},
		LoginID:   "platform.only@example.com",
		Password:  "StrongPass123!",
		FullName:  "Platform Only",
		CompanyID: "",
	})
	if err != nil {
		t.Fatalf("CreateUser err=%v", err)
	}
	if out.UserID != "web-user-no-co" {
		t.Fatalf("UserID=%q", out.UserID)
	}
	if out.MembershipID != "" {
		t.Fatalf("expected no membership, got membership_id=%q", out.MembershipID)
	}
	if out.CompanyID != "" {
		t.Fatalf("expected no company, got company_id=%q", out.CompanyID)
	}
}

func TestAdminService_ListCompanyMemberships_ListWithoutCompany(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	svc := caapp.NewAdminService(
		repo,
		fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"system.settings", "rbac.manage"}},
		fixedIDGen("orphan-user"),
	)
	_, err := svc.CreateUser(context.Background(), caapp.CreateUserRequest{
		Subject:   caapp.AdminSubject{UserID: "adm", MembershipID: "m1", CompanyID: "c1"},
		LoginID:   "orphan@example.com",
		Password:  "StrongPass123!",
		FullName:  "Orphan",
		CompanyID: "",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	items, err := svc.ListCompanyMemberships(context.Background(), caapp.ListCompanyMembershipsRequest{
		Subject:            caapp.AdminSubject{UserID: "adm", MembershipID: "m1", CompanyID: "c1"},
		CompanyID:          "",
		ListWithoutCompany: true,
	})
	if err != nil {
		t.Fatalf("ListCompanyMemberships: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items)=%d want 1", len(items))
	}
	if items[0].UserID != "orphan-user" {
		t.Fatalf("user_id=%q", items[0].UserID)
	}
}

func TestAdminService_ListCompanyMemberships_ListWithoutCompanyDeniedWithoutRbac(t *testing.T) {
	svc := caapp.NewAdminService(
		cainmem.NewAdminRepository(),
		fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"system.settings"}},
		fixedIDGen("x"),
	)
	_, err := svc.ListCompanyMemberships(context.Background(), caapp.ListCompanyMembershipsRequest{
		Subject:            caapp.AdminSubject{UserID: "adm", MembershipID: "m1", CompanyID: "c1"},
		CompanyID:          "",
		ListWithoutCompany: true,
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAdminService_ResendUserInvitation_NoCompanyScope(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	svc := caapp.NewAdminService(
		repo,
		fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"system.settings", "rbac.manage"}},
		fixedIDGen("inv-orphan"),
	)
	out, err := svc.InviteUser(context.Background(), caapp.InviteUserRequest{
		Subject:         caapp.AdminSubject{UserID: "adm", MembershipID: "m1", CompanyID: "c1"},
		Email:           "inv.orphan@example.com",
		FullName:        "Inv Orphan",
		CompanyID:       "",
		CreatedByUserID: "adm",
	})
	if err != nil {
		t.Fatalf("InviteUser: %v", err)
	}
	if out.MembershipID != "" {
		t.Fatal("expected no membership")
	}
	if err := svc.ResendUserInvitation(context.Background(), caapp.ResendUserInvitationRequest{
		Subject:              caapp.AdminSubject{UserID: "adm", MembershipID: "m1", CompanyID: "c1"},
		UserID:               out.UserID,
		CompanyID:            "",
		ResendNoCompanyScope: true,
	}); err != nil {
		t.Fatalf("ResendUserInvitation: %v", err)
	}
}

func TestAdminService_CreateUser_Validation(t *testing.T) {
	svc := caapp.NewAdminService(
		cainmem.NewAdminRepository(),
		fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"system.settings"}},
		idgen.UUIDv7Generator{},
	)
	_, err := svc.CreateUser(context.Background(), caapp.CreateUserRequest{
		Subject:  caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"},
		LoginID:  "",
		Password: "short",
		FullName: "",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok {
		t.Fatalf("expected HTTPError got %T", err)
	}
	if he.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", he.HTTPStatus)
	}
}

func TestAdminService_InviteUser_ActiveUserNewCompanyMembership(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	svc := caapp.NewAdminService(
		repo,
		fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"system.settings", "rbac.manage"}},
		fixedIDGen("invite-second-co"),
	)

	_, err := svc.CreateUser(context.Background(), caapp.CreateUserRequest{
		Subject:          caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"},
		LoginID:          "active.two.co@example.com",
		Password:         "StrongPass123!",
		FullName:         "Active Two Co",
		CompanyID:        "c_001",
		MembershipStatus: "active",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	out, err := svc.InviteUser(context.Background(), caapp.InviteUserRequest{
		Subject:          caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"},
		Email:            "active.two.co@example.com",
		FullName:         "Active Two Co",
		CompanyID:        "c_002",
		MembershipStatus: "active",
		CreatedByUserID:  "u_admin",
	})
	if err != nil {
		t.Fatalf("InviteUser: %v", err)
	}
	if out.AccountStatus != "active" {
		t.Fatalf("AccountStatus=%q want active", out.AccountStatus)
	}
	if out.CompanyID != "c_002" {
		t.Fatalf("CompanyID=%q want c_002", out.CompanyID)
	}
	if out.MembershipID == "" {
		t.Fatal("expected new membership_id")
	}
}

func TestAdminService_InviteUser_AlreadyMemberSameCompany(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	svc := caapp.NewAdminService(
		repo,
		fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"system.settings", "rbac.manage"}},
		fixedIDGen("invite-dup"),
	)

	_, err := svc.CreateUser(context.Background(), caapp.CreateUserRequest{
		Subject:          caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"},
		LoginID:          "same.co.member@example.com",
		Password:         "StrongPass123!",
		FullName:         "Same Co",
		CompanyID:        "c_001",
		MembershipStatus: "active",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	_, err = svc.InviteUser(context.Background(), caapp.InviteUserRequest{
		Subject:          caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"},
		Email:            "same.co.member@example.com",
		FullName:         "Same Co",
		CompanyID:        "c_001",
		MembershipStatus: "active",
		CreatedByUserID:  "u_admin",
	})
	if err == nil {
		t.Fatal("expected conflict: already member of this company")
	}
}

func TestAdminService_CreateUser_Denied(t *testing.T) {
	svc := caapp.NewAdminService(
		cainmem.NewAdminRepository(),
		fakeAuthService{decision: authapp.DecisionDeny},
		idgen.UUIDv7Generator{},
	)
	_, err := svc.CreateUser(context.Background(), caapp.CreateUserRequest{
		Subject:  caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_001"},
		LoginID:  "x@example.com",
		Password: "StrongPass123!",
		FullName: "X",
	})
	if err == nil {
		t.Fatal("expected permission denied")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok {
		t.Fatalf("expected HTTPError got %T", err)
	}
	if he.HTTPStatus != http.StatusForbidden {
		t.Fatalf("status=%d want 403", he.HTTPStatus)
	}
}

func TestAdminService_NotificationRulesListPatchDelete_and_AccountSettings(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	svc := caapp.NewAdminService(
		repo,
		fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"rbac.manage", "system.settings"}},
		fixedIDGen("u_owner"),
	)
	sub := caapp.AdminSubject{UserID: "u_owner", MembershipID: "m_owner", CompanyID: "c_001"}

	_, err := svc.CreateUser(context.Background(), caapp.CreateUserRequest{
		Subject:   sub,
		LoginID:   "owner.rules@example.com",
		Password:  "StrongPass123!",
		FullName:  "Owner Rules",
		Email:     "owner.rules@example.com",
		CompanyID: "c_001",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	if err := svc.CreateNotificationRule(context.Background(), caapp.CreateNotificationRuleRequest{
		Subject: sub,
		Payload: map[string]any{
			"company_id": "c_001",
			"rule_code":  "rule.test.001",
			"channels":   []any{"email"},
		},
	}); err != nil {
		t.Fatalf("CreateNotificationRule: %v", err)
	}

	items, err := svc.ListNotificationRules(context.Background(), caapp.ListNotificationRulesRequest{Subject: sub})
	if err != nil {
		t.Fatalf("ListNotificationRules: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items)=%d want 1", len(items))
	}
	rid := items[0].NotificationRuleID
	if rid == "" {
		t.Fatal("empty notification_rule_id")
	}

	st := "inactive"
	if err := svc.UpdateNotificationRule(context.Background(), caapp.UpdateNotificationRuleRequest{
		Subject:      sub,
		RuleID:       rid,
		PayloadPatch: map[string]any{"channels": []any{"in_app"}},
		Status:       &st,
	}); err != nil {
		t.Fatalf("UpdateNotificationRule: %v", err)
	}

	if err := svc.DeleteNotificationRule(context.Background(), caapp.DeleteNotificationRuleRequest{Subject: sub, RuleID: rid}); err != nil {
		t.Fatalf("DeleteNotificationRule: %v", err)
	}
	items2, _ := svc.ListNotificationRules(context.Background(), caapp.ListNotificationRulesRequest{Subject: sub})
	if len(items2) != 0 {
		t.Fatalf("after delete len=%d want 0", len(items2))
	}

	acct, err := svc.GetAdminAccountSettings(context.Background(), caapp.GetAdminAccountSettingsRequest{Subject: sub})
	if err != nil {
		t.Fatalf("GetAdminAccountSettings: %v", err)
	}
	if acct.LoginID != "owner.rules@example.com" {
		t.Fatalf("login_id=%q", acct.LoginID)
	}
	fn := "Owner Updated"
	if err := svc.PatchAdminAccountSettings(context.Background(), caapp.PatchAdminAccountSettingsRequest{
		Subject:  sub,
		FullName: &fn,
	}); err != nil {
		t.Fatalf("PatchAdminAccountSettings: %v", err)
	}
	acct2, err := svc.GetAdminAccountSettings(context.Background(), caapp.GetAdminAccountSettingsRequest{Subject: sub})
	if err != nil {
		t.Fatal(err)
	}
	if acct2.FullName != "Owner Updated" {
		t.Fatalf("full_name=%q", acct2.FullName)
	}
}


func TestListCompanyMemberships_RolesEnriched(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	svc := caapp.NewAdminService(
		repo,
		fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"rbac.manage"}},
		fixedIDGen("u_member"),
	)
	sub := caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c_roles"}

	// Create a member in company c_roles
	out, err := svc.CreateUser(context.Background(), caapp.CreateUserRequest{
		Subject:   sub,
		LoginID:   "member@roles.test",
		Password:  "StrongPass123!",
		FullName:  "Role Member",
		CompanyID: "c_roles",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	// Assign a role to the member's membership
	roleID, err := repo.LookupRoleIDForInvite(context.Background(), "c_roles", "", "company_admin", "user_thuong")
	if err != nil {
		t.Fatalf("LookupRoleID: %v", err)
	}
	if err := repo.AddRole(context.Background(), out.MembershipID, roleID); err != nil {
		t.Fatalf("AddRole: %v", err)
	}

	// List memberships and verify Roles is enriched
	items, err := svc.ListCompanyMemberships(context.Background(), caapp.ListCompanyMembershipsRequest{
		Subject:   sub,
		CompanyID: "c_roles",
	})
	if err != nil {
		t.Fatalf("ListCompanyMemberships: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected at least 1 membership")
	}
	member := items[0]
	if len(member.Roles) == 0 {
		t.Fatalf("expected Roles to be enriched, got empty. MembershipID=%q", member.MembershipID)
	}
	if member.Roles[0].RoleID == "" {
		t.Fatal("expected Roles[0].RoleID to be non-empty")
	}
}
