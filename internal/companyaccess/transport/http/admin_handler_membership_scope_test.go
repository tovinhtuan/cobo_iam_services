package http_test

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
)

// C5 (risk review 2026-10-09): every tenant route that acts on a membership answers 404
// MEMBERSHIP_NOT_FOUND for a membership of another company.

const foreignMember = "m-foreign"

// pattern is the route exactly as registered in admin_handler.go; the placeholders are replaced by
// fixed ids when the request is sent.
type membershipRoute struct{ method, pattern, body string }

var membershipRoutes = []membershipRoute{
	{"PATCH", "/api/v1/admin/memberships/{membership_id}", `{"status":"inactive"}`},
	{"DELETE", "/api/v1/admin/memberships/{membership_id}", ``},
	{"POST", "/api/v1/admin/memberships/{membership_id}/roles", `{"role_id":"r-x"}`},
	{"DELETE", "/api/v1/admin/memberships/{membership_id}/roles/{role_id}", ``},
	{"PUT", "/api/v1/admin/memberships/{membership_id}/primary-role", `{"role_id":"r-x"}`},
	{"POST", "/api/v1/admin/memberships/{membership_id}/departments", `{"department_id":"d-x"}`},
	{"DELETE", "/api/v1/admin/memberships/{membership_id}/departments/{department_id}", ``},
	{"POST", "/api/v1/admin/memberships/{membership_id}/titles", `{"title_id":"t-x"}`},
	{"DELETE", "/api/v1/admin/memberships/{membership_id}/titles/{title_id}", ``},
	{"PUT", "/api/v1/admin/memberships/{membership_id}/org-assignments", `{"department_ids":[],"title_ids":[]}`},
	{"POST", "/api/v1/admin/memberships/{membership_id}/permissions", `{"permission_code":"template.workflow.override.read"}`},
	{"DELETE", "/api/v1/admin/memberships/{membership_id}/permissions/{permission_code}", ``},
	{"GET", "/api/v1/admin/memberships/{membership_id}/permissions", ``},
	{"POST", "/api/v1/admin/departments/{department_id}/members", `{"membership_id":"` + foreignMember + `"}`},
	{"DELETE", "/api/v1/admin/departments/{department_id}/members/{membership_id}", ``},
	{"POST", "/api/v1/admin/teams/{team_id}/members", `{"membership_id":"` + foreignMember + `","department_id":"d-x"}`},
	{"DELETE", "/api/v1/admin/teams/{team_id}/members/{membership_id}", ``},
	{"POST", "/api/v1/admin/titles/{title_id}/members", `{"membership_id":"` + foreignMember + `"}`},
	{"DELETE", "/api/v1/admin/titles/{title_id}/members/{membership_id}", ``},
	{"POST", "/api/v1/admin/company/admins", `{"membership_id":"` + foreignMember + `"}`},
	{"DELETE", "/api/v1/admin/company/admins/{membership_id}", ``},
	{"POST", "/api/v1/admin/company/transfer-ownership", `{"target_membership_id":"` + foreignMember + `"}`},
	{"POST", "/api/v1/admin/config-approvals", `{"change_type":"rbac.direct_permission.remove","proposed":{"membership_id":"` + foreignMember + `","permission_code":"template.workflow.override.read"}}`},
}

var routeIDs = strings.NewReplacer(
	"{membership_id}", foreignMember,
	"{role_id}", "r-x",
	"{department_id}", "d-x",
	"{title_id}", "t-x",
	"{team_id}", "team-x",
	"{permission_code}", "template.workflow.override.read",
)

func TestTenantRoutes_ForeignMembership_NotFound(t *testing.T) {
	mux, repo := newScopeHandlerMux(t, scopeTenantPerms)
	if _, err := repo.CreateUser(context.Background(), caapp.UserView{
		UserID: "u-foreign", LoginID: "foreign@example.com", FullName: "Foreign", AccountStatus: "active",
	}, "hash", caapp.CreateUserOptions{MembershipID: foreignMember, CompanyID: "c-other", MembershipStatus: "active"}); err != nil {
		t.Fatalf("seed foreign member: %v", err)
	}
	// transfer-ownership only reaches the target check for the primary admin.
	if err := repo.SetMembershipPrimaryAdmin(context.Background(), "m-1"); err != nil {
		t.Fatal(err)
	}
	for _, rt := range membershipRoutes {
		w := scopeCall(mux, rt.method, routeIDs.Replace(rt.pattern), rt.body)
		if w.Code != http.StatusNotFound || errorCode(t, w) != "MEMBERSHIP_NOT_FOUND" {
			t.Errorf("%s %s: expected 404 MEMBERSHIP_NOT_FOUND, got %d %s", rt.method, rt.pattern, w.Code, w.Body.String())
		}
	}
	if m, err := repo.GetMembershipByID(context.Background(), foreignMember); err != nil || m.Status != "active" {
		t.Fatalf("membership of another company must be untouched: %+v err=%v", m, err)
	}
}

// Every registered route with a {membership_id} path parameter must be in the table above, so a
// new route cannot slip in without a company check being exercised. Identities are compared, not
// only counts.
func TestTenantRoutes_MembershipRouteTableIsComplete(t *testing.T) {
	src, err := os.ReadFile("admin_handler.go")
	if err != nil {
		t.Fatal(err)
	}
	registered := map[string]bool{}
	for _, line := range strings.Split(string(src), "\n") {
		if !strings.Contains(line, "mux.HandleFunc(") || !strings.Contains(line, "{membership_id}") {
			continue
		}
		start := strings.Index(line, `"`)
		end := strings.Index(line[start+1:], `"`)
		if start < 0 || end < 0 {
			t.Fatalf("cannot parse route line: %s", line)
		}
		registered[line[start+1:start+1+end]] = true // "METHOD /path"
	}
	listed := map[string]bool{}
	for _, rt := range membershipRoutes {
		if strings.Contains(rt.pattern, "{membership_id}") {
			listed[rt.method+" "+rt.pattern] = true
		}
	}
	for r := range registered {
		if !listed[r] {
			t.Errorf("route %q has a {membership_id} parameter but is not in membershipRoutes: add it with its foreign-membership case", r)
		}
	}
	for r := range listed {
		if !registered[r] {
			t.Errorf("membershipRoutes lists %q but admin_handler.go does not register it", r)
		}
	}
}
