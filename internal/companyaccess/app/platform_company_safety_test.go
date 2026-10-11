package app_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
	auditinmem "github.com/cobo/cobo_iam_services/internal/audit/infra/inmemory"
	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// Contract: docs/ai-cache/cms-companies-safety-2026-10-11/02-contract.md

type safetyIDGen struct{ n int }

func (g *safetyIDGen) NewUUID() string { g.n++; return fmt.Sprintf("evt-%d", g.n) }

type capturedEmails struct{ sent []caapp.CompanyStatusEmail }

func (c *capturedEmails) emailer(_ context.Context, e caapp.CompanyStatusEmail) error {
	c.sent = append(c.sent, e)
	return nil
}

var safetySub = caapp.AdminSubject{UserID: "u_op", MembershipID: "m_op", CompanyID: "c_platform"}

func safetySvc(repo *cainmem.AdminRepository, opts ...caapp.AdminOption) caapp.AdminService {
	return caapp.NewAdminService(repo,
		fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"platform.cms.view", "rbac.manage"}},
		&safetyIDGen{}, opts...)
}

func seedSafetyCompany(repo *cainmem.AdminRepository, status string) {
	repo.SeedCompany(caapp.PlatformCompanyDetail{
		CompanyID: "c1", CompanyCode: "ABC001", CompanyName: "Công ty ABC", Status: status,
		VerificationStatus: "unverified", ContactEmail: "lienhe@abc.vn", Phone: "0912345678",
		RepresentativeName: "Nguyễn Văn An", UpdatedAtRFC3339: "2026-10-01T03:00:00Z",
	})
}

func fieldError(t *testing.T, err error, field string) {
	t.Helper()
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("want 400 with field %s, got %v", field, err)
	}
	fe, _ := he.Details["field_errors"].(map[string]any)
	if _, ok := fe[field]; !ok {
		t.Fatalf("field_errors %v missing %s", fe, field)
	}
}

func companyStatus(t *testing.T, repo *cainmem.AdminRepository) string {
	t.Helper()
	d, err := repo.GetCompanyPlatform(context.Background(), "c1")
	if err != nil {
		t.Fatal(err)
	}
	return d.Status
}

func TestCompanyStatus_TransitionsReasonsAndConfirmation(t *testing.T) {
	ctx := context.Background()
	repo := cainmem.NewAdminRepository()
	seedSafetyCompany(repo, "active")
	svc := safetySvc(repo)

	// Reason is required and must belong to the action.
	_, err := svc.SetPlatformCompanyStatus(ctx, caapp.SetPlatformCompanyStatusRequest{Subject: safetySub, CompanyID: "c1", Status: "suspended"})
	fieldError(t, err, "reason_code")
	_, err = svc.SetPlatformCompanyStatus(ctx, caapp.SetPlatformCompanyStatusRequest{Subject: safetySub, CompanyID: "c1", Status: "suspended", ReasonCode: caapp.CompanyReasonPaymentReceived})
	fieldError(t, err, "reason_code")
	_, err = svc.SetPlatformCompanyStatus(ctx, caapp.SetPlatformCompanyStatusRequest{Subject: safetySub, CompanyID: "c1", Status: "suspended", ReasonCode: "OTHER"})
	fieldError(t, err, "reason_note")
	_, err = svc.SetPlatformCompanyStatus(ctx, caapp.SetPlatformCompanyStatusRequest{Subject: safetySub, CompanyID: "c1", Status: "suspended", ReasonCode: "OTHER", ReasonNote: strings.Repeat("x", 501)})
	fieldError(t, err, "reason_note")

	// active → suspended
	change, err := svc.SetPlatformCompanyStatus(ctx, caapp.SetPlatformCompanyStatusRequest{Subject: safetySub, CompanyID: "c1", Status: "suspended", ReasonCode: "payment_overdue"})
	if err != nil || !change.Changed || change.From != "active" || change.To != "suspended" {
		t.Fatalf("suspend: change=%+v err=%v", change, err)
	}
	// Same status again: no-op, no audit.
	before := len(repo.CompanyAuditRows())
	change, err = svc.SetPlatformCompanyStatus(ctx, caapp.SetPlatformCompanyStatusRequest{Subject: safetySub, CompanyID: "c1", Status: "suspended", ReasonCode: "SECURITY"})
	if err != nil || change.Changed {
		t.Fatalf("no-op: change=%+v err=%v", change, err)
	}
	if len(repo.CompanyAuditRows()) != before {
		t.Fatal("no-op must not write an audit row")
	}

	// Deactivate needs the company code (case-insensitive).
	_, err = svc.SetPlatformCompanyStatus(ctx, caapp.SetPlatformCompanyStatusRequest{Subject: safetySub, CompanyID: "c1", Status: "inactive", ReasonCode: "CONTRACT_ENDED"})
	fieldError(t, err, "confirm_company_code")
	_, err = svc.SetPlatformCompanyStatus(ctx, caapp.SetPlatformCompanyStatusRequest{Subject: safetySub, CompanyID: "c1", Status: "inactive", ReasonCode: "CONTRACT_ENDED", ConfirmCompanyCode: "XYZ"})
	fieldError(t, err, "confirm_company_code")
	if companyStatus(t, repo) != "suspended" {
		t.Fatal("wrong confirmation code must not change the status")
	}
	if _, err = svc.SetPlatformCompanyStatus(ctx, caapp.SetPlatformCompanyStatusRequest{Subject: safetySub, CompanyID: "c1", Status: "inactive", ReasonCode: "CONTRACT_ENDED", ConfirmCompanyCode: " abc001 "}); err != nil {
		t.Fatalf("deactivate: %v", err)
	}

	// inactive → suspended is refused (D-07).
	_, err = svc.SetPlatformCompanyStatus(ctx, caapp.SetPlatformCompanyStatusRequest{Subject: safetySub, CompanyID: "c1", Status: "suspended", ReasonCode: "SECURITY"})
	requireHTTPCode(t, err, http.StatusConflict, perr.CodeInvalidStateTransition)

	// inactive → active
	if _, err = svc.SetPlatformCompanyStatus(ctx, caapp.SetPlatformCompanyStatusRequest{Subject: safetySub, CompanyID: "c1", Status: "active", ReasonCode: "PAYMENT_RECEIVED"}); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if companyStatus(t, repo) != "active" {
		t.Fatal("company should be active again")
	}
}

func TestCompanyStatus_AuditRowAndRollback(t *testing.T) {
	ctx := context.Background()
	repo := cainmem.NewAdminRepository()
	seedSafetyCompany(repo, "active")
	svc := safetySvc(repo)

	change, err := svc.SetPlatformCompanyStatus(ctx, caapp.SetPlatformCompanyStatusRequest{
		Subject: safetySub, CompanyID: "c1", Status: "suspended", ReasonCode: "OTHER", ReasonNote: "Gọi 2 lần", RequestID: "req-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	rows := repo.CompanyAuditRows()
	if len(rows) != 1 {
		t.Fatalf("audit rows = %d, want 1", len(rows))
	}
	row := rows[0]
	if row.Record.Action != caapp.CompanyAuditStatusChange || row.Record.EventID != change.AuditEventID || row.CompanyID != "c1" {
		t.Fatalf("audit row = %+v (change %+v)", row, change)
	}
	if row.Actor.ActorUserID != "u_op" || row.Actor.RequestID != "req-1" {
		t.Fatalf("audit actor = %+v", row.Actor)
	}
	meta := row.Record.Metadata
	if meta["reason_code"] != "OTHER" || meta["reason_note"] != "Gọi 2 lần" {
		t.Fatalf("audit metadata = %+v", meta)
	}
	if b, _ := meta["before"].(map[string]any); b["status"] != "active" {
		t.Fatalf("before = %+v", meta["before"])
	}

	// Audit failure → nothing changes.
	repo.FailNextCompanyAudit()
	if _, err := svc.SetPlatformCompanyStatus(ctx, caapp.SetPlatformCompanyStatusRequest{Subject: safetySub, CompanyID: "c1", Status: "active", ReasonCode: "ISSUE_RESOLVED"}); err == nil {
		t.Fatal("expected error when the audit write fails")
	}
	if companyStatus(t, repo) != "suspended" {
		t.Fatal("status must not change when the audit write fails")
	}
}

func TestCompanyRevision_StaleAndFresh(t *testing.T) {
	ctx := context.Background()
	repo := cainmem.NewAdminRepository()
	seedSafetyCompany(repo, "active")
	svc := safetySvc(repo)

	detail, err := svc.GetPlatformCompany(ctx, caapp.GetPlatformCompanyRequest{Subject: safetySub, CompanyID: "c1"})
	if err != nil || !strings.HasPrefix(detail.Revision, "co1:") {
		t.Fatalf("detail revision = %q err=%v", detail.Revision, err)
	}
	addr := "12 Lý Thường Kiệt, Hà Nội"
	if err := svc.UpdatePlatformCompany(ctx, caapp.UpdatePlatformCompanyRequest{Subject: safetySub, CompanyID: "c1", Address: &addr, ExpectedRevision: detail.Revision}); err != nil {
		t.Fatalf("fresh revision: %v", err)
	}
	// The old revision is now stale.
	name := "Công ty ABC mới"
	err = svc.UpdatePlatformCompany(ctx, caapp.UpdatePlatformCompanyRequest{Subject: safetySub, CompanyID: "c1", CompanyName: &name, ExpectedRevision: detail.Revision})
	requireHTTPCode(t, err, http.StatusConflict, perr.CodeStaleEtag)
	var he *perr.HTTPError
	if !errors.As(err, &he) || he.Details["current_revision"] == "" {
		t.Fatalf("stale error must carry current_revision: %v", err)
	}
	_, err = svc.SetPlatformCompanyStatus(ctx, caapp.SetPlatformCompanyStatusRequest{Subject: safetySub, CompanyID: "c1", Status: "suspended", ReasonCode: "SECURITY", ExpectedRevision: detail.Revision})
	requireHTTPCode(t, err, http.StatusConflict, perr.CodeStaleEtag)
	// Without a revision (old clients) the write goes through.
	if err := svc.UpdatePlatformCompany(ctx, caapp.UpdatePlatformCompanyRequest{Subject: safetySub, CompanyID: "c1", CompanyName: &name}); err != nil {
		t.Fatalf("no revision: %v", err)
	}
}

func TestCompanyUpdate_AuditMasksPersonalDataAndSkipsNoop(t *testing.T) {
	ctx := context.Background()
	repo := cainmem.NewAdminRepository()
	seedSafetyCompany(repo, "active")
	svc := safetySvc(repo)

	email, phone, rep, verified := "ketoan@abc.vn", "0987654321", "Trần Thị Bình", "verified"
	same := "Công ty ABC"
	if err := svc.UpdatePlatformCompany(ctx, caapp.UpdatePlatformCompanyRequest{
		Subject: safetySub, CompanyID: "c1", ContactEmail: &email, Phone: &phone, RepresentativeName: &rep,
		VerificationStatus: &verified, CompanyName: &same,
	}); err != nil {
		t.Fatal(err)
	}
	rows := repo.CompanyAuditRows()
	if len(rows) != 2 || rows[0].Record.Action != caapp.CompanyAuditUpdate || rows[1].Record.Action != caapp.CompanyAuditVerificationChange {
		t.Fatalf("audit rows = %+v", rows)
	}
	changes, _ := rows[0].Record.Metadata["changes"].([]map[string]any)
	byField := map[string]map[string]any{}
	for _, c := range changes {
		byField[c["field"].(string)] = c
	}
	if _, ok := byField["company_name"]; ok {
		t.Fatal("unchanged company_name must not appear in changes")
	}
	if got := byField["contact_email"]; got["before"] != "l***@a***.vn" || got["after"] != "k***@a***.vn" || got["masked"] != true {
		t.Fatalf("email change = %+v", got)
	}
	if got := byField["phone"]; got["after"] != "09*****321" {
		t.Fatalf("phone change = %+v", got)
	}
	if got := byField["representative_name"]; got["before"] != "N. V. A." || got["after"] != "T. T. B." {
		t.Fatalf("name change = %+v", got)
	}

	// Same values again: no write, no audit.
	if err := svc.UpdatePlatformCompany(ctx, caapp.UpdatePlatformCompanyRequest{Subject: safetySub, CompanyID: "c1", ContactEmail: &email}); err != nil {
		t.Fatal(err)
	}
	if len(repo.CompanyAuditRows()) != 2 {
		t.Fatal("no-op update must not be audited")
	}
}

func TestCompanyUpdate_Validation(t *testing.T) {
	ctx := context.Background()
	repo := cainmem.NewAdminRepository()
	seedSafetyCompany(repo, "active")
	svc := safetySvc(repo)
	bad := func(field, value string) caapp.UpdatePlatformCompanyRequest {
		req := caapp.UpdatePlatformCompanyRequest{Subject: safetySub, CompanyID: "c1"}
		v := value
		switch field {
		case "tax_code":
			req.TaxCode = &v
		case "contact_email":
			req.ContactEmail = &v
		case "phone":
			req.Phone = &v
		case "company_name":
			req.CompanyName = &v
		case "verification_status":
			req.VerificationStatus = &v
		}
		return req
	}
	for _, tc := range []struct{ field, value string }{
		{"tax_code", "12345"}, {"tax_code", "0101234567-1"}, {"contact_email", "not-an-email"},
		{"phone", "abc"}, {"company_name", "   "}, {"verification_status", "pending"},
	} {
		fieldError(t, svc.UpdatePlatformCompany(ctx, bad(tc.field, tc.value)), tc.field)
	}
	for _, ok := range []string{"0101234567", "0101234567-001", ""} {
		if err := svc.UpdatePlatformCompany(ctx, bad("tax_code", ok)); err != nil {
			t.Fatalf("tax_code %q: %v", ok, err)
		}
	}
}

func TestCompanyStatus_EmailRecipientsAndContent(t *testing.T) {
	ctx := context.Background()
	repo := cainmem.NewAdminRepository()
	seedSafetyCompany(repo, "active")
	seedMem(repo, "m_admin", "u_admin", "c1")
	repo.SetPrimaryAdmin("m_admin", true)
	seedMem(repo, "m_staff", "u_staff", "c1")
	mail := &capturedEmails{}
	svc := safetySvc(repo, caapp.WithCompanyStatusEmailer(mail.emailer))

	change, err := svc.SetPlatformCompanyStatus(ctx, caapp.SetPlatformCompanyStatusRequest{
		Subject: safetySub, CompanyID: "c1", Status: "suspended", ReasonCode: "OTHER", ReasonNote: "ghi chú nội bộ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(mail.sent) != 1 || mail.sent[0].To != "u_admin@test.com" || change.NotifiedCount != 1 {
		t.Fatalf("sent = %+v, change = %+v", mail.sent, change)
	}
	e := mail.sent[0]
	if e.IdempotencyKey != "company_status:c1:"+change.AuditEventID+":u_admin@test.com" {
		t.Fatalf("idempotency key = %q", e.IdempotencyKey)
	}
	if e.Variables["new_status_label"] != "Tạm ngưng" || e.Variables["reason_label"] != "Khác" || e.Variables["company_code"] != "ABC001" {
		t.Fatalf("variables = %+v", e.Variables)
	}
	for _, v := range e.Variables {
		if s, ok := v.(string); ok && strings.Contains(s, "ghi chú nội bộ") {
			t.Fatal("the internal note must not be emailed")
		}
	}

	// No primary admin → contact email; none at all → 0.
	repo.SetPrimaryAdmin("m_admin", false)
	mail.sent = nil
	if _, err := svc.SetPlatformCompanyStatus(ctx, caapp.SetPlatformCompanyStatusRequest{Subject: safetySub, CompanyID: "c1", Status: "active", ReasonCode: "ISSUE_RESOLVED"}); err != nil {
		t.Fatal(err)
	}
	if len(mail.sent) != 1 || mail.sent[0].To != "lienhe@abc.vn" {
		t.Fatalf("fallback recipient = %+v", mail.sent)
	}
	empty := ""
	if err := svc.UpdatePlatformCompany(ctx, caapp.UpdatePlatformCompanyRequest{Subject: safetySub, CompanyID: "c1", ContactEmail: &empty}); err != nil {
		t.Fatal(err)
	}
	mail.sent = nil
	change, err = svc.SetPlatformCompanyStatus(ctx, caapp.SetPlatformCompanyStatusRequest{Subject: safetySub, CompanyID: "c1", Status: "suspended", ReasonCode: "SECURITY"})
	if err != nil || len(mail.sent) != 0 || change.NotifiedCount != 0 {
		t.Fatalf("no recipient: sent=%d change=%+v err=%v", len(mail.sent), change, err)
	}
}

func TestCompanyHistory_ListsCompanyActionsWithActorNames(t *testing.T) {
	ctx := context.Background()
	repo := cainmem.NewAdminRepository()
	seedSafetyCompany(repo, "active")
	seedMem(repo, "m_op", "u_op", "c_platform")
	audit := auditinmem.NewRepository()
	for i, e := range []auditapp.Entry{
		{EventID: "e1", OccurredAt: "2026-10-01T01:00:00Z", ActorUserID: "u_op", CompanyID: "c1", Action: caapp.CompanyAuditStatusChange,
			Metadata: map[string]any{"before": map[string]any{"status": "active"}, "after": map[string]any{"status": "suspended"}, "reason_code": "SECURITY", "reason_note": "n"}},
		{EventID: "e2", OccurredAt: "2026-10-01T02:00:00Z", ActorUserID: "u_op", CompanyID: "c1", Action: "admin.membership.create"},
		{EventID: "e3", OccurredAt: "2026-10-01T03:00:00Z", ActorUserID: "u_op", CompanyID: "c2", Action: caapp.CompanyAuditUpdate},
	} {
		if err := audit.Append(ctx, e); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}
	svc := safetySvc(repo, caapp.WithAuditRepository(audit))
	out, err := svc.ListPlatformCompanyHistory(ctx, caapp.ListPlatformCompanyHistoryRequest{Subject: safetySub, CompanyID: "c1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Items) != 1 || out.Items[0].EventID != "e1" || out.Items[0].ReasonCode != "SECURITY" || out.Items[0].After["status"] != "suspended" {
		t.Fatalf("history = %+v", out.Items)
	}
	if out.Items[0].Actor.DisplayName == "" {
		t.Fatalf("actor display name missing: %+v", out.Items[0].Actor)
	}
}

func TestPlatformCompanySafety_TenantPermissionsForbidden(t *testing.T) {
	repo := cainmem.NewAdminRepository()
	seedSafetyCompany(repo, "active")
	svc := caapp.NewAdminService(repo, fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"rbac.manage", "admin.membership.invite"}}, &safetyIDGen{})
	_, err := svc.SetPlatformCompanyStatus(context.Background(), caapp.SetPlatformCompanyStatusRequest{Subject: safetySub, CompanyID: "c1", Status: "suspended", ReasonCode: "SECURITY"})
	requireHTTPCode(t, err, http.StatusForbidden, perr.CodePermissionDenied)
	_, err = svc.ListPlatformCompanyHistory(context.Background(), caapp.ListPlatformCompanyHistoryRequest{Subject: safetySub, CompanyID: "c1"})
	requireHTTPCode(t, err, http.StatusForbidden, perr.CodePermissionDenied)
}

// D-02: tenant audit views never show platform CMS actions on the company.
func TestTenantAuditLogs_ExcludePlatformCompanyActions(t *testing.T) {
	ctx := context.Background()
	repo := cainmem.NewAdminRepository()
	audit := auditinmem.NewRepository()
	for _, e := range []auditapp.Entry{
		{EventID: "e1", OccurredAt: "2026-10-01T01:00:00Z", CompanyID: "c1", Action: caapp.CompanyAuditStatusChange},
		{EventID: "e2", OccurredAt: "2026-10-01T02:00:00Z", CompanyID: "c1", Action: "cms.company_plan.activate"}, // stays visible (D-02 is cms.company.* only)
		{EventID: "e3", OccurredAt: "2026-10-01T03:00:00Z", CompanyID: "c1", Action: "admin.membership.create"},
	} {
		if err := audit.Append(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	svc := caapp.NewAdminService(repo,
		fakeAuthService{decision: authapp.DecisionAllow, permissions: []string{"rbac.manage", "system.settings"}},
		&safetyIDGen{}, caapp.WithAuditRepository(audit))
	tenant := caapp.AdminSubject{UserID: "u_admin", MembershipID: "m_admin", CompanyID: "c1"}
	out, err := svc.ListAuditLogs(ctx, caapp.ListAuditLogsRequest{Subject: tenant})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range out.Items {
		if strings.HasPrefix(item.Action, "cms.company.") {
			t.Fatalf("tenant audit leaked %q", item.Action)
		}
	}
	if len(out.Items) != 2 {
		t.Fatalf("items = %+v, want admin.membership.create + cms.company_plan.activate", out.Items)
	}
}

// Follow-up 2026-10-11: keyset cursor (occurred_at, event_id) — same-second events are neither lost nor repeated.
func TestCompanyHistory_CursorPagesSameSecondEvents(t *testing.T) {
	ctx := context.Background()
	repo := cainmem.NewAdminRepository()
	seedSafetyCompany(repo, "active")
	audit := auditinmem.NewRepository()
	for _, e := range []auditapp.Entry{
		// Inserted out of order (time and event id) so the repository ordering is really exercised.
		{EventID: "e5", OccurredAt: "2026-10-01T03:00:00Z", CompanyID: "c1", Action: caapp.CompanyAuditUpdate},
		{EventID: "e3", OccurredAt: "2026-10-01T02:00:00Z", CompanyID: "c1", Action: caapp.CompanyAuditStatusChange},
		{EventID: "e1", OccurredAt: "2026-10-01T01:00:00Z", CompanyID: "c1", Action: caapp.CompanyAuditCreate},
		{EventID: "e4", OccurredAt: "2026-10-01T02:00:00Z", CompanyID: "c1", Action: "cms.company_plan.activate"},
		{EventID: "e2", OccurredAt: "2026-10-01T02:00:00Z", CompanyID: "c1", Action: caapp.CompanyAuditUpdate},
	} {
		if err := audit.Append(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	svc := safetySvc(repo, caapp.WithAuditRepository(audit))
	var got []string
	cursor := ""
	for page := 0; page < 5; page++ {
		out, err := svc.ListPlatformCompanyHistory(ctx, caapp.ListPlatformCompanyHistoryRequest{Subject: safetySub, CompanyID: "c1", Limit: 2, Cursor: cursor})
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range out.Items {
			got = append(got, item.EventID)
		}
		if out.NextCursor == "" {
			break
		}
		if !strings.HasPrefix(out.NextCursor, "h1.") {
			t.Fatalf("cursor must be opaque h1.: %q", out.NextCursor)
		}
		cursor = out.NextCursor
	}
	if strings.Join(got, ",") != "e5,e4,e3,e2,e1" {
		t.Fatalf("paged history = %v, want e5,e4,e3,e2,e1", got)
	}
	// Exactly limit rows left: no cursor to an empty page.
	out, err := svc.ListPlatformCompanyHistory(ctx, caapp.ListPlatformCompanyHistoryRequest{Subject: safetySub, CompanyID: "c1", Limit: 5})
	if err != nil || len(out.Items) != 5 || out.NextCursor != "" {
		t.Fatalf("full single page: items=%d next=%q err=%v", len(out.Items), out.NextCursor, err)
	}
}

func TestCompanyHistory_CursorValidation(t *testing.T) {
	ctx := context.Background()
	repo := cainmem.NewAdminRepository()
	seedSafetyCompany(repo, "active")
	audit := auditinmem.NewRepository()
	for _, e := range []auditapp.Entry{
		{EventID: "e1", OccurredAt: "2026-10-01T01:00:00Z", CompanyID: "c1", Action: caapp.CompanyAuditCreate},
		{EventID: "e2", OccurredAt: "2026-10-01T02:00:00Z", CompanyID: "c1", Action: caapp.CompanyAuditUpdate},
	} {
		if err := audit.Append(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	svc := safetySvc(repo, caapp.WithAuditRepository(audit))
	for _, bad := range []string{"abc", "h1.!!!", "h1.bm9waXBl", "2026-10-01T02:00:00.5Z"} {
		_, err := svc.ListPlatformCompanyHistory(ctx, caapp.ListPlatformCompanyHistoryRequest{Subject: safetySub, CompanyID: "c1", Cursor: bad})
		fieldError(t, err, "cursor")
	}
	// Legacy cursor (bare RFC3339, before 2026-10-11) still pages by second.
	out, err := svc.ListPlatformCompanyHistory(ctx, caapp.ListPlatformCompanyHistoryRequest{Subject: safetySub, CompanyID: "c1", Cursor: "2026-10-01T09:00:00+07:00"})
	if err != nil || len(out.Items) != 1 || out.Items[0].EventID != "e1" {
		t.Fatalf("legacy cursor: items=%+v err=%v", out, err)
	}
}
