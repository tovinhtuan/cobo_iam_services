package mysql

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// RP-07: legacyPolicy falls back to system.settings for an action it does not know, so an
// admin action string without its own case silently gets the wrong gate. Every action
// companyaccess passes to authorize must be listed here with the permission it requires.
var adminActionPolicy = map[string]string{
	"admin.account.settings.read":         "rbac.manage",
	"admin.account.settings.update":       "rbac.manage",
	"admin.membership.create":             "admin.membership.invite",
	"admin.membership.delete":             "rbac.manage",
	"admin.membership.department.assign":  "rbac.manage",
	"admin.membership.department.remove":  "rbac.manage",
	"admin.membership.role.assign":        "rbac.manage",
	"admin.membership.role.remove":        "rbac.manage",
	"admin.membership.title.assign":       "rbac.manage",
	"admin.membership.title.remove":       "rbac.manage",
	"admin.membership.update":             "rbac.manage",
	"admin.notification_rule.create":      "rbac.manage",
	"admin.notification_rule.delete":      "rbac.manage",
	"admin.notification_rule.list":        "rbac.manage",
	"admin.notification_rule.update":      "rbac.manage",
	"admin.permissions.list":              "rbac.manage",
	"admin.resource_scope_rule.create":    "rbac.manage",
	"admin.role.permission.assign":        "rbac.manage",
	"admin.role.permission.remove":        "rbac.manage",
	"admin.roles.list":                    "rbac.manage",
	"admin.workflow_assignee_rule.create": "rbac.manage",
	"company.edit":                        "company.edit",
	"company.view":                        "company.view",
}

func TestLegacyPolicy_AdminActions(t *testing.T) {
	for action, want := range adminActionPolicy {
		if got := legacyPolicy(action).RequiredPermission; got != want {
			t.Errorf("legacyPolicy(%q) requires %q, want %q", action, got, want)
		}
	}
}

var authorizeActionLiteral = regexp.MustCompile(`s\.authorize\(ctx, [A-Za-z.]+, "([a-z_.]+)"`)

func TestLegacyPolicy_CoversEveryCompanyAccessAction(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "..", "companyaccess", "app", "*.go"))
	if err != nil || len(files) == 0 {
		t.Fatalf("companyaccess sources not found: %v", err)
	}
	seen := map[string]bool{}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range authorizeActionLiteral.FindAllStringSubmatch(string(src), -1) {
			seen[m[1]] = true
		}
	}
	if len(seen) < len(adminActionPolicy)/2 {
		t.Fatalf("found only %d authorize actions; the source scan is broken", len(seen))
	}
	var missing []string
	for action := range seen {
		if _, ok := adminActionPolicy[action]; !ok {
			missing = append(missing, action)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("give these actions a case in legacyPolicy and list them in adminActionPolicy: %v", missing)
	}
}
