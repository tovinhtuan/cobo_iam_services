package mysql_test

// Integration tests for the RBAC matrix restore (ROLE-01) against a real MySQL 8 with all
// migrations applied. They run the admin service on top of the MySQL repository, so the SQL in
// admin_repository_rbac_restore.go and ApplyPendingApprovalInTx really executes.
//
//	docker compose -f docker-compose.dev.yml up -d mysql   # then run migrations/run_dev_migrations.sh
//	MYSQL_TEST_DSN='root:root@tcp(127.0.0.1:3306)/cobo_iam?parseTime=true&loc=UTC' \
//	  go test ./internal/companyaccess/infra/mysql/ -run Integration -count=1
//
// Skipped when MYSQL_TEST_DSN is not set. Every test seeds its own companies, users, roles and
// grants under a random prefix and removes them again.

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	"github.com/cobo/cobo_iam_services/internal/companyaccess/configversion"
	camysql "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/mysql"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	_ "github.com/go-sql-driver/mysql"
)

// itChangeTypeRollback is the change_type of a rollback queued for approval (kept as a literal so
// this file also compiles against code that predates the constant).
const itChangeTypeRollback = "rbac.matrix.rollback"

type itAuth struct{ perms []string }

func (itAuth) Authorize(context.Context, authapp.AuthorizeRequest) (*authapp.AuthorizeDecision, error) {
	return &authapp.AuthorizeDecision{Decision: authapp.DecisionAllow}, nil
}

func (itAuth) AuthorizeBatch(context.Context, authapp.AuthorizeBatchRequest) (*authapp.AuthorizeBatchResponse, error) {
	return &authapp.AuthorizeBatchResponse{}, nil
}

func (a itAuth) GetEffectiveAccess(context.Context, string, string) (*authapp.EffectiveAccessSummary, error) {
	return &authapp.EffectiveAccessSummary{Permissions: a.perms}, nil
}

type itIDGen struct {
	prefix string
	n      atomic.Int64
}

func (g *itIDGen) NewUUID() string { return fmt.Sprintf("%s-g%d", g.prefix, g.n.Add(1)) }

// Permission codes the tests rely on (all seeded by the migrations).
const (
	itPlatformView  = "platform.cms.view"                // outside the enterprise scope
	itCMSWrite      = "cms.template.write"               // outside the enterprise scope
	itDisclosure    = "disclosure.view"                  // grantable on a custom role
	itDeadline      = "deadline.view"                    // grantable on a custom role
	itDashboard     = "dashboard.view"                   // grantable on a custom role
	itCritical      = "rbac.manage"                      // critical
	itDirectA       = "template.workflow.override.read"  // in GrantablePermissions
	itDirectB       = "template.workflow.override.write" // in GrantablePermissions
	itDirectInvite  = "admin.membership.invite"          // in GrantablePermissions, critical
	itDirectNoGrant = "ad_hoc_alert.process_control"     // not in GrantablePermissions
)

type itWorld struct {
	t        *testing.T
	db       *sql.DB
	repo     *camysql.AdminRepository
	svc      caapp.AdminService
	prefix   string
	company  string
	company2 string
	owner    caapp.AdminSubject
	approver caapp.AdminSubject
	other    caapp.AdminSubject // membership of company2
	perm     map[string]string  // code -> permission_id
	custom   string
	custom2  string
	cmsOp    string
	global   string
}

func newITWorld(t *testing.T) *itWorld {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("MYSQL_TEST_DSN"))
	if dsn == "" {
		t.Skip("MYSQL_TEST_DSN not set")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open mysql: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Ping(); err != nil {
		t.Skipf("mysql not reachable: %v", err)
	}
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		t.Fatal(err)
	}
	p := "itr1" + hex.EncodeToString(buf)
	w := &itWorld{
		t: t, db: db, repo: camysql.NewAdminRepository(db), prefix: p,
		company: p + "-c1", company2: p + "-c2",
		owner:    caapp.AdminSubject{UserID: p + "-uo", MembershipID: p + "-mo", CompanyID: p + "-c1"},
		approver: caapp.AdminSubject{UserID: p + "-ua", MembershipID: p + "-ma", CompanyID: p + "-c1"},
		other:    caapp.AdminSubject{UserID: p + "-ux", MembershipID: p + "-mx", CompanyID: p + "-c2"},
		perm:     map[string]string{},
		custom:   p + "-r-custom", custom2: p + "-r-custom2", cmsOp: p + "-r-cmsop", global: p + "-r-global",
	}
	t.Cleanup(w.cleanup)
	w.seed()
	w.svc = caapp.NewAdminService(w.repo, itAuth{perms: []string{"rbac.manage", "system.settings"}}, &itIDGen{prefix: p})
	return w
}

func (w *itWorld) exec(q string, args ...any) {
	w.t.Helper()
	if _, err := w.db.ExecContext(context.Background(), q, args...); err != nil {
		w.t.Fatalf("seed %q: %v", q, err)
	}
}

func (w *itWorld) seed() {
	w.t.Helper()
	for _, code := range []string{itPlatformView, itCMSWrite, itDisclosure, itDeadline, itDashboard, itCritical} {
		var id string
		if err := w.db.QueryRow(`SELECT permission_id FROM permissions WHERE permission_code = ? AND status = 'active'`, code).Scan(&id); err != nil {
			w.t.Fatalf("permission %s is not seeded (run all migrations): %v", code, err)
		}
		w.perm[code] = id
	}
	for _, c := range []string{w.company, w.company2} {
		w.exec(`INSERT INTO companies (company_id, company_code, company_name, status, verification_status) VALUES (?, ?, ?, 'active', 'verified')`, c, c, "IT "+c)
	}
	for _, s := range []caapp.AdminSubject{w.owner, w.approver, w.other} {
		w.exec(`INSERT INTO users (user_id, login_id, full_name, account_status) VALUES (?, ?, 'IT user', 'active')`, s.UserID, s.UserID+"@example.test")
		w.exec(`INSERT INTO memberships (membership_id, user_id, company_id, membership_status) VALUES (?, ?, ?, 'active')`, s.MembershipID, s.UserID, s.CompanyID)
	}
	role := func(id, company, code, roleType string, protected int) {
		var companyArg any
		if company != "" {
			companyArg = company
		}
		w.exec(`INSERT INTO roles (role_id, company_id, role_code, role_name, status, role_type, is_protected) VALUES (?, ?, ?, ?, 'active', ?, ?)`,
			id, companyArg, code, code, roleType, protected)
	}
	role(w.global, "", w.prefix+"_global", caapp.RoleTypeSystemGlobal, 1)
	role(w.cmsOp, w.company, w.prefix+"_cms_operator", caapp.RoleTypeTenantDefault, 1)
	role(w.custom, w.company, w.prefix+"_custom", caapp.RoleTypeTenantCustom, 0)
	role(w.custom2, w.company2, w.prefix+"_custom2", caapp.RoleTypeTenantCustom, 0)
	for roleID, codes := range map[string][]string{
		w.global: {itDisclosure}, w.cmsOp: {itPlatformView, itCMSWrite, itDisclosure},
		w.custom: {itDisclosure}, w.custom2: {itDisclosure},
	} {
		for _, code := range codes {
			w.grantRole(roleID, code)
		}
	}
}

func (w *itWorld) cleanup() {
	both := []any{w.company, w.company2}
	for _, q := range []string{
		`DELETE FROM pending_admin_changes WHERE company_id IN (?, ?)`,
		`DELETE FROM notification_rule_versions WHERE company_id IN (?, ?)`,
		`DELETE FROM notification_rules WHERE company_id IN (?, ?)`,
		`DELETE FROM rbac_matrix_snapshots WHERE company_id IN (?, ?)`,
		`DELETE FROM membership_direct_permissions WHERE company_id IN (?, ?)`,
		`DELETE mr FROM membership_roles mr JOIN memberships m ON m.membership_id = mr.membership_id WHERE m.company_id IN (?, ?)`,
		`DELETE rp FROM role_permissions rp JOIN roles r ON r.role_id = rp.role_id WHERE r.company_id IN (?, ?)`,
		`DELETE FROM roles WHERE company_id IN (?, ?)`,
		`DELETE FROM departments WHERE company_id IN (?, ?)`,
		`DELETE FROM memberships WHERE company_id IN (?, ?)`,
	} {
		if _, err := w.db.Exec(q, both...); err != nil {
			w.t.Logf("cleanup %q: %v", q, err)
		}
	}
	_, _ = w.db.Exec(`DELETE rp FROM role_permissions rp WHERE rp.role_id = ?`, w.global)
	_, _ = w.db.Exec(`DELETE FROM roles WHERE role_id = ?`, w.global)
	for _, s := range []caapp.AdminSubject{w.owner, w.approver, w.other} {
		_, _ = w.db.Exec(`DELETE FROM users WHERE user_id = ?`, s.UserID)
	}
	_, _ = w.db.Exec(`DELETE FROM companies WHERE company_id IN (?, ?)`, both...)
}

func (w *itWorld) grantRole(roleID, code string) {
	w.t.Helper()
	if err := w.repo.AddRolePermission(context.Background(), roleID, w.perm[code]); err != nil {
		w.t.Fatalf("grant %s on %s: %v", code, roleID, err)
	}
}

func (w *itWorld) revokeRole(roleID, code string) {
	w.t.Helper()
	if err := w.repo.RemoveRolePermission(context.Background(), roleID, w.perm[code]); err != nil {
		w.t.Fatalf("revoke %s on %s: %v", code, roleID, err)
	}
}

// rolePerms reads the raw active permission ids of a role, as permission codes.
func (w *itWorld) rolePerms(roleID string) []string {
	w.t.Helper()
	rows, err := w.db.Query(`
		SELECT p.permission_code FROM role_permissions rp JOIN permissions p ON p.permission_id = rp.permission_id
		WHERE rp.role_id = ? AND rp.status = 'active' ORDER BY p.permission_code`, roleID)
	if err != nil {
		w.t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			w.t.Fatal(err)
		}
		out = append(out, c)
	}
	return out
}

func has(list []string, code string) bool {
	for _, c := range list {
		if c == code {
			return true
		}
	}
	return false
}

// outOfScope fingerprints everything a restore for company 1 must never change.
func (w *itWorld) outOfScope() string {
	w.t.Helper()
	var parts []string
	for _, id := range []string{w.global, w.cmsOp, w.custom2} {
		parts = append(parts, id+"="+strings.Join(w.rolePerms(id), ","))
	}
	return strings.Join(parts, ";")
}

func (w *itWorld) activeDirect(membershipID string) map[string]int {
	w.t.Helper()
	rows, err := w.db.Query(`SELECT permission_code, COUNT(*) FROM membership_direct_permissions WHERE membership_id = ? AND revoked_at IS NULL GROUP BY permission_code`, membershipID)
	if err != nil {
		w.t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var code string
		var n int
		if err := rows.Scan(&code, &n); err != nil {
			w.t.Fatal(err)
		}
		out[code] = n
	}
	return out
}

// makeOwnerPrimaryAdmin: granting or revoking admin.membership.invite is reserved to the primary admin.
func (w *itWorld) makeOwnerPrimaryAdmin() {
	w.t.Helper()
	if err := w.repo.SetMembershipPrimaryAdmin(context.Background(), w.owner.MembershipID); err != nil {
		w.t.Fatal(err)
	}
}

func (w *itWorld) grantDirect(sub caapp.AdminSubject, code string) {
	w.t.Helper()
	if err := w.repo.InsertDirectPermission(context.Background(), sub.MembershipID, sub.CompanyID, code, sub.UserID); err != nil {
		w.t.Fatalf("grant direct %s: %v", code, err)
	}
}

func (w *itWorld) revokeDirect(sub caapp.AdminSubject, code string) {
	w.t.Helper()
	if err := w.repo.RevokeDirectPermission(context.Background(), sub.MembershipID, code, sub.UserID); err != nil {
		w.t.Fatalf("revoke direct %s: %v", code, err)
	}
}

// snapshot captures a matrix version through a real mutation and returns its version number.
func (w *itWorld) snapshot(code string) int {
	w.t.Helper()
	if err := w.svc.AssignRolePermission(context.Background(), caapp.AssignRolePermissionRequest{
		Subject: w.owner, RoleID: w.custom, PermissionID: w.perm[code],
	}); err != nil {
		w.t.Fatalf("snapshot mutation: %v", err)
	}
	n, err := w.repo.GetMaxRBACMatrixVersionNo(context.Background(), w.company)
	if err != nil {
		w.t.Fatal(err)
	}
	return n
}

func (w *itWorld) rollback(version int) error {
	_, err := w.svc.RollbackRBACMatrixVersion(context.Background(), caapp.RollbackRBACMatrixVersionRequest{
		Subject: w.owner, VersionNo: version, Reason: "integration test",
	})
	return err
}

func (w *itWorld) pending() []caapp.PendingAdminChangeSummary {
	w.t.Helper()
	list, err := w.svc.ListConfigApprovals(context.Background(), caapp.ListConfigApprovalsRequest{
		Subject: w.owner, Status: configversion.ApprovalStatusPending,
	})
	if err != nil {
		w.t.Fatalf("list approvals: %v", err)
	}
	return list.Items
}

func (w *itWorld) approve(id string) error {
	_, err := w.svc.ApproveConfigApproval(context.Background(), caapp.ApproveConfigApprovalRequest{Subject: w.approver, ApprovalID: id})
	return err
}

func requireCode(t *testing.T, err error, status int, code perr.Code) {
	t.Helper()
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != status || he.Code != code {
		t.Fatalf("expected %d/%s, got %v", status, code, err)
	}
}

// --- role SQL: DELETE and INSERT only touch the company's own custom role -------------------

func TestIntegration_RollbackRoleSQL_OnlyTouchesOwnCustomRole(t *testing.T) {
	w := newITWorld(t)
	v := w.snapshot(itDeadline) // custom = {disclosure, deadline}

	// Drift: the custom role gains a permission; the out-of-scope roles change too.
	w.grantRole(w.custom, itDashboard)
	w.grantRole(w.global, itDeadline)
	w.grantRole(w.cmsOp, itDashboard)
	w.grantRole(w.custom2, itDashboard)
	before := w.outOfScope()

	if err := w.rollback(v); err != nil { // DELETE path
		t.Fatalf("rollback: %v", err)
	}
	if got := w.rolePerms(w.custom); has(got, itDashboard) || !has(got, itDeadline) {
		t.Fatalf("DELETE path: custom role = %v, want dashboard.view removed and deadline.view kept", got)
	}

	w.revokeRole(w.custom, itDeadline)
	if err := w.rollback(v); err != nil { // INSERT path
		t.Fatalf("rollback: %v", err)
	}
	if got := w.rolePerms(w.custom); !has(got, itDeadline) {
		t.Fatalf("INSERT path: custom role = %v, want deadline.view restored", got)
	}

	if after := w.outOfScope(); after != before {
		t.Fatalf("roles outside the company's custom roles changed:\nbefore %s\nafter  %s", before, after)
	}
	if got := w.rolePerms(w.cmsOp); !has(got, itPlatformView) || !has(got, itCMSWrite) {
		t.Fatalf("cms_operator lost platform permissions: %v", got)
	}
}

// --- direct grants: only GrantablePermissions, only this company, no duplicate rows ---------

func TestIntegration_RollbackDirectGrantSQL(t *testing.T) {
	w := newITWorld(t)
	w.grantDirect(w.owner, itDirectA)
	w.grantDirect(w.owner, itPlatformView)  // outside GrantablePermissions
	w.grantDirect(w.owner, itDirectNoGrant) // outside GrantablePermissions
	w.grantDirect(w.other, itDirectA)       // another company
	v := w.snapshot(itDeadline)

	w.revokeDirect(w.owner, itDirectA)
	w.revokeDirect(w.owner, itDirectNoGrant)
	w.grantDirect(w.owner, itDirectB)

	if err := w.rollback(v); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	got := w.activeDirect(w.owner.MembershipID)
	if got[itDirectA] != 1 {
		t.Errorf("grantable grant must be restored exactly once, got %v", got)
	}
	if got[itDirectB] != 0 {
		t.Errorf("a grant added after the snapshot must be revoked, got %v", got)
	}
	if got[itPlatformView] != 1 {
		t.Errorf("platform.cms.view direct grant must be untouched, got %v", got)
	}
	if got[itDirectNoGrant] != 0 {
		t.Errorf("a non-grantable grant must not be re-granted, got %v", got)
	}
	if other := w.activeDirect(w.other.MembershipID); other[itDirectA] != 1 {
		t.Errorf("another company's grant must be untouched, got %v", other)
	}

	// A second rollback with nothing to change must not add duplicate active rows.
	if err := w.rollback(v); err != nil {
		t.Fatalf("second rollback: %v", err)
	}
	if got := w.activeDirect(w.owner.MembershipID); got[itDirectA] != 1 {
		t.Errorf("a repeated rollback must not duplicate active grants, got %v", got)
	}
}

// BES-10: a rollback does not give a direct grant back to a membership deactivated since the snapshot.
func TestIntegration_RollbackDoesNotRegrantDirectPermissionToInactiveMembership(t *testing.T) {
	w := newITWorld(t)
	w.grantDirect(w.approver, itDirectA)
	v := w.snapshot(itDeadline)
	w.revokeDirect(w.approver, itDirectA)
	w.exec(`UPDATE memberships SET membership_status = 'inactive' WHERE membership_id = ?`, w.approver.MembershipID)

	if err := w.rollback(v); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if got := w.activeDirect(w.approver.MembershipID); got[itDirectA] != 0 {
		t.Errorf("an inactive membership got its direct grant back: %v", got)
	}
}

// BES-18: approving an alert channel preferences change writes the rule on MySQL (the apply
// path used a column that does not exist, so every approval failed).
func TestIntegration_ApproveNotificationPrefsPatch_WritesRule(t *testing.T) {
	w := newITWorld(t)
	ctx := context.Background()
	def := caapp.DefaultAlertChannelPrefsPayload(w.owner.MembershipID)
	if err := w.svc.CreateNotificationRule(ctx, caapp.CreateNotificationRuleRequest{Subject: w.owner, Payload: map[string]any{
		"rule_code": caapp.AlertChannelPrefsRuleCode, "status": "active",
		"version": def["version"], "event_scope": def["event_scope"], "channels": def["channels"],
		"schedules": []any{}, "recipient_policies": def["recipient_policies"],
	}}); err != nil {
		t.Fatalf("create prefs rule: %v", err)
	}
	rules, err := w.svc.ListNotificationRules(ctx, caapp.ListNotificationRulesRequest{Subject: w.owner})
	if err != nil || len(rules) != 1 {
		t.Fatalf("list rules: %v %d", err, len(rules))
	}
	err = w.svc.UpdateNotificationRule(ctx, caapp.UpdateNotificationRuleRequest{Subject: w.owner, RuleID: rules[0].NotificationRuleID,
		PayloadPatch: map[string]any{"schedules": []any{map[string]any{"kind": "reminder", "enabled": true}}}})
	if he, ok := perr.AsHTTPError(err); !ok || he.Code != perr.CodeApprovalRouted {
		t.Fatalf("prefs update must be queued, got %v", err)
	}
	pending, err := w.svc.ListConfigApprovals(ctx, caapp.ListConfigApprovalsRequest{Subject: w.approver, Status: "pending"})
	if err != nil || len(pending.Items) != 1 {
		t.Fatalf("pending: %v %+v", err, pending)
	}
	if _, err := w.svc.ApproveConfigApproval(ctx, caapp.ApproveConfigApprovalRequest{Subject: w.approver, ApprovalID: pending.Items[0].ApprovalID}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	var raw string
	if err := w.db.QueryRow(`SELECT payload_json FROM notification_rules WHERE notification_rule_id = ?`, rules[0].NotificationRuleID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(raw, `"reminder"`) {
		t.Fatalf("approved patch not written, payload_json = %s", raw)
	}
}

// RP-10 / H16: the transfer is a compare-and-set on the current owner, in one transaction.
func TestIntegration_TransferPrimaryAdmin_IsCompareAndSet(t *testing.T) {
	w := newITWorld(t)
	w.makeOwnerPrimaryAdmin()
	ctx := context.Background()
	if err := w.repo.TransferPrimaryAdmin(ctx, w.company, w.owner.MembershipID, w.approver.MembershipID); err != nil {
		t.Fatalf("transfer: %v", err)
	}
	err := w.repo.TransferPrimaryAdmin(ctx, w.company, w.owner.MembershipID, w.approver.MembershipID)
	if he, ok := perr.AsHTTPError(err); !ok || he.Code != perr.CodeStateConflict {
		t.Fatalf("a second transfer from the former owner must be 409, got %v", err)
	}
	var n int
	if err := w.db.QueryRow(`SELECT COUNT(*) FROM memberships WHERE company_id = ? AND is_primary_admin = 1`, w.company).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("primary admins = %d, want 1", n)
	}
	// Target outside the company -> 404; inactive target -> 409.
	err = w.repo.TransferPrimaryAdmin(ctx, w.company, w.approver.MembershipID, w.other.MembershipID)
	if he, ok := perr.AsHTTPError(err); !ok || he.Code != perr.CodeMembershipNotFound {
		t.Fatalf("target in another company must be 404, got %v", err)
	}
	w.exec(`UPDATE memberships SET membership_status = 'inactive' WHERE membership_id = ?`, w.owner.MembershipID)
	err = w.repo.TransferPrimaryAdmin(ctx, w.company, w.approver.MembershipID, w.owner.MembershipID)
	if he, ok := perr.AsHTTPError(err); !ok || he.Code != perr.CodeStateConflict {
		t.Fatalf("inactive target must be 409, got %v", err)
	}
}

// The invite scope of a department head reads its departments on MySQL (the query ordered by a
// column that does not exist, so every invite by a non-admin returned 500).
func TestIntegration_ListDepartmentIDsByHeadMembership(t *testing.T) {
	w := newITWorld(t)
	w.exec(`INSERT INTO departments (department_id, company_id, department_code, department_name, status, head_membership_id, sort_order)
		VALUES (?, ?, ?, 'IT dept', 'active', ?, 1)`, w.prefix+"-d1", w.company, w.prefix+"-d1", w.owner.MembershipID)
	ids, err := w.repo.ListDepartmentIDsByHeadMembership(context.Background(), w.company, w.owner.MembershipID)
	if err != nil {
		t.Fatalf("list departments by head: %v", err)
	}
	if len(ids) != 1 || ids[0] != w.prefix+"-d1" {
		t.Fatalf("departments = %v", ids)
	}
}

// ROLE-25: the primary admin invariants hold in the write itself on MySQL.
func TestIntegration_PrimaryAdminCannotBeDeactivatedOrDeleted(t *testing.T) {
	w := newITWorld(t)
	w.makeOwnerPrimaryAdmin()
	ctx := context.Background()
	_, err := w.repo.UpdateMembershipStatus(ctx, w.company, w.owner.MembershipID, "inactive")
	if he, ok := perr.AsHTTPError(err); !ok || he.Code != perr.CodeStateConflict {
		t.Fatalf("deactivating the primary admin must be 409, got %v", err)
	}
	err = w.repo.DeleteMembership(ctx, w.company, w.owner.MembershipID)
	if he, ok := perr.AsHTTPError(err); !ok || he.Code != perr.CodeStateConflict {
		t.Fatalf("deleting the primary admin must be 409, got %v", err)
	}
	if _, err := w.repo.UpdateMembershipStatus(ctx, w.company, w.approver.MembershipID, "inactive"); err != nil {
		t.Fatalf("deactivating another member: %v", err)
	}
}

// --- critical rollback goes through approval, and applying it runs the same SQL in a tx ------

func TestIntegration_CriticalRollback_ApprovalApplyRunsRestoreInTx(t *testing.T) {
	w := newITWorld(t)
	w.makeOwnerPrimaryAdmin()
	w.grantDirect(w.owner, itDirectInvite) // critical direct grant
	v := w.snapshot(itDeadline)
	w.revokeDirect(w.owner, itDirectInvite)
	w.grantRole(w.global, itDeadline) // drift of an out-of-scope role while the approval is pending
	before := w.outOfScope()

	requireCode(t, w.rollback(v), 202, perr.CodeApprovalRouted)
	if got := w.activeDirect(w.owner.MembershipID); got[itDirectInvite] != 0 {
		t.Fatalf("nothing may change before approval, got %v", got)
	}
	items := w.pending()
	if len(items) != 1 || items[0].ChangeType != itChangeTypeRollback {
		t.Fatalf("expected one pending %s, got %+v", itChangeTypeRollback, items)
	}

	// The requester cannot approve their own rollback.
	_, err := w.svc.ApproveConfigApproval(context.Background(), caapp.ApproveConfigApprovalRequest{Subject: w.owner, ApprovalID: items[0].ApprovalID})
	requireCode(t, err, 403, perr.CodeSelfApprovalNotAllowed)

	if err := w.approve(items[0].ApprovalID); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if got := w.activeDirect(w.owner.MembershipID); got[itDirectInvite] != 1 {
		t.Fatalf("approved rollback must restore the critical grant, got %v", got)
	}
	if after := w.outOfScope(); after != before {
		t.Fatalf("approval apply changed roles outside the company's custom roles:\nbefore %s\nafter  %s", before, after)
	}
	row, err := w.repo.GetPendingAdminChange(context.Background(), w.company, items[0].ApprovalID)
	if err != nil || row.Status != configversion.ApprovalStatusApproved {
		t.Fatalf("approval must be approved, got %+v err=%v", row, err)
	}
}

func TestIntegration_CriticalRollback_StaleAfterNewVersion(t *testing.T) {
	w := newITWorld(t)
	w.makeOwnerPrimaryAdmin()
	w.grantDirect(w.owner, itDirectInvite)
	v := w.snapshot(itDeadline)
	w.revokeDirect(w.owner, itDirectInvite)
	requireCode(t, w.rollback(v), 202, perr.CodeApprovalRouted)
	items := w.pending()
	if len(items) != 1 {
		t.Fatalf("expected 1 pending approval, got %d", len(items))
	}
	w.snapshot(itDashboard) // a new matrix version lands before the decision
	requireCode(t, w.approve(items[0].ApprovalID), 409, perr.CodeStaleProposal)
	if got := w.activeDirect(w.owner.MembershipID); got[itDirectInvite] != 0 {
		t.Fatalf("a stale approval must change nothing, got %v", got)
	}
}

// --- approval apply of a critical role-permission removal ------------------------------------

func TestIntegration_RolePermRemoveApproval_ApplyKeepsOutOfScopeRoles(t *testing.T) {
	w := newITWorld(t)
	w.grantRole(w.custom, itCritical) // legacy data: a critical permission on a custom role
	w.snapshot(itDeadline)

	err := w.svc.RemoveRolePermission(context.Background(), caapp.RemoveRolePermissionRequest{
		Subject: w.owner, RoleID: w.custom, PermissionID: w.perm[itCritical],
	})
	requireCode(t, err, 202, perr.CodeApprovalRouted)
	items := w.pending()
	if len(items) != 1 {
		t.Fatalf("expected 1 pending approval, got %d", len(items))
	}
	w.grantRole(w.global, itDeadline) // drift while pending
	w.grantRole(w.cmsOp, itDashboard)
	before := w.outOfScope()

	if err := w.approve(items[0].ApprovalID); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if got := w.rolePerms(w.custom); has(got, itCritical) || !has(got, itDeadline) {
		t.Fatalf("custom role after apply = %v, want rbac.manage removed and the rest kept", got)
	}
	if after := w.outOfScope(); after != before {
		t.Fatalf("approval apply changed roles outside the company's custom roles:\nbefore %s\nafter  %s", before, after)
	}
}

// Approvals queued for a protected or global role are refused at submit time.
func TestIntegration_SubmitRolePermRemove_RejectsProtectedAndForeignRoles(t *testing.T) {
	w := newITWorld(t)
	submit := func(roleID, code string) error {
		_, err := w.svc.SubmitConfigApproval(context.Background(), caapp.SubmitConfigApprovalRequest{
			Subject: w.owner, AggregateType: configversion.AggregateRBACMatrix, ChangeType: configversion.ChangeTypeRBACPermissionRemove,
			Proposed: map[string]any{"role_id": roleID, "permission_id": w.perm[code]},
		})
		return err
	}
	requireCode(t, submit(w.cmsOp, itPlatformView), 403, perr.CodeProtectedRoleReadOnly)
	requireCode(t, submit(w.global, itDisclosure), 403, perr.CodeProtectedRoleReadOnly)
	requireCode(t, submit(w.custom2, itDisclosure), 404, perr.CodeNotFound)
	requireCode(t, submit(w.custom, itPlatformView), 400, perr.CodePermissionOutOfEnterpriseScope)
	if n := len(w.pending()); n != 0 {
		t.Fatalf("rejected submissions must not be queued, got %d", n)
	}
}

// --- B: the restore locks the company's custom roles before it reads the state ----------------

func TestIntegration_RestoreLocksCustomRolesBeforeReading(t *testing.T) {
	w := newITWorld(t)
	v := w.snapshot(itDeadline) // the state already equals the snapshot: nothing for the restore to write

	// Another transaction holds the custom role row.
	blocker, err := w.db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	released := false
	defer func() {
		if !released {
			_ = blocker.Rollback()
		}
	}()
	var id string
	if err := blocker.QueryRow(`SELECT role_id FROM roles WHERE role_id = ? FOR UPDATE`, w.custom).Scan(&id); err != nil {
		t.Fatal(err)
	}

	// The restore must queue behind that lock even though it has nothing to write: it locks the
	// roles first and plans from what it reads afterwards.
	done := make(chan error, 1)
	go func() { done <- w.rollback(v) }()
	select {
	case err := <-done:
		t.Fatalf("the restore must wait for the role lock, but it finished (err=%v)", err)
	case <-time.After(500 * time.Millisecond):
	}

	_ = blocker.Rollback()
	released = true
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("rollback after release: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the restore did not finish after the lock was released")
	}
}

// --- B: the critical check is repeated inside the restore transaction -------------------------

func TestIntegration_RestoreInTx_RefusesCriticalUnlessAllowed(t *testing.T) {
	w := newITWorld(t)
	ctx := context.Background()
	w.grantDirect(w.owner, itDirectInvite)
	raw, err := w.repo.BuildRBACMatrixSnapshotJSON(ctx, w.company) // holds the critical grant
	if err != nil {
		t.Fatal(err)
	}
	w.revokeDirect(w.owner, itDirectInvite)
	w.grantRole(w.custom, itDashboard) // a plain change that would be applied together

	err = w.repo.RestoreRBACMatrixFromSnapshot(ctx, w.company, w.owner.UserID, raw, caapp.RBACRestoreOptions{})
	if !errors.Is(err, caapp.ErrRBACRestoreNeedsApproval) {
		t.Fatalf("expected ErrRBACRestoreNeedsApproval, got %v", err)
	}
	if got := w.activeDirect(w.owner.MembershipID); got[itDirectInvite] != 0 {
		t.Fatalf("a refused restore must change nothing, got %v", got)
	}
	if got := w.rolePerms(w.custom); !has(got, itDashboard) {
		t.Fatalf("a refused restore must change nothing, custom role = %v", got)
	}

	if err := w.repo.RestoreRBACMatrixFromSnapshot(ctx, w.company, w.owner.UserID, raw, caapp.RBACRestoreOptions{AllowCritical: true}); err != nil {
		t.Fatalf("restore with AllowCritical: %v", err)
	}
	if got := w.activeDirect(w.owner.MembershipID); got[itDirectInvite] != 1 {
		t.Fatalf("an allowed restore must apply the critical grant, got %v", got)
	}
	if got := w.rolePerms(w.custom); has(got, itDashboard) {
		t.Fatalf("an allowed restore must apply the role change, got %v", got)
	}
}

// --- B: the version stored after an approval is the real state --------------------------------

func TestIntegration_ApprovalApply_StoresPostApplyState(t *testing.T) {
	w := newITWorld(t)
	w.grantRole(w.custom, itCritical)
	w.snapshot(itDeadline)
	err := w.svc.RemoveRolePermission(context.Background(), caapp.RemoveRolePermissionRequest{
		Subject: w.owner, RoleID: w.custom, PermissionID: w.perm[itCritical],
	})
	requireCode(t, err, 202, perr.CodeApprovalRouted)
	items := w.pending()
	if len(items) != 1 {
		t.Fatalf("expected 1 pending approval, got %d", len(items))
	}
	w.grantRole(w.global, itDeadline) // real state moves on while the approval is pending

	if err := w.approve(items[0].ApprovalID); err != nil {
		t.Fatalf("approve: %v", err)
	}
	var source string
	var raw []byte
	if err := w.db.QueryRow(`SELECT source, snapshot_json FROM rbac_matrix_snapshots WHERE company_id = ? ORDER BY version_no DESC LIMIT 1`, w.company).Scan(&source, &raw); err != nil {
		t.Fatal(err)
	}
	if source != configversion.SourceApprovalApply {
		t.Fatalf("latest version source = %q, want approval_apply", source)
	}
	var snap configversion.RBACMatrixSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		t.Fatal(err)
	}
	var globalHasDeadline, customHasCritical bool
	for _, e := range snap.RolePermissions {
		if e.RoleID == w.global && e.PermissionID == w.perm[itDeadline] {
			globalHasDeadline = true
		}
		if e.RoleID == w.custom && e.PermissionID == w.perm[itCritical] {
			customHasCritical = true
		}
	}
	if !globalHasDeadline {
		t.Errorf("the stored version must describe the real state (global role holds deadline.view)")
	}
	if customHasCritical {
		t.Errorf("the stored version must not list the permission the approval removed")
	}
}

// --- B: the restore reads through its own transaction (PERF-15) -------------------------------

// With a pool of one connection a restore that opens a transaction and then reads through the
// pool would wait for a second connection forever.
func TestIntegration_RestoreNeedsOnlyItsTransactionConnection(t *testing.T) {
	w := newITWorld(t)
	v := w.snapshot(itDeadline)
	w.grantRole(w.custom, itDashboard)

	w.db.SetMaxOpenConns(1)
	defer w.db.SetMaxOpenConns(10) // the cleanup needs connections even if the restore hangs
	done := make(chan error, 1)
	go func() { done <- w.rollback(v) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("rollback: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the restore needs a second pool connection while it holds a transaction")
	}
}

// --- D: approvals on real MySQL --------------------------------------------------------------

func TestIntegration_ApprovedDirectGrantRemoval_AppliesForNonGrantableHighRiskCode(t *testing.T) {
	w := newITWorld(t)
	const code = "dept.manage" // not in GrantablePermissions, high risk: removal needs approval
	w.grantDirect(w.owner, code)
	w.grantDirect(w.owner, itDirectNoGrant)
	w.snapshot(itDeadline)

	err := w.svc.RemoveDirectPermission(context.Background(), caapp.RemoveDirectPermissionRequest{
		Subject: w.owner, MembershipID: w.owner.MembershipID, PermissionCode: code,
	})
	requireCode(t, err, 202, perr.CodeApprovalRouted)
	items := w.pending()
	if len(items) != 1 {
		t.Fatalf("expected 1 pending approval, got %d", len(items))
	}
	if err := w.approve(items[0].ApprovalID); err != nil {
		t.Fatalf("approve: %v", err)
	}
	got := w.activeDirect(w.owner.MembershipID)
	if got[code] != 0 {
		t.Errorf("the approved removal must revoke %s, now %v", code, got)
	}
	if got[itDirectNoGrant] != 1 {
		t.Errorf("only the requested grant may be revoked, now %v", got)
	}
}

func TestIntegration_Approval_NothingToApply_StaysPending(t *testing.T) {
	w := newITWorld(t)
	ctx := context.Background()
	w.snapshot(itDeadline)
	// A proposal queued the way an older binary did: it only differs on a protected role.
	raw, err := w.repo.BuildRBACMatrixSnapshotJSON(ctx, w.company)
	if err != nil {
		t.Fatal(err)
	}
	var snap configversion.RBACMatrixSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		t.Fatal(err)
	}
	kept := snap.RolePermissions[:0]
	for _, e := range snap.RolePermissions {
		if e.RoleID == w.cmsOp && e.PermissionID == w.perm[itDisclosure] {
			continue
		}
		kept = append(kept, e)
	}
	snap.RolePermissions = kept
	proposed, _ := json.Marshal(snap)
	ver, err := w.repo.GetMaxRBACMatrixVersionNo(ctx, w.company)
	if err != nil {
		t.Fatal(err)
	}
	row, err := w.repo.InsertPendingAdminChange(ctx, caapp.InsertPendingAdminChangeInput{
		ID: w.prefix + "-legacy", CompanyID: w.company, ApprovalSubjectType: configversion.ApprovalSubjectConfigSnapshot,
		AggregateType: configversion.AggregateRBACMatrix, ChangeType: configversion.ChangeTypeRBACPermissionRemove,
		ProposedSnapshotJSON: proposed, BaseLiveVersionNo: &ver, RequestedBy: w.owner.MembershipID,
	})
	if err != nil {
		t.Fatal(err)
	}

	requireCode(t, w.approve(row.ID), 409, perr.CodeApprovalNothingToApply)
	got, err := w.repo.GetPendingAdminChange(ctx, w.company, row.ID)
	if err != nil || got.Status != configversion.ApprovalStatusPending {
		t.Fatalf("a refused approval must stay pending, got %+v err=%v", got, err)
	}
	if perms := w.rolePerms(w.cmsOp); !has(perms, itDisclosure) {
		t.Fatalf("the protected role must be untouched, got %v", perms)
	}
}

// --- ROLE-19: an approval applies what the approver was shown (real MySQL) -----------------------

func TestIntegration_ApprovalSingleChange_DoesNotRevokeLaterDirectGrants(t *testing.T) {
	w := newITWorld(t)
	w.grantRole(w.custom, itCritical)
	w.snapshot(itDeadline)
	err := w.svc.RemoveRolePermission(context.Background(), caapp.RemoveRolePermissionRequest{
		Subject: w.owner, RoleID: w.custom, PermissionID: w.perm[itCritical],
	})
	requireCode(t, err, 202, perr.CodeApprovalRouted)
	items := w.pending()
	w.grantDirect(w.owner, itDirectA) // written without a new matrix version (invite flows do this)

	if err := w.approve(items[0].ApprovalID); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if got := w.rolePerms(w.custom); has(got, itCritical) {
		t.Fatalf("the requested removal must be applied, got %v", got)
	}
	if got := w.activeDirect(w.owner.MembershipID); got[itDirectA] != 1 {
		t.Fatalf("a direct grant made later must survive, got %v", got)
	}
}

func TestIntegration_ApprovalRollback_BoundToReviewedPlan(t *testing.T) {
	w := newITWorld(t)
	w.makeOwnerPrimaryAdmin()
	w.grantDirect(w.owner, itDirectInvite)
	v := w.snapshot(itDeadline)
	w.revokeDirect(w.owner, itDirectInvite)
	requireCode(t, w.rollback(v), 202, perr.CodeApprovalRouted)
	items := w.pending()
	w.grantDirect(w.owner, itDirectA) // the rollback would now revoke this too

	requireCode(t, w.approve(items[0].ApprovalID), 409, perr.CodeStaleProposal)
	got := w.activeDirect(w.owner.MembershipID)
	if got[itDirectA] != 1 || got[itDirectInvite] != 0 {
		t.Fatalf("a stale approval must change nothing, got %v", got)
	}
}

func TestIntegration_RestoreInTx_RefusesPlanThatDiffersFromTheReviewedOne(t *testing.T) {
	w := newITWorld(t)
	ctx := context.Background()
	w.grantDirect(w.owner, itDirectA)
	raw, err := w.repo.BuildRBACMatrixSnapshotJSON(ctx, w.company)
	if err != nil {
		t.Fatal(err)
	}
	w.revokeDirect(w.owner, itDirectA)
	var snap configversion.RBACMatrixSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		t.Fatal(err)
	}
	snap.PlanDigest = "0000000000000000000000000000000000000000000000000000000000000000"
	bad, _ := json.Marshal(snap)

	err = w.repo.RestoreRBACMatrixFromSnapshot(ctx, w.company, w.owner.UserID, bad, caapp.RBACRestoreOptions{AllowCritical: true})
	if !errors.Is(err, caapp.ErrRBACRestorePlanChanged) {
		t.Fatalf("expected ErrRBACRestorePlanChanged, got %v", err)
	}
	if got := w.activeDirect(w.owner.MembershipID); got[itDirectA] != 0 {
		t.Fatalf("a refused restore must change nothing, got %v", got)
	}
}

// --- BES-01: restores of one company queue up even when it has no roles and no grants to lock ----------

func TestIntegration_RestoreSerialisesOnTheCompanyRow(t *testing.T) {
	w := newITWorld(t)
	ctx := context.Background()
	// Call the repository directly: the service would also wait later, on the version insert.
	raw, err := w.repo.BuildRBACMatrixSnapshotJSON(ctx, w.company) // equals the state: nothing to write

	if err != nil {
		t.Fatal(err)
	}
	blocker, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	released := false
	defer func() {
		if !released {
			_ = blocker.Rollback()
		}
	}()
	var id string
	if err := blocker.QueryRow(`SELECT company_id FROM companies WHERE company_id = ? FOR UPDATE`, w.company).Scan(&id); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- w.repo.RestoreRBACMatrixFromSnapshot(ctx, w.company, w.owner.UserID, raw, caapp.RBACRestoreOptions{})
	}()
	select {
	case err := <-done:
		t.Fatalf("the restore must wait for the company lock, but it finished (err=%v)", err)
	case <-time.After(500 * time.Millisecond):
	}
	_ = blocker.Rollback()
	released = true
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("restore after release: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the restore did not finish after the lock was released")
	}
}
